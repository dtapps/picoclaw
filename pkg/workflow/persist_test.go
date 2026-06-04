package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestStore(t *testing.T) *PersistStore {
	t.Helper()
	dir := t.TempDir()
	store := NewPersistStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	return store
}

func TestNewPersistStore(t *testing.T) {
	store := NewPersistStore("/tmp/test-workspace")
	if store.workflowsDir != "/tmp/test-workspace/workflows" {
		t.Fatalf("workflowsDir = %q, want %q", store.workflowsDir, "/tmp/test-workspace/workflows")
	}
	if store.stateDir != "/tmp/test-workspace/state/workflows" {
		t.Fatalf("stateDir = %q, want %q", store.stateDir, "/tmp/test-workspace/state/workflows")
	}
}

func TestPersistStore_Init(t *testing.T) {
	dir := t.TempDir()
	store := NewPersistStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	// 验证目录已创建
	if _, err := os.Stat(store.workflowsDir); os.IsNotExist(err) {
		t.Fatal("workflowsDir not created")
	}
	if _, err := os.Stat(store.stateDir); os.IsNotExist(err) {
		t.Fatal("stateDir not created")
	}
	// 重复调用不应报错
	if err := store.Init(); err != nil {
		t.Fatalf("second Init() error: %v", err)
	}
}

func TestPersistStore_SaveAndLoadWorkflow(t *testing.T) {
	store := setupTestStore(t)

	wf := &Workflow{
		Name:        "my-workflow",
		Description: "test workflow",
		Vars:        map[string]string{"dir": "/tmp"},
		Triggers:    []Trigger{{Cron: "0 8 * * *", TZ: "UTC"}},
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "hello", OutputKey: "result"},
		},
		Config:    WorkflowConfig{FailureStrategy: "stop"},
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := store.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow() error: %v", err)
	}

	workflows, err := store.LoadAllWorkflows()
	if err != nil {
		t.Fatalf("LoadAllWorkflows() error: %v", err)
	}
	if len(workflows) != 1 {
		t.Fatalf("len(workflows) = %d, want 1", len(workflows))
	}

	loaded, ok := workflows["my-workflow"]
	if !ok {
		t.Fatal("workflow 'my-workflow' not found")
	}
	if loaded.Name != "my-workflow" {
		t.Fatalf("Name = %q, want %q", loaded.Name, "my-workflow")
	}
	if loaded.Description != "test workflow" {
		t.Fatalf("Description = %q, want %q", loaded.Description, "test workflow")
	}
	if len(loaded.Steps) != 1 || loaded.Steps[0].ID != "s1" {
		t.Fatalf("Steps = %v, unexpected", loaded.Steps)
	}
	if loaded.Vars["dir"] != "/tmp" {
		t.Fatalf("Vars[dir] = %q, want %q", loaded.Vars["dir"], "/tmp")
	}
	if len(loaded.Triggers) != 1 || loaded.Triggers[0].Cron != "0 8 * * *" {
		t.Fatalf("Triggers = %v, unexpected", loaded.Triggers)
	}
}

func TestPersistStore_LoadAllWorkflows_Empty(t *testing.T) {
	store := setupTestStore(t)

	workflows, err := store.LoadAllWorkflows()
	if err != nil {
		t.Fatalf("LoadAllWorkflows() error: %v", err)
	}
	if len(workflows) != 0 {
		t.Fatalf("len(workflows) = %d, want 0", len(workflows))
	}
}

func TestPersistStore_LoadAllWorkflows_SkipsInvalid(t *testing.T) {
	store := setupTestStore(t)

	// 写一个无效的 YAML 文件
	invalidPath := filepath.Join(store.workflowsDir, "invalid.yml")
	if err := os.WriteFile(invalidPath, []byte("not: valid: yaml: [["), 0o644); err != nil {
		t.Fatalf("write invalid file error: %v", err)
	}

	// 写一个非 YAML 文件（应被跳过）
	txtPath := filepath.Join(store.workflowsDir, "readme.txt")
	if err := os.WriteFile(txtPath, []byte("not a yaml"), 0o644); err != nil {
		t.Fatalf("write txt file error: %v", err)
	}

	// 写一个有效的工作流
	wf := &Workflow{Name: "valid-wf", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	if err := store.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow() error: %v", err)
	}

	workflows, err := store.LoadAllWorkflows()
	if err != nil {
		t.Fatalf("LoadAllWorkflows() error: %v", err)
	}
	if len(workflows) != 1 {
		t.Fatalf("len(workflows) = %d, want 1 (invalid skipped)", len(workflows))
	}
	if _, ok := workflows["valid-wf"]; !ok {
		t.Fatal("valid-wf not found")
	}
}

func TestPersistStore_DeleteWorkflow(t *testing.T) {
	store := setupTestStore(t)

	wf := &Workflow{Name: "to-delete", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	store.SaveWorkflow(wf)

	if err := store.DeleteWorkflow("to-delete"); err != nil {
		t.Fatalf("DeleteWorkflow() error: %v", err)
	}

	workflows, _ := store.LoadAllWorkflows()
	if _, ok := workflows["to-delete"]; ok {
		t.Fatal("workflow should be deleted")
	}

	// 删除不存在的 workflow 不应报错
	if err := store.DeleteWorkflow("nonexistent"); err != nil {
		t.Fatalf("DeleteWorkflow(nonexistent) error: %v", err)
	}
}

func TestPersistStore_SetEnabled(t *testing.T) {
	store := setupTestStore(t)

	wf := &Workflow{Name: "toggle-wf", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	store.SaveWorkflow(wf)

	// 默认启用
	workflows, _ := store.LoadAllWorkflows()
	if !workflows["toggle-wf"].Enabled {
		t.Fatal("workflow should be enabled by default")
	}

	// 禁用
	if err := store.SetEnabled("toggle-wf", false); err != nil {
		t.Fatalf("SetEnabled(false) error: %v", err)
	}
	workflows, _ = store.LoadAllWorkflows()
	if workflows["toggle-wf"].Enabled {
		t.Fatal("workflow should be disabled")
	}

	// 重新启用
	if err := store.SetEnabled("toggle-wf", true); err != nil {
		t.Fatalf("SetEnabled(true) error: %v", err)
	}
	workflows, _ = store.LoadAllWorkflows()
	if !workflows["toggle-wf"].Enabled {
		t.Fatal("workflow should be enabled again")
	}
}

func TestPersistStore_SaveAndLoadInstance(t *testing.T) {
	store := setupTestStore(t)

	now := time.Now()
	inst := &WorkflowInstance{
		ID:           "test-inst-001",
		WorkflowName: "test-wf",
		Status:       StatusCompleted,
		StepStates: map[string]*StepState{
			"s1": {Status: StatusCompleted, Attempts: 1},
		},
		StepOutputs: map[string]map[string]any{
			"s1": {"result": "hello world"},
		},
		TriggerType:    "manual",
		NotifyChannels: []NotifyTarget{{Channel: "telegram", ChatID: "-100123"}},
		Logs: []LogEntry{
			{Timestamp: now, StepID: "s1", Level: "info", Message: "step completed"},
		},
		StartedAt:  now,
		FinishedAt: &now,
	}

	if err := store.SaveInstance(inst); err != nil {
		t.Fatalf("SaveInstance() error: %v", err)
	}

	loaded, err := store.LoadInstance("test-wf", "test-inst-001")
	if err != nil {
		t.Fatalf("LoadInstance() error: %v", err)
	}
	if loaded.ID != "test-inst-001" {
		t.Fatalf("ID = %q, want %q", loaded.ID, "test-inst-001")
	}
	if loaded.WorkflowName != "test-wf" {
		t.Fatalf("WorkflowName = %q, want %q", loaded.WorkflowName, "test-wf")
	}
	if loaded.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q", loaded.Status, StatusCompleted)
	}
	if len(loaded.NotifyChannels) != 1 || loaded.NotifyChannels[0].Channel != "telegram" {
		t.Fatalf("NotifyChannels = %v, want telegram", loaded.NotifyChannels)
	}
	if loaded.StepStates["s1"].Status != StatusCompleted {
		t.Fatalf("StepStates[s1].Status = %q, want completed", loaded.StepStates["s1"].Status)
	}
	if loaded.StepOutputs["s1"]["result"] != "hello world" {
		t.Fatalf("StepOutputs[s1][result] = %v, want 'hello world'", loaded.StepOutputs["s1"]["result"])
	}
	if len(loaded.Logs) != 1 {
		t.Fatalf("len(Logs) = %d, want 1", len(loaded.Logs))
	}
}

func TestPersistStore_LoadInstance_NotFound(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.LoadInstance("nonexistent", "no-id")
	if err == nil {
		t.Fatal("expected error for nonexistent instance")
	}
}

func TestPersistStore_LoadInstances(t *testing.T) {
	store := setupTestStore(t)

	// 保存多个实例
	for i, status := range []string{StatusCompleted, StatusFailed, StatusRunning} {
		inst := &WorkflowInstance{
			ID:           fmt.Sprintf("inst-%d", i),
			WorkflowName: "multi-wf",
			Status:       status,
			StartedAt:    time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := store.SaveInstance(inst); err != nil {
			t.Fatalf("SaveInstance() error: %v", err)
		}
	}

	instances, err := store.LoadInstances("multi-wf")
	if err != nil {
		t.Fatalf("LoadInstances() error: %v", err)
	}
	if len(instances) != 3 {
		t.Fatalf("len(instances) = %d, want 3", len(instances))
	}

	// 验证按 started_at 倒序排列（最新的在前）
	if instances[0].ID == instances[1].ID {
		t.Fatal("instances should be sorted by started_at descending")
	}

	// 验证返回的是摘要格式（不包含大字段）
	for _, summary := range instances {
		if summary.ID == "" {
			t.Error("summary ID should not be empty")
		}
		if summary.WorkflowName != "multi-wf" {
			t.Errorf("summary workflow_name = %q, want %q", summary.WorkflowName, "multi-wf")
		}
	}
}

func TestPersistStore_DeleteInstance(t *testing.T) {
	store := setupTestStore(t)

	inst := &WorkflowInstance{
		ID:           "del-inst",
		WorkflowName: "del-wf",
		Status:       StatusCompleted,
		StartedAt:    time.Now(),
	}
	store.SaveInstance(inst)

	if err := store.DeleteInstance("del-wf", "del-inst"); err != nil {
		t.Fatalf("DeleteInstance() error: %v", err)
	}

	_, err := store.LoadInstance("del-wf", "del-inst")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestPersistStore_PurgeOldInstances(t *testing.T) {
	store := setupTestStore(t)

	// 创建 5 个实例
	for i := range 5 {
		inst := &WorkflowInstance{
			ID:           fmt.Sprintf("purge-inst-%d", i),
			WorkflowName: "purge-wf",
			Status:       StatusCompleted,
			StartedAt:    time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := store.SaveInstance(inst); err != nil {
			t.Fatalf("SaveInstance() error: %v", err)
		}
	}

	// 只保留 2 个
	if err := store.PurgeOldInstances("purge-wf", 2); err != nil {
		t.Fatalf("PurgeOldInstances() error: %v", err)
	}

	instances, _ := store.LoadInstances("purge-wf")
	if len(instances) != 2 {
		t.Fatalf("len(instances) after purge = %d, want 2", len(instances))
	}
	// 应保留最新的 2 个（LoadInstances 返回倒序，所以 [0] 是最新的）
	if instances[0].ID != "purge-inst-4" || instances[1].ID != "purge-inst-3" {
		t.Errorf("purge kept wrong instances: got [%s, %s], want [purge-inst-4, purge-inst-3]",
			instances[0].ID, instances[1].ID)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello-world", "hello-world"},
		{"Hello World", "Hello-World"},
		{"my_workflow", "my_workflow"},
		{"Test123", "Test123"},
		{"special!@#chars", "specialchars"},
		{"", "unnamed"},
		{"UPPERCASE", "UPPERCASE"},
		{"a b c", "a-b-c"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Fatalf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
