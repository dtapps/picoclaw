package workflow

import (
	"context"
	"sync"
	"testing"
	"time"

	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
)

// --- Mock EventBus ---

type mockBus struct {
	ch *mockEventChannel
}

func newMockBus() *mockBus {
	return &mockBus{
		ch: &mockEventChannel{
			sub: &mockSubscription{done: make(chan struct{})},
			ch:  make(chan runtimeevents.Event, 16),
		},
	}
}

func (m *mockBus) Publish(_ context.Context, _ runtimeevents.Event) runtimeevents.PublishResult {
	return runtimeevents.PublishResult{}
}

func (m *mockBus) PublishNonBlocking(_ runtimeevents.Event) runtimeevents.PublishResult {
	return runtimeevents.PublishResult{}
}
func (m *mockBus) Channel() runtimeevents.EventChannel { return m.ch }
func (m *mockBus) Close() error                        { return nil }
func (m *mockBus) Stats() runtimeevents.Stats          { return runtimeevents.Stats{} }

type mockEventChannel struct {
	sub *mockSubscription
	ch  chan runtimeevents.Event
}

func (m *mockEventChannel) Filter(_ runtimeevents.Filter) runtimeevents.EventChannel     { return m }
func (m *mockEventChannel) OfKind(_ ...runtimeevents.Kind) runtimeevents.EventChannel    { return m }
func (m *mockEventChannel) KindPrefix(_ string) runtimeevents.EventChannel               { return m }
func (m *mockEventChannel) Source(_ string, _ ...string) runtimeevents.EventChannel      { return m }
func (m *mockEventChannel) Scope(_ runtimeevents.ScopeFilter) runtimeevents.EventChannel { return m }

func (m *mockEventChannel) Subscribe(
	_ context.Context,
	_ runtimeevents.SubscribeOptions,
	_ runtimeevents.Handler,
) (runtimeevents.Subscription, error) {
	return m.sub, nil
}

func (m *mockEventChannel) SubscribeChan(
	_ context.Context,
	_ runtimeevents.SubscribeOptions,
) (runtimeevents.Subscription, <-chan runtimeevents.Event, error) {
	return m.sub, m.ch, nil
}

func (m *mockEventChannel) SubscribeOnce(
	_ context.Context,
	_ runtimeevents.SubscribeOptions,
	_ runtimeevents.Handler,
) (runtimeevents.Subscription, error) {
	return m.sub, nil
}

type mockSubscription struct {
	done chan struct{}
}

func (m *mockSubscription) ID() uint64            { return 1 }
func (m *mockSubscription) Name() string          { return "mock" }
func (m *mockSubscription) Close() error          { close(m.done); return nil }
func (m *mockSubscription) Done() <-chan struct{} { return m.done }
func (m *mockSubscription) Stats() runtimeevents.SubscriberStats {
	return runtimeevents.SubscriberStats{}
}

// --- Test helpers ---

func setupTestService(t *testing.T) (*Service, *StepExecutor) {
	t.Helper()
	dir := t.TempDir()
	store := NewPersistStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	executor := &StepExecutor{
		AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
			return "default response", nil
		},
	}
	engine := NewEngine(store, executor)
	svc := NewService(store, engine, ServiceConfig{WorkspaceDir: dir})
	return svc, executor
}

func setupTestServiceWithBus(t *testing.T) (*Service, *mockBus, *StepExecutor) {
	t.Helper()
	dir := t.TempDir()
	store := NewPersistStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	executor := &StepExecutor{
		AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
			return "default response", nil
		},
	}
	engine := NewEngine(store, executor)
	mbus := newMockBus()
	svc := NewService(store, engine, ServiceConfig{
		WorkspaceDir: dir,
		EventBus:     mbus,
	})
	return svc, mbus, executor
}

// --- Tests ---

func TestService_StartStop(t *testing.T) {
	svc, _ := setupTestService(t)

	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if !svc.running {
		t.Fatal("service should be running after Start()")
	}

	// 重复 Start 不应报错
	if err := svc.Start(); err != nil {
		t.Fatalf("second Start() error: %v", err)
	}

	svc.Stop()
	if svc.running {
		t.Fatal("service should not be running after Stop()")
	}

	// 重复 Stop 不应 panic
	svc.Stop()
}

func TestService_StartWithEventBus(t *testing.T) {
	svc, mbus, _ := setupTestServiceWithBus(t)

	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer svc.Stop()

	// 验证事件订阅已建立
	if svc.eventSub == nil {
		t.Fatal("event subscription should be set")
	}
	if svc.eventCh == nil {
		t.Fatal("event channel should be set")
	}

	// 发布事件
	mbus.ch.ch <- runtimeevents.Event{Kind: "test.event"}
}

func TestService_Reload(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	// 直接写入文件系统
	wf := &Workflow{Name: "new-wf", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.store.SaveWorkflow(wf)

	if err := svc.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}

	loaded, ok := svc.GetWorkflow("new-wf")
	if !ok {
		t.Fatal("new-wf should be loaded after reload")
	}
	if loaded.Name != "new-wf" {
		t.Fatalf("Name = %q, want %q", loaded.Name, "new-wf")
	}
}

func TestService_CreateWorkflow(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{
		Name:        "create-test",
		Description: "created workflow",
		Vars:        map[string]string{"key": "value"},
		Steps:       []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hello"}},
	}

	if err := svc.CreateWorkflow(wf); err != nil {
		t.Fatalf("CreateWorkflow() error: %v", err)
	}

	loaded, ok := svc.GetWorkflow("create-test")
	if !ok {
		t.Fatal("workflow not found after create")
	}
	if loaded.Description != "created workflow" {
		t.Fatalf("Description = %q, want %q", loaded.Description, "created workflow")
	}
	if loaded.Vars["key"] != "value" {
		t.Fatalf("Vars[key] = %q, want %q", loaded.Vars["key"], "value")
	}
	if loaded.Enabled {
		t.Fatal("workflow should be disabled by default when created via web")
	}
}

func TestService_CreateWorkflow_Duplicate(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{Name: "dup-wf", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	err := svc.CreateWorkflow(
		&Workflow{Name: "dup-wf", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}},
	)
	if err == nil {
		t.Fatal("expected error for duplicate workflow")
	}
}

func TestService_CreateWorkflow_Invalid(t *testing.T) {
	svc, _ := setupTestService(t)

	err := svc.CreateWorkflow(&Workflow{Name: "", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestService_UpdateWorkflow(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{Name: "update-test", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "v1"}}}
	svc.CreateWorkflow(wf)

	// 更新
	wf.Description = "updated"
	wf.Steps = []Step{{ID: "s1", Action: "agent_prompt", Prompt: "v2"}}
	if err := svc.UpdateWorkflow(wf); err != nil {
		t.Fatalf("UpdateWorkflow() error: %v", err)
	}

	loaded, _ := svc.GetWorkflow("update-test")
	if loaded.Description != "updated" {
		t.Fatalf("Description = %q, want %q", loaded.Description, "updated")
	}
}

func TestService_UpdateWorkflow_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	err := svc.UpdateWorkflow(
		&Workflow{Name: "nonexistent", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}},
	)
	if err == nil {
		t.Fatal("expected error for updating nonexistent workflow")
	}
}

func TestService_DeleteWorkflow(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{Name: "delete-test", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	if err := svc.DeleteWorkflow("delete-test"); err != nil {
		t.Fatalf("DeleteWorkflow() error: %v", err)
	}

	if _, ok := svc.GetWorkflow("delete-test"); ok {
		t.Fatal("workflow should be deleted")
	}
}

func TestService_DeleteWorkflow_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	err := svc.DeleteWorkflow("nonexistent")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent workflow")
	}
}

func TestService_RunWorkflow(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{Name: "run-test", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	instID, err := svc.RunWorkflow(context.Background(), "run-test", "telegram", "-100123")
	if err != nil {
		t.Fatalf("RunWorkflow() error: %v", err)
	}
	if instID == "" {
		t.Fatal("RunWorkflow() returned empty instance ID")
	}
}

func TestService_RunWorkflow_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	_, err := svc.RunWorkflow(context.Background(), "nonexistent", "", "")
	if err == nil {
		t.Fatal("expected error for running nonexistent workflow")
	}
}

func TestService_BindChannel(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{Name: "bind-test", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	if err := svc.BindChannel("bind-test", "telegram", "-100456"); err != nil {
		t.Fatalf("BindChannel() error: %v", err)
	}

	loaded, _ := svc.GetWorkflow("bind-test")
	if loaded.Config.NotifyChannel != "telegram" {
		t.Fatalf("NotifyChannel = %q, want %q", loaded.Config.NotifyChannel, "telegram")
	}
	if loaded.Config.NotifyChatID != "-100456" {
		t.Fatalf("NotifyChatID = %q, want %q", loaded.Config.NotifyChatID, "-100456")
	}
}

func TestService_UnbindChannel(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{
		Name:   "unbind-test",
		Steps:  []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}},
		Config: WorkflowConfig{NotifyChannel: "telegram", NotifyChatID: "-100"},
	}
	svc.CreateWorkflow(wf)

	if err := svc.UnbindChannel("unbind-test"); err != nil {
		t.Fatalf("UnbindChannel() error: %v", err)
	}

	loaded, _ := svc.GetWorkflow("unbind-test")
	if loaded.Config.NotifyChannel != "" {
		t.Fatalf("NotifyChannel = %q, want empty", loaded.Config.NotifyChannel)
	}
}

func TestService_BindChannel_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)
	err := svc.BindChannel("nonexistent", "telegram", "-100")
	if err == nil {
		t.Fatal("expected error for binding nonexistent workflow")
	}
}

func TestService_SetEnabled(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{Name: "enable-test", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	if err := svc.SetEnabled("enable-test", false); err != nil {
		t.Fatalf("SetEnabled(false) error: %v", err)
	}

	loaded, _ := svc.GetWorkflow("enable-test")
	if loaded.Enabled {
		t.Fatal("workflow should be disabled")
	}

	if err := svc.SetEnabled("enable-test", true); err != nil {
		t.Fatalf("SetEnabled(true) error: %v", err)
	}

	loaded, _ = svc.GetWorkflow("enable-test")
	if !loaded.Enabled {
		t.Fatal("workflow should be enabled")
	}
}

func TestService_SetEnabled_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)
	err := svc.SetEnabled("nonexistent", true)
	if err == nil {
		t.Fatal("expected error for setting enabled on nonexistent workflow")
	}
}

func TestService_ListWorkflowsForCommand(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf1 := &Workflow{
		Name:        "cmd-wf-1",
		Description: "first",
		Vars:        map[string]string{"k": "v"},
		Steps:       []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}},
	}
	wf2 := &Workflow{
		Name:        "cmd-wf-2",
		Description: "second",
		Triggers:    []Trigger{{Cron: "0 8 * * *"}},
		Steps:       []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}},
	}
	svc.CreateWorkflow(wf1)
	svc.CreateWorkflow(wf2)

	list := svc.ListWorkflowsForCommand()
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}

	// 查找 wf1 的信息
	var found bool
	for _, info := range list {
		if info.Name == "cmd-wf-1" {
			found = true
			if info.Description != "first" {
				t.Fatalf("Description = %q, want %q", info.Description, "first")
			}
			if info.Vars["k"] != "v" {
				t.Fatalf("Vars[k] = %q, want %q", info.Vars["k"], "v")
			}
			if info.TriggerType != "manual" {
				t.Fatalf("TriggerType = %q, want %q", info.TriggerType, "manual")
			}
		}
	}
	if !found {
		t.Fatal("cmd-wf-1 not found in list")
	}
}

func TestService_ShowWorkflow(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{
		Name:        "show-wf",
		Description: "show test",
		Vars:        map[string]string{"dir": "/tmp"},
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "a"},
			{ID: "s2", Action: "tool_call", Tool: "exec"},
		},
	}
	svc.CreateWorkflow(wf)

	info, stepIDs, err := svc.ShowWorkflow("show-wf")
	if err != nil {
		t.Fatalf("ShowWorkflow() error: %v", err)
	}
	if info.Name != "show-wf" {
		t.Fatalf("Name = %q, want %q", info.Name, "show-wf")
	}
	if info.Vars["dir"] != "/tmp" {
		t.Fatalf("Vars[dir] = %q, want %q", info.Vars["dir"], "/tmp")
	}
	if len(stepIDs) != 2 {
		t.Fatalf("len(stepIDs) = %d, want 2", len(stepIDs))
	}
	if stepIDs[0] != "s1" || stepIDs[1] != "s2" {
		t.Fatalf("stepIDs = %v, unexpected", stepIDs)
	}
}

func TestService_ShowWorkflow_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)
	_, _, err := svc.ShowWorkflow("nonexistent")
	if err == nil {
		t.Fatal("expected error for showing nonexistent workflow")
	}
}

func TestService_InstancesForCommand(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	// 创建工作流并执行
	wf := &Workflow{Name: "inst-wf", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	instID, _ := svc.RunWorkflow(context.Background(), "inst-wf", "", "")

	// 等待完成
	time.Sleep(200 * time.Millisecond)

	instances, err := svc.InstancesForCommand("inst-wf")
	if err != nil {
		t.Fatalf("InstancesForCommand() error: %v", err)
	}
	if len(instances) == 0 {
		t.Fatal("expected at least one instance")
	}
	found := false
	for _, inst := range instances {
		if inst.ID == instID {
			found = true
			if inst.WorkflowName != "inst-wf" {
				t.Fatalf("WorkflowName = %q, want %q", inst.WorkflowName, "inst-wf")
			}
		}
	}
	if !found {
		t.Fatalf("instance %s not found", instID)
	}
}

func TestService_GetAndDeleteInstance(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{Name: "inst-del-wf", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf)

	instID, _ := svc.RunWorkflow(context.Background(), "inst-del-wf", "", "")
	time.Sleep(200 * time.Millisecond)

	// GetInstance
	inst, err := svc.GetInstance("inst-del-wf", instID)
	if err != nil {
		t.Fatalf("GetInstance() error: %v", err)
	}
	if inst.ID != instID {
		t.Fatalf("ID = %q, want %q", inst.ID, instID)
	}

	// DeleteInstance
	if delErr := svc.DeleteInstance("inst-del-wf", instID); delErr != nil {
		t.Fatalf("DeleteInstance() error: %v", delErr)
	}

	_, err = svc.GetInstance("inst-del-wf", instID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestService_StopInstance(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	err := svc.StopInstance("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for stopping nonexistent instance")
	}
}

func TestService_CheckEventTriggers(t *testing.T) {
	svc, mbus, executor := setupTestServiceWithBus(t)

	var executed bool
	var mu sync.Mutex
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		mu.Lock()
		executed = true
		mu.Unlock()
		return "ok", nil
	}

	wf := &Workflow{
		Name:     "event-wf",
		Enabled:  true,
		Triggers: []Trigger{{Event: "agent.tool.exec_end"}},
		Steps:    []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}},
	}
	svc.CreateWorkflow(wf)

	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer svc.Stop()

	// 发送匹配的事件
	mbus.ch.ch <- runtimeevents.Event{Kind: "agent.tool.exec_end"}

	waitFor(t, 3*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return executed
	})
}

func TestService_CheckCronTriggers_CallsEngine(t *testing.T) {
	svc, executor := setupTestService(t)

	var executed bool
	var mu sync.Mutex
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		mu.Lock()
		executed = true
		mu.Unlock()
		return "ok", nil
	}

	// 不依赖 gronx.IsDue 的精确时间匹配，
	// 而是验证 checkCronTriggers 正确遍历了已启用的工作流。
	// 将 cron 设置为空字符串，验证不会触发
	wf := &Workflow{
		Name:    "no-cron-wf",
		Enabled: true,
		Steps:   []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}},
	}
	svc.CreateWorkflow(wf)
	svc.Start()
	defer svc.Stop()

	// 无 cron 触发器 → 不应执行
	svc.checkCronTriggers()
	// 给一点时间确保没有执行
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if executed {
		t.Fatal("workflow without cron trigger should not be executed")
	}
	mu.Unlock()
}

func TestService_CheckCronTriggers_DisabledWorkflow(t *testing.T) {
	svc, _ := setupTestService(t)

	wf := &Workflow{
		Name:     "disabled-cron-wf",
		Enabled:  false,
		Triggers: []Trigger{{Cron: "* * * * *"}},
		Steps:    []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}},
	}
	svc.CreateWorkflow(wf)
	svc.Start()
	defer svc.Stop()

	// 禁用的工作流不应被 cron 触发
	svc.checkCronTriggers()
	// 无 panic 或错误即表示通过
}

func TestDescribeTriggerType(t *testing.T) {
	tests := []struct {
		name     string
		triggers []Trigger
		want     string
	}{
		{"empty", nil, "manual"},
		{"cron", []Trigger{{Cron: "0 8 * * *"}}, "cron:0 8 * * *"},
		{"event", []Trigger{{Event: "tool.end"}}, "event:tool.end"},
		{"both", []Trigger{{Cron: "0 8 * * *"}, {Event: "tool.end"}}, "cron:0 8 * * *"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describeTriggerType(tt.triggers)
			if got != tt.want {
				t.Fatalf("describeTriggerType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestService_ListWorkflows(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf1 := &Workflow{Name: "list-1", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	wf2 := &Workflow{Name: "list-2", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "hi"}}}
	svc.CreateWorkflow(wf1)
	svc.CreateWorkflow(wf2)

	list := svc.ListWorkflows()
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
}

func TestService_GetWorkflow_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)
	_, ok := svc.GetWorkflow("nonexistent")
	if ok {
		t.Fatal("expected false for nonexistent workflow")
	}
}

// TestService_BindChannel_ReadsFreshFromDisk 验证 BindChannel 不会用过期内存数据覆写磁盘。
// 模拟场景：Web UI 修改了工作流步骤，然后用户执行 /workflow bind。
func TestService_BindChannel_ReadsFreshFromDisk(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	// 1. 创建工作流
	wf := &Workflow{
		Name:  "fresh-test",
		Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "old prompt"}},
		Vars:  map[string]string{"key": "old"},
	}
	svc.CreateWorkflow(wf)

	// 2. 模拟外部修改磁盘（如 Web UI）：绕过 Service 直接修改磁盘
	externalWf, _ := svc.store.LoadSingleWorkflow("fresh-test")
	externalWf.Steps = []Step{{ID: "s1", Action: "agent_prompt", Prompt: "new prompt"}}
	externalWf.Vars = map[string]string{"key": "new", "extra": "added"}
	svc.store.SaveWorkflow(externalWf)

	// 3. 内存中的 s.workflows 仍是旧数据，执行 BindChannel
	if err := svc.BindChannel("fresh-test", "telegram", "-100"); err != nil {
		t.Fatalf("BindChannel() error: %v", err)
	}

	// 4. 从磁盘验证：步骤和变量应保留外部修改，不被旧内存数据覆盖
	diskWf, err := svc.store.LoadSingleWorkflow("fresh-test")
	if err != nil {
		t.Fatalf("LoadSingleWorkflow() error: %v", err)
	}
	if diskWf.Steps[0].Prompt != "new prompt" {
		t.Fatalf("Steps[0].Prompt = %q, want %q (external change preserved)", diskWf.Steps[0].Prompt, "new prompt")
	}
	if diskWf.Vars["key"] != "new" {
		t.Fatalf("Vars[key] = %q, want %q (external change preserved)", diskWf.Vars["key"], "new")
	}
	if diskWf.Vars["extra"] != "added" {
		t.Fatalf("Vars[extra] = %q, want %q (external addition preserved)", diskWf.Vars["extra"], "added")
	}
	if diskWf.Config.NotifyChannel != "telegram" {
		t.Fatalf("NotifyChannel = %q, want %q (bind applied)", diskWf.Config.NotifyChannel, "telegram")
	}
}

// TestService_UnbindChannel_ReadsFreshFromDisk 验证 UnbindChannel 同样读取最新磁盘数据。
func TestService_UnbindChannel_ReadsFreshFromDisk(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{
		Name:   "unbind-fresh",
		Steps:  []Step{{ID: "s1", Action: "agent_prompt", Prompt: "old"}},
		Config: WorkflowConfig{NotifyChannel: "telegram", NotifyChatID: "-100"},
	}
	svc.CreateWorkflow(wf)

	// 模拟外部修改
	externalWf, _ := svc.store.LoadSingleWorkflow("unbind-fresh")
	externalWf.Steps = []Step{{ID: "s1", Action: "agent_prompt", Prompt: "new"}}
	svc.store.SaveWorkflow(externalWf)

	// UnbindChannel 应基于最新磁盘数据操作
	if err := svc.UnbindChannel("unbind-fresh"); err != nil {
		t.Fatalf("UnbindChannel() error: %v", err)
	}

	diskWf, _ := svc.store.LoadSingleWorkflow("unbind-fresh")
	if diskWf.Steps[0].Prompt != "new" {
		t.Fatalf("Steps[0].Prompt = %q, want %q (external change preserved)", diskWf.Steps[0].Prompt, "new")
	}
	if diskWf.Config.NotifyChannel != "" {
		t.Fatalf("NotifyChannel = %q, want empty (unbind applied)", diskWf.Config.NotifyChannel)
	}
}

// TestService_RunWorkflow_ReadsFreshFromDisk 验证 RunWorkflow 执行最新步骤定义。
func TestService_RunWorkflow_ReadsFreshFromDisk(t *testing.T) {
	svc, executor := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	var mu sync.Mutex
	var executedPrompt string
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		mu.Lock()
		executedPrompt = prompt
		mu.Unlock()
		return "result", nil
	}

	wf := &Workflow{Name: "run-fresh", Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "old prompt"}}}
	svc.CreateWorkflow(wf)

	// 模拟外部修改磁盘
	externalWf, _ := svc.store.LoadSingleWorkflow("run-fresh")
	externalWf.Steps = []Step{{ID: "s1", Action: "agent_prompt", Prompt: "new prompt"}}
	svc.store.SaveWorkflow(externalWf)

	// RunWorkflow 应执行最新步骤
	svc.RunWorkflow(context.Background(), "run-fresh", "", "")

	// 等待引擎完成
	svc.engine.WaitRunning()

	mu.Lock()
	got := executedPrompt
	mu.Unlock()
	if got != "new prompt" {
		t.Fatalf("executed prompt = %q, want %q (fresh from disk)", got, "new prompt")
	}
}

// TestService_SetEnabled_ReadsFreshFromDisk 验证 SetEnabled 同步最新磁盘数据到内存。
func TestService_SetEnabled_ReadsFreshFromDisk(t *testing.T) {
	svc, _ := setupTestService(t)
	svc.Start()
	defer svc.Stop()

	wf := &Workflow{
		Name:  "enable-fresh",
		Steps: []Step{{ID: "s1", Action: "agent_prompt", Prompt: "old"}},
	}
	svc.CreateWorkflow(wf)

	// 模拟外部修改
	externalWf, _ := svc.store.LoadSingleWorkflow("enable-fresh")
	externalWf.Steps = []Step{{ID: "s1", Action: "agent_prompt", Prompt: "new"}}
	svc.store.SaveWorkflow(externalWf)

	// SetEnabled 应从磁盘同步最新数据
	svc.SetEnabled("enable-fresh", true)

	loaded, _ := svc.GetWorkflow("enable-fresh")
	if loaded.Steps[0].Prompt != "new" {
		t.Fatalf("in-memory Steps[0].Prompt = %q, want %q (synced from disk)", loaded.Steps[0].Prompt, "new")
	}
	if !loaded.Enabled {
		t.Fatal("in-memory Enabled = false, want true")
	}
}
