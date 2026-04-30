import { IconLoader2, IconX } from "@tabler/icons-react"
import { useTranslation } from "react-i18next"

import {
  type ModelInfo,
} from "@/api/models"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

interface ModelSettingsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  models: ModelInfo[]
  activeModel: string
  fallbacks: string[]
  saving: boolean
  onActiveModelChange: (model: string) => void
  onAddFallback: (model: string) => void
  onRemoveFallback: (model: string) => void
  onSave: () => void
}

export function ModelSettingsDialog({
  open,
  onOpenChange,
  models,
  activeModel,
  fallbacks,
  saving,
  onActiveModelChange,
  onAddFallback,
  onRemoveFallback,
  onSave,
}: ModelSettingsDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {t("models.globalSettings.title", "Active Model Settings")}
          </DialogTitle>
          <DialogDescription>
            {t(
              "models.globalSettings.activeModelDescription",
              "The model currently used by agents for inference.",
            )}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* 活动模型 */}
          <div className="space-y-2">
            <Label className="text-foreground/90 text-sm font-medium">
              {t("models.globalSettings.activeModel", "Active Model")}
            </Label>
            <Select value={activeModel} onValueChange={onActiveModelChange}>
              <SelectTrigger className="h-9 w-full rounded-lg">
                <SelectValue
                  placeholder={t(
                    "models.globalSettings.selectModel",
                    "Select a model",
                  )}
                />
              </SelectTrigger>
              <SelectContent>
                {models
                  .filter((m) => !m.is_virtual)
                  .map((m) => (
                    <SelectItem key={m.model_name} value={m.model_name}>
                      {m.model_name}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
          </div>

          {/* 备选模型 */}
          <div className="space-y-2">
            <Label className="text-foreground/90 text-sm font-medium">
              {t(
                "models.globalSettings.fallbackModels",
                "Fallback Models",
              )}
            </Label>
            <p className="text-muted-foreground text-xs">
              {t(
                "models.globalSettings.fallbackDescription",
                "Models to fall back to when the active model fails.",
              )}
            </p>

            {fallbacks.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {fallbacks.map((fb) => (
                  <span
                    key={fb}
                    className="bg-primary/10 text-primary inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium"
                  >
                    {fb}
                    <button
                      onClick={() => onRemoveFallback(fb)}
                      className="hover:text-primary/70 rounded-sm"
                    >
                      <IconX className="size-3" />
                    </button>
                  </span>
                ))}
              </div>
            )}

            <Select value="" onValueChange={onAddFallback}>
              <SelectTrigger className="h-9 w-full rounded-lg">
                <SelectValue
                  placeholder={t(
                    "models.globalSettings.addFallback",
                    "Add fallback model...",
                  )}
                />
              </SelectTrigger>
              <SelectContent>
                {models
                  .filter(
                    (m) =>
                      !m.is_virtual &&
                      m.model_name !== activeModel &&
                      !fallbacks.includes(m.model_name),
                  )
                  .map((m) => (
                    <SelectItem key={m.model_name} value={m.model_name}>
                      {m.model_name}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="flex justify-end pt-2">
          <Button
            onClick={onSave}
            disabled={saving || !activeModel}
            size="sm"
            className="h-9 rounded-lg px-4"
          >
            {saving && (
              <IconLoader2 className="mr-2 size-4 animate-spin" />
            )}
            {t("common.save", "Save")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
