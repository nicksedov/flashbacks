import { useCallback, useEffect } from "react"
import { fetchTrash } from "@/api/endpoints"
import type { TrashDateGroup, TrashListResponse } from "@/types"
import { useCursorInfiniteScroll } from "./useCursorInfiniteScroll"

export interface UseTrashItemsResult {
  groups: TrashDateGroup[]
  totalItems: number
  hasMore: boolean
  isLoading: boolean
  error: string | null
  initialized: boolean
  loadMore: () => Promise<void>
  reset: () => void
  removeItem: (id: number) => void
  removeItemById: (id: number) => void
}

/**
 * Infinite-scroll data hook for the Trash view.
 * Wraps useCursorInfiniteScroll over GET /api/trash, transforming the grouped
 * response into TrashDateGroup[] and merging a re-opened boundary date group
 * (so a date group is never split across pages).
 */
export function useTrashItems(): UseTrashItemsResult {
  const infiniteScroll = useCursorInfiniteScroll<TrashDateGroup, TrashListResponse>({
    fetchFn: (cursor) => fetchTrash(cursor),
    transform: (response) => response.groups,
    responseNextCursor: (response) => response.nextCursor ?? null,
    responseTotal: (response) => response.totalItems,
    mergeFn: (existing, incoming) => {
      if (existing.length === 0) return incoming
      const lastExisting = existing[existing.length - 1]
      const firstIncoming = incoming[0]
      // Merge if the boundary groups share the same date.
      if (lastExisting.date === firstIncoming.date) {
        const merged: TrashDateGroup = {
          ...lastExisting,
          items: [...lastExisting.items, ...firstIncoming.items],
          itemCount: lastExisting.itemCount + firstIncoming.itemCount,
        }
        return [...existing.slice(0, -1), merged, ...incoming.slice(1)]
      }
      return [...existing, ...incoming]
    },
  })

  // Initial load
  useEffect(() => {
    void infiniteScroll.loadMore()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const removeItemById = useCallback(
    (id: number) => {
      infiniteScroll.setItems((prevGroups) =>
        prevGroups
          .map((group) => ({
            ...group,
            items: group.items.filter((item) => item.id !== id),
            itemCount: group.items.filter((item) => item.id !== id).length,
          }))
          .filter((group) => group.items.length > 0)
      )
      infiniteScroll.setTotal((prev) => Math.max(0, prev - 1))
    },
    [infiniteScroll]
  )

  return {
    groups: infiniteScroll.items,
    totalItems: infiniteScroll.total,
    hasMore: infiniteScroll.hasMore,
    isLoading: infiniteScroll.isLoading,
    error: infiniteScroll.error,
    initialized: infiniteScroll.initialized,
    loadMore: infiniteScroll.loadMore,
    reset: infiniteScroll.reset,
    removeItem: removeItemById,
    removeItemById,
  }
}
