import { useTranslation } from "react-i18next"

import type { WorkflowInstance } from "@/api/workflow"
import {
  formatInstanceStatus,
  getInstanceStatusColor,
} from "@/api/workflow"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

interface WorkflowDetailProps {
  instance: WorkflowInstance | null
}

export function WorkflowDetail({ instance }: WorkflowDetailProps) {
  const { t } = useTranslation()

  if (!instance) {
    return (
      <div className="text-muted-foreground flex items-center justify-center py-10">
        {t("pages.workflows.no_instance_selected", "Select an instance to view details")}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Badge
          variant={instance.status === "completed" ? "default" : "outline"}
          className={getInstanceStatusColor(instance.status)}
        >
          {formatInstanceStatus(instance.status, t)}
        </Badge>
        <span className="text-muted-foreground text-sm">
          {t("pages.workflows.triggered_by", "Triggered by")}:           {instance.trigger_type === "cron"
            ? t("pages.workflows.trigger_cron", "Cron")
            : instance.trigger_type === "event"
              ? t("pages.workflows.trigger_event", "Event")
              : t("pages.workflows.trigger_manual", "Manual")}
        </span>
      </div>

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
                <div className="flex-1">
                  <span className="text-sm font-medium">{stepID}</span>
                  {state.error && (
                    <p className="text-destructive text-xs mt-0.5">
                      {state.error}
                    </p>
                  )}
                  {/* 步骤输出 */}
                  {instance.step_outputs?.[stepID] && (
                    <div className="mt-1 rounded bg-muted/50 px-2 py-1 text-xs font-mono text-muted-foreground max-h-20 overflow-auto">
                      {Object.entries(instance.step_outputs[stepID]).map(([key, val]) => (
                        <div key={key}>
                          <span className="text-foreground/70">{key}:</span>{" "}
                          {typeof val === "string"
                            ? val.length > 120 ? val.slice(0, 120) + "..." : val
                            : JSON.stringify(val).slice(0, 120)}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
                <Badge variant="outline" className={getInstanceStatusColor(state.status)}>
                  {formatInstanceStatus(state.status)}
                </Badge>
              </div>
            ))}
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

      {/* 执行日志 */}
      {instance.logs && instance.logs.length > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm">
              {t("pages.workflows.execution_logs", "Execution Logs")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="max-h-48 overflow-auto rounded bg-muted/30 p-2 font-mono text-xs space-y-0.5">
              {instance.logs.map((log, i) => (
                <div key={i} className={`flex gap-2 ${log.level === "error" ? "text-destructive" : log.level === "warn" ? "text-amber-600" : "text-foreground/80"}`}>
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
          </CardContent>
        </Card>
      )}

      <div className="text-muted-foreground text-xs">
        <p>{t("pages.workflows.started_at", "Started")}: {new Date(instance.started_at).toLocaleString()}</p>
        {instance.finished_at && (
          <p>{t("pages.workflows.finished_at", "Finished")}: {new Date(instance.finished_at).toLocaleString()}</p>
        )}
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
