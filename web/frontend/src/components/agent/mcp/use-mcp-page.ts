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

/**
 * MCP 页面自定义 Hook
 * 管理 MCP 配置的加载、保存和草稿状态
 * @returns MCP 页面状态和操作函数
 */
export function useMCPPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  // 草稿状态覆盖（用户修改但未保存的配置）
  const [mcpDraftOverride, setMcpDraftOverride] =
    useState<MCPConfigResponse | null>(null)

  // 获取 MCP 配置查询
  const mcpQuery = useQuery({
    queryKey: ["mcp-config"],
    queryFn: getMCPConfig,
  })

  // 当前草稿：优先使用覆盖值，否则使用查询结果
  const mcpDraft = mcpDraftOverride ?? mcpQuery.data ?? null

  // 保存配置 mutation
  const saveMCPMutation = useMutation({
    mutationFn: updateMCPConfig,
    onSuccess: (updatedConfig) => {
      // 更新缓存
      queryClient.setQueryData(["mcp-config"], updatedConfig)
      // 清除草稿覆盖
      setMcpDraftOverride(null)
      // 显示成功提示
      toast.success(
        t("pages.agent.mcp.save_success", "MCP settings saved successfully"),
      )
      // 刷新相关查询
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

  /**
   * 更新 MCP 草稿
   * @param updater - 更新函数，接收当前配置并返回新配置
   */
  const updateMCPDraft: MCPDraftUpdater = (updater) => {
    setMcpDraftOverride((current) => {
      const base = current ?? mcpQuery.data
      if (!base) return null
      return updater(base)
    })
  }

  /**
   * 保存 MCP 配置
   * 将当前草稿提交到服务器
   */
  const saveMCPConfig = () => {
    if (mcpDraft) {
      saveMCPMutation.mutate(mcpDraft)
    }
  }

  return {
    mcpDraft,           // 当前配置草稿
    hasMCPError: mcpQuery.isError,      // 是否有加载错误
    isMCPLoading: mcpQuery.isLoading,   // 是否正在加载
    isMCPSaving: saveMCPMutation.isPending, // 是否正在保存
    saveMCPConfig,      // 保存配置函数
    updateMCPDraft,     // 更新草稿函数
  }
}
