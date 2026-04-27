import type { ModelTemplate } from "../types"

export const PRESET_MODELS_ANTHROPIC: ModelTemplate[] = [
  {
    provider: "anthropic",
    model: "claude-sonnet-4-20250514",
    name: "Claude Sonnet 4",
    tag: "popular",
  },
  {
    provider: "anthropic",
    model: "claude-3-5-sonnet-20241022",
    name: "Claude 3.5 Sonnet",
    tag: "popular",
  },
  {
    provider: "anthropic",
    model: "claude-3-5-haiku-20241022",
    name: "Claude 3.5 Haiku",
  },
  {
    provider: "anthropic",
    model: "claude-3-opus-20240229",
    name: "Claude 3 Opus",
  },
  {
    provider: "anthropic",
    model: "claude-3-sonnet-20240229",
    name: "Claude 3 Sonnet",
  },
  {
    provider: "anthropic",
    model: "claude-3-haiku-20240229",
    name: "Claude 3 Haiku",
  },
]
