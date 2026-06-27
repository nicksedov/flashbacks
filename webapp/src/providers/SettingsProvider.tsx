import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { ThemeProvider, type Theme } from "@/theme"
import { I18nProvider, type Language } from "@/i18n"
import { fetchUserSettings, fetchSettings, updateUserSettings } from "@/api/endpoints"
import type { UpdateUserSettingsRequest } from "@/types"
import { SettingsContext } from "./settingsContext"
import { useAuth } from "./AuthProvider"

interface SettingsProviderProps {
  children: ReactNode
}

export function SettingsProvider({ children }: SettingsProviderProps) {
  const [theme, setThemeState] = useState<Theme>("light-purple")
  const [language, setLanguageState] = useState<Language>("en")
  const [trashDir, setTrashDirState] = useState("")
  const { isAuthenticated, isLoading: isAuthLoading } = useAuth()
  const [settingsFetched, setSettingsFetched] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const languageRef = useRef<Language>(language)
  const themeRef = useRef<Theme>(theme)

  // Derived: loading while auth is resolving OR settings haven't been fetched yet for an authenticated user
  const isLoading = useMemo(
    () => isAuthLoading || (isAuthenticated && !settingsFetched),
    [isAuthLoading, isAuthenticated, settingsFetched]
  )

  useEffect(() => {
    languageRef.current = language
    themeRef.current = theme
  }, [language, theme])

  useEffect(() => {
    // Wait for auth to resolve; when not authenticated, isLoading
    // derived value already resolves to false — nothing to fetch.
    if (isAuthLoading || !isAuthenticated) return

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
    })

    // Load app settings for trashDir
    fetchSettings()
      .then((appSettings) => {
        if (appSettings?.trashDir) {
          setTrashDirState(appSettings.trashDir)
        }
      })
      .catch(() => {
        // Use default (empty) on failure
      })
      .finally(() => setSettingsFetched(true))
  }, [isAuthenticated, isAuthLoading])

  const persistSettings = useCallback((newTheme: Theme, newLanguage: Language) => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current)
    }
    debounceRef.current = setTimeout(() => {
      const req: UpdateUserSettingsRequest = {
        theme: newTheme,
        language: newLanguage,
      }
      updateUserSettings(req).catch(() => {
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
