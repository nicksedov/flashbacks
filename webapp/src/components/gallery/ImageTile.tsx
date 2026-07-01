import { memo } from "react"
import { useTranslation } from "@/i18n"
import type { GalleryImageDTO } from "@/types"
import { TileOverlay } from "./TileOverlay"
import { SelectionCheckbox } from "./SelectionCheckbox"

interface ImageTileProps {
  image: GalleryImageDTO
  onClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO) => void
  selected?: boolean
  selectionModeActive?: boolean
  onSelectToggle?: (e: React.MouseEvent | React.KeyboardEvent, image: GalleryImageDTO) => void
}

export const ImageTile = memo(function ImageTile({
  image,
  onClick,
  onImageDownload,
  onImageDelete,
  selected,
  selectionModeActive,
  onSelectToggle,
}: ImageTileProps) {
  const { t } = useTranslation()

  const handleSelectToggle = (e: React.MouseEvent | React.KeyboardEvent) => {
    onSelectToggle?.(e, image)
  }

  return (
    <div
      role="button"
      tabIndex={0}
      className="group flex flex-col cursor-pointer"
      onClick={(e) => {
        if (onSelectToggle && (e.ctrlKey || e.metaKey || e.shiftKey || selectionModeActive)) {
          handleSelectToggle(e)
          return
        }
        onClick(image)
      }}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onClick(image); } }}
    >
      <div
        className={`relative aspect-square overflow-hidden rounded-lg border bg-muted transition-all ${
          selected ? "ring-2 ring-primary border-primary" : "hover:ring-2 hover:ring-ring"
        }`}
      >
        {image.thumbnail ? (
          <img
            src={image.thumbnail}
            alt={image.fileName}
            className="h-full w-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
            {t("gallery.noPreview")}
          </div>
        )}

        <SelectionCheckbox
          selected={!!selected}
          visible={(!!selected || !!selectionModeActive) && !!onSelectToggle}
          onToggle={handleSelectToggle}
        />

        <TileOverlay
          onDownload={onImageDownload ? () => onImageDownload(image) : undefined}
          onDelete={onImageDelete ? () => onImageDelete(image) : undefined}
        />
      </div>
      <p className="text-[11px] text-muted-foreground truncate mt-1 px-0.5 w-full text-center" title={image.fileName}>
        {image.fileName}
      </p>
    </div>
  )
})
