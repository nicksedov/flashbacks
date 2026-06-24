import { Loader2, Wand2, Tags, Sparkles } from "lucide-react"
import { useTranslation } from "@/i18n"
import { Button } from "@/components/ui/button"
import type { TagsStateData } from "@/hooks/useTagsState"

interface TagsPanelProps {
  tagsData: TagsStateData | null
  loading: boolean
  generating: boolean
  error: string | null
  onGenerate: () => void
  formatProcessingTime: (ms?: number) => string
  className?: string
}

export function TagsPanel({
  tagsData,
  loading,
  generating,
  error,
  onGenerate,
  formatProcessingTime,
  className,
}: TagsPanelProps) {
  const { t } = useTranslation()
  const panelClass = className ?? "w-full bg-card p-4 h-full flex flex-col"

  if (loading) {
    return (
      <div className={panelClass}>
        <div className="flex flex-col items-center justify-center h-full">
          <Loader2 className="h-8 w-8 animate-spin text-primary mb-3" />
          <p className="text-sm font-medium">{t("tags.loading")}</p>
        </div>
      </div>
    )
  }

  if (generating) {
    return (
      <div className={panelClass}>
        <div className="flex flex-col items-center justify-center h-full">
          <Loader2 className="h-8 w-8 animate-spin text-primary mb-3" />
          <p className="text-sm font-medium">{t("tags.generating")}</p>
        </div>
      </div>
    )
  }

  const hasTags = tagsData && tagsData.tags.length > 0
  const hasMeta = tagsData?.provider || tagsData?.model || tagsData?.processingTimeMs !== undefined

  if (hasTags) {
    return (
      <div className={panelClass}>
        <div className="h-full overflow-y-auto">
          <div className="space-y-4">
            <h3 className="text-sm font-semibold">{t("tags.title")}</h3>

            {/* Tags section */}
            <div>
              <div className="flex items-center gap-1.5 mb-2">
                <Tags className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("tags.sectionTags")}</span>
                <span className="text-xs text-muted-foreground ml-auto">{t("tags.count", { count: tagsData!.tags.length })}</span>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {tagsData!.tags.map((tag) => (
                  <span
                    key={tag}
                    className="inline-flex items-center rounded-md bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            </div>

            {/* Generation info section */}
            {hasMeta && (
              <div>
                <div className="flex items-center gap-1.5 mb-2">
                  <Sparkles className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("tags.sectionResult")}</span>
                </div>
                <div className="space-y-1.5">
                  {tagsData!.provider && (
                    <div className="flex justify-between items-baseline gap-2 text-xs">
                      <span className="text-muted-foreground shrink-0">{t("tags.provider")}</span>
                      <span className="font-medium text-right truncate">{tagsData!.provider}</span>
                    </div>
                  )}
                  {tagsData!.model && (
                    <div className="flex justify-between items-baseline gap-2 text-xs">
                      <span className="text-muted-foreground shrink-0">{t("tags.model")}</span>
                      <span className="font-medium text-right truncate">{tagsData!.model}</span>
                    </div>
                  )}
                  {tagsData!.processingTimeMs !== undefined && (
                    <div className="flex justify-between items-baseline gap-2 text-xs">
                      <span className="text-muted-foreground shrink-0">{t("tags.processingTime")}</span>
                      <span className="font-medium text-right truncate">{formatProcessingTime(tagsData!.processingTimeMs)}</span>
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Regenerate button */}
            <Button variant="outline" size="sm" className="w-full text-xs" onClick={onGenerate}>
              <Wand2 className="h-3.5 w-3.5 mr-1.5" />
              {t("tags.regenerateButton")}
            </Button>
          </div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className={panelClass}>
        <div className="h-full overflow-y-auto">
          <div className="space-y-4">
            <h3 className="text-sm font-semibold">{t("tags.title")}</h3>
            <p className="text-xs text-destructive">{error}</p>
            <Button variant="outline" size="sm" className="w-full text-xs" onClick={onGenerate}>
              <Wand2 className="h-3.5 w-3.5 mr-1.5" />
              {t("tags.generateButton")}
            </Button>
          </div>
        </div>
      </div>
    )
  }

  // Default: no tags yet
  return (
    <div className={panelClass}>
      <div className="h-full overflow-y-auto">
        <div className="space-y-4">
          <h3 className="text-sm font-semibold">{t("tags.title")}</h3>

          <Button variant="outline" size="sm" className="w-full text-xs" onClick={onGenerate}>
            <Wand2 className="h-3.5 w-3.5 mr-1.5" />
            {t("tags.generateButton")}
          </Button>

          <p className="text-xs text-muted-foreground text-center">
            {t("tags.description")}
          </p>
        </div>
      </div>
    </div>
  )
}
