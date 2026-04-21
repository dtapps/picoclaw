export type ModelTag = "free" | "popular"

export interface ModelTemplate {
  provider: string
  model: string
  name: string
  tag?: ModelTag
}
