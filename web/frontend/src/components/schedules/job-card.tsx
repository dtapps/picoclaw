import {
  IconCalendar,
  IconCheck,
  IconEdit,
  IconHistory,
  IconMessage,
  IconPlayerPlay,
  IconPlayerStop,
  IconPlayerTrackNext,
  IconTrash,
  IconX,
} from "@tabler/icons-react"
import { useTranslation } from "react-i18next"

import { type CronJob, formatSchedule } from "@/api/cron"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"

interface JobCardProps {
  job: CronJob
  onToggle: (id: string, enabled: boolean) => void
  onEdit: (job: CronJob) => void
  onDelete: (id: string) => void
  isToggling?: boolean
}

// 格式化时间显示为相对时间
function formatRelativeTime(timestampMs: number): string {
  const date = new Date(timestampMs)
  const now = new Date()
  const diff = now.getTime() - date.getTime()

  // 未来时间
  if (diff < 0) {
    const futureDiff = -diff
    if (futureDiff < 60000) return "即将"
    if (futureDiff < 3600000) return `${Math.floor(futureDiff / 60000)}分钟后`
    if (futureDiff < 86400000)
      return `${Math.floor(futureDiff / 3600000)}小时后`
    if (futureDiff < 604800000)
      return `${Math.floor(futureDiff / 86400000)}天后`
    return date.toLocaleDateString()
  }

  // 过去时间
  if (diff < 60000) return "刚刚"
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}天前`
  return date.toLocaleDateString()
}

// 格式化完整时间
function formatFullTime(timestampMs: number): string {
  return new Date(timestampMs).toLocaleString()
}

export function JobCard({
  job,
  onToggle,
  onEdit,
  onDelete,
  isToggling,
}: JobCardProps) {
  const { t } = useTranslation()

  const getStatusBadge = () => {
    if (!job.enabled) {
      return (
        <Badge variant="secondary" className="text-xs">
          {t("pages.schedules.status_disabled", "Disabled")}
        </Badge>
      )
    }
    if (job.state.lastStatus === "error") {
      return (
        <Badge variant="destructive" className="text-xs">
          {t("pages.schedules.status_error", "Error")}
        </Badge>
      )
    }
    return (
      <Badge
        variant="default"
        className="bg-green-600 text-xs hover:bg-green-700"
      >
        {t("pages.schedules.status_active", "Active")}
      </Badge>
    )
  }

  const getTypeBadge = () => {
    const typeMap: Record<string, { label: string; color: string }> = {
      at: {
        label: t("pages.schedules.type_at", "One-time"),
        color: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
      },
      every: {
        label: t("pages.schedules.type_every", "Interval"),
        color:
          "bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300",
      },
      cron: {
        label: t("pages.schedules.type_cron", "Cron"),
        color:
          "bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300",
      },
    }
    const typeInfo = typeMap[job.schedule.kind] || {
      label: job.schedule.kind,
      color: "bg-gray-100 text-gray-700",
    }
    return (
      <span
        className={`rounded-full px-2 py-0.5 text-xs font-medium ${typeInfo.color}`}
      >
        {typeInfo.label}
      </span>
    )
  }

  // 截断过长的名称，显示前30个字符
  const displayName =
    job.name.length > 30 ? job.name.slice(0, 30) + "..." : job.name

  // 判断是否有执行记录
  const hasLastRun = !!job.state.lastRunAtMs
  const hasNextRun = !!job.state.nextRunAtMs && job.enabled

  return (
    <Card className="hover:border-primary/20 transition-all hover:shadow-md">
      <CardHeader className="pt-4 pb-3">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            {/* 第一行：名称 */}
            <h3
              className="mb-2 text-sm leading-tight font-semibold"
              title={job.name}
            >
              {displayName}
            </h3>
            {/* 第二行：状态标签 */}
            <div className="flex flex-wrap items-center gap-2">
              {getStatusBadge()}
              {getTypeBadge()}
            </div>
          </div>
          {/* 操作按钮组 */}
          <div className="flex shrink-0 items-center gap-0.5">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onToggle(job.id, !job.enabled)}
              disabled={isToggling}
              title={
                job.enabled
                  ? t("common.disable", "Disable")
                  : t("common.enable", "Enable")
              }
            >
              {job.enabled ? (
                <IconPlayerStop className="h-3.5 w-3.5" />
              ) : (
                <IconPlayerPlay className="h-3.5 w-3.5" />
              )}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onEdit(job)}
              title={t("common.edit", "Edit")}
            >
              <IconEdit className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="text-destructive hover:text-destructive hover:bg-destructive/10 h-7 w-7"
              onClick={() => onDelete(job.id)}
              title={t("common.delete", "Delete")}
            >
              <IconTrash className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="pt-0 pb-4">
        <div className="space-y-2 text-xs">
          {/* 调度信息 - 突出显示 */}
          <div className="text-muted-foreground bg-muted/50 flex items-center gap-2 rounded px-2 py-1.5">
            <IconCalendar className="text-primary/70 h-3.5 w-3.5 flex-shrink-0" />
            <span className="font-medium">{formatSchedule(job.schedule)}</span>
          </div>

          {/* 执行时间信息区域 */}
          {(hasLastRun || hasNextRun) && (
            <div className="grid grid-cols-2 gap-2">
              {/* 上次执行 */}
              {hasLastRun && (
                <div className="text-muted-foreground flex items-center gap-1.5 rounded bg-red-50 px-2 py-1.5 dark:bg-red-950/20">
                  <IconHistory className="h-3.5 w-3.5 flex-shrink-0 text-red-500" />
                  <div className="flex min-w-0 flex-col">
                    <span className="text-muted-foreground/70 text-[10px] tracking-wider uppercase">
                      {t("pages.schedules.last_run", "Last")}
                    </span>
                    <span
                      className="truncate font-medium"
                      title={formatFullTime(job.state.lastRunAtMs!)}
                    >
                      {formatRelativeTime(job.state.lastRunAtMs!)}
                    </span>
                  </div>
                  {job.state.lastStatus && (
                    <span className="ml-auto">
                      {job.state.lastStatus === "ok" ? (
                        <IconCheck className="h-3.5 w-3.5 text-green-600" />
                      ) : (
                        <IconX className="h-3.5 w-3.5 text-red-600" />
                      )}
                    </span>
                  )}
                </div>
              )}

              {/* 下次执行 */}
              {hasNextRun && (
                <div className="text-muted-foreground flex items-center gap-1.5 rounded bg-green-50 px-2 py-1.5 dark:bg-green-950/20">
                  <IconPlayerTrackNext className="h-3.5 w-3.5 flex-shrink-0 text-green-600" />
                  <div className="flex min-w-0 flex-col">
                    <span className="text-muted-foreground/70 text-[10px] tracking-wider uppercase">
                      {t("pages.schedules.next_run", "Next")}
                    </span>
                    <span
                      className="truncate font-medium"
                      title={formatFullTime(job.state.nextRunAtMs!)}
                    >
                      {formatRelativeTime(job.state.nextRunAtMs!)}
                    </span>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* 消息内容 - 只在名称和消息不同时显示 */}
          {job.payload.message && job.payload.message !== job.name && (
            <div className="text-muted-foreground flex items-start gap-2">
              <IconMessage className="mt-0.5 h-3.5 w-3.5 flex-shrink-0 text-blue-500" />
              <span className="line-clamp-2">{job.payload.message}</span>
            </div>
          )}

          {/* 频道信息 */}
          {job.payload.channel && (
            <div className="text-muted-foreground flex items-center gap-1.5">
              <span className="text-muted-foreground/70 text-[10px] font-medium tracking-wider uppercase">
                {t("pages.schedules.channel", "Channel")}
              </span>
              <span className="bg-secondary rounded px-1.5 py-0.5 text-[10px]">
                {job.payload.channel}
              </span>
              {job.payload.to && (
                <>
                  <span className="text-muted-foreground/50">→</span>
                  <span
                    className="max-w-[100px] truncate"
                    title={job.payload.to}
                  >
                    {job.payload.to.slice(0, 8)}...
                  </span>
                </>
              )}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
