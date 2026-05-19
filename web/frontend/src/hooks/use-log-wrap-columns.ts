import { useEffect, useRef, useState } from "react"

const DEFAULT_WRAP_COLUMNS = 120
const MIN_WRAP_COLUMNS = 20
const RESIZE_DEBOUNCE_MS = 100

export function useLogWrapColumns() {
  const [wrapColumns, setWrapColumns] = useState(DEFAULT_WRAP_COLUMNS)
  const contentRef = useRef<HTMLDivElement>(null)
  const measureRef = useRef<HTMLSpanElement>(null)
  const debounceRef = useRef<number | null>(null)

  useEffect(() => {
    const content = contentRef.current
    const measure = measureRef.current

    if (!content || !measure) {
      return
    }

    const updateWrapColumns = () => {
      const contentWidth = content.clientWidth
      const charWidth = measure.getBoundingClientRect().width

      if (!contentWidth || !charWidth) {
        return
      }

      const nextColumns = Math.max(
        Math.floor(contentWidth / charWidth) - 1,
        MIN_WRAP_COLUMNS,
      )

      setWrapColumns((current) =>
        current === nextColumns ? current : nextColumns,
      )
    }

    // 使用防抖减少频繁更新导致的闪屏
    const debouncedUpdate = () => {
      if (debounceRef.current) {
        window.clearTimeout(debounceRef.current)
      }
      debounceRef.current = window.setTimeout(() => {
        updateWrapColumns()
        debounceRef.current = null
      }, RESIZE_DEBOUNCE_MS)
    }

    updateWrapColumns()

    const observer = new ResizeObserver(debouncedUpdate)
    observer.observe(content)

    return () => {
      observer.disconnect()
      if (debounceRef.current) {
        window.clearTimeout(debounceRef.current)
      }
    }
  }, [])

  return {
    contentRef,
    measureRef,
    wrapColumns,
  }
}
