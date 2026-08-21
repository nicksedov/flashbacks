import { ImageOff } from "lucide-react"
import type { LucideIcon } from "lucide-react"
import { useTranslation } from "@/i18n"
import { cn } from "@/lib/utils"

interface EmptyStateProps {
  /** Icon shown above the title. Defaults to ImageOff. */
  icon?: LucideIcon
  /** Title text. Defaults to the `emptyState.title` translation key. */
  title?: string
  /** Description text. Defaults to the `emptyState.description` translation key. */
  description?: string
  /** Optional hint text shown below the description (e.g. a `*Hint` translation). */
  hint?: string
  /** Size variant controlling icon and vertical padding. */
  size?: "sm" | "md" | "lg"
  className?: string
}

const sizeStyles = {
  sm: { icon: "h-8 w-8", padding: "py-8", title: "text-base" },
  md: { icon: "h-16 w-16", padding: "py-16", title: "text-xl" },
  lg: { icon: "h-20 w-20", padding: "py-20", title: "text-2xl" },
} as const

export function EmptyState({
  icon: Icon = ImageOff,
  title,
  description,
  hint,
  size = "md",
  className,
}: EmptyStateProps) {
  const { t } = useTranslation()
  const styles = sizeStyles[size]

  return (
    <div className={cn("flex flex-col items-center justify-center text-center", styles.padding, className)}>
      <Icon className={cn("text-muted-foreground mb-4", styles.icon)} />
      <h2 className={cn("font-semibold mb-2", styles.title)}>{title ?? t("emptyState.title")}</h2>
      <p className="text-muted-foreground max-w-md">
        {description ?? t("emptyState.description")}
      </p>
      {hint && (
        <p className="mt-1 text-xs text-muted-foreground/70">
          {hint}
        </p>
      )}
    </div>
  )
}
