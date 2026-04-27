import type { ModelTemplate } from "../types"

export const PRESET_MODELS_GROQ: ModelTemplate[] = [
  {
    provider: "groq",
    model: "llama-3.3-70b-versatile",
    name: "Llama 3.3 70B",
    tag: "popular",
  },
  { provider: "groq", model: "mixtral-8x7b-32768", name: "Mixtral 8x7B" },
  {
    provider: "groq",
    model: "llama-3.1-8b-instant",
    name: "Llama 3.1 8B",
  },
  {
    provider: "groq",
    model: "llama-3.2-1b-preview",
    name: "Llama 3.2 1B",
  },
  {
    provider: "groq",
    model: "llama-3.2-3b-preview",
    name: "Llama 3.2 3B",
  },
]
