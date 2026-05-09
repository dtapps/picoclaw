package workflow

import "testing"

func TestParseYAMLWorkflow(t *testing.T) {
	t.Run("valid yaml", func(t *testing.T) {
		data := []byte(`
name: test-workflow
description: "a test"
vars:
  project_dir: "/tmp/work"
triggers:
  - cron: "0 8 * * *"
    tz: "Asia/Shanghai"
steps:
  - id: step1
    action: agent_prompt
    prompt: "hello"
    output_key: result
  - id: step2
    action: tool_call
    tool: exec
    args:
      command: "ls"
`)
		wf, err := ParseYAMLWorkflow(data)
		if err != nil {
			t.Fatalf("ParseYAMLWorkflow() error: %v", err)
		}
		if wf.Name != "test-workflow" {
			t.Fatalf("Name = %q, want %q", wf.Name, "test-workflow")
		}
		if wf.Description != "a test" {
			t.Fatalf("Description = %q, want %q", wf.Description, "a test")
		}
		if len(wf.Vars) != 1 || wf.Vars["project_dir"] != "/tmp/work" {
			t.Fatalf("Vars = %v, want {project_dir: /tmp/work}", wf.Vars)
		}
		if len(wf.Triggers) != 1 {
			t.Fatalf("len(Triggers) = %d, want 1", len(wf.Triggers))
		}
		if wf.Triggers[0].Cron != "0 8 * * *" {
			t.Fatalf("Cron = %q, want %q", wf.Triggers[0].Cron, "0 8 * * *")
		}
		if wf.Triggers[0].TZ != "Asia/Shanghai" {
			t.Fatalf("TZ = %q, want %q", wf.Triggers[0].TZ, "Asia/Shanghai")
		}
		if len(wf.Steps) != 2 {
			t.Fatalf("len(Steps) = %d, want 2", len(wf.Steps))
		}
		if wf.Steps[0].ID != "step1" || wf.Steps[0].Action != "agent_prompt" {
			t.Fatalf("Step[0] = %+v, unexpected", wf.Steps[0])
		}
		if wf.Steps[1].Action != "tool_call" || wf.Steps[1].Tool != "exec" {
			t.Fatalf("Step[1] = %+v, unexpected", wf.Steps[1])
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		data := []byte(`{invalid yaml [[`)
		_, err := ParseYAMLWorkflow(data)
		if err == nil {
			t.Fatal("expected error for invalid YAML, got nil")
		}
	})

	t.Run("empty yaml", func(t *testing.T) {
		wf, err := ParseYAMLWorkflow([]byte(""))
		if err != nil {
			t.Fatalf("ParseYAMLWorkflow() error: %v", err)
		}
		if wf.Name != "" {
			t.Fatalf("Name = %q, want empty", wf.Name)
		}
	})

	t.Run("round-trip", func(t *testing.T) {
		original := &Workflow{
			Name:        "round-trip",
			Description: "test round trip",
			Vars:        map[string]string{"key": "value"},
			Triggers:    []Trigger{{Cron: "0 9 * * *", TZ: "UTC"}},
			Steps: []Step{
				{ID: "s1", Action: "agent_prompt", Prompt: "test", OutputKey: "out"},
			},
			Config: WorkflowConfig{FailureStrategy: "stop"},
		}

		data, err := renderYAMLWorkflow(original)
		if err != nil {
			t.Fatalf("renderYAMLWorkflow() error: %v", err)
		}

		parsed, err := ParseYAMLWorkflow(data)
		if err != nil {
			t.Fatalf("ParseYAMLWorkflow() error: %v", err)
		}

		if parsed.Name != original.Name {
			t.Fatalf("Name = %q, want %q", parsed.Name, original.Name)
		}
		if parsed.Description != original.Description {
			t.Fatalf("Description = %q, want %q", parsed.Description, original.Description)
		}
		if parsed.Vars["key"] != original.Vars["key"] {
			t.Fatalf("Vars[key] = %q, want %q", parsed.Vars["key"], original.Vars["key"])
		}
		if len(parsed.Steps) != len(original.Steps) {
			t.Fatalf("len(Steps) = %d, want %d", len(parsed.Steps), len(original.Steps))
		}
		if parsed.Steps[0].ID != original.Steps[0].ID {
			t.Fatalf("Step[0].ID = %q, want %q", parsed.Steps[0].ID, original.Steps[0].ID)
		}
	})
}
