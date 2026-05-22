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
	"github.com/sipeed/picoclaw/pkg/commands"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/identity"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type YuanbaoChannel struct {
	*channels.BaseChannel
	bc     *config.Channel
	config *config.YuanbaoSettings

	yuanbaoClient *yuanbao.Client

	// 聊天路由：跟踪聊天ID是群组还是直接。
	chatType sync.Map // chatID → "group" | "direct"

	// 令牌文件路径
	tokensPath string

	// group chatID 文件路径
	chatIDsPath string

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

	// 设置日志级别
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
		tokensPath:  buildYuanbaoTokensPath(cfg),
		chatIDsPath: buildYuanbaoChatIDsPath(cfg),
	}
	ch.progress = channels.NewToolFeedbackAnimator(ch.EditMessage)

	// 加载已保存的 group chatID
	if err := ch.loadGroupChatIDs(); err != nil {
		logger.WarnCF(ch.Name(), "加载 group chatID 失败", map[string]any{
			"error": err.Error(),
		})
	}

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
		logger.InfoCF(c.Name(), "元宝频道使用配置的代理", map[string]any{
			"proxy": c.config.Proxy,
		})
	}
	return nil
}

func (c *YuanbaoChannel) Start(ctx context.Context) error {
	logger.InfoC(c.Name(), "元宝频道启动...")

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

	// 设置 Token 回调
	onToken := func(data *yuanbaoTypes.TokenCallbackData) {
		logger.InfoCF(c.Name(), "元宝 token 回调", map[string]any{
			"status":     data.Status,
			"app_id":     data.AppID,
			"expires_in": data.ExpiresIn,
		})
		if data.Status == "success" {
			// 保存 token 到文件
			if saveErr := saveYuanbaoToken(c.tokensPath, data.AppID, data.Token, data.ExpiresIn); saveErr != nil {
				logger.ErrorCF(c.Name(), "保存元宝 token 失败", map[string]any{
					"error": saveErr.Error(),
					"path":  c.tokensPath,
				})
			} else {
				logger.InfoCF(c.Name(), "元宝 token 保存成功", map[string]any{
					"path":       c.tokensPath,
					"expires_in": data.ExpiresIn,
					"app_id":     data.AppID,
				})
			}
		} else {
			logger.ErrorCF(c.Name(), "元宝 token 回调错误", map[string]any{
				"status": data.Status,
				"error":  data.Error,
			})
		}
	}

	// 创建客户端
	c.yuanbaoClient, err = yuanbao.NewClient("default", &yuanbaoTypes.Config{
		Yuanbao: defaultCfg,
	}, yuanbao.WithTokenCallback(onToken))
	if err != nil {
		return fmt.Errorf("yuanbao new client failed: %w", err)
	}

	// 设置连接成功回调
	c.yuanbaoClient.OnConnected(func() {
		logger.InfoC(c.Name(), "元宝频道已连接...")
		c.SetRunning(true)

		// 同步命令到服务器
		c.syncCommands()
	})

	// 设置断开连接回调
	c.yuanbaoClient.OnDisconnected(func() {
		logger.InfoC(c.Name(), "元宝频道已断开...")
		c.SetRunning(false)
	})

	// 设置错误回调
	c.yuanbaoClient.OnError(func(err error) {
		logger.ErrorCF(c.Name(), "元宝频道错误", map[string]any{
			"error": err.Error(),
		})
		c.SetRunning(false)
	})

	// 设置消息处理回调
	c.yuanbaoClient.OnMessage(func(msg *yuanbaoTypes.InboundMessage, chatType yuanbaoTypes.ChatType) {
		if msg == nil {
			logger.ErrorCF(c.Name(), "元宝频道错误", map[string]any{
				"error": "消息为 nil",
			})
			return
		}

		var content strings.Builder
		for _, segment := range msg.Content {
			content.WriteString(strings.TrimSpace(segment.Text))
		}
		if content.String() == "" {
			logger.ErrorCF(c.Name(), "元宝频道错误", map[string]any{
				"error": "消息为空",
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
			logger.ErrorCF(c.Name(), "元宝频道错误", map[string]any{
				"error": "发送者不在白名单中",
			})
			return
		}

		// 记录群组会话类型（仅记录 group，不记录 direct）
		// 这样主动发消息时可通过 chatType 判断是否为群组
		if chatType == "group" {
			c.chatType.Store(sender.PlatformID, "group")
			// 保存到文件，确保重启后仍能识别 group
			c.saveGroupChatID(sender.PlatformID)
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
			content.String(),
			mediaRefs,
			inboundCtx,
			sender,
		)
	})

	return nil
}

func (c *YuanbaoChannel) Stop(ctx context.Context) error {
	logger.InfoC(c.Name(), "正在停止元宝频道...")

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

	// 清理消息内容，确保只包含有效的 UTF-8 字符
	content := strings.ToValidUTF8(msg.Content, "")

	messageIDs := []string{}
	chatKind := c.getChatKind(msg.ChatID)

	if chatKind == "direct" {
		messageID, err := c.yuanbaoClient.SendMessage(&yuanbaoTypes.OutboundC2CMessage{
			ToUserID: msg.ChatID,
			Text:     content,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", channels.ErrTemporary, err)
		}
		messageIDs = append(messageIDs, messageID)
	} else if chatKind == "group" {
		messageID, err := c.yuanbaoClient.SendGroupMessage(&yuanbaoTypes.OutboundGroupMessage{
			ToGroupID: msg.ChatID,
			Text:      content,
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
	// 优先检查 chatType 映射表（从收到的消息中记录的，或从文件加载的）
	if _, ok := c.chatType.Load(chatID); ok {
		return "group"
	}
	return "direct"
}

// EditMessage implements channels.MessageEditor.
// Note: Yuanbao API does not support editing messages, so this just logs and returns nil.
func (c *YuanbaoChannel) EditMessage(ctx context.Context, chatID string, messageID string, content string) error {
	logger.DebugCF(c.Name(), "EditMessage 不支持（元宝 API 无法编辑消息）", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
	})
	return nil
}

// commandDefs 存储命令定义，用于同步到元宝服务器
var commandDefs []commands.Definition

// RegisterCommands implements channels.CommandRegistrarCapable.
// 存储命令定义，在连接成功后同步到元宝服务器。
func (c *YuanbaoChannel) RegisterCommands(ctx context.Context, defs []commands.Definition) error {
	commandDefs = defs
	// 如果已经连接，立即同步
	if c.IsRunning() && c.yuanbaoClient != nil {
		c.syncCommands()
	}
	return nil
}

// syncCommands 同步命令到元宝服务器
func (c *YuanbaoChannel) syncCommands() {
	if c.yuanbaoClient == nil {
		return
	}

	// 使用系统真实的命令定义
	defs := commands.BuiltinDefinitions()

	// 将命令转换为元宝格式
	var botCommands []yuanbaoTypes.BotCommand
	for _, def := range defs {
		if def.Name == "" || def.Description == "" {
			continue
		}
		cmd := yuanbaoTypes.BotCommand{
			Name:        "/" + def.Name,
			Description: def.Description,
		}
		botCommands = append(botCommands, cmd)
	}

	// 将注册的插件命令转换为元宝命令格式
	var pluginCommands []yuanbaoTypes.BotCommand
	for _, def := range commandDefs {
		if def.Name == "" || def.Description == "" {
			continue
		}
		cmd := yuanbaoTypes.BotCommand{
			Name:        "/" + def.Name,
			Description: def.Description,
		}
		pluginCommands = append(pluginCommands, cmd)
	}

	// 同步命令到服务器
	if err := c.yuanbaoClient.SyncCommands(botCommands, pluginCommands); err != nil {
		logger.ErrorCF(c.Name(), "同步命令到元宝服务器失败", map[string]any{
			"error": err.Error(),
		})
	} else {
		logger.InfoCF(c.Name(), "同步命令到元宝服务器成功", map[string]any{
			"bot_commands":    len(botCommands),
			"plugin_commands": len(pluginCommands),
		})
	}
}

// loadGroupChatIDs 从文件加载 group chatID 到内存
func (c *YuanbaoChannel) loadGroupChatIDs() error {
	chatIDs, err := loadYuanbaoGroupChatIDs(c.chatIDsPath)
	if err != nil {
		return err
	}
	for _, chatID := range chatIDs {
		c.chatType.Store(chatID, "group")
	}
	if len(chatIDs) > 0 {
		logger.InfoCF(c.Name(), "已加载 group chatID", map[string]any{
			"count": len(chatIDs),
		})
	}
	return nil
}

// saveGroupChatID 保存 group chatID 到文件
func (c *YuanbaoChannel) saveGroupChatID(chatID string) {
	if err := saveYuanbaoGroupChatID(c.chatIDsPath, chatID); err != nil {
		logger.WarnCF(c.Name(), "保存 group chatID 失败", map[string]any{
			"chat_id": chatID,
			"error":   err.Error(),
		})
	}
}
