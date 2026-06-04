import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"

import type { CreateJobRequest, CronJob } from "@/api/cron"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"

interface JobFormDialogProps {
  job?: CronJob | null
  open?: boolean
  onOpenChange?: (open: boolean) => void
  onSubmit: (data: CreateJobRequest) => void
  isSubmitting?: boolean
  // 用于创建按钮模式
  trigger?: React.ReactNode
}

export function JobFormDialog({
  job,
  open: controlledOpen,
  onOpenChange,
  onSubmit,
  isSubmitting,
  trigger,
}: JobFormDialogProps) {
  const { t } = useTranslation()
  const [internalOpen, setInternalOpen] = useState(false)
  const isControlled = controlledOpen !== undefined
  const open = isControlled ? controlledOpen : internalOpen
  const setOpen = isControlled ? onOpenChange! : setInternalOpen
  const isEditing = !!job

  const [formData, setFormData] = useState<CreateJobRequest>({
    name: "",
    message: "",
    command: "",
    channel: "",
    to: "",
    schedule: {
      kind: "every",
      everyMs: 3600000,
    },
  })

  // 当 job 变化时更新表单数据
  useEffect(() => {
    if (job) {
      setFormData({
        name: job.name || "",
        message: job.payload.message || "",
        command: job.payload.command || "",
        channel: job.payload.channel || "",
        to: job.payload.to || "",
        schedule: job.schedule || { kind: "every", everyMs: 3600000 },
      })
    } else {
      setFormData({
        name: "",
        message: "",
        command: "",
        channel: "",
        to: "",
        schedule: { kind: "every", everyMs: 3600000 },
      })
    }
  }, [job, open])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit(formData)
    if (!isEditing) {
      // 创建成功后重置表单
      setFormData({
        name: "",
        message: "",
        command: "",
        channel: "",
        to: "",
        schedule: { kind: "every", everyMs: 3600000 },
      })
    }
    setOpen(false)
  }

  const updateSchedule = (kind: "at" | "every" | "cron") => {
    setFormData((prev) => ({
      ...prev,
      schedule: { kind },
    }))
  }

  const handleClose = () => {
    setOpen(false)
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {trigger && <DialogTrigger asChild>{trigger}</DialogTrigger>}
      <DialogContent className="sm:max-w-[500px]">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>
              {isEditing
                ? t("pages.schedules.edit_job", "Edit Job")
                : t("pages.schedules.create_job", "Create Job")}
            </DialogTitle>
            <DialogDescription>
              {t(
                "pages.schedules.form_description",
                "Configure a scheduled task to run at specific times or intervals.",
              )}
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="name">
                {t("pages.schedules.name", "Name")} *
              </Label>
              <Input
                id="name"
                value={formData.name}
                onChange={(e) =>
                  setFormData((prev) => ({ ...prev, name: e.target.value }))
                }
                placeholder={t(
                  "pages.schedules.name_placeholder",
                  "e.g., Daily Report",
                )}
                required
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="schedule-type">
                {t("pages.schedules.schedule_type", "Schedule Type")} *
              </Label>
              <Select
                value={formData.schedule.kind}
                onValueChange={(v) =>
                  updateSchedule(v as "at" | "every" | "cron")
                }
              >
                <SelectTrigger id="schedule-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="at">
                    {t("pages.schedules.type_at", "One-time")}
                  </SelectItem>
                  <SelectItem value="every">
                    {t("pages.schedules.type_every", "Interval")}
                  </SelectItem>
                  <SelectItem value="cron">
                    {t("pages.schedules.type_cron", "Cron Expression")}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            {formData.schedule.kind === "at" && (
              <div className="grid gap-2">
                <Label htmlFor="at-time">
                  {t("pages.schedules.at_time", "Run At")} *
                </Label>
                <Input
                  id="at-time"
                  type="datetime-local"
                  value={
                    formData.schedule.atMs
                      ? new Date(formData.schedule.atMs)
                          .toISOString()
                          .slice(0, 16)
                      : ""
                  }
                  onChange={(e) => {
                    const date = new Date(e.target.value)
                    setFormData((prev) => ({
                      ...prev,
                      schedule: {
                        kind: "at",
                        atMs: date.getTime(),
                      },
                    }))
                  }}
                  required
                />
              </div>
            )}

            {formData.schedule.kind === "every" && (
              <div className="grid gap-2">
                <Label htmlFor="every-seconds">
                  {t("pages.schedules.every_seconds", "Interval (seconds)")} *
                </Label>
                <Input
                  id="every-seconds"
                  type="number"
                  min={1}
                  value={Math.floor(
                    (formData.schedule.everyMs || 3600000) / 1000,
                  )}
                  onChange={(e) => {
                    const seconds = parseInt(e.target.value) || 3600
                    setFormData((prev) => ({
                      ...prev,
                      schedule: {
                        kind: "every",
                        everyMs: seconds * 1000,
                      },
                    }))
                  }}
                  required
                />
                <p className="text-muted-foreground text-xs">
                  {t(
                    "pages.schedules.every_hint",
                    "e.g., 60 = 1 minute, 3600 = 1 hour, 86400 = 1 day",
                  )}
                </p>
              </div>
            )}

            {formData.schedule.kind === "cron" && (
              <div className="grid gap-2">
                <Label htmlFor="cron-expr">
                  {t("pages.schedules.cron_expr", "Cron Expression")} *
                </Label>
                <Input
                  id="cron-expr"
                  value={formData.schedule.expr || ""}
                  onChange={(e) =>
                    setFormData((prev) => ({
                      ...prev,
                      schedule: {
                        kind: "cron",
                        expr: e.target.value,
                      },
                    }))
                  }
                  placeholder="0 9 * * *"
                  required
                />
                <p className="text-muted-foreground text-xs">
                  {t(
                    "pages.schedules.cron_hint",
                    "Format: minute hour day month weekday (e.g., 0 9 * * * for daily at 9 AM)",
                  )}
                </p>
              </div>
            )}

            <div className="grid gap-2">
              <Label htmlFor="message">
                {t("pages.schedules.message", "Message")} *
              </Label>
              <Textarea
                id="message"
                value={formData.message}
                onChange={(e) =>
                  setFormData((prev) => ({ ...prev, message: e.target.value }))
                }
                placeholder={t(
                  "pages.schedules.message_placeholder",
                  "Message to send when the job runs...",
                )}
                rows={3}
                required
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="command">
                {t("pages.schedules.command", "Command (Optional)")}
              </Label>
              <Input
                id="command"
                value={formData.command}
                onChange={(e) =>
                  setFormData((prev) => ({ ...prev, command: e.target.value }))
                }
                placeholder={t(
                  "pages.schedules.command_placeholder",
                  "e.g., df -h (shell command to execute)",
                )}
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="channel">
                  {t("pages.schedules.channel", "Channel")}
                </Label>
                <Input
                  id="channel"
                  value={formData.channel}
                  onChange={(e) =>
                    setFormData((prev) => ({
                      ...prev,
                      channel: e.target.value,
                    }))
                  }
                  placeholder="telegram"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="to">{t("pages.schedules.to", "To")}</Label>
                <Input
                  id="to"
                  value={formData.to}
                  onChange={(e) =>
                    setFormData((prev) => ({ ...prev, to: e.target.value }))
                  }
                  placeholder={t("pages.schedules.to_placeholder", "Chat ID")}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={handleClose}>
              {t("common.cancel", "Cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting
                ? t("common.saving", "Saving...")
                : isEditing
                  ? t("common.save", "Save")
                  : t("common.create", "Create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
