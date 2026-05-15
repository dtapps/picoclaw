package commands

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func stopCommand() Definition {
	return Definition{
		Name:        "stop",
		Description: i18n.T("commands_stop_description"),
		Usage:       i18n.T("commands_stop_usage"),
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			if rt == nil || rt.StopActiveTurn == nil {
				return req.Reply(unavailableMsg())
			}

			result, err := rt.StopActiveTurn()
			if err != nil {
				return req.Reply(i18n.Tf("commands_stop_failed", map[string]any{"Error": err.Error()}))
			}

			return req.Reply(FormatStopReply(result))
		},
	}
}

// FormatStopReply renders a user-facing reply for a stop request.
func FormatStopReply(result StopResult) string {
	if !result.Stopped {
		return i18n.T("commands_stop_no_active")
	}

	taskName := compactStopTaskName(result.TaskName)
	if taskName == "" {
		return i18n.T("commands_stop_success_generic")
	}

	return i18n.Tf("commands_stop_success", map[string]any{"TaskName": taskName})
}

func compactStopTaskName(taskName string) string {
	taskName = strings.Join(strings.Fields(strings.TrimSpace(taskName)), " ")
	if taskName == "" {
		return ""
	}
	if len(taskName) > 80 {
		return taskName[:77] + "..."
	}
	return taskName
}
