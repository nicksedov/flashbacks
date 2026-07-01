import { Download, Trash2 } from "lucide-react"
import { useTranslation } from "@/i18n"

interface TileOverlayProps {
  onDownload?: () => void
  onDelete?: () => void
}

/** Shared overlay with download and delete buttons, rendered at the bottom of a tile. */
export function TileOverlay({ onDownload, onDelete }: TileOverlayProps) {
  const { t } = useTranslation()

  return (
    <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent p-2 opacity-0 group-hover:opacity-100 transition-opacity">
      <div className="flex gap-1 justify-center">
        {onDownload && (
          <button
            type="button"
            className="p-2 rounded-lg bg-white/10 hover:bg-white/20 text-white transition-colors"
            onClick={(e) => {
              e.stopPropagation()
              onDownload()
            }}
            title={t("gallery.overlay.download")}
          >
            <Download className="h-5 w-5" />
          </button>
        )}
        {onDelete && (
          <button
            type="button"
            className="p-2 rounded-lg bg-red-500/20 hover:bg-red-500/40 text-white transition-colors"
            onClick={(e) => {
              e.stopPropagation()
              onDelete()
            }}
            title={t("gallery.overlay.delete")}
          >
            <Trash2 className="h-5 w-5" />
          </button>
        )}
      </div>
    </div>
  )
}
