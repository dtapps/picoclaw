package commands

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func listCommand() Definition {
	return Definition{
		Name:        "list",
		Description: i18n.T("commands_list_description"),
		SubCommands: []SubCommand{
			{
				Name:        "models",
				Description: i18n.T("commands_list_models_description"),
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.GetModelInfo == nil {
						return req.Reply(unavailableMsg())
					}
					name, provider := rt.GetModelInfo()
					if provider == "" {
						provider = "configured default"
					}
					return req.Reply(i18n.Tf("commands_list_models_response", map[string]any{
						"Name":     name,
						"Provider": provider,
					}))
				},
			},
			{
				Name:        "channels",
				Description: i18n.T("commands_list_channels_description"),
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.GetEnabledChannels == nil {
						return req.Reply(unavailableMsg())
					}
					enabled := rt.GetEnabledChannels()
					if len(enabled) == 0 {
						return req.Reply(i18n.T("commands_list_channels_none"))
					}
					return req.Reply(i18n.Tf("commands_list_channels_response", map[string]any{
						"Channels": strings.Join(enabled, "\n- "),
					}))
				},
			},
			{
				Name:        "agents",
				Description: "Registered agents",
				Handler:     agentsHandler(),
			},
			{
				Name:        "skills",
				Description: i18n.T("commands_list_skills_description"),
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.ListSkillNames == nil {
						return req.Reply(unavailableMsg())
					}
					names := rt.ListSkillNames()
					if len(names) == 0 {
						return req.Reply(i18n.T("commands_list_skills_none"))
					}
					return req.Reply(i18n.Tf("commands_list_skills_response", map[string]any{
						"Skills": strings.Join(names, "\n- "),
					}))
				},
			},
			{
				Name:        "mcp",
				Description: "Configured MCP servers",
				Handler:     listMCPServersHandler(),
			},
		},
	}
}
