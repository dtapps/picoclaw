import { useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
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

import { wrapDefinition } from "./workflow-definition"

import {
  COMPONENT_TASK,
  COMPONENT_SWITCH,
} from "./workflow-definition"

import { getTools, type ToolParamProperty } from "@/api/tools"
import { getMCPConfig, getMCPServerDetails } from "@/api/mcp"

// --- 步骤图标 ---

const ICON_AGENT_PROMPT = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><path d="M12 7v4"/><path d="M8 16h0"/><path d="M16 16h0"/></svg>')}`

const ICON_TOOL_CALL = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>')}`

const ICON_IF = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 3v12"/><path d="M18 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"/><path d="M6 21a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"/><path d="M15 6H9"/><path d="M9 18H3"/></svg>')}`

const ICON_PARALLEL = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2v4"/><path d="M12 18v4"/><path d="M4 8h16"/><path d="M4 16h16"/><path d="M8 8v8"/><path d="M16 8v8"/></svg>')}`

// --- When 条件编辑器（用于 task 步骤）---

function WhenEditor({ labels }: { labels: StepEditorLabels }) {
  const { properties, setProperty } = useStepEditor()
  const whenValue = (properties.when as string) || ""
  const isPreset = whenValue === "" || whenValue === "on_success" || whenValue === "on_error"
  const [useCustomWhen, setUseCustomWhen] = useState(!isPreset && whenValue !== "")

  // 切换步骤时同步 state
  useEffect(() => {
    setUseCustomWhen(!isPreset && whenValue !== "")
  }, [whenValue])

  const handleSelectChange = (v: string) => {
    if (v === "__custom__") {
      setUseCustomWhen(true)
      setProperty("when", "")
    } else {
      setUseCustomWhen(false)
      setProperty("when", v)
    }
  }

  return (
    <div className="sqd-editor-field">
      <label>{labels.when}</label>
      {useCustomWhen ? (
        <input
          value={whenValue}
          onChange={(e) => setProperty("when", e.target.value)}
          placeholder='{{.step.key}} == value'
        />
      ) : (
        <select value={whenValue} onChange={(e) => handleSelectChange(e.target.value)}>
          <option value="">—</option>
          <option value="on_success">on_success</option>
          <option value="on_error">on_error</option>
          <option value="__custom__">{labels.whenCustom}</option>
        </select>
      )}
      {useCustomWhen && (
        <button type="button" className="sqd-text-button" onClick={() => { setUseCustomWhen(false); setProperty("when", "") }}>
          {labels.whenPreset}
        </button>
      )}
    </div>
  )
}

// --- Retry 编辑器 ---

function RetryEditor({ labels }: { labels: StepEditorLabels }) {
  const { properties, setProperty } = useStepEditor()

  const retryValue = (() => {
    try { return JSON.parse((properties.retry as string) || "") || {} } catch { return {} }
  })()

  const handleMaxAttemptsChange = (v: string) => {
    const n = v ? parseInt(v, 10) : undefined
    setProperty("retry", JSON.stringify({ ...retryValue, max_attempts: n && !isNaN(n) ? n : undefined }))
  }

  const handleDelayChange = (v: string) => {
    setProperty("retry", JSON.stringify({ ...retryValue, delay: v || undefined }))
  }

  return (
    <div className="sqd-editor-grid">
      <div className="sqd-editor-field">
        <label>{labels.retry_max_attempts}</label>
        <input
          type="number"
          min="1"
          value={retryValue.max_attempts ?? ""}
          onChange={(e) => handleMaxAttemptsChange(e.target.value)}
          placeholder="1"
        />
      </div>
      <div className="sqd-editor-field">
        <label>{labels.retry_delay}</label>
        <input
          value={retryValue.delay ?? ""}
          onChange={(e) => handleDelayChange(e.target.value)}
          placeholder="10s"
        />
      </div>
    </div>
  )
}

// --- Tool Call 步骤编辑器（工具选择 + 参数配置）---

/** 下拉列表中的工具项，统一内置和 MCP */
interface ToolOption {
  name: string // 运行时名称（作为 value）
  label: string // 显示名称（作为显示文本）
  category: "builtin" | "mcp"
  params: ToolParamDef[] // 工具参数定义
}

/** 工具参数定义 */
interface ToolParamDef {
  name: string
  type: string
  description: string
  required: boolean
}

/** 截断过长描述用于下拉显示 */
function formatToolLabel(name: string, description?: string): string {
  if (!description) return name
  const short = description.length > 40 ? description.slice(0, 40) + "…" : description
  return `${name} — ${short}`
}

/** 将 MCP 工具名转为运行时名称格式 mcp_{server}_{tool} */
function mcpRuntimeName(server: string, tool: string): string {
  const sanitize = (s: string) => s.replace(/[^a-zA-Z0-9_]/g, "_")
  return `mcp_${sanitize(server)}_${sanitize(tool)}`
}

/** 从 JSON Schema properties 提取参数定义 */
function extractParamDefs(schema?: { properties?: Record<string, ToolParamProperty>; required?: string[] }): ToolParamDef[] {
  if (!schema?.properties) return []
  const requiredSet = new Set(schema.required || [])
  return Object.entries(schema.properties).map(([name, prop]) => ({
    name,
    type: prop.type || "string",
    description: prop.description || "",
    required: requiredSet.has(name),
  }))
}

function ToolCallEditor({ labels }: { labels: StepEditorLabels }) {
  const { properties, setProperty } = useStepEditor()
  const [toolOptions, setToolOptions] = useState<ToolOption[]>([])
  const [toolsLoaded, setToolsLoaded] = useState(false)
  const [useCustomTool, setUseCustomTool] = useState(false)

  useEffect(() => {
    if (toolsLoaded) return

    let cancelled = false

    async function loadAllTools() {
      // 1. 获取内置工具
      const builtinTools: ToolOption[] = []
      try {
        const res = await getTools()
        for (const t of res.tools) {
          if (t.status === "enabled") {
            builtinTools.push({
              name: t.name,
              label: formatToolLabel(t.name, t.description),
              category: "builtin",
              params: extractParamDefs(t.parameters),
            })
          }
        }
      } catch {
        // 忽略
      }

      if (cancelled) return

      // 2. 获取 MCP 工具（并行探测所有已启用服务器）
      const mcpTools: ToolOption[] = []
      try {
        const mcpConfig = await getMCPConfig()
        if (mcpConfig.enabled) {
          const enabledServers = mcpConfig.servers.filter((s) => s.enabled)
          const details = await Promise.allSettled(
            enabledServers.map((s) => getMCPServerDetails(s.name))
          )
          for (const result of details) {
            if (result.status === "fulfilled" && result.value.connected) {
              for (const tool of result.value.tools) {
                const runtimeName = mcpRuntimeName(result.value.server_name, tool.name)
                mcpTools.push({
                  name: runtimeName,
                  label: formatToolLabel(`${result.value.server_name} / ${tool.name}`, tool.description),
                  category: "mcp",
                  params: tool.parameters.map((p) => ({
                    name: p.name,
                    type: p.type,
                    description: p.description,
                    required: p.required,
                  })),
                })
              }
            }
          }
        }
      } catch {
        // MCP 不可用时忽略，仍可手动输入
      }

      if (cancelled) return

      // 3. 合并：内置在前，MCP 在后
      setToolOptions([...builtinTools, ...mcpTools])
      setToolsLoaded(true)
    }

    loadAllTools()
    return () => { cancelled = true }
  }, [toolsLoaded])

  const toolValue = (properties.tool as string) || ""
  const argsRaw = (properties.args as string) || ""

  /** 尝试将字符串值解析为原始 JSON 类型（number/boolean/null），否则保持字符串 */
  const tryParseValue = (v: string, forceString?: boolean): unknown => {
    if (forceString) return v
    if (v === "true") return true
    if (v === "false") return false
    if (v === "null") return null
    if (v !== "" && !isNaN(Number(v))) return Number(v)
    return v
  }

  // 解析 args JSON 为 key-value 对（显示用，字符串化）
  const argsEntries = useMemo(() => {
    if (!argsRaw.trim()) return [] as [string, string][]
    try {
      const parsed = JSON.parse(argsRaw)
      if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
        return Object.entries(parsed).map(([k, v]) => [k, typeof v === "string" ? v : JSON.stringify(v)])
      }
    } catch { /* ignore */ }
    // 如果不是有效 JSON 对象，作为单行兜底
    return argsRaw.trim() ? [["", argsRaw] as [string, string]] : []
  }, [argsRaw])

  // 保留原始解析后的 args（带类型信息），用于重建 JSON 时保留非字符串类型
  const originalArgs = useMemo(() => {
    if (!argsRaw.trim()) return {} as Record<string, unknown>
    try {
      const parsed = JSON.parse(argsRaw)
      if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
        return parsed as Record<string, unknown>
      }
    } catch { /* ignore */ }
    return {} as Record<string, unknown>
  }, [argsRaw])

  // 当前工具的参数类型映射（key → 是否 string 类型），用于避免对 string 类型参数做自动类型转换
  const paramStringTypes = useMemo(() => {
    const m = new Map<string, boolean>()
    const selectedTool = toolOptions.find((t) => t.name === toolValue)
    if (selectedTool) {
      for (const p of selectedTool.params) {
        m.set(p.name, p.type === "string")
      }
    }
    return m
  }, [toolOptions, toolValue])

  // 如果当前工具不在列表中，自动切换到自定义输入模式
  useEffect(() => {
    if (toolsLoaded && toolOptions.length > 0 && toolValue) {
      setUseCustomTool(!toolOptions.some((t) => t.name === toolValue))
    }
  }, [toolsLoaded, toolOptions, toolValue])

  const handleToolChange = (value: string) => {
    if (value === "__custom__") {
      setUseCustomTool(true)
      setProperty("tool", "")
    } else {
      setUseCustomTool(false)
      setProperty("tool", value)
      // 自动填充参数 key
      autofillArgs(value)
    }
  }

  /** 根据工具参数定义重建 args，保留重叠 key 的已有值 */
  const autofillArgs = (toolName: string) => {
    const selectedTool = toolOptions.find((t) => t.name === toolName)
    if (!selectedTool || selectedTool.params.length === 0) return

    // 解析当前 args
    let currentArgs: Record<string, unknown> = {}
    const raw = (properties.args as string) || ""
    if (raw.trim()) {
      try {
        const parsed = JSON.parse(raw)
        if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
          currentArgs = parsed as Record<string, unknown>
        }
      } catch { /* ignore */ }
    }

    // 按工具参数定义顺序重建，保留已有值
    const rebuilt: Record<string, unknown> = {}
    for (const param of selectedTool.params) {
      rebuilt[param.name] = param.name in currentArgs ? currentArgs[param.name] : ""
    }
    setProperty("args", JSON.stringify(rebuilt))
  }

  const handleArgChange = (index: number, field: "key" | "value", val: string) => {
    const entries = [...argsEntries]
    if (index >= entries.length) {
      entries.push(field === "key" ? [val, ""] : ["", val])
    } else {
      const [oldKey, oldValue] = entries[index]
      entries[index] = field === "key" ? [val, oldValue] : [oldKey, val]
    }
    // 重建 JSON，保留非字符串值类型；对 string 类型参数不做自动类型转换
    const obj: Record<string, unknown> = {}
    for (const [k, v] of entries) {
      if (!k) continue
      const isStringType = paramStringTypes.get(k) === true
      if (field === "value" && entries[index][0] === k) {
        obj[k] = tryParseValue(v, isStringType)
      } else if (originalArgs[k] !== undefined && v === String(originalArgs[k])) {
        obj[k] = originalArgs[k]
      } else {
        obj[k] = tryParseValue(v, isStringType)
      }
    }
    setProperty("args", Object.keys(obj).length > 0 ? JSON.stringify(obj) : "")
  }

  const handleAddArg = () => {
    const obj: Record<string, unknown> = {}
    for (const [k, v] of argsEntries) {
      if (k) {
        const isStringType = paramStringTypes.get(k) === true
        obj[k] = originalArgs[k] !== undefined && v === String(originalArgs[k]) ? originalArgs[k] : tryParseValue(v, isStringType)
      }
    }
    let key = "arg_1"
    let n = 1
    while (Object.prototype.hasOwnProperty.call(obj, key)) {
      n++
      key = `arg_${n}`
    }
    obj[key] = ""
    setProperty("args", JSON.stringify(obj))
  }

  const handleRemoveArg = (index: number) => {
    const entries = argsEntries.filter((_, i) => i !== index)
    const obj: Record<string, unknown> = {}
    for (const [k, v] of entries) {
      if (k) {
        const isStringType = paramStringTypes.get(k) === true
        obj[k] = originalArgs[k] !== undefined && v === String(originalArgs[k]) ? originalArgs[k] : tryParseValue(v, isStringType)
      }
    }
    setProperty("args", Object.keys(obj).length > 0 ? JSON.stringify(obj) : "")
  }

  const builtinTools = toolOptions.filter((t) => t.category === "builtin")
  const mcpTools = toolOptions.filter((t) => t.category === "mcp")
  const showDropdown = toolsLoaded && toolOptions.length > 0 && !useCustomTool

  return (
    <>
      <div className="sqd-editor-field">
        <label>{labels.tool}</label>
        {showDropdown ? (
          <select
            value={toolValue}
            onChange={(e) => handleToolChange(e.target.value)}
          >
            <option value="">{labels.toolSelect}</option>
            {builtinTools.length > 0 && (
              <optgroup label={labels.builtinTools}>
                {builtinTools.map((t) => (
                  <option key={t.name} value={t.name}>{t.label}</option>
                ))}
              </optgroup>
            )}
            {mcpTools.length > 0 && (
              <optgroup label={labels.mcpTools}>
                {mcpTools.map((t) => (
                  <option key={t.name} value={t.name}>{t.label}</option>
                ))}
              </optgroup>
            )}
            <option value="__custom__">{labels.toolCustom}</option>
          </select>
        ) : (
          <input
            value={toolValue}
            onChange={(e) => setProperty("tool", e.target.value)}
            placeholder={labels.toolSelect}
          />
        )}
        {useCustomTool && toolOptions.length > 0 && (
          <button type="button" className="sqd-text-button" onClick={() => setUseCustomTool(false)}>
            {labels.toolBackToList}
          </button>
        )}
      </div>
      <div className="sqd-editor-field">
        <label>{labels.args}</label>
        <div style={{ display: "flex", flexDirection: "column", gap: "4px" }}>
          {argsEntries.map(([k, v], index) => (
            <div key={index} className="sqd-editor-grid" style={{ gap: "4px", alignItems: "center" }}>
              <input
                value={k}
                onChange={(e) => handleArgChange(index, "key", e.target.value)}
                placeholder={labels.argsKey}
                style={{ flex: 1 }}
              />
              <input
                value={v}
                onChange={(e) => handleArgChange(index, "value", e.target.value)}
                placeholder={labels.argsValue}
                style={{ flex: 2 }}
              />
              <button
                type="button"
                onClick={() => handleRemoveArg(index)}
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
                title={labels.argsRemove}
              >
                ×
              </button>
            </div>
          ))}
          <button
            type="button"
            onClick={handleAddArg}
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
            + {labels.argsAdd}
          </button>
        </div>
      </div>
    </>
  )
}

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

  // 切换步骤时同步 state
  useEffect(() => {
    setCondType(parsed.type)
    setStepId(parsed.stepId)
    setOutputKey(parsed.outputKey)
    setCondValue(parsed.value)
  }, [whenExpr])

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
  toolSelect: string
  toolCustom: string
  toolBackToList: string
  builtinTools: string
  mcpTools: string
  args: string
  argsKey: string
  argsValue: string
  argsAdd: string
  argsRemove: string
  outputKey: string
  delay: string
  timeout: string
  retry_max_attempts: string
  retry_delay: string
  agentPrompt: string
  toolCall: string
  parallelLabel: string
  addBranch: string
  removeBranch: string
  ifLabel: string
  when: string
  whenCustom: string
  whenPreset: string
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
  const stepId = (properties.stepId as string) || ""

  const handleStepIdChange = (value: string) => {
    // 只允许 a-zA-Z0-9_
    const sanitized = value.replace(/[^a-zA-Z0-9_]/g, "")
    setProperty("stepId", sanitized)
    // 如果显示名称和旧 ID 相同，同步更新显示名称
    if (name === stepId) {
      setName(sanitized)
    }
  }

  const handleNameChange = (value: string) => {
    setName(value)
  }

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
        <label>ID *</label>
        <input value={stepId} onChange={(e) => handleStepIdChange(e.target.value)} placeholder="step_id" />
      </div>
      <div className="sqd-editor-field">
        <label>{labels.name}</label>
        <input value={name === stepId ? "" : name} onChange={(e) => handleNameChange(e.target.value)} placeholder={stepId} />
      </div>
          {isIfStep ? (
        <>
          <IfStepEditor labels={labels} />
          <div className="sqd-editor-grid">
            <div className="sqd-editor-field">
              <label>{labels.delay}</label>
              <input
                value={(properties.delay as string) || ""}
                onChange={(e) => setProperty("delay", e.target.value)}
                placeholder="5s"
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
              <label>{labels.delay}</label>
              <input
                value={(properties.delay as string) || ""}
                onChange={(e) => setProperty("delay", e.target.value)}
                placeholder="5s"
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
            <ToolCallEditor labels={labels} />
          )}
          <WhenEditor labels={labels} />
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
              <label>{labels.delay}</label>
              <input
                value={(properties.delay as string) || ""}
                onChange={(e) => setProperty("delay", e.target.value)}
                placeholder="5s"
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
          <RetryEditor labels={labels} />
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
  const { t } = useTranslation()
  const { properties, setProperty } = useRootEditor()

  const vars = (properties.vars as Record<string, string>) || {}
  const varEntries = Object.entries(vars)

  const handleVarChange = (index: number, field: "key" | "value", val: string) => {
    const entries = Object.entries(vars)
    if (index >= entries.length) return
    const [oldKey, oldValue] = entries[index]
    const newKey = field === "key" ? val : oldKey
    const newValue = field === "value" ? val : oldValue
    // 检查 key 重复
    if (field === "key" && newKey && newKey !== oldKey) {
      for (let i = 0; i < entries.length; i++) {
        if (i !== index && entries[i][0] === newKey) {
          toast.warning(t("pages.workflows.duplicate_var_key", "Variable key '{{key}}' already exists, the previous value will be overwritten", { key: newKey }))
          break
        }
      }
    }
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
          onChange={(e) => setProperty("name", e.target.value.replace(/[^a-zA-Z0-9_-]/g, ""))}
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

function createTaskStep(action: string) {
  return {
    componentType: COMPONENT_TASK,
    type: action,
    name: "",
    properties: {
      stepId: "",
      action,
      prompt: "",
      tool: "",
      args: "",
      retry: "",
      when: "",
      delay: "",
      output_key: "",
      timeout: "",
    },
  }
}

function createIfStep() {
  return {
    componentType: COMPONENT_SWITCH,
    type: "if",
    name: "",
    properties: {
      stepId: "",
      action: "if",
      when: "",
      delay: "",
      timeout: "",
    },
    branches: {
      true: [] as Step[],
      false: [] as Step[],
    },
  }
}

function createParallelStep(): BranchedStep {
  return {
    id: "",
    componentType: COMPONENT_SWITCH,
    type: "parallel",
    name: "",
    properties: {
      stepId: "",
      action: "parallel",
      when: "",
      delay: "",
      output_key: "",
      timeout: "",
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

  // 步骤 ID 计数器（按类型独立计数）
  const stepIdCounters = useRef<Record<string, number>>({})

  const getStepPrefix = (action: string, type: string): string => {
    if (action === "agent_prompt") return "prompt"
    if (action === "tool_call") return "tool"
    if (action === "parallel" || type === "parallel") return "parallel"
    if (action === "if" || type === "if") return "if"
    return "step"
  }

  // 为新增的空 ID 步骤自动分配递增 ID，仅在存在空 ID 时才返回新对象
  const ensureStepIds = (def: Definition): Definition => {
    const existingIds = new Set<string>()
    // 第一遍：收集所有已有的 stepId
    const collectIds = (steps: Step[]) => {
      for (const s of steps) {
        const sid = (s.properties.stepId as string) || ""
        if (sid) existingIds.add(sid)
        if ("branches" in s) {
          const b = s as BranchedStep
          for (const branch of Object.values(b.branches || {})) {
            collectIds(branch)
          }
        }
      }
    }
    collectIds(def.sequence)

    // 检查是否有空 ID
    const hasEmptyId = (steps: Step[]): boolean => {
      for (const s of steps) {
        if (!(s.properties.stepId as string)) return true
        if ("branches" in s) {
          const b = s as BranchedStep
          for (const branch of Object.values(b.branches || {})) {
            if (hasEmptyId(branch)) return true
          }
        }
      }
      return false
    }

    if (!hasEmptyId(def.sequence)) return def

    // 第二遍：分配 ID（尽量保持原引用不变，避免触发不必要的重渲染）
    const assignIds = (steps: Step[]): Step[] => {
      let changed = false
      const result = steps.map((s) => {
        let newStep = s
        const sid = (s.properties.stepId as string) || ""
        if (!sid) {
          const action = (s.properties.action as string) || s.type
          const prefix = getStepPrefix(action, s.type)
          // 按类型独立计数
          if (!stepIdCounters.current[prefix]) stepIdCounters.current[prefix] = 1
          while (existingIds.has(`${prefix}_${stepIdCounters.current[prefix]}`)) {
            stepIdCounters.current[prefix]++
          }
          const newId = `${prefix}_${stepIdCounters.current[prefix]}`
          stepIdCounters.current[prefix]++
          existingIds.add(newId)
          changed = true
          newStep = { ...s, name: newId, properties: { ...s.properties, stepId: newId } }
        }
        if ("branches" in newStep) {
          const b = newStep as BranchedStep
          let branchChanged = false
          const newBranches: Record<string, Step[]> = {}
          for (const [key, branch] of Object.entries(b.branches || {})) {
            const newBranch = assignIds(branch)
            if (newBranch !== branch) branchChanged = true
            newBranches[key] = newBranch
          }
          if (branchChanged) {
            changed = true
            newStep = { ...newStep, branches: newBranches } as BranchedStep
          }
        }
        return newStep
      })
      return changed ? result : steps
    }

    const newSequence = assignIds(def.sequence)
    if (newSequence === def.sequence) return def
    return { ...def, sequence: newSequence }
  }

  const handleChange = (def: WrappedDefinition<Definition>) => {
    const fixed = ensureStepIds(def.value)
    if (fixed !== def.value) {
      onChange(wrapDefinition(fixed))
    } else {
      onChange(def)
    }
  }

  const stepEditorLabels: StepEditorLabels = useMemo(() => ({
    name: t("pages.workflows.name", "Name"),
    action: t("pages.workflows.action", "Action"),
    prompt: t("pages.workflows.prompt_placeholder", "Prompt for the agent..."),
    tool: t("pages.workflows.tool_placeholder", "Tool name"),
    toolSelect: t("pages.workflows.tool_select", "Select or type tool name"),
    toolCustom: t("pages.workflows.tool_custom", "Custom..."),
    toolBackToList: t("pages.workflows.tool_back_to_list", "Back to list"),
    builtinTools: t("pages.workflows.builtin_tools", "Built-in Tools"),
    mcpTools: t("pages.workflows.mcp_tools", "MCP Tools"),
    args: t("pages.workflows.tool_args", "Arguments"),
    argsKey: t("pages.workflows.tool_args_key", "Key"),
    argsValue: t("pages.workflows.tool_args_value", "Value"),
    argsAdd: t("pages.workflows.tool_args_add", "Add Argument"),
    argsRemove: t("pages.workflows.tool_args_remove", "Remove"),
    outputKey: t("pages.workflows.output_key", "Output Key"),
    delay: t("pages.workflows.delay", "Delay"),
    timeout: t("pages.workflows.timeout", "Timeout"),
    retry_max_attempts: t("pages.workflows.retry_max_attempts", "Max Attempts"),
    retry_delay: t("pages.workflows.retry_delay", "Retry Delay"),
    agentPrompt: t("pages.workflows.trigger_agent", "Agent Prompt"),
    toolCall: t("pages.workflows.trigger_tool", "Tool Call"),
    parallelLabel: t("pages.workflows.parallel", "Parallel"),
    addBranch: t("pages.workflows.add_branch", "Add Branch"),
    removeBranch: t("pages.workflows.remove_branch", "Remove Branch"),
    ifLabel: t("pages.workflows.trigger_if", "If"),
    when: t("pages.workflows.when", "When"),
    whenCustom: t("pages.workflows.when_custom", "Custom condition..."),
    whenPreset: t("pages.workflows.when_preset", "Preset"),
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
            createTaskStep("agent_prompt"),
            createTaskStep("tool_call"),
          ],
        },
        {
          name: t("pages.workflows.toolbox_logic", "Logic"),
          steps: [
            createParallelStep(),
            createIfStep(),
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
        onDefinitionChange={handleChange}
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

