import type { ModelTemplate } from "../types"

export const PRESET_MODELS_QWEN: ModelTemplate[] = [
  { provider: "qwen", model: "qwen-plus", name: "Qwen Plus" },
  { provider: "qwen", model: "qwen-max", name: "Qwen Max" },
  { provider: "qwen", model: "qwen-turbo", name: "Qwen Turbo" },
  {
    provider: "qwen",
    model: "qwen2.5-72b-instruct",
    name: "Qwen 2.5 72B",
  },
  {
    provider: "qwen",
    model: "qwen2.5-32b-instruct",
    name: "Qwen 2.5 32B",
  },
  { provider: "qwen", model: "qwen2.5-7b-instruct", name: "Qwen 2.5 7B" },
]
