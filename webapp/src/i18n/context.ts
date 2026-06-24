import { createContext } from "react"
import type { Language, TranslationKey } from "./types"

export interface I18nContextValue {
  language: Language
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}

export const I18nContext = createContext<I18nContextValue | null>(null)
