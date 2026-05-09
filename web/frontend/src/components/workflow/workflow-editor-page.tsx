import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearch } from "@tanstack/react-router"
import type { Definition } from "sequential-workflow-designer"
import { toast } from "sonner"

import type { CreateWorkflowRequest, UpdateWorkflowRequest, Workflow } from "@/api/workflow"
import { createWorkflow, getWorkflow, updateWorkflow } from "@/api/workflow"
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

export function WorkflowEditorPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = useSearch({ strict: false }) as { name?: string }
  const editName = search?.name || ""

  const { isCreating } = useWorkflows()

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
    setSubmitting(true)
    try {
      if (isEdit) {
        await updateWorkflow(editWorkflow!.name, {
          description: wfData.description,
          triggers: wfData.triggers,
          vars: wfData.vars,
          steps: wfData.steps,
        } as UpdateWorkflowRequest)
        toast.success(t("pages.workflows.update_success", "Workflow updated successfully"))
      } else {
        await createWorkflow(wfData as CreateWorkflowRequest)
        toast.success(t("pages.workflows.create_success", "Workflow created successfully"))
      }
      void navigate({ to: "/workflows" })
    } catch (err) {
      const message = err instanceof Error ? err.message : ""
      toast.error(message || t("pages.workflows.create_error", "Failed to save workflow"))
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
