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
  IconExternalLink,
  IconClock,
} from "@tabler/icons-react"
import { useEffect, useRef, useState, type DragEvent } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "@tanstack/react-router"
import { useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { cn } from "@/lib/utils"

import type { Step, WorkflowInstance, WorkflowInstanceSummary, WorkflowListItem, CronTaskInfo } from "@/api/workflow"
import {
  createInstanceStream,
  formatInstanceStatus,
  formatTriggerDescription,
  getInstanceStatusColor,
  getCronTasks,
} from "@/api/workflow"
import { getWorkflowInstances, getWorkflowInstance, getWorkflow, deleteWorkflowInstance, importWorkflow } from "@/api/workflow"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
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

import { ConfirmDialog } from "./confirm-dialog"
import { DeleteDialog } from "./delete-dialog"
import { RunDialog } from "./run-dialog"
import { WorkflowImportDialog } from "./import-dialog"
import { type StatusFilter, type TriggerFilter, useWorkflows } from "./use-workflows"

export function WorkflowsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const {
    filteredWorkflows,
    isLoading,
    error,
    statusFilter,
    triggerFilter,
    searchQuery,
    setStatusFilter,
    setTriggerFilter,
    setSearchQuery,
    handleDelete,
    handleToggle,
    handleRun,
    handleStop,
    isDeleting,
    isToggling,
    isRunning,
    isStopping,
  } = useWorkflows()

  const [deleteWorkflow, setDeleteWorkflow] = useState<WorkflowListItem | null>(null)
  const [runWorkflow, setRunWorkflow] = useState<WorkflowListItem | null>(null)
  const [stopWorkflow, setStopWorkflow] = useState<WorkflowListItem | null>(null)
  const [toggleOffWorkflow, setToggleOffWorkflow] = useState<WorkflowListItem | null>(null)
  const [historyWorkflow, setHistoryWorkflow] = useState<string | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [importPending, setImportPending] = useState(false)
  const [isDragActive, setIsDragActive] = useState(false)
  const [cronDialogOpen, setCronDialogOpen] = useState(false)
  const [cronTasks, setCronTasks] = useState<CronTaskInfo[]>([])
  const [cronLoading, setCronLoading] = useState(false)
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
      toast.error(err)
      return
    }
    setImportPending(true)
    try {
      const yamlContent = await file.text()
      await importWorkflow(yamlContent)
      setImportOpen(false)
      await queryClient.invalidateQueries({ queryKey: ["workflows"] })
      toast.success(t("pages.workflows.import_success", "Workflow imported successfully"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("pages.workflows.import_error"))
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

  // 打开 Cron 调度弹窗时加载
  useEffect(() => {
    if (cronDialogOpen) {
      setCronLoading(true)
      getCronTasks()
        .then((data) => setCronTasks(data.tasks || []))
        .catch(() => setCronTasks([]))
        .finally(() => setCronLoading(false))
    }
  }, [cronDialogOpen])

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
          <Button variant="outline" onClick={() => setCronDialogOpen(true)}>
            <IconClock className="mr-2 h-4 w-4" />
            {t("pages.workflows.cron_schedule", "Cron Schedule")}
          </Button>
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
              placeholder={t("pages.workflows.search_placeholder", "Search workflows...")}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9"
            />
          </div>
          <div className="flex items-center gap-2">
            <IconFilter className="text-muted-foreground h-4 w-4" />
            <Select
              value={statusFilter}
              onValueChange={(v) => setStatusFilter(v as StatusFilter)}
            >
              <SelectTrigger className="w-[120px]">
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
            <Select
              value={triggerFilter}
              onValueChange={(v) => setTriggerFilter(v as TriggerFilter)}
            >
              <SelectTrigger className="w-[120px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {t("pages.workflows.filter_all", "All")}
                </SelectItem>
                <SelectItem value="manual">
                  {t("pages.workflows.trigger_manual", "Manual")}
                </SelectItem>
                <SelectItem value="cron">
                  {t("pages.workflows.trigger_cron", "Cron")}
                </SelectItem>
                <SelectItem value="event">
                  {t("pages.workflows.trigger_event", "Event")}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>

      <div
        className="flex-1 overflow-auto px-6 py-6"
        onDragEnter={handleDragEnter}
        onDragOver={(e) => e.preventDefault()}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        {isDragActive && (
          <div className="absolute inset-0 z-50 flex items-center justify-center bg-background/80">
            <p className="text-lg font-medium">{t("pages.workflows.drop_yaml", "Drop YAML file here")}</p>
          </div>
        )}
        <div className="mx-auto w-full max-w-6xl">
          {isLoading ? (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {[...Array(6)].map((_, i) => (
                <div key={i} className="bg-card h-48 animate-pulse rounded-lg border" />
              ))}
            </div>
          ) : filteredWorkflows.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20">
              <IconSubtask className="text-muted-foreground mb-4 h-12 w-12" />
              <h3 className="text-lg font-medium">
                {t("pages.workflows.empty_title", "No workflows yet")}
              </h3>
              <p className="text-muted-foreground mt-1">
                {t("pages.workflows.empty_description", "Create your first workflow to automate multi-step tasks.")}
              </p>
              <Button className="mt-4" onClick={() => void navigate({ to: "/workflows/editor", search: { name: "" } })}>
                <IconPlus className="mr-2 h-4 w-4" />
                {t("pages.workflows.create", "Create Workflow")}
              </Button>
            </div>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {filteredWorkflows.map((wf) => {
                // 获取该工作流的所有 cron 任务，找出最近的下次执行时间
                const workflowCronTasks = cronTasks.filter((ct) => ct.workflow_name === wf.name)
                const nextRun = workflowCronTasks.length > 0
                  ? workflowCronTasks.sort((a, b) => new Date(a.next_run).getTime() - new Date(b.next_run).getTime())[0].next_run
                  : null
                return (
                <WorkflowCard
                  key={wf.name}
                  workflow={wf}
                  nextRun={nextRun}
                  onToggle={(name, checked) => {
                    if (checked) {
                      handleToggle(name, checked)
                    } else {
                      const found = filteredWorkflows.find((w) => w.name === name)
                      setToggleOffWorkflow(found || null)
                    }
                  }}
                  onRun={(name) => {
                    const found = filteredWorkflows.find((w) => w.name === name)
                    setRunWorkflow(found || null)
                  }}
                  onStop={(name) => {
                    const found = filteredWorkflows.find((w) => w.name === name)
                    setStopWorkflow(found || null)
                  }}
                  onEdit={(name) =>
                    void navigate({ to: "/workflows/editor", search: { name } })
                  }
                  onDelete={(name) => {
                    const found = filteredWorkflows.find((w) => w.name === name)
                    setDeleteWorkflow(found || null)
                  }}
                  onHistory={(name) => setHistoryWorkflow(name)}
                  isToggling={isToggling}
                  isRunning={isRunning}
                />
                )
              })}
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

      <RunDialog
        workflow={runWorkflow}
        open={!!runWorkflow}
        onOpenChange={(open) => !open && setRunWorkflow(null)}
        onConfirm={() => {
          if (runWorkflow) handleRun(runWorkflow.name)
          setRunWorkflow(null)
        }}
        isRunning={isRunning}
      />

      <ConfirmDialog
        title={t("pages.workflows.stop_title", "Stop Workflow")}
        description={t("pages.workflows.stop_description", "Are you sure you want to stop \"{{name}}\"?", { name: stopWorkflow?.name || "" })}
        open={!!stopWorkflow}
        onOpenChange={(open) => !open && setStopWorkflow(null)}
        onConfirm={() => {
          if (stopWorkflow) handleStop(stopWorkflow.name)
          setStopWorkflow(null)
        }}
        confirmLabel={t("pages.workflows.stop", "Stop")}
        confirmVariant="destructive"
        isPending={isStopping}
      />

      <ConfirmDialog
        title={t("pages.workflows.disable_title", "Disable Workflow")}
        description={t("pages.workflows.disable_description", "Are you sure you want to disable \"{{name}}\"?", { name: toggleOffWorkflow?.name || "" })}
        open={!!toggleOffWorkflow}
        onOpenChange={(open) => !open && setToggleOffWorkflow(null)}
        onConfirm={() => {
          if (toggleOffWorkflow) handleToggle(toggleOffWorkflow.name, false)
          setToggleOffWorkflow(null)
        }}
        confirmLabel={t("common.disable", "Disable")}
        isPending={isToggling}
      />

      {/* Cron 调度弹窗 */}
      <Dialog open={cronDialogOpen} onOpenChange={setCronDialogOpen}>
        <DialogContent className="sm:max-w-lg max-h-[80vh] flex flex-col">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <IconClock className="h-5 w-5" />
              {t("pages.workflows.cron_schedule", "Cron Schedule")}
            </DialogTitle>
          </DialogHeader>
          <div className="py-2 flex-1 overflow-hidden flex flex-col">
            {cronLoading ? (
              <div className="py-8 text-center text-muted-foreground text-sm">
                {t("common.loading", "Loading...")}
              </div>
            ) : cronTasks.length === 0 ? (
              <div className="py-8 text-center text-muted-foreground text-sm">
                {t("pages.workflows.no_cron_tasks", "No scheduled cron tasks")}
              </div>
            ) : (
              <div className="space-y-2 overflow-y-auto pr-1">
                {cronTasks
                  .slice()
                  .sort((a, b) => {
                    // 按下次执行时间排序（从早到晚）
                    // 将时间转换为 UTC 时间戳进行比较，确保时区一致性
                    const parseToUTC = (timeStr: string): number => {
                      // "2006-01-02 15:04:05 MST" 格式
                      // 尝试直接解析，JavaScript 会尽量处理时区
                      const date = new Date(timeStr);
                      const timestamp = date.getTime();
                      
                      // 如果解析失败（NaN），尝试移除时区后按本地时间解析
                      if (isNaN(timestamp)) {
                        const dateTimePart = timeStr.replace(/\s+[A-Z]{2,4}$/, '');
                        return new Date(dateTimePart).getTime();
                      }
                      
                      return timestamp;
                    };
                    
                    const timeA = parseToUTC(a.next_run);
                    const timeB = parseToUTC(b.next_run);
                    return timeA - timeB;
                  })
                  .map((task) => (
                  <div
                    key={`${task.workflow_name}-${task.trigger_type}-${task.schedule}`}
                    className="flex items-start justify-between rounded-md border px-3 py-2.5 gap-2"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="font-medium text-sm truncate">{task.workflow_name}</p>
                      <div className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                        <span className={cn(
                          "rounded px-1.5 py-0.5 text-[10px] font-medium",
                          task.trigger_type === "cron" && "bg-blue-100 text-blue-700",
                          task.trigger_type === "at" && "bg-green-100 text-green-700",
                          task.trigger_type === "interval" && "bg-purple-100 text-purple-700"
                        )}>
                          {task.trigger_type === "cron" && "CRON"}
                          {task.trigger_type === "at" && "AT"}
                          {task.trigger_type === "interval" && "INTERVAL"}
                        </span>
                        <code className="rounded bg-muted px-1.5 py-0.5 text-[11px]">{task.schedule}</code>
                        {task.timezone && task.timezone !== "UTC" && (
                          <span className="text-[11px]">({task.timezone})</span>
                        )}
                      </div>
                    </div>
                    <div className="shrink-0 text-right">
                      <div className="text-xs font-medium text-blue-600 whitespace-nowrap">
                        → {task.next_run.split(' ').slice(0, 2).join(' ')}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>

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
  const navigate = useNavigate()
  const [instances, setInstances] = useState<WorkflowInstanceSummary[]>([])
  const [expandedInstanceId, setExpandedInstanceId] = useState<string | null>(null) // 展开的实例 ID
  const [expandedInstanceDetail, setExpandedInstanceDetail] = useState<WorkflowInstance | null>(null) // 展开的完整实例数据
  const [workflowSteps, setWorkflowSteps] = useState<Step[]>([]) // 工作流步骤定义（用于层级显示）
  const [loading, setLoading] = useState(false)
  const [deleteInstanceId, setDeleteInstanceId] = useState<string | null>(null)
  const esRef = useRef<EventSource | null>(null)
  const retryCountRef = useRef(0)
  const instancesRef = useRef(instances)
  useEffect(() => { instancesRef.current = instances })
  const MAX_SSE_RETRIES = 5

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

  // 切换展开状态，并加载完整实例数据
  const toggleExpand = async (instId: string) => {
    if (expandedInstanceId === instId) {
      // 已展开，则收起
      setExpandedInstanceId(null)
      setExpandedInstanceDetail(null)
      setWorkflowSteps([])
    } else {
      // 展开新实例
      setExpandedInstanceId(instId)
      // 异步加载完整数据和工作流定义
      try {
        const [detail, wf] = await Promise.all([
          getWorkflowInstance(workflowName!, instId),
          getWorkflow(workflowName!)
        ])
        setExpandedInstanceDetail(detail)
        setWorkflowSteps(wf.steps)
      } catch (err) {
        console.error('Failed to load instance detail:', err)
      }
    }
  }

  // workflowName 变化时加载数据（点击历史按钮触发）
  useEffect(() => {
    if (workflowName) {
      setExpandedInstanceId(null)
      setExpandedInstanceDetail(null)
      loadInstances(workflowName)
    }
  }, [workflowName])

  // 对正在运行的展开实例建立 SSE 连接，实时更新状态
  useEffect(() => {
    // 清除旧连接
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }
    retryCountRef.current = 0

    if (!workflowName || !expandedInstanceId) return

    // 仅对运行中的实例建立 SSE
    const expanded = instancesRef.current.find((i) => i.id === expandedInstanceId)
    if (!expanded || expanded.status !== "running") return

    const es = createInstanceStream(workflowName, expandedInstanceId)
    esRef.current = es

    es.onopen = () => {
      retryCountRef.current = 0
    }

    es.addEventListener("snapshot", (e) => {
      try {
        const updated = JSON.parse(e.data) as WorkflowInstance
        // 只更新摘要字段
        setInstances((prev) => prev.map((i) => {
          if (i.id !== updated.id) return i
          return {
            id: updated.id,
            workflow_name: updated.workflow_name,
            status: updated.status,
            trigger_type: updated.trigger_type,
            channel: updated.channel,
            chat_id: updated.chat_id,
            notify_channels: updated.notify_channels,
            started_at: updated.started_at,
            finished_at: updated.finished_at,
            error: updated.error,
          }
        }))
      } catch { /* ignore */ }
    })

    es.addEventListener("step_update", () => {
      // 列表页不处理步骤更新，只在详情页显示
      // 这里可以忽略，或者触发一个轻量级的状态刷新
    })

    es.addEventListener("instance_complete", () => {
      // 实例完成时，重新获取完整数据
      if (workflowName && expandedInstanceId) {
        getWorkflowInstance(workflowName, expandedInstanceId)
          .then((inst) => {
            // 更新摘要列表
            setInstances((prev) => prev.map((i) => {
              if (i.id !== inst.id) return i
              return {
                id: inst.id,
                workflow_name: inst.workflow_name,
                status: inst.status,
                trigger_type: inst.trigger_type,
                channel: inst.channel,
                chat_id: inst.chat_id,
                started_at: inst.started_at,
                finished_at: inst.finished_at,
                error: inst.error,
              }
            }))
            // 同时更新展开的详情
            setExpandedInstanceDetail(inst)
          })
          .catch(() => {})
      }
      es.close()
      esRef.current = null
    })

    es.onerror = () => {
      retryCountRef.current++
      if (retryCountRef.current >= MAX_SSE_RETRIES) {
        es.close()
        if (esRef.current === es) esRef.current = null
      }
    }

    return () => {
      es.close()
      if (esRef.current === es) esRef.current = null
    }
  }, [workflowName, expandedInstanceId])

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
                  <div className="flex w-full items-center justify-between px-4 py-3">
                    <button
                      type="button"
                      className="flex flex-1 items-center gap-3 text-left hover:bg-muted/50 -mx-4 -my-3 px-4 py-3"
                      onClick={() => toggleExpand(inst.id)}
                    >
                      <Badge variant="outline" className={getInstanceStatusColor(inst.status)}>
                        {formatInstanceStatus(inst.status, t)}
                      </Badge>
                      {inst.status === "running" && (
                        <span className="relative flex h-2 w-2">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75" />
                          <span className="relative inline-flex rounded-full h-2 w-2 bg-blue-500" />
                        </span>
                      )}
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
                      <span className="text-xs font-mono text-muted-foreground">
                        {inst.id.slice(0, 8)}
                      </span>
                      {/* 显示频道信息（v2 优先，v1 兼容） */}
                      {inst.notify_channels && inst.notify_channels.length > 0 ? (
                        // v2 格式：多频道
                        <span className="text-xs text-muted-foreground truncate max-w-[300px] flex items-center gap-1">
                          {inst.notify_channels.map((target, idx) => (
                            <span key={idx} className="inline-flex items-center">
                              {t(`channels.name.${target.channel}`, target.channel)}
                              {target.chat_id ? `:${target.chat_id.slice(0, 8)}...` : ""}
                              {idx < inst.notify_channels!.length - 1 ? ", " : ""}
                            </span>
                          ))}
                        </span>
                      ) : inst.channel ? (
                        // v1 格式：单频道
                        <span className="text-xs text-muted-foreground truncate max-w-[200px]">
                          {t(`channels.name.${inst.channel}`, inst.channel)}{inst.chat_id ? `:${inst.chat_id}` : ""}
                        </span>
                      ) : null}
                    </button>
                    <div className="flex items-center gap-1 shrink-0 ml-2">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7"
                        onClick={() => void navigate({
                          to: "/workflows/instance",
                          search: { workflow: workflowName!, instance: inst.id },
                        })}
                      >
                        <IconExternalLink className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-destructive hover:text-destructive hover:bg-destructive/10 h-7 w-7"
                        onClick={() => setDeleteInstanceId(inst.id)}
                      >
                        <IconTrash className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                  {expandedInstanceId === inst.id && (
                    <div className="border-t px-4 py-3 space-y-3">
                      {!expandedInstanceDetail ? (
                        // 加载中
                        <div className="py-4 text-center text-sm text-muted-foreground">
                          {t("common.loading", "Loading...")}
                        </div>
                      ) : (
                        <>
                          {/* 步骤进度 */}
                          {Object.entries(expandedInstanceDetail.step_states).length > 0 && (
                            <div>
                              <p className="text-xs font-medium mb-2">
                                {t("pages.workflows.step_progress", "Step Progress")}
                              </p>
                              <div className="space-y-1">
                                {workflowSteps.length > 0 ? (
                                  // 有工作流定义，使用树形结构显示
                                  <CompactStepTree steps={workflowSteps} stepStates={expandedInstanceDetail.step_states} />
                                ) : (
                                  // 没有工作流定义，平铺显示
                                  Object.entries(expandedInstanceDetail.step_states)
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
                                      <span>{state.name || `#${stepID}`}</span>
                                      {state.error && (
                                        <span className="text-destructive ml-2">{state.error}</span>
                                      )}
                                    </div>
                                  ))
                                )}
                              </div>
                            </div>
                          )}
                          
                          {/* 执行日志 */}
                          {expandedInstanceDetail.logs && expandedInstanceDetail.logs.length > 0 && (
                            <div>
                              <p className="text-xs font-medium mb-2">
                                {t("pages.workflows.execution_logs", "Execution Logs")}
                              </p>
                              <div className="max-h-40 overflow-auto rounded bg-muted/30 p-2 font-mono text-xs space-y-0.5">
                                {expandedInstanceDetail.logs.map((log, i) => (
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
                                      <span className="text-blue-600 shrink-0">[{expandedInstanceDetail.step_states?.[log.step_id]?.name || `#${log.step_id}`}]</span>
                                    )}
                                    <span>{log.message}</span>
                                  </div>
                                ))}
                              </div>
                            </div>
                          )}
                          
                          {/* 错误信息 */}
                          {expandedInstanceDetail.error && (
                            <p className="text-xs text-destructive">{expandedInstanceDetail.error}</p>
                          )}
                        </>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        <ConfirmDialog
          title={t("pages.workflows.delete_instance_title", "Delete Execution Record")}
          description={t("pages.workflows.delete_instance_confirm", "Are you sure you want to delete this execution record?")}
          open={!!deleteInstanceId}
          onOpenChange={(open) => !open && setDeleteInstanceId(null)}
          onConfirm={async () => {
            if (deleteInstanceId && workflowName) {
              try {
                await deleteWorkflowInstance(workflowName, deleteInstanceId)
                setInstances((prev) => prev.filter(i => i.id !== deleteInstanceId))
              } catch {
                // ignore
              }
            }
            setDeleteInstanceId(null)
          }}
          confirmVariant="destructive"
          confirmLabel={t("common.delete", "Delete")}
        />
      </DialogContent>
    </Dialog>
  )
}

function WorkflowCard({
  workflow,
  nextRun,
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
  nextRun: string | null
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
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <Badge variant="outline">{triggerLabel}</Badge>
          <Badge variant="secondary">
            {t("pages.workflows.step_count", "{{count}} steps", { count: workflow.step_count })}
          </Badge>
          {workflow.trigger_type !== "manual" && (
            <span className="text-muted-foreground text-xs ml-1">{triggerDesc}</span>
          )}
          {nextRun && (
            <span className="flex items-center gap-1 text-xs text-blue-600">
              <IconClock className="h-3 w-3" />
              → {nextRun}
            </span>
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

/** 紧凑树形步骤进度（用于实例弹窗） */
function CompactStepTree({
  steps,
  stepStates,
}: {
  steps: Step[]
  stepStates: Record<string, { name?: string; status: string; started_at?: string; finished_at?: string; error?: string; attempts: number }>
}) {
  return (
    <div className="space-y-1">
      {steps.map((step) => (
        <CompactStepNode key={step.id} step={step} stepStates={stepStates} depth={0} />
      ))}
    </div>
  )
}

/** 紧凑步骤节点 */
function CompactStepNode({
  step,
  stepStates,
  depth,
}: {
  step: Step
  stepStates: Record<string, { name?: string; status: string; started_at?: string; finished_at?: string; error?: string; attempts: number }>
  depth: number
}) {
  const state = stepStates[step.id]
  if (!state) return null

  const indent = depth * 16

  return (
    <div>
      <div className="flex items-center gap-2 text-xs" style={{ paddingLeft: indent }}>
        <span className={getInstanceStatusColor(state.status)}>
          {state.status === "completed" ? "✓" : state.status === "failed" ? "✗" : state.status === "running" ? "⏳" : "○"}
        </span>
        <span>{state.name || `#${step.id}`}</span>
        {(step.action === "parallel" || step.action === "if") && (
          <span className="text-muted-foreground">{step.action === "parallel" ? "∥" : "↔"}</span>
        )}
        {step.action === "tool_call" && step.tool && (
          <span className="text-muted-foreground">{step.tool}</span>
        )}
        {state.error && (
          <span className="text-destructive ml-1">{state.error}</span>
        )}
      </div>
      {/* 递归渲染子步骤 */}
      {step.action === "if" && step.if_true && step.if_true.length > 0 && (
        <div>
          <div className="text-muted-foreground text-xs" style={{ paddingLeft: indent + 16 }}>✓ true</div>
          {step.if_true.map((sub) => (
            <CompactStepNode key={sub.id} step={sub} stepStates={stepStates} depth={depth + 1} />
          ))}
        </div>
      )}
      {step.action === "if" && step.if_false && step.if_false.length > 0 && (
        <div>
          <div className="text-muted-foreground text-xs" style={{ paddingLeft: indent + 16 }}>✗ false</div>
          {step.if_false.map((sub) => (
            <CompactStepNode key={sub.id} step={sub} stepStates={stepStates} depth={depth + 1} />
          ))}
        </div>
      )}
      {step.action === "parallel" && step.parallel && step.parallel.map((sub) => (
        <CompactStepNode key={sub.id} step={sub} stepStates={stepStates} depth={depth + 1} />
      ))}
    </div>
  )
}
