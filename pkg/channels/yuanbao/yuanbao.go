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
		"yuanbao",
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
	logger.InfoC("yuanbao", "Yuanbao channel started...")

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
		logger.InfoC("yuanbao", "Yuanbao channel connected...")
		c.SetRunning(true)
	})

	c.yuanbaoClient.OnDisconnected(func() {
		logger.InfoC("yuanbao", "Yuanbao channel disconnected...")
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

		// 2. 确定聊天类型和 ID
		var ctxChatType string
		var chatID string
		var senderID string
		var displayName string

		senderID = msg.FromAccount

		if chatType == "group" {
			ctxChatType = "group"
			chatID = msg.GroupCode
			displayName = msg.GroupName // 群聊显示群名，或者你可以选择显示 sender 昵称
		} else {
			ctxChatType = "direct"
			chatID = msg.FromAccount // 私聊 ChatID 通常为对方账号
			displayName = msg.SenderNickname
		}

		// 构建发送者信息
		sender := bus.SenderInfo{
			Platform:    "yuanbao",
			PlatformID:  senderID,
			CanonicalID: identity.BuildCanonicalID("yuanbao", senderID),
			Username:    msg.SenderNickname,
			DisplayName: displayName,
		}

		// 权限校验
		if !c.IsAllowedSender(sender) {
			return
		}

		// 构建标准化上下文
		inboundCtx := bus.InboundContext{
			Channel:   "yuanbao",
			ChatID:    chatID,
			ChatType:  ctxChatType,
			SenderID:  senderID,
			MessageID: msg.MsgID,
			Mentioned: false, // 如果需要支持 @机器人，需解析 msg.MsgBody 中的 TIMAtElem
			Raw: map[string]string{
				"platform":      "yuanbao",
				"group_code":    msg.GroupCode,
				"sender_nick":   msg.SenderNickname,
				"chat_type_raw": chatType,
			},
		}

		// 设置回复句柄
		inboundCtx.ReplyHandles = map[string]string{
			"message_id":   msg.MsgID,
			"from_account": msg.FromAccount,
		}
		if chatType == "group" {
			inboundCtx.ReplyHandles["group_code"] = msg.GroupCode
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
	logger.InfoC("yuanbao", "Stopping Yuanbao channel...")

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
	logger.DebugCF("yuanbao", "Unknown chat type for chatID, defaulting to group", map[string]any{
		"chat_id": chatID,
	})
	return ""
}
