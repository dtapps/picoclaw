import type { ModelTemplate } from "../types"

export const PRESET_MODELS_OPENAI: ModelTemplate[] = [
  {
    provider: "openai",
    model: "openai/gpt-4o",
    name: "GPT-4o",
    tag: "popular",
  },
  {
    provider: "openai",
    model: "openai/gpt-4o-mini",
    name: "GPT-4o Mini",
    tag: "popular",
  },
  { provider: "openai", model: "openai/gpt-4-turbo", name: "GPT-4 Turbo" },
  { provider: "openai", model: "openai/gpt-4", name: "GPT-4" },
  { provider: "openai", model: "openai/gpt-3.5-turbo", name: "GPT-3.5 Turbo" },
]
