package yuanbao

import (
	"context"
	"fmt"
	"strings"
	"sync"

	yuanbao "github.com/dtapps/yuanbao-go"
	yuanbaoLogger "github.com/dtapps/yuanbao-go/logger"
	yuanbaoTypes "github.com/dtapps/yuanbao-go/types"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/identity"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type YuanbaoChannel struct {
	*channels.BaseChannel
	bc     *config.Channel
	config *config.YuanbaoSettings

	yuanbaoClient *yuanbao.Client
	// Chat routing: track whether a chatID is group or direct.
	chatType sync.Map // chatID → "group" | "direct"

	ctx    context.Context
	cancel context.CancelFunc
}

func NewYuanbaoChannel(
	bc *config.Channel,
	cfg *config.YuanbaoSettings,
	messageBus *bus.MessageBus,
	logLevel string,
) (*YuanbaoChannel, error) {
	if cfg.AppID == "" || cfg.AppSecret.String() == "" {
		return nil, fmt.Errorf("yuanbao app_id and app_secret are required")
	}

	yuanbaoLogger.SetLevelByName(logLevel)

	base := channels.NewBaseChannel(
		config.ChannelYuanbao,
		cfg,
		messageBus,
		bc.AllowFrom.FilterEmpty(),
		channels.WithGroupTrigger(bc.GroupTrigger),
		channels.WithReasoningChannelID(bc.ReasoningChannelID),
	)

	return &YuanbaoChannel{
		BaseChannel: base,
		bc:          bc,
		config:      cfg,
		chatType:    sync.Map{},
	}, nil
}

func (c *YuanbaoChannel) Start(ctx context.Context) error {
	logger.InfoC(config.ChannelYuanbao, "Yuanbao channel started...")

	c.ctx, c.cancel = context.WithCancel(ctx)

	var err error
	c.yuanbaoClient, err = yuanbao.NewClient("default", &yuanbaoTypes.Config{
		Yuanbao: &yuanbaoTypes.YuanbaoConfig{
			AppKey:         c.config.AppID,
			AppSecret:      c.config.AppSecret.String(),
			RequireMention: &c.bc.GroupTrigger.MentionOnly,
		},
	})
	if err != nil {
		return fmt.Errorf("yuanbao new client failed: %w", err)
	}

	c.yuanbaoClient.OnConnected(func() {
		logger.InfoC(config.ChannelYuanbao, "Yuanbao channel connected...")
		c.SetRunning(true)
	})

	c.yuanbaoClient.OnDisconnected(func() {
		logger.InfoC(config.ChannelYuanbao, "Yuanbao channel disconnected...")
		c.SetRunning(false)
	})

	c.yuanbaoClient.OnMessage(func(msg *yuanbaoTypes.InboundMessage, chatType string) {
		if msg == nil {
			return
		}

		// 提取文本内容
		var content string
		for _, elem := range msg.MsgBody {
			if elem.MsgType == "TIMTextElem" {
				content = strings.TrimSpace(elem.MsgContent.Text)
				break
			}
		}

		if content == "" {
			return // 忽略空消息
		}

		// 聊天类型
		var icChatType = "direct"
		if chatType == "group" {
			icChatType = "group"
		}

		// 构建发送者信息
		sender := bus.SenderInfo{
			Platform:    config.ChannelYuanbao,                                             // 平台名称
			PlatformID:  msg.FromAccount,                                                   // 原始 ID
			CanonicalID: identity.BuildCanonicalID(config.ChannelYuanbao, msg.FromAccount), // 规范化 ID
			Username:    msg.SenderNickname,                                                // 用户名
			DisplayName: msg.SenderNickname,                                                // 显示名称
		}
		if chatType == "group" {
			sender.PlatformID = msg.GroupCode
			sender.CanonicalID = identity.BuildCanonicalID(config.ChannelYuanbao, msg.GroupCode)
			sender.DisplayName = msg.GroupName
		}

		// 权限校验
		if !c.IsAllowedSender(sender) {
			return
		}

		// 记录会话类型
		if chatType == "group" {
			c.chatType.Store(sender.PlatformID, "group")
		} else {
			c.chatType.Store(sender.PlatformID, "direct")
		}

		// 构建标准化上下文
		inboundCtx := bus.InboundContext{
			Channel:          config.ChannelYuanbao, // 来源渠道
			Account:          "",                    // 机器人账号
			ChatID:           sender.PlatformID,     // 会话 ID / 用户 ID
			ChatType:         icChatType,            // 会话类型 direct / group
			TopicID:          "",                    // 话题 ID
			SpaceID:          "",                    // 空间 ID
			SpaceType:        "",                    // 空间类型
			SenderID:         msg.FromAccount,       // 发送者 ID
			MessageID:        msg.MsgID,             // 消息 ID
			Mentioned:        msg.IsAtBot,           // 是否被提及其
			ReplyToMessageID: "",                    // 回复消息 ID
			ReplyHandles:     map[string]string{},   // 回复句柄
			Raw:              map[string]string{},   // 原始数据
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

func (c *YuanbaoChannel) Stop(ctx context.Context) error {
	logger.InfoC(config.ChannelYuanbao, "Stopping Yuanbao channel...")

	if c.cancel != nil {
		c.cancel()
	}

	if c.yuanbaoClient != nil {
		c.yuanbaoClient.Stop()
	}

	c.SetRunning(false)

	return nil
}

func (c *YuanbaoChannel) Send(ctx context.Context, msg bus.OutboundMessage) ([]string, error) {
	if !c.IsRunning() {
		return nil, channels.ErrNotRunning
	}

	chatKind := c.getChatKind(msg.ChatID)
	if chatKind == "direct" {
		err := c.yuanbaoClient.SendMessage(msg.ChatID, msg.Content)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", channels.ErrTemporary, err)
		}
	} else if chatKind == "group" {
		err := c.yuanbaoClient.SendGroupMessage(msg.ChatID, msg.Content)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", channels.ErrTemporary, err)
		}
	} else {
		return nil, fmt.Errorf("unknown chat type: %s", chatKind)
	}

	return []string{}, nil
}

func (c *YuanbaoChannel) getChatKind(chatID string) string {
	if v, ok := c.chatType.Load(chatID); ok {
		if k, ok := v.(string); ok {
			return k
		}
	}
	logger.DebugCF(config.ChannelYuanbao, "Unknown chat type for chatID, defaulting to group", map[string]any{
		"chat_id": chatID,
	})
	return ""
}
