import { useState, useEffect, memo } from "react"
import { ImageIcon, Loader2 } from "lucide-react"
import { fetchThumbnail } from "@/api/endpoints"

interface LazyThumbnailProps {
  path: string
  fileName: string
  /** Icon shown while loading. */
  loadingIcon?: "image" | "spinner"
}

/**
 * Lazily fetches and renders a thumbnail via the JSON thumbnail API.
 * Shared by SmartSearchTile and ChatPanel thumbnails.
 */
export const LazyThumbnail = memo(function LazyThumbnail({
  path,
  fileName,
  loadingIcon = "image",
}: LazyThumbnailProps) {
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
        {loadingIcon === "spinner" ? (
          <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
        ) : (
          <ImageIcon className="h-8 w-8 text-muted-foreground" />
        )}
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
})
