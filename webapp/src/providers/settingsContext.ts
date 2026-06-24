import { createContext } from "react"
import type { Theme } from "@/theme"
import type { Language } from "@/i18n"

export interface SettingsContextValue {
  theme: Theme
  setTheme: (theme: Theme) => void
  toggleTheme: () => void
  language: Language
  setLanguage: (language: Language) => void
  trashDir: string
  setTrashDir: (trashDir: string) => void
  isLoading: boolean
}

export const SettingsContext = createContext<SettingsContextValue | null>(null)
