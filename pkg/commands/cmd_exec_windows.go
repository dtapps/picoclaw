//go:build windows

package commands

import (
	"os/exec"
)

func prepareCommandForTermination(cmd *exec.Cmd) {
	// On Windows, process group management is handled differently
	// The default behavior is sufficient for basic command execution
}
