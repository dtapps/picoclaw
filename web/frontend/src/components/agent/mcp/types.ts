import type { MCPConfigResponse } from "@/api/mcp"

export type MCPDraftUpdater = (
  updater: (current: MCPConfigResponse) => MCPConfigResponse,
) => void
