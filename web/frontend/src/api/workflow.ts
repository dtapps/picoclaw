import { launcherFetch } from "./http"

// --- 类型定义 ---

export interface Trigger {
  cron?: string
  at?: string
  interval?: string
  event?: string
  event_filters?: Record<string, string>
  event_mapping?: Record<string, string>
  tz?: string
}

// 标准事件类型
export const EventTypes = {
  // 工具相关事件
  TOOL_EXEC_START: "agent.tool.exec_start",
  TOOL_EXEC_END: "agent.tool.exec_end",
  TOOL_EXEC_ERROR: "agent.tool.exec_error",

  // Agent 相关事件
  AGENT_PROMPT_START: "agent.prompt.start",
  AGENT_PROMPT_END: "agent.prompt.end",
  AGENT_RESPONSE: "agent.response",

  // 工作流相关事件
  WORKFLOW_START: "workflow.instance.start",
  WORKFLOW_COMPLETE: "workflow.instance.complete",
  WORKFLOW_ERROR: "workflow.instance.error",

  // 系统相关事件
  SYSTEM_STARTUP: "system.startup",
  SYSTEM_SHUTDOWN: "system.shutdown",
} as const

export type EventType = (typeof EventTypes)[keyof typeof EventTypes]

// 事件类型分组
export const EventTypeGroups = [
  {
    label: "工具事件",
    options: [
      { value: EventTypes.TOOL_EXEC_START, label: "工具开始执行", description: "当工具开始执行时触发" },
      { value: EventTypes.TOOL_EXEC_END, label: "工具执行完成", description: "当工具成功执行完成时触发" },
      { value: EventTypes.TOOL_EXEC_ERROR, label: "工具执行错误", description: "当工具执行出错时触发" },
    ],
  },
  {
    label: "Agent 事件",
    options: [
      { value: EventTypes.AGENT_PROMPT_START, label: "Agent 开始处理", description: "当 Agent 开始处理提示词时触发" },
      { value: EventTypes.AGENT_PROMPT_END, label: "Agent 处理完成", description: "当 Agent 完成提示词处理时触发" },
      { value: EventTypes.AGENT_RESPONSE, label: "Agent 响应", description: "当 Agent 产生响应时触发" },
    ],
  },
  {
    label: "工作流事件",
    options: [
      { value: EventTypes.WORKFLOW_START, label: "工作流开始", description: "当工作流实例开始时触发" },
      { value: EventTypes.WORKFLOW_COMPLETE, label: "工作流完成", description: "当工作流实例成功完成时触发" },
      { value: EventTypes.WORKFLOW_ERROR, label: "工作流错误", description: "当工作流实例执行出错时触发" },
    ],
  },
  {
    label: "系统事件",
    options: [
      { value: EventTypes.SYSTEM_STARTUP, label: "系统启动", description: "当系统启动时触发" },
      { value: EventTypes.SYSTEM_SHUTDOWN, label: "系统关闭", description: "当系统关闭时触发" },
    ],
  },
]

export interface Step {
  id: string
  name?: string
  action: "agent_prompt" | "tool_call" | "parallel" | "if" | "notify"
  prompt?: string
  tool?: string
  args?: Record<string, unknown>
  parallel?: Step[]
  if_true?: Step[]
  if_false?: Step[]
  message?: string
  when?: string
  delay?: string
  retry?: { max_attempts: number; delay: string }
  timeout?: string
  output_key?: string
  workdir?: string
  enabled?: boolean
}

export interface WorkflowConfig {
  failure_strategy?: string
  notify_channel?: string
  notify_chat_id?: string
  workdir?: string
}

export interface Workflow {
  name: string
  description: string
  enabled: boolean
  triggers: Trigger[]
  vars?: Record<string, string>
  steps: Step[]
  config: WorkflowConfig
  createdAt: string
  updatedAt: string
  nextRunAt?: string
  lastRunAt?: string
  lastRunStatus?: "running" | "success" | "failed"
}

export interface WorkflowListItem {
  name: string
  description: string
  enabled: boolean
  trigger_type: string
  triggers: Trigger[]
  step_count: number
  vars?: Record<string, string>
}

export interface ResolvedInput {
  prompt?: string
  args?: Record<string, unknown>
}

export interface StepState {
  name?: string
  status: string
  started_at?: string
  finished_at?: string
  error?: string
  attempts: number
  resolved_input?: ResolvedInput
}

export interface LogEntry {
  timestamp: string
  step_id?: string
  level: string
  message: string
}

export interface WorkflowInstance {
  id: string
  workflow_name: string
  status: string
  step_states: Record<string, StepState>
  step_outputs: Record<string, Record<string, unknown>>
  trigger_type: string
  channel?: string
  chat_id?: string
  logs?: LogEntry[]
  started_at: string
  finished_at?: string
  error?: string
}

// --- API 函数 ---

export async function getWorkflows(): Promise<{
  workflows: WorkflowListItem[]
}> {
  const res = await launcherFetch("/api/workflows")
  if (!res.ok) throw new Error("Failed to fetch workflows")
  return res.json()
}

export async function getWorkflow(name: string): Promise<Workflow> {
  const res = await launcherFetch(`/api/workflows/${encodeURIComponent(name)}`)
  if (!res.ok) throw new Error("Failed to fetch workflow")
  return res.json()
}

export async function createWorkflow(
  workflow: CreateWorkflowRequest,
): Promise<Workflow> {
  const res = await launcherFetch("/api/workflows", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(workflow),
  })
  if (!res.ok) {
    const msg = await res.text().catch(() => "")
    throw new Error(msg || "Failed to create workflow")
  }
  return res.json()
}

export async function updateWorkflow(
  name: string,
  data: UpdateWorkflowRequest,
): Promise<Workflow> {
  const res = await launcherFetch(`/api/workflows/${encodeURIComponent(name)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
  if (!res.ok) {
    const msg = await res.text().catch(() => "")
    throw new Error(msg || "Failed to update workflow")
  }
  return res.json()
}

export async function deleteWorkflow(name: string): Promise<void> {
  const res = await launcherFetch(`/api/workflows/${encodeURIComponent(name)}`, {
    method: "DELETE",
  })
  if (!res.ok) throw new Error("Failed to delete workflow")
}

export async function runWorkflow(
  name: string,
): Promise<{ instance_id: string; status: string }> {
  const res = await launcherFetch(
    `/api/workflows/${encodeURIComponent(name)}/run`,
    { method: "POST" },
  )
  if (!res.ok) throw new Error("Failed to run workflow")
  return res.json()
}

export async function stopWorkflow(name: string): Promise<void> {
  const res = await launcherFetch(
    `/api/workflows/${encodeURIComponent(name)}/stop`,
    { method: "POST" },
  )
  if (!res.ok) throw new Error("Failed to stop workflow")
}

export async function toggleWorkflow(
  name: string,
  enabled: boolean,
): Promise<void> {
  const res = await launcherFetch(
    `/api/workflows/${encodeURIComponent(name)}/toggle`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    },
  )
  if (!res.ok) throw new Error("Failed to toggle workflow")
}

export async function getWorkflowInstances(
  name: string,
): Promise<{ instances: WorkflowInstance[] }> {
  const res = await launcherFetch(
    `/api/workflows/${encodeURIComponent(name)}/instances`,
  )
  if (!res.ok) throw new Error("Failed to fetch workflow instances")
  return res.json()
}

export async function getWorkflowInstance(
  name: string,
  id: string,
): Promise<WorkflowInstance> {
  const res = await launcherFetch(
    `/api/workflows/${encodeURIComponent(name)}/instances/${encodeURIComponent(id)}`,
  )
  if (!res.ok) throw new Error("Failed to fetch workflow instance")
  return res.json()
}

export async function deleteWorkflowInstance(
  name: string,
  id: string,
): Promise<void> {
  const res = await launcherFetch(
    `/api/workflows/${encodeURIComponent(name)}/instances/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  )
  if (!res.ok) throw new Error("Failed to delete workflow instance")
}

/** 获取系统已注册的工具名称列表（从 agent registry 直接读取，名称准确） */
export async function getRegisteredTools(): Promise<string[]> {
  const res = await launcherFetch("/api/workflow/registered-tools")
  if (!res.ok) throw new Error("Failed to fetch registered tools")
  const data = await res.json()
  return data.tools || []
}

// --- SSE 实时事件 ---

export interface WorkflowStepEvent {
  event: string
  payload: {
    step_id?: string
    action?: string
    status?: string
    workflow?: string
    trigger?: string
    error?: string
  }
  time: string
}

/**
 * 创建工作流实例的 SSE 连接，返回 EventSource 实例。
 * 用于实时监听步骤状态变更和实例完成事件。
 */
export function createInstanceStream(
  name: string,
  id: string,
): EventSource {
  const url = `/api/workflows/${encodeURIComponent(name)}/instances/${encodeURIComponent(id)}/stream`
  return new EventSource(url)
}

export async function importWorkflow(
  yamlContent: string,
): Promise<Workflow> {
  const res = await launcherFetch("/api/workflows/import", {
    method: "POST",
    headers: { "Content-Type": "text/yaml" },
    body: yamlContent,
  })
  if (!res.ok) {
    const msg = await res.text().catch(() => "")
    throw new Error(msg || "Failed to import workflow")
  }
  return res.json()
}

// --- 请求类型 ---

export interface CreateWorkflowRequest {
  name: string
  description: string
  triggers: Trigger[]
  vars?: Record<string, string>
  steps: Step[]
  config?: WorkflowConfig
}

export interface UpdateWorkflowRequest {
  description?: string
  triggers?: Trigger[]
  vars?: Record<string, string>
  steps?: Step[]
  config?: WorkflowConfig
}

// --- 辅助函数 ---

export function formatTriggerType(triggers: Trigger[]): string {
  for (const t of triggers) {
    if (t.cron) return "cron"
    if (t.event) return "event"
  }
  return "manual"
}

export interface CronTaskInfo {
  workflow_name: string
  trigger_type: "cron" | "at" | "interval"
  schedule: string
  timezone: string
  next_run: string
}

export async function getCronTasks(): Promise<{ tasks: CronTaskInfo[] }> {
  const res = await launcherFetch("/api/workflow/cron-tasks")
  if (!res.ok) throw new Error("Failed to fetch cron tasks")
  return res.json()
}

export function formatTriggerDescription(triggers: Trigger[], t?: (key: string, fallback: string) => string): string {
  if (triggers.length === 0) {
    return t ? t("pages.workflows.trigger_manual", "Manual") : "Manual"
  }
  return triggers
    .map((tr) => {
      if (tr.cron) return t ? `${t("pages.workflows.trigger_cron", "Cron")}: ${tr.cron}` : `Cron: ${tr.cron}`
      if (tr.event) return t ? `${t("pages.workflows.trigger_event", "Event")}: ${tr.event}` : `Event: ${tr.event}`
      return t ? t("pages.workflows.trigger_manual", "Manual") : "Manual"
    })
    .join(", ")
}

export function formatStepAction(action: string): string {
  switch (action) {
    case "agent_prompt":
      return "Agent Prompt"
    case "tool_call":
      return "Tool Call"
    case "parallel":
      return "Parallel"
    case "if":
      return "If"
    default:
      return action
  }
}

export function formatInstanceStatus(status: string, t?: (key: string, fallback: string) => string): string {
  if (t) {
    switch (status) {
      case "pending":
        return t("pages.workflows.status_pending", "Pending")
      case "running":
        return t("pages.workflows.status_running", "Running")
      case "completed":
        return t("pages.workflows.status_completed", "Completed")
      case "failed":
        return t("pages.workflows.status_failed", "Failed")
      case "canceled":
        return t("pages.workflows.status_canceled", "Canceled")
      case "skipped":
        return t("pages.workflows.status_skipped", "Skipped")
      default:
        return status
    }
  }
  switch (status) {
    case "pending":
      return "Pending"
    case "running":
      return "Running"
    case "completed":
      return "Completed"
    case "failed":
      return "Failed"
    case "canceled":
      return "Canceled"
    case "skipped":
      return "Skipped"
    default:
      return status
  }
}

export function getInstanceStatusColor(status: string): string {
  switch (status) {
    case "completed":
      return "text-green-600"
    case "running":
      return "text-blue-600"
    case "failed":
      return "text-red-600"
    case "canceled":
      return "text-yellow-600"
    default:
      return "text-muted-foreground"
  }
}
