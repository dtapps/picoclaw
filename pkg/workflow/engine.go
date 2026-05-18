package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type contextKey string

const (
	channelCtxKey    contextKey = "workflow_channel"
	chatIDCtxKey     contextKey = "workflow_chat_id"
	workdirCtxKey    contextKey = "workflow_workdir"
	sessionKeyCtxKey contextKey = "workflow_session_key"
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
func withChannelCtx(ctx context.Context, channel, chatID, sessionKey string) context.Context {
	ctx = context.WithValue(ctx, channelCtxKey, channel)
	ctx = context.WithValue(ctx, chatIDCtxKey, chatID)
	ctx = context.WithValue(ctx, sessionKeyCtxKey, sessionKey)
	return ctx
}

// SessionKeyFromCtx 从上下文中提取工作流的 session key。
func SessionKeyFromCtx(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(sessionKeyCtxKey).(string)
	return v, ok
}

// WorkdirFromCtx 从上下文中提取工作流的工作目录。
func WorkdirFromCtx(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(workdirCtxKey).(string)
	return v, ok
}

// withWorkdirCtx 将工作目录注入上下文。
func withWorkdirCtx(ctx context.Context, workdir string) context.Context {
	return context.WithValue(ctx, workdirCtxKey, workdir)
}

// Engine 是工作流的核心编排引擎。
// 负责按步骤顺序驱动工作流执行，管理状态机、条件判断、
// 数据传递和错误处理。支持异步执行和取消。
type Engine struct {
	store          *PersistStore                                                                               // 持久化存储
	executor       *StepExecutor                                                                               // 步骤执行器
	eventBus       runtimeevents.Bus                                                                           // 事件总线（可选，为 nil 时不发布事件）
	onStart        func(inst *WorkflowInstance) <-chan struct{}                                                // 执行开始回调（用于频道通知等），返回确认信号通道
	onStepStart    func(step Step, inst *WorkflowInstance, resolvedPrompt string, resolvedArgs map[string]any) // 步骤开始回调（resolvedPrompt/Args 为展开后的值）
	onStepComplete func(step Step, inst *WorkflowInstance, result StepResult)                                  // 步骤完成回调
	onComplete     func(inst *WorkflowInstance) <-chan struct{}                                                // 执行完成回调（用于频道通知等），返回确认信号通道
	mu             sync.RWMutex                                                                                // 保护 running 和 cancelFuncs
	running        map[string]*WorkflowInstance                                                                // 当前运行中的实例（按实例 ID 索引）
	cancelFuncs    map[string]context.CancelFunc                                                               // 取消函数（按实例 ID 索引）
	doneChs        map[string]chan struct{}                                                                    // 实例完成信号（用于等待异步 goroutine 退出）
}

// NewEngine 创建新的工作流引擎。
func NewEngine(store *PersistStore, executor *StepExecutor) *Engine {
	return &Engine{
		store:       store,
		executor:    executor,
		running:     make(map[string]*WorkflowInstance),
		cancelFuncs: make(map[string]context.CancelFunc),
		doneChs:     make(map[string]chan struct{}),
	}
}

// SetOnComplete 设置执行完成回调。
// 回调在工作流执行结束（成功/失败/取消）后调用，可用于频道通知等场景。
// 回调应同步发布通知消息，返回 nil 即可；若返回确认信号通道，
// 引擎会等待该通道关闭后再继续。
func (e *Engine) SetOnComplete(fn func(inst *WorkflowInstance) <-chan struct{}) {
	e.onComplete = fn
}

// SetOnStart 设置执行开始回调。
// 回调在工作流开始执行时调用，可用于发送开始通知等场景。
// 回调应同步发布通知消息，返回 nil 即可；若返回确认信号通道，
// 引擎会等待该通道关闭后再启动异步执行，确保开始通知已进入消息管线。
func (e *Engine) SetOnStart(fn func(inst *WorkflowInstance) <-chan struct{}) {
	e.onStart = fn
}

// SetOnStepStart 设置步骤开始回调。
// 回调在每个步骤开始执行时调用，可用于通知频道即将执行的步骤。
// resolvedPrompt 为展开后的提示词（仅对 agent_prompt 步骤有意义）。
// resolvedArgs 为展开后的参数（仅对 tool_call 步骤有意义）。
func (e *Engine) SetOnStepStart(
	fn func(step Step, inst *WorkflowInstance, resolvedPrompt string, resolvedArgs map[string]any),
) {
	e.onStepStart = fn
}

// SetOnStepComplete 设置步骤完成回调。
// 回调在每个步骤执行完成后调用，可用于将 AI 响应实时推送到频道。
func (e *Engine) SetOnStepComplete(fn func(step Step, inst *WorkflowInstance, result StepResult)) {
	e.onStepComplete = fn
}

// SetEventBus 设置事件总线，用于发布工作流状态变更事件。
// 设置后，引擎在步骤开始/完成、实例开始/完成时发布事件，
// 可供 SSE 等实时推送机制订阅。
func (e *Engine) SetEventBus(bus runtimeevents.Bus) {
	e.eventBus = bus
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
		Logs:         make([]LogEntry, 0),
		StartedAt:    time.Now(),
	}

	// 如果配置为复用 session，则生成固定的 session key
	if wf.Config.ReuseSession {
		inst.SessionKey = fmt.Sprintf("agent:workflow:%s", wf.Name)
	}

	// 频道绑定：始终使用工作流配置的通知目标
	// 如果配置了多频道，就发送到所有频道；否则使用运行时传入的频道
	targets := wf.Config.GetNotifyTargets()
	if len(targets) > 0 {
		// 使用配置的所有通知目标
		inst.NotifyChannels = targets
		logger.InfoCF("workflow", "工作流配置了多频道通知", map[string]any{
			"workflow":        wf.Name,
			"notify_channels": targets,
			"count":           len(targets),
		})
	} else if channel != "" {
		// 没有配置，使用运行时传入的频道
		inst.NotifyChannels = []NotifyTarget{{Channel: channel, ChatID: chatID}}
		logger.InfoCF("workflow", "使用运行时传入的单频道", map[string]any{
			"workflow": wf.Name,
			"channel":  channel,
			"chat_id":  chatID,
		})
	}

	inst.appendLog("", "info", fmt.Sprintf("工作流 '%s' 开始执行（触发: %s）", wf.Name, triggerType))

	// 初始化所有步骤状态为 pending（含子步骤）
	initStepStatesRecursive(wf.Steps, inst.StepStates)

	// 创建可取消的上下文（独立于调用方的上下文，避免 HTTP 请求结束后取消执行）
	runCtx, cancel := context.WithCancel(context.Background())

	e.mu.Lock()
	e.running[inst.ID] = inst
	e.cancelFuncs[inst.ID] = cancel
	e.doneChs[inst.ID] = make(chan struct{})
	e.mu.Unlock()

	// 持久化初始状态
	if err := e.store.SaveInstance(inst); err != nil {
		logger.ErrorCF("workflow", "持久化初始实例状态失败", map[string]any{"error": err.Error()})
	}

	// 在启动 goroutine 前同步执行开始回调，确保开始通知已进入消息管线，
	// 避免步骤通知先于开始通知到达客户端。
	// onStart 回调应同步发布通知消息，返回 nil 即可；
	// 若返回确认信号通道，引擎会等待该通道关闭后再继续。
	if e.onStart != nil {
		if ackCh := e.onStart(inst); ackCh != nil {
			select {
			case <-ackCh:
			case <-time.After(5 * time.Second):
				logger.WarnCF("workflow", "等待 onStart 确认信号超时，继续执行", map[string]any{"workflow": wf.Name})
			}
		}
	}
	e.publishEvent(runtimeevents.KindWorkflowInstanceStart, inst, map[string]any{
		"workflow": inst.WorkflowName, "trigger": inst.TriggerType,
	})

	// 异步执行工作流
	go e.executeWorkflow(runCtx, wf, inst)

	return inst.ID, nil
}

// StopInstance 取消正在运行的工作流实例。
// 仅取消上下文，由 executeWorkflow goroutine 负责更新状态和持久化，
// 避免与正在运行的 goroutine 产生数据竞争。
func (e *Engine) StopInstance(instanceID string) error {
	e.mu.Lock()
	cancel, ok := e.cancelFuncs[instanceID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("实例 %s 不存在或未在运行", instanceID)
	}
	cancel()
	delete(e.cancelFuncs, instanceID)
	e.mu.Unlock()
	return nil
}

// WaitRunning 等待所有运行中的实例完成。
// 用于服务停止时确保异步 goroutine 退出，避免临时目录清理时仍有文件写入。
func (e *Engine) WaitRunning() {
	e.mu.RLock()
	chs := make([]chan struct{}, 0, len(e.doneChs))
	for _, ch := range e.doneChs {
		chs = append(chs, ch)
	}
	e.mu.RUnlock()

	for _, ch := range chs {
		<-ch
	}
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
	// 当工作流绑定了通知频道时，通知通过 SendMessage 同步发送
	// （标记 workflow_notification 绕过 preSend 的 streamActive/placeholder 检查），
	// 消息在回调返回时已到达远程 API，无需额外延迟。

	// 将工作目录注入上下文，供 tool_call 步骤自动注入 cwd
	if wf.Config.Workdir != "" {
		ctx = withWorkdirCtx(ctx, wf.Config.Workdir)
	}

	// 将频道信息和 session key 注入上下文，供步骤执行器回调时读取
	// 如果开启了 ReuseSession，注入固定的 session key 以复用 LLM 会话历史
	// 注意：使用固定的 channel/chatID（workflow/default），不依赖通知频道配置
	// 这样即使修改通知频道，LLM 会话也能保持一致
	if wf.Config.ReuseSession && inst.SessionKey != "" {
		ctx = withChannelCtx(ctx, "workflow", "default", inst.SessionKey)
	}

	// 将工作流变量注入 stepOutputs，使 {{.vars.key}} 可在步骤中引用
	if len(wf.Vars) > 0 {
		varsOutput := make(map[string]any, len(wf.Vars))
		for k, v := range wf.Vars {
			varsOutput[k] = v
		}
		inst.mu.Lock()
		inst.StepOutputs["vars"] = varsOutput
		inst.mu.Unlock()
	}

	defer func() {
		e.mu.Lock()
		delete(e.running, inst.ID)
		delete(e.cancelFuncs, inst.ID)
		if ch, ok := e.doneChs[inst.ID]; ok {
			delete(e.doneChs, inst.ID)
			close(ch)
		}
		e.mu.Unlock()

		// 发布实例完成事件
		e.publishEvent(runtimeevents.KindWorkflowInstanceComplete, inst, map[string]any{
			"workflow": inst.WorkflowName, "status": inst.Status, "error": inst.Error,
		})

		// 执行完成回调（频道通知等），等待确认信号确保通知已进入消息管线
		if e.onComplete != nil {
			if ackCh := e.onComplete(inst); ackCh != nil {
				select {
				case <-ackCh:
				case <-time.After(5 * time.Second):
					logger.WarnCF(
						"workflow",
						"等待 onComplete 确认信号超时，继续执行",
						map[string]any{"workflow": inst.WorkflowName},
					)
				}
			}
		}
	}()

	// 默认失败策略：中止
	failureStrategy := wf.Config.FailureStrategy
	if failureStrategy == "" {
		failureStrategy = "stop"
	}
	inst.FailureStrategy = failureStrategy

	var prevStepState *StepState

	for _, step := range wf.Steps {
		// 检查取消信号
		select {
		case <-ctx.Done():
			inst.mu.Lock()
			inst.Status = StatusCancelled
			now := time.Now()
			inst.FinishedAt = &now
			_ = e.store.SaveInstance(inst)
			inst.mu.Unlock()
			return
		default:
		}

		// if 步骤：评估条件后执行对应分支
		if step.Action == "if" {
			// enabled 检查：禁用时跳过
			if step.Enabled != nil && !*step.Enabled {
				inst.mu.Lock()
				inst.StepStates[step.ID] = &StepState{Name: step.Name, Status: StatusSkipped}
				_ = e.store.SaveInstance(inst)
				inst.mu.Unlock()
				inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 已禁用，跳过", step.ID))
				prevStepState = inst.StepStates[step.ID]
				continue
			}

			// 步骤开始回调
			if e.onStepStart != nil {
				e.onStepStart(step, inst, "", nil)
			}
			condResult := EvaluateCondition(step.When, prevStepState, inst.StepOutputs)
			var branchSteps []Step
			var skippedBranch []Step
			branchName := "false"
			if condResult {
				branchSteps = step.IfTrue
				skippedBranch = step.IfFalse
				branchName = "true"
			} else {
				branchSteps = step.IfFalse
				skippedBranch = step.IfTrue
			}
			inst.appendLog(
				step.ID,
				"info",
				fmt.Sprintf("if 条件 '%s' 评估结果: %s，执行 %s 分支", step.When, fmt.Sprintf("%v", condResult), branchName),
			)
			// 未执行的分支子步骤标记为 skipped
			inst.mu.Lock()
			for _, s := range skippedBranch {
				inst.StepStates[s.ID] = &StepState{Name: s.Name, Status: StatusSkipped}
			}
			inst.mu.Unlock()
			// 记录 if 步骤的开始时间
			ifStepStart := time.Now()
			// 执行选中的分支步骤
			for _, branchStep := range branchSteps {
				select {
				case <-ctx.Done():
					inst.mu.Lock()
					inst.Status = StatusCancelled
					now := time.Now()
					inst.FinishedAt = &now
					_ = e.store.SaveInstance(inst)
					inst.mu.Unlock()
					return
				default:
				}
				result := e.executeStepWithState(ctx, branchStep, inst)
				if result.Error != nil && failureStrategy == "stop" {
					inst.mu.Lock()
					inst.Status = StatusFailed
					inst.Error = fmt.Sprintf("if 分支步骤 '%s' 失败: %v", branchStep.ID, result.Error)
					now := time.Now()
					inst.FinishedAt = &now
					_ = e.store.SaveInstance(inst)
					inst.mu.Unlock()
					return
				}
			}
			// if 步骤本身标记为完成
			inst.mu.Lock()
			inst.StepStates[step.ID] = &StepState{
				Name:      step.Name,
				Status:    StatusCompleted,
				StartedAt: &ifStepStart,
				Attempts:  1,
			}
			now := time.Now()
			inst.StepStates[step.ID].FinishedAt = &now
			_ = e.store.SaveInstance(inst)
			inst.mu.Unlock()
			// 步骤完成回调
			if e.onStepComplete != nil {
				e.onStepComplete(step, inst, StepResult{Output: "if: true"})
			}
			prevStepState = inst.StepStates[step.ID]
			continue
		}

		// 非 if 步骤：评估 when 条件和 enabled，不满足则跳过
		if !EvaluateCondition(step.When, prevStepState, inst.StepOutputs) {
			inst.mu.Lock()
			inst.StepStates[step.ID] = &StepState{Name: step.Name, Status: StatusSkipped}
			inst.mu.Unlock()
			inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 条件不满足，跳过", step.ID))
			inst.mu.Lock()
			_ = e.store.SaveInstance(inst)
			inst.mu.Unlock()
			prevStepState = inst.StepStates[step.ID]
			continue
		}

		// 检查步骤是否启用
		if step.Enabled != nil && !*step.Enabled {
			inst.mu.Lock()
			inst.StepStates[step.ID] = &StepState{Name: step.Name, Status: StatusSkipped}
			inst.mu.Unlock()
			inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 已禁用，跳过", step.ID))
			inst.mu.Lock()
			_ = e.store.SaveInstance(inst)
			inst.mu.Unlock()
			prevStepState = inst.StepStates[step.ID]
			continue
		}

		// 执行步骤
		result := e.executeStepWithState(ctx, step, inst)
		prevStepState = inst.StepStates[step.ID]

		// 处理失败
		if result.Error != nil && failureStrategy == "stop" {
			// 尝试查找并执行 on_error 处理步骤
			handled, handlerSuccess := e.tryErrorHandlers(ctx, wf, inst, step.ID)
			inst.mu.Lock()
			if handled && handlerSuccess {
				// 错误已被成功处理，跳过后续步骤，标记为 completed
				inst.Status = StatusCompleted
			} else {
				inst.Status = StatusFailed
				inst.Error = fmt.Sprintf("步骤 '%s' 失败: %v", step.ID, result.Error)
			}
			now := time.Now()
			inst.FinishedAt = &now
			_ = e.store.SaveInstance(inst)
			inst.mu.Unlock()
			return
		}
	}

	// 所有步骤执行完成
	inst.mu.Lock()
	inst.Status = StatusCompleted
	now := time.Now()
	inst.FinishedAt = &now
	inst.mu.Unlock()
	inst.appendLog("", "info", fmt.Sprintf("工作流 '%s' 执行完成", wf.Name))
	inst.mu.Lock()
	_ = e.store.SaveInstance(inst)
	inst.mu.Unlock()

	logger.InfoCF(
		"workflow",
		"工作流执行完成",
		map[string]any{"workflow": wf.Name, "instance": inst.ID, "status": inst.Status},
	)
}

// initStepStatesRecursive 递归初始化所有步骤（含 parallel/if 子步骤）的状态为 pending。
func initStepStatesRecursive(steps []Step, states map[string]*StepState) {
	for _, step := range steps {
		if _, exists := states[step.ID]; !exists {
			states[step.ID] = &StepState{Name: step.Name, Status: StatusPending}
		}
		// v2 格式：每个分支包含多个步骤
		for _, branch := range step.Parallel {
			initStepStatesRecursive(branch.Branch, states)
		}
		initStepStatesRecursive(step.IfTrue, states)
		initStepStatesRecursive(step.IfFalse, states)
	}
}

// executeStepWithState 执行单个步骤并更新实例状态。
// 包括设置运行状态、执行步骤、记录输出、更新完成状态。
// parallel 步骤在引擎层直接编排：为每个子步骤启动 goroutine 调用 executeStepWithState，
// 确保子步骤也有完整的 state tracking、日志和回调。
func (e *Engine) executeStepWithState(ctx context.Context, step Step, inst *WorkflowInstance) StepResult {
	// enabled 检查：禁用时跳过（与主循环行为一致）
	if step.Enabled != nil && !*step.Enabled {
		inst.mu.Lock()
		inst.StepStates[step.ID] = &StepState{Name: step.Name, Status: StatusSkipped}
		_ = e.store.SaveInstance(inst)
		inst.mu.Unlock()
		inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 已禁用，跳过", step.ID))
		return StepResult{}
	}

	// 执行前延迟
	if step.Delay != "" {
		if d, err := time.ParseDuration(step.Delay); err == nil && d > 0 {
			inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 延迟 %s 后执行", step.ID, step.Delay))
			select {
			case <-time.After(d):
			case <-ctx.Done():
				inst.mu.Lock()
				inst.StepStates[step.ID] = &StepState{Name: step.Name, Status: StatusCancelled}
				inst.mu.Unlock()
				return StepResult{Error: ctx.Err()}
			}
		} else if err != nil {
			logger.WarnCF("workflow", "步骤 delay 格式无效，跳过延迟", map[string]any{"step": step.ID, "delay": step.Delay})
		}
	}

	now := time.Now()
	state := &StepState{
		Name:      step.Name,
		Status:    StatusRunning,
		StartedAt: &now,
	}
	inst.mu.Lock()
	inst.StepStates[step.ID] = state
	inst.mu.Unlock()
	inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 开始执行", step.ID))
	inst.mu.Lock()
	_ = e.store.SaveInstance(inst)
	inst.mu.Unlock()

	// 发布步骤开始事件
	e.publishEvent(runtimeevents.KindWorkflowStepStart, inst, map[string]any{
		"step_id": step.ID, "action": step.Action,
	})

	// 计算展开后的提示词和参数（用于步骤开始通知和 ResolvedInput）
	var resolvedPrompt string
	var resolvedArgs map[string]any
	if step.Action == "tool_call" || step.Action == "agent_prompt" {
		inst.mu.Lock()
		outputsSnapshot := make(map[string]map[string]any, len(inst.StepOutputs)+1)
		maps.Copy(outputsSnapshot, inst.StepOutputs)
		inst.mu.Unlock()
		// 添加 self 变量
		selfOutput := map[string]any{"id": step.ID}
		if step.Name != "" {
			selfOutput["name"] = step.Name
		}
		outputsSnapshot["self"] = selfOutput
		resolvedPrompt = ResolveStepTemplates(step.Prompt, outputsSnapshot)
		resolvedArgs = resolveArgsTemplates(step.Args, outputsSnapshot)
	}

	// 为 tool_call 步骤自动注入 workdir 到 resolvedArgs 的 cwd
	if step.Action == "tool_call" {
		if _, hasCwd := resolvedArgs["cwd"]; !hasCwd {
			if wd, ok := WorkdirFromCtx(ctx); ok && wd != "" {
				inst.mu.Lock()
				snapshot := make(map[string]map[string]any, len(inst.StepOutputs)+1)
				maps.Copy(snapshot, inst.StepOutputs)
				inst.mu.Unlock()
				selfOutput := map[string]any{"id": step.ID}
				if step.Name != "" {
					selfOutput["name"] = step.Name
				}
				snapshot["self"] = selfOutput
				resolvedWorkdir := ResolveStepTemplates(wd, snapshot)
				if resolvedArgs == nil {
					resolvedArgs = make(map[string]any)
				}
				resolvedArgs["cwd"] = resolvedWorkdir
			}
		}
	}

	// 存储渲染后的输入参数到 StepState
	if resolvedPrompt != "" || len(resolvedArgs) > 0 {
		inst.mu.Lock()
		state.ResolvedInput = &ResolvedInput{
			Prompt: resolvedPrompt,
			Args:   resolvedArgs,
		}
		inst.mu.Unlock()
	}

	// 步骤开始回调（通知频道即将执行的步骤）
	if e.onStepStart != nil {
		e.onStepStart(step, inst, resolvedPrompt, resolvedArgs)
	}

	// 执行步骤
	var result StepResult
	if step.Action == "parallel" {
		result = e.executeParallelInEngine(ctx, step, inst)
	} else if step.Action == "if" {
		result = e.executeIfInEngine(ctx, step, inst)
	} else {
		// 快照 stepOutputs 避免并行子步骤写入时的 map 数据竞争
		inst.mu.Lock()
		outputsSnapshot := make(map[string]map[string]any, len(inst.StepOutputs))
		maps.Copy(outputsSnapshot, inst.StepOutputs)
		inst.mu.Unlock()
		result = e.executor.ExecuteWithRetry(ctx, step, outputsSnapshot)
	}

	// 更新步骤状态
	inst.mu.Lock()
	if result.Attempts < 1 {
		result.Attempts = 1
	}
	state.Attempts = result.Attempts
	now2 := time.Now()
	state.FinishedAt = &now2

	if result.Error != nil {
		state.Status = StatusFailed
		state.Error = result.Error.Error()
	} else {
		state.Status = StatusCompleted
		// 存储步骤输出，供后续步骤引用
		if inst.StepOutputs[step.ID] == nil {
			inst.StepOutputs[step.ID] = make(map[string]any)
		}
		// 使用配置的 output_key，如果没有设置则使用默认键名 "result"
		key := step.OutputKey
		if key == "" {
			key = "result"
		}
		inst.StepOutputs[step.ID][key] = result.Output
	}
	inst.mu.Unlock()

	if result.Error != nil {
		inst.appendLog(step.ID, "error", fmt.Sprintf("步骤 '%s' 执行失败: %s", step.ID, result.Error.Error()))
	} else {
		key := step.OutputKey
		if key == "" {
			key = "result"
		}
		inst.appendLog(step.ID, "info", fmt.Sprintf("步骤 '%s' 完成，输出键 '%s'", step.ID, key))
	}

	inst.mu.Lock()
	_ = e.store.SaveInstance(inst)
	inst.mu.Unlock()

	// 发布步骤完成事件
	e.publishEvent(runtimeevents.KindWorkflowStepComplete, inst, map[string]any{
		"step_id": step.ID, "action": step.Action, "status": state.Status,
	})

	// 步骤完成回调（将 AI 响应等实时推送到频道）
	if e.onStepComplete != nil {
		e.onStepComplete(step, inst, result)
	}

	return result
}

// executeParallelInEngine 在引擎层编排并行步骤，为每个子步骤调用 executeStepWithState。
// 这样子步骤也有完整的 state tracking、日志记录和回调。
// executeIfInEngine 在引擎层执行 if 步骤：评估条件，执行对应分支，标记跳过的分支。
func (e *Engine) executeIfInEngine(ctx context.Context, step Step, inst *WorkflowInstance) StepResult {
	// 快照 stepOutputs 避免并行子步骤写入时的数据竞争
	inst.mu.Lock()
	outputsSnapshot := make(map[string]map[string]any, len(inst.StepOutputs))
	for k, v := range inst.StepOutputs {
		cp := make(map[string]any, len(v))
		maps.Copy(cp, v)
		outputsSnapshot[k] = cp
	}
	// 对于 on_success/on_error 条件，需要前一个步骤的状态
	// 查找最近一个已完成/失败/跳过的步骤状态作为 prevStepState
	var prevStepState *StepState
	for _, ss := range inst.StepStates {
		if ss == nil || (ss.Status != StatusCompleted && ss.Status != StatusFailed && ss.Status != StatusSkipped) {
			continue
		}
		if prevStepState == nil {
			prevStepState = ss
			continue
		}
		// 优先选择有 FinishedAt 的 step；都有则选最新的
		if ss.FinishedAt != nil && prevStepState.FinishedAt == nil {
			prevStepState = ss
		} else if ss.FinishedAt != nil && prevStepState.FinishedAt != nil && ss.FinishedAt.After(*prevStepState.FinishedAt) {
			prevStepState = ss
		}
	}
	if prevStepState == nil {
		prevStepState = inst.StepStates[step.ID] // fallback
	}
	inst.mu.Unlock()

	condResult := EvaluateCondition(step.When, prevStepState, outputsSnapshot)

	var branchSteps []Step
	var skippedBranch []Step
	if condResult {
		branchSteps = step.IfTrue
		skippedBranch = step.IfFalse
	} else {
		branchSteps = step.IfFalse
		skippedBranch = step.IfTrue
	}

	// 未执行的分支子步骤标记为 skipped
	inst.mu.Lock()
	for _, s := range skippedBranch {
		inst.StepStates[s.ID] = &StepState{Name: s.Name, Status: StatusSkipped}
	}
	inst.mu.Unlock()

	inst.appendLog(step.ID, "info", fmt.Sprintf("if 条件 '%s' 评估结果: %v", step.When, condResult))

	// 执行选中的分支步骤
	for _, branchStep := range branchSteps {
		select {
		case <-ctx.Done():
			return StepResult{Error: ctx.Err()}
		default:
		}
		branchResult := e.executeStepWithState(ctx, branchStep, inst)
		if branchResult.Error != nil {
			if inst.FailureStrategy == "stop" {
				return branchResult
			}
			// continue 策略：记录错误但继续执行
			inst.appendLog(
				step.ID,
				"warn",
				fmt.Sprintf("if 分支步骤 '%s' 失败（continue 策略）: %v", branchStep.ID, branchResult.Error),
			)
		}
	}

	return StepResult{Output: fmt.Sprintf("if: %v", condResult)}
}

func (e *Engine) executeParallelInEngine(ctx context.Context, step Step, inst *WorkflowInstance) StepResult {
	type parallelResult struct {
		index   int
		results []StepResult
	}

	branches := step.Parallel
	ch := make(chan parallelResult, len(branches))

	// v2 格式：每个分支包含多个步骤，分支内串行执行，分支间并行执行
	for i, branch := range branches {
		go func(idx int, b ParallelBranch) {
			var results []StepResult
			for _, s := range b.Branch {
				r := e.executeStepWithState(ctx, s, inst)
				results = append(results, r)
				// 如果步骤失败且策略为 stop，停止该分支的后续步骤
				if r.Error != nil && inst.FailureStrategy == "stop" {
					break
				}
			}
			ch <- parallelResult{index: idx, results: results}
		}(i, branch)
	}

	var errs []error
	outputs := make(map[string]any)
	for range branches {
		r := <-ch
		for _, result := range r.results {
			if result.Error != nil {
				errs = append(errs, result.Error)
			}
		}
		// 收集该分支最后一个步骤的输出
		if len(r.results) > 0 {
			lastResult := r.results[len(r.results)-1]
			// 使用分支最后一个步骤的 OutputKey
			branch := branches[r.index]
			var key string
			if len(branch.Branch) > 0 {
				key = branch.Branch[len(branch.Branch)-1].OutputKey
			}
			if key == "" {
				key = "result"
			}
			outputs[key] = lastResult.Output
		}
	}

	if len(errs) > 0 {
		if inst.FailureStrategy == "stop" {
			return StepResult{
				Output: fmt.Sprintf("%v", outputs),
				Error:  fmt.Errorf("并行步骤有 %d 个错误: %v", len(errs), errs),
			}
		}
		// continue 策略：记录错误但返回成功
		inst.appendLog(step.ID, "warn", fmt.Sprintf("并行步骤有 %d 个错误（continue 策略）: %v", len(errs), errs))
	}

	// 合并输出
	var combined strings.Builder
	for _, branch := range branches {
		if len(branch.Branch) == 0 {
			continue
		}
		lastStep := branch.Branch[len(branch.Branch)-1]
		key := lastStep.OutputKey
		if key == "" {
			key = "result"
		}
		if v, ok := outputs[key]; ok {
			combined.WriteString(fmt.Sprintf("[%s] %s\n", key, valueToString(v)))
		}
	}

	return StepResult{Output: combined.String()}
}

// tryErrorHandlers 在步骤失败后查找并执行 on_error 处理步骤。
// 返回两个值：是否有错误处理步骤、错误处理步骤是否全部成功。
func (e *Engine) tryErrorHandlers(
	ctx context.Context,
	wf *Workflow,
	inst *WorkflowInstance,
	failedStepID string,
) (found bool, allSuccess bool) {
	var errorSteps []Step
	for _, step := range wf.Steps {
		if step.When == "on_error" {
			errorSteps = append(errorSteps, step)
		}
	}

	if len(errorSteps) == 0 {
		return false, false
	}

	allSuccess = true
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
			allSuccess = false
			logger.ErrorCF("workflow", "错误处理步骤也失败了", map[string]any{"step": step.ID, "error": result.Error.Error()})
		}
	}

	return len(errorSteps) > 0, allSuccess
}

// generateInstanceID 生成唯一的实例 ID。
func generateInstanceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// publishEvent 发布工作流事件到事件总线。
// 如果事件总线未配置，则为空操作。
func (e *Engine) publishEvent(kind runtimeevents.Kind, inst *WorkflowInstance, payload any) {
	if e.eventBus == nil {
		return
	}
	e.eventBus.PublishNonBlocking(runtimeevents.Event{
		Kind: kind,
		Source: runtimeevents.Source{
			Component: "workflow",
			Name:      inst.WorkflowName,
		},
		Scope: runtimeevents.Scope{
			RuntimeID: inst.ID,
		},
		Payload: payload,
	})
}
