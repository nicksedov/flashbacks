import type { LucideIcon } from "lucide-react"
import type { TranslationKey } from "@/i18n"
import { useTranslation } from "@/i18n"

interface UnderlineTab<T extends string> {
  id: T
  labelKey: TranslationKey
  icon: LucideIcon
}

interface UnderlineTabsProps<T extends string> {
  tabs: UnderlineTab<T>[]
  value: T
  onValueChange: (value: T) => void
  className?: string
}

export function UnderlineTabs<T extends string>({
  tabs,
  value,
  onValueChange,
  className = "",
}: UnderlineTabsProps<T>) {
  const { t } = useTranslation()

  return (
    <div className={`flex pt-2 shrink-0 ${className}`}>
      {tabs.map(({ id, labelKey, icon: Icon }) => (
        <button
          key={id}
          type="button"
          className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors ${
            value === id
              ? "text-primary border-b-2 border-primary"
              : "text-muted-foreground hover:text-foreground"
          }`}
          onClick={() => onValueChange(id)}
        >
          <Icon className="h-3.5 w-3.5" />
          {t(labelKey)}
        </button>
      ))}
    </div>
  )
}
