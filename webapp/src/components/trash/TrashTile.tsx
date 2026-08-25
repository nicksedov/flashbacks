import { memo } from "react"
import { Eye, RotateCcw } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { TrashItemDTO } from "@/types"
import { TileFrame, TileThumbnail } from "@/components/gallery/TileFrame"

interface TrashTileOverlayProps {
  onView: () => void
  onRestore: () => void
}

/** Hover overlay with exactly two actions: View and Restore. */
function TrashTileOverlay({ onView, onRestore }: TrashTileOverlayProps) {
  const { t } = useTranslation()

  return (
    <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent p-2 opacity-0 group-hover:opacity-100 transition-opacity">
      <div className="flex gap-1 justify-center">
        <button
          type="button"
          className="p-2 rounded-lg bg-white/10 hover:bg-white/20 text-white transition-colors"
          onClick={(e) => {
            e.stopPropagation()
            onView()
          }}
          title={t("trashTab.view")}
          aria-label={t("trashTab.view")}
        >
          <Eye className="h-5 w-5" />
        </button>
        <button
          type="button"
          className="p-2 rounded-lg bg-emerald-500/20 hover:bg-emerald-500/40 text-white transition-colors"
          onClick={(e) => {
            e.stopPropagation()
            onRestore()
          }}
          title={t("trashTab.restore")}
          aria-label={t("trashTab.restore")}
        >
          <RotateCcw className="h-5 w-5" />
        </button>
      </div>
    </div>
  )
}

interface TrashTileProps {
  item: TrashItemDTO
  onView: (item: TrashItemDTO) => void
  onRestore: (item: TrashItemDTO) => void
  loading?: "lazy" | "eager"
}

export const TrashTile = memo(function TrashTile({ item, onView, onRestore, loading = "lazy" }: TrashTileProps) {
  return (
    <TileFrame
      fileName={item.fileName}
      onClick={() => onView(item)}
    >
      <TileThumbnail src={item.thumbnail} alt={item.fileName} loading={loading} />
      <TrashTileOverlay
        onView={() => onView(item)}
        onRestore={() => onRestore(item)}
      />
    </TileFrame>
  )
})
