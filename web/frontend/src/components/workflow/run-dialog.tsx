import { useTranslation } from "react-i18next"

import type { WorkflowListItem } from "@/api/workflow"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

interface RunDialogProps {
  workflow: WorkflowListItem | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
  isRunning: boolean
}

export function RunDialog({
  workflow,
  open,
  onOpenChange,
  onConfirm,
  isRunning,
}: RunDialogProps) {
  const { t } = useTranslation()

  if (!workflow) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {t("pages.workflows.run_title", "Run Workflow")}
          </DialogTitle>
          <DialogDescription>
            {t(
              "pages.workflows.run_description",
              `Are you sure you want to run "${workflow.name}"?`,
              { name: workflow.name },
            )}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel", "Cancel")}
          </Button>
          <Button
            onClick={onConfirm}
            disabled={isRunning}
          >
            {isRunning
              ? t("pages.workflows.running", "Running...")
              : t("pages.workflows.run", "Run")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
