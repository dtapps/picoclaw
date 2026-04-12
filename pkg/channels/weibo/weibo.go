package weibo

import (
	"context"
	"fmt"

	weibo "github.com/dtapps/weibo-go"

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

func NewWeiboChannel(cfg config.WeiboConfig, messageBus *bus.MessageBus) (*WeiboChannel, error) {
	if cfg.AppID == "" || cfg.AppSecret.String() == "" {
		return nil, fmt.Errorf("weibo app_id and app_secret are required")
	}

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

	c.weiboClient = weibo.NewClientWithAppCredentials(c.clientID, c.clientSecret)

	err := c.weiboClient.Connect(&weibo.ConnectOptions{
		OnMessage: func(msg *weibo.InboundMessage) {
			senderID := msg.Payload.FromUserId

			sender := bus.SenderInfo{
				Platform:    "weibo",
				PlatformID:  senderID,
				CanonicalID: identity.BuildCanonicalID("weibo", msg.Payload.FromUserId),
				DisplayName: senderID,
			}

			peer := bus.Peer{
				Kind: "direct",
				ID:   senderID,
			}

			messageID := msg.Payload.MessageId

			content := *msg.Payload.Text

			metadata := map[string]string{
				"from_user_id": senderID,
			}

			c.HandleMessage(
				c.ctx,
				peer,
				messageID,
				senderID,
				senderID,
				content,
				nil,
				metadata,
				sender,
			)
		},
		OnOpen: func() {
			logger.InfoC("weibo", "Connected to Weibo service")
			c.SetRunning(true)
		},
	})
	if err != nil {
		return fmt.Errorf("weibo connect failed: %w", err)
	}

	return nil
}

func (c *WeiboChannel) Stop(ctx context.Context) error {
	logger.InfoC("weibo", "Stopping Weibo channel...")

	if c.cancel != nil {
		c.cancel()
	}

	if c.weiboClient != nil {
		c.weiboClient.Close()
	}

	c.SetRunning(false)

	return nil
}

func (c *WeiboChannel) Send(ctx context.Context, msg bus.OutboundMessage) ([]string, error) {
	if !c.IsRunning() {
		return nil, channels.ErrNotRunning
	}

	result, err := c.weiboClient.Send(msg.ChatID, msg.Content)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", channels.ErrTemporary, err)
	}

	return []string{result.MessageId}, nil
}
