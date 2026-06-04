package commands

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func checkCommand() Definition {
	return Definition{
		Name:        "check",
		Description: i18n.T("commands_check_description"),
		SubCommands: []SubCommand{
			{
				Name:        "channel",
				Description: i18n.T("commands_check_channel_description"),
				ArgsUsage:   "<name>",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.SwitchChannel == nil {
						return req.Reply(unavailableMsg())
					}
					value := nthToken(req.Text, 2)
					if value == "" {
						return req.Reply(i18n.T("commands_check_channel_usage"))
					}
					if err := rt.SwitchChannel(value); err != nil {
						return req.Reply(err.Error())
					}
					return req.Reply(i18n.Tf("commands_check_channel_success", map[string]any{"Name": value}))
				},
			},
		},
	}
}
