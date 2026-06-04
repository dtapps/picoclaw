package workflow

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecute_AgentPrompt(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		called := false
		se := &StepExecutor{
			AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
				called = true
				return "response: " + prompt, nil
			},
		}
		step := Step{ID: "s1", Action: "agent_prompt", Prompt: "hello"}
		result := se.Execute(context.Background(), step, nil)
		if result.Error != nil {
			t.Fatalf("unexpected error: %v", result.Error)
		}
		if !called {
			t.Fatal("AgentPromptFunc not called")
		}
		if result.Output != "response: hello" {
			t.Fatalf("Output = %q, want %q", result.Output, "response: hello")
		}
	})

	t.Run("nil func", func(t *testing.T) {
		se := &StepExecutor{}
		step := Step{ID: "s1", Action: "agent_prompt", Prompt: "hello"}
		result := se.Execute(context.Background(), step, nil)
		if result.Error == nil {
			t.Fatal("expected error for nil AgentPromptFunc")
		}
	})

	t.Run("func error", func(t *testing.T) {
		se := &StepExecutor{
			AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
				return "", errors.New("llm error")
			},
		}
		step := Step{ID: "s1", Action: "agent_prompt", Prompt: "hello"}
		result := se.Execute(context.Background(), step, nil)
		if result.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestExecute_ToolCall(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		se := &StepExecutor{
			ToolCallFunc: func(ctx context.Context, toolName string, args map[string]any) (string, bool, error) {
				return "tool result", false, nil
			},
		}
		step := Step{ID: "s1", Action: "tool_call", Tool: "exec", Args: map[string]any{"cmd": "ls"}}
		result := se.Execute(context.Background(), step, nil)
		if result.Error != nil {
			t.Fatalf("unexpected error: %v", result.Error)
		}
		if result.Output != "tool result" {
			t.Fatalf("Output = %q, want %q", result.Output, "tool result")
		}
	})

	t.Run("tool error flag", func(t *testing.T) {
		se := &StepExecutor{
			ToolCallFunc: func(ctx context.Context, toolName string, args map[string]any) (string, bool, error) {
				return "tool error msg", true, nil
			},
		}
		step := Step{ID: "s1", Action: "tool_call", Tool: "exec"}
		result := se.Execute(context.Background(), step, nil)
		if result.Error == nil {
			t.Fatal("expected error from tool error flag")
		}
	})

	t.Run("nil func", func(t *testing.T) {
		se := &StepExecutor{}
		step := Step{ID: "s1", Action: "tool_call", Tool: "exec"}
		result := se.Execute(context.Background(), step, nil)
		if result.Error == nil {
			t.Fatal("expected error for nil ToolCallFunc")
		}
	})
}

func TestExecute_Parallel(t *testing.T) {
	t.Run("all succeed", func(t *testing.T) {
		var callCount int32
		se := &StepExecutor{
			AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
				atomic.AddInt32(&callCount, 1)
				return "result: " + prompt, nil
			},
		}
		step := Step{
			ID:     "p1",
			Action: "parallel",
			Parallel: []ParallelBranch{
				{Branch: []Step{{ID: "sub1", Action: "agent_prompt", Prompt: "a", OutputKey: "a"}}},
				{Branch: []Step{{ID: "sub2", Action: "agent_prompt", Prompt: "b", OutputKey: "b"}}},
			},
		}
		result := se.Execute(context.Background(), step, nil)
		if result.Error != nil {
			t.Fatalf("unexpected error: %v", result.Error)
		}
		if atomic.LoadInt32(&callCount) != 2 {
			t.Fatalf("callCount = %d, want 2", callCount)
		}
	})

	t.Run("one fails", func(t *testing.T) {
		se := &StepExecutor{
			AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
				if prompt == "fail" {
					return "", errors.New("sub-step error")
				}
				return "ok", nil
			},
		}
		step := Step{
			ID:     "p1",
			Action: "parallel",
			Parallel: []ParallelBranch{
				{Branch: []Step{{ID: "sub1", Action: "agent_prompt", Prompt: "ok"}}},
				{Branch: []Step{{ID: "sub2", Action: "agent_prompt", Prompt: "fail"}}},
			},
		}
		result := se.Execute(context.Background(), step, nil)
		if result.Error == nil {
			t.Fatal("expected error from parallel failure")
		}
	})
}

func TestExecute_UnknownAction(t *testing.T) {
	se := &StepExecutor{}
	step := Step{ID: "s1", Action: "unknown"}
	result := se.Execute(context.Background(), step, nil)
	if result.Error == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestExecuteWithRetry(t *testing.T) {
	t.Run("succeeds on first try", func(t *testing.T) {
		se := &StepExecutor{
			AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
				return "ok", nil
			},
		}
		step := Step{ID: "s1", Action: "agent_prompt", Prompt: "hi"}
		result := se.ExecuteWithRetry(context.Background(), step, nil)
		if result.Error != nil {
			t.Fatalf("unexpected error: %v", result.Error)
		}
		if result.Output != "ok" {
			t.Fatalf("Output = %q, want %q", result.Output, "ok")
		}
	})

	t.Run("succeeds on retry", func(t *testing.T) {
		var attempts int32
		se := &StepExecutor{
			AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
				if atomic.AddInt32(&attempts, 1) < 3 {
					return "", errors.New("retry me")
				}
				return "ok", nil
			},
		}
		step := Step{
			ID:     "s1",
			Action: "agent_prompt",
			Prompt: "hi",
			Retry:  &RetryConfig{MaxAttempts: 3, Delay: "1ms"},
		}
		result := se.ExecuteWithRetry(context.Background(), step, nil)
		if result.Error != nil {
			t.Fatalf("unexpected error: %v", result.Error)
		}
		if atomic.LoadInt32(&attempts) != 3 {
			t.Fatalf("attempts = %d, want 3", attempts)
		}
	})

	t.Run("all retries fail", func(t *testing.T) {
		se := &StepExecutor{
			AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
				return "", errors.New("always fail")
			},
		}
		step := Step{
			ID:     "s1",
			Action: "agent_prompt",
			Prompt: "hi",
			Retry:  &RetryConfig{MaxAttempts: 2, Delay: "1ms"},
		}
		result := se.ExecuteWithRetry(context.Background(), step, nil)
		if result.Error == nil {
			t.Fatal("expected error after all retries fail")
		}
	})

	t.Run("context canceled during retry", func(t *testing.T) {
		se := &StepExecutor{
			AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
				return "", errors.New("fail")
			},
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(5 * time.Millisecond)
			cancel()
		}()
		step := Step{
			ID:     "s1",
			Action: "agent_prompt",
			Prompt: "hi",
			Retry:  &RetryConfig{MaxAttempts: 10, Delay: "50ms"},
		}
		result := se.ExecuteWithRetry(ctx, step, nil)
		if result.Error == nil {
			t.Fatal("expected error from canceled context")
		}
	})
}

func TestExecute_TemplateResolution(t *testing.T) {
	se := &StepExecutor{
		AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
			return "got: " + prompt, nil
		},
		ToolCallFunc: func(ctx context.Context, toolName string, args map[string]any) (string, bool, error) {
			return args["query"].(string), false, nil
		},
	}

	outputs := map[string]map[string]any{
		"prev": {"result": "world"},
		"vars": {"dir": "/tmp"},
	}

	t.Run("prompt template resolved", func(t *testing.T) {
		step := Step{ID: "s1", Action: "agent_prompt", Prompt: "hello {{.prev.result}}"}
		result := se.Execute(context.Background(), step, outputs)
		if result.Output != "got: hello world" {
			t.Fatalf("Output = %q, want %q", result.Output, "got: hello world")
		}
	})

	t.Run("args template resolved", func(t *testing.T) {
		step := Step{
			ID:     "s1",
			Action: "tool_call",
			Tool:   "search",
			Args:   map[string]any{"query": "{{.vars.dir}}/data"},
		}
		result := se.Execute(context.Background(), step, outputs)
		if result.Output != "/tmp/data" {
			t.Fatalf("Output = %q, want %q", result.Output, "/tmp/data")
		}
	})
}

func TestExecute_SelfReference(t *testing.T) {
	se := &StepExecutor{
		AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
			return "got: " + prompt, nil
		},
		ToolCallFunc: func(ctx context.Context, toolName string, args map[string]any) (string, bool, error) {
			return args["query"].(string), false, nil
		},
	}

	t.Run("self.name in prompt", func(t *testing.T) {
		step := Step{ID: "s1", Name: "hello", Action: "agent_prompt", Prompt: "{{.self.name}} world"}
		result := se.Execute(context.Background(), step, nil)
		if result.Output != "got: hello world" {
			t.Fatalf("Output = %q, want %q", result.Output, "got: hello world")
		}
	})

	t.Run("self.name in args", func(t *testing.T) {
		step := Step{
			ID: "search_item", Name: "hello", Action: "tool_call", Tool: "search",
			Args: map[string]any{"query": "{{.self.name}} world {{.vars.date}}"},
		}
		outputs := map[string]map[string]any{"vars": {"date": "2025-01-01"}}
		result := se.Execute(context.Background(), step, outputs)
		if result.Output != "hello world 2025-01-01" {
			t.Fatalf("Output = %q, want %q", result.Output, "hello world 2025-01-01")
		}
	})

	t.Run("self.id resolved", func(t *testing.T) {
		step := Step{ID: "search_maoming", Action: "agent_prompt", Prompt: "step={{.self.id}}"}
		result := se.Execute(context.Background(), step, nil)
		if result.Output != "got: step=search_maoming" {
			t.Fatalf("Output = %q, want %q", result.Output, "got: step=search_maoming")
		}
	})

	t.Run("self.name missing when no name", func(t *testing.T) {
		step := Step{ID: "s1", Action: "agent_prompt", Prompt: "{{.self.name}}"}
		result := se.Execute(context.Background(), step, nil)
		// name 未设置时模板无法解析，保留原文
		if result.Output != "got: {{.self.name}}" {
			t.Fatalf("Output = %q, want %q", result.Output, "got: {{.self.name}}")
		}
	})
}

func TestResolveArgsTemplates(t *testing.T) {
	outputs := map[string]map[string]any{
		"vars": {"dir": "/tmp"},
	}

	t.Run("nil args", func(t *testing.T) {
		got := resolveArgsTemplates(nil, outputs)
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("string values resolved", func(t *testing.T) {
		args := map[string]any{"path": "{{.vars.dir}}/data", "count": 42}
		got := resolveArgsTemplates(args, outputs)
		if got["path"] != "/tmp/data" {
			t.Fatalf("path = %v, want /tmp/data", got["path"])
		}
		if got["count"] != 42 {
			t.Fatalf("count = %v, want 42", got["count"])
		}
	})
}
