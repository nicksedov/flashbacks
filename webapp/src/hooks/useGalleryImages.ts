import { useCallback, useEffect, useRef } from "react"
import { fetchGalleryImages } from "@/api/endpoints"
import type { GalleryImageDTO, GalleryImagesResponse } from "@/types"
import { useInfiniteScroll } from "./useInfiniteScroll"

const PAGE_SIZE = 50

export function useGalleryImages(view: string, sortOrder: string = "newest", search?: string, dirPath?: string) {
  const viewRef = useRef(view)
  const sortOrderRef = useRef(sortOrder)
  const searchRef = useRef(search)
  const dirPathRef = useRef(dirPath)

  const { items, total, hasMore, isLoading, error, initialized, loadMore, reset, removeItem } =
    useInfiniteScroll<GalleryImageDTO, GalleryImagesResponse>({
      fetchFn: (page, pageSize) => fetchGalleryImages(page, pageSize, viewRef.current, sortOrderRef.current, searchRef.current, dirPathRef.current),
      pageSize: PAGE_SIZE,
      transform: (response) => response.images,
      responseTotal: (response) => response.totalImages,
      responseHasNext: (response) => response.hasNextPage,
    })

  // Reset when view, sortOrder, search, or dirPath changes
  useEffect(() => {
    if (viewRef.current !== view || sortOrderRef.current !== sortOrder || searchRef.current !== search || dirPathRef.current !== dirPath) {
      viewRef.current = view
      sortOrderRef.current = sortOrder
      searchRef.current = search
      dirPathRef.current = dirPath
      reset()
    }
  }, [view, sortOrder, search, dirPath, reset])

  const resetWithView = useCallback(
    (newView?: string, newSortOrder?: string, newSearch?: string, newDirPath?: string) => {
      if (newView !== undefined) {
        viewRef.current = newView
      }
      if (newSortOrder !== undefined) {
        sortOrderRef.current = newSortOrder
      }
      if (newSearch !== undefined) {
        searchRef.current = newSearch
      }
      if (newDirPath !== undefined) {
        dirPathRef.current = newDirPath
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
