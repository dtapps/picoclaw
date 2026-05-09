import { createFileRoute } from "@tanstack/react-router"

import { WorkflowEditorPage } from "@/components/workflow/workflow-editor-page"

export const Route = createFileRoute("/workflows/editor")({
  component: WorkflowEditorPage,
  validateSearch: (search: Record<string, string>) => {
    return {
      name: search.name || "",
    }
  },
})
