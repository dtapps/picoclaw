import { useState } from "react"
import { useTranslation } from "react-i18next"
import { IconMaximize, IconMinimize } from "@tabler/icons-react"

interface ExpandableTextareaProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  rows?: number
  className?: string
}

export function ExpandableTextarea({
  value,
  onChange,
  placeholder,
  rows = 3,
  className,
}: ExpandableTextareaProps) {
  const { t } = useTranslation()
  const [isExpanded, setIsExpanded] = useState(false)

  return (
    <div className={`relative ${className}`}>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        rows={isExpanded ? 15 : rows}
        placeholder={placeholder}
        className="w-full resize-none rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        style={{ minHeight: isExpanded ? "300px" : undefined }}
      />
      <button
        type="button"
        onClick={() => setIsExpanded(!isExpanded)}
        className="absolute right-2 top-2 rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
        title={isExpanded ? t("common.collapse", "Collapse") : t("common.expand", "Expand")}
      >
        {isExpanded ? <IconMinimize className="h-4 w-4" /> : <IconMaximize className="h-4 w-4" />}
      </button>
    </div>
  )
}
