import type {
  BranchedStep,
  Definition,
  Step,
} from "sequential-workflow-designer"
import { wrapDefinition, type WrappedDefinition } from "sequential-workflow-designer-react"

import type { Step as WorkflowStep, Trigger, Workflow, WorkflowConfig } from "@/api/workflow"

// --- 组件类型常量 ---

export const COMPONENT_TASK = "task"
export const COMPONENT_SWITCH = "switch"

// --- 模型转换：Workflow ↔ SWD Definition ---

/** 触发器配置 */
export interface TriggerConfig {
  type: "manual" | "cron" | "at" | "interval" | "event"
  cron?: string
  at?: string
  interval?: string
  event?: string
  event_filters?: Record<string, string>
  event_mapping?: Record<string, string>
  tz?: string
}

/** 将我们的 Workflow 转换为 SWD Definition */
export function workflowToDefinition(wf: Workflow): Definition {
  const triggers: TriggerConfig[] = []

  for (const t of wf.triggers || []) {
    if (t.cron) {
      triggers.push({ type: "cron", cron: t.cron, tz: t.tz || "" })
    } else if (t.at) {
      triggers.push({ type: "at", at: t.at, tz: t.tz || "" })
    } else if (t.interval) {
      triggers.push({ type: "interval", interval: t.interval, tz: t.tz || "" })
    } else if (t.event) {
      triggers.push({ type: "event", event: t.event })
    }
  }

  // 如果没有触发器，默认添加一个手动触发器
  if (triggers.length === 0) {
    triggers.push({ type: "manual" })
  }

  const sequence: Step[] = (wf.steps || []).map((s) =>
    stepToSwd(s),
  )

  return {
    properties: {
      name: wf.name,
      description: wf.description,
      triggers,
      failureStrategy: wf.config?.failure_strategy || "stop",
      workdir: wf.config?.workdir || "",
      timeout: wf.config?.timeout || "",
      reuseSession: wf.config?.reuse_session || false,
      history: wf.config?.history?.mode || "default",
      systemPrompt: wf.config?.system_prompt?.mode || "default",
      vars: wf.vars || {},
    },
    sequence,
  }
}

/** 单个 Step 转换为 SWD Step */
function stepToSwd(s: WorkflowStep): Step {
  const displayName = s.name || s.id
  const stepId = s.id

  // if 步骤使用 BranchedStep 格式
  if (s.action === "if") {
    return {
      id: stepId,
      componentType: COMPONENT_SWITCH,
      type: "if",
      name: displayName,
      properties: {
        stepId,
        action: "if",
        when: s.when || "",
        delay: s.delay || "",
        timeout: s.timeout || "",
        enabled: s.enabled ?? true,
        notify_on_start: s.notify_on_start ?? true,
        notify_on_complete: s.notify_on_complete ?? true,
      },
      branches: {
        true: (s.if_true || []).map(stepToSwd),
        false: (s.if_false || []).map(stepToSwd),
      },
    } as BranchedStep
  }

  // parallel 步骤使用 BranchedStep 格式，每个分支可以包含多个步骤
  if (s.action === "parallel") {
    const branches: Record<string, Step[]> = {}
    const branchOrder: string[] = []
    const parallelData = s.parallel

    // 检查是新的分支格式还是旧的扁平格式
    if (parallelData && Array.isArray(parallelData) && parallelData.length > 0) {
      // 格式: { branch: Step[] }[]
      const branchArray = parallelData as unknown as { branch: WorkflowStep[] }[]
      branchArray.forEach((b, i) => {
        const branchId = String(i)
        branches[branchId] = (b.branch || []).map(stepToSwd)
        branchOrder.push(branchId)
      })
    }

    return {
      id: stepId,
      componentType: COMPONENT_SWITCH,
      type: "parallel",
      name: displayName,
      properties: {
        stepId,
        action: "parallel",
        when: s.when || "",
        delay: s.delay || "",
        output_key: s.output_key || "",
        timeout: s.timeout || "",
        enabled: s.enabled ?? true,
        notify_on_start: s.notify_on_start ?? true,
        notify_on_complete: s.notify_on_complete ?? true,
        // 存储分支顺序，确保保存时能恢复正确的顺序
        _branchOrder: branchOrder.join(","),
      },
      branches,
    } as BranchedStep
  }

  // notify 步骤
  if (s.action === "notify") {
    return {
      id: stepId,
      componentType: COMPONENT_TASK,
      type: "notify",
      name: displayName,
      properties: {
        stepId,
        action: "notify",
        message: s.message || "",
        when: s.when || "",
        delay: s.delay || "",
        timeout: s.timeout || "",
        enabled: s.enabled ?? true,
        notify_on_start: s.notify_on_start ?? true,
      },
    }
  }

  return {
    id: stepId,
    componentType: COMPONENT_TASK,
    type: s.action,
    name: displayName,
    properties: {
      stepId,
      action: s.action,
      prompt: s.prompt || "",
      skills: s.skills?.mode || "default",
      tools: s.tools?.mode || "default",
      tool: s.tool || "",
      args: s.args ? JSON.stringify(s.args) : "",
      retry: s.retry ? JSON.stringify(s.retry) : "",
      when: s.when || "",
      delay: s.delay || "",
      output_key: s.output_key || "",
      timeout: s.timeout || "",
      enabled: s.enabled ?? true,
      notify_on_start: s.notify_on_start ?? true,
      notify_on_complete: s.notify_on_complete ?? true,
    },
  }
}

/** 将 SWD Definition 转换回我们的 Workflow 数据 */
export function definitionToWorkflow(def: Definition): {
  name: string
  description: string
  triggers: Trigger[]
  vars: Record<string, string>
  steps: WorkflowStep[]
  config: WorkflowConfig
} {
  const props = def.properties

  // 从 triggers 数组转换
  const triggers: Trigger[] = []
  const triggerConfigs = (props.triggers as TriggerConfig[]) || []
  for (const t of triggerConfigs) {
    if (t.type === "cron" && t.cron) {
      // 始终包含 tz 字段，即使为空字符串
      triggers.push({ cron: t.cron, tz: t.tz })
    } else if (t.type === "at" && t.at) {
      triggers.push({ at: t.at, tz: t.tz })
    } else if (t.type === "interval" && t.interval) {
      triggers.push({ interval: t.interval, tz: t.tz })
    } else if (t.type === "event" && t.event) {
      triggers.push({ event: t.event })
    }
  }

  // Extract vars from properties, filter out placeholder/empty keys for saving
  const rawVars = (props.vars as Record<string, string>) || {}
  const vars: Record<string, string> = {}
  for (const [k, v] of Object.entries(rawVars)) {
    // Skip internal placeholder keys and empty keys
    if (k.trim() && !k.startsWith("__var_placeholder_")) {
      vars[k] = v
    }
  }

  const steps: WorkflowStep[] = def.sequence.map((s) => swdToStep(s))

  return {
    name: props.name as string,
    description: (props.description as string) || "",
    triggers,
    vars,
    steps,
    config: {
      failure_strategy: (props.failureStrategy as "stop" | "continue") || "stop",
      workdir: (props.workdir as string) || undefined,
      timeout: (props.timeout as string) || undefined,
      reuse_session: (props.reuseSession as boolean) || undefined,
      history: props.history ? { mode: props.history as "default" | "off" } : undefined,
      system_prompt: props.systemPrompt ? { mode: props.systemPrompt as "default" | "off" } : undefined,
    },
  }
}

/** SWD Step 转换为我们的 Step */
function swdToStep(s: Step): WorkflowStep {
  // stepId 存在 properties 中，是实际的后端 ID；s.name 是显示名称
  const stepId = ((s.properties.stepId as string) || s.name).replace(/[^a-zA-Z0-9_]/g, "_")
  const displayName = s.name !== stepId ? s.name : undefined

  // BranchedStep（if 条件）
  if (s.componentType === COMPONENT_SWITCH && s.type === "if" && "branches" in s) {
    const branched = s as BranchedStep
    return {
      id: stepId,
      name: displayName,
      action: "if",
      when: (s.properties.when as string) || undefined,
      delay: (s.properties.delay as string) || undefined,
      timeout: (s.properties.timeout as string) || undefined,
      enabled: s.properties.enabled === false ? false : undefined,
      notify_on_start: s.properties.notify_on_start === false ? false : undefined,
      notify_on_complete: s.properties.notify_on_complete === false ? false : undefined,
      if_true: (branched.branches.true || []).map(swdToStep),
      if_false: (branched.branches.false || []).map(swdToStep),
    }
  }

  // BranchedStep（parallel 并行）
  if (s.componentType === COMPONENT_SWITCH && s.type === "parallel" && "branches" in s) {
    const branched = s as BranchedStep
    
    const branches = branched.branches || {}
    const branchKeys = Object.keys(branches)
    
    // 尝试从 _branchOrder 属性恢复顺序，否则使用原始顺序
    const branchOrderStr = (s.properties._branchOrder as string) || ""
    let sortedKeys: string[]
    
    if (branchOrderStr) {
      // 使用保存的顺序
      const orderMap = new Map(branchOrderStr.split(",").map((k, i) => [k, i]))
      sortedKeys = branchKeys.sort((a, b) => {
        const orderA = orderMap.get(a) ?? Infinity
        const orderB = orderMap.get(b) ?? Infinity
        return orderA - orderB
      })
    } else {
      // 回退：按插入顺序（Object.keys 在现代引擎中通常保持插入顺序）
      sortedKeys = branchKeys
    }
    
    const parallelBranches: { branch: WorkflowStep[] }[] = []
    for (const key of sortedKeys) {
      const branchSteps = branches[key]
        .filter((s): s is Step => s != null)
        .map(swdToStep)
        .filter((s): s is WorkflowStep => s != null)
      if (branchSteps.length > 0) {
        parallelBranches.push({ branch: branchSteps })
      }
    }

    return {
      id: stepId,
      name: displayName,
      action: "parallel",
      when: (s.properties.when as string) || undefined,
      delay: (s.properties.delay as string) || undefined,
      timeout: (s.properties.timeout as string) || undefined,
      enabled: s.properties.enabled === false ? false : undefined,
      notify_on_start: s.properties.notify_on_start === false ? false : undefined,
      notify_on_complete: s.properties.notify_on_complete === false ? false : undefined,
      parallel: parallelBranches,
    }
  }

  // 普通任务步骤
  const sp = s.properties
  const action = (sp.action || s.type) as WorkflowStep["action"]

  // 解析 args: 从 SWD properties 中读取 JSON 字符串并还原为对象
  let args: Record<string, unknown> | undefined
  if (action === "tool_call" && sp.args) {
    try {
      const parsed = JSON.parse(sp.args as string)
      if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
        args = parsed as Record<string, unknown>
      }
    } catch {
      // 如果不是有效 JSON，忽略
    }
  }

  // 解析 retry
  let retry: { max_attempts: number; delay: string } | undefined
  if (sp.retry) {
    try {
      const parsed = JSON.parse(sp.retry as string)
      if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed) && typeof parsed.max_attempts === "number") {
        retry = parsed as { max_attempts: number; delay: string }
      }
    } catch {
      // 如果不是有效 JSON，忽略
    }
  }

  return {
    id: stepId,
    name: displayName,
    action,
    prompt: action === "agent_prompt" ? (sp.prompt as string) : undefined,
    skills: action === "agent_prompt" && sp.skills ? { mode: sp.skills as "default" | "off" } : undefined,
    tools: action === "agent_prompt" && sp.tools ? { mode: sp.tools as "default" | "off" } : undefined,
    tool: action === "tool_call" ? (sp.tool as string) : undefined,
    args,
    message: action === "notify" ? (sp.message as string) : undefined,
    retry,
    when: (sp.when as string) || undefined,
    delay: (sp.delay as string) || undefined,
    output_key: (sp.output_key as string) || undefined,
    timeout: (sp.timeout as string) || undefined,
    enabled: sp.enabled === false ? false : undefined,
    notify_on_start: sp.notify_on_start === false ? false : undefined,
    notify_on_complete: action !== "notify" ? (sp.notify_on_complete === false ? false : undefined) : undefined,
  }
}

/** 创建空白 Definition */
export function createEmptyDefinition(): Definition {
  return {
    properties: {
      name: "",
      description: "",
      triggers: [{ type: "manual" }],
      failureStrategy: "stop",
      workdir: "",
      timeout: "",
      reuseSession: false,
      vars: {},
    },
    sequence: [],
  }
}

export { wrapDefinition, type WrappedDefinition }
