import { useEffect, useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Card, CardContent } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  IconPlus,
  IconTrash,
  IconUpload,
  IconDownload,
  IconEye,
  IconEyeOff,
  IconEdit,
} from "@tabler/icons-react"

import {
  getEnvVarsConfig,
  updateEnvVarsConfig,
  importEnvFile,
  exportEnvFile,
  type EnvVarEntry,
} from "@/api/env-vars"

interface EnvVarFormData {
  key: string
  value: string
  enabled: boolean
  sensitive: boolean
  note: string
}

const EMPTY_FORM: EnvVarFormData = {
  key: "",
  value: "",
  enabled: true,
  sensitive: false,
  note: "",
}

export function EnvironmentPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [variables, setVariables] = useState<EnvVarEntry[]>([])
  const [isDirty, setIsDirty] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingIndex, setEditingIndex] = useState<number | null>(null)
  const [form, setForm] = useState<EnvVarFormData>(EMPTY_FORM)
  const [showSecret, setShowSecret] = useState(false)
  // 跟踪列表中哪些敏感变量应该显示真实值
  const [visibleSecrets, setVisibleSecrets] = useState<Set<number>>(new Set())

  const { data, isLoading } = useQuery({
    queryKey: ["env-vars"],
    queryFn: getEnvVarsConfig,
  })

  useEffect(() => {
    if (data) {
      setVariables(data.variables)
    }
  }, [data])

  const updateMutation = useMutation({
    mutationFn: updateEnvVarsConfig,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["env-vars"] })
      setIsDirty(false)
      toast.success(t("environment.save_success"))
    },
    onError: (error) => {
      toast.error(t("environment.save_error", { error: String(error) }))
    },
  })

  const handleSave = useCallback(() => {
    updateMutation.mutate({
      variables,
    })
  }, [variables, updateMutation])

  const handleAdd = useCallback(() => {
    setEditingIndex(null)
    setForm(EMPTY_FORM)
    setShowSecret(false)
    setDialogOpen(true)
  }, [])

  const handleEdit = useCallback((index: number) => {
    const v = variables[index]
    setEditingIndex(index)
    setForm({
      key: v.key,
      value: v.value,
      enabled: v.enabled,
      sensitive: v.sensitive,
      note: v.note,
    })
    setShowSecret(false)
    setDialogOpen(true)
  }, [variables])

  const handleDelete = useCallback((index: number) => {
    setVariables((prev) => prev.filter((_, i) => i !== index))
    setIsDirty(true)
  }, [])

  const handleToggleEnabled = useCallback((index: number) => {
    setVariables((prev) =>
      prev.map((v, i) => (i === index ? { ...v, enabled: !v.enabled } : v))
    )
    setIsDirty(true)
  }, [])

  const handleToggleSensitive = useCallback((index: number) => {
    setVisibleSecrets((prev) => {
      const newSet = new Set(prev)
      if (newSet.has(index)) {
        newSet.delete(index)
      } else {
        newSet.add(index)
      }
      return newSet
    })
  }, [])

  const handleSaveForm = useCallback(() => {
    if (!form.key.trim()) {
      toast.error(t("environment.key_required"))
      return
    }

    const newVar: EnvVarEntry = {
      key: form.key.trim(),
      value: form.value,
      enabled: form.enabled,
      sensitive: form.sensitive,
      note: form.note,
    }

    if (editingIndex !== null) {
      setVariables((prev) =>
        prev.map((v, i) => (i === editingIndex ? newVar : v))
      )
    } else {
      setVariables((prev) => [...prev, newVar])
    }

    setIsDirty(true)
    setDialogOpen(false)
  }, [form, editingIndex, t])

  const handleImport = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    try {
      const imported = await importEnvFile(file)
      setVariables((prev) => [...prev, ...imported])
      setIsDirty(true)
      toast.success(t("environment.import_success", { count: imported.length }))
    } catch (error) {
      toast.error(t("environment.import_error", { error: String(error) }))
    }

    // 重置输入
    e.target.value = ""
  }, [t])

  const handleExport = useCallback(async () => {
    try {
      const blob = await exportEnvFile()
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = "picoclaw.env"
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      window.URL.revokeObjectURL(url)
      toast.success(t("environment.export_success"))
    } catch (error) {
      toast.error(t("environment.export_error", { error: String(error) }))
    }
  }, [t])

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-muted-foreground">{t("common.loading")}</div>
      </div>
    )
  }

  return (
    <div className="container mx-auto max-w-5xl py-8">
      <div className="mb-8">
        <h1 className="text-2xl font-semibold">{t("environment.title")}</h1>
        <p className="text-muted-foreground mt-1">
          {t("environment.description")}
        </p>
      </div>

      <div className="space-y-6">
        {/* 操作按钮 */}
        <div className="flex items-center gap-2">
          <Button onClick={handleAdd} variant="default" size="sm">
            <IconPlus className="mr-2 h-4 w-4" />
            {t("environment.add_variable")}
          </Button>

          <div className="relative">
            <input
              type="file"
              accept=".env"
              onChange={handleImport}
              className="absolute inset-0 cursor-pointer opacity-0"
              id="env-import"
            />
            <Button variant="outline" size="sm" asChild>
              <label htmlFor="env-import" className="cursor-pointer">
                <IconUpload className="mr-2 h-4 w-4" />
                {t("environment.import")}
              </label>
            </Button>
          </div>

          <Button variant="outline" size="sm" onClick={handleExport}>
            <IconDownload className="mr-2 h-4 w-4" />
            {t("environment.export")}
          </Button>
        </div>

        {/* 变量列表 */}
        <div className="space-y-2">
          {variables.length === 0 ? (
            <Card>
              <CardContent className="flex h-24 items-center justify-center">
                <p className="text-muted-foreground text-sm">{t("environment.no_variables")}</p>
              </CardContent>
            </Card>
          ) : (
            variables.map((v, index) => (
              <div
                key={index}
                className={`flex items-center justify-between py-2 px-3 rounded-md border ${!v.enabled ? "opacity-50 bg-muted/30" : "bg-card hover:bg-accent/50"}`}
              >
                <div className="flex-1 min-w-0 flex items-center gap-3">
                  <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-medium shrink-0">
                    {v.key}
                  </code>
                  <span className="text-muted-foreground text-sm truncate max-w-[200px]">
                    {v.sensitive && !visibleSecrets.has(index) ? "********" : v.value}
                  </span>
                  {v.note && (
                    <span className="text-muted-foreground text-xs truncate max-w-[150px] hidden sm:inline">
                      {v.note}
                    </span>
                  )}
                  {v.sensitive && (
                    <span className="text-[10px] bg-yellow-100 text-yellow-800 px-1.5 py-0.5 rounded shrink-0">
                      {t("environment.sensitive")}
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-2 ml-2">
                  {v.sensitive && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => handleToggleSensitive(index)}
                      title={visibleSecrets.has(index) ? t("environment.hide_secret") : t("environment.show_secret")}
                    >
                      {visibleSecrets.has(index) ? (
                        <IconEye className="h-3.5 w-3.5" />
                      ) : (
                        <IconEyeOff className="h-3.5 w-3.5" />
                      )}
                    </Button>
                  )}
                  <Switch
                    checked={v.enabled}
                    onCheckedChange={() => handleToggleEnabled(index)}
                    className="scale-75"
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    onClick={() => handleEdit(index)}
                  >
                    <IconEdit className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-destructive h-7 w-7"
                    onClick={() => handleDelete(index)}
                  >
                    <IconTrash className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            ))
          )}
        </div>

        {/* 保存按钮 */}
        {isDirty && (
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              onClick={() => {
                if (data) {
                  setVariables(data.variables)
                  setIsDirty(false)
                }
              }}
            >
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSave} disabled={updateMutation.isPending}>
              {updateMutation.isPending
                ? t("common.saving")
                : t("common.save_changes")}
            </Button>
          </div>
        )}
      </div>

      {/* 添加/编辑对话框 */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>
              {editingIndex !== null
                ? t("environment.edit_variable")
                : t("environment.add_variable")}
            </DialogTitle>
            <DialogDescription>
              {t("environment.variable_description")}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="key">{t("environment.key")} *</Label>
              <Input
                id="key"
                placeholder="API_KEY"
                value={form.key}
                onChange={(e) => setForm({ ...form, key: e.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="value">{t("environment.value")} *</Label>
              <div className="relative">
                <Input
                  id="value"
                  type={form.sensitive && !showSecret ? "password" : "text"}
                  placeholder="KEY_VALUE"
                  value={form.value}
                  onChange={(e) => setForm({ ...form, value: e.target.value })}
                />
                {form.sensitive && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="absolute right-2 top-1/2 -translate-y-1/2 h-6 w-6"
                    onClick={() => setShowSecret(!showSecret)}
                  >
                    {showSecret ? (
                      <IconEye className="h-4 w-4" />
                    ) : (
                      <IconEyeOff className="h-4 w-4" />
                    )}
                  </Button>
                )}
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="note">{t("environment.note")}</Label>
              <Input
                id="note"
                placeholder={t("environment.note_placeholder")}
                value={form.note}
                onChange={(e) => setForm({ ...form, note: e.target.value })}
              />
            </div>

            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <Switch
                  id="enabled"
                  checked={form.enabled}
                  onCheckedChange={(checked) =>
                    setForm({ ...form, enabled: checked })
                  }
                />
                <Label htmlFor="enabled">{t("environment.enabled")}</Label>
              </div>

              <div className="flex items-center space-x-2">
                <Switch
                  id="sensitive"
                  checked={form.sensitive}
                  onCheckedChange={(checked) => {
                    setForm({ ...form, sensitive: checked })
                    // 切换敏感开关时，重置显示密码的状态
                    setShowSecret(false)
                  }}
                />
                <Label htmlFor="sensitive">{t("environment.sensitive")}</Label>
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSaveForm}>
              {editingIndex !== null ? t("common.save") : t("common.add")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
