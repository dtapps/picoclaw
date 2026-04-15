import type { ModelTemplate } from "../types"

export const PRESET_MODELS_OTHERS: ModelTemplate[] = [
  {
    provider: "cerebras",
    model: "cerebras/llama-3.3-70b",
    name: "Llama 3.3 70B (Cerebras)",
  },
  {
    provider: "cerebras",
    model: "cerebras/llama-3.1-8b",
    name: "Llama 3.1 8B (Cerebras)",
  },
  {
    provider: "mistral",
    model: "mistral/mistral-large-latest",
    name: "Mistral Large",
  },
  {
    provider: "mistral",
    model: "mistral/mistral-small-latest",
    name: "Mistral Small",
  },
  {
    provider: "mistral",
    model: "mistral/mixtral-8x22b-instruct",
    name: "Mixtral 8x22B",
  },
  { provider: "minimax", model: "minimax/minimax-01", name: "Minimax 01" },
  { provider: "minimax", model: "minimax/abab6.5s-chat", name: "ABAB 6.5S" },
  { provider: "vivgrid", model: "vivgrid/vivgrid-1", name: "Vivgrid 1" },
  {
    provider: "vivgrid",
    model: "vivgrid/vivgrid-1-preview",
    name: "Vivgrid 1 Preview",
  },
  {
    provider: "volcengine",
    model: "volcengine/doubao-pro-32k",
    name: "Doubao Pro 32K",
  },
  {
    provider: "volcengine",
    model: "volcengine/doubao-pro-128k",
    name: "Doubao Pro 128K",
  },
  { provider: "venice", model: "venice/venice-1", name: "Venice 1" },
  { provider: "avian", model: "avian/avian-1", name: "Avian 1" },
  { provider: "longcat", model: "longcat/cat-1", name: "Cat 1" },
]
