package commands

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func clearCommand() Definition {
	return Definition{
		Name:        "clear",
		Description: i18n.T("commands_clear_description"),
		Usage:       i18n.T("commands_clear_usage"),
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			if rt == nil || rt.ClearHistory == nil {
				return req.Reply(unavailableMsg())
			}
			if err := rt.ClearHistory(); err != nil {
				return req.Reply(i18n.Tf("commands_clear_failed", map[string]any{"Error": err.Error()}))
			}
			return req.Reply(i18n.T("commands_clear_success"))
		},
	}
}
