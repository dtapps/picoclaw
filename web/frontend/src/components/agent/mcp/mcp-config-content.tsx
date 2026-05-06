import { IconPlus, IconTrash } from "@tabler/icons-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import type { MCPConfigResponse } from "@/api/mcp"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"

import type { MCPDraftUpdater } from "./types"
import { MCPServerDetails } from "./mcp-server-details"

/**
 * MCP 配置内容组件属性
 */
interface MCPConfigContentProps {
  draft: MCPConfigResponse | null // 当前配置草稿
  isLoading: boolean // 是否正在加载
  hasError: boolean // 是否有错误
  isSaving: boolean // 是否正在保存
  onSave: () => void // 保存回调
  onUpdateDraft: MCPDraftUpdater // 更新草稿的回调
}

/**
 * 验证发现模式配置是否有效
 * 如果启用发现模式，至少需要选择一种搜索方式（BM25 或正则）
 * @param discovery - 发现模式配置
 * @returns 是否有效
 */
function isDiscoveryValid(discovery: MCPConfigResponse["discovery"]): boolean {
  if (!discovery.enabled) return true
  return discovery.use_bm25 || discovery.use_regex
}

export function MCPConfigContent({
  draft,
  isLoading,
  hasError,
  isSaving,
  onSave,
  onUpdateDraft,
}: MCPConfigContentProps) {
  const { t } = useTranslation()

  const isValid = draft ? isDiscoveryValid(draft.discovery) : true

  return (
    <div className="animate-in fade-in slide-in-from-bottom-2 space-y-5 pt-2 duration-500">
      {hasError ? (
        <div className="py-20 text-center">
          <p className="text-destructive font-medium">
            {t(
              "pages.agent.mcp.load_error",
              "Failed to load MCP configuration",
            )}
          </p>
        </div>
      ) : isLoading || !draft ? (
        <LoadingState />
      ) : (
        <>
          <div className="flex flex-col gap-6 sm:flex-row sm:items-start sm:justify-between">
            <div className="max-w-xl space-y-3">
              <div className="flex items-center gap-3">
                <h1 className="text-foreground/90 text-2xl font-semibold tracking-tight">
                  {t("pages.agent.mcp.title", "MCP Configuration")}
                </h1>
                <div
                  className={`rounded-full px-2.5 py-0.5 text-[11px] font-semibold tracking-wide uppercase ${
                    draft.enabled
                      ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                      : "bg-muted text-muted-foreground"
                  }`}
                >
                  {draft.enabled
                    ? t("pages.agent.mcp.enabled", "Enabled")
                    : t("pages.agent.mcp.disabled", "Disabled")}
                </div>
              </div>
              <p className="text-muted-foreground/80 text-[14px] leading-relaxed">
                {t(
                  "pages.agent.mcp.description",
                  "Configure MCP (Model Context Protocol) servers to extend agent capabilities with external tools.",
                )}
              </p>
            </div>

            <Button
              onClick={onSave}
              disabled={isSaving || !isValid}
              className="h-10 shrink-0 rounded-xl px-6 shadow-sm transition-all active:scale-95"
            >
              {t("pages.agent.mcp.save", "Save Changes")}
            </Button>
          </div>

          <div className="space-y-4">
            <GeneralSettings draft={draft} onUpdateDraft={onUpdateDraft} />
            <DiscoverySettings draft={draft} onUpdateDraft={onUpdateDraft} />
            <ServersList draft={draft} onUpdateDraft={onUpdateDraft} />
          </div>
        </>
      )}
    </div>
  )
}

function GeneralSettings({
  draft,
  onUpdateDraft,
}: {
  draft: MCPConfigResponse
  onUpdateDraft: MCPDraftUpdater
}) {
  const { t } = useTranslation()

  return (
    <Card className="border-border/60 overflow-hidden rounded-xl">
      <CardHeader className="bg-muted/30 border-border/60 border-b px-4 py-2">
        <CardTitle className="text-foreground/90 text-sm font-semibold">
          {t("pages.agent.mcp.general.title", "General Settings")}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 p-4">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <Label className="text-foreground/90 text-sm font-medium">
              {t("pages.agent.mcp.general.enable", "Enable MCP")}
            </Label>
            <p className="text-muted-foreground text-xs">
              {t(
                "pages.agent.mcp.general.enable_description",
                "Enable MCP to allow agents to use external tools via Model Context Protocol.",
              )}
            </p>
          </div>
          <Switch
            checked={draft.enabled}
            onCheckedChange={(checked) =>
              onUpdateDraft((current) => ({
                ...current,
                enabled: checked,
              }))
            }
          />
        </div>

        <div className="space-y-2">
          <Label className="text-foreground/90 text-sm font-medium">
            {t(
              "pages.agent.mcp.general.max_inline_text_chars",
              "Max Inline Text Chars",
            )}
          </Label>
          <p className="text-muted-foreground text-xs">
            {t(
              "pages.agent.mcp.general.max_inline_text_chars_description",
              "Maximum text length to display inline before saving as artifact.",
            )}
          </p>
          <Input
            type="number"
            value={draft.max_inline_text_chars}
            onChange={(e) =>
              onUpdateDraft((current) => ({
                ...current,
                max_inline_text_chars: parseInt(e.target.value) || 16384,
              }))
            }
            className="h-9 max-w-xs rounded-lg"
            min={1024}
            max={102400}
            step={1024}
          />
        </div>
      </CardContent>
    </Card>
  )
}

function DiscoverySettings({
  draft,
  onUpdateDraft,
}: {
  draft: MCPConfigResponse
  onUpdateDraft: MCPDraftUpdater
}) {
  const { t } = useTranslation()

  const hasSearchEngine = draft.discovery.use_bm25 || draft.discovery.use_regex
  const showEngineWarning = draft.discovery.enabled && !hasSearchEngine

  return (
    <Card className="border-border/60 overflow-hidden rounded-xl">
      <CardHeader className="bg-muted/30 border-border/60 border-b px-4 py-2">
        <CardTitle className="text-foreground/90 text-sm font-semibold">
          {t("pages.agent.mcp.discovery.title", "Discovery Settings")}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 p-4">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <Label className="text-foreground/90 text-sm font-medium">
              {t("pages.agent.mcp.discovery.enable", "Enable Discovery")}
            </Label>
            <p className="text-muted-foreground text-xs">
              {t(
                "pages.agent.mcp.discovery.enable_description",
                "When enabled, all MCP tools are hidden and loaded on-demand via search. When disabled, all tools are loaded into context.",
              )}
            </p>
          </div>
          <Switch
            checked={draft.discovery.enabled}
            onCheckedChange={(checked) =>
              onUpdateDraft((current) => ({
                ...current,
                discovery: { ...current.discovery, enabled: checked },
              }))
            }
          />
        </div>

        {draft.discovery.enabled && (
          <>
            {showEngineWarning && (
              <div className="bg-destructive/10 text-destructive rounded-lg px-4 py-3 text-sm">
                {t(
                  "pages.agent.mcp.discovery.engine_warning",
                  "Warning: At least one search engine (BM25 or Regex) must be enabled when Discovery is enabled.",
                )}
              </div>
            )}

            <div className="grid gap-6 md:grid-cols-2">
              <div className="space-y-1.5">
                <Label className="text-foreground/90 text-sm font-medium">
                  {t("pages.agent.mcp.discovery.ttl", "TTL")}
                </Label>
                <p className="text-muted-foreground text-xs">
                  {t(
                    "pages.agent.mcp.discovery.ttl_description",
                    "Number of conversation rounds to keep discovered tools unlocked.",
                  )}
                </p>
                <Input
                  type="number"
                  value={draft.discovery.ttl}
                  onChange={(e) =>
                    onUpdateDraft((current) => ({
                      ...current,
                      discovery: {
                        ...current.discovery,
                        ttl: parseInt(e.target.value) || 5,
                      },
                    }))
                  }
                  className="h-9 rounded-lg"
                  min={1}
                  max={100}
                />
              </div>

              <div className="space-y-3">
                <Label className="text-foreground/90 text-sm font-medium">
                  {t(
                    "pages.agent.mcp.discovery.max_search_results",
                    "Max Search Results",
                  )}
                </Label>
                <p className="text-muted-foreground text-xs">
                  {t(
                    "pages.agent.mcp.discovery.max_search_results_description",
                    "Maximum number of tools to return per search.",
                  )}
                </p>
                <Input
                  type="number"
                  value={draft.discovery.max_search_results}
                  onChange={(e) =>
                    onUpdateDraft((current) => ({
                      ...current,
                      discovery: {
                        ...current.discovery,
                        max_search_results: parseInt(e.target.value) || 5,
                      },
                    }))
                  }
                  className="h-9 rounded-lg"
                  min={1}
                  max={50}
                />
              </div>
            </div>

      <div className="grid gap-3 md:grid-cols-2">
              <div className="flex items-center justify-between">
                <div className="space-y-1">
                  <Label className="text-foreground/90 text-sm font-medium">
                    {t("pages.agent.mcp.discovery.use_bm25", "Use BM25")}
                  </Label>
                  <p className="text-muted-foreground text-xs">
                    {t(
                      "pages.agent.mcp.discovery.use_bm25_description",
                      "Enable natural language/keyword search for tools. Consumes more resources than regex search.",
                    )}
                  </p>
                </div>
                <Switch
                  checked={draft.discovery.use_bm25}
                  onCheckedChange={(checked) =>
                    onUpdateDraft((current) => ({
                      ...current,
                      discovery: { ...current.discovery, use_bm25: checked },
                    }))
                  }
                />
              </div>

              <div className="flex items-center justify-between">
                <div className="space-y-1">
                  <Label className="text-foreground/90 text-sm font-medium">
                    {t("pages.agent.mcp.discovery.use_regex", "Use Regex")}
                  </Label>
                  <p className="text-muted-foreground text-xs">
                    {t(
                      "pages.agent.mcp.discovery.use_regex_description",
                      "Enable regex pattern search for tools.",
                    )}
                  </p>
                </div>
                <Switch
                  checked={draft.discovery.use_regex}
                  onCheckedChange={(checked) =>
                    onUpdateDraft((current) => ({
                      ...current,
                      discovery: { ...current.discovery, use_regex: checked },
                    }))
                  }
                />
              </div>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}

function ServersList({
  draft,
  onUpdateDraft,
}: {
  draft: MCPConfigResponse
  onUpdateDraft: MCPDraftUpdater
}) {
  const { t } = useTranslation()
  const [serverToDelete, setServerToDelete] = useState<{
    index: number
    name: string
  } | null>(null)

  const addServer = () => {
    onUpdateDraft((current) => ({
      ...current,
      servers: [
        ...current.servers,
        {
          name: `server-${current.servers.length + 1}`,
          enabled: true,
          command: "",
          args: [],
          type: "stdio",
        },
      ],
    }))
  }

  const removeServer = (index: number) => {
    onUpdateDraft((current) => ({
      ...current,
      servers: current.servers.filter((_, i) => i !== index),
    }))
    setServerToDelete(null)
  }

  const updateServer = (
    index: number,
    updates: Partial<(typeof draft.servers)[0]>,
  ) => {
    onUpdateDraft((current) => ({
      ...current,
      servers: current.servers.map((server, i) =>
        i === index ? { ...server, ...updates } : server,
      ),
    }))
  }

  return (
    <>
      <Card className="border-border/60 overflow-hidden rounded-xl">
        <CardHeader className="bg-muted/30 border-border/60 flex flex-row items-center justify-between border-b px-4 py-2">
          <CardTitle className="text-foreground/90 text-sm font-semibold">
            {t("pages.agent.mcp.servers.title", "MCP Servers")}
          </CardTitle>
          <Button
            onClick={addServer}
            variant="outline"
            size="sm"
            className="h-7 gap-1.5 rounded-md px-2.5 text-xs"
          >
            <IconPlus className="size-3.5" />
            {t("pages.agent.mcp.servers.add", "Add Server")}
          </Button>
        </CardHeader>
        <CardContent className="space-y-3 p-4">
          {draft.servers.length === 0 ? (
            <div className="text-muted-foreground py-4 text-center text-sm">
              {t(
                "pages.agent.mcp.servers.empty",
                'No MCP servers configured. Click "Add Server" to add one.',
              )}
            </div>
          ) : (
            draft.servers.map((server, index) => (
              <ServerCard
                key={index}
                server={server}
                index={index}
                onUpdate={(updates) => updateServer(index, updates)}
                onRemove={() => setServerToDelete({ index, name: server.name })}
              />
            ))
          )}
        </CardContent>
      </Card>

      <AlertDialog
        open={serverToDelete !== null}
        onOpenChange={(open) => !open && setServerToDelete(null)}
      >
        <AlertDialogContent size="sm">
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("pages.agent.mcp.servers.delete_title", "Delete Server")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                "pages.agent.mcp.servers.delete_description",
                'Are you sure you want to delete the server "{{name}}"? This action cannot be undone.',
                { name: serverToDelete?.name },
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("common.cancel", "Cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() =>
                serverToDelete && removeServer(serverToDelete.index)
              }
            >
              <IconTrash className="mr-2 size-4" />
              {t("pages.agent.mcp.servers.delete_confirm", "Delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

function ServerCard({
  server,
  index,
  onUpdate,
  onRemove,
}: {
  server: MCPConfigResponse["servers"][0]
  index: number
  onUpdate: (updates: Partial<typeof server>) => void
  onRemove: () => void
}) {
  const { t } = useTranslation()

  return (
    <div className="bg-muted/30 border-border/60 space-y-2 rounded-lg border p-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h4 className="text-foreground/90 text-sm font-medium">
            {t(
              "pages.agent.mcp.servers.server_title",
              "Server {{index}}",
              {
                index: index + 1,
              },
            )}
          </h4>
          <div
            className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${
              server.enabled
                ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                : "bg-muted text-muted-foreground"
            }`}
          >
            {server.enabled
              ? t("pages.agent.mcp.servers.enabled", "Enabled")
              : t("pages.agent.mcp.servers.disabled", "Disabled")}
          </div>
        </div>
        <Button
          onClick={onRemove}
          variant="ghost"
          size="sm"
          className="text-destructive hover:text-destructive h-8 w-8 rounded-lg p-0"
        >
          <IconTrash className="size-4" />
        </Button>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="flex items-center justify-between md:col-span-2">
          <div className="space-y-1">
            <Label className="text-foreground/90 text-sm font-medium">
              {t("pages.agent.mcp.servers.enable", "Enable Server")}
            </Label>
            <p className="text-muted-foreground text-xs">
              {t(
                "pages.agent.mcp.servers.enable_description",
                "Enable this MCP server to make it available to agents.",
              )}
            </p>
          </div>
          <Switch
            checked={server.enabled}
            onCheckedChange={(checked) => onUpdate({ enabled: checked })}
          />
        </div>

        <div className="space-y-2">
          <Label className="text-xs font-medium">
            {t("pages.agent.mcp.servers.name", "Name")}
          </Label>
          <Input
            value={server.name}
            onChange={(e) => onUpdate({ name: e.target.value })}
            placeholder="my-mcp-server"
            className="h-9 rounded-lg"
          />
        </div>

        <div className="space-y-2">
          <Label className="text-xs font-medium">
            {t("pages.agent.mcp.servers.type", "Type")}
          </Label>
          <select
            value={server.type || "stdio"}
            onChange={(e) => onUpdate({ type: e.target.value })}
            className="border-input bg-background h-9 w-full rounded-lg border px-3 text-sm"
          >
            <option value="stdio">stdio</option>
            <option value="sse">sse</option>
            <option value="http">http</option>
          </select>
        </div>

        {(server.type === "stdio" || !server.type) && (
          <>
            <div className="space-y-2">
              <Label className="text-xs font-medium">
                {t("pages.agent.mcp.servers.command", "Command")}
              </Label>
              <Input
                value={server.command || ""}
                onChange={(e) => onUpdate({ command: e.target.value })}
                placeholder="npx"
                className="h-9 rounded-lg"
              />
            </div>

            <div className="space-y-2">
              <Label className="text-xs font-medium">
                {t("pages.agent.mcp.servers.args", "Arguments")}
              </Label>
              <Input
                value={(server.args || []).join(" ")}
                onChange={(e) =>
                  onUpdate({
                    args: e.target.value
                      .split(" ")
                      .filter((arg) => arg.trim() !== ""),
                  })
                }
                placeholder="-y @modelcontextprotocol/server-filesystem"
                className="h-9 rounded-lg"
              />
            </div>

            <div className="space-y-2 md:col-span-2">
              <Label className="text-xs font-medium">
                {t(
                  "pages.agent.mcp.servers.env",
                  "Environment Variables",
                )}
              </Label>
              <p className="text-muted-foreground text-xs">
                {t(
                  "pages.agent.mcp.servers.env_description",
                  "Environment variables for the server process (one per line, format: KEY=value)",
                )}
              </p>
              <textarea
                value={Object.entries(server.env || {})
                  .map(([k, v]) => `${k}=${v}`)
                  .join("\n")}
                onChange={(e) => {
                  const env: Record<string, string> = {}
                  e.target.value.split("\n").forEach((line) => {
                    const match = line.match(/^([^=]+)=(.*)$/)
                    if (match) {
                      env[match[1].trim()] = match[2].trim()
                    }
                  })
                  onUpdate({ env })
                }}
                placeholder="API_KEY=secret&#10;DEBUG=true"
                className="border-input bg-background min-h-[80px] w-full rounded-lg border px-3 py-2 text-sm"
              />
            </div>
          </>
        )}

        {(server.type === "sse" || server.type === "http") && (
          <>
            <div className="space-y-2 md:col-span-2">
              <Label className="text-xs font-medium">
                {t("pages.agent.mcp.servers.url", "URL")}
              </Label>
              <Input
                value={server.url || ""}
                onChange={(e) => onUpdate({ url: e.target.value })}
                placeholder="http://localhost:3000/sse"
                className="h-9 rounded-lg"
              />
            </div>
            <div className="space-y-2 md:col-span-2">
              <Label className="text-xs font-medium">
                {t("pages.agent.mcp.servers.headers", "Headers")}
              </Label>
              <p className="text-muted-foreground text-xs">
                {t(
                  "pages.agent.mcp.servers.headers_description",
                  "HTTP headers to send with requests (one per line, format: Key: Value)",
                )}
              </p>
              <textarea
                value={Object.entries(server.headers || {})
                  .map(([k, v]) => `${k}: ${v}`)
                  .join("\n")}
                onChange={(e) => {
                  const headers: Record<string, string> = {}
                  e.target.value.split("\n").forEach((line) => {
                    const match = line.match(/^([^:]+):\s*(.+)$/)
                    if (match) {
                      headers[match[1].trim()] = match[2].trim()
                    }
                  })
                  onUpdate({ headers })
                }}
                placeholder="Authorization: Bearer token&#10;X-Custom-Header: value"
                className="border-input bg-background min-h-[80px] w-full rounded-lg border px-3 py-2 text-sm"
              />
            </div>
          </>
        )}

        {(server.type === "stdio" || !server.type) && (
          <div className="space-y-2 md:col-span-2">
            <Label className="text-xs font-medium">
              {t(
                "pages.agent.mcp.servers.env_file",
                "Env File (optional)",
              )}
            </Label>
            <Input
              value={server.env_file || ""}
              onChange={(e) => onUpdate({ env_file: e.target.value })}
              placeholder="/path/to/.env"
              className="h-9 rounded-lg"
            />
          </div>
        )}

        {/* Server Details */}
        <div className="md:col-span-2">
          <MCPServerDetails serverName={server.name} />
        </div>
      </div>
    </div>
  )
}

function LoadingState() {
  return (
    <div className="space-y-5">
      <Skeleton className="h-20 rounded-xl" />
      <Skeleton className="h-48 rounded-xl" />
    </div>
  )
}
