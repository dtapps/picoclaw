import { launcherFetch } from "@/api/http"

export interface MCPServerConfig {
  name: string
  enabled: boolean
  command?: string
  args?: string[]
  env?: Record<string, string>
  env_file?: string
  type?: string
  url?: string
  headers?: Record<string, string>
}

export interface MCPDiscoveryConfig {
  enabled: boolean
  ttl: number
  max_search_results: number
  use_bm25: boolean
  use_regex: boolean
}

export interface MCPConfigResponse {
  enabled: boolean
  max_inline_text_chars: number
  discovery: MCPDiscoveryConfig
  servers: MCPServerConfig[]
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
