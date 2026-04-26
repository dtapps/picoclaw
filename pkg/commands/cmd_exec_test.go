package commands

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestExecCommand_Run_Success(t *testing.T) {
	defs := BuiltinDefinitions()
	var execDef *Definition
	for i := range defs {
		if defs[i].Name == "exec" {
			execDef = &defs[i]
			break
		}
	}
	if execDef == nil {
		t.Fatal("exec command not found in builtin definitions")
	}

	// Find run subcommand
	var runSub *SubCommand
	for i := range execDef.SubCommands {
		if execDef.SubCommands[i].Name == "run" {
			runSub = &execDef.SubCommands[i]
			break
		}
	}
	if runSub == nil {
		t.Fatal("run subcommand not found")
	}

	// Test with echo command
	var reply string
	err := runSub.Handler(context.Background(), Request{
		Text: "/exec run echo hello world",
		Reply: func(text string) error {
			reply = text
			return nil
		},
	}, &Runtime{
		Config: &config.Config{
			Tools: config.ToolsConfig{
				Exec: config.ExecConfig{
					AllowRemote:    true,
					TimeoutSeconds: 30,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if !strings.Contains(reply, "hello world") {
		t.Fatalf("expected 'hello world' in reply, got: %s", reply)
	}
	if !strings.Contains(reply, "Exit Code: 0") {
		t.Fatalf("expected exit code 0 in reply, got: %s", reply)
	}
}

func TestExecCommand_Run_NoCommand(t *testing.T) {
	defs := BuiltinDefinitions()
	var execDef *Definition
	for i := range defs {
		if defs[i].Name == "exec" {
			execDef = &defs[i]
			break
		}
	}
	if execDef == nil {
		t.Fatal("exec command not found")
	}

	var runSub *SubCommand
	for i := range execDef.SubCommands {
		if execDef.SubCommands[i].Name == "run" {
			runSub = &execDef.SubCommands[i]
			break
		}
	}

	var reply string
	_ = runSub.Handler(context.Background(), Request{
		Text: "/exec run", // No command provided
		Reply: func(text string) error {
			reply = text
			return nil
		},
	}, nil)

	if !strings.Contains(reply, "Usage:") {
		t.Fatalf("expected usage message, got: %s", reply)
	}
}

func TestExecCommand_Run_ExitCode(t *testing.T) {
	defs := BuiltinDefinitions()
	var execDef *Definition
	for i := range defs {
		if defs[i].Name == "exec" {
			execDef = &defs[i]
			break
		}
	}

	var runSub *SubCommand
	for i := range execDef.SubCommands {
		if execDef.SubCommands[i].Name == "run" {
			runSub = &execDef.SubCommands[i]
			break
		}
	}

	// Test command that returns non-zero exit code
	var reply string
	cmd := "exit 42"
	if runtime.GOOS == "windows" {
		cmd = "exit 42"
	}

	_ = runSub.Handler(context.Background(), Request{
		Text: "/exec run " + cmd,
		Reply: func(text string) error {
			reply = text
			return nil
		},
	}, &Runtime{
		Config: &config.Config{
			Tools: config.ToolsConfig{
				Exec: config.ExecConfig{
					AllowRemote:    true,
					TimeoutSeconds: 30,
				},
			},
		},
	})

	if !strings.Contains(reply, "Exit Code: 42") {
		t.Fatalf("expected exit code 42 in reply, got: %s", reply)
	}
}

func TestExecCommand_Sessions(t *testing.T) {
	defs := BuiltinDefinitions()
	var execDef *Definition
	for i := range defs {
		if defs[i].Name == "exec" {
			execDef = &defs[i]
			break
		}
	}

	var sessionsSub *SubCommand
	for i := range execDef.SubCommands {
		if execDef.SubCommands[i].Name == "sessions" {
			sessionsSub = &execDef.SubCommands[i]
			break
		}
	}

	var reply string
	_ = sessionsSub.Handler(context.Background(), Request{
		Text: "/exec sessions",
		Reply: func(text string) error {
			reply = text
			return nil
		},
	}, nil)

	if !strings.Contains(reply, "Active exec sessions") {
		t.Fatalf("expected sessions list message, got: %s", reply)
	}
}

func TestExecCommand_Kill(t *testing.T) {
	defs := BuiltinDefinitions()
	var execDef *Definition
	for i := range defs {
		if defs[i].Name == "exec" {
			execDef = &defs[i]
			break
		}
	}

	var killSub *SubCommand
	for i := range execDef.SubCommands {
		if execDef.SubCommands[i].Name == "kill" {
			killSub = &execDef.SubCommands[i]
			break
		}
	}

	// Test kill without session ID
	var reply string
	_ = killSub.Handler(context.Background(), Request{
		Text: "/exec kill", // No session ID
		Reply: func(text string) error {
			reply = text
			return nil
		},
	}, nil)

	if !strings.Contains(reply, "Usage:") {
		t.Fatalf("expected usage message, got: %s", reply)
	}

	// Test kill with session ID
	reply = ""
	_ = killSub.Handler(context.Background(), Request{
		Text: "/exec kill test-session-123",
		Reply: func(text string) error {
			reply = text
			return nil
		},
	}, nil)

	if !strings.Contains(reply, "not found") {
		t.Fatalf("expected 'not found' message, got: %s", reply)
	}
}

func TestExecCommand_RestrictedFromRemoteChannel(t *testing.T) {
	defs := BuiltinDefinitions()
	var execDef *Definition
	for i := range defs {
		if defs[i].Name == "exec" {
			execDef = &defs[i]
			break
		}
	}

	var runSub *SubCommand
	for i := range execDef.SubCommands {
		if execDef.SubCommands[i].Name == "run" {
			runSub = &execDef.SubCommands[i]
			break
		}
	}

	// Test from telegram channel (should be blocked)
	var reply string
	_ = runSub.Handler(context.Background(), Request{
		Channel: "telegram",
		Text:    "/exec run echo test",
		Reply: func(text string) error {
			reply = text
			return nil
		},
	}, &Runtime{
		Config: &config.Config{
			Tools: config.ToolsConfig{
				Exec: config.ExecConfig{
					AllowRemote:    false, // Remote channels not allowed
					TimeoutSeconds: 30,
				},
			},
		},
	})

	if !strings.Contains(reply, "restricted") {
		t.Fatalf("expected restriction message for remote channel, got: %s", reply)
	}
}

func TestExtractCommandFromRequest(t *testing.T) {
	tests := []struct {
		input      string
		skipTokens int
		want       string
	}{
		{"/exec run ls -la", 2, "ls -la"},
		{"/exec run echo hello world", 2, "echo hello world"},
		{"/exec run", 2, ""},
		{"/exec", 2, ""},
		{"", 2, ""},
		{"/exec run pwd", 3, ""}, // Skip too many tokens
	}

	for _, tt := range tests {
		got := extractCommandFromRequest(tt.input, tt.skipTokens)
		if got != tt.want {
			t.Errorf("extractCommandFromRequest(%q, %d) = %q, want %q", tt.input, tt.skipTokens, got, tt.want)
		}
	}
}

func TestIsInternalChannel(t *testing.T) {
	tests := []struct {
		channel string
		want    bool
	}{
		{"cli", true},
		{"", true},
		{"internal", true},
		{"CLI", true}, // Case insensitive
		{"telegram", false},
		{"discord", false},
		{"slack", false},
	}

	for _, tt := range tests {
		got := isInternalChannel(tt.channel)
		if got != tt.want {
			t.Errorf("isInternalChannel(%q) = %v, want %v", tt.channel, got, tt.want)
		}
	}
}

func TestFormatExecResult(t *testing.T) {
	result := &ExecResult{
		Output:   "hello world",
		ExitCode: 0,
		Error:    nil,
		Duration: 1000000, // 1ms in nanoseconds
	}

	formatted := formatExecResult(result)

	if !strings.Contains(formatted, "Exit Code: 0") {
		t.Errorf("expected exit code in formatted result, got: %s", formatted)
	}
	if !strings.Contains(formatted, "hello world") {
		t.Errorf("expected output in formatted result, got: %s", formatted)
	}
}
