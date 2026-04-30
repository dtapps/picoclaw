package common

import (
	"testing"
)

func TestExtractInlineToolCalls_KimiK2Format(t *testing.T) {
	// This is the exact format from the kimi-k2 response in the error log
	content := `[{'type': 'text', 'text': '[tool_use: exec, args: {"action":"run","command":"git push -u origin master","cwd":"/root/.picoclaw/workspace/news"}]'}]<|tool_call_end|><|tool_calls_section_end|>`

	toolCalls, cleaned := ExtractInlineToolCalls(content)
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]
	if tc.Name != "exec" {
		t.Errorf("expected tool name %q, got %q", "exec", tc.Name)
	}
	if tc.Arguments["action"] != "run" {
		t.Errorf("expected args.action=%q, got %v", "run", tc.Arguments["action"])
	}
	if tc.Arguments["command"] != "git push -u origin master" {
		t.Errorf("expected args.command=%q, got %v", "git push -u origin master", tc.Arguments["command"])
	}
	if tc.Arguments["cwd"] != "/root/.picoclaw/workspace/news" {
		t.Errorf("expected args.cwd=%q, got %v", "/root/.picoclaw/workspace/news", tc.Arguments["cwd"])
	}
	if tc.Function == nil {
		t.Fatal("expected Function to be set")
	}
	if tc.Function.Name != "exec" {
		t.Errorf("expected Function.Name=%q, got %q", "exec", tc.Function.Name)
	}

	// The cleaned content should be empty since there's no text besides the tool call
	if cleaned != "" {
		t.Errorf("expected empty cleaned content, got %q", cleaned)
	}
}

func TestExtractInlineToolCalls_SimpleFormat(t *testing.T) {
	content := `[tool_use: get_weather, args: {"city": "SF"}]`

	toolCalls, cleaned := ExtractInlineToolCalls(content)
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]
	if tc.Name != "get_weather" {
		t.Errorf("expected tool name %q, got %q", "get_weather", tc.Name)
	}
	if tc.Arguments["city"] != "SF" {
		t.Errorf("expected args.city=%q, got %v", "SF", tc.Arguments["city"])
	}

	if cleaned != "" {
		t.Errorf("expected empty cleaned content, got %q", cleaned)
	}
}

func TestExtractInlineToolCalls_WithSurroundingText(t *testing.T) {
	content := `I will check the weather for you. [tool_use: get_weather, args: {"city": "NYC"}] Let me know if you need anything else.`

	toolCalls, cleaned := ExtractInlineToolCalls(content)
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]
	if tc.Name != "get_weather" {
		t.Errorf("expected tool name %q, got %q", "get_weather", tc.Name)
	}
	if tc.Arguments["city"] != "NYC" {
		t.Errorf("expected args.city=%q, got %v", "NYC", tc.Arguments["city"])
	}

	expected := "I will check the weather for you.  Let me know if you need anything else."
	if cleaned != expected {
		t.Errorf("expected cleaned=%q, got %q", expected, cleaned)
	}
}

func TestExtractInlineToolCalls_MultipleToolCalls(t *testing.T) {
	content := `[tool_use: read_file, args: {"path": "/tmp/a.txt"}][tool_use: write_file, args: {"path": "/tmp/b.txt", "content": "hello"}]`

	toolCalls, cleaned := ExtractInlineToolCalls(content)
	if len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(toolCalls))
	}

	if toolCalls[0].Name != "read_file" {
		t.Errorf("expected first tool name %q, got %q", "read_file", toolCalls[0].Name)
	}
	if toolCalls[1].Name != "write_file" {
		t.Errorf("expected second tool name %q, got %q", "write_file", toolCalls[1].Name)
	}

	if cleaned != "" {
		t.Errorf("expected empty cleaned content, got %q", cleaned)
	}
}

func TestExtractInlineToolCalls_NoInlineToolCalls(t *testing.T) {
	content := "Hello, how can I help you today?"

	toolCalls, cleaned := ExtractInlineToolCalls(content)
	if len(toolCalls) != 0 {
		t.Fatalf("expected 0 tool calls, got %d", len(toolCalls))
	}
	if cleaned != content {
		t.Errorf("expected content unchanged, got %q", cleaned)
	}
}

func TestExtractInlineToolCalls_InvalidFormat(t *testing.T) {
	// Marker present but no args separator
	content := `[tool_use: something weird here]`

	toolCalls, cleaned := ExtractInlineToolCalls(content)
	if len(toolCalls) != 0 {
		t.Fatalf("expected 0 tool calls for invalid format, got %d", len(toolCalls))
	}
	// Should preserve the content since it wasn't a valid tool call
	if cleaned != "[tool_use: something weird here]" {
		t.Errorf("expected content preserved, got %q", cleaned)
	}
}

func TestExtractInlineToolCalls_WithSpecialTokens(t *testing.T) {
	content := `[tool_use: exec, args: {"cmd": "ls"}]<|tool_call_end|><|tool_calls_section_end|>`

	toolCalls, cleaned := ExtractInlineToolCalls(content)
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if cleaned != "" {
		t.Errorf("expected empty cleaned content, got %q", cleaned)
	}
}

func TestExtractInlineToolCalls_NestedJSON(t *testing.T) {
	content := `[tool_use: exec, args: {"action":"run","command":"echo hello","env":{"HOME":"/root","PATH":"/usr/bin"}}]`

	toolCalls, cleaned := ExtractInlineToolCalls(content)
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]
	if tc.Name != "exec" {
		t.Errorf("expected tool name %q, got %q", "exec", tc.Name)
	}
	env, ok := tc.Arguments["env"].(map[string]any)
	if !ok {
		t.Fatalf("expected env to be map[string]interface{}, got %T", tc.Arguments["env"])
	}
	if env["HOME"] != "/root" {
		t.Errorf("expected env.HOME=%q, got %v", "/root", env["HOME"])
	}

	if cleaned != "" {
		t.Errorf("expected empty cleaned content, got %q", cleaned)
	}
}

func TestNormalizeInlineToolCalls_WithStandardToolCalls(t *testing.T) {
	// When standard tool_calls are present, should not modify the response
	resp := &LLMResponse{
		Content: "some content with [tool_use: test, args: {}]",
		ToolCalls: []ToolCall{{
			ID:   "call_123",
			Name: "standard_tool",
		}},
		FinishReason: "tool_calls",
	}

	NormalizeInlineToolCalls(resp)

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call (unchanged), got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "standard_tool" {
		t.Errorf("expected tool name %q (unchanged), got %q", "standard_tool", resp.ToolCalls[0].Name)
	}
}

func TestNormalizeInlineToolCalls_WithInlineToolCalls(t *testing.T) {
	resp := &LLMResponse{
		Content:      `[tool_use: exec, args: {"action":"run","command":"ls"}]`,
		ToolCalls:    nil,
		FinishReason: "stop",
	}

	NormalizeInlineToolCalls(resp)

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call after normalization, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "exec" {
		t.Errorf("expected tool name %q, got %q", "exec", resp.ToolCalls[0].Name)
	}
	if resp.FinishReason != "tool_calls" {
		t.Errorf("expected finish_reason=%q, got %q", "tool_calls", resp.FinishReason)
	}
}

func TestNormalizeInlineToolCalls_NoInlineToolCalls(t *testing.T) {
	resp := &LLMResponse{
		Content:      "Hello, how can I help?",
		ToolCalls:    nil,
		FinishReason: "stop",
	}

	NormalizeInlineToolCalls(resp)

	if len(resp.ToolCalls) != 0 {
		t.Fatalf("expected 0 tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.Content != "Hello, how can I help?" {
		t.Errorf("expected content unchanged, got %q", resp.Content)
	}
}

func TestExtractInlineToolCalls_EmptyArgs(t *testing.T) {
	content := `[tool_use: list_files, args: {}]`

	toolCalls, cleaned := ExtractInlineToolCalls(content)
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0].Name != "list_files" {
		t.Errorf("expected tool name %q, got %q", "list_files", toolCalls[0].Name)
	}
	if len(toolCalls[0].Arguments) != 0 {
		t.Errorf("expected empty arguments, got %v", toolCalls[0].Arguments)
	}
	if cleaned != "" {
		t.Errorf("expected empty cleaned content, got %q", cleaned)
	}
}

func TestFindMatchingBraceInString(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		start    int
		expected int
	}{
		{"simple", `{"a":1}`, 0, 6},
		{"nested", `{"a":{"b":2}}`, 0, 12},
		{"with_string_braces", `{"a": "hello {world}"}`, 0, 21},
		{"with_escaped_quotes", `{"a": "he said \"hi\""}`, 0, 22},
		{"not_starting_with_brace", `hello`, 0, -1},
		{"out_of_range", `hello`, 10, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findMatchingBraceInString(tt.s, tt.start)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}
