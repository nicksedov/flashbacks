import { useState, useEffect, useRef, memo } from "react"
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
 * Shared by SmartSearchTile, ChatPanel thumbnails and the People grid.
 *
 * The fetch starts only when the tile approaches the viewport: a People page
 * can hold hundreds of tiles, and fetching every thumbnail at once swamped the
 * API (and each request validates the session).
 */
export const LazyThumbnail = memo(function LazyThumbnail({
  path,
  fileName,
  loadingIcon = "image",
}: LazyThumbnailProps) {
  const [src, setSrc] = useState<string | null>(null)
  const [visible, setVisible] = useState(false)
  const containerRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const element = containerRef.current
    // Older browsers (and jsdom in tests) lack IntersectionObserver.
    if (!element || typeof IntersectionObserver === "undefined") {
      setVisible(true)
      return
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setVisible(true)
          observer.disconnect()
        }
      },
      { rootMargin: "200px" },
    )
    observer.observe(element)
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    if (!visible) return
    let cancelled = false
    fetchThumbnail(path)
      .then((res) => {
        if (!cancelled) setSrc(res.thumbnail)
      })
      .catch(() => {
        // leave blank on error
      })
    return () => { cancelled = true }
  }, [visible, path])

  return (
    <div ref={containerRef} className="w-full h-full">
      {src ? (
        <img
          src={src}
          alt={fileName}
          className="w-full h-full object-cover"
        />
      ) : (
        <div className="w-full h-full flex items-center justify-center bg-muted">
          {loadingIcon === "spinner" ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
          ) : (
            <ImageIcon className="h-8 w-8 text-muted-foreground" />
          )}
        </div>
      )}
    </div>
  )
})
