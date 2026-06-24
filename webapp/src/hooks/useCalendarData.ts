import { useCallback, useEffect, useRef, useState } from "react"
import { fetchGalleryCalendar, fetchCalendarSeek } from "@/api/endpoints"
import { useCursorInfiniteScroll } from "./useCursorInfiniteScroll"
import type { CalendarDateGroup, CalendarDateRange, GalleryCalendarResponse } from "@/types"

const PAGE_SIZE = 50

export interface UseCalendarDataOptions {
  initialMonthYear: string
}

export interface UseCalendarDataResult {
  // From useCursorInfiniteScroll
  groups: CalendarDateGroup[]
  totalImages: number
  hasMore: boolean
  isLoading: boolean
  error: string | null
  initialized: boolean

  // Calendar-specific state
  dateRange: CalendarDateRange
  dateRangeFilter: { start: string | null; end: string | null }
  sortOrder: "oldest" | "newest"

  // Actions
  setDateRangeFilter: (filter: { start: string | null; end: string | null }) => void
  setSortOrder: (order: "oldest" | "newest") => void
  toggleSortOrder: () => void
  clearDateRangeFilter: () => void
  setMonthYear: (monthYear: string) => void
  jumpToDate: (date: string) => Promise<void>
  removeImage: (imageId: number) => void
  updateGroupGpsStatus: (date: string) => void
  loadMore: () => Promise<void>
  reset: () => void
}

/**
 * Custom hook for calendar data with cursor-based pagination.
 * Wraps useCursorInfiniteScroll with calendar-specific logic for filters, sorting, and date jumping.
 */
export function useCalendarData({ initialMonthYear }: UseCalendarDataOptions): UseCalendarDataResult {
  // Filter and sort state
  const [dateRangeFilter, setDateRangeFilter] = useState<{ start: string | null; end: string | null }>({
    start: null,
    end: null,
  })
  const [sortOrder, setSortOrder] = useState<"oldest" | "newest">("oldest")
  const [dateRange, setDateRange] = useState<CalendarDateRange>({ minDate: "", maxDate: "", totalWithDate: 0 })

  // Refs for dynamic values (avoid re-creating fetchFn on every change)
  const dateRangeFilterRef = useRef(dateRangeFilter)
  const monthYearRef = useRef(initialMonthYear)
  const sortOrderRef = useRef(sortOrder)
  const dateRangeCapturedRef = useRef(false) // Track if we've captured dateRange from first response
  const jumpCursorRef = useRef<string | null>(null) // Stores cursor for jump-to-date

  // Sync refs with state
  useEffect(() => {
    dateRangeFilterRef.current = dateRangeFilter
  }, [dateRangeFilter])

  useEffect(() => {
    sortOrderRef.current = sortOrder
  }, [sortOrder])

  // Core infinite scroll hook
  const infiniteScroll = useCursorInfiniteScroll<CalendarDateGroup, GalleryCalendarResponse>({
    fetchFn: async (cursor) => {
      // Use jump cursor if available and no cursor was passed
      const actualCursor = cursor || jumpCursorRef.current
      if (jumpCursorRef.current && !cursor) {
        // Clear jump cursor after first use
        jumpCursorRef.current = null
      }

      const result = await fetchGalleryCalendar(
        1, // page is ignored when cursor is used
        PAGE_SIZE,
        dateRangeFilterRef.current.start ?? undefined,
        dateRangeFilterRef.current.end ?? undefined,
        monthYearRef.current,
        sortOrderRef.current,
        actualCursor || undefined
      )

      // Capture dateRange from first response
      if (!dateRangeCapturedRef.current && result.dateRange.minDate) {
        setDateRange(result.dateRange)
        dateRangeCapturedRef.current = true
      }

      return result
    },
    transform: (response) => response.groups,
    responseNextCursor: (response) => response.nextCursor ?? null,
    responseTotal: (response) => response.totalImages,
    mergeFn: (existing, incoming) => {
      if (existing.length === 0) return incoming
      const lastExisting = existing[existing.length - 1]
      const firstIncoming = incoming[0]
      // Merge if the boundary groups share the same date
      if (lastExisting.date === firstIncoming.date) {
        const merged: CalendarDateGroup = {
          ...lastExisting,
          images: [...lastExisting.images, ...firstIncoming.images],
          imageCount: lastExisting.imageCount + firstIncoming.imageCount,
        }
        return [...existing.slice(0, -1), merged, ...incoming.slice(1)]
      }
      return [...existing, ...incoming]
    },
  })

  // Reset pagination when filters change and reload data
  useEffect(() => {
    infiniteScroll.reset()
    // Reset dateRange capture so it updates from the new response
    dateRangeCapturedRef.current = false
    setDateRange({ minDate: "", maxDate: "", totalWithDate: 0 })
    // Load fresh data with new filters/sort order
    infiniteScroll.loadMore()
  }, [dateRangeFilter.start, dateRangeFilter.end, sortOrder])

  // Update monthYear ref without resetting pagination
  const setMonthYear = useCallback((newMonthYear: string) => {
    monthYearRef.current = newMonthYear
  }, [])

  // Toggle sort order
  const toggleSortOrder = useCallback(() => {
    setSortOrder((prev) => (prev === "oldest" ? "newest" : "oldest"))
  }, [])

  // Clear date range filter
  const clearDateRangeFilter = useCallback(() => {
    setDateRangeFilter({ start: null, end: null })
  }, [])

  // Jump to a specific date using the seek endpoint
  const jumpToDate = useCallback(
    async (date: string) => {
      // Check if date is already in loaded groups
      if (infiniteScroll.items.some((g) => g.date === date)) {
        return // Already loaded, caller will handle scrolling
      }

      // Get cursor for this date
      const seekResult = await fetchCalendarSeek(date)

      // Reset pagination state
      infiniteScroll.reset()

      // Set the jump cursor - the next loadMore will use it
      jumpCursorRef.current = seekResult.cursor

      // Trigger loadMore which will use the jump cursor
      await infiniteScroll.loadMore()
    },
    [infiniteScroll]
  )

  // Remove an image from the list
  const removeImage = useCallback(
    (imageId: number) => {
      // Remove image from groups state
      infiniteScroll.setItems((prevGroups) => {
        return prevGroups
          .map((group) => ({
            ...group,
            images: group.images.filter((img) => img.id !== imageId),
            imageCount: group.images.filter((img) => img.id !== imageId).length,
          }))
          .filter((group) => group.images.length > 0) // Remove empty groups
      })
      infiniteScroll.setTotal((prev) => Math.max(0, prev - 1))
    },
    [infiniteScroll]
  )

  // Mark all images in a date group as having GPS
  const updateGroupGpsStatus = useCallback(
    (date: string) => {
      infiniteScroll.setItems((prevGroups) =>
        prevGroups.map((group) =>
          group.date === date
            ? {
                ...group,
                images: group.images.map((img) => ({ ...img, missingGps: false })),
              }
            : group
        )
      )
    },
    [infiniteScroll]
  )

  return {
    groups: infiniteScroll.items,
    totalImages: infiniteScroll.total,
    hasMore: infiniteScroll.hasMore,
    isLoading: infiniteScroll.isLoading,
    error: infiniteScroll.error,
    initialized: infiniteScroll.initialized,
    dateRange,
    dateRangeFilter,
    sortOrder,
    setDateRangeFilter,
    setSortOrder,
    toggleSortOrder,
    clearDateRangeFilter,
    setMonthYear,
    jumpToDate,
    removeImage,
    updateGroupGpsStatus,
    loadMore: infiniteScroll.loadMore,
    reset: infiniteScroll.reset,
  }
}
