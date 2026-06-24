import { translations } from "./translations"

export type Language = "en" | "ru"

export type TranslationKey = keyof typeof translations.en
