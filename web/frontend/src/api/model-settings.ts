import { launcherFetch } from "@/api/http"

/**
 * 模型设置响应接口
 */
export interface ModelSettingsResponse {
  /** 当前活动模型名称 */
  model_name: string
  /** 备选模型列表（按优先级排序） */
  model_fallbacks: string[]
}

/**
 * 模型设置更新接口
 */
export interface ModelSettingsUpdate {
  /** 要设置的活动模型名称 */
  model_name: string
  /** 备选模型列表（按优先级排序） */
  model_fallbacks: string[]
}

/**
 * 获取模型设置
 * @returns 当前模型设置（活动模型和备选模型链）
 */
export async function getModelSettings(): Promise<ModelSettingsResponse> {
  const res = await launcherFetch("/api/agent/model-settings")
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`)
  }
  return res.json() as Promise<ModelSettingsResponse>
}

/**
 * 更新模型设置
 * @param settings - 要更新的模型设置
 * @returns 更新后的模型设置
 */
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
