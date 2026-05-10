import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearch } from "@tanstack/react-router"
import type { Definition } from "sequential-workflow-designer"
import { toast } from "sonner"

import type { CreateWorkflowRequest, Step as WorkflowStep, UpdateWorkflowRequest, Workflow } from "@/api/workflow"
import { getWorkflow } from "@/api/workflow"
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

  const isEdit = !!editWorkflow

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

    setSubmitting(true)
    try {
      if (isEdit) {
        await handleUpdate(editWorkflow!.name, {
          description: wfData.description,
          triggers: wfData.triggers,
          vars: wfData.vars,
          steps: wfData.steps,
          config: wfData.config,
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
          <WorkflowVisualEditor value={definition} onChange={setDefinition} />
        )}
      </div>
    </div>
  )
}
