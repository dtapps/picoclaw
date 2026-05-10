// Package workflow 实现声明式工作流引擎，支持多步骤编排、条件执行、
// 数据传递、重试策略和定时/事件触发。
//
// 核心概念：
//   - Workflow: 工作流定义，由触发器和步骤列表组成
//   - Trigger: 触发器，支持 cron 定时、事件订阅和手动触发
//   - Step: 步骤，支持 agent_prompt（LLM 执行）、tool_call（工具调用）、parallel（并行）
//   - WorkflowInstance: 工作流运行实例，记录步骤状态和输出
//
// 使用流程：
//  1. 在 workspace/workflows/ 目录下创建 YAML 定义文件
//  2. WorkflowService.Start() 加载定义并注册触发器
//  3. 触发时由 Engine 创建 Instance 并按顺序执行步骤
//  4. 每步执行结果存入 StepOutputs，后续步骤可通过 {{.step_id.key}} 引用
package workflow

import (
	"fmt"
	"time"
)

// --- 工作流定义（YAML 结构） ---

// Workflow 表示一个声明式工作流定义。
// 工作流由名称、描述、触发器列表、步骤列表和变量组成，以 YAML 文件存储在 workspace/workflows/ 目录下。
type Workflow struct {
	Name        string            `yaml:"name"             json:"name"`             // 工作流名称，全局唯一标识
	Description string            `yaml:"description"      json:"description"`      // 工作流描述
	Triggers    []Trigger         `yaml:"triggers"         json:"triggers"`         // 触发器列表
	Vars        map[string]string `yaml:"vars,omitempty"   json:"vars,omitempty"`   // 全局变量，可在步骤中通过 {{.vars.key}} 引用
	Steps       []Step            `yaml:"steps"            json:"steps"`            // 步骤列表，按顺序执行
	Config      WorkflowConfig    `yaml:"config,omitempty" json:"config,omitempty"` // 全局配置
	Enabled     bool              `yaml:"-"                json:"enabled"`          // 是否启用（运行时状态，不序列化到 YAML）
	CreatedAt   time.Time         `yaml:"-"                json:"createdAt"`        // 创建时间
	UpdatedAt   time.Time         `yaml:"-"                json:"updatedAt"`        // 更新时间
}

// Trigger 定义工作流的触发条件。
// 支持三种触发方式：cron 定时表达式、事件总线订阅、手动触发。
// 同一个工作流可以有多个触发器。
type Trigger struct {
	Cron  string `yaml:"cron,omitempty"  json:"cron,omitempty"`  // Cron 定时表达式，如 "0 9 * * *"
	Event string `yaml:"event,omitempty" json:"event,omitempty"` // 事件总线 Kind，如 "agent.tool.exec_end"
	TZ    string `yaml:"tz,omitempty"    json:"tz,omitempty"`    // 时区，如 "Asia/Shanghai"
}

// Step 表示工作流中的一个执行步骤。
// 步骤是工作流的核心执行单元，支持四种动作类型：
//   - agent_prompt: 让 LLM agent 执行一段提示词
//   - tool_call: 直接调用已注册的工具
//   - parallel: 并行执行多个子步骤
//   - if: 条件判断，根据 when 表达式执行 true 或 false 分支
type Step struct {
	ID        string         `yaml:"id"                   json:"id"`                   // 步骤唯一标识，用于步骤间引用（仅限 a-zA-Z0-9_）
	Name      string         `yaml:"name,omitempty"       json:"name,omitempty"`       // 步骤显示名称（可选，支持任意字符）
	Action    string         `yaml:"action"               json:"action"`               // 动作类型：agent_prompt | tool_call | parallel | if
	Prompt    string         `yaml:"prompt,omitempty"     json:"prompt,omitempty"`     // agent_prompt 的提示词内容
	Tool      string         `yaml:"tool,omitempty"       json:"tool,omitempty"`       // tool_call 的工具名称
	Args      map[string]any `yaml:"args,omitempty"       json:"args,omitempty"`       // tool_call 的参数
	Parallel  []Step         `yaml:"parallel,omitempty"   json:"parallel,omitempty"`   // parallel 的子步骤列表
	IfTrue    []Step         `yaml:"if_true,omitempty"    json:"if_true,omitempty"`    // if 条件为 true 时执行的步骤
	IfFalse   []Step         `yaml:"if_false,omitempty"   json:"if_false,omitempty"`   // if 条件为 false 时执行的步骤
	When      string         `yaml:"when,omitempty"       json:"when,omitempty"`       // 执行条件：on_error | on_success | 模板比较
	Delay     string         `yaml:"delay,omitempty"      json:"delay,omitempty"`      // 执行前等待时间，如 "5s"、"1m"
	Retry     *RetryConfig   `yaml:"retry,omitempty"      json:"retry,omitempty"`      // 重试配置
	Timeout   string         `yaml:"timeout,omitempty"    json:"timeout,omitempty"`    // 超时时间，如 "30s"、"5m"
	OutputKey string         `yaml:"output_key,omitempty" json:"output_key,omitempty"` // 输出键名，用于步骤间数据传递
}

// RetryConfig 定义步骤的重试策略。
// 当步骤执行失败时，按配置的重试次数和延迟进行重试。
type RetryConfig struct {
	MaxAttempts int    `yaml:"max_attempts" json:"max_attempts"` // 最大重试次数
	Delay       string `yaml:"delay"        json:"delay"`        // 重试间隔，如 "10s"、"1m"
}

// WorkflowConfig 包含工作流的全局配置。
type WorkflowConfig struct {
	FailureStrategy string `yaml:"failure_strategy,omitempty" json:"failure_strategy,omitempty"` // 失败策略：stop（中止）| continue（继续）
	NotifyChannel   string `yaml:"notify_channel,omitempty"   json:"notify_channel,omitempty"`   // 完成通知频道，如 "telegram"
	NotifyChatID    string `yaml:"notify_chat_id,omitempty"   json:"notify_chat_id,omitempty"`   // 完成通知聊天 ID，如 "chat-123"
}

// --- 运行时状态 ---

const (
	StatusPending   = "pending"   // 等待执行
	StatusRunning   = "running"   // 正在执行
	StatusCompleted = "completed" // 执行完成
	StatusFailed    = "failed"    // 执行失败
	StatusCancelled = "canceled"  // 已取消
	StatusSkipped   = "skipped"   // 已跳过（条件不满足）
)

// WorkflowInstance 表示一次工作流执行的运行实例。
// 记录了每个步骤的执行状态和输出，用于追踪和恢复。
type WorkflowInstance struct {
	ID           string                    `json:"id"`                    // 实例唯一 ID
	WorkflowName string                    `json:"workflow_name"`         // 所属工作流名称
	Status       string                    `json:"status"`                // 实例状态
	StepStates   map[string]*StepState     `json:"step_states"`           // 各步骤执行状态
	StepOutputs  map[string]map[string]any `json:"step_outputs"`          // 各步骤输出数据
	TriggerType  string                    `json:"trigger_type"`          // 触发类型：cron | event | manual
	Channel      string                    `json:"channel,omitempty"`     // 绑定频道（从触发上下文继承）
	ChatID       string                    `json:"chat_id,omitempty"`     // 绑定聊天 ID（从触发上下文继承）
	Logs         []LogEntry                `json:"logs,omitempty"`        // 执行日志
	StartedAt    time.Time                 `json:"started_at"`            // 开始时间
	FinishedAt   *time.Time                `json:"finished_at,omitempty"` // 结束时间
	Error        string                    `json:"error,omitempty"`       // 错误信息
}

// LogEntry 记录工作流执行过程中的日志信息。
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`         // 日志时间
	StepID    string    `json:"step_id,omitempty"` // 关联步骤 ID
	Level     string    `json:"level"`             // 日志级别：info | warn | error
	Message   string    `json:"message"`           // 日志内容
}

// appendLog 追加一条日志到实例。
func (inst *WorkflowInstance) appendLog(stepID, level, message string) {
	inst.Logs = append(inst.Logs, LogEntry{
		Timestamp: time.Now(),
		StepID:    stepID,
		Level:     level,
		Message:   message,
	})
}

// StepState 记录单个步骤的执行状态。
type StepState struct {
	Status     string     `json:"status"`                // 步骤状态
	StartedAt  *time.Time `json:"started_at,omitempty"`  // 开始时间
	FinishedAt *time.Time `json:"finished_at,omitempty"` // 结束时间
	Error      string     `json:"error,omitempty"`       // 错误信息
	Attempts   int        `json:"attempts"`              // 执行次数（含重试）
}

// WorkflowListItem 是列表 API 的摘要视图。
type WorkflowListItem struct {
	Name        string            `json:"name"`           // 工作流名称
	Description string            `json:"description"`    // 描述
	Enabled     bool              `json:"enabled"`        // 是否启用
	TriggerType string            `json:"trigger_type"`   // 触发器类型：cron | event | manual
	LastStatus  string            `json:"last_status"`    // 最近一次执行状态
	StepCount   int               `json:"step_count"`     // 步骤数量
	Vars        map[string]string `json:"vars,omitempty"` // 全局变量
}

// Validate 校验工作流定义是否合法。
// 检查名称非空、至少一个步骤、步骤 ID 唯一、动作类型合法等。
func (w *Workflow) Validate() error {
	if w.Name == "" {
		return validationError("工作流校验：名称不能为空")
	}
	if !isValidWorkflowName(w.Name) {
		return validationError("工作流校验：名称不合法，仅允许英文字母、数字、连字符和下划线")
	}
	if len(w.Steps) == 0 {
		return validationError("工作流校验：至少需要一个步骤")
	}
	ids := make(map[string]bool)
	for _, step := range w.Steps {
		if err := validateStep(step, ids); err != nil {
			return err
		}
	}

	return nil
}

// validateStep 递归校验步骤及其子步骤，同时检查 ID 唯一性。
func validateStep(step Step, ids map[string]bool) error {
	label := step.Name
	if label == "" {
		label = step.ID
	}
	if step.ID == "" {
		return validationError(fmt.Sprintf("工作流校验：步骤「%s」缺少 id", label))
	}
	if !isValidStepID(step.ID) {
		return validationError(fmt.Sprintf("工作流校验：步骤「%s」id '%s' 不合法，仅允许英文字母、数字和下划线", label, step.ID))
	}
	if ids[step.ID] {
		return validationError(fmt.Sprintf("工作流校验：步骤「%s」id '%s' 重复", label, step.ID))
	}
	ids[step.ID] = true
	if step.Action != "agent_prompt" && step.Action != "tool_call" && step.Action != "parallel" &&
		step.Action != "if" {
		return validationError(fmt.Sprintf("工作流校验：步骤「%s」动作类型 '%s' 无效", label, step.Action))
	}
	if step.Action == "agent_prompt" && step.Prompt == "" {
		return validationError(fmt.Sprintf("工作流校验：步骤「%s」需要提供 prompt", label))
	}
	if step.Action == "tool_call" && step.Tool == "" {
		return validationError(fmt.Sprintf("工作流校验：步骤「%s」需要提供 tool 名称", label))
	}
	if step.Action == "parallel" && len(step.Parallel) == 0 {
		return validationError(fmt.Sprintf("工作流校验：步骤「%s」需要至少一个子步骤", label))
	}
	if step.Action == "if" && step.When == "" {
		return validationError(fmt.Sprintf("工作流校验：步骤「%s」需要提供 when 条件", label))
	}
	if step.Action == "if" && len(step.IfTrue) == 0 && len(step.IfFalse) == 0 {
		return validationError(fmt.Sprintf("工作流校验：步骤「%s」需要至少一个分支步骤", label))
	}

	// 校验 duration 格式字段
	if step.Delay != "" {
		if _, err := time.ParseDuration(step.Delay); err != nil {
			return validationError(fmt.Sprintf("工作流校验：步骤「%s」delay '%s' 格式无效（如 5s、1m30s）", label, step.Delay))
		}
	}
	if step.Timeout != "" {
		if _, err := time.ParseDuration(step.Timeout); err != nil {
			return validationError(fmt.Sprintf("工作流校验：步骤「%s」timeout '%s' 格式无效（如 30s、1m）", label, step.Timeout))
		}
	}
	if step.Retry != nil && step.Retry.Delay != "" {
		if _, err := time.ParseDuration(step.Retry.Delay); err != nil {
			return validationError(fmt.Sprintf("工作流校验：步骤「%s」retry delay '%s' 格式无效（如 10s）", label, step.Retry.Delay))
		}
	}

	// 递归校验子步骤
	for _, sub := range step.Parallel {
		if err := validateStep(sub, ids); err != nil {
			return err
		}
	}
	for _, sub := range step.IfTrue {
		if err := validateStep(sub, ids); err != nil {
			return err
		}
	}
	for _, sub := range step.IfFalse {
		if err := validateStep(sub, ids); err != nil {
			return err
		}
	}
	return nil
}

// isValidStepID 检查步骤 ID 是否只包含合法字符（英文字母、数字、下划线）。
// 这确保了模板引用 {{.step_id.key}} 能被正确解析。
func isValidStepID(id string) bool {
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return len(id) > 0
}

// isValidWorkflowName 检查工作流名称是否只包含合法字符（英文字母、数字、连字符、下划线）。
// 这确保了文件名 sanitizeName() 不会产生空字符串（导致 unnamed.yml）。
func isValidWorkflowName(name string) bool {
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return len(name) > 0
}

// validationError 表示工作流校验错误。
type validationError string

func (e validationError) Error() string { return string(e) }
