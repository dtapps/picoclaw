import {
  IconBolt,
  IconBook,
  IconChevronDown,
  IconChevronRight,
  IconFile,
  IconRefresh,
  IconTools,
  IconAlertCircle,
  IconCheck,
} from "@tabler/icons-react"
import { useQuery } from "@tanstack/react-query"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import type { MCPServerDetailsResponse } from "@/api/mcp"
import { getMCPServerDetails } from "@/api/mcp"
import { Button } from "@/components/ui/button"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import { Skeleton } from "@/components/ui/skeleton"

/**
 * MCP 服务器详情组件属性
 */
interface MCPServerDetailsProps {
  serverName: string // 要展示详情的服务器名称
}

/**
 * MCP 服务器详情展示组件
 * 可折叠面板，展示服务器的连接状态、工具列表、提示列表和资源列表
 * @param serverName - 服务器名称
 */
export function MCPServerDetails({ serverName }: MCPServerDetailsProps) {
  const { t } = useTranslation()
  const [isOpen, setIsOpen] = useState(false) // 控制面板的展开/折叠状态

  // 使用 React Query 获取服务器详情
  const {
    data: details,
    isLoading,
    error,
    refetch,
  } = useQuery<MCPServerDetailsResponse>({
    queryKey: ["mcp-server-details", serverName],
    queryFn: () => getMCPServerDetails(serverName),
    enabled: isOpen, // 只在面板展开时加载数据
    staleTime: 60000, // 数据缓存 1 分钟
  })

  const hasTools = details && details.tools.length > 0
  const hasPrompts = details && details.prompts.length > 0
  const hasResources = details && details.resources.length > 0
  const hasAnyContent = hasTools || hasPrompts || hasResources

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <CollapsibleTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 gap-1.5 rounded-md px-2 text-xs text-muted-foreground hover:text-foreground"
        >
          {isOpen ? (
            <IconChevronDown className="size-3.5" />
          ) : (
            <IconChevronRight className="size-3.5" />
          )}
          {isOpen
            ? t("pages.agent.mcp.servers.hide_details", "Hide Details")
            : t("pages.agent.mcp.servers.show_details", "Show Details")}
        </Button>
      </CollapsibleTrigger>

      <CollapsibleContent>
        <div className="mt-3 space-y-3">
          {isLoading ? (
            <LoadingState />
          ) : error ? (
            <ErrorState
              error={error instanceof Error ? error.message : String(error)}
              onRetry={refetch}
            />
          ) : details ? (
            <>
              <ConnectionStatus connected={details.connected} error={details.error} />
              {details.connected && (
                <>
                  {hasAnyContent ? (
                    <div className="max-h-[400px] overflow-y-auto rounded-lg border">
                      <div className="space-y-4 p-3">
                        {hasTools && <ToolsSection tools={details.tools} />}
                        {hasPrompts && <PromptsSection prompts={details.prompts} />}
                        {hasResources && <ResourcesSection resources={details.resources} />}
                      </div>
                    </div>
                  ) : (
                    <EmptyState />
                  )}
                </>
              )}
            </>
          ) : null}
        </div>
      </CollapsibleContent>
    </Collapsible>
  )
}

/**
 * 连接状态展示组件
 * 显示服务器连接成功或失败的状态
 * @param connected - 是否已连接
 * @param error - 错误信息（连接失败时）
 */
function ConnectionStatus({
  connected,
  error,
}: {
  connected: boolean
  error?: string
}) {
  const { t } = useTranslation()

  return (
    <div
      className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm ${
        connected
          ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
          : "bg-destructive/10 text-destructive"
      }`}
    >
      {connected ? (
        <>
          <IconCheck className="size-4" />
          <span>
            {t("pages.agent.mcp.servers.connected", "Connected")}
          </span>
        </>
      ) : (
        <>
          <IconAlertCircle className="size-4" />
          <span>
            {error ||
              t("pages.agent.mcp.servers.disconnected", "Disconnected")}
          </span>
        </>
      )}
    </div>
  )
}

/**
 * 工具列表展示组件
 * 展示服务器提供的所有工具
 * @param tools - 工具列表
 */
function ToolsSection({ tools }: { tools: MCPServerDetailsResponse["tools"] }) {
  const { t } = useTranslation()

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 text-sm font-medium">
        <IconTools className="size-4 text-muted-foreground" />
        <span>
          {t("pages.agent.mcp.servers.tools", "Tools")}
          <span className="ml-1.5 text-xs text-muted-foreground">
            ({tools.length})
          </span>
        </span>
      </div>
      <div className="space-y-2">
        {tools.map((tool) => (
          <ToolItem key={tool.name} tool={tool} />
        ))}
      </div>
    </div>
  )
}

/**
 * 单个工具项展示组件
 * 可折叠显示工具的详细信息和参数
 * @param tool - 工具定义
 */
function ToolItem({ tool }: { tool: MCPServerDetailsResponse["tools"][0] }) {
  const [isOpen, setIsOpen] = useState(false) // 控制参数展开/折叠
  const hasParameters = tool.parameters && tool.parameters.length > 0

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <div className="rounded-lg border bg-muted/30">
        <CollapsibleTrigger asChild>
          <button className="flex w-full items-center justify-between px-3 py-2 text-left hover:bg-muted/50">
            <div className="flex items-center gap-2">
              <IconBolt className="size-3.5 text-amber-500" />
              <span className="text-sm font-medium">{tool.name}</span>
            </div>
            <div className="flex items-center gap-2">
              {hasParameters && (
                <span className="text-xs text-muted-foreground">
                  {tool.parameters.length} params
                </span>
              )}
              {hasParameters ? (
                isOpen ? (
                  <IconChevronDown className="size-3.5 text-muted-foreground" />
                ) : (
                  <IconChevronRight className="size-3.5 text-muted-foreground" />
                )
              ) : null}
            </div>
          </button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div className="border-t px-3 py-2">
            {tool.description && (
              <p className="mb-2 text-xs text-muted-foreground">
                {tool.description}
              </p>
            )}
            {hasParameters && (
              <div className="space-y-1.5">
                <p className="text-xs font-medium">Parameters:</p>
                {tool.parameters.map((param) => (
                  <div
                    key={param.name}
                    className="flex items-start gap-2 text-xs"
                  >
                    <code className="rounded bg-muted px-1 py-0.5 font-mono">
                      {param.name}
                    </code>
                    <span className="text-muted-foreground">
                      {param.type}
                      {param.required && (
                        <span className="ml-1 text-amber-500">*</span>
                      )}
                    </span>
                    {param.description && (
                      <span className="text-muted-foreground">
                        - {param.description}
                      </span>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  )
}

/**
 * 提示列表展示组件（预留）
 * 展示服务器提供的提示模板
 * @param prompts - 提示列表
 */
function PromptsSection({
  prompts,
}: {
  prompts: MCPServerDetailsResponse["prompts"]
}) {
  const { t } = useTranslation()

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 text-sm font-medium">
        <IconBook className="size-4 text-muted-foreground" />
        <span>
          {t("pages.agent.mcp.servers.prompts", "Prompts")}
          <span className="ml-1.5 text-xs text-muted-foreground">
            ({prompts.length})
          </span>
        </span>
      </div>
      <div className="space-y-1">
        {prompts.map((prompt) => (
          <div
            key={prompt.name}
            className="flex items-center gap-2 rounded-lg border bg-muted/30 px-3 py-2"
          >
            <IconBook className="size-3.5 text-blue-500" />
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">{prompt.name}</p>
              {prompt.description && (
                <p className="text-xs text-muted-foreground">
                  {prompt.description}
                </p>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

/**
 * 资源列表展示组件（预留）
 * 展示服务器可访问的资源
 * @param resources - 资源列表
 */
function ResourcesSection({
  resources,
}: {
  resources: MCPServerDetailsResponse["resources"]
}) {
  const { t } = useTranslation()

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 text-sm font-medium">
        <IconFile className="size-4 text-muted-foreground" />
        <span>
          {t("pages.agent.mcp.servers.resources", "Resources")}
          <span className="ml-1.5 text-xs text-muted-foreground">
            ({resources.length})
          </span>
        </span>
      </div>
      <div className="space-y-1">
        {resources.map((resource) => (
          <div
            key={resource.uri}
            className="flex items-center gap-2 rounded-lg border bg-muted/30 px-3 py-2"
          >
            <IconFile className="size-3.5 text-green-500" />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{resource.name}</p>
              <p className="truncate text-xs text-muted-foreground">
                {resource.uri}
              </p>
              {resource.description && (
                <p className="text-xs text-muted-foreground">
                  {resource.description}
                </p>
              )}
              {resource.mime_type && (
                <span className="mt-1 inline-block rounded bg-muted px-1.5 py-0.5 text-[10px]">
                  {resource.mime_type}
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

/**
 * 加载状态组件
 * 数据加载时显示的骨架屏
 */
function LoadingState() {
  return (
    <div className="space-y-3">
      <Skeleton className="h-8 w-full" />
      <Skeleton className="h-20 w-full" />
      <Skeleton className="h-20 w-full" />
    </div>
  )
}

/**
 * 错误状态组件
 * 加载失败时显示错误信息和重试按钮
 * @param error - 错误信息
 * @param onRetry - 重试回调函数
 */
function ErrorState({ error, onRetry }: { error: string; onRetry: () => void }) {
  const { t } = useTranslation()

  return (
    <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4">
      <div className="flex items-start gap-3">
        <IconAlertCircle className="mt-0.5 size-4 text-destructive" />
        <div className="flex-1">
          <p className="text-sm font-medium text-destructive">
            {t("pages.agent.mcp.servers.load_error", "Failed to load server details")}
          </p>
          <p className="mt-1 text-xs text-destructive/80">{error}</p>
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={onRetry}
          className="h-7 gap-1 text-xs"
        >
          <IconRefresh className="size-3.5" />
          {t("common.retry", "Retry")}
        </Button>
      </div>
    </div>
  )
}

/**
 * 空状态组件
 * 服务器没有提供任何能力时显示
 */
function EmptyState() {
  const { t } = useTranslation()

  return (
    <div className="rounded-lg border bg-muted/30 py-6 text-center">
      <p className="text-sm text-muted-foreground">
        {t(
          "pages.agent.mcp.servers.no_capabilities",
          "No tools, prompts, or resources available from this server.",
        )}
      </p>
    </div>
  )
}
