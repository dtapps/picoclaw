import { launcherFetch } from "./http"

export interface CronSchedule {
  kind: "at" | "every" | "cron"
  atMs?: number
  everyMs?: number
  expr?: string
  tz?: string
}

export interface CronPayload {
  kind: string
  message: string
  command?: string
  channel?: string
  to?: string
}

export interface CronJobState {
  nextRunAtMs?: number
  lastRunAtMs?: number
  lastStatus?: string
  lastError?: string
}

export interface CronJob {
  id: string
  name: string
  enabled: boolean
  schedule: CronSchedule
  payload: CronPayload
  state: CronJobState
  createdAtMs: number
  updatedAtMs: number
  deleteAfterRun: boolean
}

export interface CreateJobRequest {
  name: string
  schedule: CronSchedule
  message: string
  command?: string
  channel: string
  to: string
}

export interface UpdateJobRequest {
  name?: string
  enabled?: boolean
  schedule?: CronSchedule
  message?: string
  command?: string
}

export async function getCronJobs(): Promise<{ jobs: CronJob[] }> {
  const res = await launcherFetch("/api/cron/jobs")
  if (!res.ok) {
    throw new Error("Failed to fetch cron jobs")
  }
  return res.json()
}

export async function getCronJob(id: string): Promise<CronJob> {
  const res = await launcherFetch(`/api/cron/jobs/${id}`)
  if (!res.ok) {
    throw new Error("Failed to fetch cron job")
  }
  return res.json()
}

export async function createCronJob(job: CreateJobRequest): Promise<CronJob> {
  const res = await launcherFetch("/api/cron/jobs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(job),
  })
  if (!res.ok) {
    throw new Error("Failed to create cron job")
  }
  return res.json()
}

export async function updateCronJob(
  id: string,
  job: UpdateJobRequest,
): Promise<CronJob> {
  const res = await launcherFetch(`/api/cron/jobs/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(job),
  })
  if (!res.ok) {
    throw new Error("Failed to update cron job")
  }
  return res.json()
}

export async function deleteCronJob(id: string): Promise<void> {
  const res = await launcherFetch(`/api/cron/jobs/${id}`, {
    method: "DELETE",
  })
  if (!res.ok) {
    throw new Error("Failed to delete cron job")
  }
}

export async function toggleCronJob(
  id: string,
  enabled: boolean,
): Promise<CronJob> {
  const res = await launcherFetch(`/api/cron/jobs/${id}/toggle`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled }),
  })
  if (!res.ok) {
    throw new Error("Failed to toggle cron job")
  }
  return res.json()
}

export function formatSchedule(schedule: CronSchedule): string {
  switch (schedule.kind) {
    case "at":
      if (schedule.atMs) {
        return new Date(schedule.atMs).toLocaleString()
      }
      return "One-time"
    case "every":
      if (schedule.everyMs) {
        const seconds = Math.floor(schedule.everyMs / 1000)
        if (seconds < 60) return `Every ${seconds} seconds`
        const minutes = Math.floor(seconds / 60)
        if (minutes < 60) return `Every ${minutes} minutes`
        const hours = Math.floor(minutes / 60)
        if (hours < 24) return `Every ${hours} hours`
        const days = Math.floor(hours / 24)
        return `Every ${days} days`
      }
      return "Recurring"
    case "cron":
      return schedule.expr || "Cron expression"
    default:
      return "Unknown"
  }
}

export function getNextRunText(state: CronJobState): string {
  if (state.nextRunAtMs) {
    const nextRun = new Date(state.nextRunAtMs)
    const now = new Date()
    const diff = nextRun.getTime() - now.getTime()

    if (diff < 0) return "Overdue"
    if (diff < 60000) return "In less than a minute"
    if (diff < 3600000) {
      const minutes = Math.floor(diff / 60000)
      return `In ${minutes} minutes`
    }
    if (diff < 86400000) {
      const hours = Math.floor(diff / 3600000)
      return `In ${hours} hours`
    }
    return nextRun.toLocaleString()
  }
  return "Not scheduled"
}
