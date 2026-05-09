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
