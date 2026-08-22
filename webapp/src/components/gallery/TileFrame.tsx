import { type ReactNode } from "react"
import { useTranslation } from "@/i18n"

interface TileFrameProps {
  fileName: string
  /** Thumbnail image or custom content rendered in the tile body. */
  children: ReactNode
  onClick: () => void
  selected?: boolean
  /** Custom container class for the thumbnail area (defaults to gallery grid style). */
  className?: string
}

/**
 * Shared image tile frame: clickable aspect-square thumbnail, optional selection
 * ring, and a fileName caption. Used by ImageTile, ExifImageTile, and other tiles.
 */
export function TileFrame({ fileName, children, onClick, selected, className }: TileFrameProps) {
  return (
    <div
      role="button"
      tabIndex={0}
      className="group flex flex-col cursor-pointer"
      onClick={onClick}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onClick(); } }}
    >
      <div
        className={`relative aspect-square overflow-hidden rounded-lg border transition-all ${
          className ?? "bg-muted"
        } ${selected ? "ring-2 ring-primary border-primary" : "hover:ring-2 hover:ring-ring"}`}
      >
        {children}
      </div>
      <p className="text-[11px] text-muted-foreground truncate mt-1 px-0.5 w-full text-center" title={fileName}>
        {fileName}
      </p>
    </div>
  )
}

/** Default thumbnail body with "no preview" fallback. */
export function TileThumbnail({ src, alt, loading = "lazy" }: { src?: string; alt: string; loading?: "lazy" | "eager" }) {
  const { t } = useTranslation()

  if (src) {
    return <img src={src} alt={alt} className="h-full w-full object-cover" loading={loading} />
  }
  return (
    <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
      {t("gallery.noPreview")}
    </div>
  )
}
