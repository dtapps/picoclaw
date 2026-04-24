import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useDeferredValue, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  type EncyclopediaSearchConfigResponse,
  type WebSearchConfigResponse,
  getEncyclopediaSearchConfig,
  getTools,
  getWebSearchConfig,
  setToolEnabled,
  updateEncyclopediaSearchConfig,
  updateWebSearchConfig,
} from "@/api/tools"
import { refreshGatewayState } from "@/store/gateway"

import type { GroupedTools, ToolStatusFilter, ToolsPageTab } from "./types"

export function useToolsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [activeTab, setActiveTab] = useState<ToolsPageTab>("library")
  const [searchQuery, setSearchQuery] = useState("")
  const deferredSearchQuery = useDeferredValue(searchQuery)
  const [statusFilter, setStatusFilter] = useState<ToolStatusFilter>("all")
  const [expandedWebSearchProvider, setExpandedWebSearchProvider] = useState<
    string | null
  >(null)
  const [
    expandedEncyclopediaSearchProvider,
    setExpandedEncyclopediaSearchProvider,
  ] = useState<string | null>(null)
  const [webSearchDraftOverride, setWebSearchDraftOverride] =
    useState<WebSearchConfigResponse | null>(null)
  const [encyclopediaSearchDraftOverride, setEncyclopediaSearchDraftOverride] =
    useState<EncyclopediaSearchConfigResponse | null>(null)

  const toolsQuery = useQuery({
    queryKey: ["tools"],
    queryFn: getTools,
  })
  const webSearchQuery = useQuery({
    queryKey: ["tools", "web-search-config"],
    queryFn: getWebSearchConfig,
  })
  const encyclopediaSearchQuery = useQuery({
    queryKey: ["tools", "encyclopedia-search-config"],
    queryFn: getEncyclopediaSearchConfig,
  })

  const tools = useMemo(
    () => toolsQuery.data?.tools ?? [],
    [toolsQuery.data?.tools],
  )
  const normalizedSearchQuery = deferredSearchQuery.trim().toLowerCase()
  const webSearchDraft = webSearchDraftOverride ?? webSearchQuery.data ?? null
  const encyclopediaSearchDraft =
    encyclopediaSearchDraftOverride ?? encyclopediaSearchQuery.data ?? null

  const toggleToolMutation = useMutation({
    mutationFn: async ({ name, enabled }: { name: string; enabled: boolean }) =>
      setToolEnabled(name, enabled),
    onSuccess: (_, variables) => {
      toast.success(
        variables.enabled
          ? t("pages.agent.tools.enable_success", "Tool enabled successfully")
          : t(
              "pages.agent.tools.disable_success",
              "Tool disabled successfully",
            ),
      )
      void queryClient.invalidateQueries({ queryKey: ["tools"] })
      void refreshGatewayState({ force: true })
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t("pages.agent.tools.toggle_error", "Failed to toggle tool"),
      )
    },
  })

  const saveWebSearchMutation = useMutation({
    mutationFn: updateWebSearchConfig,
    onSuccess: (updatedConfig) => {
      queryClient.setQueryData(["tools", "web-search-config"], updatedConfig)
      setWebSearchDraftOverride(null)
      toast.success(
        t(
          "pages.agent.tools.web_search.save_success",
          "Settings saved successfully",
        ),
      )
      void queryClient.invalidateQueries({
        queryKey: ["tools", "web-search-config"],
      })
      void queryClient.invalidateQueries({ queryKey: ["tools"] })
      void refreshGatewayState({ force: true })
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t(
              "pages.agent.tools.web_search.save_error",
              "Failed to save settings",
            ),
      )
    },
  })

  const saveEncyclopediaSearchMutation = useMutation({
    mutationFn: updateEncyclopediaSearchConfig,
    onSuccess: (updatedConfig) => {
      queryClient.setQueryData(
        ["tools", "encyclopedia-search-config"],
        updatedConfig,
      )
      setEncyclopediaSearchDraftOverride(null)
      toast.success(
        t(
          "pages.agent.tools.encyclopedia_search.save_success",
          "Settings saved successfully",
        ),
      )
      void queryClient.invalidateQueries({
        queryKey: ["tools", "encyclopedia-search-config"],
      })
      void queryClient.invalidateQueries({ queryKey: ["tools"] })
      void refreshGatewayState({ force: true })
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t(
              "pages.agent.tools.encyclopedia_search.save_error",
              "Failed to save settings",
            ),
      )
    },
  })

  const groupedTools = useMemo<{
    groupedTools: GroupedTools
    totalFilteredCount: number
  }>(() => {
    let totalFilteredCount = 0
    const grouped = new Map<string, typeof tools>()

    for (const tool of tools) {
      if (statusFilter !== "all" && tool.status !== statusFilter) {
        continue
      }

      if (normalizedSearchQuery) {
        const matchesName = tool.name
          .toLowerCase()
          .includes(normalizedSearchQuery)
        const matchesDescription = (tool.description || "")
          .toLowerCase()
          .includes(normalizedSearchQuery)

        if (!matchesName && !matchesDescription) {
          continue
        }
      }

      totalFilteredCount += 1
      const items = grouped.get(tool.category) ?? []
      items.push(tool)
      grouped.set(tool.category, items)
    }

    return {
      groupedTools: Array.from(grouped.entries()),
      totalFilteredCount,
    }
  }, [normalizedSearchQuery, statusFilter, tools])

  const webSearchProviderLabelMap = useMemo(() => {
    const providers = webSearchDraft?.providers ?? []
    return new Map(providers.map((provider) => [provider.id, provider.label]))
  }, [webSearchDraft])

  const encyclopediaSearchProviderLabelMap = useMemo(() => {
    const providers = encyclopediaSearchDraft?.providers ?? []
    return new Map(providers.map((provider) => [provider.id, provider.label]))
  }, [encyclopediaSearchDraft])

  const currentWebSearchProviderLabel = webSearchDraft?.current_service
    ? (webSearchProviderLabelMap.get(webSearchDraft.current_service) ??
      webSearchDraft.current_service)
    : t("pages.agent.tools.web_search.none", "None")

  const currentEncyclopediaSearchProviderLabel =
    encyclopediaSearchDraft?.current_service
      ? (encyclopediaSearchProviderLabelMap.get(
          encyclopediaSearchDraft.current_service,
        ) ?? encyclopediaSearchDraft.current_service)
      : t("pages.agent.tools.encyclopedia_search.none", "None")

  const pendingToolName = toggleToolMutation.isPending
    ? (toggleToolMutation.variables?.name ?? null)
    : null

  const updateWebSearchDraft = (
    updater: (current: WebSearchConfigResponse) => WebSearchConfigResponse,
  ) => {
    setWebSearchDraftOverride((current) => {
      const draft = current ?? webSearchQuery.data
      return draft ? updater(draft) : current
    })
  }

  const updateEncyclopediaSearchDraft = (
    updater: (
      current: EncyclopediaSearchConfigResponse,
    ) => EncyclopediaSearchConfigResponse,
  ) => {
    setEncyclopediaSearchDraftOverride((current) => {
      const draft = current ?? encyclopediaSearchQuery.data
      return draft ? updater(draft) : current
    })
  }

  const toggleTool = (name: string, enabled: boolean) => {
    toggleToolMutation.mutate({ name, enabled })
  }

  const saveWebSearchConfig = () => {
    if (webSearchDraft) {
      saveWebSearchMutation.mutate(webSearchDraft)
    }
  }

  const saveEncyclopediaSearchConfig = () => {
    if (encyclopediaSearchDraft) {
      saveEncyclopediaSearchMutation.mutate(encyclopediaSearchDraft)
    }
  }

  const toggleExpandedWebSearchProvider = (providerId: string) => {
    setExpandedWebSearchProvider((current) =>
      current === providerId ? null : providerId,
    )
  }

  const toggleExpandedEncyclopediaSearchProvider = (providerId: string) => {
    setExpandedEncyclopediaSearchProvider((current) =>
      current === providerId ? null : providerId,
    )
  }

  return {
    activeTab,
    currentWebSearchProviderLabel,
    currentEncyclopediaSearchProviderLabel,
    expandedWebSearchProvider,
    expandedEncyclopediaSearchProvider,
    groupedTools: groupedTools.groupedTools,
    pendingToolName,
    webSearchProviderLabelMap,
    encyclopediaSearchProviderLabelMap,
    searchQuery,
    statusFilter,
    tools,
    totalFilteredCount: groupedTools.totalFilteredCount,
    webSearchDraft,
    encyclopediaSearchDraft,
    hasToolsError: toolsQuery.error != null,
    hasWebSearchError: webSearchQuery.error != null,
    hasEncyclopediaSearchError: encyclopediaSearchQuery.error != null,
    isToolsLoading: toolsQuery.isLoading,
    isWebSearchLoading: webSearchQuery.isLoading,
    isEncyclopediaSearchLoading: encyclopediaSearchQuery.isLoading,
    isWebSearchSaving: saveWebSearchMutation.isPending,
    isEncyclopediaSearchSaving: saveEncyclopediaSearchMutation.isPending,
    setActiveTab,
    setSearchQuery,
    setStatusFilter,
    saveWebSearchConfig,
    saveEncyclopediaSearchConfig,
    toggleExpandedWebSearchProvider,
    toggleExpandedEncyclopediaSearchProvider,
    toggleTool,
    updateWebSearchDraft,
    updateEncyclopediaSearchDraft,
  }
}
