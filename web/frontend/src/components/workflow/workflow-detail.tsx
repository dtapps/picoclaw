import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"
import { useNavigate } from "@tanstack/react-router"

import type { Step, WorkflowInstance, WorkflowStepEvent } from "@/api/workflow"
import {
  createInstanceStream,
  formatInstanceStatus,
  getInstanceStatusColor,
  getWorkflow,
  getWorkflowInstance,
} from "@/api/workflow"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { IconArrowLeft, IconBrain, IconTool, IconGitBranch, IconStack2 } from "@tabler/icons-react"

interface WorkflowDetailProps {
  workflowName: string
  instanceId: string
}

export function WorkflowDetail({ workflowName, instanceId }: WorkflowDetailProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [instance, setInstance] = useState<WorkflowInstance | null>(null)
  const [steps, setSteps] = useState<Step[]>([])
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

    es.addEventListener("instance_complete", () => {
      if (workflowName && instanceId) {
        getWorkflowInstance(workflowName, instanceId)
          .then((inst) => { setInstance(inst) })
          .catch(() => {})
      }
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

  useEffect(() => {
    if (!workflowName) return
    getWorkflow(workflowName)
      .then((wf) => setSteps(wf.steps))
      .catch(() => setSteps([]))
  }, [workflowName])

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

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">
                {t("pages.workflows.step_progress", "Step Progress")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-2">
                {steps.length > 0 ? (
                  <StepTree steps={steps} stepStates={instance.step_states} stepOutputs={instance.step_outputs} t={t} />
                ) : (
                  <FlatStepList instance={instance} t={t} />
                )}
              </div>
            </CardContent>
          </Card>

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

function FlatStepList({ instance, t }: { instance: WorkflowInstance; t: TFunction }) {
  return (
    <>
      {Object.entries(instance.step_states)
        .sort(([, a], [, b]) => {
          if (a.started_at && b.started_at) return a.started_at.localeCompare(b.started_at)
          if (a.started_at) return -1
          if (b.started_at) return 1
          return 0
        })
        .map(([stepID, state]) => (
        <StepCard
          key={stepID}
          stepId={stepID}
          stepName={state.name}
          state={state}
          output={instance.step_outputs?.[stepID]}
          t={t}
        />
      ))}
    </>
  )
}

function StepCard({
  stepId,
  stepName,
  state,
  output,
  t,
}: {
  stepId: string
  stepName?: string
  state: { status: string; started_at?: string; finished_at?: string; error?: string; attempts: number }
  output?: Record<string, unknown>
  t: TFunction
}) {
  const stepDuration = () => {
    if (!state.started_at || !state.finished_at) return null
    const ms = new Date(state.finished_at).getTime() - new Date(state.started_at).getTime()
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${(ms / 60000).toFixed(1)}m`
  }

  return (
    <div className="border rounded-lg p-3 bg-card">
      <div className="flex items-start gap-3">
        <div className="mt-0.5">
          <StepStatusIcon status={state.status} size="md" />
        </div>
        <div className="flex-1 min-w-0 space-y-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-medium text-sm">{stepName || `#${stepId}`}</span>
            {stepName && (
              <span className="text-muted-foreground text-xs font-mono">#{stepId}</span>
            )}
            <Badge variant="outline" className={`text-xs ${getInstanceStatusColor(state.status)}`}>
              {formatInstanceStatus(state.status, t)}
            </Badge>
            {stepDuration() && (
              <span className="text-muted-foreground text-xs">{stepDuration()}</span>
            )}
            {state.attempts > 1 && (
              <span className="text-muted-foreground text-xs">×{state.attempts}</span>
            )}
          </div>
          {state.error && (
            <p className="text-destructive text-xs bg-destructive/10 rounded px-2 py-1">{state.error}</p>
          )}
          {output && Object.keys(output).length > 0 && (
            <div className="mt-2">
              <p className="text-muted-foreground text-xs mb-1">{t("pages.workflows.output", "Output")}:</p>
              <div className="rounded bg-muted/30 p-2 font-mono text-xs max-h-40 overflow-auto">
                {Object.entries(output).map(([key, val]) => (
                  <div key={key} className="whitespace-pre-wrap break-all">
                    <span className="text-blue-600">{key}</span>:{" "}
                    <span className="text-foreground/80">{typeof val === "string" ? val : JSON.stringify(val, null, 2)}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function StepStatusIcon({ status, size = "sm" }: { status: string; size?: "sm" | "md" }) {
  const sizeClass = size === "md" ? "text-lg" : "text-base"
  switch (status) {
    case "completed":
      return <span className={`text-green-600 ${sizeClass}`}>✓</span>
    case "running":
      return <span className={`text-blue-600 animate-pulse ${sizeClass}`}>⏳</span>
    case "failed":
      return <span className={`text-red-600 ${sizeClass}`}>✗</span>
    case "skipped":
      return <span className={`text-muted-foreground ${sizeClass}`}>∅</span>
    default:
      return <span className={`text-muted-foreground ${sizeClass}`}>○</span>
  }
}

function StepActionIcon({ action }: { action: string }) {
  const iconClass = "h-4 w-4"
  switch (action) {
    case "agent_prompt":
      return <IconBrain className={`${iconClass} text-purple-500`} />
    case "tool_call":
      return <IconTool className={`${iconClass} text-orange-500`} />
    case "parallel":
      return <IconStack2 className={`${iconClass} text-blue-500`} />
    case "if":
      return <IconGitBranch className={`${iconClass} text-green-500`} />
    default:
      return null
  }
}

function StepTree({
  steps,
  stepStates,
  stepOutputs,
  t,
}: {
  steps: Step[]
  stepStates: Record<string, { name?: string; status: string; started_at?: string; finished_at?: string; error?: string; attempts: number }>
  stepOutputs: Record<string, Record<string, unknown>>
  t: TFunction
}) {
  return (
    <div className="space-y-2">
      {steps.map((step) => (
        <StepTreeNode key={step.id} step={step} stepStates={stepStates} stepOutputs={stepOutputs} t={t} depth={0} />
      ))}
    </div>
  )
}

function StepTreeNode({
  step,
  stepStates,
  stepOutputs,
  t,
  depth,
}: {
  step: Step
  stepStates: Record<string, { name?: string; status: string; started_at?: string; finished_at?: string; error?: string; attempts: number }>
  stepOutputs: Record<string, Record<string, unknown>>
  t: TFunction
  depth: number
}) {
  const state = stepStates[step.id]
  if (!state) return null

  const childSteps = step.action === "parallel"
    ? step.parallel || []
    : step.action === "if"
      ? [...(step.if_true || []), ...(step.if_false || [])]
      : []

  const stepDuration = () => {
    if (!state.started_at || !state.finished_at) return null
    const ms = new Date(state.finished_at).getTime() - new Date(state.started_at).getTime()
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${(ms / 60000).toFixed(1)}m`
  }

  const hasInput = step.prompt || (step.args && Object.keys(step.args).length > 0)
  const hasOutput = stepOutputs?.[step.id] && Object.keys(stepOutputs[step.id]).length > 0

  return (
    <div className={`${depth > 0 ? "ml-4 border-l-2 border-muted pl-3" : ""}`}>
      <div className={`border rounded-lg p-3 bg-card ${state.status === "running" ? "ring-2 ring-blue-500/30" : ""}`}>
        <div className="flex items-start gap-3">
          <div className="mt-0.5 flex items-center gap-1">
            <StepStatusIcon status={state.status} size="md" />
            <StepActionIcon action={step.action} />
          </div>
          <div className="flex-1 min-w-0 space-y-1">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="font-medium text-sm">{state.name || step.name || `#${step.id}`}</span>
              {(state.name || step.name) && (
                <span className="text-muted-foreground text-xs font-mono">#{step.id}</span>
              )}
              <Badge variant="outline" className={`text-xs ${getInstanceStatusColor(state.status)}`}>
                {formatInstanceStatus(state.status, t)}
              </Badge>
              {stepDuration() && (
                <span className="text-muted-foreground text-xs">{stepDuration()}</span>
              )}
              {state.attempts > 1 && (
                <span className="text-muted-foreground text-xs">×{state.attempts}</span>
              )}
            </div>

            {step.action === "tool_call" && step.tool && (
              <div className="flex items-center gap-2 text-xs">
                <span className="text-orange-600 font-medium">{step.tool}</span>
                {step.output_key && (
                  <span className="text-muted-foreground">→ {step.output_key}</span>
                )}
              </div>
            )}

            {step.action === "agent_prompt" && step.output_key && (
              <div className="text-xs text-muted-foreground">
                → {step.output_key}
              </div>
            )}

            {hasInput && (
              <details className="mt-1">
                <summary className="text-xs text-muted-foreground cursor-pointer hover:text-foreground">
                  {t("pages.workflows.input", "Input")}
                </summary>
                <div className="mt-1 rounded bg-muted/30 p-2 font-mono text-xs max-h-40 overflow-auto space-y-1">
                  {step.prompt && (
                    <div>
                      <span className="text-purple-600">prompt</span>:{" "}
                      <span className="text-foreground/80 whitespace-pre-wrap">{step.prompt}</span>
                    </div>
                  )}
                  {step.args && Object.keys(step.args).length > 0 && (
                    <div>
                      <span className="text-orange-600">args</span>:{" "}
                      <span className="text-foreground/80">{JSON.stringify(step.args, null, 2)}</span>
                    </div>
                  )}
                </div>
              </details>
            )}

            {state.error && (
              <p className="text-destructive text-xs bg-destructive/10 rounded px-2 py-1">{state.error}</p>
            )}

            {hasOutput && (
              <details className="mt-1" open>
                <summary className="text-xs text-muted-foreground cursor-pointer hover:text-foreground">
                  {t("pages.workflows.output", "Output")}
                </summary>
                <div className="mt-1 rounded bg-muted/30 p-2 font-mono text-xs max-h-40 overflow-auto">
                  {Object.entries(stepOutputs[step.id]).map(([key, val]) => (
                    <div key={key} className="whitespace-pre-wrap break-all">
                      <span className="text-blue-600">{key}</span>:{" "}
                      <span className="text-foreground/80">{typeof val === "string" ? val : JSON.stringify(val, null, 2)}</span>
                    </div>
                  ))}
                </div>
              </details>
            )}
          </div>
        </div>
      </div>

      {childSteps.length > 0 && (
        <div className="mt-2 space-y-2">
          {step.action === "if" && step.if_true && step.if_true.length > 0 && (
            <div>
              <div className="text-xs font-medium text-green-600 mb-1 flex items-center gap-1">
                <span>✓</span> true
              </div>
              {step.if_true.map((sub) => (
                <StepTreeNode key={sub.id} step={sub} stepStates={stepStates} stepOutputs={stepOutputs} t={t} depth={depth + 1} />
              ))}
            </div>
          )}
          {step.action === "if" && step.if_false && step.if_false.length > 0 && (
            <div>
              <div className="text-xs font-medium text-red-600 mb-1 flex items-center gap-1">
                <span>✗</span> false
              </div>
              {step.if_false.map((sub) => (
                <StepTreeNode key={sub.id} step={sub} stepStates={stepStates} stepOutputs={stepOutputs} t={t} depth={depth + 1} />
              ))}
            </div>
          )}
          {step.action === "parallel" && step.parallel && step.parallel.map((sub) => (
            <StepTreeNode key={sub.id} step={sub} stepStates={stepStates} stepOutputs={stepOutputs} t={t} depth={depth + 1} />
          ))}
        </div>
      )}
    </div>
  )
}
