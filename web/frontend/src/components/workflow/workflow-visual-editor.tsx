import { useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useTheme } from "@/hooks/use-theme"
import type {
  BranchedStep,
  Definition,
  PlaceholderConfiguration,
  Step,
  StepsConfiguration,
  ToolboxConfiguration,
  ValidatorConfiguration,
} from "sequential-workflow-designer"
import { StepsDesignerExtension, DefinitionWalker } from "sequential-workflow-designer"
import {
  SequentialWorkflowDesigner,
  useRootEditor,
  useStepEditor,
  type WrappedDefinition,
} from "sequential-workflow-designer-react"

import "sequential-workflow-designer/css/designer.css"
import "sequential-workflow-designer/css/designer-light.css"
import "sequential-workflow-designer/css/designer-dark.css"
import "./workflow-editor.css"

import {
  COMPONENT_TASK,
  COMPONENT_SWITCH,
} from "./workflow-definition"

// --- 步骤图标 ---

const ICON_AGENT_PROMPT = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><path d="M12 7v4"/><path d="M8 16h0"/><path d="M16 16h0"/></svg>')}`

const ICON_TOOL_CALL = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>')}`

const ICON_IF = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 3v12"/><path d="M18 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"/><path d="M6 21a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"/><path d="M15 6H9"/><path d="M9 18H3"/></svg>')}`

const ICON_PARALLEL = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2v4"/><path d="M12 18v4"/><path d="M4 8h16"/><path d="M4 16h16"/><path d="M8 8v8"/><path d="M16 8v8"/></svg>')}`

// --- If 步骤编辑器（结构化条件表单）---

/** 解析 when 表达式，提取结构化字段 */
function parseWhenExpression(when: string): {
  type: "prev_success" | "prev_error" | "output_equals"
  stepId: string
  outputKey: string
  value: string
} {
  if (when === "on_success") return { type: "prev_success", stepId: "", outputKey: "", value: "" }
  if (when === "on_error") return { type: "prev_error", stepId: "", outputKey: "", value: "" }
  // 解析 {{.step_id.key}} == value
  const match = when.match(/^\{\{\.(.+?)\.(.+?)\}\}\s*==\s*(.+)$/)
  if (match) {
    return { type: "output_equals", stepId: match[1], outputKey: match[2], value: match[3].trim() }
  }
  return { type: "prev_success", stepId: "", outputKey: "", value: "" }
}

/** 将结构化字段拼回 when 表达式 */
function buildWhenExpression(
  type: "prev_success" | "prev_error" | "output_equals",
  stepId: string,
  outputKey: string,
  value: string,
): string {
  if (type === "prev_success") return "on_success"
  if (type === "prev_error") return "on_error"
  if (stepId && outputKey && value) {
    return `{{.${stepId}.${outputKey}}} == ${value}`
  }
  return ""
}

function IfStepEditor({ labels }: { labels: StepEditorLabels }) {
  const { properties, setProperty } = useStepEditor()
  const whenExpr = (properties.when as string) || ""
  const parsed = parseWhenExpression(whenExpr)

  // 用 state 管理，避免解析循环
  const [condType, setCondType] = useState(parsed.type)
  const [stepId, setStepId] = useState(parsed.stepId)
  const [outputKey, setOutputKey] = useState(parsed.outputKey)
  const [condValue, setCondValue] = useState(parsed.value)

  const handleCondTypeChange = (newType: string) => {
    const t = newType as "prev_success" | "prev_error" | "output_equals"
    setCondType(t)
    setProperty("when", buildWhenExpression(t, stepId, outputKey, condValue))
  }

  const handleFieldChange = (field: "stepId" | "outputKey" | "value", val: string) => {
    if (field === "stepId") setStepId(val)
    if (field === "outputKey") setOutputKey(val)
    if (field === "value") setCondValue(val)
    const s = field === "stepId" ? val : stepId
    const k = field === "outputKey" ? val : outputKey
    const v = field === "value" ? val : condValue
    setProperty("when", buildWhenExpression(condType, s, k, v))
  }

  return (
    <>
      <div className="sqd-editor-field">
        <label>{labels.when} *</label>
        <select value={condType} onChange={(e) => handleCondTypeChange(e.target.value)}>
          <option value="prev_success">{labels.condPrevSuccess}</option>
          <option value="prev_error">{labels.condPrevError}</option>
          <option value="output_equals">{labels.condOutputEquals}</option>
        </select>
      </div>
      {condType === "output_equals" && (
        <>
          <div className="sqd-editor-grid">
            <div className="sqd-editor-field">
              <label>{labels.condStepId}</label>
              <input value={stepId} onChange={(e) => handleFieldChange("stepId", e.target.value)} placeholder="fetch_data" />
            </div>
            <div className="sqd-editor-field">
              <label>{labels.condOutputKey}</label>
              <input value={outputKey} onChange={(e) => handleFieldChange("outputKey", e.target.value)} placeholder="status" />
            </div>
          </div>
          <div className="sqd-editor-grid">
            <div className="sqd-editor-field">
              <label>{labels.condOperator}</label>
              <select value="==" disabled style={{ opacity: 0.7 }}>
                <option value="==">==</option>
              </select>
            </div>
            <div className="sqd-editor-field">
              <label>{labels.condValue}</label>
              <input value={condValue} onChange={(e) => handleFieldChange("value", e.target.value)} placeholder="ok" />
            </div>
          </div>
        </>
      )}
    </>
  )
}

// --- Task 步骤编辑器 ---

interface StepEditorLabels {
  name: string
  action: string
  prompt: string
  tool: string
  outputKey: string
  timeout: string
  agentPrompt: string
  toolCall: string
  parallelLabel: string
  addBranch: string
  removeBranch: string
  ifLabel: string
  when: string
  condPrevSuccess: string
  condPrevError: string
  condOutputEquals: string
  condStepId: string
  condOutputKey: string
  condOperator: string
  condValue: string
}

function StepEditorPanel({ labels, definition }: { labels: StepEditorLabels; definition: Definition }) {
  const { id, type, componentType, name, setName, properties, setProperty, notifyChildrenChanged } = useStepEditor()
  const action = (properties.action as string) || type
  const isIfStep = componentType === COMPONENT_SWITCH && type !== "parallel"
  const isParallelStep = type === "parallel"

  const handleAddBranch = () => {
    const walker = new DefinitionWalker()
    const found = walker.findById(definition, id)
    if (!found) return
    const step = found as BranchedStep
    if (!step.branches) return
    const branchIndex = Object.keys(step.branches).length
    step.branches[`step_${branchIndex}`] = []
    notifyChildrenChanged()
  }

  const handleRemoveBranch = () => {
    const walker = new DefinitionWalker()
    const found = walker.findById(definition, id)
    if (!found) return
    const step = found as BranchedStep
    if (!step.branches) return
    const keys = Object.keys(step.branches)
    if (keys.length <= 2) return
    delete step.branches[keys[keys.length - 1]]
    notifyChildrenChanged()
  }

  return (
    <div className="sqd-editor">
      <div className="sqd-editor-field">
        <label>{labels.name} (ID)</label>
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="step_id" />
      </div>
      {isIfStep ? (
        <IfStepEditor labels={labels} />
      ) : isParallelStep ? (
        <>
          <div className="sqd-editor-field">
            <label>{labels.action}</label>
            <input
              value={labels.parallelLabel}
              readOnly
              className="opacity-70"
            />
          </div>
          <div className="sqd-editor-grid">
            <button type="button" onClick={handleAddBranch} style={{ fontSize: '0.8rem', padding: '4px 8px', border: '1px solid var(--input)', borderRadius: 'var(--radius)', background: 'transparent', color: 'var(--foreground)', cursor: 'pointer' }}>
              + {labels.addBranch}
            </button>
            <button type="button" onClick={handleRemoveBranch} style={{ fontSize: '0.8rem', padding: '4px 8px', border: '1px solid var(--input)', borderRadius: 'var(--radius)', background: 'transparent', color: 'var(--foreground)', cursor: 'pointer' }}>
              − {labels.removeBranch}
            </button>
          </div>
        </>
      ) : (
        <>
          <div className="sqd-editor-field">
            <label>{labels.action}</label>
            <input
              value={action === "agent_prompt" ? labels.agentPrompt : labels.toolCall}
              readOnly
              className="opacity-70"
            />
          </div>
          {action === "agent_prompt" && (
            <div className="sqd-editor-field">
              <label>{labels.prompt}</label>
              <textarea
                value={(properties.prompt as string) || ""}
                onChange={(e) => setProperty("prompt", e.target.value)}
                rows={3}
              />
            </div>
          )}
          {action === "tool_call" && (
            <div className="sqd-editor-field">
              <label>{labels.tool}</label>
              <input
                value={(properties.tool as string) || ""}
                onChange={(e) => setProperty("tool", e.target.value)}
              />
            </div>
          )}
          <div className="sqd-editor-grid">
            <div className="sqd-editor-field">
              <label>{labels.outputKey}</label>
              <input
                value={(properties.output_key as string) || ""}
                onChange={(e) => setProperty("output_key", e.target.value)}
                placeholder="result"
              />
            </div>
            <div className="sqd-editor-field">
              <label>{labels.timeout}</label>
              <input
                value={(properties.timeout as string) || ""}
                onChange={(e) => setProperty("timeout", e.target.value)}
                placeholder="30s"
              />
            </div>
          </div>
        </>
      )}
    </div>
  )
}

// --- 根属性编辑器 ---

interface RootEditorLabels {
  name: string
  description: string
  trigger: string
  triggerManual: string
  triggerCron: string
  triggerEvent: string
  failureStrategy: string
  strategyStop: string
  strategyContinue: string
  vars: string
  varsKey: string
  varsValue: string
  varsAdd: string
  varsRemove: string
}

function RootEditorPanel({ labels }: { labels: RootEditorLabels }) {
  const { properties, setProperty } = useRootEditor()

  const vars = (properties.vars as Record<string, string>) || {}
  const varEntries = Object.entries(vars)

  const handleVarChange = (index: number, field: "key" | "value", val: string) => {
    const entries = Object.entries(vars)
    if (index >= entries.length) return
    const [oldKey, oldValue] = entries[index]
    const newKey = field === "key" ? val : oldKey
    const newValue = field === "value" ? val : oldValue
    // Rebuild preserving order, replacing entry at index
    const rebuilt: Record<string, string> = {}
    for (let i = 0; i < entries.length; i++) {
      if (i === index) {
        // Use a placeholder key if newKey is empty, to keep the entry in the list
        const effectiveKey = newKey || `__var_placeholder_${index}__`
        rebuilt[effectiveKey] = newValue
      } else {
        rebuilt[entries[i][0]] = entries[i][1]
      }
    }
    setProperty("vars", rebuilt)
  }

  const handleAddVar = () => {
    const updated = { ...vars }
    let key = "var_1"
    let n = 1
    while (Object.prototype.hasOwnProperty.call(updated, key)) {
      n++
      key = `var_${n}`
    }
    updated[key] = ""
    setProperty("vars", updated)
  }

  const handleRemoveVar = (index: number) => {
    const entries = Object.entries(vars)
    if (index >= entries.length) return
    const updated: Record<string, string> = {}
    for (let i = 0; i < entries.length; i++) {
      if (i !== index) updated[entries[i][0]] = entries[i][1]
    }
    setProperty("vars", updated)
  }

  // Display key: strip placeholder prefix for editing
  const getDisplayKey = (key: string) => {
    const match = key.match(/^__var_placeholder_(\d+)__$/)
    return match ? "" : key
  }

  return (
    <div className="sqd-editor">
      <div className="sqd-editor-field">
        <label>{labels.name} *</label>
        <input
          value={(properties.name as string) || ""}
          onChange={(e) => setProperty("name", e.target.value)}
          placeholder="daily-report"
        />
      </div>
      <div className="sqd-editor-field">
        <label>{labels.description}</label>
        <textarea
          value={(properties.description as string) || ""}
          onChange={(e) => setProperty("description", e.target.value)}
          rows={2}
        />
      </div>
      <div className="sqd-editor-field">
        <label>{labels.trigger}</label>
        <select
          value={(properties.triggerType as string) || "manual"}
          onChange={(e) => setProperty("triggerType", e.target.value)}
        >
          <option value="manual">{labels.triggerManual}</option>
          <option value="cron">{labels.triggerCron}</option>
          <option value="event">{labels.triggerEvent}</option>
        </select>
      </div>
      {properties.triggerType === "cron" && (
        <div className="sqd-editor-grid">
          <div className="sqd-editor-field">
            <label>Cron</label>
            <input
              value={(properties.cronExpr as string) || ""}
              onChange={(e) => setProperty("cronExpr", e.target.value)}
              placeholder="0 9 * * *"
            />
          </div>
          <div className="sqd-editor-field">
            <label>Timezone</label>
            <input
              value={(properties.triggerTZ as string) || ""}
              onChange={(e) => setProperty("triggerTZ", e.target.value)}
              placeholder="Asia/Shanghai"
            />
          </div>
        </div>
      )}
      {properties.triggerType === "event" && (
        <div className="sqd-editor-field">
          <label>Event</label>
          <input
            value={(properties.eventKind as string) || ""}
            onChange={(e) => setProperty("eventKind", e.target.value)}
            placeholder="agent.tool.exec_end"
          />
        </div>
      )}
      <div className="sqd-editor-field">
        <label>{labels.failureStrategy}</label>
        <select
          value={(properties.failureStrategy as string) || "stop"}
          onChange={(e) => setProperty("failureStrategy", e.target.value)}
        >
          <option value="stop">{labels.strategyStop}</option>
          <option value="continue">{labels.strategyContinue}</option>
        </select>
      </div>
      <div className="sqd-editor-field">
        <label>{labels.vars}</label>
        <div style={{ display: "flex", flexDirection: "column", gap: "4px" }}>
          {varEntries.map(([key, value], index) => (
            <div key={index} className="sqd-editor-grid" style={{ gap: "4px", alignItems: "center" }}>
              <input
                value={getDisplayKey(key)}
                onChange={(e) => handleVarChange(index, "key", e.target.value)}
                placeholder={labels.varsKey}
                style={{ flex: 1 }}
              />
              <input
                value={value}
                onChange={(e) => handleVarChange(index, "value", e.target.value)}
                placeholder={labels.varsValue}
                style={{ flex: 2 }}
              />
              <button
                type="button"
                onClick={() => handleRemoveVar(index)}
                style={{
                  background: "transparent",
                  border: "1px solid var(--input)",
                  borderRadius: "var(--radius)",
                  color: "var(--destructive)",
                  cursor: "pointer",
                  fontSize: "0.75rem",
                  padding: "2px 6px",
                  lineHeight: 1,
                }}
                title={labels.varsRemove}
              >
                ×
              </button>
            </div>
          ))}
          <button
            type="button"
            onClick={handleAddVar}
            style={{
              background: "transparent",
              border: "1px dashed var(--input)",
              borderRadius: "var(--radius)",
              color: "var(--muted-foreground)",
              cursor: "pointer",
              fontSize: "0.75rem",
              padding: "4px 8px",
            }}
          >
            + {labels.varsAdd}
          </button>
        </div>
      </div>
    </div>
  )
}

// --- 工具箱步骤模板 ---

function createTaskStep(action: string, label: string) {
  return {
    componentType: COMPONENT_TASK,
    type: action,
    name: label,
    properties: {
      action,
      prompt: "",
      tool: "",
      when: "",
      output_key: "",
      timeout: "",
    },
  }
}

function createIfStep(label: string) {
  return {
    componentType: COMPONENT_SWITCH,
    type: "if",
    name: label,
    properties: {
      action: "if",
      when: "",
    },
    branches: {
      true: [] as Step[],
      false: [] as Step[],
    },
  }
}

function createParallelStep(label: string): BranchedStep {
  return {
    id: label,
    componentType: COMPONENT_SWITCH,
    type: "parallel",
    name: label,
    properties: {
      action: "parallel",
      when: "",
      output_key: "",
    },
    branches: {
      step_0: [],
      step_1: [],
      step_2: [],
    },
  }
}

// --- 主编辑器组件 ---

interface WorkflowVisualEditorProps {
  value: WrappedDefinition<Definition>
  onChange: (def: WrappedDefinition<Definition>) => void
}

export function WorkflowVisualEditor({ value, onChange }: WorkflowVisualEditorProps) {
  const { t } = useTranslation()
  const { theme } = useTheme()

  const [isToolboxCollapsed, setIsToolboxCollapsed] = useState(false)
  const [isEditorCollapsed, setIsEditorCollapsed] = useState(false)

  const stepEditorLabels: StepEditorLabels = useMemo(() => ({
    name: t("pages.workflows.name", "Name"),
    action: t("pages.workflows.action", "Action"),
    prompt: t("pages.workflows.prompt_placeholder", "Prompt for the agent..."),
    tool: t("pages.workflows.tool_placeholder", "Tool name"),
    outputKey: t("pages.workflows.output_key", "Output Key"),
    timeout: t("pages.workflows.timeout", "Timeout"),
    agentPrompt: t("pages.workflows.trigger_agent", "Agent Prompt"),
    toolCall: t("pages.workflows.trigger_tool", "Tool Call"),
    parallelLabel: t("pages.workflows.parallel", "Parallel"),
    addBranch: t("pages.workflows.add_branch", "Add Branch"),
    removeBranch: t("pages.workflows.remove_branch", "Remove Branch"),
    ifLabel: t("pages.workflows.trigger_if", "If"),
    when: t("pages.workflows.when", "When"),
    condPrevSuccess: t("pages.workflows.cond_prev_success", "Previous step succeeded"),
    condPrevError: t("pages.workflows.cond_prev_error", "Previous step failed"),
    condOutputEquals: t("pages.workflows.cond_output_equals", "Step output equals"),
    condStepId: t("pages.workflows.cond_step_id", "Step ID"),
    condOutputKey: t("pages.workflows.cond_output_key", "Output Key"),
    condOperator: t("pages.workflows.cond_operator", "Operator"),
    condValue: t("pages.workflows.cond_value", "Value"),
  }), [t])

  const rootEditorLabels: RootEditorLabels = useMemo(() => ({
    name: t("pages.workflows.name", "Name"),
    description: t("pages.workflows.description", "Description"),
    trigger: t("pages.workflows.trigger", "Trigger"),
    triggerManual: t("pages.workflows.trigger_manual", "Manual"),
    triggerCron: t("pages.workflows.trigger_cron", "Cron"),
    triggerEvent: t("pages.workflows.trigger_event", "Event"),
    failureStrategy: t("pages.workflows.failure_strategy", "Failure Strategy"),
    strategyStop: t("pages.workflows.strategy_stop", "Stop on failure"),
    strategyContinue: t("pages.workflows.strategy_continue", "Continue on failure"),
    vars: t("pages.workflows.vars", "Variables"),
    varsKey: t("pages.workflows.vars_key", "Key"),
    varsValue: t("pages.workflows.vars_value", "Value"),
    varsAdd: t("pages.workflows.vars_add", "Add Variable"),
    varsRemove: t("pages.workflows.vars_remove", "Remove"),
  }), [t])

  const trueLabel = t("pages.workflows.branch_true", "True")
  const falseLabel = t("pages.workflows.branch_false", "False")

  const extensions = useMemo(() => [
    StepsDesignerExtension.create({
      task: {
        componentType: COMPONENT_TASK,
      },
      switch: {
        componentType: COMPONENT_SWITCH,
        branchNamesResolver: (step) => {
          if (step.type === "parallel") {
            return Object.keys((step as BranchedStep).branches || {})
          }
          return ["true", "false"]
        },
        branchNameLabelResolver: (branchName: string, step) => {
          if (step.type === "parallel") {
            return branchName.replace("step_", "#")
          }
          return branchName === "true" ? trueLabel : falseLabel
        },
      },
    }),
  ], [trueLabel, falseLabel])

  const toolboxConfiguration: ToolboxConfiguration = useMemo(
    () => ({
      labelProvider: (step) => {
        if (step.type === "agent_prompt") return t("pages.workflows.agent_prompt", "Agent Prompt")
        if (step.type === "tool_call") return t("pages.workflows.tool_call", "Tool Call")
        if (step.type === "parallel") return t("pages.workflows.parallel", "Parallel")
        if (step.type === "if") return t("pages.workflows.trigger_if", "If")
        return step.type
      },
      descriptionProvider: (step) => {
        if (step.type === "agent_prompt") return t("pages.workflows.agent_prompt_desc", "Send a prompt to the LLM agent and get a response")
        if (step.type === "tool_call") return t("pages.workflows.tool_call_desc", "Call a registered tool by name with parameters")
        if (step.type === "parallel") return t("pages.workflows.parallel_desc", "Execute multiple sub-steps concurrently")
        if (step.type === "if") return t("pages.workflows.if_desc", "Conditional branch: execute true or false path based on condition")
        return ""
      },
      groups: [
        {
          name: t("pages.workflows.toolbox_steps", "Steps"),
          steps: [
            createTaskStep("agent_prompt", t("pages.workflows.agent_prompt", "Agent Prompt")),
            createTaskStep("tool_call", t("pages.workflows.tool_call", "Tool Call")),
          ],
        },
        {
          name: t("pages.workflows.toolbox_logic", "Logic"),
          steps: [
            createParallelStep(t("pages.workflows.parallel", "Parallel")),
            createIfStep(t("pages.workflows.trigger_if", "If")),
          ],
        },
      ],
    }),
    [t],
  )

  const stepsConfiguration: StepsConfiguration = useMemo(
    () => ({
      iconUrlProvider: (_componentType, type) => {
        if (type === "agent_prompt") return ICON_AGENT_PROMPT
        if (type === "tool_call") return ICON_TOOL_CALL
        if (type === "parallel") return ICON_PARALLEL
        if (type === "if") return ICON_IF
        return null
      },
      isDraggable: () => true,
      isDeletable: () => true,
    }),
    [],
  )

  const validatorConfiguration: ValidatorConfiguration = useMemo(
    () => ({
      step: (step) => step.name.length > 0,
      root: () => true,
    }),
    [],
  )

  const placeholderConfiguration: PlaceholderConfiguration = useMemo(
    () => ({
      canShow: () => true,
    }),
    [],
  )

  const designerRef = useRef<HTMLDivElement>(null)

  // SWD 不会动态响应 theme 变化，手动切换 CSS class
  useEffect(() => {
    const el = designerRef.current?.querySelector(".sqd-designer")
    if (!el) return
    el.classList.remove("sqd-theme-light", "sqd-theme-dark", "sqd-theme-soft")
    el.classList.add(`sqd-theme-${theme}`)
  }, [theme])

  // 把工具箱步骤的 description (title) 复制到 .sqd-toolbox-item-text 的 data-desc 上
  // 这样 CSS ::after { content: attr(data-desc) } 才能读取到
  useEffect(() => {
    const container = designerRef.current
    if (!container) return

    const syncDescs = () => {
      container.querySelectorAll(".sqd-toolbox-item[title]").forEach((item) => {
        const textEl = item.querySelector(".sqd-toolbox-item-text")
        if (textEl) {
          textEl.setAttribute("data-desc", (item as HTMLElement).title)
        }
      })
    }

    syncDescs()
    const observer = new MutationObserver(syncDescs)
    observer.observe(container, { childList: true, subtree: true })
    return () => observer.disconnect()
  }, [])

  return (
    <div ref={designerRef} style={{ width: "100%", height: "100%" }}>
      <SequentialWorkflowDesigner
        definition={value}
        onDefinitionChange={onChange}
        stepsConfiguration={stepsConfiguration}
        toolboxConfiguration={toolboxConfiguration}
        validatorConfiguration={validatorConfiguration}
        placeholderConfiguration={placeholderConfiguration}
        extensions={extensions}
        theme={theme}
        controlBar={true}
        contextMenu={true}
        isToolboxCollapsed={isToolboxCollapsed}
        onIsToolboxCollapsedChanged={setIsToolboxCollapsed}
        isEditorCollapsed={isEditorCollapsed}
        onIsEditorCollapsedChanged={setIsEditorCollapsed}
        rootEditor={<RootEditorPanel labels={rootEditorLabels} />}
        stepEditor={<StepEditorPanel labels={stepEditorLabels} definition={value.value} />}
      />
    </div>
  )
}

