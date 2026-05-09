import { Outlet, createFileRoute, useRouterState } from "@tanstack/react-router"

import { WorkflowsPage } from "@/components/workflow/workflows-page"

export const Route = createFileRoute("/workflows")({
  component: WorkflowsLayout,
})

function WorkflowsLayout() {
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  })

  if (pathname === "/workflows") {
    return <WorkflowsPage />
  }

  return <Outlet />
}
