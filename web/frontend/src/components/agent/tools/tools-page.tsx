import { useTranslation } from "react-i18next"

import { PageHeader } from "@/components/page-header"

import { EncyclopediaSearchTab } from "./encyclopedia-search-tab"
import { MCPTab } from "./mcp-tab"
import { ToolLibraryTab } from "./tool-library-tab"
import { ToolsTabs } from "./tools-tabs"
import { useToolsPage } from "./use-tools-page"
import { WebSearchTab } from "./web-search-tab"

export function ToolsPage() {
  const { t } = useTranslation()
  const {
    activeTab,
    currentWebSearchProviderLabel,
    currentEncyclopediaSearchProviderLabel,
    expandedWebSearchProvider,
    expandedEncyclopediaSearchProvider,
    groupedTools,
    pendingToolName,
    webSearchProviderLabelMap,
    encyclopediaSearchProviderLabelMap,
    searchQuery,
    statusFilter,
    tools,
    totalFilteredCount,
    webSearchDraft,
    encyclopediaSearchDraft,
    mcpDraft,
    hasToolsError,
    hasWebSearchError,
    hasEncyclopediaSearchError,
    hasMCPError,
    isToolsLoading,
    isWebSearchLoading,
    isWebSearchSaving,
    isEncyclopediaSearchLoading,
    isEncyclopediaSearchSaving,
    isMCPLoading,
    isMCPSaving,
    setActiveTab,
    setSearchQuery,
    setStatusFilter,
    saveWebSearchConfig,
    saveEncyclopediaSearchConfig,
    saveMCPConfig,
    toggleExpandedWebSearchProvider,
    toggleExpandedEncyclopediaSearchProvider,
    toggleTool,
    updateWebSearchDraft,
    updateEncyclopediaSearchDraft,
    updateMCPDraft,
  } = useToolsPage()

  return (
    <div className="bg-background flex h-full flex-col">
      <PageHeader title={t("navigation.tools", "Tools")} />
      <ToolsTabs activeTab={activeTab} onChange={setActiveTab} />

      <div className="flex-1 overflow-auto px-6 py-6 pb-20">
        <div className="mx-auto w-full max-w-6xl">
          {activeTab === "library" ? (
            <ToolLibraryTab
              allTools={tools}
              groupedTools={groupedTools}
              totalFilteredCount={totalFilteredCount}
              searchQuery={searchQuery}
              statusFilter={statusFilter}
              isLoading={isToolsLoading}
              hasError={hasToolsError}
              pendingToolName={pendingToolName}
              onSearchQueryChange={setSearchQuery}
              onStatusFilterChange={setStatusFilter}
              onToggleTool={toggleTool}
            />
          ) : activeTab === "web-search" ? (
            <WebSearchTab
              draft={webSearchDraft}
              currentProviderLabel={currentWebSearchProviderLabel}
              providerLabelMap={webSearchProviderLabelMap}
              expandedProvider={expandedWebSearchProvider}
              isLoading={isWebSearchLoading}
              hasError={hasWebSearchError}
              isSaving={isWebSearchSaving}
              onSave={saveWebSearchConfig}
              onToggleProviderExpand={toggleExpandedWebSearchProvider}
              onUpdateDraft={updateWebSearchDraft}
            />
          ) : activeTab === "encyclopedia-search" ? (
            <EncyclopediaSearchTab
              draft={encyclopediaSearchDraft}
              currentProviderLabel={currentEncyclopediaSearchProviderLabel}
              providerLabelMap={encyclopediaSearchProviderLabelMap}
              expandedProvider={expandedEncyclopediaSearchProvider}
              isLoading={isEncyclopediaSearchLoading}
              hasError={hasEncyclopediaSearchError}
              isSaving={isEncyclopediaSearchSaving}
              onSave={saveEncyclopediaSearchConfig}
              onToggleProviderExpand={toggleExpandedEncyclopediaSearchProvider}
              onUpdateDraft={updateEncyclopediaSearchDraft}
            />
          ) : activeTab === "mcp" ? (
            <MCPTab
              draft={mcpDraft}
              isLoading={isMCPLoading}
              hasError={hasMCPError}
              isSaving={isMCPSaving}
              onSave={saveMCPConfig}
              onUpdateDraft={updateMCPDraft}
            />
          ) : (
            ""
          )}
        </div>
      </div>
    </div>
  )
}
