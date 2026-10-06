import { createContext } from "react"
import type { Theme } from "@/theme"
import type { Language } from "@/i18n"

export interface SettingsContextValue {
  /** Active theme — the account theme when authenticated, otherwise the default theme. */
  theme: Theme
  setTheme: (theme: Theme) => void
  toggleTheme: () => void
  /** Display language of the authenticated account (persisted to user settings). */
  language: Language
  setLanguage: (language: Language) => void
  /** Display language used before authentication; independent of account settings. */
  authLanguage: Language
  setAuthLanguage: (language: Language) => void
  trashDir: string
  setTrashDir: (trashDir: string) => void
  isLoading: boolean
}

export const SettingsContext = createContext<SettingsContextValue | null>(null)
