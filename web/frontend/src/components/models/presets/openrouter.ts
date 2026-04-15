import type { ModelTemplate } from "../types"

export const PRESET_MODELS_OPENROUTER: ModelTemplate[] = [
  {
    provider: "openrouter",
    model: "openrouter/anthropic/claude-opus-4.6",
    name: "Claude Opus 4.6 (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/deepseek/deepseek-v3.2",
    name: "DeepSeek V3.2 (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/anthropic/claude-sonnet-4.6",
    name: "Claude Sonnet 4.6 (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/minimax/minimax-m2.7",
    name: "MiniMax M2.7 (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/minimax/minimax-m2.5",
    name: "MiniMax M2.5 (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemini-3-flash-preview",
    name: "Gemini 3 Flash Preview (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/xiaomi/mimo-v2-pro",
    name: "MiMo V2 Pro (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemini-2.5-flash",
    name: "Gemini 2.5 Flash (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemini-2.5-flash-lite",
    name: "Gemini 2.5 Flash Lite (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/x-ai/grok-4.1-fast",
    name: "Grok 4.1 Fast (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemini-3.1-pro-preview",
    name: "Gemini 3.1 Pro Preview (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/openai/gpt-5.4",
    name: "GPT-5.4 (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/moonshotai/kimi-k2.5",
    name: "Kimi K2.5 (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/z-ai/glm-5",
    name: "GLM-5 (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/z-ai/glm-5.1",
    name: "GLM-5.1 (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/openai/gpt-oss-120b",
    name: "GPT-OSS 120B (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/openai/gpt-4o-mini",
    name: "GPT-4o-mini (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemini-3.1-flash-lite-preview",
    name: "Gemini 3.1 Flash Lite Preview (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/z-ai/glm-5-turbo",
    name: "GLM-5 Turbo (OR)",
  },
  {
    provider: "openrouter",
    model: "openrouter/qwen/qwen3.6-plus",
    name: "Qwen3.6 Plus (OR)",
  },
]

// https://openrouter.ai/api/frontend/models/find?order=most-popular&q=free
export const PRESET_MODELS_OPENROUTER_FREE: ModelTemplate[] = [
  {
    provider: "openrouter",
    model: "openrouter/nvidia/nemotron-3-super-120b-a12b:free",
    name: "Nemotron 3 Super 120B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/z-ai/glm-4.5-air:free",
    name: "GLM 4.5 Air (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/openai/gpt-oss-120b:free",
    name: "GPT-OSS 120B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/nvidia/nemotron-3-nano-30b-a3b:free",
    name: "Nemotron 3 Nano 30B A3B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/minimax/minimax-m2.5:free",
    name: "MiniMax M2.5 (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/nvidia/nemotron-nano-9b-v2:free",
    name: "Nemotron Nano 9B V2 (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemma-4-31b-it:free",
    name: "Gemma 4 31B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/nvidia/nemotron-nano-12b-v2-vl:free",
    name: "Nemotron Nano 12B 2VL (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemma-4-26b-a4b-it:free",
    name: "Gemma 4 26B A4B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/openai/gpt-oss-20b:free",
    name: "GPT-OSS 20B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/nvidia/llama-nemotron-embed-vl-1b-v2:free",
    name: "Llama Nemotron Embed VL 1B V2 (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/qwen/qwen3-coder:free",
    name: "Qwen3 Coder 480B A35B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/meta-llama/llama-3.3-70b-instruct:free",
    name: "Llama 3.3 70B Instruct (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/liquid/lfm-2.5-1.2b-thinking:free",
    name: "LFM2.5 1.2B Thinking (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/qwen/qwen3-next-80b-a3b-instruct:free",
    name: "Qwen3 Next 80B A3B Instruct (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/liquid/lfm-2.5-1.2b-instruct:free",
    name: "LFM2.5 1.2B Instruct (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemma-3-27b-it:free",
    name: "Gemma 3 27B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model:
      "openrouter/cognitivecomputations/dolphin-mistral-24b-venice-edition:free",
    name: "Venice Uncensored (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/nousresearch/hermes-3-llama-3.1-405b:free",
    name: "Hermes 3 405B Instruct (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/meta-llama/llama-3.2-3b-instruct:free",
    name: "Llama 3.2 3B Instruct (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemma-3-4b-it:free",
    name: "Gemma 3 4B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemma-3n-e4b-it:free",
    name: "Gemma 3n 4B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemma-3n-e2b-it:free",
    name: "Gemma 3n 2B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/google/gemma-3-12b-it:free",
    name: "Gemma 3 12B (OR Free)",
    tag: "free",
  },
  {
    provider: "openrouter",
    model: "openrouter/free",
    name: "Free Models Router (OR)",
    tag: "free",
  },
]
