import { memo } from "react"
import { CalendarX2, MapPinX } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { GalleryImageDTO } from "@/types"
import { TileFrame, TileThumbnail } from "./TileFrame"
import { TileOverlay } from "./TileOverlay"

interface ExifImageTileProps {
  image: GalleryImageDTO
  onClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO) => void
  onAddGeo?: (image: GalleryImageDTO) => void
}

export const ExifImageTile = memo(function ExifImageTile({
  image,
  onClick,
  onImageDownload,
  onImageDelete,
  onAddGeo,
}: ExifImageTileProps) {
  const { t } = useTranslation()

  return (
    <TileFrame
      fileName={image.fileName}
      onClick={() => onClick(image)}
    >
      <TileThumbnail src={image.thumbnail} alt={image.fileName} />

      {/* EXIF missing data indicators in top-left corner */}
      <div className="absolute top-1 left-1 flex gap-1">
        {image.missingDate && (
          <div
            className="flex items-center justify-center h-5 w-5 rounded bg-black/60 text-amber-400"
            title={t("exif.missingDate")}
          >
            <CalendarX2 className="h-3.5 w-3.5" />
          </div>
        )}
        {image.missingGps && (
          onAddGeo ? (
            <button
              type="button"
              className="flex items-center justify-center h-5 w-5 rounded bg-black/60 text-red-400 hover:bg-red-500/80 hover:text-white cursor-pointer transition-colors"
              title={t("exif.missingGps")}
              onClick={(e) => {
                e.stopPropagation()
                onAddGeo(image)
              }}
            >
              <MapPinX className="h-3.5 w-3.5" />
            </button>
          ) : (
            <div
              className="flex items-center justify-center h-5 w-5 rounded bg-black/60 text-red-400"
              title={t("exif.missingGps")}
            >
              <MapPinX className="h-3.5 w-3.5" />
            </div>
          )
        )}
      </div>

      {/* Overlay with action buttons */}
      <TileOverlay
        onDownload={onImageDownload ? () => onImageDownload(image) : undefined}
        onDelete={onImageDelete ? () => onImageDelete(image) : undefined}
      />
    </TileFrame>
  )
})
