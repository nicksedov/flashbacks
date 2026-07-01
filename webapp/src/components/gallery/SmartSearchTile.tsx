import { useState, useEffect, memo } from "react"
import { ImageIcon } from "lucide-react"
import { fetchThumbnail } from "@/api/endpoints"
import { Badge } from "@/components/ui/badge"
import type { SmartSearchResult } from "@/types"
import { TileOverlay } from "./TileOverlay"
import { SelectionCheckbox } from "./SelectionCheckbox"

interface SmartSearchTileProps {
  result: SmartSearchResult
  onClick: (result: SmartSearchResult) => void
  onDownload?: (result: SmartSearchResult) => void
  onDelete?: (result: SmartSearchResult) => void
  selected?: boolean
  selectionModeActive?: boolean
  onSelectToggle?: (e: React.MouseEvent | React.KeyboardEvent, result: SmartSearchResult) => void
}

/** Lazily fetches and renders a thumbnail via the JSON thumbnail API. */
function LazyThumbnail({ path, fileName }: { path: string; fileName: string }) {
  const [src, setSrc] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    fetchThumbnail(path)
      .then((res) => {
        if (!cancelled) setSrc(res.thumbnail)
      })
      .catch(() => {
        // leave blank on error
      })
    return () => { cancelled = true }
  }, [path])

  if (!src) {
    return (
      <div className="w-full h-full flex items-center justify-center bg-muted">
        <ImageIcon className="h-8 w-8 text-muted-foreground" />
      </div>
    )
  }

  return (
    <img
      src={src}
      alt={fileName}
      className="w-full h-full object-cover"
    />
  )
}

export const SmartSearchTile = memo(function SmartSearchTile({
  result,
  onClick,
  onDownload,
  onDelete,
  selected,
  selectionModeActive,
  onSelectToggle,
}: SmartSearchTileProps) {
  const handleSelectToggle = (e: React.MouseEvent | React.KeyboardEvent) => {
    onSelectToggle?.(e, result)
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
        onClick(result)
      }}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onClick(result); } }}
    >
      <div
        className={`relative aspect-square overflow-hidden rounded-lg border bg-card transition-all ${
          selected ? "ring-2 ring-primary border-primary" : "hover:ring-2 hover:ring-ring"
        }`}
      >
        {/* Thumbnail */}
        <LazyThumbnail path={result.path} fileName={result.fileName} />

        {/* Similarity badge */}
        <Badge
          variant="secondary"
          className="absolute top-2 left-2 text-xs font-semibold bg-primary/90 text-primary-foreground"
        >
          {(result.similarity * 100).toFixed(0)}%
        </Badge>

        <SelectionCheckbox
          selected={!!selected}
          visible={(!!selected || !!selectionModeActive) && !!onSelectToggle}
          onToggle={handleSelectToggle}
        />

        <TileOverlay
          onDownload={onDownload ? () => onDownload(result) : undefined}
          onDelete={onDelete ? () => onDelete(result) : undefined}
        />
      </div>
      <p className="text-[11px] text-muted-foreground truncate mt-1 px-0.5 w-full text-center" title={result.fileName}>
        {result.fileName}
      </p>
    </div>
  )
})
