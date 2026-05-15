package commands

import "github.com/sipeed/picoclaw/pkg/i18n"

func useCommand() Definition {
	return Definition{
		Name:        "use",
		Description: i18n.T("commands_use_description"),
		Usage:       i18n.T("commands_use_usage"),
	}
}
