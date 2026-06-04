import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  type CreateJobRequest,
  type CronJob,
  type UpdateJobRequest,
  createCronJob,
  deleteCronJob,
  getCronJobs,
  toggleCronJob,
  updateCronJob,
} from "@/api/cron"

export type ScheduleFilter = "all" | "enabled" | "disabled"
export type ScheduleTypeFilter = "all" | "at" | "every" | "cron"

export function useSchedules() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [filter, setFilter] = useState<ScheduleFilter>("all")
  const [typeFilter, setTypeFilter] = useState<ScheduleTypeFilter>("all")
  const [searchQuery, setSearchQuery] = useState("")

  const {
    data: jobsData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["cron", "jobs"],
    queryFn: getCronJobs,
  })

  const jobs = jobsData?.jobs || []

  const filteredJobs = jobs.filter((job: CronJob) => {
    // Status filter
    if (filter === "enabled" && !job.enabled) return false
    if (filter === "disabled" && job.enabled) return false

    // Type filter
    if (typeFilter !== "all" && job.schedule.kind !== typeFilter) return false

    // Search filter
    if (searchQuery) {
      const query = searchQuery.toLowerCase()
      const matchesName = job.name.toLowerCase().includes(query)
      const matchesMessage = job.payload.message.toLowerCase().includes(query)
      const matchesChannel = job.payload.channel?.toLowerCase().includes(query)
      if (!matchesName && !matchesMessage && !matchesChannel) return false
    }

    return true
  })

  const createMutation = useMutation({
    mutationFn: createCronJob,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cron", "jobs"] })
      toast.success(
        t("pages.schedules.create_success", "Job created successfully"),
      )
    },
    onError: () => {
      toast.error(t("pages.schedules.create_error", "Failed to create job"))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateJobRequest }) =>
      updateCronJob(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cron", "jobs"] })
      toast.success(
        t("pages.schedules.update_success", "Job updated successfully"),
      )
    },
    onError: () => {
      toast.error(t("pages.schedules.update_error", "Failed to update job"))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteCronJob,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cron", "jobs"] })
      toast.success(
        t("pages.schedules.delete_success", "Job deleted successfully"),
      )
    },
    onError: () => {
      toast.error(t("pages.schedules.delete_error", "Failed to delete job"))
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      toggleCronJob(id, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cron", "jobs"] })
    },
    onError: () => {
      toast.error(t("pages.schedules.toggle_error", "Failed to toggle job"))
    },
  })

  const handleCreate = (data: CreateJobRequest) => {
    createMutation.mutate(data)
  }

  const handleUpdate = (id: string, data: UpdateJobRequest) => {
    updateMutation.mutate({ id, data })
  }

  const handleDelete = (id: string) => {
    deleteMutation.mutate(id)
  }

  const handleToggle = (id: string, enabled: boolean) => {
    toggleMutation.mutate({ id, enabled })
  }

  return {
    jobs,
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
    handleUpdate,
    handleDelete,
    handleToggle,
    isCreating: createMutation.isPending,
    isUpdating: updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
    isToggling: toggleMutation.isPending,
  }
}
