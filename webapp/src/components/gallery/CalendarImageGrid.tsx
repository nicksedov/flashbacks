import { Calendar, MapPinPlus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useTranslation } from "@/i18n"
import type { CalendarDateGroup } from "@/types"
import type { GalleryImageDTO } from "@/types"
import { ImageTile } from "./ImageTile"

interface CalendarImageGridProps {
  groups: CalendarDateGroup[]
  onImageClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO, removeThumbnail: () => void) => void
  onBulkGeo?: (group: CalendarDateGroup) => void
}

export function CalendarImageGrid({ groups, onImageClick, onImageDownload, onImageDelete, onBulkGeo }: CalendarImageGridProps) {
  const { t } = useTranslation()

  const handleDelete = (image: GalleryImageDTO) => {
    onImageDelete?.(image, () => {
      // Removal from parent state is handled by the parent callback (calendar.removeImage)
    })
  }

  return (
    <div className="space-y-5">
      {groups.map((group) => {
        const missingGpsCount = group.images.filter((img) => img.missingGps).length
        const allHaveGps = missingGpsCount === 0

        return (
          <div key={group.date} id={`date-group-${group.date}`} className="mb-6">
            <div className="flex items-center gap-2 mb-2 px-0.5">
              <Calendar className="h-4 w-4 text-muted-foreground shrink-0" />
              <span className="text-sm font-medium">{group.label}</span>
              <span className="text-xs text-muted-foreground shrink-0">
                ({group.imageCount})
              </span>
              <div className="ml-auto">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs gap-1"
                  disabled={allHaveGps}
                  title={allHaveGps ? t("geo.allHaveLocation") : t("geo.photosWithoutLocation", { count: missingGpsCount })}
                  onClick={() => onBulkGeo?.(group)}
                >
                  <MapPinPlus className="h-3.5 w-3.5" />
                  <span className="hidden sm:inline">{t("geo.bulkSetButton")}</span>
                </Button>
              </div>
            </div>
            <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-7 xl:grid-cols-8 gap-1.5">
              {group.images.map((image) => (
                <ImageTile
                  key={image.id}
                  image={image}
                  onClick={onImageClick}
                  onImageDownload={onImageDownload}
                  onImageDelete={handleDelete}
                />
              ))}
            </div>
          </div>
        )
      })}
    </div>
  )
}
