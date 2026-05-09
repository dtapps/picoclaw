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

interface DeleteDialogProps {
  workflow: WorkflowListItem | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
  isDeleting: boolean
}

export function DeleteDialog({
  workflow,
  open,
  onOpenChange,
  onConfirm,
  isDeleting,
}: DeleteDialogProps) {
  const { t } = useTranslation()

  if (!workflow) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {t("pages.workflows.delete_title", "Delete Workflow")}
          </DialogTitle>
          <DialogDescription>
            {t(
              "pages.workflows.delete_description",
              `Are you sure you want to delete "${workflow.name}"? This action cannot be undone.`,
            )}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel", "Cancel")}
          </Button>
          <Button
            variant="destructive"
            onClick={onConfirm}
            disabled={isDeleting}
          >
            {isDeleting
              ? t("common.deleting", "Deleting...")
              : t("common.delete", "Delete")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
