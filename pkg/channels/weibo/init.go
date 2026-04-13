package weibo

import (
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

func init() {
	channels.RegisterFactory("weibo", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		return NewWeiboChannel(cfg.Channels.Weibo, b)
	})
}
