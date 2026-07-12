import { useCallback, useEffect, useRef } from "react"
import { fetchGeoImages } from "@/api/endpoints"
import type { GalleryImageDTO, GeoImagesResponse } from "@/types"
import { useInfiniteScroll } from "./useInfiniteScroll"

const PAGE_SIZE = 50

export function useGeoImages(clusterId: string | null) {
  const clusterIdRef = useRef(clusterId)

  // Keep clusterIdRef in sync
  useEffect(() => {
    clusterIdRef.current = clusterId
  }, [clusterId])

  const { items, total, hasMore, isLoading, error, initialized, loadMore, reset, removeItem } =
    useInfiniteScroll<GalleryImageDTO, GeoImagesResponse>({
      fetchFn: (page, pageSize) => {
        if (!clusterIdRef.current) {
          return Promise.resolve({
            images: [],
            totalImages: 0,
            currentPage: page,
            pageSize,
            totalPages: 0,
            hasNextPage: false,
          })
        }
        return fetchGeoImages(page, pageSize, { clusterId: clusterIdRef.current })
      },
      pageSize: PAGE_SIZE,
      transform: (response) => response.images,
      responseTotal: (response) => response.totalImages,
      responseHasNext: (response) => response.hasNextPage,
    })

  const resetWithClusterId = useCallback(
    (newClusterId?: string | null) => {
      if (newClusterId !== undefined) {
        clusterIdRef.current = newClusterId
      }
      reset()
    },
    [reset]
  )

  return {
    images: items,
    totalImages: total,
    hasMore,
    isLoading,
    error,
    initialized,
    loadMore,
    reset: resetWithClusterId,
    removeImage: removeItem,
  }
}
