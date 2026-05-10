import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "@tanstack/react-router"

import type { WorkflowInstance, WorkflowStepEvent } from "@/api/workflow"
import {
  createInstanceStream,
  formatInstanceStatus,
  getInstanceStatusColor,
  getWorkflowInstance,
} from "@/api/workflow"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { IconArrowLeft } from "@tabler/icons-react"

interface WorkflowDetailProps {
  workflowName: string
  instanceId: string
}

export function WorkflowDetail({ workflowName, instanceId }: WorkflowDetailProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [instance, setInstance] = useState<WorkflowInstance | null>(null)
  const [connected, setConnected] = useState(false)
  const esRef = useRef<EventSource | null>(null)
  const retryCountRef = useRef(0)
  const MAX_RETRIES = 5

  const closeSSE = useCallback(() => {
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }
    setConnected(false)
  }, [])

  const connectSSE = useCallback(() => {
    closeSSE()
    const es = createInstanceStream(workflowName, instanceId)
    esRef.current = es

    es.onopen = () => {
      retryCountRef.current = 0
      setConnected(true)
    }

    es.addEventListener("snapshot", (e) => {
      try {
        setInstance(JSON.parse(e.data) as WorkflowInstance)
      } catch { /* ignore */ }
    })

    es.addEventListener("step_update", (e) => {
      try {
        const evt = JSON.parse(e.data) as WorkflowStepEvent
        const payload = evt.payload
        if (!payload.step_id) return
        const stepID = payload.step_id
        setInstance((prev) => {
          if (!prev) return prev
          const stepStates = { ...prev.step_states }
          stepStates[stepID] = {
            ...stepStates[stepID],
            status: payload.status || stepStates[stepID]?.status || "running",
          }
          return { ...prev, step_states: stepStates }
        })
      } catch { /* ignore */ }
    })

    es.addEventListener("instance_complete", (e) => {
      try {
        const evt = JSON.parse(e.data) as WorkflowStepEvent
        const payload = evt.payload
        setInstance((prev) => {
          if (!prev) return prev
          return {
            ...prev,
            status: payload.status || prev.status,
            error: payload.error || prev.error,
            finished_at: evt.time,
          }
        })
      } catch { /* ignore */ }
      closeSSE()
    })

    es.onerror = () => {
      setConnected(false)
      retryCountRef.current++
      if (retryCountRef.current >= MAX_RETRIES) {
        es.close()
        esRef.current = null
      }
    }
  }, [workflowName, instanceId, closeSSE])

  // 加载初始数据，仅运行中实例建立 SSE
  useEffect(() => {
    if (!workflowName || !instanceId) return

    let cancelled = false
    getWorkflowInstance(workflowName, instanceId)
      .then((inst) => {
        if (cancelled) return
        setInstance(inst)
        if (inst.status === "running") {
          retryCountRef.current = 0
          connectSSE()
        }
      })
      .catch(() => {
        if (!cancelled) setInstance(null)
      })

    return () => {
      cancelled = true
      closeSSE()
    }
  }, [workflowName, instanceId, connectSSE, closeSSE])

  if (!instance) {
    return (
      <div className="bg-background flex h-full flex-col">
        <PageHeader title={t("pages.workflows.instance_detail", "Instance Detail")} />
        <div className="text-muted-foreground flex flex-1 items-center justify-center">
          {t("pages.workflows.no_instance_selected", "No instance selected")}
        </div>
      </div>
    )
  }

  const duration = () => {
    if (!instance.finished_at) return null
    const ms = new Date(instance.finished_at).getTime() - new Date(instance.started_at).getTime()
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${(ms / 60000).toFixed(1)}m`
  }

  const totalSteps = Object.keys(instance.step_states).length
  const completedSteps = Object.values(instance.step_states).filter(s => s.status === "completed" || s.status === "skipped").length

  return (
    <div className="bg-background flex h-full flex-col">
      <PageHeader title={t("pages.workflows.instance_detail", "Instance Detail")}>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void navigate({ to: "/workflows" })}
        >
          <IconArrowLeft className="mr-1 h-4 w-4" />
          {t("common.back", "Back")}
        </Button>
      </PageHeader>

      <div className="flex-1 overflow-auto px-6 py-4">
        <div className="mx-auto w-full max-w-4xl space-y-4">

          {/* 状态概览 */}
          <Card>
            <CardContent className="pt-4 space-y-3">
              <div className="flex items-center gap-3 flex-wrap">
                <Badge
                  variant={instance.status === "completed" ? "default" : "outline"}
                  className={getInstanceStatusColor(instance.status)}
                >
                  {formatInstanceStatus(instance.status, t)}
                </Badge>
                {instance.status === "running" && connected && (
                  <span className="relative flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75" />
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-blue-500" />
                  </span>
                )}
                <span className="text-muted-foreground text-sm">
                  {t("pages.workflows.triggered_by", "Triggered by")}:
                  {" "}{instance.trigger_type === "cron"
                    ? t("pages.workflows.trigger_cron", "Cron")
                    : instance.trigger_type === "event"
                      ? t("pages.workflows.trigger_event", "Event")
                      : t("pages.workflows.trigger_manual", "Manual")}
                </span>
              </div>
              <div className="grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-2 lg:grid-cols-3 text-sm">
                <div className="min-w-0">
                  <p className="text-muted-foreground text-xs">{t("pages.workflows.instance_id", "Instance ID")}</p>
                  <p className="font-mono text-xs truncate">{instance.id.slice(0, 16)}</p>
                </div>
                <div className="min-w-0">
                  <p className="text-muted-foreground text-xs">{t("pages.workflows.started_at", "Started")}</p>
                  <p className="truncate">{new Date(instance.started_at).toLocaleString()}</p>
                </div>
                {instance.finished_at && (
                  <div className="min-w-0">
                    <p className="text-muted-foreground text-xs">{t("pages.workflows.finished_at", "Finished")}</p>
                    <p className="truncate">{new Date(instance.finished_at).toLocaleString()}</p>
                  </div>
                )}
                {duration() && (
                  <div className="min-w-0">
                    <p className="text-muted-foreground text-xs">{t("pages.workflows.duration", "Duration")}</p>
                    <p>{duration()}</p>
                  </div>
                )}
                <div className="min-w-0">
                  <p className="text-muted-foreground text-xs">{t("pages.workflows.step_progress", "Step Progress")}</p>
                  <p>{completedSteps}/{totalSteps}</p>
                </div>
                {instance.channel && (
                  <div className="min-w-0">
                    <p className="text-muted-foreground text-xs">{t("pages.workflows.notify_channel", "Channel")}</p>
                    <p className="truncate">{t(`channels.name.${instance.channel}`, instance.channel)}{instance.chat_id ? `:${instance.chat_id}` : ""}</p>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>

          {/* 步骤进度 */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">
                {t("pages.workflows.step_progress", "Step Progress")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                {Object.entries(instance.step_states)
                  .sort(([, a], [, b]) => {
                    if (a.started_at && b.started_at) return a.started_at.localeCompare(b.started_at)
                    if (a.started_at) return -1
                    if (b.started_at) return 1
                    return 0
                  })
                  .map(([stepID, state]) => (
                  <div key={stepID} className="flex items-center gap-3">
                    <StepStatusIcon status={state.status} />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium">{state.name || `#${stepID}`}</span>
                        {state.name && (
                          <span className="text-muted-foreground text-xs font-mono">#{stepID}</span>
                        )}
                      </div>
                      {state.error && (
                        <p className="text-destructive text-xs mt-0.5">
                          {state.error}
                        </p>
                      )}
                      {instance.step_outputs?.[stepID] && (
                        <div className="mt-1 rounded bg-muted/50 px-2 py-1 text-xs font-mono text-muted-foreground max-h-60 overflow-auto">
                          {Object.entries(instance.step_outputs[stepID]).map(([key, val]) => (
                            <div key={key} className="whitespace-pre-wrap break-all">
                              <span className="text-foreground/70">{key}:</span>{" "}
                              {typeof val === "string" ? val : JSON.stringify(val, null, 2)}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                    <div className="shrink-0 text-right">
                      <Badge variant="outline" className={getInstanceStatusColor(state.status)}>
                        {formatInstanceStatus(state.status, t)}
                      </Badge>
                      {state.attempts > 1 && (
                        <p className="text-muted-foreground text-xs mt-0.5">
                          {t("pages.workflows.attempts", "{{count}} attempts", { count: state.attempts })}
                        </p>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* 错误信息 */}
          {instance.error && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-destructive text-sm">
                  {t("pages.workflows.error", "Error")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-destructive text-sm">{instance.error}</p>
              </CardContent>
            </Card>
          )}

          {/* 执行日志 */}
          {instance.logs && instance.logs.length > 0 && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm">
                  {t("pages.workflows.execution_logs", "Execution Logs")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="max-h-64 overflow-auto rounded bg-muted/30 p-2 font-mono text-xs space-y-0.5">
                  {instance.logs.map((log, i) => (
                    <div key={i} className={`flex gap-2 ${log.level === "error" ? "text-destructive" : log.level === "warn" ? "text-amber-600" : "text-foreground/80"}`}>
                      <span className="text-muted-foreground shrink-0">
                        {new Date(log.timestamp).toLocaleTimeString()}
                      </span>
                      {log.step_id && (
                        <span className="text-blue-600 shrink-0">
                          [{instance.step_states?.[log.step_id]?.name || `#${log.step_id}`}]
                        </span>
                      )}
                      <span>{log.message}</span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

        </div>
      </div>
    </div>
  )
}

function StepStatusIcon({ status }: { status: string }) {
  switch (status) {
    case "completed":
      return <span className="text-green-600">✓</span>
    case "running":
      return <span className="text-blue-600 animate-pulse">⏳</span>
    case "failed":
      return <span className="text-red-600">✗</span>
    case "skipped":
      return <span className="text-muted-foreground">∅</span>
    default:
      return <span className="text-muted-foreground">○</span>
  }
}
