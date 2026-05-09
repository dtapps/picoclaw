package tools

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sipeed/picoclaw/pkg/workflow"
)

// WorkflowTool 为 Agent 提供工作流管理能力的工具。
// 支持的动作：list（列表）、run（执行）、stop（停止）、create（创建）、delete（删除）、show（详情）。
type WorkflowTool struct {
	service *workflow.Service // 工作流服务实例
}

// NewWorkflowTool 创建新的 WorkflowTool 实例。
func NewWorkflowTool(service *workflow.Service) (*WorkflowTool, error) {
	return &WorkflowTool{
		service: service,
	}, nil
}

// Name 返回工具名称。
func (t *WorkflowTool) Name() string {
	return "workflow"
}

// Description 返回工具描述信息，供 LLM 理解工具用途。
func (t *WorkflowTool) Description() string {
	return "Manage declarative workflows. Workflows define multi-step procedures with triggers, conditions, and guaranteed execution. Use 'list' to see existing workflows, 'run' to trigger one, 'create' to define a new workflow, and 'delete' to remove one."
}

// Parameters 返回工具的参数定义，遵循 JSON Schema 格式。
// 必选参数：action（操作类型）
// 可选参数：name（工作流名）、description（描述）、steps_yaml（步骤定义）、cron_expr（Cron 表达式）、instance_id（实例 ID）
func (t *WorkflowTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{
					"list",
					"run",
					"stop",
					"create",
					"delete",
					"show",
					"bind",
					"unbind",
					"enable",
					"disable",
					"instances",
				},
				"description": "Action to perform. 'bind' links the current channel for completion notifications; 'unbind' removes it; 'instances' shows execution history.",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Workflow name (for run/stop/delete/show).",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Workflow description (for create).",
			},
			"steps_yaml": map[string]any{
				"type":        "string",
				"description": "YAML string defining the workflow steps (for create). Each step has id, action, prompt/tool, and optional when/retry/timeout/output_key.",
			},
			"cron_expr": map[string]any{
				"type":        "string",
				"description": "Cron expression for the trigger (for create), e.g., '0 9 * * *' for daily at 9am.",
			},
			"vars": map[string]any{
				"type":                 "object",
				"description":          "Workflow variables (for create). Key-value pairs that can be referenced as {{.vars.key}} in step prompts and args.",
				"additionalProperties": map[string]any{"type": "string"},
			},
			"instance_id": map[string]any{
				"type":        "string",
				"description": "Instance ID (for stop).",
			},
		},
		"required": []string{"action"},
	}
}

// Execute 根据动作类型执行对应的操作。
func (t *WorkflowTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, ok := args["action"].(string)
	if !ok {
		return ErrorResult("action is required")
	}

	switch action {
	case "list":
		return t.listWorkflows()
	case "run":
		return t.runWorkflow(ctx, args)
	case "stop":
		return t.stopWorkflow(args)
	case "create":
		return t.createWorkflow(args)
	case "delete":
		return t.deleteWorkflow(args)
	case "show":
		return t.showWorkflow(args)
	case "bind":
		return t.bindWorkflow(ctx, args)
	case "unbind":
		return t.unbindWorkflow(args)
	case "enable":
		return t.enableWorkflow(args, true)
	case "disable":
		return t.enableWorkflow(args, false)
	case "instances":
		return t.listInstances(args)
	default:
		return ErrorResult(fmt.Sprintf("unknown action: %s", action))
	}
}

// listWorkflows 列出所有工作流。
func (t *WorkflowTool) listWorkflows() *ToolResult {
	workflows := t.service.ListWorkflows()
	if len(workflows) == 0 {
		return SilentResult("No workflows defined")
	}

	var sb strings.Builder
	sb.WriteString("Workflows:\n")
	for _, wf := range workflows {
		status := "enabled"
		if !wf.Enabled {
			status = "disabled"
		}
		triggerType := describeTriggers(wf.Triggers)
		sb.WriteString(fmt.Sprintf("- %s (%s, %s, %d steps): %s\n",
			wf.Name, status, triggerType, len(wf.Steps), wf.Description))
	}
	return SilentResult(sb.String())
}

// runWorkflow 手动触发工作流执行。
// 从上下文中提取 channel/chatID，绑定到工作流实例，执行完成后自动通知该频道。
func (t *WorkflowTool) runWorkflow(ctx context.Context, args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return ErrorResult("name is required for run")
	}

	// 从上下文提取频道信息（与 CronTool 一致）
	channel := ToolChannel(ctx)
	chatID := ToolChatID(ctx)

	instanceID, err := t.service.RunWorkflow(ctx, name, channel, chatID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to run workflow '%s': %v", name, err))
	}

	result := fmt.Sprintf("Workflow '%s' triggered, instance: %s", name, instanceID)
	if channel != "" {
		result += fmt.Sprintf(" (bound to channel: %s)", channel)
	}
	return SilentResult(result)
}

// stopWorkflow 停止正在运行的工作流实例。
func (t *WorkflowTool) stopWorkflow(args map[string]any) *ToolResult {
	instanceID, _ := args["instance_id"].(string)
	if instanceID == "" {
		return ErrorResult("instance_id is required for stop")
	}

	if err := t.service.StopInstance(instanceID); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to stop instance '%s': %v", instanceID, err))
	}

	return SilentResult(fmt.Sprintf("Workflow instance '%s' stopped", instanceID))
}

// createWorkflow 通过 YAML 定义创建新工作流。
func (t *WorkflowTool) createWorkflow(args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return ErrorResult("name is required for create")
	}

	description, _ := args["description"].(string)
	stepsYAML, _ := args["steps_yaml"].(string)
	cronExpr, _ := args["cron_expr"].(string)

	// 解析 vars 参数
	var vars map[string]string
	if v, ok := args["vars"].(map[string]any); ok {
		vars = make(map[string]string, len(v))
		for key, val := range v {
			if s, ok := val.(string); ok {
				vars[key] = s
			}
		}
	}

	wf := &workflow.Workflow{
		Name:        name,
		Description: description,
		Vars:        vars,
	}

	// 从 YAML 字符串解析步骤定义
	if stepsYAML != "" {
		var wrapper struct {
			Steps []workflow.Step `yaml:"steps"`
		}
		if err := yaml.Unmarshal([]byte(stepsYAML), &wrapper); err != nil {
			return ErrorResult(fmt.Sprintf("Invalid steps YAML: %v", err))
		}
		wf.Steps = wrapper.Steps
	}

	// 如果指定了 cron 表达式，添加 cron 触发器
	if cronExpr != "" {
		wf.Triggers = append(wf.Triggers, workflow.Trigger{Cron: cronExpr})
	}

	if err := t.service.CreateWorkflow(wf); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create workflow: %v", err))
	}

	return SilentResult(fmt.Sprintf("Workflow '%s' created with %d steps", name, len(wf.Steps)))
}

// deleteWorkflow 删除指定工作流。
func (t *WorkflowTool) deleteWorkflow(args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return ErrorResult("name is required for delete")
	}

	if err := t.service.DeleteWorkflow(name); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to delete workflow '%s': %v", name, err))
	}

	return SilentResult(fmt.Sprintf("Workflow '%s' deleted", name))
}

// showWorkflow 展示指定工作流的详细信息。
func (t *WorkflowTool) showWorkflow(args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return ErrorResult("name is required for show")
	}

	wf, ok := t.service.GetWorkflow(name)
	if !ok {
		return ErrorResult(fmt.Sprintf("Workflow '%s' not found", name))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Workflow: %s\n", wf.Name))
	sb.WriteString(fmt.Sprintf("Description: %s\n", wf.Description))
	sb.WriteString(fmt.Sprintf("Enabled: %v\n", wf.Enabled))
	sb.WriteString(fmt.Sprintf("Triggers: %s\n", describeTriggers(wf.Triggers)))
	if len(wf.Vars) > 0 {
		sb.WriteString("Vars:\n")
		for k, v := range wf.Vars {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}
	sb.WriteString("Steps:\n")
	for i, step := range wf.Steps {
		sb.WriteString(fmt.Sprintf("  %d. [%s] %s", i+1, step.ID, step.Action))
		if step.When != "" {
			sb.WriteString(fmt.Sprintf(" (when: %s)", step.When))
		}
		if step.OutputKey != "" {
			sb.WriteString(fmt.Sprintf(" -> %s", step.OutputKey))
		}
		sb.WriteString("\n")
	}

	return SilentResult(sb.String())
}

// listInstances 列出指定工作流的执行历史。
func (t *WorkflowTool) listInstances(args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return ErrorResult("name is required for instances")
	}

	instances, err := t.service.GetInstances(name)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to get instances for '%s': %v", name, err))
	}

	if len(instances) == 0 {
		return SilentResult(fmt.Sprintf("No instances for workflow '%s'", name))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Instances for '%s':\n", name))
	for _, inst := range instances {
		sb.WriteString(fmt.Sprintf("- %s (%s, %s)", inst.ID, inst.Status, inst.TriggerType))
		if !inst.StartedAt.IsZero() {
			sb.WriteString(fmt.Sprintf(", started: %s", inst.StartedAt.Format("2006-01-02 15:04:05")))
		}
		if inst.Error != "" {
			sb.WriteString(fmt.Sprintf(", error: %s", inst.Error))
		}
		sb.WriteString("\n")
	}
	return SilentResult(sb.String())
}

// bindWorkflow 将当前频道绑定到工作流，执行完成后自动通知该频道。
// 从上下文中提取 channel/chatID，保存到工作流配置。
func (t *WorkflowTool) bindWorkflow(ctx context.Context, args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return ErrorResult("name is required for bind")
	}

	channel := ToolChannel(ctx)
	chatID := ToolChatID(ctx)

	if channel == "" || chatID == "" {
		return ErrorResult("no session context (channel/chat_id not set). Use this command in an active conversation.")
	}

	if err := t.service.BindChannel(name, channel, chatID); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to bind workflow '%s': %v", name, err))
	}

	return SilentResult(
		fmt.Sprintf(
			"Workflow '%s' bound to channel %s (chat_id: %s). Completion notifications will be sent here.",
			name,
			channel,
			chatID,
		),
	)
}

// unbindWorkflow 移除工作流的频道绑定。
func (t *WorkflowTool) unbindWorkflow(args map[string]any) *ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return ErrorResult("name is required for unbind")
	}

	if err := t.service.UnbindChannel(name); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to unbind workflow '%s': %v", name, err))
	}

	return SilentResult(fmt.Sprintf("Workflow '%s' channel binding removed", name))
}

// enableWorkflow 启用或禁用工作流。
func (t *WorkflowTool) enableWorkflow(args map[string]any, enabled bool) *ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return ErrorResult("name is required for enable/disable")
	}

	if err := t.service.SetEnabled(name, enabled); err != nil {
		return ErrorResult(
			fmt.Sprintf(
				"Failed to %s workflow '%s': %v",
				map[bool]string{true: "enable", false: "disable"}[enabled],
				name,
				err,
			),
		)
	}

	status := "enabled"
	if !enabled {
		status = "disabled"
	}
	return SilentResult(fmt.Sprintf("Workflow '%s' %s", name, status))
}

// describeTriggers 生成触发器的可读描述。
func describeTriggers(triggers []workflow.Trigger) string {
	if len(triggers) == 0 {
		return "manual"
	}
	var parts []string
	for _, t := range triggers {
		if t.Cron != "" {
			parts = append(parts, "cron:"+t.Cron)
		}
		if t.Event != "" {
			parts = append(parts, "event:"+t.Event)
		}
	}
	if len(parts) == 0 {
		return "manual"
	}
	return strings.Join(parts, ", ")
}
