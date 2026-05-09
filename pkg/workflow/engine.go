package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

type contextKey string

const (
	channelCtxKey contextKey = "workflow_channel"
	chatIDCtxKey  contextKey = "workflow_chat_id"
)

// ChannelFromCtx 从上下文中提取工作流绑定的频道名称。
func ChannelFromCtx(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(channelCtxKey).(string)
	return v, ok
}

// ChatIDFromCtx 从上下文中提取工作流绑定的聊天 ID。
func ChatIDFromCtx(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(chatIDCtxKey).(string)
	return v, ok
}

// withChannelCtx 将频道信息注入上下文，供步骤执行器回调时读取。
func withChannelCtx(ctx context.Context, channel, chatID string) context.Context {
	ctx = context.WithValue(ctx, channelCtxKey, channel)
	ctx = context.WithValue(ctx, chatIDCtxKey, chatID)
	return ctx
}

// Engine 是工作流的核心编排引擎。
// 负责按步骤顺序驱动工作流执行，管理状态机、条件判断、
// 数据传递和错误处理。支持异步执行和取消。
type Engine struct {
	store          *PersistStore                                              // 持久化存储
	executor       *StepExecutor                                              // 步骤执行器
	onStart        func(inst *WorkflowInstance)                               // 执行开始回调（用于频道通知等）
	onStepStart    func(step Step, inst *WorkflowInstance)                    // 步骤开始回调
	onStepComplete func(step Step, inst *WorkflowInstance, result StepResult) // 步骤完成回调
	onComplete     func(inst *WorkflowInstance)                               // 执行完成回调（用于频道通知等）
	mu             sync.RWMutex                                               // 保护 running 和 cancelFuncs
	running        map[string]*WorkflowInstance                               // 当前运行中的实例（按实例 ID 索引）
	cancelFuncs    map[string]context.CancelFunc                              // 取消函数（按实例 ID 索引）
}

// NewEngine 创建新的工作流引擎。
func NewEngine(store *PersistStore, executor *StepExecutor) *Engine {
	return &Engine{
		store:       store,
		executor:    executor,
		running:     make(map[string]*WorkflowInstance),
		cancelFuncs: make(map[string]context.CancelFunc),
	}
}

// SetOnComplete 设置执行完成回调。
// 回调在工作流执行结束（成功/失败/取消）后调用，可用于频道通知等场景。
func (e *Engine) SetOnComplete(fn func(inst *WorkflowInstance)) {
	e.onComplete = fn
}

// SetOnStart 设置执行开始回调。
// 回调在工作流开始执行时调用，可用于发送开始通知等场景。
func (e *Engine) SetOnStart(fn func(inst *WorkflowInstance)) {
	e.onStart = fn
}

// SetOnStepStart 设置步骤开始回调。
// 回调在每个步骤开始执行时调用，可用于通知频道即将执行的步骤。
func (e *Engine) SetOnStepStart(fn func(step Step, inst *WorkflowInstance)) {
	e.onStepStart = fn
}

// SetOnStepComplete 设置步骤完成回调。
// 回调在每个步骤执行完成后调用，可用于将 AI 响应实时推送到频道。
func (e *Engine) SetOnStepComplete(fn func(step Step, inst *WorkflowInstance, result StepResult)) {
	e.onStepComplete = fn
}

// RunWorkflow 启动一次工作流执行。
// 创建 WorkflowInstance，初始化所有步骤状态为 pending，然后异步执行。
// channel 和 chatID 用于绑定频道，执行完成后通过回调通知该频道。
// 返回实例 ID。
func (e *Engine) RunWorkflow(ctx context.Context, wf *Workflow, triggerType, channel, chatID string) (string, error) {
	inst := &WorkflowInstance{
		ID:           generateInstanceID(),
		WorkflowName: wf.Name,
		Status:       StatusRunning,
		StepStates:   make(map[string]*StepState),
		StepOutputs:  make(map[string]map[string]any),
		TriggerType:  triggerType,
		Channel:      channel,
		ChatID:       chatID,
		Logs:         make([]LogEntry, 0),
		StartedAt:    time.Now(),
	}

	// 频道绑定：优先使用触发时传入的频道，否则使用工作流配置的默认通知频道
	if channel == "" {
		channel = wf.Config.NotifyChannel
		inst.Channel = channel
	}
	if chatID == "" {
		chatID = wf.Config.NotifyChatID
		inst.ChatID = chatID
	}

	inst.appendLog("", "info", fmt.Sprintf("工作流 '%s' 开始执行（触发: %s）", wf.Name, triggerType))

	// 初始化所有步骤状态为 pending
	for _, step := range wf.Steps {
		inst.StepStates[step.ID] = &StepState{
			Status: StatusPending,
		}
	}

	// 创建可取消的上下文（独立于调用方的上下文，避免 HTTP 请求结束后取消执行）
	runCtx, cancel := context.WithCancel(context.Background())

	e.mu.Lock()
	e.running[inst.ID] = inst
	e.cancelFuncs[inst.ID] = cancel
	e.mu.Unlock()

	// 持久化初始状态
	if err := e.store.SaveInstance(inst); err != nil {
		log.Printf("[workflow] 持久化初始实例状态失败: %v", err)
	}

	// 异步执行工作流
	go e.executeWorkflow(runCtx, wf, inst)

	return inst.ID, nil
}

// StopInstance 取消正在运行的工作流实例。
func (e *Engine) StopInstance(instanceID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cancel, ok := e.cancelFuncs[instanceID]
	if !ok {
		return fmt.Errorf("实例 %s 不存在或未在运行", instanceID)
	}
	cancel()

	// 更新实例状态为已取消
	if inst, ok := e.running[instanceID]; ok {
		inst.Status = StatusCancelled
		now := time.Now()
		inst.FinishedAt = &now
		_ = e.store.SaveInstance(inst)
	}

	delete(e.cancelFuncs, instanceID)
	delete(e.running, instanceID)
	return nil
}

// GetRunningInstances 获取指定工作流的所有运行中实例。
func (e *Engine) GetRunningInstances(workflowName string) []*WorkflowInstance {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*WorkflowInstance
	for _, inst := range e.running {
		if inst.WorkflowName == workflowName {
			result = append(result, inst)
		}
	}
	return result
}

// IsRunning 检查指定工作流是否有实例正在运行。
func (e *Engine) IsRunning(workflowName string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, inst := range e.running {
		if inst.WorkflowName == workflowName {
			return true
		}
	}
	return false
}

// executeWorkflow 按顺序执行工作流的所有步骤。
// 执行完成后清理运行状态并调用完成回调。
func (e *Engine) executeWorkflow(ctx context.Context, wf *Workflow, inst *WorkflowInstance) {
	// 将频道信息注入上下文，供步骤执行器回调时读取
	if inst.Channel != "" {
		ctx = withChannelCtx(ctx, inst.Channel, inst.ChatID)
	}

	// 将工作流变量注入 stepOutputs，使 {{.vars.key}} 可在步骤中引用
	if len(wf.Vars) > 0 {
		varsOutput := make(map[string]any, len(wf.Vars))
		for k, v := range wf.Vars {
			varsOutput[k] = v
		}
		inst.StepOutputs["vars"] = varsOutput
	}

	// 执行开始回调（频道通知等）
	if e.onStart != nil {
		e.onStart(inst)
	}

	defer func() {
		e.mu.Lock()
		delete(e.running, inst.ID)
		delete(e.cancelFuncs, inst.ID)
		e.mu.Unlock()

		// 执行完成回调（频道通知等）
		if e.onComplete != nil {
			e.onComplete(inst)
		}
	}()

	// 默认失败策略：中止
	failureStrategy := wf.Config.FailureStrategy
	if failureStrategy == "" {
		failureStrategy = "stop"
	}

	var prevStepState *StepState

	for _, step := range wf.Steps {
		// 检查取消信号
		select {
		case <-ctx.Done():
			inst.Status = StatusCancelled
			now := time.Now()
			inst.FinishedAt = &now
			_ = e.store.SaveInstance(inst)
			return
		default:
		}

		// if 步骤：评估条件后执行对应分支（if 步骤始终执行，不跳过）
		if step.Action == "if" {
			condResult := EvaluateCondition(step.When, prevStepState, inst.StepOutputs)
			var branchSteps []Step
			branchName := "false"
			if condResult {
				branchSteps = step.IfTrue
				branchName = "true"
			} else {
				branchSteps = step.IfFalse
			}
			inst.appendLog(
				step.ID,
				"info",
				fmt.Sprintf("if 条件 '%s' 评估结果: %s，执行 %s 分支", step.When, fmt.Sprintf("%v", condResult), branchName),
			)
			// 记录 if 步骤的开始时间
			ifStepStart := time.Now()
			// 执行选中的分支步骤
			for _, branchStep := range branchSteps {
				select {
				case <-ctx.Done():
					inst.Status = StatusCancelled
					now := time.Now()
					inst.FinishedAt = &now
					_ = e.store.SaveInstance(inst)
					return
				default:
				}
				result := e.executeStepWithState(ctx, branchStep, inst)
				if result.Error != nil && failureStrategy == "stop" {
					inst.Status = StatusFailed
					inst.Error = fmt.Sprintf("if 分支步骤 '%s' 失败: %v", branchStep.ID, result.Error)
					now := time.Now()
					inst.FinishedAt = &now
					_ = e.store.SaveInstance(inst)
					return
				}
			}
			// if 步骤本身标记为完成
			inst.StepStates[step.ID] = &StepState{Status: StatusCompleted, StartedAt: &ifStepStart}
			now := time.Now()
			inst.StepStates[step.ID].FinishedAt = &now
			_ = e.store.SaveInstance(inst)
			prevStepState = inst.StepStates[step.ID]
			continue
		}

		// 非 if 步骤：评估 when 条件，不满足则跳过
		if !EvaluateCondition(step.When, prevStepState, inst.StepOutputs) {
			inst.StepStates[step.ID] = &StepState{Status: StatusSkipped}
			inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 条件不满足，跳过", step.ID))
			_ = e.store.SaveInstance(inst)
			prevStepState = inst.StepStates[step.ID]
			continue
		}

		// 执行步骤
		result := e.executeStepWithState(ctx, step, inst)
		prevStepState = inst.StepStates[step.ID]

		// 处理失败
		if result.Error != nil && failureStrategy == "stop" {
			// 尝试查找并执行 on_error 处理步骤
			handled := e.tryErrorHandlers(ctx, wf, inst, step.ID)
			if !handled {
				inst.Status = StatusFailed
				inst.Error = fmt.Sprintf("步骤 '%s' 失败: %v", step.ID, result.Error)
				now := time.Now()
				inst.FinishedAt = &now
				_ = e.store.SaveInstance(inst)
				return
			}
		}
	}

	// 所有步骤执行完成
	inst.Status = StatusCompleted
	now := time.Now()
	inst.FinishedAt = &now
	inst.appendLog("", "info", fmt.Sprintf("工作流 '%s' 执行完成", wf.Name))
	_ = e.store.SaveInstance(inst)

	log.Printf("[workflow] ✓ 工作流 '%s' 实例 %s 执行完成", wf.Name, inst.ID)
}

// executeStepWithState 执行单个步骤并更新实例状态。
// 包括设置运行状态、执行步骤、记录输出、更新完成状态。
func (e *Engine) executeStepWithState(ctx context.Context, step Step, inst *WorkflowInstance) StepResult {
	now := time.Now()
	state := &StepState{
		Status:    StatusRunning,
		StartedAt: &now,
	}
	inst.StepStates[step.ID] = state
	inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 开始执行（action: %s）", step.ID, step.Action))
	_ = e.store.SaveInstance(inst)

	// 步骤开始回调（通知频道即将执行的步骤）
	if e.onStepStart != nil {
		e.onStepStart(step, inst)
	}

	// 执行步骤（含重试逻辑）
	result := e.executor.ExecuteWithRetry(ctx, step, inst.StepOutputs)

	// 更新步骤状态
	state.Attempts++
	now2 := time.Now()
	state.FinishedAt = &now2

	if result.Error != nil {
		state.Status = StatusFailed
		state.Error = result.Error.Error()
		inst.appendLog(step.ID, "error", fmt.Sprintf("步骤 '%s' 执行失败: %s", step.ID, result.Error.Error()))
	} else {
		state.Status = StatusCompleted
		// 存储步骤输出，供后续步骤引用
		if step.OutputKey != "" {
			if inst.StepOutputs[step.ID] == nil {
				inst.StepOutputs[step.ID] = make(map[string]any)
			}
			inst.StepOutputs[step.ID][step.OutputKey] = result.Output
			inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 完成，输出键 '%s'", step.ID, step.OutputKey))
		} else {
			inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 完成", step.ID))
		}
	}

	_ = e.store.SaveInstance(inst)

	// 步骤完成回调（将 AI 响应等实时推送到频道）
	if e.onStepComplete != nil {
		e.onStepComplete(step, inst, result)
	}

	return result
}

// tryErrorHandlers 在步骤失败后查找并执行 on_error 处理步骤。
// 返回 true 表示已找到并执行了错误处理步骤。
func (e *Engine) tryErrorHandlers(ctx context.Context, wf *Workflow, inst *WorkflowInstance, failedStepID string) bool {
	var errorSteps []Step
	for _, step := range wf.Steps {
		if step.When == "on_error" {
			errorSteps = append(errorSteps, step)
		}
	}

	if len(errorSteps) == 0 {
		return false
	}

	for _, step := range errorSteps {
		// 跳过失败步骤本身
		if step.ID == failedStepID {
			continue
		}
		// 跳过已执行的步骤
		if existing, ok := inst.StepStates[step.ID]; ok && existing.Status != StatusPending &&
			existing.Status != StatusSkipped {
			continue
		}

		result := e.executeStepWithState(ctx, step, inst)
		if result.Error != nil {
			log.Printf("[workflow] 错误处理步骤 '%s' 也失败了: %v", step.ID, result.Error)
		}
	}

	return len(errorSteps) > 0
}

// generateInstanceID 生成唯一的实例 ID。
func generateInstanceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
