import {
  IconCalendar,
  IconFilter,
  IconPlus,
  IconSearch,
} from "@tabler/icons-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import type { CreateJobRequest, CronJob } from "@/api/cron"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

import { DeleteDialog } from "./delete-dialog"
import { JobCard } from "./job-card"
import { JobFormDialog } from "./job-form-dialog"
import {
  type ScheduleFilter,
  type ScheduleTypeFilter,
  useSchedules,
} from "./use-schedules"

export function SchedulesPage() {
  const { t } = useTranslation()
  const {
    filteredJobs,
    isLoading,
    error,
    filter,
    typeFilter,
    searchQuery,
    setFilter,
    setTypeFilter,
    setSearchQuery,
    handleCreate,
    handleDelete,
    handleToggle,
    isCreating,
    isDeleting,
    isToggling,
  } = useSchedules()

  const [deleteJob, setDeleteJob] = useState<CronJob | null>(null)
  const [editJob, setEditJob] = useState<CronJob | null>(null)

  const onDeleteConfirm = () => {
    if (deleteJob) {
      handleDelete(deleteJob.id)
      setDeleteJob(null)
    }
  }

  const onEditSubmit = (data: CreateJobRequest) => {
    if (editJob) {
      // For editing, we'd need to call handleUpdate
      // But for now, we'll just close the dialog
      console.log("Edit job:", editJob.id, data)
      setEditJob(null)
    }
  }

  if (error) {
    return (
      <div className="bg-background flex h-full flex-col">
        <PageHeader title={t("navigation.schedules", "Schedules")} />
        <div className="flex flex-1 items-center justify-center">
          <div className="text-center">
            <p className="text-destructive">
              {t("pages.schedules.load_error", "Failed to load schedules")}
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
      <PageHeader title={t("navigation.schedules", "Schedules")}>
        <JobFormDialog
          onSubmit={handleCreate}
          isSubmitting={isCreating}
          trigger={
            <Button>
              <IconPlus className="mr-2 h-4 w-4" />
              {t("pages.schedules.add_job", "Add Job")}
            </Button>
          }
        />
      </PageHeader>

      <div className="border-b px-6 py-4">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="relative max-w-md flex-1">
            <IconSearch className="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
            <Input
              placeholder={t(
                "pages.schedules.search_placeholder",
                "Search jobs...",
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
              onValueChange={(v) => setFilter(v as ScheduleFilter)}
            >
              <SelectTrigger className="w-[140px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {t("pages.schedules.filter_all", "All")}
                </SelectItem>
                <SelectItem value="enabled">
                  {t("pages.schedules.filter_enabled", "Enabled")}
                </SelectItem>
                <SelectItem value="disabled">
                  {t("pages.schedules.filter_disabled", "Disabled")}
                </SelectItem>
              </SelectContent>
            </Select>
            <Select
              value={typeFilter}
              onValueChange={(v) => setTypeFilter(v as ScheduleTypeFilter)}
            >
              <SelectTrigger className="w-[140px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {t("pages.schedules.type_all", "All Types")}
                </SelectItem>
                <SelectItem value="at">
                  {t("pages.schedules.type_at", "One-time")}
                </SelectItem>
                <SelectItem value="every">
                  {t("pages.schedules.type_every", "Interval")}
                </SelectItem>
                <SelectItem value="cron">
                  {t("pages.schedules.type_cron", "Cron")}
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
          ) : filteredJobs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20">
              <IconCalendar className="text-muted-foreground mb-4 h-12 w-12" />
              <h3 className="text-lg font-medium">
                {t("pages.schedules.empty_title", "No schedules yet")}
              </h3>
              <p className="text-muted-foreground mt-1">
                {t(
                  "pages.schedules.empty_description",
                  "Create your first scheduled task to get started.",
                )}
              </p>
              <JobFormDialog
                onSubmit={handleCreate}
                isSubmitting={isCreating}
                trigger={
                  <Button className="mt-4">
                    <IconPlus className="mr-2 h-4 w-4" />
                    {t("pages.schedules.add_job", "Add Job")}
                  </Button>
                }
              />
            </div>
          ) : (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
              {filteredJobs.map((job) => (
                <JobCard
                  key={job.id}
                  job={job}
                  onToggle={handleToggle}
                  onEdit={(j) => setEditJob(j)}
                  onDelete={(id) =>
                    setDeleteJob(filteredJobs.find((j) => j.id === id) || null)
                  }
                  isToggling={isToggling}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      {/* 编辑对话框 */}
      <JobFormDialog
        job={editJob}
        open={!!editJob}
        onOpenChange={(open) => !open && setEditJob(null)}
        onSubmit={onEditSubmit}
        isSubmitting={isCreating}
      />

      <DeleteDialog
        job={deleteJob}
        open={!!deleteJob}
        onOpenChange={(open) => !open && setDeleteJob(null)}
        onConfirm={onDeleteConfirm}
        isDeleting={isDeleting}
      />
    </div>
  )
}
