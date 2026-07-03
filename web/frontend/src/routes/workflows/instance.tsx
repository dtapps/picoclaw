import { createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"
import { WorkflowDetail } from "@/components/workflow/workflow-detail"

export const Route = createFileRoute("/workflows/instance")({
  component: WorkflowInstancePage,
  validateSearch: (search: Record<string, string>) => {
    return {
      workflow: search.workflow || "",
      instance: search.instance || "",
    }
  },
})

function WorkflowInstancePage() {
  const { t } = useTranslation()
  const { workflow, instance } = Route.useSearch()

  if (!workflow || !instance) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        {t("pages.workflows.no_instance_selected", "No instance selected")}
      </div>
    )
  }

  return <WorkflowDetail workflowName={workflow} instanceId={instance} />
}
