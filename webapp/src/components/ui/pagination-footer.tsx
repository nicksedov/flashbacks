import { Loader2 } from "lucide-react"
import { useTranslation } from "@/i18n"

export interface PaginationFooterProps {
  /** Whether currently loading more items */
  isLoading: boolean
  /** Whether there are more items to load */
  hasMore: boolean
  /** Total count of loaded items */
  totalCount: number
  /** Custom loading text (overrides translation) */
  loadingText?: string
  /** Custom "all loaded" text (overrides translation) */
  allLoadedText?: string
  /** Custom class for loading state */
  loadingClassName?: string
  /** Custom class for all loaded state */
  allLoadedClassName?: string
}

export function PaginationFooter({
  isLoading,
  hasMore,
  totalCount,
  loadingText,
  allLoadedText,
  loadingClassName = "flex justify-center py-4",
  allLoadedClassName = "text-center text-xs text-muted-foreground py-4",
}: PaginationFooterProps) {
  const { t } = useTranslation()

  if (!isLoading && (hasMore || totalCount === 0)) {
    return null
  }

  return (
    <>
      {isLoading && (
        <div className={loadingClassName}>
          <div className="text-sm text-muted-foreground flex items-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            {loadingText || t("gallery.loadingMore")}
          </div>
        </div>
      )}

      {!hasMore && totalCount > 0 && (
        <div className={allLoadedClassName}>
          {allLoadedText || t("gallery.allLoaded", { count: totalCount.toLocaleString() })}
        </div>
      )}
    </>
  )
}
