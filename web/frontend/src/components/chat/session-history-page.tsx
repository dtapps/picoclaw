import { IconArrowLeft } from "@tabler/icons-react"
import { useNavigate, useParams } from "@tanstack/react-router"
import dayjs from "dayjs"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"

import type { SessionDetail } from "@/api/sessions"
import { getAllSessionHistory } from "@/api/sessions"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"

function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text
  return text.slice(0, maxLength) + "..."
}

export function SessionHistoryPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const params = useParams({ from: "/sessions/$id" })
  const sessionId = params.id
  const [session, setSession] = useState<SessionDetail | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    if (!sessionId) return

    setIsLoading(true)
    setLoadError(null)

    getAllSessionHistory(sessionId)
      .then((data) => {
        setSession(data)
        setIsLoading(false)
      })
      .catch((err) => {
        console.error("Failed to fetch session:", err)
        setLoadError(
          err instanceof Error ? err.message : "Failed to load session",
        )
        setIsLoading(false)
      })
  }, [sessionId])

  const handleBack = () => {
    void navigate({ to: "/sessions" })
  }

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={
          session?.summary
            ? truncateText(session.summary, 18)
            : t("session_history.title", "Conversation Details")
        }
        titleExtra={
          session && (
            <Badge variant="secondary" className="ml-2">
              {t(`channels.name.${session.channel}`, {
                defaultValue: session.channel,
              })}
            </Badge>
          )
        }
      >
        <Button
          variant="ghost"
          size="sm"
          onClick={handleBack}
          className="gap-2"
        >
          <IconArrowLeft className="size-4" />
          {t("session_history.back", "Back")}
        </Button>
      </PageHeader>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-6 md:px-8 lg:px-24 xl:px-48">
        <div className="mx-auto flex w-full max-w-250 flex-col gap-8 pb-8">
          {isLoading ? (
            <div className="space-y-4">
              {Array.from({ length: 6 }).map((_, i) => (
                <div
                  key={i}
                  className={`flex gap-3 ${i % 2 === 0 ? "" : "flex-row-reverse"}`}
                >
                  <Skeleton className="size-8 shrink-0 rounded-full" />
                  <Skeleton className="h-20 flex-1 rounded-2xl" />
                </div>
              ))}
            </div>
          ) : loadError ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <p className="text-destructive mb-4">{loadError}</p>
              <Button variant="outline" onClick={handleBack}>
                {t("session_history.back", "Back")}
              </Button>
            </div>
          ) : !session ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <p className="text-muted-foreground">
                {t("session_history.notFound", "Session not found")}
              </p>
            </div>
          ) : (
            <div className="space-y-6">
              {session.summary && (
                <div className="bg-muted/50 rounded-lg border p-4">
                  <p className="text-muted-foreground mb-1 text-xs font-medium">
                    {t("session_history.summary", "Summary")}
                  </p>
                  <p className="text-sm">{session.summary}</p>
                </div>
              )}

              <div className="space-y-4">
                {session.messages.length === 0 ? (
                  <p className="text-muted-foreground py-8 text-center">
                    {t(
                      "session_history.empty",
                      "No messages in this conversation",
                    )}
                  </p>
                ) : (
                  session.messages.map((message, index) => (
                    <div
                      key={index}
                      className={`flex gap-3 ${
                        message.role === "user" ? "flex-row-reverse" : ""
                      }`}
                    >
                      {message.role === "assistant" && (
                        <div className="bg-primary text-primary-foreground flex size-8 shrink-0 items-center justify-center rounded-full text-xs font-medium">
                          {session?.channel
                            ? session.channel[0].toUpperCase()
                            : "P"}
                        </div>
                      )}
                      {message.role === "user" && (
                        <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-violet-500 text-xs font-medium text-white">
                          U
                        </div>
                      )}
                      <div
                        className={`max-w-[70%] rounded-2xl px-4 py-3 text-[15px] leading-relaxed ${
                          message.role === "user"
                            ? "bg-violet-500 text-white"
                            : "bg-muted"
                        }`}
                      >
                        {message.content}
                        {message.media && message.media.length > 0 && (
                          <div className="mt-2 flex flex-wrap gap-2">
                            {message.media.map((media, mediaIndex) => (
                              <img
                                key={mediaIndex}
                                src={media}
                                alt=""
                                className="max-h-48 max-w-full rounded-lg object-cover"
                              />
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  ))
                )}
              </div>

              <div className="border-border/50 text-muted-foreground flex items-center justify-between rounded-lg border px-4 py-3 text-xs">
                <span>
                  {t("session_history.created", "Created")}:{" "}
                  {dayjs(session.created).format("YYYY-MM-DD HH:mm")}
                </span>
                <span>
                  {t("session_history.updated", "Updated")}:{" "}
                  {dayjs(session.updated).fromNow()}
                </span>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
