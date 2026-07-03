package commands

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func btwCommand() Definition {
	return Definition{
		Name:        "btw",
		Description: i18n.T("commands_btw_description"),
		Usage:       i18n.T("commands_btw_usage"),
		Handler: func(ctx context.Context, req Request, rt *Runtime) error {
			if rt == nil || rt.AskSideQuestion == nil {
				return req.Reply(unavailableMsg())
			}

			question := sideQuestionText(req.Text)
			if question == "" {
				return req.Reply(i18n.T("commands_btw_usage_example"))
			}

			answer, err := rt.AskSideQuestion(ctx, question)
			if err != nil {
				return req.Reply(err.Error())
			}
			if strings.TrimSpace(answer) == "" {
				return req.Reply(i18n.T("commands_btw_empty_answer"))
			}

			return req.Reply(answer)
		},
	}
}

func sideQuestionText(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return ""
	}
	if !strings.HasPrefix(input, parts[0]) {
		return ""
	}
	return strings.TrimSpace(input[len(parts[0]):])
}
