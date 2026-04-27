import type { ModelTemplate } from "../types"

export const PRESET_MODELS_DEEPSEEK: ModelTemplate[] = [
  {
    provider: "deepseek",
    model: "deepseek-chat",
    name: "DeepSeek Chat",
    tag: "popular",
  },
  {
    provider: "deepseek",
    model: "deepseek-coder",
    name: "DeepSeek Coder",
  },
  {
    provider: "deepseek",
    model: "deepseek-reasoner",
    name: "DeepSeek Reasoner",
  },
]
