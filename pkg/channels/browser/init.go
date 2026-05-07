package browser

import (
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

func init() {
	channels.RegisterFactory(
		config.ChannelBrowser,
		func(channelName, channelType string, cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
			bc := cfg.Channels[channelName]
			decoded, err := bc.GetDecoded()
			if err != nil {
				return nil, err
			}
			c, ok := decoded.(*config.BrowserSettings)
			if !ok {
				return nil, channels.ErrSendFailed
			}
			ch, err := NewBrowserChannel(bc, c, b)
			if err != nil {
				return nil, err
			}
			if channelName != config.ChannelBrowser {
				ch.SetName(channelName)
			}
			return ch, nil
		},
	)
}
