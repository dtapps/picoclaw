import type { ModelTemplate } from "../types"

export const PRESET_MODELS_ZHIPU: ModelTemplate[] = [
  { provider: "zhipu", model: "zhipu/glm-5.1", name: "GLM-5.1" },
  { provider: "zhipu", model: "zhipu/glm-5", name: "GLM-5" },
  { provider: "zhipu", model: "zhipu/glm-5-turbo", name: "GLM-5-Turbo" },
  { provider: "zhipu", model: "zhipu/glm-4.7", name: "GLM-4.7" },
  { provider: "zhipu", model: "zhipu/glm-4.7-flashx", name: "GLM-4.7-FlashX" },
  { provider: "zhipu", model: "zhipu/glm-4.6", name: "GLM-4.6" },
  { provider: "zhipu", model: "zhipu/glm-4.5-air", name: "GLM-4.5-Air" },
  { provider: "zhipu", model: "zhipu/glm-4.5-airx", name: "GLM-4.5-AirX" },
  { provider: "zhipu", model: "zhipu/glm-4-long", name: "GLM-4-Long" },
  {
    provider: "zhipu",
    model: "zhipu/glm-4-flashx-250414",
    name: "GLM-4-FlashX-250414",
  },
]

export const PRESET_MODELS_ZHIPU_FREE: ModelTemplate[] = [
  { provider: "zhipu", model: "zhipu/glm-4.7-flash", name: "GLM-4.7-Flash" },
  { provider: "zhipu", model: "zhipu/glm-4.5-flash", name: "GLM-4.5-Flash" },
  {
    provider: "zhipu",
    model: "zhipu/glm-4-flash-250414",
    name: "GLM-4-Flash-250414",
  },
]
