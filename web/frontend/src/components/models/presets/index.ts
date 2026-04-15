import type { ModelTemplate } from "../types"
import { PRESET_MODELS_ANTHROPIC } from "./anthropic"
import { PRESET_MODELS_DEEPSEEK } from "./deepseek"
import { PRESET_MODELS_GEMINI } from "./gemini"
import { PRESET_MODELS_GROQ } from "./groq"
import { PRESET_MODELS_MOONSHOT } from "./moonshot"
import { PRESET_MODELS_NVIDIA, PRESET_MODELS_NVIDIA_FREE } from "./nvidia"
import { PRESET_MODELS_OPENAI } from "./openai"
import {
  PRESET_MODELS_OPENROUTER,
  PRESET_MODELS_OPENROUTER_FREE,
} from "./openrouter"
import { PRESET_MODELS_OTHERS } from "./others"
import { PRESET_MODELS_QWEN } from "./qwen"
import { PRESET_MODELS_ZHIPU } from "./zhipu"

export { PRESET_MODELS_OPENAI } from "./openai"
export {
  PRESET_MODELS_OPENROUTER,
  PRESET_MODELS_OPENROUTER_FREE,
} from "./openrouter"
export { PRESET_MODELS_ANTHROPIC } from "./anthropic"
export { PRESET_MODELS_DEEPSEEK } from "./deepseek"
export { PRESET_MODELS_GEMINI } from "./gemini"
export { PRESET_MODELS_GROQ } from "./groq"
export { PRESET_MODELS_QWEN } from "./qwen"
export { PRESET_MODELS_MOONSHOT } from "./moonshot"
export { PRESET_MODELS_ZHIPU } from "./zhipu"
export { PRESET_MODELS_OTHERS } from "./others"
export { PRESET_MODELS_NVIDIA, PRESET_MODELS_NVIDIA_FREE } from "./nvidia"

export const PRESET_MODELS: ModelTemplate[] = [
  ...PRESET_MODELS_OPENAI,
  ...PRESET_MODELS_OPENROUTER,
  ...PRESET_MODELS_ANTHROPIC,
  ...PRESET_MODELS_DEEPSEEK,
  ...PRESET_MODELS_GEMINI,
  ...PRESET_MODELS_GROQ,
  ...PRESET_MODELS_QWEN,
  ...PRESET_MODELS_MOONSHOT,
  ...PRESET_MODELS_ZHIPU,
  ...PRESET_MODELS_OTHERS,
  ...PRESET_MODELS_NVIDIA,
  ...PRESET_MODELS_NVIDIA_FREE,
  ...PRESET_MODELS_OPENROUTER_FREE,
]
