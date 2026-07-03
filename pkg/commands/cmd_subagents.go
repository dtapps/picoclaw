package commands

import (
	"context"
	"fmt"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

// TurnInfo is a mirrored struct from agent.TurnInfo to avoid circular dependencies.
type TurnInfo struct {
	TurnID       string
	ParentTurnID string
	Depth        int
	ChildTurnIDs []string
	IsFinished   bool
}

func subagentsCommand() Definition {
	return Definition{
		Name:        "subagents",
		Description: i18n.T("commands_subagents_description"),
		Handler: func(ctx context.Context, req Request, rt *Runtime) error {
			getTurnFn := rt.GetActiveTurn
			if getTurnFn == nil {
				return req.Reply(i18n.T("commands_subagents_not_supported"))
			}

			turnRaw := getTurnFn()
			if turnRaw == nil {
				return req.Reply(i18n.T("commands_subagents_no_active"))
			}

			if treeStr, ok := turnRaw.(string); ok {
				if treeStr == "" {
					return req.Reply(i18n.T("commands_subagents_no_active"))
				}
				return req.Reply(fmt.Sprintf("%s\n```text\n%s\n```", i18n.T("commands_subagents_title"), treeStr))
			}

			return req.Reply(fmt.Sprintf("%s\n```text\n%+v\n```", i18n.T("commands_subagents_title_list"), turnRaw))
		},
	}
}
