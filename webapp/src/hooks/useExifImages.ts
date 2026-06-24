import { useCallback, useEffect } from "react"
import { fetchImagesMissingExif } from "@/api/endpoints"
import type { GalleryImageDTO, GalleryImagesResponse } from "@/types"
import { useInfiniteScroll } from "./useInfiniteScroll"

const PAGE_SIZE = 50

export function useExifImages() {
  const { items, total, hasMore, isLoading, error, initialized, loadMore, reset, removeItem } =
    useInfiniteScroll<GalleryImageDTO, GalleryImagesResponse>({
      fetchFn: (page, pageSize) => fetchImagesMissingExif(page, pageSize),
      pageSize: PAGE_SIZE,
      transform: (response) => response.images,
      responseTotal: (response) => response.totalImages,
      responseHasNext: (response) => response.hasNextPage,
    })

  useEffect(() => {
    if (!initialized && !isLoading) {
      loadMore()
    }
  }, [initialized, isLoading, loadMore])

  const resetImages = useCallback(() => {
    reset()
  }, [reset])

  return {
    images: items,
    totalImages: total,
    hasMore,
    isLoading,
    error,
    initialized,
    loadMore,
    reset: resetImages,
    removeImage: removeItem,
  }
}
