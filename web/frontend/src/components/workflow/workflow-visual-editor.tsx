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
import { ExpandableTextarea } from "./expandable-textarea"

import {
  COMPONENT_TASK,
  COMPONENT_SWITCH,
} from "./workflow-definition"

import { getTools, type ToolParamProperty } from "@/api/tools"
import { getMCPConfig, getMCPServerDetails } from "@/api/mcp"
import { EventTypeGroups, getRegisteredTools } from "@/api/workflow"
import { Switch } from "@/components/ui/switch"

// --- 步骤图标 ---

const ICON_AGENT_PROMPT = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><path d="M12 7v4"/><path d="M8 16h0"/><path d="M16 16h0"/></svg>')}`

const ICON_TOOL_CALL = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>')}`

const ICON_IF = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 3v12"/><path d="M18 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"/><path d="M6 21a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"/><path d="M15 6H9"/><path d="M9 18H3"/></svg>')}`

const ICON_PARALLEL = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2v4"/><path d="M12 18v4"/><path d="M4 8h16"/><path d="M4 16h16"/><path d="M8 8v8"/><path d="M16 8v8"/></svg>')}`

const ICON_NOTIFY = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/></svg>')}`

// --- When 条件编辑器（用于 task 步骤）---

function WhenEditor({ labels }: { labels: StepEditorLabels }) {
  const { properties, setProperty } = useStepEditor()
  const whenValue = (properties.when as string) || ""
  const isPreset = whenValue === "" || whenValue === "on_success" || whenValue === "on_error"
  const [useCustomWhen, setUseCustomWhen] = useState(!isPreset && whenValue !== "")

  // 切换步骤时同步 state
  useEffect(() => {
    const ip = whenValue === "" || whenValue === "on_success" || whenValue === "on_error"
    setUseCustomWhen(!ip && whenValue !== "")
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
          placeholder='{{.step.key}} == value / {{.self.name}}'
        />
      ) : (
        <select value={whenValue} onChange={(e) => handleSelectChange(e.target.value)}>
          <option value="">{labels.whenEmpty}</option>
          <option value="on_success">{labels.whenOnSuccess}</option>
          <option value="on_error">{labels.whenOnError}</option>
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

      // 2. 获取 MCP 工具：从后端已注册列表获取准确的运行时名称，从 MCP server details 获取参数定义
      const mcpTools: ToolOption[] = []
      try {
        const [mcpConfig, registeredNames] = await Promise.all([
          getMCPConfig(),
          getRegisteredTools().catch(() => [] as string[]),
        ])
        // 构建 "mcp_{sanitizedServer}_{sanitizedTool}" -> runtimeName 映射
        const registeredMap = new Map<string, string>()
        for (const name of registeredNames) {
          registeredMap.set(name, name)
        }

        if (mcpConfig.enabled) {
          const enabledServers = mcpConfig.servers.filter((s) => s.enabled)
          const details = await Promise.allSettled(
            enabledServers.map((s) => getMCPServerDetails(s.name))
          )
          for (const result of details) {
            if (result.status === "fulfilled" && result.value.connected) {
              for (const tool of result.value.tools) {
                // 在已注册列表中查找匹配的工具名（后端 sanitize 的真实名称）
                const server = result.value.server_name.toLowerCase()
                const toolName = tool.name.toLowerCase()
                let runtimeName = ""
                // 精确匹配或模糊匹配（处理 -/_ 差异）
                for (const registered of registeredNames) {
                  const lower = registered.toLowerCase()
                  if (lower.includes(server) && lower.includes(toolName)) {
                    runtimeName = registered
                    break
                  }
                }
                if (!runtimeName) continue

                mcpTools.push({
                  name: runtimeName,
                  label: formatToolLabel(`${result.value.server_name} / ${tool.name}`, tool.description),
                  category: "mcp",
                  params: tool.parameters.map((p) => ({
                    name: p.name,
                    type: p.type,
                    description: p.description || "",
                    required: p.required || false,
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

  // 当前工具的必填参数集合，用于 UI 标记
  const requiredParamKeys = useMemo(() => {
    const s = new Set<string>()
    const selectedTool = toolOptions.find((t) => t.name === toolValue)
    if (selectedTool) {
      for (const p of selectedTool.params) {
        if (p.required) s.add(p.name)
      }
    }
    return s
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
          {argsEntries.map(([k, v], index) => {
            const isRequired = requiredParamKeys.has(k)
            return (
              <div key={index} className="sqd-editor-grid" style={{ gap: "4px", alignItems: "center" }}>
                <input
                  value={k}
                  onChange={(e) => handleArgChange(index, "key", e.target.value)}
                  placeholder={labels.argsKey}
                  style={{
                    flex: 1,
                    ...(isRequired ? { borderColor: "var(--destructive)" } : {}),
                  }}
                />
                <input
                  value={v}
                  onChange={(e) => handleArgChange(index, "value", e.target.value)}
                  placeholder={isRequired ? `* ${labels.argsValue}` : labels.argsValue}
                  style={{
                    flex: 2,
                    ...(isRequired ? { borderColor: "var(--destructive)" } : {}),
                  }}
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
            )
          })}
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
          {requiredParamKeys.size > 0 && (
            <p style={{ fontSize: "0.7rem", color: "var(--muted-foreground)", margin: "2px 0 0" }}>
              {labels.argsRequiredHint}
            </p>
          )}
        </div>
      </div>
    </>
  )
}

// --- If 步骤编辑器（结构化条件表单）---

// 支持的操作符列表
type Operator = "==" | "!=" | ">" | "<" | ">=" | "<=" | "contains"

/** 解析 when 表达式，提取结构化字段 */
function parseWhenExpression(when: string): {
  type: "prev_success" | "prev_error" | "output_compare"
  stepId: string
  outputKey: string
  operator: Operator
  value: string
} {
  if (when === "on_success") return { type: "prev_success", stepId: "", outputKey: "", operator: "==", value: "" }
  if (when === "on_error") return { type: "prev_error", stepId: "", outputKey: "", operator: "==", value: "" }

  // 解析 {{.step_id.key}} operator value，支持多种操作符
  const operators: Operator[] = ["contains", "!=", ">=", "<=", "==", ">", "<"]
  for (const op of operators) {
    const regex = new RegExp(`^\\{\\{\\.(.+?)\\.(.+?)\\}\\}\\s*${op.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*(.+)$`)
    const match = when.match(regex)
    if (match) {
      return {
        type: "output_compare",
        stepId: match[1],
        outputKey: match[2],
        operator: op,
        value: match[3].trim()
      }
    }
  }

  return { type: "prev_success", stepId: "", outputKey: "", operator: "==", value: "" }
}

/** 将结构化字段拼回 when 表达式 */
function buildWhenExpression(
  type: "prev_success" | "prev_error" | "output_compare",
  stepId: string,
  outputKey: string,
  operator: Operator,
  value: string,
): string {
  if (type === "prev_success") return "on_success"
  if (type === "prev_error") return "on_error"
  if (stepId && outputKey && value) {
    return `{{.${stepId}.${outputKey}}} ${operator} ${value}`
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
  const [operator, setOperator] = useState<Operator>(parsed.operator)
  const [condValue, setCondValue] = useState(parsed.value)

  // 切换步骤时同步 state
  useEffect(() => {
    const p = parseWhenExpression(whenExpr)
    setCondType(p.type)
    setStepId(p.stepId)
    setOutputKey(p.outputKey)
    setOperator(p.operator)
    setCondValue(p.value)
  }, [whenExpr])

  const handleCondTypeChange = (newType: string) => {
    const t = newType as "prev_success" | "prev_error" | "output_compare"
    setCondType(t)
    setProperty("when", buildWhenExpression(t, stepId, outputKey, operator, condValue))
  }

  const handleFieldChange = (field: "stepId" | "outputKey" | "operator" | "value", val: string) => {
    let newStepId = stepId
    let newOutputKey = outputKey
    let newOperator = operator
    let newCondValue = condValue

    if (field === "stepId") {
      newStepId = val
      setStepId(val)
    }
    if (field === "outputKey") {
      newOutputKey = val
      setOutputKey(val)
    }
    if (field === "operator") {
      newOperator = val as Operator
      setOperator(val as Operator)
    }
    if (field === "value") {
      newCondValue = val
      setCondValue(val)
    }

    setProperty("when", buildWhenExpression(condType, newStepId, newOutputKey, newOperator, newCondValue))
  }

  return (
    <>
      <div className="sqd-editor-field">
        <label>{labels.when} *</label>
        <select value={condType} onChange={(e) => handleCondTypeChange(e.target.value)}>
          <option value="prev_success">{labels.condPrevSuccess}</option>
          <option value="prev_error">{labels.condPrevError}</option>
          <option value="output_compare">{labels.condOutputCompare}</option>
        </select>
      </div>
      {condType === "output_compare" && (
        <>
          <div className="sqd-editor-grid">
            <div className="sqd-editor-field">
              <label>{labels.condStepId}</label>
              <input value={stepId} onChange={(e) => handleFieldChange("stepId", e.target.value)} placeholder="step_id / self" />
            </div>
            <div className="sqd-editor-field">
              <label>{labels.condOutputKey}</label>
              <input value={outputKey} onChange={(e) => handleFieldChange("outputKey", e.target.value)} placeholder="status" />
            </div>
          </div>
          <div className="sqd-editor-grid">
            <div className="sqd-editor-field">
              <label>{labels.condOperator}</label>
              <select value={operator} onChange={(e) => handleFieldChange("operator", e.target.value)}>
                <option value="==">==</option>
                <option value="!=">!=</option>
                <option value="contains">contains</option>
                <option value=">">&gt;</option>
                <option value="<">&lt;</option>
                <option value=">=">&gt;=</option>
                <option value="<=">&lt;=</option>
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
  message: string
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
  argsRequiredHint: string
  outputKey: string
  delay: string
  timeout: string
  retry_max_attempts: string
  retry_delay: string
  agentPrompt: string
  toolCall: string
  notify: string
  parallelLabel: string
  addBranch: string
  removeBranch: string
  ifLabel: string
  when: string
  whenEmpty: string
  whenOnSuccess: string
  whenOnError: string
  whenCustom: string
  whenPreset: string
  condPrevSuccess: string
  condPrevError: string
  condOutputEquals: string
  condOutputCompare: string
  condStepId: string
  condOutputKey: string
  condOperator: string
  condValue: string
  enabled: string
  enabledYes: string
  enabledNo: string
  notifyOnStart: string
  notifyOnComplete: string
  skills: string
  skillsDesc: string
  tools: string
  toolsDesc: string
  modeDefault: string
  modeOff: string
}

function StepEditorPanel({ labels, definition }: { labels: StepEditorLabels; definition: Definition }) {
  const { id, type, componentType, name, setName, properties, setProperty, notifyChildrenChanged } = useStepEditor()
  const action = (properties.action as string) || type
  const isIfStep = componentType === COMPONENT_SWITCH && type !== "parallel"
  const isParallelStep = type === "parallel"
  const stepId = (properties.stepId as string) || ""

  const handleStepIdChange = (value: string) => {
    const sanitized = value.replace(/[^a-zA-Z0-9_]/g, "")
    setProperty("stepId", sanitized)
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
      <div className="sqd-editor-field sqd-step-toggle">
        <label className="sqd-step-toggle__label">{labels.enabled}</label>
        <div className="sqd-step-toggle__control">
          <span>{(properties.enabled as boolean) === false ? labels.enabledNo : labels.enabledYes}</span>
          <button
            type="button"
            onClick={() => setProperty("enabled", (properties.enabled as boolean) === false)}
            className={`sqd-step-toggle__switch${(properties.enabled as boolean) === false ? "" : " sqd-step-toggle__switch--on"}`}
          />
        </div>
      </div>
      {/* 通知开关：开始执行 */}
      <div className="sqd-editor-field sqd-step-toggle">
        <label className="sqd-step-toggle__label">{labels.notifyOnStart}</label>
        <div className="sqd-step-toggle__control">
          <span>{(properties.notify_on_start as boolean) === false ? labels.enabledNo : labels.enabledYes}</span>
          <button
            type="button"
            onClick={() => setProperty("notify_on_start", (properties.notify_on_start as boolean) === false)}
            className={`sqd-step-toggle__switch${(properties.notify_on_start as boolean) === false ? "" : " sqd-step-toggle__switch--on"}`}
          />
        </div>
      </div>
      {/* 通知开关：执行完成（notify 不需要） */}
      {action !== "notify" && (
        <div className="sqd-editor-field sqd-step-toggle">
          <label className="sqd-step-toggle__label">{labels.notifyOnComplete}</label>
          <div className="sqd-step-toggle__control">
            <span>{(properties.notify_on_complete as boolean) === false ? labels.enabledNo : labels.enabledYes}</span>
            <button
              type="button"
              onClick={() => setProperty("notify_on_complete", (properties.notify_on_complete as boolean) === false)}
              className={`sqd-step-toggle__switch${(properties.notify_on_complete as boolean) === false ? "" : " sqd-step-toggle__switch--on"}`}
            />
          </div>
        </div>
      )}
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
                placeholder="30m"
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
                placeholder="30m"
              />
            </div>
          </div>
        </>
      ) : (
        <>
          <div className="sqd-editor-field">
            <label>{labels.action}</label>
            <input
              value={action === "agent_prompt" ? labels.agentPrompt : action === "notify" ? labels.notify : labels.toolCall}
              readOnly
              className="opacity-70"
            />
          </div>
          {action === "agent_prompt" && (
            <>
              <div className="sqd-editor-field">
                <label>{labels.prompt}</label>
                <ExpandableTextarea
                  value={(properties.prompt as string) || ""}
                  onChange={(value) => setProperty("prompt", value)}
                  rows={3}
                  placeholder="支持 {{.vars.key}}、{{.step_id.key}}、{{.step_id._status}}、{{.self.name}} 模板引用"
                />
              </div>
              <div className="sqd-editor-field">
                <label>{labels.skills}</label>
                <select
                  value={(properties.skills as string) || "default"}
                  onChange={(e) => setProperty("skills", e.target.value)}
                >
                  <option value="default">{labels.modeDefault}</option>
                  <option value="off">{labels.modeOff}</option>
                </select>
                {(properties.skills as string) === "off" && (
                  <p className="sqd-editor-hint">{labels.skillsDesc}</p>
                )}
              </div>
              <div className="sqd-editor-field">
                <label>{labels.tools}</label>
                <select
                  value={(properties.tools as string) || "default"}
                  onChange={(e) => setProperty("tools", e.target.value)}
                >
                  <option value="default">{labels.modeDefault}</option>
                  <option value="off">{labels.modeOff}</option>
                </select>
                {(properties.tools as string) === "off" && (
                  <p className="sqd-editor-hint">{labels.toolsDesc}</p>
                )}
              </div>
            </>
          )}
          {action === "tool_call" && (
            <ToolCallEditor labels={labels} />
          )}
          {action === "notify" && (
            <div className="sqd-editor-field">
              <label>{labels.message}</label>
              <ExpandableTextarea
                value={(properties.message as string) || ""}
                onChange={(value) => setProperty("message", value)}
                rows={3}
                placeholder="支持 {{.vars.key}}、{{.step_id.key}}、{{.step_id._status}}、{{.self.name}} 模板引用"
              />
            </div>
          )}
          <WhenEditor labels={labels} />
          <div className="sqd-editor-grid">
            {action !== "notify" && (
              <div className="sqd-editor-field">
                <label>{labels.outputKey}</label>
                <input
                  value={(properties.output_key as string) || ""}
                  onChange={(e) => setProperty("output_key", e.target.value)}
                  placeholder="result"
                />
              </div>
            )}
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
                placeholder="30m"
              />
              {(properties.timeout as string) === "" && (
                <span style={{ fontSize: "0.7rem", color: "var(--muted-foreground)" }}>Default: 30m</span>
              )}
            </div>
          </div>
          {action !== "notify" && <RetryEditor labels={labels} />}
        </>
      )}
    </div>
  )
}

// --- 触发器编辑器 ---
import type { TriggerConfig } from "./workflow-definition"

function TriggersEditor({ labels }: { labels: RootEditorLabels }) {
  const { t } = useTranslation()
  const { properties, setProperty } = useRootEditor()

  const triggers = (properties.triggers as TriggerConfig[]) || [{ type: "manual" }]

  const handleAddTrigger = () => {
    const updated = [...triggers, { type: "manual" as const }]
    setProperty("triggers", updated)
  }

  const handleRemoveTrigger = (index: number) => {
    if (triggers.length <= 1) {
      toast.warning(t("pages.workflows.at_least_one_trigger", "At least one trigger is required"))
      return
    }
    const updated = triggers.filter((_, i) => i !== index)
    setProperty("triggers", updated)
  }

  const handleTypeChange = (index: number, type: TriggerConfig["type"]) => {
    const updated = [...triggers]
    updated[index] = { type }
    setProperty("triggers", updated)
  }

  const handleCronChange = (index: number, field: "cron" | "tz", value: string) => {
    const updated = [...triggers]
    updated[index] = { ...updated[index], [field]: value }
    setProperty("triggers", updated)
  }

  const handleAtChange = (index: number, field: "at" | "tz", value: string) => {
    const updated = [...triggers]
    updated[index] = { ...updated[index], [field]: value }
    setProperty("triggers", updated)
  }

  const handleIntervalChange = (index: number, field: "interval" | "tz", value: string) => {
    const updated = [...triggers]
    updated[index] = { ...updated[index], [field]: value }
    setProperty("triggers", updated)
  }

  const handleEventChange = (index: number, value: string) => {
    const updated = [...triggers]
    updated[index] = { ...updated[index], event: value }
    setProperty("triggers", updated)
  }

  const handleEventFilterChange = (index: number, key: string, value: string) => {
    const updated = [...triggers]
    const currentFilters = updated[index].event_filters || {}
    if (value === "") {
      delete currentFilters[key]
    } else {
      currentFilters[key] = value
    }
    updated[index] = { ...updated[index], event_filters: { ...currentFilters } }
    setProperty("triggers", updated)
  }

  return (
    <div className="sqd-editor-field">
      <label>{labels.trigger}</label>
      <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
        {triggers.map((trigger, index) => (
          <div key={index} className="border rounded p-2 space-y-2">
            <div className="flex items-center gap-2">
              <select
                value={trigger.type}
                onChange={(e) => handleTypeChange(index, e.target.value as TriggerConfig["type"])}
                className="flex-1"
              >
                <option value="manual">{labels.triggerManual}</option>
                <option value="cron">{labels.triggerCron}</option>
                <option value="at">{labels.triggerAt || "At (Once)"}</option>
                <option value="interval">{labels.triggerInterval || "Interval"}</option>
                <option value="event">{labels.triggerEvent}</option>
              </select>
              <button
                type="button"
                onClick={() => handleRemoveTrigger(index)}
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
                title={t("pages.workflows.remove_trigger", "Remove Trigger")}
              >
                ×
              </button>
            </div>
            {trigger.type === "cron" && (
              <div className="sqd-editor-grid">
                <div className="sqd-editor-field">
                  <label>Cron</label>
                  <input
                    value={trigger.cron || ""}
                    onChange={(e) => handleCronChange(index, "cron", e.target.value)}
                    placeholder="0 9 * * *"
                  />
                </div>
                <div className="sqd-editor-field">
                  <label>Timezone</label>
                  <input
                    value={trigger.tz || ""}
                    onChange={(e) => handleCronChange(index, "tz", e.target.value)}
                    placeholder="Asia/Shanghai"
                  />
                </div>
              </div>
            )}
            {trigger.type === "at" && (
              <div className="sqd-editor-grid">
                <div className="sqd-editor-field">
                  <label>DateTime</label>
                  <input
                    value={trigger.at || ""}
                    onChange={(e) => handleAtChange(index, "at", e.target.value)}
                    placeholder="2025-05-15 09:00:00"
                  />
                </div>
                <div className="sqd-editor-field">
                  <label>Timezone</label>
                  <input
                    value={trigger.tz || ""}
                    onChange={(e) => handleAtChange(index, "tz", e.target.value)}
                    placeholder="Asia/Shanghai"
                  />
                </div>
              </div>
            )}
            {trigger.type === "interval" && (
              <div className="sqd-editor-grid">
                <div className="sqd-editor-field">
                  <label>Interval</label>
                  <input
                    value={trigger.interval || ""}
                    onChange={(e) => handleIntervalChange(index, "interval", e.target.value)}
                    placeholder="30m, 1h, 2h30m"
                  />
                </div>
                <div className="sqd-editor-field">
                  <label>Timezone</label>
                  <input
                    value={trigger.tz || ""}
                    onChange={(e) => handleIntervalChange(index, "tz", e.target.value)}
                    placeholder="Asia/Shanghai"
                  />
                </div>
              </div>
            )}
            {trigger.type === "event" && (
              <div className="space-y-2">
                <div className="sqd-editor-field">
                  <label>{labels.eventType}</label>
                  <select
                    value={trigger.event || ""}
                    onChange={(e) => handleEventChange(index, e.target.value)}
                  >
                    <option value="">{labels.selectEventType}</option>
                    {EventTypeGroups.map((group) => (
                      <optgroup key={group.label} label={group.label}>
                        {group.options.map((opt) => (
                          <option key={opt.value} value={opt.value}>
                            {opt.label}
                          </option>
                        ))}
                      </optgroup>
                    ))}
                  </select>
                </div>
                {/* 事件过滤条件 */}
                <div className="sqd-editor-field">
                  <label>{labels.eventFilters}</label>
                  <div className="space-y-2">
                    {/* 工具事件过滤 */}
                    {(trigger.event === "agent.tool.exec_end" || 
                      trigger.event === "agent.tool.exec_error" ||
                      trigger.event === "agent.tool.exec_start") && (
                      <>
                        <div className="flex gap-2">
                          <input
                            value={trigger.event_filters?.tool || ""}
                            onChange={(e) => handleEventFilterChange(index, "tool", e.target.value)}
                            placeholder={labels.filterToolPlaceholder}
                            className="flex-1"
                          />
                        </div>
                        <p className="text-xs text-muted-foreground">{labels.filterToolDesc}</p>
                      </>
                    )}
                    {/* 通用过滤条件 - 所有事件类型都显示 */}
                    <div className="space-y-2">
                      <p className="text-xs text-muted-foreground">{labels.customFilters}</p>
                      {Object.entries(trigger.event_filters || {}).filter(([k]) => k !== "tool").map(([key, value]) => (
                        <div key={key} className="flex gap-2">
                          <input
                            value={key}
                            onChange={(e) => {
                              const newFilters = { ...trigger.event_filters }
                              delete newFilters[key]
                              if (e.target.value) {
                                newFilters[e.target.value] = value
                              }
                              const updated = [...triggers]
                              updated[index] = { ...updated[index], event_filters: newFilters }
                              setProperty("triggers", updated)
                            }}
                            placeholder={labels.filterKeyPlaceholder}
                            className="flex-1"
                          />
                          <input
                            value={value}
                            onChange={(e) => handleEventFilterChange(index, key, e.target.value)}
                            placeholder={labels.filterValuePlaceholder}
                            className="flex-1"
                          />
                          <button
                            type="button"
                            onClick={() => {
                              const newFilters = { ...trigger.event_filters }
                              delete newFilters[key]
                              const updated = [...triggers]
                              updated[index] = { ...updated[index], event_filters: newFilters }
                              setProperty("triggers", updated)
                            }}
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
                          >
                            ×
                          </button>
                        </div>
                      ))}
                      <button
                        type="button"
                        onClick={() => {
                          const newFilters = { ...trigger.event_filters, "": "" }
                          const updated = [...triggers]
                          updated[index] = { ...updated[index], event_filters: newFilters }
                          setProperty("triggers", updated)
                        }}
                        style={{
                          background: "transparent",
                          border: "1px dashed var(--input)",
                          borderRadius: "var(--radius)",
                          color: "var(--muted-foreground)",
                          cursor: "pointer",
                          fontSize: "0.75rem",
                          padding: "4px 8px",
                          textAlign: "left",
                        }}
                      >
                        + {labels.addFilter}
                      </button>
                    </div>
                  </div>
                </div>
                {/* 事件变量映射 */}
                <div className="sqd-editor-field">
                  <label>{labels.eventMapping}</label>
                  <p className="text-xs text-muted-foreground mb-2">{labels.eventMappingDesc}</p>
                  <div className="space-y-1 text-xs text-muted-foreground">
                    <p>{labels.eventVarsAvailable}</p>
                  </div>
                </div>
              </div>
            )}
          </div>
        ))}
        <button
          type="button"
          onClick={handleAddTrigger}
          style={{
            background: "transparent",
            border: "1px dashed var(--input)",
            borderRadius: "var(--radius)",
            color: "var(--muted-foreground)",
            cursor: "pointer",
            fontSize: "0.75rem",
            padding: "4px 8px",
            textAlign: "left",
          }}
        >
          + {t("pages.workflows.add_trigger", "Add Trigger")}
        </button>
      </div>
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
  triggerAt: string
  triggerInterval: string
  triggerEvent: string
  eventType: string
  selectEventType: string
  eventFilters: string
  filterToolPlaceholder: string
  filterToolDesc: string
  customFilters: string
  filterKeyPlaceholder: string
  filterValuePlaceholder: string
  addFilter: string
  eventMapping: string
  eventMappingDesc: string
  eventVarsAvailable: string
  failureStrategy: string
  strategyStop: string
  strategyContinue: string
  workdir: string
  timeout: string
  timeoutDesc: string
  reuseSession: string
  reuseSessionDesc: string
  history: string
  historyDesc: string
  systemPrompt: string
  systemPromptDesc: string
  modeDefault: string
  modeOff: string
  vars: string
  varsKey: string
  varsValue: string
  varsAdd: string
  varsRemove: string
}

function RootEditorPanel({ labels, isEdit }: { labels: RootEditorLabels; isEdit?: boolean }) {
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
          disabled={isEdit}
        />
      </div>
      <div className="sqd-editor-field">
        <label>{labels.description}</label>
        <ExpandableTextarea
          value={(properties.description as string) || ""}
          onChange={(value) => setProperty("description", value)}
          rows={2}
        />
      </div>
      <TriggersEditor labels={labels} />
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
        <label>{labels.workdir}</label>
        <input
          type="text"
          value={(properties.workdir as string) || ""}
          onChange={(e) => setProperty("workdir", e.target.value)}
          placeholder="/path/to/project"
        />
      </div>
      <div className="sqd-editor-field">
        <label>{labels.timeout}</label>
        <input
          type="text"
          value={(properties.timeout as string) || ""}
          onChange={(e) => setProperty("timeout", e.target.value)}
          placeholder="30m"
        />
        <span className="sqd-editor-field__hint">{labels.timeoutDesc}</span>
      </div>
      <div className="sqd-editor-field sqd-reuse-session-toggle">
        <Switch
          id="reuse-session"
          checked={(properties.reuseSession as boolean) || false}
          onCheckedChange={(checked) => setProperty("reuseSession", checked)}
        />
        <label htmlFor="reuse-session" className="sqd-reuse-session-toggle__label">
          {labels.reuseSession}
        </label>
        <span className="sqd-reuse-session-toggle__desc">
          {labels.reuseSessionDesc}
        </span>
      </div>
      <div className="sqd-editor-field">
        <label>{labels.history}</label>
        <select
          value={(properties.history as string) || "default"}
          onChange={(e) => setProperty("history", e.target.value)}
        >
          <option value="default">{labels.modeDefault}</option>
          <option value="off">{labels.modeOff}</option>
        </select>
        {(properties.history as string) === "off" && (
          <span className="sqd-editor-field__hint">{labels.historyDesc}</span>
        )}
      </div>
      <div className="sqd-editor-field">
        <label>{labels.systemPrompt}</label>
        <select
          value={(properties.systemPrompt as string) || "default"}
          onChange={(e) => setProperty("systemPrompt", e.target.value)}
        >
          <option value="default">{labels.modeDefault}</option>
          <option value="off">{labels.modeOff}</option>
        </select>
        {(properties.systemPrompt as string) === "off" && (
          <span className="sqd-editor-field__hint">{labels.systemPromptDesc}</span>
        )}
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

function createTaskStep(action: string): Step {
  const baseProperties: Record<string, unknown> = {
    stepId: "",
    action,
    when: "",
    delay: "",
    timeout: "",
    enabled: true,
    notify_on_start: true,
  }

  if (action === "agent_prompt") {
    baseProperties.prompt = ""
    baseProperties.output_key = "result"
    baseProperties.retry = ""
    baseProperties.notify_on_complete = true
  } else if (action === "tool_call") {
    baseProperties.tool = ""
    baseProperties.args = ""
    baseProperties.output_key = "result"
    baseProperties.retry = ""
    baseProperties.notify_on_complete = true
  } else if (action === "notify") {
    baseProperties.message = ""
  }

  return {
    componentType: COMPONENT_TASK,
    type: action,
    name: "",
    properties: baseProperties,
  } as Step
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
      enabled: true,
      notify_on_start: true,
      notify_on_complete: true,
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
      enabled: true,
      notify_on_start: true,
      notify_on_complete: true,
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
  isEdit?: boolean
}

export function WorkflowVisualEditor({ value, onChange, isEdit }: WorkflowVisualEditorProps) {
  const { t } = useTranslation()
  const { theme } = useTheme()

  const [isToolboxCollapsed, setIsToolboxCollapsed] = useState(false)
  const [isEditorCollapsed, setIsEditorCollapsed] = useState(false)

  // 步骤 ID 计数器（按类型独立计数）
  const stepIdCounters = useRef<Record<string, number>>({})

  const getStepPrefix = (action: string, type: string): string => {
    if (action === "agent_prompt") return "prompt"
    if (action === "tool_call") return "tool"
    if (action === "notify") return "notify"
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
    message: t("pages.workflows.message_placeholder", "Notification message..."),
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
    argsRequiredHint: t("pages.workflows.tool_args_required_hint", "* Required parameters must have a value before saving. Template references like {{.vars.x}} are allowed."),
    outputKey: t("pages.workflows.output_key", "Output Key"),
    delay: t("pages.workflows.delay", "Delay"),
    timeout: t("pages.workflows.timeout", "Timeout"),
    retry_max_attempts: t("pages.workflows.retry_max_attempts", "Max Attempts"),
    retry_delay: t("pages.workflows.retry_delay", "Retry Delay"),
    agentPrompt: t("pages.workflows.trigger_agent", "Agent Prompt"),
    toolCall: t("pages.workflows.trigger_tool", "Tool Call"),
    notify: t("pages.workflows.trigger_notify", "Notify"),
    parallelLabel: t("pages.workflows.parallel", "Parallel"),
    addBranch: t("pages.workflows.add_branch", "Add Branch"),
    removeBranch: t("pages.workflows.remove_branch", "Remove Branch"),
    ifLabel: t("pages.workflows.trigger_if", "If"),
    when: t("pages.workflows.when", "Pre-condition"),
    whenEmpty: t("pages.workflows.when_empty", "None"),
    whenOnSuccess: t("pages.workflows.when_on_success", "Run when previous step succeeded"),
    whenOnError: t("pages.workflows.when_on_error", "Run when previous step failed"),
    whenCustom: t("pages.workflows.when_custom", "Custom Condition"),
    whenPreset: t("pages.workflows.when_preset", "Preset"),
    condPrevSuccess: t("pages.workflows.cond_prev_success", "Previous step succeeded"),
    condPrevError: t("pages.workflows.cond_prev_error", "Previous step failed"),
    condOutputEquals: t("pages.workflows.cond_output_equals", "Step output equals"),
    condOutputCompare: t("pages.workflows.cond_output_compare", "Step output comparison"),
    condStepId: t("pages.workflows.cond_step_id", "Step ID"),
    condOutputKey: t("pages.workflows.cond_output_key", "Output Key"),
    condOperator: t("pages.workflows.cond_operator", "Operator"),
    condValue: t("pages.workflows.cond_value", "Value"),
    enabled: t("pages.workflows.enabled", "Enabled"),
    enabledYes: t("pages.workflows.enabled_yes", "On"),
    enabledNo: t("pages.workflows.enabled_no", "Off"),
    notifyOnStart: t("pages.workflows.notify_on_start", "Notify on Start"),
    notifyOnComplete: t("pages.workflows.notify_on_complete", "Notify on Complete"),
    skills: t("pages.config.turn_profile_skills", "Skills"),
    skillsDesc: t("pages.config.turn_profile_skills_hint", ""),
    tools: t("pages.config.turn_profile_tools", "Tools"),
    toolsDesc: t("pages.config.turn_profile_tools_hint", ""),
    modeDefault: t("pages.config.turn_profile_mode_default", "Default"),
    modeOff: t("pages.config.turn_profile_mode_off", "Off"),
  }), [t])

  const rootEditorLabels: RootEditorLabels = useMemo(() => ({
    name: t("pages.workflows.name", "Name"),
    description: t("pages.workflows.description", "Description"),
    trigger: t("pages.workflows.trigger", "Trigger"),
    triggerManual: t("pages.workflows.trigger_manual", "Manual"),
    triggerCron: t("pages.workflows.trigger_cron", "Cron"),
    triggerAt: t("pages.workflows.trigger_at", "At (Once)"),
    triggerInterval: t("pages.workflows.trigger_interval", "Interval"),
    triggerEvent: t("pages.workflows.trigger_event", "Event"),
    eventType: t("pages.workflows.event_type", "Event Type"),
    selectEventType: t("pages.workflows.select_event_type", "Select Event Type"),
    eventFilters: t("pages.workflows.event_filters", "Event Filters"),
    filterToolPlaceholder: t("pages.workflows.filter_tool_placeholder", "Tool name (e.g., git_commit)"),
    filterToolDesc: t("pages.workflows.filter_tool_desc", "Only respond to completion events for this tool"),
    customFilters: t("pages.workflows.custom_filters", "Custom Filters"),
    filterKeyPlaceholder: t("pages.workflows.filter_key_placeholder", "Field name (e.g., result)"),
    filterValuePlaceholder: t("pages.workflows.filter_value_placeholder", "Field value"),
    addFilter: t("pages.workflows.add_filter", "Add Filter"),
    eventMapping: t("pages.workflows.event_mapping", "Event Variable Mapping"),
    eventMappingDesc: t("pages.workflows.event_mapping_desc", "Event data will be automatically mapped to workflow variables"),
    eventVarsAvailable: t("pages.workflows.event_vars_available", "Available: event_kind, event_time, event_tool, event_result, event_error"),
    failureStrategy: t("pages.workflows.failure_strategy", "Failure Strategy"),
    strategyStop: t("pages.workflows.strategy_stop", "Stop on failure"),
    strategyContinue: t("pages.workflows.strategy_continue", "Continue on failure"),
    workdir: t("pages.workflows.workdir", "Working Directory"),
    timeout: t("pages.workflows.timeout", "Timeout"),
    timeoutDesc: t("pages.workflows.timeout_desc", "Global workflow timeout, e.g., 30m, 1h (default: 30m)"),
    reuseSession: t("pages.workflows.reuse_session", "Reuse Session"),
    reuseSessionDesc: t("pages.workflows.reuse_session_desc", "Reuse the same session for each execution"),
    history: t("pages.config.turn_profile_history", "History"),
    historyDesc: t("pages.config.turn_profile_history_hint", ""),
    systemPrompt: t("pages.config.turn_profile_system_prompt", "System Prompt"),
    systemPromptDesc: t("pages.config.turn_profile_system_prompt_hint", ""),
    modeDefault: t("pages.config.turn_profile_mode_default", "Default"),
    modeOff: t("pages.config.turn_profile_mode_off", "Off"),
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
        if (step.type === "notify") return t("pages.workflows.trigger_notify", "Notify")
        if (step.type === "parallel") return t("pages.workflows.parallel", "Parallel")
        if (step.type === "if") return t("pages.workflows.trigger_if", "If")
        return step.type
      },
      descriptionProvider: (step) => {
        if (step.type === "agent_prompt") return t("pages.workflows.agent_prompt_desc", "Send a prompt to the LLM agent and get a response")
        if (step.type === "tool_call") return t("pages.workflows.tool_call_desc", "Call a registered tool by name with parameters")
        if (step.type === "notify") return t("pages.workflows.notify_desc", "Send a notification message to the bound channel")
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
            createTaskStep("notify"),
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
        if (type === "notify") return ICON_NOTIFY
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
        rootEditor={<RootEditorPanel labels={rootEditorLabels} isEdit={isEdit} />}
        stepEditor={<StepEditorPanel labels={stepEditorLabels} definition={value.value} />}
      />
    </div>
  )
}

