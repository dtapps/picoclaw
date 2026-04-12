import { IconMenu2 } from "@tabler/icons-react"
import type { ReactNode } from "react"

import { SidebarTrigger } from "@/components/ui/sidebar"
import { cn } from "@/lib/utils"

interface PageHeaderProps {
  title: string
  titleExtra?: ReactNode
  children?: ReactNode
  className?: string
  channel?: string
}

export function PageHeader({
  title,
  titleExtra,
  children,
  className,
  channel,
}: PageHeaderProps) {
  return (
    <div
      className={cn(
        "z-40 flex h-14 shrink-0 items-center justify-between px-6 pt-2",
        className,
      )}
    >
      <div className="flex items-center gap-4">
        <SidebarTrigger className="border-border/60 bg-background text-muted-foreground hover:bg-accent hover:text-foreground hidden h-9 w-9 rounded-lg border sm:flex [&>svg]:size-5">
          <IconMenu2 />
        </SidebarTrigger>
        <h2 className="text-foreground/90 text-xl font-medium tracking-tight">
          {title}
        </h2>
        {channel && (
          <span className="border-border/60 bg-background text-muted-foreground flex h-6 items-center rounded-md border px-2 text-[10px] font-medium tracking-wide shadow-sm">
            {channel}
          </span>
        )}
        {titleExtra}
      </div>
      {children && <div className="flex items-center gap-2">{children}</div>}
    </div>
  )
}
