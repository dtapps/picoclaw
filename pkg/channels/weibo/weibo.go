package weibo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	weibo "github.com/dtapps/weibo-go"
	weiboConfig "github.com/dtapps/weibo-go/config"
	weiboHttp "github.com/dtapps/weibo-go/http"
	weiboLogger "github.com/dtapps/weibo-go/logger"
	weiboTypes "github.com/dtapps/weibo-go/types"
	weiboWs "github.com/dtapps/weibo-go/ws"
	"github.com/gorilla/websocket"

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

	// 令牌文件路径
	tokensPath string

	ctx    context.Context
	cancel context.CancelFunc

	progress *channels.ToolFeedbackAnimator
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

	// 设置日志级别
	weiboLogger.SetLevelByName(logLevel)

	base := channels.NewBaseChannel(
		config.ChannelWeibo,
		cfg,
		messageBus,
		bc.AllowFrom,
		channels.WithReasoningChannelID(bc.ReasoningChannelID),
	)

	ch := &WeiboChannel{
		BaseChannel: base,
		bc:          bc,
		config:      cfg,
		tokensPath:  buildWeiboTokensPath(cfg),
	}
	ch.progress = channels.NewToolFeedbackAnimator(ch.EditMessage)
	return ch, nil
}

func (c *WeiboChannel) Name() string { return config.ChannelWeibo }

// applyWeiboProxy 根据配置应用代理设置
func (c *WeiboChannel) applyWeiboProxy() error {
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
		weiboHttp.SetDefaultHTTPClient(httpClient)
		// 设置 WebSocket Dialer 代理
		dialer := &websocket.Dialer{
			Proxy:            http.ProxyURL(proxyURL),
			HandshakeTimeout: 45 * time.Second,
		}
		weiboWs.SetDefaultDialer(dialer)
		logger.InfoCF(c.Name(), "微博频道使用配置的代理", map[string]any{
			"proxy": c.config.Proxy,
		})
	}
	return nil
}

func (c *WeiboChannel) Start(ctx context.Context) error {
	logger.InfoC(c.Name(), "微博频道启动...")

	c.ctx, c.cancel = context.WithCancel(ctx)

	var err error

	// 应用代理设置
	if err = c.applyWeiboProxy(); err != nil {
		return err
	}

	// 创建配置
	defaultCfg := weiboConfig.DefaultConfig()
	defaultCfg.AppID = c.config.AppID
	defaultCfg.AppSecret = c.config.AppSecret.String()

	// 设置 Token 回调
	onToken := func(data *weiboTypes.TokenCallbackData) {
		logger.InfoCF(c.Name(), "微博 token 回调", map[string]any{
			"status":     data.Status,
			"app_id":     data.AppID,
			"expires_in": data.ExpiresIn,
		})
		if data.Status == "success" {
			// 保存 token 到文件
			if saveErr := saveWeiboToken(c.tokensPath, data.AppID, data.Token, data.ExpiresIn); saveErr != nil {
				logger.ErrorCF(c.Name(), "保存微博 token 失败", map[string]any{
					"error": saveErr.Error(),
					"path":  c.tokensPath,
				})
			} else {
				logger.InfoCF(c.Name(), "微博 token 保存成功", map[string]any{
					"path":       c.tokensPath,
					"expires_in": data.ExpiresIn,
					"app_id":     data.AppID,
				})
			}
		} else {
			logger.ErrorCF(c.Name(), "微博 token 回调错误", map[string]any{
				"status": data.Status,
				"error":  data.Error,
			})
		}
	}

	// 创建客户端
	c.weiboClient, err = weibo.NewClient("default", &weiboTypes.Config{
		Weibo: defaultCfg,
	}, weibo.WithTokenCallback(onToken))
	if err != nil {
		return fmt.Errorf("weibo new client failed: %w", err)
	}

	// 设置连接成功回调
	c.weiboClient.OnConnected(func() {
		logger.InfoC(c.Name(), "微博频道已连接...")
		c.SetRunning(true)
	})

	// 设置断开连接回调
	c.weiboClient.OnDisconnected(func() {
		logger.InfoC(c.Name(), "微博频道已断开...")
		c.SetRunning(false)
	})

	// 设置错误回调
	c.weiboClient.OnError(func(err error) {
		logger.ErrorCF(c.Name(), "微博频道错误", map[string]any{
			"error": err.Error(),
		})
		c.SetRunning(false)
	})

	// 设置消息处理回调
	c.weiboClient.OnMessage(func(msg *weiboTypes.InboundMessage) {
		if msg == nil {
			logger.ErrorCF(c.Name(), "微博频道错误", map[string]any{
				"error": "消息为 nil",
			})
			return
		}

		var content strings.Builder
		for _, segment := range msg.Content {
			content.WriteString(strings.TrimSpace(segment.Text))
		}
		if content.String() == "" {
			logger.ErrorCF(c.Name(), "微博频道错误", map[string]any{
				"error": "消息为空",
			})
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
			logger.ErrorCF(c.Name(), "微博频道错误", map[string]any{
				"error": "发送者不在白名单中",
			})
			return
		}

		// 构建标准化上下文
		inboundCtx := bus.InboundContext{
			Channel:          c.Name(),            // 来源渠道
			Account:          msg.AppID,           // 机器人账号
			ChatID:           sender.PlatformID,   // 会话 ID / 用户 ID
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
			content.String(),
			mediaRefs,
			inboundCtx,
			sender,
		)
	})

	return nil
}

func (c *WeiboChannel) Stop(ctx context.Context) error {
	logger.InfoC(c.Name(), "正在停止微博频道...")

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

	// 分片发送消息
	messageIDs, err := c.weiboClient.SendMessageChunked(&weiboTypes.OutboundMessage{
		ToUserID: msg.ChatID,
		Text:     msg.Content,
	}, 2000)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", channels.ErrTemporary, err)
	}

	return messageIDs, nil
}

// EditMessage implements channels.MessageEditor.
// Note: Weibo API does not support editing messages, so this just logs and returns nil.
func (c *WeiboChannel) EditMessage(ctx context.Context, chatID string, messageID string, content string) error {
	logger.DebugCF(c.Name(), "EditMessage 不支持（微博 API 无法编辑消息）", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
	})
	return nil
}
