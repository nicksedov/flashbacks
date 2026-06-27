import { memo } from "react"
import { useTranslation } from "@/i18n"
import { Download, Trash2, Check } from "lucide-react"
import type { GalleryImageDTO } from "@/types"

interface ImageTileProps {
  image: GalleryImageDTO
  onClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO) => void
  selected?: boolean
  selectionActiveInOtherFolder?: boolean
  onSelectToggle?: (e: React.MouseEvent | React.KeyboardEvent, image: GalleryImageDTO) => void
}

export const ImageTile = memo(function ImageTile({
  image,
  onClick,
  onImageDownload,
  onImageDelete,
  selected,
  selectionActiveInOtherFolder,
  onSelectToggle,
}: ImageTileProps) {
  const { t } = useTranslation()

  return (
    <div
      role="button"
      tabIndex={0}
      className="group flex flex-col cursor-pointer"
      onClick={() => onClick(image)}
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

        {/* Selection checkmark in top-right corner */}
        {selected !== undefined && onSelectToggle && (
          <button
            type="button"
            className={`absolute top-1 right-1 flex h-5 w-5 items-center justify-center rounded-full border-2 transition-colors z-10 ${
              selectionActiveInOtherFolder
                ? "bg-muted-foreground/10 border-muted-foreground/20 text-muted-foreground/30 cursor-not-allowed"
                : selected
                  ? "bg-primary border-primary text-primary-foreground"
                  : "bg-background/80 border-muted-foreground/30 hover:border-primary/60"
            }`}
            onClick={(e) => {
              if (selectionActiveInOtherFolder) return;
              e.stopPropagation()
              onSelectToggle(e, image)
            }}
            title={
              selectionActiveInOtherFolder
                ? t("gallery.selection.select")
                : selected
                  ? t("gallery.selection.unselect")
                  : t("gallery.selection.select")
            }
            disabled={selectionActiveInOtherFolder}
          >
            {selected && <Check className="h-3 w-3" />}
          </button>
        )}

        {/* Overlay with action buttons */}
        <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent p-2 opacity-0 group-hover:opacity-100 transition-opacity">
          <div className="flex gap-1 justify-center">
            {onImageDownload && (
              <button
                type="button"
                className="p-2 rounded-lg bg-white/10 hover:bg-white/20 text-white transition-colors"
                onClick={(e) => {
                  e.stopPropagation()
                  onImageDownload(image)
                }}
                title={t("gallery.overlay.download")}
              >
                <Download className="h-5 w-5" />
              </button>
            )}
            {onImageDelete && (
              <button
                type="button"
                className="p-2 rounded-lg bg-red-500/20 hover:bg-red-500/40 text-white transition-colors"
                onClick={(e) => {
                  e.stopPropagation()
                  onImageDelete(image)
                }}
                title={t("gallery.overlay.delete")}
              >
                <Trash2 className="h-5 w-5" />
              </button>
            )}
          </div>
        </div>
      </div>
      <p className="text-[11px] text-muted-foreground truncate mt-1 px-0.5 w-full text-center" title={image.fileName}>
        {image.fileName}
      </p>
    </div>
  )
})
