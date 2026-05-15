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
	"regexp"
	"sync"
	"time"
)

// --- 工作流定义（YAML 结构） ---

// Workflow 表示一个声明式工作流定义。
// 工作流由名称、描述、触发器列表、步骤列表和变量组成，以 YAML 文件存储在 workspace/workflows/ 目录下。
type Workflow struct {
	Name        string            `yaml:"name"                 json:"name"`           // 工作流名称，全局唯一标识
	Description string            `yaml:"description"          json:"description"`    // 工作流描述
	Triggers    []Trigger         `yaml:"triggers"             json:"triggers"`       // 触发器列表
	Vars        map[string]string `yaml:"vars,omitempty"       json:"vars,omitempty"` // 全局变量，可在步骤中通过 {{.vars.key}} 引用
	Steps       []Step            `yaml:"steps"                json:"steps"`          // 步骤列表，按顺序执行
	Config      WorkflowConfig    `yaml:"config,omitempty"     json:"config"`         // 全局配置
	Enabled     bool              `yaml:"-"                    json:"enabled"`        // 是否启用（运行时状态，不序列化到 YAML）
	CreatedAt   time.Time         `yaml:"created_at,omitempty" json:"createdAt"`      // 创建时间
	UpdatedAt   time.Time         `yaml:"updated_at,omitempty" json:"updatedAt"`      // 更新时间

	// 运行时状态字段
	NextRunAt     *time.Time `yaml:"-"                         json:"nextRunAt,omitempty"`     // 下次运行时间（动态计算）
	LastRunAt     *time.Time `yaml:"last_run_at,omitempty"     json:"lastRunAt,omitempty"`     // 上次运行时间
	LastRunStatus string     `yaml:"last_run_status,omitempty" json:"lastRunStatus,omitempty"` // 上次运行状态：running/success/failed
}

// Trigger 定义工作流的触发条件。
// 支持多种触发方式：cron 定时表达式、一次性执行、间隔执行、事件总线订阅、手动触发。
// 同一个工作流可以有多个触发器。
type Trigger struct {
	Cron         string            `yaml:"cron,omitempty"          json:"cron,omitempty"`          // Cron 定时表达式，如 "0 9 * * *"
	At           string            `yaml:"at,omitempty"            json:"at,omitempty"`            // 一次性执行时间，如 "2025-05-15 09:00:00"
	Interval     string            `yaml:"interval,omitempty"      json:"interval,omitempty"`      // 间隔执行，如 "30m", "1h", "2h30m"
	Event        string            `yaml:"event,omitempty"         json:"event,omitempty"`         // 事件总线 Kind，如 "agent.tool.exec_end"
	EventFilters map[string]string `yaml:"event_filters,omitempty" json:"event_filters,omitempty"` // 事件过滤条件，如 {"tool": "git_commit"}
	EventMapping map[string]string `yaml:"event_mapping,omitempty" json:"event_mapping,omitempty"` // 事件数据映射到变量，如 {"result": "output.result"}
	TZ           string            `yaml:"tz,omitempty"            json:"tz,omitempty"`            // 时区，如 "Asia/Shanghai"
}

// EventType 标准事件类型常量
type EventType string

const (
	// EventToolExecStart 工具开始执行
	EventToolExecStart EventType = "agent.tool.exec_start"
	// EventToolExecEnd 工具执行完成
	EventToolExecEnd EventType = "agent.tool.exec_end"
	// EventToolExecError 工具执行错误
	EventToolExecError EventType = "agent.tool.exec_error"

	// EventAgentPromptStart Agent 开始处理提示词
	EventAgentPromptStart EventType = "agent.prompt.start"
	// EventAgentPromptEnd Agent 完成提示词处理
	EventAgentPromptEnd EventType = "agent.prompt.end"
	// EventAgentResponse Agent 产生响应
	EventAgentResponse EventType = "agent.response"

	// EventWorkflowStart 工作流实例开始
	EventWorkflowStart EventType = "workflow.instance.start"
	// EventWorkflowComplete 工作流实例完成
	EventWorkflowComplete EventType = "workflow.instance.complete"
	// EventWorkflowError 工作流实例错误
	EventWorkflowError EventType = "workflow.instance.error"

	// EventSystemStartup 系统启动
	EventSystemStartup EventType = "system.startup"
	// EventSystemShutdown 系统关闭
	EventSystemShutdown EventType = "system.shutdown"
)

// EventTypeList 返回所有标准事件类型列表
func EventTypeList() []EventType {
	return []EventType{
		EventToolExecStart,
		EventToolExecEnd,
		EventToolExecError,
		EventAgentPromptStart,
		EventAgentPromptEnd,
		EventAgentResponse,
		EventWorkflowStart,
		EventWorkflowComplete,
		EventWorkflowError,
		EventSystemStartup,
		EventSystemShutdown,
	}
}

// String 返回事件类型的字符串表示
func (e EventType) String() string {
	return string(e)
}

// DefaultStepTimeout 是步骤的默认超时时间。
// 当步骤未设置 timeout 字段时使用此值。
const DefaultStepTimeout = "30m"

// Step 表示工作流中的一个执行步骤。
// 步骤是工作流的核心执行单元，支持五种动作类型：
//   - agent_prompt: 让 LLM agent 执行一段提示词
//   - tool_call: 直接调用已注册的工具
//   - parallel: 并行执行多个子步骤
//   - if: 条件判断，根据 when 表达式执行 true 或 false 分支
//   - notify: 发送通知消息到绑定的频道
type Step struct {
	ID        string         `yaml:"id"                   json:"id"`                   // 步骤唯一标识，用于步骤间引用（仅限 a-zA-Z0-9_）
	Name      string         `yaml:"name,omitempty"       json:"name,omitempty"`       // 步骤显示名称（可选，支持任意字符）
	Action    string         `yaml:"action"               json:"action"`               // 动作类型：agent_prompt | tool_call | parallel | if | notify
	Prompt    string         `yaml:"prompt,omitempty"     json:"prompt,omitempty"`     // agent_prompt 的提示词内容
	Tool      string         `yaml:"tool,omitempty"       json:"tool,omitempty"`       // tool_call 的工具名称
	Args      map[string]any `yaml:"args,omitempty"       json:"args,omitempty"`       // tool_call 的参数
	Parallel  []Step         `yaml:"parallel,omitempty"   json:"parallel,omitempty"`   // parallel 的子步骤列表
	IfTrue    []Step         `yaml:"if_true,omitempty"    json:"if_true,omitempty"`    // if 条件为 true 时执行的步骤
	IfFalse   []Step         `yaml:"if_false,omitempty"   json:"if_false,omitempty"`   // if 条件为 false 时执行的步骤
	Message   string         `yaml:"message,omitempty"    json:"message,omitempty"`    // notify 的消息内容（支持模板）
	When      string         `yaml:"when,omitempty"       json:"when,omitempty"`       // 执行条件：on_error | on_success | 模板比较
	Delay     string         `yaml:"delay,omitempty"      json:"delay,omitempty"`      // 执行前等待时间，如 "5s"、"1m"
	Retry     *RetryConfig   `yaml:"retry,omitempty"      json:"retry,omitempty"`      // 重试配置
	Timeout   string         `yaml:"timeout,omitempty"    json:"timeout,omitempty"`    // 超时时间，如 "30s"、"5m"，默认 30m
	OutputKey string         `yaml:"output_key,omitempty" json:"output_key,omitempty"` // 输出键名，用于步骤间数据传递
	Enabled   *bool          `yaml:"enabled,omitempty"    json:"enabled,omitempty"`    // 是否启用，nil/false 为禁用
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
	Workdir         string `yaml:"workdir,omitempty"          json:"workdir,omitempty"`          // 工作目录，tool_call 步骤执行时的默认目录
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

	mu sync.Mutex `json:"-"` // 保护 StepStates/StepOutputs/Logs 等字段的并发访问

	FailureStrategy string `json:"-"` // 运行时：失败策略（stop/continue），不持久化
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
	inst.mu.Lock()
	inst.Logs = append(inst.Logs, LogEntry{
		Timestamp: time.Now(),
		StepID:    stepID,
		Level:     level,
		Message:   message,
	})
	inst.mu.Unlock()
}

// StepState 记录单个步骤的执行状态。
type StepState struct {
	Name          string         `json:"name,omitempty"`           // 步骤显示名称
	Status        string         `json:"status"`                   // 步骤状态
	StartedAt     *time.Time     `json:"started_at,omitempty"`     // 开始时间
	FinishedAt    *time.Time     `json:"finished_at,omitempty"`    // 结束时间
	Error         string         `json:"error,omitempty"`          // 错误信息
	Attempts      int            `json:"attempts"`                 // 执行次数（含重试）
	ResolvedInput *ResolvedInput `json:"resolved_input,omitempty"` // 渲染后的输入参数
}

// ResolvedInput 存储步骤执行时的渲染后输入参数。
// 用于在实例详情中展示实际执行的 prompt 和 args（模板变量已替换为实际值）。
type ResolvedInput struct {
	Prompt string         `json:"prompt,omitempty"` // 渲染后的提示词
	Args   map[string]any `json:"args,omitempty"`   // 渲染后的工具参数
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

// ToolSchemaProvider 根据工具名称返回该工具的参数 JSON Schema。
// 返回 (nil, false) 表示工具不存在。
// 由调用方（gateway）注入实际的工具注册表查询逻辑。
type ToolSchemaProvider func(toolName string) (map[string]any, bool)

// Validate 校验工作流定义是否合法。
// 检查名称非空、至少一个步骤、步骤 ID 唯一、动作类型合法等。
func (w *Workflow) Validate() error {
	return w.ValidateWithToolSchema(nil)
}

// ValidateWithToolSchema 校验工作流定义是否合法。
// 当 toolSchema 非空时，会额外校验 tool_call 步骤的必填参数。
func (w *Workflow) ValidateWithToolSchema(toolSchema ToolSchemaProvider) error {
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
	outputKeys := make(map[string]string) // stepID -> OutputKey
	for _, step := range w.Steps {
		if err := validateStep(step, ids); err != nil {
			return err
		}
		collectOutputKeys(step, outputKeys)
	}

	// 校验模板引用：{{.vars.key}} 和 {{.step_id.key}}
	if err := validateTemplateRefs(w, outputKeys); err != nil {
		return err
	}

	// 校验 tool_call 步骤的必填参数
	if toolSchema != nil {
		if err := validateToolCallArgs(w, toolSchema); err != nil {
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
		step.Action != "if" && step.Action != "notify" {
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
	if step.Action == "notify" && step.Message == "" {
		return validationError(fmt.Sprintf("工作流校验：步骤「%s」需要提供 message", label))
	}

	// 校验 duration 格式字段
	if step.Delay != "" {
		if _, err := time.ParseDuration(step.Delay); err != nil {
			return validationError(fmt.Sprintf("工作流校验：步骤「%s」delay '%s' 格式无效（如 5s、1m30s）", label, step.Delay))
		}
	}
	if step.Timeout != "" {
		d, err := time.ParseDuration(step.Timeout)
		if err != nil {
			return validationError(fmt.Sprintf("工作流校验：步骤「%s」timeout '%s' 格式无效（如 30s、1m）", label, step.Timeout))
		}
		if d < time.Second {
			return validationError(fmt.Sprintf("工作流校验：步骤「%s」timeout '%s' 过小，最小允许 1s", label, step.Timeout))
		}
	}

	// enabled 为 *bool，yaml 反序列化只接受 bool/nil，无需额外格式校验

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

// templateRefRe 匹配 {{.xxx.yyy}} 模板引用。
var templateRefRe = regexp.MustCompile(`\{\{\.([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)\}\}`)

// templateFuncRefRe 匹配 {{.fn.xxx}} 模板函数引用（可能包含参数，如 {{.fn.now_tz "Asia/Shanghai"}}）。
var templateFuncRefRe = regexp.MustCompile(`\{\{\.fn\.([a-zA-Z0-9_]+)[^}]*\}\}`)

// validFuncNames 是支持的模板函数名称列表。
var validFuncNames = map[string]bool{
	"now":     true,
	"now_tz":  true,
	"date":    true,
	"date_tz": true,
	"unix":    true,
	"env":     true,
}

// collectOutputKeys 递归收集步骤及其子步骤的 OutputKey 映射。
func collectOutputKeys(step Step, m map[string]string) {
	if step.OutputKey != "" {
		m[step.ID] = step.OutputKey
	}
	for _, sub := range step.Parallel {
		collectOutputKeys(sub, m)
	}
	for _, sub := range step.IfTrue {
		collectOutputKeys(sub, m)
	}
	for _, sub := range step.IfFalse {
		collectOutputKeys(sub, m)
	}
}

// validateTemplateRefs 校验步骤中的模板引用是否合法。
// 检查 {{.vars.key}} 引用的 key 是否在 Vars 中定义，
// 以及 {{.step_id.key}} 引用的 step_id 是否存在。
func validateTemplateRefs(w *Workflow, outputKeys map[string]string) error {
	for _, step := range w.Steps {
		if err := validateStepTemplateRefs(step, w.Vars, outputKeys); err != nil {
			return err
		}
	}
	return nil
}

// validateStepTemplateRefs 递归校验步骤中的模板引用。
func validateStepTemplateRefs(step Step, vars map[string]string, outputKeys map[string]string) error {
	label := stepLabel(step)
	check := func(text, field string) error {
		matches := templateRefRe.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			refID, refKey := m[1], m[2]
			switch refID {
			case "vars":
				if _, ok := vars[refKey]; !ok {
					return validationError(fmt.Sprintf(
						"工作流校验：步骤「%s」%s 引用了不存在的变量 {{.vars.%s}}",
						label, field, refKey))
				}
			case "self":
				if refKey != "id" && refKey != "name" {
					return validationError(fmt.Sprintf(
						"工作流校验：步骤「%s」%s 引用了不存在的自身属性 {{.self.%s}}（仅支持 id 和 name）",
						label, field, refKey))
				}
			case "fn":
				// fn 引用由 templateFuncRefRe 单独校验，此处跳过
			default:
				actualKey, ok := outputKeys[refID]
				if !ok {
					return validationError(fmt.Sprintf(
						"工作流校验：步骤「%s」%s 引用了不存在的步骤 {{.%s.%s}}",
						label, field, refID, refKey))
				}
				if refKey != actualKey {
					return validationError(fmt.Sprintf(
						"工作流校验：步骤「%s」%s 引用了步骤 '%s' 不存在的输出键 '{{.%s.%s}}'（该步骤的 output_key 为 '%s'）",
						label, field, refID, refID, refKey, actualKey))
				}
			}
		}

		funcMatches := templateFuncRefRe.FindAllStringSubmatch(text, -1)
		for _, m := range funcMatches {
			funcName := m[1]
			if !validFuncNames[funcName] {
				return validationError(fmt.Sprintf(
					"工作流校验：步骤「%s」%s 引用了不存在的模板函数 {{.fn.%s}}",
					label, field, funcName))
			}
		}

		return nil
	}

	if err := check(step.Prompt, "prompt"); err != nil {
		return err
	}
	if err := check(step.When, "when"); err != nil {
		return err
	}
	for k, v := range step.Args {
		if s, ok := v.(string); ok {
			if err := check(s, fmt.Sprintf("args[%s]", k)); err != nil {
				return err
			}
		}
	}

	for _, sub := range step.Parallel {
		if err := validateStepTemplateRefs(sub, vars, outputKeys); err != nil {
			return err
		}
	}
	for _, sub := range step.IfTrue {
		if err := validateStepTemplateRefs(sub, vars, outputKeys); err != nil {
			return err
		}
	}
	for _, sub := range step.IfFalse {
		if err := validateStepTemplateRefs(sub, vars, outputKeys); err != nil {
			return err
		}
	}
	return nil
}

// StepLabel 返回步骤的可读标签，优先使用 Name，为空时回退到 #ID。
func StepLabel(step Step) string {
	if step.Name != "" {
		return step.Name
	}
	return "#" + step.ID
}

// stepLabel 是 StepLabel 的内部别名，供同包使用（校验错误等，保持原有格式）。
func stepLabel(step Step) string {
	if step.Name != "" {
		return step.Name
	}
	return step.ID
}

// validateToolCallArgs 校验 tool_call 步骤的必填参数是否已提供。
// 通过 ToolSchemaProvider 查询工具的 JSON Schema，检查 required 字段是否在 args 中存在。
func validateToolCallArgs(w *Workflow, provider ToolSchemaProvider) error {
	for _, step := range w.Steps {
		if err := validateStepToolCallArgs(step, provider); err != nil {
			return err
		}
	}
	return nil
}

// validateStepToolCallArgs 递归校验步骤中 tool_call 的必填参数。
func validateStepToolCallArgs(step Step, provider ToolSchemaProvider) error {
	if step.Action == "tool_call" && step.Tool != "" {
		schema, ok := provider(step.Tool)
		if !ok {
			// 工具不存在时不阻止保存（可能在运行时才注册），但给出提示
			return validationError(fmt.Sprintf(
				"工作流校验：步骤「%s」引用了不存在的工具 '%s'",
				stepLabel(step), step.Tool))
		}
		if err := checkRequiredArgs(step, schema); err != nil {
			return err
		}
	}
	// 递归校验子步骤
	for _, sub := range step.Parallel {
		if err := validateStepToolCallArgs(sub, provider); err != nil {
			return err
		}
	}
	for _, sub := range step.IfTrue {
		if err := validateStepToolCallArgs(sub, provider); err != nil {
			return err
		}
	}
	for _, sub := range step.IfFalse {
		if err := validateStepToolCallArgs(sub, provider); err != nil {
			return err
		}
	}
	return nil
}

// checkRequiredArgs 检查步骤的 args 是否包含工具 Schema 中声明的所有必填参数，且值不为空。
func checkRequiredArgs(step Step, schema map[string]any) error {
	required, _ := schema["required"].([]string)
	if len(required) == 0 {
		return nil
	}
	label := stepLabel(step)
	for _, key := range required {
		val, exists := step.Args[key]
		if !exists || isEmptyValue(val) {
			return validationError(fmt.Sprintf(
				"工作流校验：步骤「%s」缺少工具 '%s' 的必填参数 '%s'",
				label, step.Tool, key))
		}
	}
	return nil
}

// isEmptyValue 判断参数值是否为空（nil、空字符串、零值）。
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case float64:
		return val == 0
	case int:
		return val == 0
	case bool:
		return false // bool 类型不算空值
	default:
		return false
	}
}

func (e validationError) Error() string { return string(e) }
