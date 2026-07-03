package commands

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func startCommand() Definition {
	return Definition{
		Name:        "start",
		Description: i18n.T("commands_start_description"),
		Usage:       i18n.T("commands_start_usage"),
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			return req.Reply(i18n.T("commands_start_response"))
		},
	}
}
