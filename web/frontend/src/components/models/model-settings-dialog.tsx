import {
  IconChevronDown,
  IconChevronUp,
  IconLoader2,
  IconX,
} from "@tabler/icons-react"
import { useTranslation } from "react-i18next"

import { type ModelInfo } from "@/api/models"
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

/**
 * 模型设置弹窗组件属性
 */
interface ModelSettingsDialogProps {
  /** 弹窗是否打开 */
  open: boolean
  /** 弹窗开关状态变化回调 */
  onOpenChange: (open: boolean) => void
  /** 可用模型列表 */
  models: ModelInfo[]
  /** 当前活动模型 */
  activeModel: string
  /** 备选模型列表（按优先级排序） */
  fallbacks: string[]
  /** 是否正在保存 */
  saving: boolean
  /** 活动模型变更回调 */
  onActiveModelChange: (model: string) => void
  /** 添加备选模型回调 */
  onAddFallback: (model: string) => void
  /** 移除备选模型回调 */
  onRemoveFallback: (model: string) => void
  /** 移动备选模型位置回调 */
  onMoveFallback: (index: number, direction: "up" | "down") => void
  /** 保存设置回调 */
  onSave: () => void
}

/**
 * 模型设置弹窗组件
 * 用于配置活动模型和备选模型链
 */
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
  onMoveFallback,
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
          {/* 活动模型选择 */}
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

          {/* 备选模型配置 */}
          <div className="space-y-2">
            <Label className="text-foreground/90 text-sm font-medium">
              {t("models.globalSettings.fallbackModels", "Fallback Models")}
            </Label>
            <p className="text-muted-foreground text-xs">
              {t(
                "models.globalSettings.fallbackDescription",
                "Models to fall back to when the active model fails.",
              )}
            </p>

            {/* 备选模型列表（可排序） */}
            {fallbacks.length > 0 && (
              <div className="space-y-1">
                {fallbacks.map((fb, index) => (
                  <div
                    key={fb}
                    className="flex items-center justify-between rounded-md border px-3 py-2"
                  >
                    {/* 模型名称 */}
                    <span className="text-sm font-medium">{fb}</span>
                    {/* 操作按钮组 */}
                    <div className="flex items-center gap-1">
                      {/* 上移按钮 */}
                      <button
                        onClick={() => onMoveFallback(index, "up")}
                        disabled={index === 0}
                        className="hover:bg-muted disabled:opacity-30 rounded p-1 transition-colors"
                        title={t("models.globalSettings.moveUp", "Move up")}
                      >
                        <IconChevronUp className="size-4" />
                      </button>
                      {/* 下移按钮 */}
                      <button
                        onClick={() => onMoveFallback(index, "down")}
                        disabled={index === fallbacks.length - 1}
                        className="hover:bg-muted disabled:opacity-30 rounded p-1 transition-colors"
                        title={t("models.globalSettings.moveDown", "Move down")}
                      >
                        <IconChevronDown className="size-4" />
                      </button>
                      {/* 删除按钮 */}
                      <button
                        onClick={() => onRemoveFallback(fb)}
                        className="hover:bg-muted hover:text-destructive rounded p-1 transition-colors"
                        title={t("common.remove", "Remove")}
                      >
                        <IconX className="size-4" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* 添加备选模型下拉框 */}
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

        {/* 保存按钮 */}
        <div className="flex justify-end pt-2">
          <Button
            onClick={onSave}
            disabled={saving || !activeModel}
            size="sm"
            className="h-9 rounded-lg px-4"
          >
            {saving && <IconLoader2 className="mr-2 size-4 animate-spin" />}
            {t("common.save", "Save")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
