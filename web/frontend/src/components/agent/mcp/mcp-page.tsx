import { useTranslation } from "react-i18next"

import { PageHeader } from "@/components/page-header"

import { MCPConfigContent } from "./mcp-config-content"
import { useMCPPage } from "./use-mcp-page"

export function MCPPage() {
  const { t } = useTranslation()
  const {
    mcpDraft,
    hasMCPError,
    isMCPLoading,
    isMCPSaving,
    saveMCPConfig,
    updateMCPDraft,
  } = useMCPPage()

  return (
    <div className="bg-background flex h-full flex-col">
      <PageHeader title={t("navigation.mcp", "MCP")} />

      <div className="flex-1 overflow-auto px-6 py-6 pb-20">
        <div className="mx-auto w-full max-w-6xl">
          <MCPConfigContent
            draft={mcpDraft}
            isLoading={isMCPLoading}
            hasError={hasMCPError}
            isSaving={isMCPSaving}
            onSave={saveMCPConfig}
            onUpdateDraft={updateMCPDraft}
          />
        </div>
      </div>
    </div>
  )
}
