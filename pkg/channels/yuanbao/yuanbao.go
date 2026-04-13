package yuanbao

import (
	"context"
	"fmt"
	"sync"

	yuanbao "github.com/dtapps/yuanbao-go"
	yuanbaoConfig "github.com/dtapps/yuanbao-go/config"
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
	config config.YuanbaoConfig

	clientID             string
	clientSecret         string
	clientRequireMention bool
	yuanbaoClient        *yuanbao.Client

	// Chat routing: track whether a chatID is group or direct.
	chatType sync.Map // chatID → "group" | "direct"

	ctx    context.Context
	cancel context.CancelFunc
}

func NewYuanbaoChannel(cfg config.YuanbaoConfig, messageBus *bus.MessageBus, logLevel string) (*YuanbaoChannel, error) {
	if cfg.AppID == "" || cfg.AppSecret.String() == "" {
		return nil, fmt.Errorf("yuanbao app_id and app_secret are required")
	}

	yuanbaoLogger.SetLevelByName(logLevel)

	base := channels.NewBaseChannel(
		"yuanbao",
		cfg,
		messageBus,
		cfg.AllowFrom.FilterEmpty(),
		channels.WithGroupTrigger(cfg.GroupTrigger),
		channels.WithReasoningChannelID(cfg.ReasoningChannelID),
	)

	return &YuanbaoChannel{
		BaseChannel:          base,
		config:               cfg,
		clientID:             cfg.AppID,
		clientSecret:         cfg.AppSecret.String(),
		clientRequireMention: cfg.GroupTrigger.MentionOnly,
		chatType:             sync.Map{},
	}, nil
}

func (c *YuanbaoChannel) Name() string { return "yuanbao" }

func (c *YuanbaoChannel) Start(ctx context.Context) error {
	logger.InfoC("yuanbao", "Yuanbao channel started...")

	c.ctx, c.cancel = context.WithCancel(ctx)

	var err error
	c.yuanbaoClient, err = yuanbao.NewClient("default", &yuanbaoConfig.Config{
		Yuanbao: &yuanbaoConfig.YuanbaoConfig{
			AppKey:         c.clientID,
			AppSecret:      c.clientSecret,
			RequireMention: &c.clientRequireMention,
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
		var content string
		for _, elem := range msg.MsgBody {
			if elem.MsgType == "TIMTextElem" {
				content = elem.MsgContent.Text
				break
			}
		}

		sender := bus.SenderInfo{}

		peer := bus.Peer{}

		if chatType == "group" {
			peer = bus.Peer{
				Kind: "group",
				ID:   msg.GroupCode,
			}

			sender = bus.SenderInfo{
				Platform:    "yuanbao",
				PlatformID:  msg.GroupCode,
				CanonicalID: identity.BuildCanonicalID("yuanbao", msg.GroupCode),
				Username:    msg.SenderNickname,
				DisplayName: msg.GroupName,
			}

			c.chatType.Store(peer.ID, "group")
		} else {
			peer = bus.Peer{
				Kind: "direct",
				ID:   msg.FromAccount,
			}

			sender = bus.SenderInfo{
				Platform:    "yuanbao",
				PlatformID:  msg.FromAccount,
				CanonicalID: identity.BuildCanonicalID("yuanbao", msg.FromAccount),
				Username:    msg.SenderNickname,
				DisplayName: msg.SenderNickname,
			}

			c.chatType.Store(peer.ID, "direct")
		}

		mediaRefs := []string{}

		metadata := map[string]string{}

		c.HandleMessage(
			c.ctx,
			peer,
			msg.MsgID,
			msg.FromAccount,
			peer.ID,
			content,
			mediaRefs,
			metadata,
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
