import { launcherFetch } from "@/api/http"

export interface ModelSettingsResponse {
  model_name: string
  model_fallbacks: string[]
}

export interface ModelSettingsUpdate {
  model_name: string
  model_fallbacks: string[]
}

export async function getModelSettings(): Promise<ModelSettingsResponse> {
  const res = await launcherFetch("/api/agent/model-settings")
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`)
  }
  return res.json() as Promise<ModelSettingsResponse>
}

export async function updateModelSettings(
  settings: ModelSettingsUpdate,
): Promise<ModelSettingsResponse> {
  const res = await launcherFetch("/api/agent/model-settings", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  })
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`)
  }
  return res.json() as Promise<ModelSettingsResponse>
}
