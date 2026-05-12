package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// setupTestEngine 创建用于测试的引擎，使用临时目录作为持久化存储。
func setupTestEngine(t *testing.T) (*Engine, *StepExecutor) {
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
	return engine, executor
}

func TestRunWorkflow_BasicExecution(t *testing.T) {
	engine, _ := setupTestEngine(t)

	wf := &Workflow{
		Name: "test-wf",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "step1", OutputKey: "result"},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	instID, err := engine.RunWorkflow(context.Background(), wf, "manual", "", "")
	if err != nil {
		t.Fatalf("RunWorkflow() error: %v", err)
	}
	if instID == "" {
		t.Fatal("RunWorkflow() returned empty instance ID")
	}

	// 等待异步执行完成
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	inst, err := engine.store.LoadInstance("test-wf", instID)
	if err != nil {
		t.Fatalf("LoadInstance() error: %v", err)
	}
	if inst.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q", inst.Status, StatusCompleted)
	}
	if inst.StepStates["s1"].Status != StatusCompleted {
		t.Fatalf("StepStates[s1].Status = %q, want %q", inst.StepStates["s1"].Status, StatusCompleted)
	}
}

func TestRunWorkflow_WithVars(t *testing.T) {
	engine, executor := setupTestEngine(t)

	var receivedPrompt string
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		receivedPrompt = prompt
		return "ok", nil
	}

	wf := &Workflow{
		Name: "vars-wf",
		Vars: map[string]string{"dir": "/tmp/work", "site": "https://example.com"},
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "Save to {{.vars.dir}} from {{.vars.site}}"},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	_, err := engine.RunWorkflow(context.Background(), wf, "manual", "", "")
	if err != nil {
		t.Fatalf("RunWorkflow() error: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	if receivedPrompt != "Save to /tmp/work from https://example.com" {
		t.Fatalf("receivedPrompt = %q, want %q", receivedPrompt, "Save to /tmp/work from https://example.com")
	}
}

func TestRunWorkflow_ConditionalSkip(t *testing.T) {
	engine, _ := setupTestEngine(t)

	wf := &Workflow{
		Name: "skip-wf",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "step1"},
			{ID: "s2", Action: "agent_prompt", Prompt: "on_error step", When: "on_error"},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("skip-wf", instID)
	if inst.StepStates["s1"].Status != StatusCompleted {
		t.Fatalf("s1 status = %q, want completed", inst.StepStates["s1"].Status)
	}
	if inst.StepStates["s2"].Status != StatusSkipped {
		t.Fatalf("s2 status = %q, want skipped", inst.StepStates["s2"].Status)
	}
}

func TestRunWorkflow_OnErrorHandler(t *testing.T) {
	engine, executor := setupTestEngine(t)

	// 第一步失败，on_error 步骤应该执行
	var s1Attempts int
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		if prompt == "fail-step" {
			s1Attempts++
			return "", errors.New("intentional failure")
		}
		return "error handled", nil
	}

	wf := &Workflow{
		Name:   "error-wf",
		Config: WorkflowConfig{FailureStrategy: "stop"},
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "fail-step"},
			{ID: "s2", Action: "agent_prompt", Prompt: "error handler", When: "on_error", OutputKey: "err_msg"},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("error-wf", instID)
	if inst.StepStates["s1"].Status != StatusFailed {
		t.Fatalf("s1 status = %q, want failed", inst.StepStates["s1"].Status)
	}
	if inst.StepStates["s2"].Status != StatusCompleted {
		t.Fatalf("s2 status = %q, want completed", inst.StepStates["s2"].Status)
	}
	// on_error handler 执行后引擎继续运行，最终标记为 completed
	if inst.Status != StatusCompleted {
		t.Fatalf("instance status = %q, want completed", inst.Status)
	}
}

func TestRunWorkflow_FailureStrategyContinue(t *testing.T) {
	engine, executor := setupTestEngine(t)

	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		if prompt == "fail" {
			return "", errors.New("fail")
		}
		return "ok", nil
	}

	wf := &Workflow{
		Name:   "continue-wf",
		Config: WorkflowConfig{FailureStrategy: "continue"},
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "fail"},
			{ID: "s2", Action: "agent_prompt", Prompt: "after-fail", When: "on_error"},
			{ID: "s3", Action: "agent_prompt", Prompt: "always-run"},
		},
	}

	var completed bool
	var mu2 sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu2.Lock()
		completed = true
		mu2.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu2.Lock()
		defer mu2.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("continue-wf", instID)
	if inst.StepStates["s1"].Status != StatusFailed {
		t.Fatalf("s1 status = %q, want failed", inst.StepStates["s1"].Status)
	}
	// s2 有 when="on_error"，上一步失败所以条件满足
	if inst.StepStates["s2"].Status != StatusCompleted {
		t.Fatalf("s2 status = %q, want completed", inst.StepStates["s2"].Status)
	}
	// s3 无 when 条件（默认 on_success），上一步 s2 成功所以执行
	if inst.StepStates["s3"].Status != StatusCompleted {
		t.Fatalf("s3 status = %q, want completed", inst.StepStates["s3"].Status)
	}
	// 引擎当前不追踪历史失败步骤，所有步骤执行完毕后标记为 completed
	if inst.Status != StatusCompleted {
		t.Fatalf("instance status = %q, want completed", inst.Status)
	}
}

func TestRunWorkflow_IfStep(t *testing.T) {
	engine, _ := setupTestEngine(t)

	wf := &Workflow{
		Name: "if-wf",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "first"},
			{
				ID:     "check",
				Action: "if",
				When:   "on_error",
				IfTrue: []Step{
					{ID: "handle_err", Action: "agent_prompt", Prompt: "error path"},
				},
				IfFalse: []Step{
					{ID: "handle_ok", Action: "agent_prompt", Prompt: "ok path"},
				},
			},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("if-wf", instID)
	// s1 成功，所以 if on_error 走 if_false 分支
	if inst.StepStates["check"].Status != StatusCompleted {
		t.Fatalf("check status = %q, want completed", inst.StepStates["check"].Status)
	}
}

func TestRunWorkflow_Cancel(t *testing.T) {
	engine, executor := setupTestEngine(t)

	// 创建一个长时间运行的步骤
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		select {
		case <-time.After(10 * time.Second):
			return "done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	wf := &Workflow{
		Name: "cancel-wf",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "long task"},
		},
	}

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	// 等待步骤开始执行
	time.Sleep(50 * time.Millisecond)

	if !engine.IsRunning("cancel-wf") {
		t.Fatal("expected workflow to be running")
	}

	err := engine.StopInstance(instID)
	if err != nil {
		t.Fatalf("StopInstance() error: %v", err)
	}

	// 等待取消生效
	time.Sleep(100 * time.Millisecond)

	instances := engine.GetRunningInstances("cancel-wf")
	if len(instances) != 0 {
		t.Fatalf("GetRunningInstances() = %d, want 0", len(instances))
	}
}

func TestRunWorkflow_Callbacks(t *testing.T) {
	engine, _ := setupTestEngine(t)

	var startCalled, stepStartCalled, stepCompleteCalled, completeCalled bool
	var mu sync.Mutex

	engine.SetOnStart(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		startCalled = true
		mu.Unlock()
		return nil
	})
	engine.SetOnStepStart(func(step Step, inst *WorkflowInstance, resolvedPrompt string, resolvedArgs map[string]any) {
		mu.Lock()
		stepStartCalled = true
		mu.Unlock()
	})
	engine.SetOnStepComplete(func(step Step, inst *WorkflowInstance, result StepResult) {
		mu.Lock()
		stepCompleteCalled = true
		mu.Unlock()
	})
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completeCalled = true
		mu.Unlock()

		return nil
	})

	wf := &Workflow{
		Name: "callback-wf",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "hi"},
		},
	}

	_, err := engine.RunWorkflow(context.Background(), wf, "manual", "", "")
	if err != nil {
		t.Fatalf("RunWorkflow() error: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completeCalled
	})

	mu.Lock()
	defer mu.Unlock()
	if !startCalled {
		t.Fatal("onStart not called")
	}
	if !stepStartCalled {
		t.Fatal("onStepStart not called")
	}
	if !stepCompleteCalled {
		t.Fatal("onStepComplete not called")
	}
	if !completeCalled {
		t.Fatal("onComplete not called")
	}
}

func TestRunWorkflow_ChannelFromConfig(t *testing.T) {
	engine, _ := setupTestEngine(t)

	wf := &Workflow{
		Name: "channel-wf",
		Config: WorkflowConfig{
			NotifyChannel: "telegram",
			NotifyChatID:  "-100123",
		},
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "hi"},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("channel-wf", instID)
	if inst.Channel != "telegram" {
		t.Fatalf("Channel = %q, want %q", inst.Channel, "telegram")
	}
	if inst.ChatID != "-100123" {
		t.Fatalf("ChatID = %q, want %q", inst.ChatID, "-100123")
	}
}

func TestRunWorkflow_DataPassingBetweenSteps(t *testing.T) {
	engine, executor := setupTestEngine(t)

	var receivedPrompt string
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		if prompt == "step1" {
			return "sunny, 25°C", nil
		}
		receivedPrompt = prompt
		return "briefing generated", nil
	}

	wf := &Workflow{
		Name: "data-passing-wf",
		Steps: []Step{
			{ID: "fetch", Action: "agent_prompt", Prompt: "step1", OutputKey: "weather"},
			{ID: "summarize", Action: "agent_prompt", Prompt: "Weather: {{.fetch.weather}}, generate briefing"},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	if receivedPrompt != "Weather: sunny, 25°C, generate briefing" {
		t.Fatalf("receivedPrompt = %q, want %q", receivedPrompt, "Weather: sunny, 25°C, generate briefing")
	}

	inst, _ := engine.store.LoadInstance("data-passing-wf", instID)
	if inst.StepOutputs["fetch"]["weather"] != "sunny, 25°C" {
		t.Fatalf("StepOutputs[fetch][weather] = %v, want 'sunny, 25°C'", inst.StepOutputs["fetch"]["weather"])
	}
}

func TestRunWorkflow_IfStepOnSuccess(t *testing.T) {
	engine, executor := setupTestEngine(t)

	var executedSteps []string
	var mu sync.Mutex
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		mu.Lock()
		executedSteps = append(executedSteps, prompt)
		mu.Unlock()
		return "ok", nil
	}

	wf := &Workflow{
		Name: "if-success-wf",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "first"},
			{
				ID:     "check",
				Action: "if",
				When:   "on_success",
				IfTrue: []Step{
					{ID: "handle_ok", Action: "agent_prompt", Prompt: "success path"},
				},
				IfFalse: []Step{
					{ID: "handle_err", Action: "agent_prompt", Prompt: "error path"},
				},
			},
		},
	}

	var completed bool
	var mu2 sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu2.Lock()
		completed = true
		mu2.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu2.Lock()
		defer mu2.Unlock()
		return completed
	})

	mu.Lock()
	defer mu.Unlock()
	// s1 成功 → on_success → if_true 分支
	found := false
	for _, s := range executedSteps {
		if s == "success path" {
			found = true
		}
	}
	if !found {
		t.Fatalf("if_true branch not executed, got steps: %v", executedSteps)
	}

	inst, _ := engine.store.LoadInstance("if-success-wf", instID)
	if inst.StepStates["handle_ok"] == nil || inst.StepStates["handle_ok"].Status != StatusCompleted {
		t.Fatal("handle_ok step should be completed")
	}
	// if_false 分支的步骤应标记为 skipped（未执行的分支）
	if inst.StepStates["handle_err"] == nil || inst.StepStates["handle_err"].Status != StatusSkipped {
		t.Fatal("handle_err step should be skipped")
	}
}

func TestRunWorkflow_ParallelStep(t *testing.T) {
	engine, executor := setupTestEngine(t)

	var mu sync.Mutex
	var callOrder []string
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		mu.Lock()
		callOrder = append(callOrder, prompt)
		mu.Unlock()
		if prompt == "step1" {
			return "first result", nil
		}
		return "parallel result", nil
	}

	wf := &Workflow{
		Name: "parallel-wf",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "step1", OutputKey: "first"},
			{
				ID:     "s2",
				Action: "parallel",
				Parallel: []Step{
					{ID: "p1", Action: "agent_prompt", Prompt: "parallel-a", OutputKey: "a"},
					{ID: "p2", Action: "agent_prompt", Prompt: "parallel-b", OutputKey: "b"},
				},
			},
		},
	}

	var completed bool
	var mu2 sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu2.Lock()
		completed = true
		mu2.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu2.Lock()
		defer mu2.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("parallel-wf", instID)
	if inst.StepStates["s1"].Status != StatusCompleted {
		t.Fatalf("s1 status = %q, want completed", inst.StepStates["s1"].Status)
	}
	if inst.StepStates["s2"].Status != StatusCompleted {
		t.Fatalf("s2 status = %q, want completed", inst.StepStates["s2"].Status)
	}

	mu.Lock()
	defer mu.Unlock()
	// 确保所有步骤都被调用
	if len(callOrder) != 3 {
		t.Fatalf("callOrder = %v, expected 3 calls", callOrder)
	}
	if callOrder[0] != "step1" {
		t.Fatalf("first call = %q, want 'step1'", callOrder[0])
	}
}

func TestRunWorkflow_ParallelSubStepStateTracking(t *testing.T) {
	engine, _ := setupTestEngine(t)

	wf := &Workflow{
		Name: "parallel-track-wf",
		Steps: []Step{
			{
				ID:     "s1",
				Action: "parallel",
				Parallel: []Step{
					{ID: "p1", Action: "agent_prompt", Prompt: "parallel-a", OutputKey: "a"},
					{ID: "p2", Action: "agent_prompt", Prompt: "parallel-b", OutputKey: "b"},
				},
			},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("parallel-track-wf", instID)

	// 验证 parallel 父步骤有状态
	if inst.StepStates["s1"].Status != StatusCompleted {
		t.Fatalf("s1 status = %q, want completed", inst.StepStates["s1"].Status)
	}
	// 验证子步骤有独立的状态追踪
	if inst.StepStates["p1"] == nil || inst.StepStates["p1"].Status != StatusCompleted {
		t.Fatalf("p1 state = %v, want completed", inst.StepStates["p1"])
	}
	if inst.StepStates["p2"] == nil || inst.StepStates["p2"].Status != StatusCompleted {
		t.Fatalf("p2 state = %v, want completed", inst.StepStates["p2"])
	}
}

func TestRunWorkflow_VarsAndStepOutputCombined(t *testing.T) {
	engine, executor := setupTestEngine(t)

	var receivedPrompt string
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		if prompt == "fetch date" {
			return "2026-05-09", nil
		}
		receivedPrompt = prompt
		return "report done", nil
	}

	wf := &Workflow{
		Name: "combined-wf",
		Vars: map[string]string{"dir": "/tmp/reports"},
		Steps: []Step{
			{ID: "get_date", Action: "agent_prompt", Prompt: "fetch date", OutputKey: "date"},
			{ID: "report", Action: "agent_prompt", Prompt: "Date {{.get_date.date}}, save to {{.vars.dir}}/output.txt"},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	_, err := engine.RunWorkflow(context.Background(), wf, "manual", "", "")
	if err != nil {
		t.Fatalf("RunWorkflow() error: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	if receivedPrompt != "Date 2026-05-09, save to /tmp/reports/output.txt" {
		t.Fatalf("receivedPrompt = %q, want combined template resolution", receivedPrompt)
	}
}

func TestRunWorkflow_IfBranchSkippedState(t *testing.T) {
	engine, _ := setupTestEngine(t)

	wf := &Workflow{
		Name: "if-skip-wf",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "first"},
			{
				ID:     "check",
				Action: "if",
				When:   "on_success",
				IfTrue: []Step{
					{ID: "ok_step", Action: "agent_prompt", Prompt: "success path"},
				},
				IfFalse: []Step{
					{ID: "err_step", Action: "agent_prompt", Prompt: "error path"},
				},
			},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("if-skip-wf", instID)
	// s1 成功 → on_success → if_true 分支执行
	if inst.StepStates["ok_step"] == nil || inst.StepStates["ok_step"].Status != StatusCompleted {
		t.Fatalf("ok_step state = %v, want completed", inst.StepStates["ok_step"])
	}
	// if_false 分支的步骤应标记为 skipped
	if inst.StepStates["err_step"] == nil || inst.StepStates["err_step"].Status != StatusSkipped {
		t.Fatalf("err_step state = %v, want skipped", inst.StepStates["err_step"])
	}
}

func TestRunWorkflow_StepTimeout(t *testing.T) {
	engine, executor := setupTestEngine(t)

	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		select {
		case <-time.After(5 * time.Second):
			return "done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	wf := &Workflow{
		Name: "timeout-wf",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "slow task", Timeout: "50ms"},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 3*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("timeout-wf", instID)
	if inst.StepStates["s1"].Status != StatusFailed {
		t.Fatalf("s1 status = %q, want failed (timed out)", inst.StepStates["s1"].Status)
	}
}

func TestRunWorkflow_IfStepOnError(t *testing.T) {
	engine, executor := setupTestEngine(t)

	var executedSteps []string
	var mu sync.Mutex
	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		mu.Lock()
		executedSteps = append(executedSteps, prompt)
		mu.Unlock()
		if prompt == "fail" {
			return "", errors.New("intentional failure")
		}
		return "ok", nil
	}

	wf := &Workflow{
		Name: "if-error-wf",
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "fail"},
			{
				ID:     "check",
				Action: "if",
				When:   "on_error",
				IfTrue: []Step{
					{ID: "handle_err", Action: "agent_prompt", Prompt: "error path"},
				},
				IfFalse: []Step{
					{ID: "handle_ok", Action: "agent_prompt", Prompt: "ok path"},
				},
			},
		},
		Config: WorkflowConfig{FailureStrategy: "stop"},
	}

	var completed bool
	var mu2 sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu2.Lock()
		completed = true
		mu2.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu2.Lock()
		defer mu2.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("if-error-wf", instID)
	// s1 失败 → on_error → if 步骤执行 → if_true 分支
	if inst.StepStates["check"] == nil || inst.StepStates["check"].Status != StatusCompleted {
		t.Fatalf("check status = %v, want completed", inst.StepStates["check"])
	}
}

func TestRunWorkflow_OnErrorHandlerFails(t *testing.T) {
	engine, executor := setupTestEngine(t)

	executor.AgentPromptFunc = func(ctx context.Context, prompt string) (string, error) {
		return "", errors.New("always fail")
	}

	wf := &Workflow{
		Name:   "handler-fail-wf",
		Config: WorkflowConfig{FailureStrategy: "stop"},
		Steps: []Step{
			{ID: "s1", Action: "agent_prompt", Prompt: "step1"},
			{ID: "s2", Action: "agent_prompt", Prompt: "handler", When: "on_error"},
		},
	}

	var completed bool
	var mu sync.Mutex
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		mu.Unlock()

		return nil
	})

	instID, _ := engine.RunWorkflow(context.Background(), wf, "manual", "", "")

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	inst, _ := engine.store.LoadInstance("handler-fail-wf", instID)
	if inst.StepStates["s1"].Status != StatusFailed {
		t.Fatalf("s1 status = %q, want failed", inst.StepStates["s1"].Status)
	}
	// on_error handler 执行了但本身也失败
	if inst.StepStates["s2"].Status != StatusFailed {
		t.Fatalf("s2 status = %q, want failed (handler also failed)", inst.StepStates["s2"].Status)
	}
	// handler 也失败时，实例应标记为 failed 而非 completed
	if inst.Status != StatusFailed {
		t.Fatalf("instance status = %q, want failed (handler also failed)", inst.Status)
	}
}

func TestStopInstance_NotFound(t *testing.T) {
	engine, _ := setupTestEngine(t)

	err := engine.StopInstance("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for stopping nonexistent instance")
	}
}

func TestChannelFromCtx(t *testing.T) {
	t.Run("value present", func(t *testing.T) {
		ctx := withChannelCtx(context.Background(), "telegram", "chat-123")
		ch, ok := ChannelFromCtx(ctx)
		if !ok || ch != "telegram" {
			t.Fatalf("ChannelFromCtx() = (%q, %v), want (%q, true)", ch, ok, "telegram")
		}
		cid, ok := ChatIDFromCtx(ctx)
		if !ok || cid != "chat-123" {
			t.Fatalf("ChatIDFromCtx() = (%q, %v), want (%q, true)", cid, ok, "chat-123")
		}
	})

	t.Run("value absent", func(t *testing.T) {
		_, ok := ChannelFromCtx(context.Background())
		if ok {
			t.Fatal("ChannelFromCtx() should return false for empty context")
		}
	})
}

// waitFor 轮询等待条件满足，超时后 Fatal。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func TestStepStateNamePreserved(t *testing.T) {
	engine, _ := setupTestEngine(t)

	wf := &Workflow{
		Name: "name-test",
		Steps: []Step{
			{ID: "s1", Name: "第一步", Action: "agent_prompt", Prompt: "step1"},
			{ID: "s2", Name: "第二步", Action: "agent_prompt", Prompt: "step2", When: "on_error"},
		},
	}

	var completed bool
	var mu sync.Mutex
	var finalInst *WorkflowInstance
	engine.SetOnComplete(func(inst *WorkflowInstance) <-chan struct{} {
		mu.Lock()
		completed = true
		finalInst = inst
		mu.Unlock()

		return nil
	})

	_, err := engine.RunWorkflow(context.Background(), wf, "manual", "", "")
	if err != nil {
		t.Fatalf("RunWorkflow() error: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return completed
	})

	mu.Lock()
	inst := finalInst
	mu.Unlock()

	// s1 completed — Name should be preserved
	if ss, ok := inst.StepStates["s1"]; !ok {
		t.Fatal("missing StepState for s1")
	} else if ss.Name != "第一步" {
		t.Errorf("s1 Name = %q, want %q", ss.Name, "第一步")
	}

	// s2 skipped — Name should be preserved
	if ss, ok := inst.StepStates["s2"]; !ok {
		t.Fatal("missing StepState for s2")
	} else if ss.Name != "第二步" {
		t.Errorf("s2 Name = %q, want %q", ss.Name, "第二步")
	} else if ss.Status != StatusSkipped {
		t.Errorf("s2 Status = %q, want %q", ss.Status, StatusSkipped)
	}
}
