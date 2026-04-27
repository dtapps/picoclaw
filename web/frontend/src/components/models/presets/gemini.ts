import type { ModelTemplate } from "../types"

export const PRESET_MODELS_GEMINI: ModelTemplate[] = [
  {
    provider: "gemini",
    model: "gemini-2.0-flash",
    name: "Gemini 2.0 Flash",
    tag: "popular",
  },
  {
    provider: "gemini",
    model: "gemini-1.5-pro",
    name: "Gemini 1.5 Pro",
  },
  {
    provider: "gemini",
    model: "gemini-1.5-flash",
    name: "Gemini 1.5 Flash",
  },
  {
    provider: "gemini",
    model: "gemini-1.5-flash-8b",
    name: "Gemini 1.5 Flash 8B",
  },
]
