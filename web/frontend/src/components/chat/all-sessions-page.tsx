import { IconMessageCircle } from "@tabler/icons-react"
import { useNavigate } from "@tanstack/react-router"
import dayjs from "dayjs"
import relativeTime from "dayjs/plugin/relativeTime"
import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import type { SessionSummary } from "@/api/sessions"
import { getAllSessions } from "@/api/sessions"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"

dayjs.extend(relativeTime)

const LIMIT = 20

export function AllSessionsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const observerRef = useRef<HTMLDivElement>(null)
  const [sessions, setSessions] = useState<SessionSummary[]>([])
  const [offset, setOffset] = useState(0)
  const [hasMore, setHasMore] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [isInitialLoading, setIsInitialLoading] = useState(true)

  const loadSessions = useCallback(
    async (reset = true) => {
      try {
        const currentOffset = reset ? 0 : offset
        if (reset) {
          setLoadError(null)
          setHasMore(true)
          setOffset(0)
        }

        const data = await getAllSessions(currentOffset, LIMIT)
        setLoadError(null)

        if (data.length < LIMIT) {
          setHasMore(false)
        }

        if (reset) {
          setSessions(data)
        } else {
          setSessions((prev) => {
            const existingIds = new Set(prev.map((s) => s.id))
            const newItems = data.filter((s) => !existingIds.has(s.id))
            return [...prev, ...newItems]
          })
        }

        setOffset(currentOffset + data.length)
      } catch (err) {
        console.error("Failed to fetch sessions:", err)
        setLoadError(
          err instanceof Error ? err.message : "Failed to load sessions",
        )
        if (!reset) {
          setHasMore(false)
        }
      } finally {
        setIsLoadingMore(false)
        setIsInitialLoading(false)
      }
    },
    [offset],
  )

  useEffect(() => {
    void loadSessions(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!observerRef.current || !hasMore || isLoadingMore || loadError) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (
          entries[0].isIntersecting &&
          hasMore &&
          !isLoadingMore &&
          !loadError
        ) {
          setIsLoadingMore(true)
          void loadSessions(false)
        }
      },
      { threshold: 0.1 },
    )

    observer.observe(observerRef.current)
    return () => observer.disconnect()
  }, [hasMore, isLoadingMore, loadError, loadSessions])

  return (
    <div className="flex h-full flex-col">
      <PageHeader title={t("all_sessions.title", "All Conversations")} />
      <p className="text-muted-foreground px-6 text-sm">
        {t(
          "all_sessions.description",
          "View conversation history from all channels",
        )}
      </p>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-6 md:px-8 lg:px-24 xl:px-48">
        <div className="mx-auto flex w-full max-w-250 flex-col gap-8 pb-8">
          {isInitialLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <div
                  key={i}
                  className="border-border/50 flex items-start gap-3 rounded-lg border p-4"
                >
                  <Skeleton className="size-10 rounded-full" />
                  <div className="flex-1 space-y-2">
                    <Skeleton className="h-4 w-3/4" />
                    <Skeleton className="h-3 w-1/2" />
                  </div>
                </div>
              ))}
            </div>
          ) : loadError ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <p className="text-destructive mb-4">{loadError}</p>
              <Button variant="outline" onClick={() => void loadSessions(true)}>
                {t("all_sessions.retry", "Retry")}
              </Button>
            </div>
          ) : sessions.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <IconMessageCircle className="text-muted-foreground/50 mb-4 size-12" />
              <p className="text-muted-foreground">
                {t("all_sessions.empty", "No conversations yet")}
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {sessions.map((session) => (
                <button
                  key={session.id}
                  type="button"
                  onClick={() =>
                    void navigate({
                      to: "/sessions/$id",
                      params: { id: session.id },
                    })
                  }
                  className="border-border/50 hover:bg-muted/50 w-full rounded-lg border p-4 text-left transition-colors"
                >
                  <div className="flex items-start gap-3">
                    <div className="bg-muted flex size-10 shrink-0 items-center justify-center rounded-full">
                      <IconMessageCircle className="text-muted-foreground size-5" />
                    </div>
                    <div className="flex-1 overflow-hidden">
                      <div className="mb-1 flex items-center gap-2">
                        <Badge variant="secondary" className="text-[10px]">
                          {t(`channels.name.${session.channel}`, {
                            defaultValue: session.channel,
                          })}
                        </Badge>
                        <span className="text-muted-foreground text-xs">
                          {session.message_count}{" "}
                          {t("all_sessions.messages", "messages")}
                        </span>
                      </div>
                      <h3 className="mb-1 line-clamp-1 text-sm font-medium">
                        {session.title}
                      </h3>
                      <p className="text-muted-foreground line-clamp-2 text-xs">
                        {session.preview}
                      </p>
                      <p className="text-muted-foreground/60 mt-2 text-xs">
                        {dayjs(session.updated).fromNow()}
                      </p>
                    </div>
                  </div>
                </button>
              ))}

              {hasMore && (
                <div ref={observerRef} className="py-4 text-center">
                  {isLoadingMore && (
                    <div className="space-y-3">
                      {Array.from({ length: 3 }).map((_, i) => (
                        <div
                          key={i}
                          className="border-border/50 flex items-start gap-3 rounded-lg border p-4"
                        >
                          <Skeleton className="size-10 rounded-full" />
                          <div className="flex-1 space-y-2">
                            <Skeleton className="h-4 w-3/4" />
                            <Skeleton className="h-3 w-1/2" />
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
