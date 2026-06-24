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
      setMetadata(null)
      setIsLoading(false)
      setError(null)
      return
    }
    loadMetadata(imagePath)
  }, [imagePath, loadMetadata])

  return { metadata, isLoading, error, reload: () => imagePath && loadMetadata(imagePath) }
}
