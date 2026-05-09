import {
  IconEdit,
  IconFilter,
  IconHistory,
  IconPlayerPlay,
  IconPlayerStop,
  IconPlus,
  IconSearch,
  IconSubtask,
  IconTrash,
  IconUpload,
} from "@tabler/icons-react"
import { useEffect, useRef, useState, type DragEvent } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "@tanstack/react-router"

import type { WorkflowInstance, WorkflowListItem } from "@/api/workflow"
import {
  formatInstanceStatus,
  formatTriggerDescription,
  getInstanceStatusColor,
} from "@/api/workflow"
import { getWorkflowInstances, deleteWorkflowInstance, importWorkflow } from "@/api/workflow"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

import { DeleteDialog } from "./delete-dialog"
import { WorkflowImportDialog } from "./import-dialog"
import { type WorkflowFilter, useWorkflows } from "./use-workflows"

export function WorkflowsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const {
    filteredWorkflows,
    isLoading,
    error,
    filter,
    searchQuery,
    setFilter,
    setSearchQuery,
    handleDelete,
    handleToggle,
    handleRun,
    handleStop,
    isDeleting,
    isToggling,
    isRunning,
  } = useWorkflows()

  const [deleteWorkflow, setDeleteWorkflow] = useState<WorkflowListItem | null>(null)
  const [historyWorkflow, setHistoryWorkflow] = useState<string | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [importPending, setImportPending] = useState(false)
  const [isDragActive, setIsDragActive] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const dragDepthRef = useRef(0)

  const onDeleteConfirm = () => {
    if (deleteWorkflow) {
      handleDelete(deleteWorkflow.name)
      setDeleteWorkflow(null)
    }
  }

  const validateImportFile = (file: File): string | null => {
    const fileName = file.name.toLowerCase()
    if (!fileName.endsWith(".yml") && !fileName.endsWith(".yaml")) {
      return t("pages.workflows.import_invalid_type")
    }
    if (file.size > 1 << 20) {
      return t("pages.workflows.import_invalid_size")
    }
    return null
  }

  const doImport = async (file: File) => {
    const err = validateImportFile(file)
    if (err) {
      alert(err)
      return
    }
    setImportPending(true)
    try {
      const yamlContent = await file.text()
      await importWorkflow(yamlContent)
      setImportOpen(false)
      window.location.reload()
    } catch (err) {
      alert(err instanceof Error ? err.message : t("pages.workflows.import_error"))
    }
    setImportPending(false)
  }

  const handleImportFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) await doImport(file)
    if (fileInputRef.current) fileInputRef.current.value = ""
  }

  const handleDragEnter = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    dragDepthRef.current++
    setIsDragActive(true)
  }

  const handleDragLeave = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    dragDepthRef.current--
    if (dragDepthRef.current <= 0) {
      dragDepthRef.current = 0
      setIsDragActive(false)
    }
  }

  const handleDrop = async (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    dragDepthRef.current = 0
    setIsDragActive(false)
    const file = e.dataTransfer.files[0]
    if (file) await doImport(file)
  }

  if (error) {
    return (
      <div className="bg-background flex h-full flex-col">
        <PageHeader title={t("navigation.workflows", "Workflows")} />
        <div className="flex flex-1 items-center justify-center">
          <div className="text-center">
            <p className="text-destructive">
              {t("pages.workflows.load_error", "Failed to load workflows")}
            </p>
            <Button
              variant="outline"
              className="mt-4"
              onClick={() => window.location.reload()}
            >
              {t("common.retry", "Retry")}
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="bg-background flex h-full flex-col">
      <PageHeader title={t("navigation.workflows", "Workflows")}>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => setImportOpen(true)}>
            <IconUpload className="mr-2 h-4 w-4" />
            {t("pages.workflows.import", "Import")}
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".yml,.yaml"
            className="hidden"
            onChange={handleImportFileChange}
          />
          <Button onClick={() => void navigate({ to: "/workflows/editor", search: { name: "" } })}>
            <IconPlus className="mr-2 h-4 w-4" />
            {t("pages.workflows.create", "Create Workflow")}
          </Button>
        </div>
      </PageHeader>

      <div className="border-b px-6 py-4">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="relative max-w-md flex-1">
            <IconSearch className="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
            <Input
              placeholder={t(
                "pages.workflows.search_placeholder",
                "Search workflows...",
              )}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9"
            />
          </div>
          <div className="flex items-center gap-2">
            <IconFilter className="text-muted-foreground h-4 w-4" />
            <Select
              value={filter}
              onValueChange={(v) => setFilter(v as WorkflowFilter)}
            >
              <SelectTrigger className="w-[140px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {t("pages.workflows.filter_all", "All")}
                </SelectItem>
                <SelectItem value="enabled">
                  {t("pages.workflows.filter_enabled", "Enabled")}
                </SelectItem>
                <SelectItem value="disabled">
                  {t("pages.workflows.filter_disabled", "Disabled")}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto px-6 py-6">
        <div className="mx-auto w-full max-w-6xl">
          {isLoading ? (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {[...Array(6)].map((_, i) => (
                <div
                  key={i}
                  className="bg-card h-48 animate-pulse rounded-lg border"
                />
              ))}
            </div>
          ) : filteredWorkflows.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20">
              <IconSubtask className="text-muted-foreground mb-4 h-12 w-12" />
              <h3 className="text-lg font-medium">
                {t("pages.workflows.empty_title", "No workflows yet")}
              </h3>
              <p className="text-muted-foreground mt-1">
                {t(
                  "pages.workflows.empty_description",
                  "Create your first workflow to automate multi-step tasks.",
                )}
              </p>
              <Button className="mt-4" onClick={() => void navigate({ to: "/workflows/editor", search: { name: "" } })}>
                <IconPlus className="mr-2 h-4 w-4" />
                {t("pages.workflows.create", "Create Workflow")}
              </Button>
            </div>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {filteredWorkflows.map((wf) => (
                <WorkflowCard
                  key={wf.name}
                  workflow={wf}
                  onToggle={handleToggle}
                  onRun={handleRun}
                  onStop={handleStop}
                  onEdit={(name) =>
                    void navigate({ to: "/workflows/editor", search: { name } })
                  }
                  onDelete={(name) => {
                    const found = filteredWorkflows.find(
                      (w) => w.name === name,
                    )
                    setDeleteWorkflow(found || null)
                  }}
                  onHistory={(name) => setHistoryWorkflow(name)}
                  isToggling={isToggling}
                  isRunning={isRunning}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      <InstanceDialog
        workflowName={historyWorkflow}
        open={!!historyWorkflow}
        onOpenChange={(open: boolean) => !open && setHistoryWorkflow(null)}
      />

      <DeleteDialog
        workflow={deleteWorkflow}
        open={!!deleteWorkflow}
        onOpenChange={(open) => !open && setDeleteWorkflow(null)}
        onConfirm={onDeleteConfirm}
        isDeleting={isDeleting}
      />

      <WorkflowImportDialog
        open={importOpen}
        isImportPending={importPending}
        isDragActive={isDragActive}
        onOpenChange={setImportOpen}
        onImportClick={() => fileInputRef.current?.click()}
        onDragEnter={handleDragEnter}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      />
    </div>
  )
}

function InstanceDialog({
  workflowName,
  open,
  onOpenChange,
}: {
  workflowName: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const [instances, setInstances] = useState<WorkflowInstance[]>([])
  const [loading, setLoading] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const loadInstances = async (name: string) => {
    setLoading(true)
    try {
      const data = await getWorkflowInstances(name)
      setInstances(data.instances || [])
    } catch {
      setInstances([])
    }
    setLoading(false)
  }

  // workflowName 变化时加载数据（点击历史按钮触发）
  useEffect(() => {
    if (workflowName) {
      setExpandedId(null)
      loadInstances(workflowName)
    }
  }, [workflowName])

  const handleOpenChange = (nextOpen: boolean) => {
    onOpenChange(nextOpen)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-4xl max-h-[85vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle>
            {t("pages.workflows.instance_title", "Execution History")}: {workflowName}
          </DialogTitle>
        </DialogHeader>
        <div className="flex-1 overflow-auto">
          {loading ? (
            <div className="py-8 text-center text-muted-foreground">
              {t("common.loading", "Loading...")}
            </div>
          ) : instances.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground">
              {t("pages.workflows.no_instances", "No execution records")}
            </div>
          ) : (
            <div className="space-y-2">
              {instances.map((inst) => (
                <div key={inst.id} className="rounded-lg border">
                  <button
                    type="button"
                    className="flex w-full items-center justify-between px-4 py-3 text-left hover:bg-muted/50"
                    onClick={() => setExpandedId(expandedId === inst.id ? null : inst.id)}
                  >
                    <div className="flex items-center gap-3">
                      <Badge variant="outline" className={getInstanceStatusColor(inst.status)}>
                        {formatInstanceStatus(inst.status, t)}
                      </Badge>
                      <span className="text-sm text-muted-foreground">
                        {inst.trigger_type === "cron"
                          ? t("pages.workflows.trigger_cron", "Cron")
                          : inst.trigger_type === "event"
                            ? t("pages.workflows.trigger_event", "Event")
                            : t("pages.workflows.trigger_manual", "Manual")}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {new Date(inst.started_at).toLocaleString()}
                      </span>
                      {inst.channel && (
                        <span className="text-xs text-muted-foreground">
                          {t("pages.workflows.notify_channel", "Channel")}: {t(`channels.name.${inst.channel}`, inst.channel)}{inst.chat_id ? `:${inst.chat_id}` : ""}
                        </span>
                      )}
                    </div>
                    <span className="text-xs font-mono text-muted-foreground">
                      {inst.id.slice(0, 8)}
                    </span>
                  </button>
                  {expandedId === inst.id && (
                    <div className="border-t px-4 py-3 space-y-3">
                      {Object.entries(inst.step_states).length > 0 && (
                        <div>
                          <p className="text-xs font-medium mb-2">
                            {t("pages.workflows.step_progress", "Step Progress")}
                          </p>
                          <div className="space-y-1">
                            {Object.entries(inst.step_states)
                              .sort(([, a], [, b]) => {
                                if (a.started_at && b.started_at) return a.started_at.localeCompare(b.started_at)
                                if (a.started_at) return -1
                                if (b.started_at) return 1
                                return 0
                              })
                              .map(([stepID, state]) => (
                              <div key={stepID} className="flex items-center gap-2 text-xs">
                                <span className={getInstanceStatusColor(state.status)}>
                                  {state.status === "completed" ? "✓" : state.status === "failed" ? "✗" : state.status === "running" ? "⏳" : "○"}
                                </span>
                                <span>{stepID}</span>
                                {state.error && (
                                  <span className="text-destructive ml-2">{state.error}</span>
                                )}
                              </div>
                            ))}
                          </div>
                        </div>
                      )}
                      {inst.logs && inst.logs.length > 0 && (
                        <div>
                          <p className="text-xs font-medium mb-2">
                            {t("pages.workflows.execution_logs", "Execution Logs")}
                          </p>
                          <div className="max-h-40 overflow-auto rounded bg-muted/30 p-2 font-mono text-xs space-y-0.5">
                            {inst.logs.map((log, i) => (
                              <div
                                key={i}
                                className={`flex gap-2 ${
                                  log.level === "error"
                                    ? "text-destructive"
                                    : log.level === "warn"
                                      ? "text-amber-600"
                                      : "text-foreground/80"
                                }`}
                              >
                                <span className="text-muted-foreground shrink-0">
                                  {new Date(log.timestamp).toLocaleTimeString()}
                                </span>
                                {log.step_id && (
                                  <span className="text-blue-600 shrink-0">[{log.step_id}]</span>
                                )}
                                <span>{log.message}</span>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}
                      {inst.error && (
                        <p className="text-xs text-destructive">{inst.error}</p>
                      )}
                      <div className="flex justify-end">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-destructive hover:text-destructive hover:bg-destructive/10 h-7 text-xs"
                          onClick={async () => {
                            if (!confirm(t("pages.workflows.delete_instance_confirm", "Are you sure you want to delete this execution record?"))) return
                            try {
                              await deleteWorkflowInstance(inst.workflow_name, inst.id)
                              setInstances(instances.filter(i => i.id !== inst.id))
                            } catch {
                              // ignore
                            }
                          }}
                        >
                          <IconTrash className="mr-1 h-3 w-3" />
                          {t("common.delete", "Delete")}
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

function WorkflowCard({
  workflow,
  onToggle,
  onRun,
  onStop,
  onEdit,
  onDelete,
  onHistory,
  isToggling,
  isRunning,
}: {
  workflow: WorkflowListItem
  onToggle: (name: string, enabled: boolean) => void
  onRun: (name: string) => void
  onStop: (name: string) => void
  onEdit: (name: string) => void
  onDelete: (name: string) => void
  onHistory: (name: string) => void
  isToggling: boolean
  isRunning: boolean
}) {
  const { t } = useTranslation()
  const triggerLabel = workflow.trigger_type === "cron"
    ? t("pages.workflows.trigger_cron", "Cron")
    : workflow.trigger_type === "event"
      ? t("pages.workflows.trigger_event", "Event")
      : t("pages.workflows.trigger_manual", "Manual")
  const triggerDesc = (workflow.triggers && workflow.triggers.length > 0)
    ? formatTriggerDescription(workflow.triggers, t)
    : triggerLabel

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <div className="flex-1 min-w-0">
            <CardTitle className="text-base">{workflow.name}</CardTitle>
            <p className="text-muted-foreground mt-1 line-clamp-2 text-sm">
              {workflow.description || t("pages.workflows.no_description", "No description")}
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-0.5 ml-2">
            {workflow.trigger_type !== "manual" && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span tabIndex={0}>
                    <Switch
                      checked={workflow.enabled}
                      onCheckedChange={(checked) => onToggle(workflow.name, checked)}
                      disabled={isToggling}
                    />
                  </span>
                </TooltipTrigger>
                <TooltipContent>
                  {workflow.enabled
                    ? t("common.disable", "Disable")
                    : t("common.enable", "Enable")}
                </TooltipContent>
              </Tooltip>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="mb-3 flex items-center gap-2">
          <Badge variant="outline">{triggerLabel}</Badge>
          <Badge variant="secondary">
            {t("pages.workflows.step_count", "{{count}} steps", { count: workflow.step_count })}
          </Badge>
          {workflow.trigger_type !== "manual" && (
            <span className="text-muted-foreground text-xs ml-1">{triggerDesc}</span>
          )}
        </div>

        <TooltipProvider delayDuration={300}>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-0.5">
              <Tooltip>
                <TooltipTrigger asChild>
                  <span tabIndex={0}>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => onRun(workflow.name)}
                      disabled={isRunning}
                    >
                      <IconPlayerPlay className="h-3.5 w-3.5" />
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>{t("pages.workflows.run", "Run")}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span tabIndex={0}>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => onHistory(workflow.name)}
                    >
                      <IconHistory className="h-3.5 w-3.5" />
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>{t("pages.workflows.history", "History")}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span tabIndex={0}>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => onStop(workflow.name)}
                      disabled={!workflow.enabled}
                    >
                      <IconPlayerStop className="h-3.5 w-3.5" />
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>{t("pages.workflows.stop", "Stop")}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span tabIndex={0}>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => onEdit(workflow.name)}
                      disabled={workflow.enabled}
                    >
                      <IconEdit className="h-3.5 w-3.5" />
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>{t("pages.workflows.edit", "Edit")}</TooltipContent>
              </Tooltip>
            </div>
            <Tooltip>
              <TooltipTrigger asChild>
                <span tabIndex={0}>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-destructive hover:text-destructive hover:bg-destructive/10 h-7 w-7"
                    onClick={() => onDelete(workflow.name)}
                    disabled={workflow.enabled}
                  >
                    <IconTrash className="h-3.5 w-3.5" />
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent>{t("common.delete", "Delete")}</TooltipContent>
            </Tooltip>
          </div>
        </TooltipProvider>
      </CardContent>
    </Card>
  )
}
