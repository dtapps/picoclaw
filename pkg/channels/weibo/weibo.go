package weibo

import (
	"context"
	"fmt"
	"strings"

	weibo "github.com/dtapps/weibo-go"
	weiboConfig "github.com/dtapps/weibo-go/config"
	weiboLogger "github.com/dtapps/weibo-go/logger"
	weiboTypes "github.com/dtapps/weibo-go/types"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/identity"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type WeiboChannel struct {
	*channels.BaseChannel
	bc     *config.Channel
	config *config.WeiboSettings

	weiboClient *weibo.Client

	ctx    context.Context
	cancel context.CancelFunc
}

func NewWeiboChannel(
	bc *config.Channel,
	cfg *config.WeiboSettings,
	messageBus *bus.MessageBus,
	logLevel string,
) (*WeiboChannel, error) {
	if cfg.AppID == "" || cfg.AppSecret.String() == "" {
		return nil, fmt.Errorf("weibo app_id and app_secret are required")
	}

	weiboLogger.SetLevelByName(logLevel)

	base := channels.NewBaseChannel(
		config.ChannelWeibo,
		cfg,
		messageBus,
		bc.AllowFrom.FilterEmpty(),
		channels.WithReasoningChannelID(bc.ReasoningChannelID),
	)

	return &WeiboChannel{
		BaseChannel: base,
		bc:          bc,
		config:      cfg,
	}, nil
}

func (c *WeiboChannel) Name() string { return config.ChannelWeibo }

func (c *WeiboChannel) Start(ctx context.Context) error {
	logger.InfoC(c.Name(), "Weibo channel started...")

	c.ctx, c.cancel = context.WithCancel(ctx)

	var err error

	// 创建配置
	defaultCfg := weiboConfig.DefaultConfig()
	defaultCfg.AppID = c.config.AppID
	defaultCfg.AppSecret = c.config.AppSecret.String()

	// 创建客户端
	c.weiboClient, err = weibo.NewClient("picoclaw", &weiboTypes.Config{
		Weibo: defaultCfg,
	})
	if err != nil {
		return fmt.Errorf("weibo new client failed: %w", err)
	}

	c.weiboClient.OnConnected(func() {
		logger.InfoC(c.Name(), "Weibo channel connected...")
		c.SetRunning(true)
	})

	c.weiboClient.OnDisconnected(func() {
		logger.InfoC(c.Name(), "Weibo channel disconnected...")
		c.SetRunning(false)
	})

	c.weiboClient.OnMessage(func(msg *weiboTypes.InboundMessage) {
		if msg == nil {
			return
		}

		content := ""
		for _, segment := range msg.Content {
			content += strings.TrimSpace(segment.Text)
		}
		if content == "" {
			return // 忽略空消息
		}

		// 构建发送者信息
		sender := bus.SenderInfo{
			Platform:    c.Name(),                                          // 平台名称
			PlatformID:  msg.SenderID,                                      // 原始 ID
			CanonicalID: identity.BuildCanonicalID(c.Name(), msg.SenderID), // 规范化 ID
			Username:    msg.SenderID,                                      // 用户名
			DisplayName: msg.SenderID,                                      // 显示名称
		}

		// 权限校验
		if !c.IsAllowedSender(sender) {
			return
		}

		// 构建标准化上下文
		inboundCtx := bus.InboundContext{
			Channel:          c.Name(),            // 来源渠道
			Account:          msg.AppID,           // 机器人账号
			ChatID:           msg.SenderID,        // 会话 ID / 用户 ID
			ChatType:         "direct",            // 会话类型 direct / group
			TopicID:          "",                  // 话题 ID
			SpaceID:          "",                  // 空间 ID
			SpaceType:        "",                  // 空间类型
			SenderID:         msg.SenderID,        // 发送者 ID
			MessageID:        msg.MessageID,       // 消息 ID
			Mentioned:        false,               // 是否被提及其
			ReplyToMessageID: "",                  // 回复消息 ID
			ReplyHandles:     map[string]string{}, // 回复句柄
			Raw: map[string]string{
				"raw_data": string(msg.RawMessage),
			}, // 原始数据
		}

		// 媒体处理
		mediaRefs := []string{}

		// 调用统一处理入口
		c.HandleInboundContext(
			c.ctx,
			inboundCtx.ChatID,
			content,
			mediaRefs,
			inboundCtx,
			sender,
		)
	})

	return nil
}

func (c *WeiboChannel) Stop(ctx context.Context) error {
	logger.InfoC(c.Name(), "Stopping Weibo channel...")

	if c.cancel != nil {
		c.cancel()
	}

	if c.weiboClient != nil {
		c.weiboClient.Stop()
	}

	c.SetRunning(false)

	return nil
}

func (c *WeiboChannel) Send(ctx context.Context, msg bus.OutboundMessage) ([]string, error) {
	if !c.IsRunning() {
		return nil, channels.ErrNotRunning
	}

	messageIDs, err := c.weiboClient.SendMessageChunked(&weiboTypes.OutboundMessage{
		ToUserID: msg.ChatID,
		Text:     msg.Content,
	}, 2000)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", channels.ErrTemporary, err)
	}

	return messageIDs, nil
}
