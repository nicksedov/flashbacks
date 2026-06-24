import { useCallback, useEffect, useRef, useState, type ReactNode } from "react"
import { ThemeProvider, type Theme } from "@/theme"
import { I18nProvider, type Language } from "@/i18n"
import { fetchUserSettings, updateUserSettings } from "@/api/endpoints"
import { SettingsContext } from "./settingsContext"
import { useAuth } from "./AuthProvider"

interface SettingsProviderProps {
  children: ReactNode
}

export function SettingsProvider({ children }: SettingsProviderProps) {
  const [theme, setThemeState] = useState<Theme>("light-purple")
  const [language, setLanguageState] = useState<Language>("en")
  const [trashDir, setTrashDirState] = useState("")
  const { isAuthenticated } = useAuth()
  const [isLoading, setIsLoading] = useState(!isAuthenticated)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const languageRef = useRef<Language>(language)
  languageRef.current = language
  const themeRef = useRef<Theme>(theme)
  themeRef.current = theme

  useEffect(() => {
    if (!isAuthenticated) {
      setIsLoading(false)
      return
    }

    // Theme migration mapping
    const themeMigration: Record<string, Theme> = {
      "light": "light-purple",
      "dark": "dark-purple",
    }

    // Load user settings for theme and language
    fetchUserSettings().catch(() => null).then((userSettings) => {
      let effectiveTheme = userSettings?.theme || "light-purple"
      
      // Migrate old theme values
      if (effectiveTheme in themeMigration) {
        effectiveTheme = themeMigration[effectiveTheme]
      }
      
      const effectiveLanguage = userSettings?.language || "en"

      setThemeState(effectiveTheme as Theme)
      setLanguageState(effectiveLanguage)
    }).catch(() => {
      // Use defaults on failure
    }).finally(() => setIsLoading(false))
  }, [isAuthenticated])

  const persistSettings = useCallback((newTheme: Theme, newLanguage: Language) => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current)
    }
    debounceRef.current = setTimeout(() => {
      const req: { theme?: string; language?: string } = {
        theme: newTheme,
        language: newLanguage,
      }
      updateUserSettings(req as any).catch(() => {
        // Silently fail - UI already updated
      })
    }, 300)
  }, [])

  const setTheme = useCallback(
    (newTheme: Theme) => {
      setThemeState(newTheme)
      persistSettings(newTheme, languageRef.current)
    },
    [persistSettings]
  )

  const toggleTheme = useCallback(() => {
    setThemeState((prev) => {
      const themeOrder: Theme[] = [
        "light-purple", "dark-purple",
        "light-green", "dark-green",
        "light-blue", "dark-blue",
        "light-orange", "dark-orange",
        "dark-contrast"
      ]
      const currentIndex = themeOrder.indexOf(prev)
      const nextIndex = (currentIndex + 1) % themeOrder.length
      const next = themeOrder[nextIndex]
      persistSettings(next, languageRef.current)
      return next
    })
  }, [persistSettings])

  const setLanguage = useCallback(
    (newLanguage: Language) => {
      setLanguageState(newLanguage)
      persistSettings(themeRef.current, newLanguage)
    },
    [persistSettings]
  )
  const setTrashDir = useCallback(
    (newTrashDir: string) => {
      setTrashDirState(newTrashDir)
    },
    []
  )

  return (
    <SettingsContext.Provider
      value={{ theme, setTheme, toggleTheme, language, setLanguage, trashDir, setTrashDir, isLoading }}
    >
      <ThemeProvider theme={theme}>
        <I18nProvider language={language}>
          {children}
        </I18nProvider>
      </ThemeProvider>
    </SettingsContext.Provider>
  )
}
