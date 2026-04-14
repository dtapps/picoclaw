package weibo

import (
	"context"
	"fmt"
	"strings"

	weibo "github.com/dtapps/weibo-go"
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
		"weibo",
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

func (c *WeiboChannel) Start(ctx context.Context) error {
	logger.InfoC("weibo", "Weibo channel started...")

	c.ctx, c.cancel = context.WithCancel(ctx)

	var err error
	c.weiboClient, err = weibo.NewClient("picoclaw", &weiboTypes.Config{
		Weibo: &weiboTypes.WeiboConfig{
			AppId:     c.config.AppID,
			AppSecret: c.config.AppSecret.String(),
		},
	})
	if err != nil {
		return fmt.Errorf("weibo new client failed: %w", err)
	}

	c.weiboClient.OnConnected(func() {
		logger.InfoC("weibo", "Weibo channel connected...")
		c.SetRunning(true)
	})

	c.weiboClient.OnDisconnected(func() {
		logger.InfoC("weibo", "Weibo channel disconnected...")
		c.SetRunning(false)
	})

	c.weiboClient.OnMessage(func(msg *weiboTypes.WsMessageMsg) {
		if msg == nil {
			return
		}

		content := strings.TrimSpace(msg.Payload.Text)
		if content == "" {
			return // 忽略空消息
		}

		senderID := msg.Payload.FromUserId
		messageID := msg.Payload.MessageId

		// 构建发送者信息
		sender := bus.SenderInfo{
			Platform:    "weibo",
			PlatformID:  senderID,
			CanonicalID: identity.BuildCanonicalID("weibo", senderID),
			Username:    senderID,
			DisplayName: senderID,
		}

		// 权限校验
		if !c.IsAllowedSender(sender) {
			return
		}

		// 构建标准化上下文
		inboundCtx := bus.InboundContext{
			Channel:   "weibo",
			ChatID:    senderID, // 微博私信中，ChatID 通常等同于 SenderID (一对一)
			ChatType:  "direct", // 微博私信默认为单聊
			SenderID:  senderID,
			MessageID: messageID,
			Mentioned: false, // 私信不涉及 @
			Raw: map[string]string{
				"platform": "weibo",
			},
		}

		// 设置回复句柄
		if messageID != "" {
			inboundCtx.ReplyHandles = map[string]string{
				"message_id": messageID,
				"chat_id":    senderID,
			}
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
	logger.InfoC("weibo", "Stopping Weibo channel...")

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

	result, err := c.weiboClient.SendMessageChunked(msg.ChatID, msg.Content, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", channels.ErrTemporary, err)
	}

	messageIDs := []string{}

	for _, item := range result {
		messageIDs = append(messageIDs, item.MessageID)
	}

	return messageIDs, nil
}
