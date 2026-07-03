package commands

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func showCommand() Definition {
	return Definition{
		Name:        "show",
		Description: i18n.T("commands_show_description"),
		SubCommands: []SubCommand{
			{
				Name:        "model",
				Description: i18n.T("commands_show_model_description"),
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.GetModelInfo == nil {
						return req.Reply(unavailableMsg())
					}
					name, provider := rt.GetModelInfo()
					return req.Reply(i18n.Tf("commands_show_model_response", map[string]any{
						"Name":     name,
						"Provider": provider,
					}))
				},
			},
			{
				Name:        "channel",
				Description: i18n.T("commands_show_channel_description"),
				Handler: func(_ context.Context, req Request, _ *Runtime) error {
					return req.Reply(i18n.Tf("commands_show_channel_response", map[string]any{
						"Channel": req.Channel,
					}))
				},
			},
			{
				Name:        "agents",
				Description: "Registered agents",
				Handler:     agentsHandler(),
			},
			{
				Name:        "mcp",
				Description: "Active tools for an MCP server",
				ArgsUsage:   "<server>",
				Handler:     showMCPToolsHandler(),
			},
		},
	}
}
