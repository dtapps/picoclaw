import type {
  EncyclopediaSearchConfigResponse,
  ToolSupportItem,
  WebSearchConfigResponse,
} from "@/api/tools"

export type ToolsPageTab = "library" | "web-search" | "encyclopedia-search"
export type ToolStatusFilter = "all" | ToolSupportItem["status"]
export type GroupedTools = Array<[string, ToolSupportItem[]]>

export type WebSearchDraftUpdater = (
  updater: (current: WebSearchConfigResponse) => WebSearchConfigResponse,
) => void

export type EncyclopediaSearchDraftUpdater = (
  updater: (
    current: EncyclopediaSearchConfigResponse,
  ) => EncyclopediaSearchConfigResponse,
) => void
