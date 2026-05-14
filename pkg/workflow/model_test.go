package workflow

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		wf      Workflow
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty name",
			wf:      Workflow{Name: "", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}},
			wantErr: true,
			errMsg:  "名称不能为空",
		},
		{
			name:    "no steps",
			wf:      Workflow{Name: "test", Steps: []Step{}},
			wantErr: true,
			errMsg:  "至少需要一个步骤",
		},
		{
			name:    "step missing id",
			wf:      Workflow{Name: "test", Steps: []Step{{ID: "", Action: "agent_prompt", Prompt: "hi"}}},
			wantErr: true,
			errMsg:  "缺少 id",
		},
		{
			name: "duplicate step id",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "agent_prompt", Prompt: "a"},
					{ID: "s1", Action: "agent_prompt", Prompt: "b"},
				},
			},
			wantErr: true,
			errMsg:  "重复",
		},
		{
			name:    "invalid action type",
			wf:      Workflow{Name: "test", Steps: []Step{{ID: "s1", Action: "unknown"}}},
			wantErr: true,
			errMsg:  "动作类型",
		},
		{
			name:    "agent_prompt without prompt",
			wf:      Workflow{Name: "test", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: ""}}},
			wantErr: true,
			errMsg:  "需要提供 prompt",
		},
		{
			name:    "tool_call without tool",
			wf:      Workflow{Name: "test", Steps: []Step{{ID: "s1", Action: "tool_call", Tool: ""}}},
			wantErr: true,
			errMsg:  "需要提供 tool",
		},
		{
			name: "parallel without sub-steps",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "parallel", Parallel: []Step{}}},
			},
			wantErr: true,
			errMsg:  "至少一个子步骤",
		},
		{
			name: "if without when",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "if", IfTrue: []Step{{ID: "t1", Action: "agent_prompt", Prompt: "x"}}},
				},
			},
			wantErr: true,
			errMsg:  "需要提供 when 条件",
		},
		{
			name: "if without any branch",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "if", When: "on_error", IfTrue: []Step{}, IfFalse: []Step{}}},
			},
			wantErr: true,
			errMsg:  "至少一个分支步骤",
		},
		{
			name:    "valid agent_prompt",
			wf:      Workflow{Name: "test", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hello"}}},
			wantErr: false,
		},
		{
			name:    "valid tool_call",
			wf:      Workflow{Name: "test", Steps: []Step{{ID: "s1", Action: "tool_call", Tool: "exec"}}},
			wantErr: false,
		},
		{
			name: "valid parallel",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "parallel", Parallel: []Step{{ID: "p1", Action: "agent_prompt", Prompt: "x"}}},
				},
			},
			wantErr: false,
		},
		{
			name: "valid if",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{
						ID:     "s1",
						Action: "if",
						When:   "on_error",
						IfTrue: []Step{{ID: "t1", Action: "agent_prompt", Prompt: "err"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid workflow with vars",
			wf: Workflow{
				Name:  "test",
				Vars:  map[string]string{"key": "val"},
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "{{.vars.key}}"}},
			},
			wantErr: false,
		},
		{
			name:    "step id with Chinese characters",
			wf:      Workflow{Name: "test", Steps: []Step{{ID: "步骤1", Action: "agent_prompt", Prompt: "hi"}}},
			wantErr: true,
			errMsg:  "不合法",
		},
		{
			name:    "step id with spaces",
			wf:      Workflow{Name: "test", Steps: []Step{{ID: "step one", Action: "agent_prompt", Prompt: "hi"}}},
			wantErr: true,
			errMsg:  "不合法",
		},
		{
			name:    "step id with dots",
			wf:      Workflow{Name: "test", Steps: []Step{{ID: "step.one", Action: "agent_prompt", Prompt: "hi"}}},
			wantErr: true,
			errMsg:  "不合法",
		},
		{
			name:    "step id with underscores is valid",
			wf:      Workflow{Name: "test", Steps: []Step{{ID: "step_one", Action: "agent_prompt", Prompt: "hi"}}},
			wantErr: false,
		},
		{
			name: "step with optional name",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Name: "获取日期", Action: "agent_prompt", Prompt: "hi"}},
			},
			wantErr: false,
		},
		{
			name:    "workflow name with Chinese characters",
			wf:      Workflow{Name: "每日简报", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}},
			wantErr: true,
			errMsg:  "名称不合法",
		},
		{
			name:    "workflow name with spaces",
			wf:      Workflow{Name: "my workflow", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}},
			wantErr: true,
			errMsg:  "名称不合法",
		},
		{
			name:    "workflow name with hyphen is valid",
			wf:      Workflow{Name: "my-workflow", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}},
			wantErr: false,
		},
		{
			name: "invalid delay format",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi", Delay: "abc"}},
			},
			wantErr: true,
			errMsg:  "delay",
		},
		{
			name: "invalid delay format - number without unit",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi", Delay: "5"}},
			},
			wantErr: true,
			errMsg:  "delay",
		},
		{
			name: "valid delay format",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi", Delay: "5s"}},
			},
			wantErr: false,
		},
		{
			name: "valid delay format - compound",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi", Delay: "1m30s"}},
			},
			wantErr: false,
		},
		{
			name: "invalid timeout format",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi", Timeout: "30"}},
			},
			wantErr: true,
			errMsg:  "timeout",
		},
		{
			name: "valid timeout format",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi", Timeout: "30s"}},
			},
			wantErr: false,
		},
		{
			name: "invalid retry delay format",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "agent_prompt", Prompt: "hi", Retry: &RetryConfig{MaxAttempts: 3, Delay: "bad"}},
				},
			},
			wantErr: true,
			errMsg:  "retry delay",
		},
		{
			name: "valid retry delay format",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "agent_prompt", Prompt: "hi", Retry: &RetryConfig{MaxAttempts: 3, Delay: "10s"}},
				},
			},
			wantErr: false,
		},
		// 模板引用校验
		{
			name: "valid step output reference",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "agent_prompt", Prompt: "hi", OutputKey: "result"},
					{ID: "s2", Action: "agent_prompt", Prompt: "{{.s1.result}}"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid vars reference - key not defined",
			wf: Workflow{
				Name:  "test",
				Vars:  map[string]string{"name": "alice"},
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "{{.vars.missing_key}}"}},
			},
			wantErr: true,
			errMsg:  "不存在的变量",
		},
		{
			name: "invalid step reference - step not defined",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "{{.nonexistent.key}}"}},
			},
			wantErr: true,
			errMsg:  "不存在的步骤",
		},
		{
			name: "invalid step reference - step has no output_key",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "agent_prompt", Prompt: "hi"},
					{ID: "s2", Action: "agent_prompt", Prompt: "{{.s1.result}}"},
				},
			},
			wantErr: true,
			errMsg:  "不存在的步骤",
		},
		{
			name: "invalid output_key mismatch",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "agent_prompt", Prompt: "hi", OutputKey: "data"},
					{ID: "s2", Action: "agent_prompt", Prompt: "{{.s1.wrong_key}}"},
				},
			},
			wantErr: true,
			errMsg:  "不存在的输出键",
		},
		{
			name: "template ref in args validated",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "tool_call", Tool: "exec", Args: map[string]any{"q": "{{.vars.missing}}"}},
				},
			},
			wantErr: true,
			errMsg:  "不存在的变量",
		},
		{
			name: "template ref in when validated",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "agent_prompt", Prompt: "hi", OutputKey: "ok"},
					{
						ID:     "s2",
						Action: "if",
						When:   "{{.missing_step.val}}",
						IfTrue: []Step{{ID: "t1", Action: "agent_prompt", Prompt: "x"}},
					},
				},
			},
			wantErr: true,
			errMsg:  "不存在的步骤",
		},
		{
			name: "valid vars reference passes",
			wf: Workflow{
				Name:  "test",
				Vars:  map[string]string{"dir": "/tmp"},
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "{{.vars.dir}}"}},
			},
			wantErr: false,
		},
		{
			name: "parallel sub-step template ref validated",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "parallel", Parallel: []Step{
						{ID: "p1", Action: "agent_prompt", Prompt: "{{.vars.missing}}"},
					}},
				},
			},
			wantErr: true,
			errMsg:  "不存在的变量",
		},
		// self 引用校验
		{
			name: "self.name reference passes",
			wf: Workflow{
				Name: "test",
				Steps: []Step{{
					ID: "s1", Name: "hello", Action: "tool_call", Tool: "search",
					Args: map[string]any{"query": "{{.self.name}} world"},
				}},
			},
			wantErr: false,
		},
		{
			name: "self.id reference passes",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "step={{.self.id}}"}},
			},
			wantErr: false,
		},
		{
			name: "self with unsupported property fails",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "{{.self.foo}}"}},
			},
			wantErr: true,
			errMsg:  "不存在的自身属性",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wf.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMsg)
				}
				if !contains(err.Error(), tt.errMsg) {
					t.Fatalf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || searchString(s, substr))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestIsValidWorkflowName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"my-workflow", true},
		{"my_workflow", true},
		{"Workflow123", true},
		{"", false},
		{"每日简报", false},
		{"my workflow", false},
		{"my.workflow", false},
		{"my!workflow", false},
		{"_private", true},
		{"123", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidWorkflowName(tt.name); got != tt.want {
				t.Errorf("isValidWorkflowName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsValidStepID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"s1", true},
		{"get_date", true},
		{"Step123", true},
		{"", false},
		{"步骤1", false},
		{"step one", false},
		{"step.one", false},
		{"step-one", false},
		{"step!1", false},
		{"_private", true},
		{"123", true},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := isValidStepID(tt.id); got != tt.want {
				t.Errorf("isValidStepID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestValidateWithToolSchema(t *testing.T) {
	// 模拟工具 Schema 查询：exec 工具需要 action 参数
	provider := func(toolName string) (map[string]any, bool) {
		schemas := map[string]map[string]any{
			"exec": {
				"type": "object",
				"properties": map[string]any{
					"action":  map[string]any{"type": "string"},
					"command": map[string]any{"type": "string"},
				},
				"required": []string{"action"},
			},
			"search": {
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
				"required": []string{"query"},
			},
		}
		schema, ok := schemas[toolName]
		return schema, ok
	}

	tests := []struct {
		name        string
		wf          Workflow
		wantErr     bool
		errMsg      string
		nilProvider bool
	}{
		{
			name: "tool_call with all required params passes",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{
						ID:     "s1",
						Action: "tool_call",
						Tool:   "exec",
						Args:   map[string]any{"action": "run", "command": "ls"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "tool_call missing required param fails",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "tool_call", Tool: "exec", Args: map[string]any{"command": "ls"}}},
			},
			wantErr: true,
			errMsg:  "缺少工具 'exec' 的必填参数 'action'",
		},
		{
			name: "tool_call with unknown tool fails",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "tool_call", Tool: "nonexistent", Args: map[string]any{}}},
			},
			wantErr: true,
			errMsg:  "不存在的工具 'nonexistent'",
		},
		{
			name: "tool_call with no required params in schema passes",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "tool_call", Tool: "search", Args: map[string]any{"query": "hello"}}},
			},
			wantErr: false,
		},
		{
			name: "tool_call missing required query param fails",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "tool_call", Tool: "search", Args: map[string]any{"other": "val"}}},
			},
			wantErr: true,
			errMsg:  "缺少工具 'search' 的必填参数 'query'",
		},
		{
			name: "tool_call with empty args fails if required",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "tool_call", Tool: "exec", Args: map[string]any{}}},
			},
			wantErr: true,
			errMsg:  "缺少工具 'exec' 的必填参数 'action'",
		},
		{
			name: "parallel sub-step missing required param fails",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "parallel", Parallel: []Step{
						{ID: "p1", Action: "tool_call", Tool: "exec", Args: map[string]any{}},
					}},
				},
			},
			wantErr: true,
			errMsg:  "缺少工具 'exec' 的必填参数 'action'",
		},
		{
			name: "if sub-step missing required param fails",
			wf: Workflow{
				Name: "test",
				Steps: []Step{
					{ID: "s1", Action: "if", When: "on_error", IfTrue: []Step{
						{ID: "t1", Action: "tool_call", Tool: "exec", Args: map[string]any{}},
					}},
				},
			},
			wantErr: true,
			errMsg:  "缺少工具 'exec' 的必填参数 'action'",
		},
		{
			name: "tool_call with empty string value fails",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "tool_call", Tool: "exec", Args: map[string]any{"action": ""}}},
			},
			wantErr: true,
			errMsg:  "缺少工具 'exec' 的必填参数 'action'",
		},
		{
			name: "tool_call with nil value fails",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "tool_call", Tool: "exec", Args: map[string]any{"action": nil}}},
			},
			wantErr: true,
			errMsg:  "缺少工具 'exec' 的必填参数 'action'",
		},
		{
			name: "tool_call with template ref value passes",
			wf: Workflow{
				Name: "test",
				Vars: map[string]string{"act": "run"},
				Steps: []Step{
					{ID: "s1", Action: "tool_call", Tool: "exec", Args: map[string]any{"action": "{{.vars.act}}"}},
				},
			},
			wantErr: false,
		},
		{
			name: "nil provider skips tool validation",
			wf: Workflow{
				Name:  "test",
				Steps: []Step{{ID: "s1", Action: "tool_call", Tool: "exec", Args: map[string]any{}}},
			},
			wantErr:     false,
			errMsg:      "",
			nilProvider: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := provider
			if tt.nilProvider {
				p = nil
			}
			err := tt.wf.ValidateWithToolSchema(p)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMsg)
				}
				if !contains(err.Error(), tt.errMsg) {
					t.Fatalf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

// TestValidate nil provider 测试 Validate() 兼容性
func TestValidate_NoProvider(t *testing.T) {
	wf := Workflow{
		Name:  "test",
		Steps: []Step{{ID: "s1", Action: "tool_call", Tool: "exec", Args: map[string]any{}}},
	}
	// Validate() 内部调用 ValidateWithToolSchema(nil)，不会校验工具参数
	if err := wf.Validate(); err != nil {
		t.Fatalf("expected no error with nil provider, got %v", err)
	}
}
