package weibo

import (
	"context"
	"fmt"

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
	config       config.WeiboConfig
	clientID     string
	clientSecret string
	weiboClient  *weibo.Client
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewWeiboChannel(cfg config.WeiboConfig, messageBus *bus.MessageBus, logLevel string) (*WeiboChannel, error) {
	if cfg.AppID == "" || cfg.AppSecret.String() == "" {
		return nil, fmt.Errorf("weibo app_id and app_secret are required")
	}

	weiboLogger.SetLevelByName(logLevel)

	base := channels.NewBaseChannel(
		"weibo",
		cfg,
		messageBus,
		cfg.AllowFrom.FilterEmpty(),
		channels.WithReasoningChannelID(cfg.ReasoningChannelID),
	)

	return &WeiboChannel{
		BaseChannel:  base,
		config:       cfg,
		clientID:     cfg.AppID,
		clientSecret: cfg.AppSecret.String(),
	}, nil
}

func (c *WeiboChannel) Name() string { return "weibo" }

func (c *WeiboChannel) Start(ctx context.Context) error {
	logger.InfoC("weibo", "Weibo channel started...")

	c.ctx, c.cancel = context.WithCancel(ctx)

	var err error
	c.weiboClient, err = weibo.NewClient("picoclaw", &weiboTypes.Config{
		Weibo: &weiboTypes.WeiboConfig{
			AppId:     c.clientID,
			AppSecret: c.clientSecret,
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
		content := msg.Payload.Text

		sender := bus.SenderInfo{
			Platform:    "weibo",
			PlatformID:  msg.Payload.FromUserId,
			CanonicalID: identity.BuildCanonicalID("weibo", msg.Payload.FromUserId),
			Username:    msg.Payload.FromUserId,
			DisplayName: msg.Payload.FromUserId,
		}

		peer := bus.Peer{
			Kind: "direct",
			ID:   msg.Payload.FromUserId,
		}

		mediaRefs := []string{}

		metadata := map[string]string{}

		c.HandleMessage(
			c.ctx,
			peer,
			msg.Payload.MessageId,
			msg.Payload.FromUserId,
			peer.ID,
			content,
			mediaRefs,
			metadata,
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
