import { createFileRoute } from "@tanstack/react-router"

import { SessionHistoryPage } from "@/components/chat/session-history-page"

export const Route = createFileRoute("/sessions/$id")({
  component: SessionHistoryPage,
})
