import {
  IconBolt,
  IconCheck,
  IconLoader2,
  IconSearch,
  IconStar,
} from "@tabler/icons-react"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"

import { addModel } from "@/api/models"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"

import { PROVIDER_API_BASES } from "./config"
import { PRESET_MODELS } from "./presets"
import { ProviderIcon } from "./provider-icon"
import type { ModelTag, ModelTemplate } from "./types"

interface QuickAddModelSheetProps {
  open: boolean
  onClose: () => void
  onImported: () => void
  existingModelNames: string[]
}

export function QuickAddModelSheet({
  open,
  onClose,
  onImported,
  existingModelNames,
}: QuickAddModelSheetProps) {
  const { t } = useTranslation()
  const [search, setSearch] = useState("")
  const [filterTag, setFilterTag] = useState<ModelTag | null>(null)
  const [importing, setImporting] = useState<string | null>(null)
  const [imported, setImported] = useState<string | null>(null)
  const existingNamesString = existingModelNames.join(",")

  useEffect(() => {
    if (open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSearch("")
       
      setFilterTag(null)
       
      setImporting(null)
       
      setImported(null)
    }
  }, [open])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setImported(null)
  }, [search, filterTag, existingNamesString])

  const filteredModels = PRESET_MODELS.filter((m) => {
    const matchesSearch =
      m.name.toLowerCase().includes(search.toLowerCase()) ||
      m.model.toLowerCase().includes(search.toLowerCase()) ||
      m.provider.toLowerCase().includes(search.toLowerCase())
    const matchesTag = !filterTag || m.tag === filterTag
    return matchesSearch && matchesTag
  })

  const handleImport = async (template: ModelTemplate) => {
    if (existingModelNames.some((n) => n === template.name)) {
      return
    }
    setImporting(template.model)
    try {
      await addModel({
        model_name: template.name,
        model: template.model,
        api_base: PROVIDER_API_BASES[template.provider],
      })
      setImported(template.model)
      onImported()
      setTimeout(() => {
        onClose()
      }, 500)
    } finally {
      setImporting(null)
    }
  }

  return (
    <Sheet open={open} onOpenChange={(v) => !v && onClose()}>
      <SheetContent
        side="right"
        className="flex flex-col gap-0 p-0 sm:!w-[480px] sm:!max-w-[480px]"
      >
        <SheetHeader className="border-b-muted border-b px-6 py-5">
          <SheetTitle className="text-base">
            {t("models.quickAdd.title")}
          </SheetTitle>
          <SheetDescription className="text-xs">
            {t("models.quickAdd.description")}
          </SheetDescription>
        </SheetHeader>

        <div className="border-b-muted border-b px-6 py-3">
          <div className="mb-2 flex items-center gap-2">
            <Button
              size="sm"
              variant={filterTag === null ? "secondary" : "ghost"}
              onClick={() => setFilterTag(null)}
            >
              {t("models.quickAdd.filterAll")}
            </Button>
            <Button
              size="sm"
              variant={filterTag === "popular" ? "secondary" : "ghost"}
              onClick={() => setFilterTag("popular")}
            >
              <IconStar className="mr-1 size-3" />
              {t("models.quickAdd.filterPopular")}
            </Button>
            <Button
              size="sm"
              variant={filterTag === "free" ? "secondary" : "ghost"}
              onClick={() => setFilterTag("free")}
            >
              <IconBolt className="mr-1 size-3" />
              {t("models.quickAdd.filterFree")}
            </Button>
          </div>
          <div className="relative">
            <IconSearch className="text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2" />
            <Input
              className="pl-9"
              placeholder={t("models.quickAdd.searchPlaceholder")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          <div className="grid grid-cols-1 gap-1 p-2">
            {filteredModels.map((template) => {
              const isImported = imported === template.model
              const isImporting = importing === template.model
              const isDuplicate = existingModelNames.some(
                (n) => n === template.name,
              )

              return (
                <button
                  key={template.model}
                  type="button"
                  onClick={() => handleImport(template)}
                  disabled={isImporting || isDuplicate || isImported}
                  className="hover:bg-muted flex items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <ProviderIcon
                    providerKey={template.provider}
                    providerLabel={template.provider}
                  />
                  <div className="min-w-0 flex-1">
                    <div className="text-foreground text-sm font-medium">
                      {template.name}
                    </div>
                    <div className="text-muted-foreground truncate font-mono text-xs">
                      {template.model}
                    </div>
                  </div>
                  {isImported && (
                    <IconCheck className="size-5 shrink-0 text-green-500" />
                  )}
                  {isImporting && (
                    <IconLoader2 className="text-muted-foreground size-4 shrink-0 animate-spin" />
                  )}
                  {isDuplicate && (
                    <span className="text-muted-foreground text-xs">
                      {t("models.quickAdd.alreadyAdded")}
                    </span>
                  )}
                </button>
              )
            })}
          </div>

          {filteredModels.length === 0 && (
            <div className="text-muted-foreground py-12 text-center text-sm">
              {t("models.quickAdd.noResults")}
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
