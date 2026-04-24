import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  type MCPConfigResponse,
  getMCPConfig,
  updateMCPConfig,
} from "@/api/mcp"
import { refreshGatewayState } from "@/store/gateway"

import type { MCPDraftUpdater } from "./types"

export function useMCPPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [mcpDraftOverride, setMcpDraftOverride] =
    useState<MCPConfigResponse | null>(null)

  const mcpQuery = useQuery({
    queryKey: ["mcp-config"],
    queryFn: getMCPConfig,
  })

  const mcpDraft = mcpDraftOverride ?? mcpQuery.data ?? null

  const saveMCPMutation = useMutation({
    mutationFn: updateMCPConfig,
    onSuccess: (updatedConfig) => {
      queryClient.setQueryData(["mcp-config"], updatedConfig)
      setMcpDraftOverride(null)
      toast.success(
        t("pages.agent.mcp.save_success", "MCP settings saved successfully"),
      )
      void queryClient.invalidateQueries({
        queryKey: ["mcp-config"],
      })
      void queryClient.invalidateQueries({ queryKey: ["tools"] })
      void refreshGatewayState({ force: true })
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t("pages.agent.mcp.save_error", "Failed to save MCP settings"),
      )
    },
  })

  const updateMCPDraft: MCPDraftUpdater = (updater) => {
    setMcpDraftOverride((current) => {
      const base = current ?? mcpQuery.data
      if (!base) return null
      return updater(base)
    })
  }

  const saveMCPConfig = () => {
    if (mcpDraft) {
      saveMCPMutation.mutate(mcpDraft)
    }
  }

  return {
    mcpDraft,
    hasMCPError: mcpQuery.isError,
    isMCPLoading: mcpQuery.isLoading,
    isMCPSaving: saveMCPMutation.isPending,
    saveMCPConfig,
    updateMCPDraft,
  }
}
