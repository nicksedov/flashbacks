import { useCallback, useEffect, useRef } from "react"
import { fetchGalleryImages } from "@/api/endpoints"
import type { GalleryImageDTO, GalleryImagesResponse } from "@/types"
import { useInfiniteScroll } from "./useInfiniteScroll"

const PAGE_SIZE = 50

export function useGalleryImages(view: string, sortOrder: string = "newest", search?: string) {
  const viewRef = useRef(view)
  const sortOrderRef = useRef(sortOrder)
  const searchRef = useRef(search)

  const { items, total, hasMore, isLoading, error, initialized, loadMore, reset, removeItem } =
    useInfiniteScroll<GalleryImageDTO, GalleryImagesResponse>({
      fetchFn: (page, pageSize) => fetchGalleryImages(page, pageSize, viewRef.current, sortOrderRef.current, searchRef.current),
      pageSize: PAGE_SIZE,
      transform: (response) => response.images,
      responseTotal: (response) => response.totalImages,
      responseHasNext: (response) => response.hasNextPage,
    })

  // Reset when view, sortOrder, or search changes
  useEffect(() => {
    if (viewRef.current !== view || sortOrderRef.current !== sortOrder || searchRef.current !== search) {
      viewRef.current = view
      sortOrderRef.current = sortOrder
      searchRef.current = search
      reset()
    }
  }, [view, sortOrder, search, reset])

  const resetWithView = useCallback(
    (newView?: string, newSortOrder?: string, newSearch?: string) => {
      if (newView !== undefined) {
        viewRef.current = newView
      }
      if (newSortOrder !== undefined) {
        sortOrderRef.current = newSortOrder
      }
      if (newSearch !== undefined) {
        searchRef.current = newSearch
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
    reset: resetWithView,
    removeImage: removeItem,
  }
}
