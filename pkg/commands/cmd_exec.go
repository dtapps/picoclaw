package commands

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/i18n"
	"github.com/sipeed/picoclaw/pkg/isolation"
)

// ExecResult represents the result of a command execution
type ExecResult struct {
	Output   string
	ExitCode int
	Error    error
	Duration time.Duration
}

// ExecSessionInfo represents information about a running exec session
type ExecSessionInfo struct {
	ID        string
	Command   string
	Status    string // running, completed, error
	StartTime time.Time
	ExitCode  int
}

func execCommand() Definition {
	return Definition{
		Name:        "exec",
		Description: i18n.T("commands_exec_description"),
		SubCommands: []SubCommand{
			{
				Name:        "run",
				Description: i18n.T("commands_exec_run_description"),
				ArgsUsage:   "<command>",
				Handler:     execRunHandler(),
			},
			{
				Name:        "sessions",
				Description: i18n.T("commands_exec_sessions_description"),
				Handler:     execSessionsHandler(),
			},
			{
				Name:        "kill",
				Description: i18n.T("commands_exec_kill_description"),
				ArgsUsage:   "<session-id>",
				Handler:     execKillHandler(),
			},
		},
	}
}

func execRunHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		// Extract command from request text: /exec run <command>
		command := extractCommandFromRequest(req.Text, 2) // Skip "exec" and "run"
		if command == "" {
			return req.Reply(i18n.T("commands_exec_run_usage_example"))
		}

		// Check if exec is allowed from this channel
		if rt != nil && rt.Config != nil {
			if !isExecAllowedFromChannel(req.Channel, rt.Config) {
				return req.Reply(i18n.T("commands_exec_run_restricted"))
			}
		}

		// Execute the command
		result := executeCommand(ctx, command, "", rt)

		// Format and return the result
		reply := formatExecResult(result)
		return req.Reply(reply)
	}
}

func execSessionsHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		// For now, return a simple message
		// In a full implementation, this would track background sessions
		return req.Reply(i18n.T("commands_exec_sessions_response"))
	}
}

func execKillHandler() Handler {
	return func(ctx context.Context, req Request, rt *Runtime) error {
		sessionID := nthToken(req.Text, 2)
		if sessionID == "" {
			return req.Reply(i18n.T("commands_exec_kill_usage"))
		}
		return req.Reply(i18n.Tf("commands_exec_kill_not_found", map[string]any{"ID": sessionID}))
	}
}

// extractCommandFromRequest extracts the command portion from the request text
// skipTokens is the number of tokens to skip (e.g., 2 for "/exec run")
func extractCommandFromRequest(text string, skipTokens int) string {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) <= skipTokens {
		return ""
	}
	return strings.Join(parts[skipTokens:], " ")
}

// isExecAllowedFromChannel checks if exec is allowed from the given channel
func isExecAllowedFromChannel(channel string, cfg *config.Config) bool {
	// Check if remote channels are allowed
	if cfg.Tools.Exec.AllowRemote {
		return true
	}
	// Only allow from internal channels
	return isInternalChannel(channel)
}

// isInternalChannel checks if a channel is considered internal
func isInternalChannel(channel string) bool {
	internalChannels := []string{"cli", "", "internal"}
	for _, internal := range internalChannels {
		if strings.EqualFold(channel, internal) {
			return true
		}
	}
	return false
}

// executeCommand executes a shell command and returns the result
func executeCommand(ctx context.Context, command, cwd string, rt *Runtime) *ExecResult {
	startTime := time.Now()

	// Create timeout context
	var cmdCtx context.Context
	var cancel context.CancelFunc

	timeout := 30 * time.Second // Default timeout
	if rt != nil && rt.Config != nil && rt.Config.Tools.Exec.TimeoutSeconds > 0 {
		timeout = time.Duration(rt.Config.Tools.Exec.TimeoutSeconds) * time.Second
	}

	cmdCtx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	} else {
		cmd = exec.CommandContext(cmdCtx, "sh", "-c", command)
	}

	if cwd != "" {
		cmd.Dir = cwd
	}

	// Apply isolation
	prepareCommandForTermination(cmd)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the command through isolation
	startErr := isolation.Start(cmd)
	if startErr != nil {
		return &ExecResult{
			Output:   "",
			ExitCode: -1,
			Error:    startErr,
			Duration: time.Since(startTime),
		}
	}

	// Wait for completion
	waitErr := cmd.Wait()
	duration := time.Since(startTime)

	// Build output
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += "STDERR:\n" + stderr.String()
	}

	// Determine exit code
	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &ExecResult{
		Output:   output,
		ExitCode: exitCode,
		Error:    waitErr,
		Duration: duration,
	}
}

// formatExecResult formats the execution result for display
func formatExecResult(result *ExecResult) string {
	var sb strings.Builder

	// Header with exit code and duration
	sb.WriteString(
		i18n.Tf("commands_exec_result_exit_code", map[string]any{
			"ExitCode": result.ExitCode,
			"Duration": result.Duration.Round(time.Millisecond),
		}) + "\n",
	)
	sb.WriteString(strings.Repeat("-", 40) + "\n")

	// Output
	if result.Output == "" {
		sb.WriteString(i18n.T("commands_exec_result_no_output"))
	} else {
		sb.WriteString(result.Output)
	}

	// Error message if any
	if result.Error != nil && result.ExitCode != 0 {
		sb.WriteString("\n\n" + i18n.Tf("commands_exec_result_error", map[string]any{"Error": result.Error}))
	}

	return sb.String()
}
