import { useEffect, type ReactNode } from "react"
import { ThemeContext, type Theme } from "./context"

interface ThemeProviderProps {
  theme: Theme
  children: ReactNode
}

export function ThemeProvider({ theme, children }: ThemeProviderProps) {
  useEffect(() => {
    const html = document.documentElement
    
    // Remove all theme-related classes
    html.classList.remove(
      "light-purple", "dark-purple",
      "light-green", "dark-green",
      "light-blue", "dark-blue",
      "light-orange", "dark-orange",
      "dark-contrast"
    )
    
    // Add the new theme class
    html.classList.add(theme)
  }, [theme])

  return (
    <ThemeContext.Provider value={{ theme }}>
      {children}
    </ThemeContext.Provider>
  )
}
