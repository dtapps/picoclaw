import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearch } from "@tanstack/react-router"
import type { Definition } from "sequential-workflow-designer"
import { toast } from "sonner"

import type { CreateWorkflowRequest, Step as WorkflowStep, UpdateWorkflowRequest, Workflow } from "@/api/workflow"
import { getWorkflow } from "@/api/workflow"
import { getTools } from "@/api/tools"
import { getMCPConfig, getMCPServerDetails } from "@/api/mcp"
import { Button } from "@/components/ui/button"
import { PageHeader } from "@/components/page-header"

import { WorkflowVisualEditor } from "./workflow-visual-editor"
import {
  createEmptyDefinition,
  definitionToWorkflow,
  workflowToDefinition,
  wrapDefinition,
  type WrappedDefinition,
} from "./workflow-definition"
import { useWorkflows } from "./use-workflows"

/** 检查工作流名称是否合法（仅允许 a-zA-Z0-9_-，且非空） */
function isValidWorkflowName(name: string): boolean {
	if (!name) return false
	for (const c of name) {
		const ok = (c >= "a" && c <= "z") || (c >= "A" && c <= "Z") || (c >= "0" && c <= "9") || c === "-" || c === "_"
		if (!ok) return false
	}
	return true
}

/** 获取步骤显示名称：name > id > action */
function stepLabel(step: WorkflowStep): string {
	return step.name || step.id || step.action
}

/** 从步骤的 prompt、args、when 中提取所有 {{.xxx.yyy}} 模板引用 */
function extractTemplateRefs(step: WorkflowStep): { refId: string; refKey: string }[] {
	const refs: { refId: string; refKey: string }[] = []
	const re = /\{\{\.([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)\}\}/g
	const check = (text: string) => { let m; while ((m = re.exec(text)) !== null) refs.push({ refId: m[1], refKey: m[2] }) }
	if (step.prompt) check(step.prompt)
	if (step.when) check(step.when)
	if (step.args) {
		for (const v of Object.values(step.args)) {
			if (typeof v === "string") check(v)
		}
	}
	const subs = step.parallel || [...(step.if_true || []), ...(step.if_false || [])]
	for (const sub of subs) refs.push(...extractTemplateRefs(sub))
	return refs
}

const VALID_FUNC_NAMES = new Set(["now", "now_tz", "date", "date_tz", "unix", "env"])

/** 从步骤的 prompt、args、when 中提取所有 {{.fn.xxx}} 模板函数引用 */
function extractFuncRefs(step: WorkflowStep): string[] {
	const refs: string[] = []
	const re = /\{\{\.fn\.([a-zA-Z0-9_]+)[^}]*\}\}/g
	const check = (text: string) => { let m; while ((m = re.exec(text)) !== null) refs.push(m[1]) }
	if (step.prompt) check(step.prompt)
	if (step.when) check(step.when)
	if (step.args) {
		for (const v of Object.values(step.args)) {
			if (typeof v === "string") check(v)
		}
	}
	const subs = step.parallel || [...(step.if_true || []), ...(step.if_false || [])]
	for (const sub of subs) refs.push(...extractFuncRefs(sub))
	return refs
}

/** 递归收集步骤 ID 和对应的 output_key */
function collectStepOutputKeys(step: WorkflowStep, map: Map<string, string>) {
	if (step.action === "agent_prompt" || step.action === "tool_call") {
		map.set(step.id, step.output_key || "result")
	}
	const subs = step.parallel || [...(step.if_true || []), ...(step.if_false || [])]
	for (const sub of subs) collectStepOutputKeys(sub, map)
}

/** 前端校验模板引用，返回错误信息或空字符串 */
function validateTemplateRefs(steps: WorkflowStep[], vars: Record<string, string> | undefined, t: (key: string, options?: Record<string, unknown>) => string): string {
	const outputKeys = new Map<string, string>()
	for (const step of steps) collectStepOutputKeys(step, outputKeys)
	const varsKeys = new Set(Object.keys(vars || {}))

	for (const step of steps) {
		const refs = extractTemplateRefs(step)
		for (const { refId, refKey } of refs) {
			if (refId === "vars") {
				if (!varsKeys.has(refKey))
					return t("pages.workflows.invalid_var_ref", { step: step.name || step.id, key: refKey })
			} else if (refId === "self") {
				if (refKey !== "id" && refKey !== "name")
					return t("pages.workflows.invalid_self_ref", { step: step.name || step.id, key: refKey })
			} else if (refId === "fn") {
				// fn 引用由 extractFuncRefs 单独校验
			} else {
				const actualKey = outputKeys.get(refId)
				if (!actualKey)
					return t("pages.workflows.invalid_step_ref", { step: step.name || step.id, refId })
				if (refKey !== actualKey)
					return t("pages.workflows.invalid_output_key_ref", { step: step.name || step.id, refId, refKey, actualKey })
			}
		}

		const funcRefs = extractFuncRefs(step)
		for (const funcName of funcRefs) {
			if (!VALID_FUNC_NAMES.has(funcName))
				return t("pages.workflows.invalid_func_ref", { step: step.name || step.id, funcName })
		}
	}
	return ""
}

/** 递归校验 tool_call 步骤的必填参数，返回错误信息或空字符串 */
function validateToolCallArgs(steps: WorkflowStep[], toolRequiredParams: Map<string, string[]>, t: (key: string, options?: Record<string, unknown>) => string): string {
	for (const step of steps) {
		if (step.action === "tool_call" && step.tool) {
			const required = toolRequiredParams.get(step.tool)
			if (!required) {
				// 工具不在列表中（可能是 MCP 未连接或自定义），跳过校验
				continue
			}
			const args = step.args || {}
			for (const key of required) {
				if (!(key in args) || args[key] === "" || args[key] === null || args[key] === undefined) {
					return t("pages.workflows.missing_required_param", { step: step.name || step.id, tool: step.tool, param: key })
				}
			}
		}
		// 递归校验子步骤
		const subs = step.parallel || [...(step.if_true || []), ...(step.if_false || [])]
		const err = validateToolCallArgs(subs, toolRequiredParams, t)
		if (err) return err
	}
	return ""
}

/** 前端校验工作流步骤，返回错误信息或空字符串 */
function validateStep(step: WorkflowStep, index: number, t: (key: string, options?: Record<string, unknown>) => string, ids: Set<string>): string {
	const label = stepLabel(step)
	if (!step.id) return t("pages.workflows.step_no_id", { index, label })
	if (!/^[a-zA-Z0-9_]+$/.test(step.id))
		return t("pages.workflows.step_invalid_id", { index, label })
	if (ids.has(step.id))
		return t("pages.workflows.duplicate_step_id", { id: step.id })
	ids.add(step.id)
	if (step.action === "agent_prompt" && !step.prompt)
		return t("pages.workflows.step_no_prompt", { index, label })
	if (step.action === "tool_call" && !step.tool)
		return t("pages.workflows.step_no_tool", { index, label })
	if (step.action === "parallel" && (!step.parallel || step.parallel.length === 0))
		return t("pages.workflows.step_no_parallel", { index, label })
	if (step.action === "if" && !step.when)
		return t("pages.workflows.step_no_when", { index, label })
	if (step.action === "if" && (!step.if_true || step.if_true.length === 0) && (!step.if_false || step.if_false.length === 0))
		return t("pages.workflows.step_no_branch", { index, label })
	// 递归校验子步骤
	const subSteps = step.parallel || [...(step.if_true || []), ...(step.if_false || [])]
	for (let i = 0; i < subSteps.length; i++) {
		const err = validateStep(subSteps[i], i, t, ids)
		if (err) return err
	}
	return ""
}

export function WorkflowEditorPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = useSearch({ strict: false }) as { name?: string }
  const editName = search?.name || ""

  const { handleCreate, handleUpdate, isCreating } = useWorkflows()

  // 用 useMemo 保持初始 definition 引用稳定，避免 SWD 重复创建
  const startDefinition = useMemo(() => createEmptyDefinition(), [])
  const [definition, setDefinition] = useState<WrappedDefinition<Definition>>(() =>
    wrapDefinition(startDefinition),
  )
  const [editWorkflow, setEditWorkflow] = useState<Workflow | null>(null)
  const [loading, setLoading] = useState(!!editName)
  const [submitting, setSubmitting] = useState(false)
  const [toolRequiredParams, setToolRequiredParams] = useState<Map<string, string[]>>(new Map())

  const isEdit = !!editWorkflow

  // 加载工具参数 Schema（内置 + MCP，用于保存时校验 tool_call 必填参数）
  useEffect(() => {
    let cancelled = false

    async function loadToolSchemas() {
      const m = new Map<string, string[]>()

      // 1. 内置工具
      try {
        const res = await getTools()
        if (cancelled) return
        for (const t of res.tools) {
          if (t.status === "enabled" && t.parameters?.required) {
            m.set(t.name, t.parameters.required)
          }
        }
      } catch { /* 忽略 */ }

      // 2. MCP 工具
      try {
        const mcpConfig = await getMCPConfig()
        if (mcpConfig.enabled) {
          const enabledServers = mcpConfig.servers.filter((s) => s.enabled)
          const details = await Promise.allSettled(
            enabledServers.map((s) => getMCPServerDetails(s.name))
          )
          if (cancelled) return
          for (const result of details) {
            if (result.status === "fulfilled" && result.value.connected) {
              for (const tool of result.value.tools) {
                const runtimeName = `mcp_${result.value.server_name.replace(/[^a-zA-Z0-9_]/g, "_")}_${tool.name.replace(/[^a-zA-Z0-9_]/g, "_")}`
                const required = tool.parameters.filter((p) => p.required).map((p) => p.name)
                if (required.length > 0) {
                  m.set(runtimeName, required)
                }
              }
            }
          }
        }
      } catch { /* MCP 不可用时忽略 */ }

      if (!cancelled) setToolRequiredParams(m)
    }

    loadToolSchemas()
    return () => { cancelled = true }
  }, [])

  // 编辑模式：加载工作流数据
  useEffect(() => {
    if (editName) {
      setLoading(true)
      getWorkflow(editName)
        .then((wf) => {
          setEditWorkflow(wf)
          setDefinition(wrapDefinition(workflowToDefinition(wf)))
        })
        .catch(() => {
          // 加载失败则返回列表
          void navigate({ to: "/workflows" })
        })
        .finally(() => setLoading(false))
    }
  }, [editName, navigate])

  const handleSubmit = async () => {
    const wfData = definitionToWorkflow(definition.value)

    // 前端校验：名称
    if (!isEdit && !isValidWorkflowName(wfData.name)) {
      toast.error(t("pages.workflows.name_invalid"))
      return
    }

    // 前端校验：步骤
    if (wfData.steps.length === 0) {
      toast.error(t("pages.workflows.no_steps"))
      return
    }
    const ids = new Set<string>()
    for (let i = 0; i < wfData.steps.length; i++) {
      const err = validateStep(wfData.steps[i], i, t, ids)
      if (err) {
        toast.error(err)
        return
      }
    }

    // 前端校验：模板引用
    const refErr = validateTemplateRefs(wfData.steps, wfData.vars, t)
    if (refErr) {
      toast.error(refErr)
      return
    }

    // 前端校验：tool_call 必填参数
    const toolErr = validateToolCallArgs(wfData.steps, toolRequiredParams, t)
    if (toolErr) {
      toast.error(toolErr)
      return
    }

    setSubmitting(true)
    try {
      if (isEdit) {
        await handleUpdate(editWorkflow!.name, {
          description: wfData.description,
          triggers: wfData.triggers,
          vars: wfData.vars,
          steps: wfData.steps,
          config: {
            ...wfData.config,
            // 保留编辑器未管理的通知配置字段
            notify_channel: editWorkflow!.config?.notify_channel,
            notify_chat_id: editWorkflow!.config?.notify_chat_id,
          },
        } as UpdateWorkflowRequest)
      } else {
        await handleCreate(wfData as CreateWorkflowRequest)
      }
      void navigate({ to: "/workflows" })
    } catch {
      // mutation 的 onError 已处理 toast 提示
    } finally {
      setSubmitting(false)
    }
  }

  const handleBack = () => {
    void navigate({ to: "/workflows" })
  }

  return (
    <div className="bg-background flex h-full flex-col">
      <PageHeader
        title={
          isEdit
            ? t("pages.workflows.edit_workflow", "Edit Workflow")
            : t("pages.workflows.create_workflow", "Create Workflow")
        }
      >
        <Button variant="outline" onClick={handleBack}>
          {t("common.cancel", "Cancel")}
        </Button>
        <Button onClick={handleSubmit} disabled={isCreating || loading || submitting}>
          {submitting
            ? t("common.saving", "Saving...")
            : isEdit
              ? t("pages.workflows.save", "Save")
              : t("pages.workflows.create", "Create Workflow")}
        </Button>
      </PageHeader>

      <div className="flex-1 overflow-hidden p-2" style={{ minHeight: 500 }}>
        {loading ? (
          <div className="flex h-full items-center justify-center">
            <div className="bg-card h-48 w-full animate-pulse rounded-lg border" />
          </div>
        ) : (
          <WorkflowVisualEditor value={definition} onChange={setDefinition} isEdit={isEdit} />
        )}
      </div>
    </div>
  )
}
