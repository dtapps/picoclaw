import { Fragment, useMemo } from "react"

import { parseAnsiSegments, wrapLogLine } from "@/lib/ansi-log"

type AnsiLogLineProps = {
  line: string
  wrapColumns: number
}

export function AnsiLogLine({ line, wrapColumns }: AnsiLogLineProps) {
  const segments = useMemo(() => {
    return parseAnsiSegments(wrapLogLine(line, wrapColumns))
  }, [line, wrapColumns])

  return (
    <div className="break-normal whitespace-pre-wrap">
      {segments.map((segment, index) => (
        // 使用 index 作为 key，因为 segment 的顺序是稳定的
        <Fragment key={index}>
          <span style={segment.style}>{segment.text}</span>
        </Fragment>
      ))}
    </div>
  )
}
