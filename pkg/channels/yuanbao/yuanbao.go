package yuanbao

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	yuanbao "github.com/dtapps/yuanbao-go"
	yuanbaoConfig "github.com/dtapps/yuanbao-go/config"
	yuanbaoHttp "github.com/dtapps/yuanbao-go/http"
	yuanbaoLogger "github.com/dtapps/yuanbao-go/logger"
	yuanbaoTypes "github.com/dtapps/yuanbao-go/types"
	yuanbaoWs "github.com/dtapps/yuanbao-go/ws"
	"github.com/gorilla/websocket"

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

	progress *channels.ToolFeedbackAnimator
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

	ch := &YuanbaoChannel{
		BaseChannel: base,
		bc:          bc,
		config:      cfg,
		chatType:    sync.Map{},
	}
	ch.progress = channels.NewToolFeedbackAnimator(ch.EditMessage)
	return ch, nil
}

func (c *YuanbaoChannel) Name() string { return config.ChannelYuanbao }

// applyYuanbaoProxy 根据配置应用代理设置
func (c *YuanbaoChannel) applyYuanbaoProxy() error {
	if c.config.Proxy != "" {
		proxyURL, parseErr := url.Parse(c.config.Proxy)
		if parseErr != nil {
			return fmt.Errorf("invalid proxy URL %q: %w", c.config.Proxy, parseErr)
		}
		// 设置 HTTP 客户端代理
		httpClient := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}
		yuanbaoHttp.SetDefaultHTTPClient(httpClient)
		// 设置 WebSocket Dialer 代理
		dialer := &websocket.Dialer{
			Proxy:            http.ProxyURL(proxyURL),
			HandshakeTimeout: 45 * time.Second,
		}
		yuanbaoWs.SetDefaultDialer(dialer)
		logger.InfoCF(c.Name(), "Yuanbao channel using configured proxy", map[string]any{
			"proxy": c.config.Proxy,
		})
	}
	return nil
}

func (c *YuanbaoChannel) Start(ctx context.Context) error {
	logger.InfoC(c.Name(), "Yuanbao channel started...")

	c.ctx, c.cancel = context.WithCancel(ctx)

	var err error

	// 应用代理设置
	if err = c.applyYuanbaoProxy(); err != nil {
		return err
	}

	// 创建配置
	defaultCfg := yuanbaoConfig.DefaultConfig()
	defaultCfg.AppID = c.config.AppID
	defaultCfg.AppSecret = c.config.AppSecret.String()
	defaultCfg.RequireMention = &c.bc.GroupTrigger.MentionOnly

	// 创建客户端
	c.yuanbaoClient, err = yuanbao.NewClient("default", &yuanbaoTypes.Config{
		Yuanbao: defaultCfg,
	})
	if err != nil {
		return fmt.Errorf("yuanbao new client failed: %w", err)
	}

	// 设置连接成功回调
	c.yuanbaoClient.OnConnected(func() {
		logger.InfoC(c.Name(), "Yuanbao channel connected...")
		c.SetRunning(true)
	})

	// 设置断开连接回调
	c.yuanbaoClient.OnDisconnected(func() {
		logger.InfoC(c.Name(), "Yuanbao channel disconnected...")
		c.SetRunning(false)
	})

	// 设置错误回调
	c.yuanbaoClient.OnError(func(err error) {
		logger.ErrorCF(c.Name(), "Yuanbao channel error", map[string]any{
			"error": err.Error(),
		})
		c.SetRunning(false)
	})

	// 设置消息处理回调
	c.yuanbaoClient.OnMessage(func(msg *yuanbaoTypes.InboundMessage, chatType yuanbaoTypes.ChatType) {
		if msg == nil {
			logger.ErrorCF(c.Name(), "Yuanbao channel error", map[string]any{
				"error": "message is nil",
			})
			return
		}

		content := ""
		for _, segment := range msg.Content {
			content += strings.TrimSpace(segment.Text)
		}
		if content == "" {
			logger.ErrorCF(c.Name(), "Yuanbao channel error", map[string]any{
				"error": "message is empty",
			})
			return // 忽略空消息
		}

		// 聊天类型
		icChatType := "direct"
		if chatType == "group" {
			icChatType = "group"
		}

		// 构建发送者信息
		sender := bus.SenderInfo{
			Platform:    c.Name(),                                          // 平台名称
			PlatformID:  msg.SenderID,                                      // 原始 ID
			CanonicalID: identity.BuildCanonicalID(c.Name(), msg.SenderID), // 规范化 ID
			Username:    msg.SenderName,                                    // 用户名
			DisplayName: msg.SenderName,                                    // 显示名称
		}
		if chatType == "group" {
			sender.PlatformID = msg.GroupCode
			sender.CanonicalID = identity.BuildCanonicalID(c.Name(), msg.GroupCode)
			sender.DisplayName = msg.GroupName
		}

		// 权限校验
		if !c.IsAllowedSender(sender) {
			logger.ErrorCF(c.Name(), "Yuanbao channel error", map[string]any{
				"error": "sender not allowed to send",
			})
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
			Channel:          c.Name(),            // 来源渠道
			Account:          msg.BotID,           // 机器人账号
			ChatID:           sender.PlatformID,   // 会话 ID / 用户 ID
			ChatType:         icChatType,          // 会话类型 direct / group
			TopicID:          "",                  // 话题 ID
			SpaceID:          "",                  // 空间 ID
			SpaceType:        "",                  // 空间类型
			SenderID:         msg.SenderID,        // 发送者 ID
			MessageID:        msg.MessageID,       // 消息 ID
			ReplyToMessageID: "",                  // 回复消息 ID
			ReplyHandles:     map[string]string{}, // 回复句柄
			Raw: map[string]string{
				"raw_data": string(msg.RawMessage),
			}, // 原始数据
		}

		// 是否被提及其
		if len(msg.AtList) > 0 {
			inboundCtx.Mentioned = true
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
	logger.InfoC(c.Name(), "Stopping Yuanbao channel...")

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

	messageIDs := []string{}
	chatKind := c.getChatKind(msg.ChatID)
	if chatKind == "direct" {
		messageID, err := c.yuanbaoClient.SendMessage(&yuanbaoTypes.OutboundC2CMessage{
			ToUserID: msg.ChatID,
			Text:     msg.Content,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", channels.ErrTemporary, err)
		}
		messageIDs = append(messageIDs, messageID)
	} else if chatKind == "group" {
		messageID, err := c.yuanbaoClient.SendGroupMessage(&yuanbaoTypes.OutboundGroupMessage{
			ToGroupID: msg.ChatID,
			Text:      msg.Content,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", channels.ErrTemporary, err)
		}
		messageIDs = append(messageIDs, messageID)
	} else {
		return nil, fmt.Errorf("unknown chat type: %s", chatKind)
	}

	return messageIDs, nil
}

func (c *YuanbaoChannel) getChatKind(chatID string) string {
	if v, ok := c.chatType.Load(chatID); ok {
		if k, ok := v.(string); ok {
			return k
		}
	}
	logger.DebugCF(c.Name(), "Unknown chat type for chatID, defaulting to group", map[string]any{
		"chat_id": chatID,
	})
	return ""
}

// EditMessage implements channels.MessageEditor.
// Note: Yuanbao API does not support editing messages, so this just logs and returns nil.
func (c *YuanbaoChannel) EditMessage(ctx context.Context, chatID string, messageID string, content string) error {
	logger.DebugCF(c.Name(), "EditMessage called but not supported by Yuanbao API", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
	})
	return nil
}
