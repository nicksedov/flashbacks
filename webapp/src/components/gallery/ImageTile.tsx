import { memo } from "react"
import type { GalleryImageDTO } from "@/types"
import { TileFrame, TileThumbnail } from "./TileFrame"
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
  /** Image loading strategy: "lazy" (default) or "eager". Use "eager" when all thumbnails must load immediately (e.g., calendar view). */
  loading?: "lazy" | "eager"
}

export const ImageTile = memo(function ImageTile({
  image,
  onClick,
  onImageDownload,
  onImageDelete,
  selected,
  selectionModeActive,
  onSelectToggle,
  loading = "lazy",
}: ImageTileProps) {
  const handleSelectToggle = (e: React.MouseEvent | React.KeyboardEvent) => {
    onSelectToggle?.(e, image)
  }

  return (
    <TileFrame
      fileName={image.fileName}
      selected={selected}
      onClick={() => onClick(image)}
    >
      <TileThumbnail src={image.thumbnail} alt={image.fileName} loading={loading} />

      <SelectionCheckbox
        selected={!!selected}
        visible={(!!selected || !!selectionModeActive) && !!onSelectToggle}
        onToggle={handleSelectToggle}
      />

      <TileOverlay
        onDownload={onImageDownload ? () => onImageDownload(image) : undefined}
        onDelete={onImageDelete ? () => onImageDelete(image) : undefined}
      />
    </TileFrame>
  )
})
