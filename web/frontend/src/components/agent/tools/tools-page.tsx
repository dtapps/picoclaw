import { useTranslation } from "react-i18next"

import { PageHeader } from "@/components/page-header"

import { ToolLibraryTab } from "./tool-library-tab"
import { ToolsTabs } from "./tools-tabs"
import { useToolsPage } from "./use-tools-page"
import { WebSearchTab } from "./web-search-tab"

export function ToolsPage() {
  const { t } = useTranslation()
  const {
    activeTab,
    currentWebSearchProviderLabel,
    expandedWebSearchProvider,
    groupedTools,
    pendingToolName,
    webSearchProviderLabelMap,
    searchQuery,
    statusFilter,
    tools,
    totalFilteredCount,
    webSearchDraft,
    hasToolsError,
    hasWebSearchError,
    isToolsLoading,
    isWebSearchLoading,
    isWebSearchSaving,
    setActiveTab,
    setSearchQuery,
    setStatusFilter,
    saveWebSearchConfig,
    toggleExpandedWebSearchProvider,
    toggleTool,
    updateWebSearchDraft,
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
          ) : (
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
          )}
        </div>
      </div>
    </div>
  )
}
