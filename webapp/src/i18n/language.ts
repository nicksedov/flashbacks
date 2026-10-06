import type { Language } from "./types"

/** Browser storage key for the pre-authentication language choice. */
export const AUTH_LANGUAGE_STORAGE_KEY = "flashbacks.authLanguage"

function isSupportedLanguage(value: unknown): value is Language {
  return value === "en" || value === "ru"
}

function readBrowserLocales(): readonly string[] {
  if (typeof navigator === "undefined") return []
  const languages = navigator.languages
  if (Array.isArray(languages) && languages.length > 0) return languages
  if (typeof navigator.language === "string" && navigator.language) {
    return [navigator.language]
  }
  return []
}

/**
 * Resolve a supported UI language from the browser/system locale.
 * Russian selects `ru`, English selects `en`, and any other locale falls back to `en`.
 */
export function resolveSystemLanguage(languages?: readonly string[]): Language {
  const candidates = languages ?? readBrowserLocales()
  const primary = candidates[0]
  if (typeof primary !== "string") return "en"
  const subtag = primary.toLowerCase().split("-")[0]
  if (subtag === "ru") return "ru"
  return "en"
}

/** Read the remembered pre-authentication language, or `null` when absent, unsupported, or unavailable. */
export function readStoredAuthLanguage(): Language | null {
  try {
    if (typeof localStorage === "undefined") return null
    const stored = localStorage.getItem(AUTH_LANGUAGE_STORAGE_KEY)
    return isSupportedLanguage(stored) ? stored : null
  } catch {
    return null
  }
}

/** Persist the pre-authentication language choice, ignoring storage failures. */
export function writeStoredAuthLanguage(language: Language): void {
  try {
    if (typeof localStorage === "undefined") return
    localStorage.setItem(AUTH_LANGUAGE_STORAGE_KEY, language)
  } catch {
    // Storage unavailable (for example private mode) - the choice is simply not remembered.
  }
}

/** The initial pre-authentication language: a remembered choice wins, otherwise the system locale. */
export function resolveInitialAuthLanguage(languages?: readonly string[]): Language {
  return readStoredAuthLanguage() ?? resolveSystemLanguage(languages)
}

/** Pick the active UI language: the account language when authenticated, otherwise the pre-auth language. */
export function selectActiveLanguage(
  isAuthenticated: boolean,
  accountLanguage: Language,
  authLanguage: Language
): Language {
  return isAuthenticated ? accountLanguage : authLanguage
}
