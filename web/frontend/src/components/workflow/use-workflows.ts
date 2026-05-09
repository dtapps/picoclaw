import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  type CreateWorkflowRequest,
  type UpdateWorkflowRequest,
  type WorkflowListItem,
  createWorkflow,
  deleteWorkflow,
  getWorkflows,
  runWorkflow,
  stopWorkflow,
  toggleWorkflow,
  updateWorkflow,
} from "@/api/workflow"

export type WorkflowFilter = "all" | "enabled" | "disabled"

export function useWorkflows() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [filter, setFilter] = useState<WorkflowFilter>("all")
  const [searchQuery, setSearchQuery] = useState("")

  const { data: workflowsData, isLoading, error } = useQuery({
    queryKey: ["workflows"],
    queryFn: getWorkflows,
  })

  const workflows = workflowsData?.workflows || []

  const filteredWorkflows = workflows.filter((wf: WorkflowListItem) => {
    if (filter === "enabled" && !wf.enabled) return false
    if (filter === "disabled" && wf.enabled) return false

    if (searchQuery) {
      const query = searchQuery.toLowerCase()
      const matchesName = wf.name.toLowerCase().includes(query)
      const matchesDesc = wf.description.toLowerCase().includes(query)
      if (!matchesName && !matchesDesc) return false
    }

    return true
  })

  const createMutation = useMutation({
    mutationFn: createWorkflow,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["workflows"] })
      toast.success(t("pages.workflows.create_success", "Workflow created successfully"))
    },
    onError: (error: Error) => {
      toast.error(error.message || t("pages.workflows.create_error", "Failed to create workflow"))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ name, data }: { name: string; data: UpdateWorkflowRequest }) =>
      updateWorkflow(name, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["workflows"] })
      toast.success(t("pages.workflows.update_success", "Workflow updated successfully"))
    },
    onError: (error: Error) => {
      toast.error(error.message || t("pages.workflows.update_error", "Failed to update workflow"))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteWorkflow,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["workflows"] })
      toast.success(t("pages.workflows.delete_success", "Workflow deleted successfully"))
    },
    onError: () => {
      toast.error(t("pages.workflows.delete_error", "Failed to delete workflow"))
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ name, enabled }: { name: string; enabled: boolean }) =>
      toggleWorkflow(name, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["workflows"] })
    },
    onError: () => {
      toast.error(t("pages.workflows.toggle_error", "Failed to toggle workflow"))
    },
  })

  const runMutation = useMutation({
    mutationFn: runWorkflow,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["workflows"] })
      toast.success(t("pages.workflows.run_success", "Workflow triggered"))
    },
    onError: () => {
      toast.error(t("pages.workflows.run_error", "Failed to run workflow"))
    },
  })

  const stopMutation = useMutation({
    mutationFn: stopWorkflow,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["workflows"] })
      toast.success(t("pages.workflows.stop_success", "Workflow stopped"))
    },
    onError: () => {
      toast.error(t("pages.workflows.stop_error", "Failed to stop workflow"))
    },
  })

  const handleCreate = (data: CreateWorkflowRequest) => {
    createMutation.mutate(data)
  }

  const handleUpdate = (name: string, data: UpdateWorkflowRequest) => {
    updateMutation.mutate({ name, data })
  }

  const handleDelete = (name: string) => {
    deleteMutation.mutate(name)
  }

  const handleToggle = (name: string, enabled: boolean) => {
    toggleMutation.mutate({ name, enabled })
  }

  const handleRun = (name: string) => {
    runMutation.mutate(name)
  }

  const handleStop = (name: string) => {
    stopMutation.mutate(name)
  }

  return {
    workflows,
    filteredWorkflows,
    isLoading,
    error,
    filter,
    searchQuery,
    setFilter,
    setSearchQuery,
    handleCreate,
    handleUpdate,
    handleDelete,
    handleToggle,
    handleRun,
    handleStop,
    isCreating: createMutation.isPending,
    isDeleting: deleteMutation.isPending,
    isToggling: toggleMutation.isPending,
    isRunning: runMutation.isPending,
  }
}
