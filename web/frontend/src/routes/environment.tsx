import { createFileRoute } from "@tanstack/react-router"

import { EnvironmentPage } from "@/components/environment/environment-page"

export const Route = createFileRoute("/environment")({
  component: EnvironmentPage,
})
