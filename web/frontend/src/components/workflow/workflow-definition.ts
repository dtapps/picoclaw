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

/** 将我们的 Workflow 转换为 SWD Definition */
export function workflowToDefinition(wf: Workflow): Definition {
  let triggerType = "manual"
  let cronExpr = ""
  let eventKind = ""
  let triggerTZ = ""

  for (const t of wf.triggers || []) {
    if (t.cron) {
      triggerType = "cron"
      cronExpr = t.cron
      triggerTZ = t.tz || ""
    } else if (t.event) {
      triggerType = "event"
      eventKind = t.event
    }
  }

  const sequence: Step[] = (wf.steps || []).map((s) =>
    stepToSwd(s),
  )

  return {
    properties: {
      name: wf.name,
      description: wf.description,
      triggerType,
      cronExpr,
      eventKind,
      triggerTZ,
      failureStrategy: wf.config?.failure_strategy || "stop",
      vars: wf.vars || {},
    },
    sequence,
  }
}

/** 单个 Step 转换为 SWD Step */
function stepToSwd(s: WorkflowStep): Step {
  // if 步骤使用 BranchedStep 格式
  if (s.action === "if") {
    return {
      id: s.id,
      componentType: COMPONENT_SWITCH,
      type: "if",
      name: s.id,
      properties: {
        action: "if",
        when: s.when || "",
      },
      branches: {
        true: (s.if_true || []).map(stepToSwd),
        false: (s.if_false || []).map(stepToSwd),
      },
    } as BranchedStep
  }

  // parallel 步骤使用 BranchedStep 格式，每个子步骤作为一个分支（并排显示）
  if (s.action === "parallel") {
    const subSteps = s.parallel || []
    const branches: Record<string, Step[]> = {}
    subSteps.forEach((sub, i) => {
      branches[`step_${i}`] = [stepToSwd(sub)]
    })
    return {
      id: s.id,
      componentType: COMPONENT_SWITCH,
      type: "parallel",
      name: s.id,
      properties: {
        action: "parallel",
        when: s.when || "",
        output_key: s.output_key || "",
      },
      branches,
    } as BranchedStep
  }

  return {
    id: s.id,
    componentType: COMPONENT_TASK,
    type: s.action,
    name: s.id,
    properties: {
      action: s.action,
      prompt: s.prompt || "",
      tool: s.tool || "",
      when: s.when || "",
      output_key: s.output_key || "",
      timeout: s.timeout || "",
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

  const triggers: Trigger[] = []
  if (props.triggerType === "cron" && props.cronExpr) {
    triggers.push({ cron: props.cronExpr as string, tz: (props.triggerTZ as string) || undefined })
  } else if (props.triggerType === "event" && props.eventKind) {
    triggers.push({ event: props.eventKind as string })
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
    config: { failure_strategy: (props.failureStrategy as "stop" | "continue") || "stop" },
  }
}

/** SWD Step 转换为我们的 Step */
function swdToStep(s: Step): WorkflowStep {
  // BranchedStep（if 条件）
  if (s.componentType === COMPONENT_SWITCH && "branches" in s) {
    const branched = s as BranchedStep
    return {
      id: s.name,
      action: "if",
      when: (s.properties.when as string) || "",
      if_true: (branched.branches.true || []).map(swdToStep),
      if_false: (branched.branches.false || []).map(swdToStep),
    }
  }

  // BranchedStep（parallel 并行）
  if (s.componentType === COMPONENT_SWITCH && s.type === "parallel" && "branches" in s) {
    const branched = s as BranchedStep
    const parallelSteps = Object.values(branched.branches || {})
      .flat()
      .map(swdToStep)
    return {
      id: s.name,
      action: "parallel",
      when: (s.properties.when as string) || undefined,
      output_key: (s.properties.output_key as string) || undefined,
      parallel: parallelSteps,
    }
  }

  // 普通任务步骤
  const sp = s.properties
  const action = (sp.action || s.type) as WorkflowStep["action"]
  return {
    id: s.name,
    action,
    prompt: action === "agent_prompt" ? (sp.prompt as string) : undefined,
    tool: action === "tool_call" ? (sp.tool as string) : undefined,
    when: (sp.when as string) || undefined,
    output_key: (sp.output_key as string) || undefined,
    timeout: (sp.timeout as string) || undefined,
  }
}

/** 创建空白 Definition */
export function createEmptyDefinition(): Definition {
  return {
    properties: {
      name: "",
      description: "",
      triggerType: "manual",
      cronExpr: "",
      eventKind: "",
      triggerTZ: "",
      failureStrategy: "stop",
      vars: {},
    },
    sequence: [],
  }
}

export { wrapDefinition, type WrappedDefinition }
