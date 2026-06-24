import { useEffect } from "react"
import { useTranslation } from "@/i18n"
import { FileWarning } from "lucide-react"
import { Skeleton } from "@/components/ui/skeleton"
import { ViewHeader } from "@/components/ui/view-header"
import { PaginationFooter } from "@/components/ui/pagination-footer"
import { useIntersectionObserver } from "@/hooks/useIntersectionObserver"
import { useExifImages } from "@/hooks/useExifImages"
import type { GalleryImageDTO } from "@/types"
import { ExifImageGrid } from "./ExifImageGrid"

interface ExifFoldersViewProps {
  onImageClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO, removeThumbnail: () => void) => void
  onAddGeo?: (image: GalleryImageDTO) => void
}

export function ExifFoldersView({ onImageClick, onImageDownload, onImageDelete, onAddGeo }: ExifFoldersViewProps) {
  const { images, totalImages, hasMore, isLoading, error, initialized, loadMore, removeImage } = useExifImages()
  const { t } = useTranslation()

  const sentinelRef = useIntersectionObserver({
    onIntersect: loadMore,
    enabled: hasMore && !isLoading,
    dependencies: [hasMore, isLoading, loadMore],
  })

  useEffect(() => {
    if (!initialized && !isLoading) {
      loadMore()
    }
  }, [initialized, isLoading, loadMore])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <ViewHeader
          icon={FileWarning}
          textKey={totalImages === 1 ? "exif.imageCountOne" : "exif.imageCount"}
          textValues={{ count: totalImages.toLocaleString() }}
          isLoading={!initialized}
        />
      </div>

      {error && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      )}

      {!initialized ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-40 w-full rounded-lg" />
          ))}
        </div>
      ) : images.length === 0 && !isLoading ? (
        <div className="rounded-lg border border-dashed p-12 text-center">
          <FileWarning className="mx-auto h-10 w-10 text-muted-foreground/50" />
          <p className="mt-2 text-sm font-medium text-muted-foreground">
            {t("exif.empty")}
          </p>
          <p className="text-xs text-muted-foreground/70">
            {t("exif.emptyHint")}
          </p>
        </div>
      ) : (
        <>
          <ExifImageGrid
            images={images}
            onImageClick={onImageClick}
            onImageDownload={onImageDownload}
            onImageDelete={(image) => onImageDelete?.(image, () => removeImage(image.id))}
            onAddGeo={onAddGeo}
          />

          <div ref={sentinelRef} className="h-4" />

          <PaginationFooter
            isLoading={isLoading}
            hasMore={hasMore}
            totalCount={totalImages}
          />
        </>
      )}
    </div>
  )
}
