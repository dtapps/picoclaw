import { launcherFetch } from "@/api/http"

/**
 * MCP 服务器配置
 * 定义单个 MCP 服务器的连接和运行参数
 */
export interface MCPServerConfig {
  name: string // 服务器名称
  enabled: boolean // 是否启用
  command?: string // 启动命令（stdio 类型）
  args?: string[] // 命令参数
  env?: Record<string, string> // 环境变量
  env_file?: string // 环境变量文件路径
  type?: string // 连接类型：stdio | sse | http
  url?: string // 服务器 URL（sse/http 类型）
  headers?: Record<string, string> // HTTP 请求头
}

/**
 * MCP 发现模式配置
 * 控制工具的发现和加载行为
 */
export interface MCPDiscoveryConfig {
  enabled: boolean // 是否启用发现模式
  ttl: number // 工具解锁状态的存活轮数
  max_search_results: number // 每次搜索返回的最大工具数
  use_bm25: boolean // 是否使用 BM25 搜索
  use_regex: boolean // 是否使用正则搜索
}

/**
 * MCP 配置响应
 * 完整的 MCP 模块配置信息
 */
export interface MCPConfigResponse {
  enabled: boolean // MCP 总开关
  max_inline_text_chars: number // 最大内联文本字符数
  discovery: MCPDiscoveryConfig // 发现模式配置
  servers: MCPServerConfig[] // 服务器列表
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await launcherFetch(path, options)
  if (!res.ok) {
    let message = `API error: ${res.status} ${res.statusText}`
    try {
      const body = (await res.json()) as {
        error?: string
        errors?: string[]
      }
      if (Array.isArray(body.errors) && body.errors.length > 0) {
        message = body.errors.join("; ")
      } else if (typeof body.error === "string" && body.error.trim() !== "") {
        message = body.error
      }
    } catch {
      // ignore invalid body
    }
    throw new Error(message)
  }
  return res.json() as Promise<T>
}

export async function getMCPConfig(): Promise<MCPConfigResponse> {
  return request<MCPConfigResponse>("/api/mcp/config")
}

export async function updateMCPConfig(
  payload: MCPConfigResponse,
): Promise<MCPConfigResponse> {
  return request<MCPConfigResponse>("/api/mcp/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
}

// ==================== MCP 服务器详情类型 ====================

/**
 * MCP 工具参数定义
 * 描述工具接受的参数及其类型
 */
export interface MCPToolParameter {
  name: string // 参数名称
  type: string // 参数类型（string, number, boolean, object, array 等）
  description: string // 参数描述
  required: boolean // 是否必填
}

/**
 * MCP 工具定义
 * 表示服务器提供的一个可调用工具/函数
 */
export interface MCPTool {
  name: string // 工具名称
  description: string // 工具功能描述
  parameters: MCPToolParameter[] // 工具参数列表
}

/**
 * MCP 提示模板定义
 * 服务器提供的预定义提示模板（预留）
 */
export interface MCPPrompt {
  name: string // 提示名称
  description: string // 提示描述
}

/**
 * MCP 资源定义
 * 服务器可访问的资源（预留）
 */
export interface MCPResource {
  uri: string // 资源唯一标识符
  name: string // 资源名称
  description?: string // 资源描述
  mime_type?: string // MIME 类型
}

/**
 * MCP 服务器详情响应
 * 包含服务器的连接状态和能力列表
 */
export interface MCPServerDetailsResponse {
  server_name: string // 服务器名称
  connected: boolean // 连接状态
  error?: string // 错误信息（连接失败时）
  tools: MCPTool[] // 可用工具列表
  prompts: MCPPrompt[] // 可用提示列表（预留）
  resources: MCPResource[] // 可用资源列表（预留）
}

/**
 * 获取 MCP 服务器详情
 * 包括连接状态、工具列表、提示列表和资源列表
 * @param serverName - 服务器名称
 * @returns 服务器详情响应
 */
export async function getMCPServerDetails(
  serverName: string,
): Promise<MCPServerDetailsResponse> {
  return request<MCPServerDetailsResponse>(
    `/api/mcp/servers/${encodeURIComponent(serverName)}/details`,
  )
}
