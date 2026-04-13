package yuanbao

import (
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

func init() {
	channels.RegisterFactory("yuanbao", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		return NewYuanbaoChannel(cfg.Channels.Yuanbao, b, cfg.Gateway.LogLevel)
	})
}
