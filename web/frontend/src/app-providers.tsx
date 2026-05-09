import type { ReactNode } from "react"

import { ThemeProvider } from "./hooks/use-theme"
import { useHighlightTheme } from "./hooks/use-highlight-theme"

interface AppProvidersProps {
  children: ReactNode
}

export function AppProviders({ children }: AppProvidersProps) {
  useHighlightTheme()

  return (
    <ThemeProvider>
      {children}
    </ThemeProvider>
  )
}
