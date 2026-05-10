package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// StepExecutor 负责执行工作流中的单个步骤。
// 它将步骤动作委托给具体的执行函数：
//   - agent_prompt → AgentPromptFunc（通过 SubTurn 调用 LLM）
//   - tool_call → ToolCallFunc（通过 ToolRegistry 调用工具）
//   - parallel → 并发执行多个子步骤
type StepExecutor struct {
	// AgentPromptFunc 执行 agent prompt，返回 LLM 的文本响应。
	AgentPromptFunc func(ctx context.Context, prompt string) (string, error)

	// ToolCallFunc 执行工具调用，返回（输出文本, 是否错误, 错误）。
	ToolCallFunc func(ctx context.Context, toolName string, args map[string]any) (string, bool, error)
}

// StepResult 保存步骤执行的输出结果。
type StepResult struct {
	Output string // 步骤输出文本
	Error  error  // 执行错误
}

// Execute 执行单个步骤并返回结果。
// 会先解析模板变量，再根据动作类型分发执行。
func (se *StepExecutor) Execute(ctx context.Context, step Step, stepOutputs map[string]map[string]any) StepResult {
	// 执行前延迟
	if step.Delay != "" {
		if d, err := time.ParseDuration(step.Delay); err == nil && d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return StepResult{Error: ctx.Err()}
			}
		}
	}

	// 解析 prompt 和 args 中的模板引用
	prompt := ResolveStepTemplates(step.Prompt, stepOutputs)
	args := resolveArgsTemplates(step.Args, stepOutputs)

	// 应用步骤级超时
	if step.Timeout != "" {
		var cancel context.CancelFunc
		timeout, err := time.ParseDuration(step.Timeout)
		if err == nil {
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		} else {
			logger.WarnCF(
				"workflow",
				"步骤 timeout 格式无效，跳过超时设置",
				map[string]any{"step": step.ID, "timeout": step.Timeout},
			)
		}
	}

	switch step.Action {
	case "agent_prompt":
		return se.executeAgentPrompt(ctx, prompt)
	case "tool_call":
		return se.executeToolCall(ctx, step.Tool, args)
	case "parallel":
		return se.executeParallel(ctx, step.Parallel, stepOutputs)
	case "if":
		// if 步骤不直接执行，由 Engine 评估条件后选择分支执行
		return StepResult{Output: "if step: use engine branch evaluation"}
	default:
		return StepResult{Error: fmt.Errorf("未知动作类型: %s", step.Action)}
	}
}

// executeAgentPrompt 执行 agent prompt 步骤。
// 通过 SubTurn 让 LLM 处理提示词并返回结果。
func (se *StepExecutor) executeAgentPrompt(ctx context.Context, prompt string) StepResult {
	if se.AgentPromptFunc == nil {
		return StepResult{Error: fmt.Errorf("agent prompt 执行器未配置")}
	}

	result, err := se.AgentPromptFunc(ctx, prompt)
	if err != nil {
		return StepResult{Error: err}
	}
	return StepResult{Output: result}
}

// executeToolCall 执行工具调用步骤。
// 直接通过 ToolRegistry 调用指定工具。
func (se *StepExecutor) executeToolCall(ctx context.Context, toolName string, args map[string]any) StepResult {
	if se.ToolCallFunc == nil {
		return StepResult{Error: fmt.Errorf("tool call 执行器未配置")}
	}

	output, isError, err := se.ToolCallFunc(ctx, toolName, args)
	if err != nil {
		return StepResult{Error: err}
	}
	if isError {
		return StepResult{Error: fmt.Errorf("%s", output)}
	}
	return StepResult{Output: output}
}

// executeParallel 并行执行多个子步骤。
// 为每个子步骤启动一个 goroutine，等待全部完成后聚合结果。
func (se *StepExecutor) executeParallel(
	ctx context.Context,
	steps []Step,
	stepOutputs map[string]map[string]any,
) StepResult {
	type parallelResult struct {
		index  int
		result StepResult
	}

	ch := make(chan parallelResult, len(steps))
	for i, step := range steps {
		go func(idx int, s Step) {
			ch <- parallelResult{index: idx, result: se.ExecuteWithRetry(ctx, s, stepOutputs)}
		}(i, step)
	}

	var errs []error
	outputs := make(map[string]any)
	for range steps {
		r := <-ch
		if r.result.Error != nil {
			errs = append(errs, fmt.Errorf("步骤[%d]: %w", r.index, r.result.Error))
		}
		if steps[r.index].OutputKey != "" {
			outputs[steps[r.index].OutputKey] = r.result.Output
		}
	}

	if len(errs) > 0 {
		return StepResult{
			Output: fmt.Sprintf("%v", outputs),
			Error:  fmt.Errorf("并行步骤有 %d 个错误: %v", len(errs), errs),
		}
	}

	// 合并输出
	var combined string
	for _, step := range steps {
		if step.OutputKey != "" {
			if v, ok := outputs[step.OutputKey]; ok {
				combined += fmt.Sprintf("[%s] %s\n", step.OutputKey, valueToString(v))
			}
		}
	}

	return StepResult{Output: combined}
}

// ExecuteWithRetry 执行步骤，支持可选的重试逻辑。
// 当步骤配置了 Retry 时，在失败后按指定延迟重试，直到成功或达到最大重试次数。
func (se *StepExecutor) ExecuteWithRetry(
	ctx context.Context,
	step Step,
	stepOutputs map[string]map[string]any,
) StepResult {
	maxAttempts := 1
	retryDelay := time.Duration(0)

	if step.Retry != nil && step.Retry.MaxAttempts > 0 {
		maxAttempts = step.Retry.MaxAttempts
		if step.Retry.Delay != "" {
			if d, err := time.ParseDuration(step.Retry.Delay); err == nil {
				retryDelay = d
			}
		}
	}

	var lastResult StepResult
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result := se.Execute(ctx, step, stepOutputs)
		if result.Error == nil {
			return result
		}
		lastResult = result

		if attempt < maxAttempts {
			logger.WarnCF("workflow", "步骤执行失败，即将重试", map[string]any{
				"step": step.ID, "attempt": attempt, "max_attempts": maxAttempts,
				"error": result.Error.Error(), "retry_delay": retryDelay.String(),
			})
			if retryDelay > 0 {
				select {
				case <-time.After(retryDelay):
				case <-ctx.Done():
					return StepResult{Error: ctx.Err()}
				}
			}
		}
	}

	return lastResult
}

// resolveArgsTemplates 解析工具调用参数中的模板引用。
// 只对字符串类型的参数值进行模板替换。
func resolveArgsTemplates(args map[string]any, outputs map[string]map[string]any) map[string]any {
	if args == nil {
		return nil
	}
	resolved := make(map[string]any, len(args))
	for k, v := range args {
		switch val := v.(type) {
		case string:
			resolved[k] = ResolveStepTemplates(val, outputs)
		default:
			resolved[k] = v
		}
	}
	return resolved
}
