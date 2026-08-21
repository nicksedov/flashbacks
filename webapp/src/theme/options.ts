import { Sun, Moon } from "lucide-react"
import type { LucideIcon } from "lucide-react"
import type { TranslationKey } from "@/i18n"
import type { Theme } from "./context"

export interface ThemeOption {
  id: Theme
  labelKey: TranslationKey
  icon: LucideIcon
  iconClassName: string
  swatch: string
}

/** Theme picker options — single source of truth for the settings theme Select. */
export const THEMES: ThemeOption[] = [
  { id: "light-purple", labelKey: "settings.lightPurpleTheme", icon: Sun, iconClassName: "text-yellow-500", swatch: "bg-purple-200" },
  { id: "dark-purple", labelKey: "settings.darkPurpleTheme", icon: Moon, iconClassName: "text-blue-400", swatch: "bg-purple-900" },
  { id: "light-green", labelKey: "settings.lightGreenTheme", icon: Sun, iconClassName: "text-yellow-500", swatch: "bg-green-200" },
  { id: "dark-green", labelKey: "settings.darkGreenTheme", icon: Moon, iconClassName: "text-blue-400", swatch: "bg-green-900" },
  { id: "light-blue", labelKey: "settings.lightBlueTheme", icon: Sun, iconClassName: "text-yellow-500", swatch: "bg-blue-200" },
  { id: "dark-blue", labelKey: "settings.darkBlueTheme", icon: Moon, iconClassName: "text-blue-400", swatch: "bg-blue-900" },
  { id: "light-orange", labelKey: "settings.lightOrangeTheme", icon: Sun, iconClassName: "text-yellow-500", swatch: "bg-orange-200" },
  { id: "dark-orange", labelKey: "settings.darkOrangeTheme", icon: Moon, iconClassName: "text-blue-400", swatch: "bg-orange-900" },
  { id: "dark-contrast", labelKey: "settings.darkContrastTheme", icon: Moon, iconClassName: "text-blue-400", swatch: "bg-gray-900" },
]
