package commands

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func switchCommand() Definition {
	return Definition{
		Name:        "switch",
		Description: i18n.T("commands_switch_description"),
		SubCommands: []SubCommand{
			{
				Name:        "model",
				Description: i18n.T("commands_switch_model_description"),
				ArgsUsage:   "to <name>",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.SwitchModel == nil {
						return req.Reply(unavailableMsg())
					}
					// Parse: /switch model to <value>
					value := nthToken(req.Text, 3) // tokens: [/switch, model, to, <value>]
					if nthToken(req.Text, 2) != "to" || value == "" {
						return req.Reply(i18n.T("commands_switch_model_usage"))
					}
					oldModel, err := rt.SwitchModel(value)
					if err != nil {
						return req.Reply(err.Error())
					}
					return req.Reply(i18n.Tf("commands_switch_model_success", map[string]any{
						"OldModel": oldModel,
						"NewModel": value,
					}))
				},
			},
			{
				Name:        "channel",
				Description: i18n.T("commands_switch_channel_description"),
				Handler: func(_ context.Context, req Request, _ *Runtime) error {
					return req.Reply(i18n.T("commands_switch_channel_moved"))
				},
			},
		},
	}
}
