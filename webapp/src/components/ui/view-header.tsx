import type { LucideIcon } from "lucide-react"
import { Loader2 } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { TranslationKey } from "@/i18n/types"

export interface ViewHeaderProps {
  /** Icon to display */
  icon: LucideIcon
  /** Translation key for the text */
  textKey: TranslationKey
  /** Translation values */
  textValues?: Record<string, string | number>
  /** Fallback text if no translation */
  fallbackText?: string
  /** Custom class for container */
  className?: string
  /** Custom class for icon */
  iconClassName?: string
  /** Custom class for text */
  textClassName?: string
  /** When true, display a loading spinner instead of the text */
  isLoading?: boolean
}

export function ViewHeader({
  icon: Icon,
  textKey,
  textValues,
  fallbackText,
  className = "flex items-center gap-2",
  iconClassName = "h-5 w-5 text-muted-foreground",
  textClassName = "text-sm text-muted-foreground",
  isLoading = false,
}: ViewHeaderProps) {
  const { t } = useTranslation()

  const text = textValues
    ? t(textKey, textValues)
    : t(textKey)

  return (
    <div className={className}>
      <Icon className={iconClassName} />
      {isLoading ? (
        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
      ) : (
        <span className={textClassName}>
          {text || fallbackText}
        </span>
      )}
    </div>
  )
}
