import { useCallback, useEffect, useRef, useState } from "react"
import { fetchGalleryClusters } from "@/api/endpoints"
import type { GeoCluster } from "@/types"

interface UseGeoClustersParams {
  bounds: { minLat: number; maxLat: number; minLng: number; maxLng: number } | null
  zoom: number
  width: number
  height: number
}

interface UseGeoClustersReturn {
  clusters: GeoCluster[]
  totalImages: number
  isLoading: boolean
  error: string | null
  initialized: boolean
  refetch: () => void
}

export function useGeoClusters({ bounds, zoom, width, height }: UseGeoClustersParams): UseGeoClustersReturn {
  const [clusters, setClusters] = useState<GeoCluster[]>([])
  const [totalImages, setTotalImages] = useState(0)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [refetchKey, setRefetchKey] = useState(0)
  const [initialized, setInitialized] = useState(false)

  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  const fetchClusters = useCallback(async () => {
    if (!bounds) {
      setClusters([])
      setTotalImages(0)
      setIsLoading(false)
      setInitialized(true)
      return
    }

    // Cancel previous request
    abortControllerRef.current?.abort()
    const controller = new AbortController()
    abortControllerRef.current = controller

    setIsLoading(true)
    setError(null)
    try {
      const result = await fetchGalleryClusters({
        minLat: bounds.minLat,
        maxLat: bounds.maxLat,
        minLng: bounds.minLng,
        maxLng: bounds.maxLng,
        zoom,
        width,
        height,
      }, controller.signal)
      setClusters(result.clusters)
      setTotalImages(result.totalImages)
      setInitialized(true)
    } catch (err) {
      if (err instanceof Error && err.name === "AbortError") return
      setError(err instanceof Error ? err.message : "Failed to fetch clusters")
      setClusters([])
      setTotalImages(0)
      setInitialized(true)
    } finally {
      setIsLoading(false)
    }
  }, [bounds, zoom, width, height])

  // Debounced fetch on bounds/zoom/size change
  useEffect(() => {
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current)
    }

    debounceTimerRef.current = setTimeout(() => {
      fetchClusters()
    }, 300)

    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current)
      }
      abortControllerRef.current?.abort()
    }
  }, [fetchClusters, refetchKey])

  const refetch = useCallback(() => {
    setRefetchKey((k) => k + 1)
  }, [])

  return { clusters, totalImages, isLoading, error, initialized, refetch }
}
