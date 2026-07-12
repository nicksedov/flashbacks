import { useState, useEffect, useCallback } from "react"
import { fetchImageMetadata } from "@/api/endpoints"
import type { ImageMetadataDTO } from "@/types"

export function useImageMetadata(imagePath: string | null) {
  const [metadata, setMetadata] = useState<ImageMetadataDTO | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadMetadata = useCallback(async (path: string) => {
    setIsLoading(true)
    setError(null)
    try {
      const result = await fetchImageMetadata(path)
      if (result.found && result.metadata) {
        setMetadata(result.metadata)
      } else {
        setMetadata(null)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load metadata")
      setMetadata(null)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!imagePath) {
      // Derive state from imagePath being null — no effect needed for state reset
      return
    }
    let cancelled = false
    ;(async () => {
      try {
        const result = await fetchImageMetadata(imagePath)
        if (cancelled) return
        if (result.found && result.metadata) {
          setMetadata(result.metadata)
        } else {
          setMetadata(null)
        }
        setError(null)
      } catch (err) {
        if (cancelled) return
        setError(err instanceof Error ? err.message : "Failed to load metadata")
        setMetadata(null)
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [imagePath])

  return { metadata, isLoading, error, reload: () => imagePath && loadMetadata(imagePath) }
}
