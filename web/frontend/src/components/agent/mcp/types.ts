import type { MCPConfigResponse } from "@/api/mcp"

/**
 * MCP 草稿更新器类型
 * 用于更新 MCP 配置草稿的函数类型
 * @param updater - 接收当前配置并返回新配置的更新函数
 */
export type MCPDraftUpdater = (
  updater: (current: MCPConfigResponse) => MCPConfigResponse,
) => void
