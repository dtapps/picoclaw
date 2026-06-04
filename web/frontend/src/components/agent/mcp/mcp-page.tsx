import { useTranslation } from "react-i18next"

import { PageHeader } from "@/components/page-header"

import { MCPConfigContent } from "./mcp-config-content"
import { useMCPPage } from "./use-mcp-page"

/**
 * MCP 配置页面组件
 * MCP (Model Context Protocol) 服务器配置的主页面
 * 提供 MCP 服务器的添加、编辑、删除和详情查看功能
 */
export function MCPPage() {
  const { t } = useTranslation()
  // 使用自定义 Hook 获取页面状态和数据操作
  const {
    mcpDraft,        // 当前配置草稿
    hasMCPError,     // 加载错误状态
    isMCPLoading,    // 加载状态
    isMCPSaving,     // 保存状态
    saveMCPConfig,   // 保存配置函数
    updateMCPDraft,  // 更新草稿函数
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
