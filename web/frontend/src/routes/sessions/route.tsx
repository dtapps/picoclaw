import { Outlet, createFileRoute, useRouterState } from "@tanstack/react-router"

import { AllSessionsPage } from "@/components/chat/all-sessions-page"

export const Route = createFileRoute("/sessions")({
  component: SessionsLayout,
})

function SessionsLayout() {
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  })

  if (pathname === "/sessions") {
    return <AllSessionsPage />
  }

  return <Outlet />
}
