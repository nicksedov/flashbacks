import { ImageOff } from "lucide-react"
import { useTranslation } from "@/i18n"

export function EmptyState() {
  const { t } = useTranslation()

  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <ImageOff className="h-16 w-16 text-muted-foreground mb-4" />
      <h2 className="text-xl font-semibold mb-2">{t("emptyState.title")}</h2>
      <p className="text-muted-foreground max-w-md">
        {t("emptyState.description")}
      </p>
    </div>
  )
}
