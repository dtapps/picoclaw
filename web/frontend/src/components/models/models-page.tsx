import {
  IconDatabase,
  IconLoader2,
  IconPlus,
  IconSettings,
  IconStar,
} from "@tabler/icons-react"
import { useCallback, useEffect, useState, type ComponentType } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  type ModelInfo,
  type ModelProviderOption,
  getModels,
  setDefaultModel,
} from "@/api/models"
import {
  getModelSettings,
  updateModelSettings,
} from "@/api/model-settings"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { showSaveSuccessOrRestartToast } from "@/lib/restart-required"
import { refreshGatewayState } from "@/store/gateway"

import { AddModelSheet } from "./add-model-sheet"
import { CatalogDialog } from "./catalog-dialog"
import { DeleteModelDialog } from "./delete-model-dialog"
import { EditModelSheet } from "./edit-model-sheet"
import {
  getCanonicalProviderKey,
  getProviderCatalogMap,
} from "./provider-registry"
import { ModelSettingsDialog } from "./model-settings-dialog"
import { ProviderSection } from "./provider-section"
import type { ProviderCatalogEntry } from "./provider-registry"

interface ProviderGroup {
  key: string
  provider: Pick<ProviderCatalogEntry, "key" | "label" | "iconSlug" | "domain">
  models: ModelInfo[]
  hasDefault: boolean
  availableCount: number
}

export function ModelsPage() {
  const { t } = useTranslation()
  const [models, setModels] = useState<ModelInfo[]>([])
  const [providerOptions, setProviderOptions] = useState<
    ModelProviderOption[]
  >([])
  const [loading, setLoading] = useState(true)
  const [fetchError, setFetchError] = useState("")

  const [editingModel, setEditingModel] = useState<ModelInfo | null>(null)
  const [deletingModel, setDeletingModel] = useState<ModelInfo | null>(null)
  const [addOpen, setAddOpen] = useState(false)
  const [catalogOpen, setCatalogOpen] = useState(false)
  const [settingDefaultIndex, setSettingDefaultIndex] = useState<number | null>(
    null,
  )
  const providerMap = getProviderCatalogMap(providerOptions)

  // Dynamic import for CatalogDialog (added in PR2)
  const [CatalogDialogComp, setCatalogDialogComp] = useState<ComponentType<{
    open: boolean; onClose: () => void; onModelAdded: () => void;
  }> | null>(null)
  useEffect(() => {
    import("./catalog-dialog").then((m) => setCatalogDialogComp(() => m.CatalogDialog)).catch(() => {})
  }, [])

  // ModelSettings 相关状态
  const [activeModel, setActiveModel] = useState("")
  const [fallbacks, setFallbacks] = useState<string[]>([])
  const [savingGlobal, setSavingGlobal] = useState(false)
  const [globalSettingsOpen, setGlobalSettingsOpen] = useState(false)

  const fetchModels = useCallback(async () => {
    setLoading(true)
    try {
      const [data, settings] = await Promise.all([
        getModels(),
        getModelSettings(),
      ])
      const sorted = [...data.models].sort((a, b) => {
        if (a.is_default && !b.is_default) return -1
        if (!a.is_default && b.is_default) return 1
        if (a.available && !b.available) return -1
        if (!a.available && b.available) return 1
        return a.model_name.localeCompare(b.model_name)
      })
      setModels(sorted)
      setProviderOptions(data.provider_options ?? [])
      setActiveModel(settings.model_name)
      setFallbacks(settings.model_fallbacks || [])
      setFetchError("")
    } catch (e) {
      setFetchError(e instanceof Error ? e.message : t("models.loadError"))
    } finally {
      setLoading(false)
    }
  }, [t, setActiveModel, setFallbacks])

  useEffect(() => {
    fetchModels()
  }, [fetchModels])

  const handleSetDefault = async (model: ModelInfo) => {
    if (model.is_default) return

    setSettingDefaultIndex(model.index)
    try {
      await setDefaultModel(model.model_name)
      await fetchModels()
      const gateway = await refreshGatewayState({ force: true })
      showSaveSuccessOrRestartToast(
        t,
        t("models.defaultChangeSuccess"),
        model.model_name,
        gateway?.restartRequired === true,
      )
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("models.loadError"))
    } finally {
      setSettingDefaultIndex(null)
    }
  }

  // ModelSettings 保存全局设置
  const handleSaveGlobalSettings = async () => {
    if (!activeModel) return
    setSavingGlobal(true)
    try {
      await updateModelSettings({
        model_name: activeModel,
        model_fallbacks: fallbacks,
      })
      await fetchModels()
      const gateway = await refreshGatewayState({ force: true })
      showSaveSuccessOrRestartToast(
        t,
        t("models.settingsSaved"),
        activeModel,
        gateway?.restartRequired === true,
      )
      setGlobalSettingsOpen(false)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("models.loadError"))
    } finally {
      setSavingGlobal(false)
    }
  }

  // ModelSettings 添加备选模型
  const addFallback = (modelName: string) => {
    if (!modelName || fallbacks.includes(modelName) || modelName === activeModel)
      return
    setFallbacks((prev) => [...prev, modelName])
  }

  // ModelSettings 移除备选模型
  const removeFallback = (modelName: string) => {
    setFallbacks((prev) => prev.filter((f) => f !== modelName))
  }

  // ModelSettings 移动备选模型位置（上移/下移）
  const handleMoveFallback = (index: number, direction: "up" | "down") => {
    setFallbacks((prev) => {
      const newFallbacks = [...prev]
      if (direction === "up" && index > 0) {
        ;[newFallbacks[index], newFallbacks[index - 1]] = [
          newFallbacks[index - 1],
          newFallbacks[index],
        ]
      } else if (direction === "down" && index < newFallbacks.length - 1) {
        ;[newFallbacks[index], newFallbacks[index + 1]] = [
          newFallbacks[index + 1],
          newFallbacks[index],
        ]
      }
      return newFallbacks
    })
  }

  const grouped: Record<
    string,
    { provider: Pick<ProviderCatalogEntry, "key" | "label" | "iconSlug" | "domain">; models: ModelInfo[] }
  > = {}
  for (const model of models) {
    const providerKey = getCanonicalProviderKey(model.provider, providerOptions)
    const providerDef = providerKey ? providerMap.get(providerKey) : undefined
    if (!grouped[providerKey]) {
      grouped[providerKey] = {
        provider: {
          key: providerKey,
          label: providerDef?.label || providerKey,
          iconSlug: providerDef?.iconSlug,
          domain: providerDef?.domain,
        },
        models: [],
      }
    }
    grouped[providerKey].models.push(model)
  }

  const providerGroups: ProviderGroup[] = Object.entries(grouped)
    .map(([key, group]) => {
      const availableCount = group.models.filter(
        (model) => model.available,
      ).length
      return {
        key,
        provider: group.provider,
        models: group.models,
        hasDefault: group.models.some((model) => model.is_default),
        availableCount,
      }
    })
    .sort((a, b) => {
      if (a.hasDefault && !b.hasDefault) return -1
      if (!a.hasDefault && b.hasDefault) return 1

      if (a.availableCount !== b.availableCount) {
        return b.availableCount - a.availableCount
      }

      const aPriority = -(providerMap.get(a.key)?.priority ?? 0)
      const bPriority = -(providerMap.get(b.key)?.priority ?? 0)
      if (aPriority !== bPriority) {
        return aPriority - bPriority
      }

      return a.provider.label.localeCompare(b.provider.label)
    })

  const defaultModel = models.find((model) => model.is_default)

  return (
    <div className="flex h-full flex-col">
      <PageHeader title={t("navigation.models")}>
        <div className="flex items-center gap-3">
          <Button
            size="sm"
            variant="outline"
            onClick={() => setGlobalSettingsOpen(true)}
          >
            <IconSettings className="size-4" />
            {t("models.globalSettings.configure", "Configure")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={!CatalogDialogComp || providerOptions.length === 0}
            onClick={() => setCatalogOpen(true)}
          >
            <IconDatabase className="size-4" />
            {t("models.catalog.button")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setAddOpen(true)}
            disabled={providerOptions.length === 0}
          >
            <IconPlus className="size-4" />
            {t("models.add.button")}
          </Button>
        </div>
      </PageHeader>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 sm:px-6">
        <div className="pt-2">
          {!defaultModel && (
            <div className="text-muted-foreground flex items-center gap-1.5 text-sm">
              <span>{t("models.noDefaultHintPrefix")}</span>
              <IconStar className="size-3.5 shrink-0" />
              <span>{t("models.noDefaultHintSuffix")}</span>
            </div>
          )}
          <p className="text-muted-foreground mt-1 text-sm">
            {t("models.description")}
          </p>
          {!loading && providerOptions.length === 0 && (
            <p className="text-muted-foreground mt-1 text-sm">
              {t("models.providerCatalogUnavailable")}
            </p>
          )}
        </div>

        {loading && (
          <div className="flex items-center justify-center py-20">
            <IconLoader2 className="text-muted-foreground size-6 animate-spin" />
          </div>
        )}

        {fetchError && (
          <div className="bg-destructive/10 rounded-lg px-4 py-3 text-sm">
            <p className="text-destructive">{fetchError}</p>
            <div className="mt-3 flex items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  void fetchModels()
                }}
              >
                {t("models.retry")}
              </Button>
            </div>
          </div>
        )}

        {!loading && !fetchError && (
          <div className="pb-8">
            {providerGroups.map((providerGroup) => (
              <ProviderSection
                key={providerGroup.key}
                provider={providerGroup.provider}
                models={providerGroup.models}
                onEdit={setEditingModel}
                onSetDefault={handleSetDefault}
                onDelete={setDeletingModel}
                settingDefaultIndex={settingDefaultIndex}
              />
            ))}
          </div>
        )}
      </div>

      {/* ModelSettings 弹窗 */}
      <ModelSettingsDialog
        open={globalSettingsOpen}
        onOpenChange={setGlobalSettingsOpen}
        models={models}
        activeModel={activeModel}
        fallbacks={fallbacks}
        saving={savingGlobal}
        onActiveModelChange={setActiveModel}
        onAddFallback={addFallback}
        onRemoveFallback={removeFallback}
        onMoveFallback={handleMoveFallback}
        onSave={handleSaveGlobalSettings}
      />

      <EditModelSheet
        model={editingModel}
        open={editingModel !== null}
        onClose={() => setEditingModel(null)}
        onSaved={fetchModels}
        providerOptions={providerOptions}
      />

      <AddModelSheet
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onSaved={fetchModels}
        existingModelNames={models.map((model) => model.model_name)}
        providerOptions={providerOptions}
      />

      <DeleteModelDialog
        model={deletingModel}
        onClose={() => setDeletingModel(null)}
        onDeleted={fetchModels}
      />

      <CatalogDialog
        open={catalogOpen}
        onClose={() => setCatalogOpen(false)}
        onModelAdded={fetchModels}
        providerOptions={providerOptions}
      />
    </div>
  )
}
