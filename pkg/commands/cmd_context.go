package commands

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/i18n"
)

func contextCommand() Definition {
	return Definition{
		Name:        "context",
		Description: i18n.T("commands_context_description"),
		Usage:       i18n.T("commands_context_usage"),
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			if rt == nil || rt.GetContextStats == nil {
				return req.Reply(unavailableMsg())
			}
			stats := rt.GetContextStats()
			if stats == nil {
				return req.Reply(i18n.T("commands_context_no_session"))
			}
			return req.Reply(formatContextStats(stats))
		},
	}
}

func formatContextStats(s *ContextStats) string {
	remaining := s.CompressAtTokens - s.UsedTokens
	if remaining < 0 {
		remaining = 0
	}
	usedWindowPercent := s.UsedTokens * 100 / max(s.TotalTokens, 1)
	return i18n.Tf("commands_context_stats", map[string]any{
		"MessageCount":       s.MessageCount,
		"UsedTokens":         s.UsedTokens,
		"TotalTokens":        s.TotalTokens,
		"UsedPercent":        usedWindowPercent,
		"CompressAtTokens":   s.CompressAtTokens,
		"CompressionPercent": s.UsedPercent,
		"Remaining":          remaining,
	})
}
