package commands

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func reloadCommand() Definition {
	return Definition{
		Name:        "reload",
		Description: i18n.T("commands_reload_description"),
		Usage:       i18n.T("commands_reload_usage"),
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			if rt == nil || rt.ReloadConfig == nil {
				return req.Reply(unavailableMsg())
			}
			if err := rt.ReloadConfig(); err != nil {
				return req.Reply(i18n.Tf("commands_reload_failed", map[string]any{"Error": err.Error()}))
			}
			return req.Reply(i18n.T("commands_reload_success"))
		},
	}
}
