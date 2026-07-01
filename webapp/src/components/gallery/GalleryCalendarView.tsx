import { useEffect, useMemo, useRef, useState } from "react"
import { fetchCalendarMonthInfo, fetchCalendarAllDates } from "@/api/endpoints"
import { Skeleton } from "@/components/ui/skeleton"
import { Calendar as CalendarIcon, ArrowDown, ArrowUp, Loader2 } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { GalleryImageDTO, CalendarDateGroup, CalendarMonthInfo, TimelineDateMarker } from "@/types"
import { useCalendarData } from "@/hooks/useCalendarData"
import { CalendarImageGrid } from "./CalendarImageGrid"
import { CalendarWidget } from "./CalendarWidget"
import { TimelineBar } from "./TimelineBar"
import { BulkGeoDialog } from "./BulkGeoDialog"

interface GalleryCalendarViewProps {
  onImageClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO, removeThumbnail: () => void) => void
}

export function GalleryCalendarView({ onImageClick, onImageDownload, onImageDelete }: GalleryCalendarViewProps) {
  const { t } = useTranslation()

  // Calendar widget state (separate from pagination)
  const [calendarViewDate, setCalendarViewDate] = useState(() => new Date())
  const [monthInfo, setMonthInfo] = useState<CalendarMonthInfo | null>(null)
  const [dayCounts, setDayCounts] = useState<Map<number, number>>(new Map())
  const [allDates, setAllDates] = useState<TimelineDateMarker[]>([])
  const [rangeSelecting, setRangeSelecting] = useState(false)

  // Bulk geo dialog state
  const [bulkGeoGroup, setBulkGeoGroup] = useState<CalendarDateGroup | null>(null)

  // Auto-load remaining pages for a target date after navigation.
  // When set, a useEffect loop keeps calling loadMore() until the date
  // is no longer the last loaded group (i.e., all its images are fetched).
  const [pendingDateLoad, setPendingDateLoad] = useState<string | null>(null)

  // Image preloading cache
  const preloadImageCache = useRef<Map<string, HTMLImageElement>>(new Map())
  const sentinelRef = useRef<HTMLDivElement>(null)

  // Cursor-based pagination hook
  const calendarMonthKey = useMemo(() => {
    const y = calendarViewDate.getFullYear()
    const m = calendarViewDate.getMonth() + 1
    return `${y}-${String(m).padStart(2, "0")}`
  }, [calendarViewDate])

  const calendar = useCalendarData({
    initialMonthYear: calendarMonthKey,
  })

  // Track if user has manually navigated via timeline or calendar widget
  const hasUserNavigated = useRef(false)

  // Initial load on mount
  useEffect(() => {
    calendar.loadMore()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Sync calendar widget to show the month of loaded images (only on initial load)
  useEffect(() => {
    if (!hasUserNavigated.current && calendar.initialized && calendar.dateRange.minDate && !calendar.dateRangeFilter.start) {
      // Only set calendar to minDate month on initial load (no filter active, no user navigation)
      const minDate = new Date(calendar.dateRange.minDate + "T00:00:00")
      setCalendarViewDate(new Date(minDate.getFullYear(), minDate.getMonth(), 1))
    }
  }, [calendar.initialized, calendar.dateRange.minDate, calendar.dateRangeFilter.start])

  // Update monthYear ref when calendar widget changes month
  useEffect(() => {
    calendar.setMonthYear(calendarMonthKey)
  }, [calendarMonthKey])

  // Fetch all dates for timeline markers on mount
  useEffect(() => {
    fetchCalendarAllDates()
      .then((result) => {
        setAllDates(result.dates)
      })
      .catch(() => {
        setAllDates([])
      })
  }, [])

  // Load month info when calendar month changes (using lightweight endpoint)
  useEffect(() => {
    fetchCalendarMonthInfo(calendarMonthKey)
      .then((result) => {
        setMonthInfo({ year: result.year, month: result.month, days: result.days })
        // Build day count map
        const countMap = new Map<number, number>()
        result.dayCounts.forEach((dc) => {
          countMap.set(dc.day, dc.count)
        })
        setDayCounts(countMap)
      })
      .catch(() => {
        // Reset on error
        setMonthInfo(null)
        setDayCounts(new Map())
      })
  }, [calendarMonthKey])

  // Derived state for timeline
  const loadedDates = useMemo(() => {
    return new Set(calendar.groups.map((g) => g.date))
  }, [calendar.groups])

  // Preload images for smoother scrolling
  const preloadImages = (imageUrls: string[]) => {
    const MAX_CACHE_SIZE = 100

    imageUrls.forEach((url) => {
      if (!url || preloadImageCache.current.has(url)) return

      // Limit cache size
      if (preloadImageCache.current.size >= MAX_CACHE_SIZE) {
        const firstKey = preloadImageCache.current.keys().next().value
        if (firstKey) {
          preloadImageCache.current.delete(firstKey)
        }
      }

      const img = new Image()
      img.src = url
      preloadImageCache.current.set(url, img)
    })
  }

  // Preload images when groups change
  useEffect(() => {
    const imageUrls = calendar.groups.flatMap((group) =>
      group.images.map((img) => img.thumbnail).filter(Boolean) as string[]
    )

    // Preload with slight delay to not block initial render
    const timer = setTimeout(() => {
      preloadImages(imageUrls)
    }, 100)

    return () => clearTimeout(timer)
  }, [calendar.groups])

  // Infinite scroll observer
  useEffect(() => {
    const sentinel = sentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && calendar.hasMore && !calendar.isLoading) {
          calendar.loadMore()
        }
      },
      { threshold: 0.1 }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [calendar.hasMore, calendar.isLoading, calendar.loadMore])

  // Auto-load remaining pages for the target date after navigation.
  // When pendingDateLoad is set, this effect keeps calling loadMore()
  // until the date is no longer the last group (all images are fetched)
  // or there is no more data.
  //
  // IMPORTANT: Do NOT clear pendingDateLoad when isLoading is true —
  // loadMore() sets isLoading=true which triggers a re-render and would
  // kill the loop. Instead, just return and wait for isLoading→false
  // to re-fire the effect and continue loading.
  useEffect(() => {
    if (!pendingDateLoad) return
    const idx = calendar.groups.findIndex((g) => g.date === pendingDateLoad)
    // Terminal conditions: clear pendingDateLoad and stop
    if (idx === -1 || idx !== calendar.groups.length - 1 || !calendar.hasMore) {
      setPendingDateLoad(null)
      return
    }
    // Transient condition: loadMore() is in progress, wait for it to finish
    if (calendar.isLoading) return
    calendar.loadMore()
  }, [pendingDateLoad, calendar.groups, calendar.hasMore, calendar.isLoading, calendar.loadMore])

  // Handlers
  const handleDateSelect = async (date: string) => {
    // Navigate to the selected date
    await handleNavigateToDate(date)
  }

  const handleStartDateRange = (date: string) => {
    // Start range selection
    calendar.setDateRangeFilter({ start: date, end: null })
    setRangeSelecting(true)
  }

  const handleCompleteDateRange = async (start: string, end: string) => {
    calendar.setDateRangeFilter({ start, end })
    setRangeSelecting(false)
    // Navigate to the start date of the range
    await handleNavigateToDate(start)
  }

  const clearDateRangeFilter = () => {
    calendar.clearDateRangeFilter()
    setRangeSelecting(false)
  }

  const handleNavigateToDate = async (date: string) => {
    // Check if the date is already in loaded groups
    const existingGroupIndex = calendar.groups.findIndex((g) => g.date === date)
    if (existingGroupIndex !== -1) {
      // If this is the last group and more data exists, trigger auto-load
      // via the pendingDateLoad effect to fetch all remaining images.
      const isLastGroup = existingGroupIndex === calendar.groups.length - 1
      if (isLastGroup && calendar.hasMore && !calendar.isLoading) {
        setPendingDateLoad(date)
      }
      // Scroll to the group
      const element = document.getElementById(`date-group-${date}`)
      if (element) {
        element.scrollIntoView({ behavior: "smooth", block: "start" })
      }
      return
    }

    try {
      // Mark that user has manually navigated
      hasUserNavigated.current = true

      // Use the hook's jumpToDate method
      await calendar.jumpToDate(date)

      // After jumpToDate, trigger auto-load for any remaining pages of this date
      setPendingDateLoad(date)

      // Update calendar widget to the month of the navigated date
      const navDate = new Date(date + "T00:00:00")
      setCalendarViewDate(new Date(navDate.getFullYear(), navDate.getMonth(), 1))

      // Scroll to the date element after it's loaded
      setTimeout(() => {
        const element = document.getElementById(`date-group-${date}`)
        if (element) {
          element.scrollIntoView({ behavior: "smooth", block: "start" })
        }
      }, 100)
    } catch (err) {
      // Error is already handled by the hook's internal error state
      console.error("Failed to navigate to date:", err)
    }
  }

  const handleSortToggle = () => {
    calendar.toggleSortOrder()
  }

  const handleBulkGeoSaved = () => {
    if (bulkGeoGroup) {
      calendar.updateGroupGpsStatus(bulkGeoGroup.date)
      setBulkGeoGroup(null)
    }
  }

  return (
    <div className="space-y-4" style={{ cursor: calendar.isLoading ? "wait" : "auto" }}>
      {/* Global loading overlay */}
      {calendar.isLoading && (
        <div
          style={{
            position: "fixed",
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            zIndex: 9999,
            pointerEvents: "none",
            cursor: "wait",
          }}
        />
      )}

      {/* Header with image count and sort toggle */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <CalendarIcon className="h-5 w-5 text-muted-foreground" />
          {!calendar.initialized ? (
            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
          ) : (
            <span className="text-sm text-muted-foreground">
              {calendar.dateRange.totalWithDate > 0
                ? calendar.dateRange.totalWithDate === 1
                  ? t("gallery.imageCountOne", { count: calendar.dateRange.totalWithDate.toLocaleString() })
                  : t("gallery.imageCount", { count: calendar.dateRange.totalWithDate.toLocaleString() })
                : t("gallery.calendar.noDateInfo")
              }
            </span>
          )}
        </div>

        <button
          onClick={handleSortToggle}
          className="inline-flex items-center gap-2 rounded-md bg-transparent px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
          title={calendar.sortOrder === "newest" ? t("gallery.sortNewest") : t("gallery.sortOldest")}
        >
          {calendar.sortOrder === "newest" ? (
            <ArrowDown className="h-4 w-4" />
          ) : (
            <ArrowUp className="h-4 w-4" />
          )}
          <span>{calendar.sortOrder === "newest" ? t("gallery.sortNewest") : t("gallery.sortOldest")}</span>
        </button>
      </div>

      {/* Horizontal Calendar Widget */}
      <CalendarWidget
        dateRange={calendar.dateRange}
        monthInfo={monthInfo}
        dayCounts={dayCounts}
        calendarViewDate={calendarViewDate}
        dateRangeFilter={calendar.dateRangeFilter}
        rangeSelecting={rangeSelecting}
        onMonthChange={setCalendarViewDate}
        onDateSelect={handleDateSelect}
        onStartRangeSelect={handleStartDateRange}
        onDateRangeSelect={handleCompleteDateRange}
        onClearFilter={clearDateRangeFilter}
      />

      {/* Main content area with images and timeline */}
      <div className="flex gap-4" style={{ position: "relative" }}>
        {/* Images area */}
        <div className="flex-1 min-w-0">
          {calendar.error && (
            <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
              {calendar.error}
            </div>
          )}

          {!calendar.initialized ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-40 w-full rounded-lg" />
              ))}
            </div>
          ) : calendar.groups.length === 0 && !calendar.isLoading ? (
            <div className="rounded-lg border border-dashed p-12 text-center">
              <CalendarIcon className="mx-auto h-10 w-10 text-muted-foreground/50" />
              <p className="mt-2 text-sm font-medium text-muted-foreground">
                {calendar.dateRangeFilter.start ? t("gallery.calendar.noImagesForDate") : t("gallery.calendar.noDateInfo")}
              </p>
              <p className="text-xs text-muted-foreground/70">
                {calendar.dateRangeFilter.start ? t("gallery.calendar.clearFilterHint") : t("gallery.calendar.noDateInfoHint")}
              </p>
            </div>
          ) : (
            <>
              <CalendarImageGrid
                groups={calendar.groups}
                onImageClick={onImageClick}
                onImageDownload={onImageDownload}
                onImageDelete={(image) => {
                  onImageDelete?.(image, () => {
                    calendar.removeImage(image.id)
                  })
                }}
                onBulkGeo={setBulkGeoGroup}
              />

              <div ref={sentinelRef} className="h-4" />

              {calendar.isLoading && (
                <div className="flex justify-center py-4">
                  <div className="text-sm text-muted-foreground">{t("gallery.loadingMore")}</div>
                </div>
              )}

              {!calendar.hasMore && calendar.groups.length > 0 && (
                <div className="text-center text-xs text-muted-foreground py-4">
                  {t("gallery.allLoaded", { count: calendar.totalImages.toLocaleString() })}
                </div>
              )}
            </>
          )}
        </div>

        {/* Timeline sidebar */}
        <TimelineBar
          dateRange={calendar.dateRange}
          groups={calendar.groups}
          allDates={allDates}
          dateRangeFilter={calendar.dateRangeFilter}
          loadedDates={loadedDates}
          onNavigateToDate={handleNavigateToDate}
        />
      </div>

      {/* Bulk geo dialog */}
      {bulkGeoGroup && (
        <BulkGeoDialog
          open={bulkGeoGroup != null}
          onOpenChange={(open) => { if (!open) setBulkGeoGroup(null) }}
          date={bulkGeoGroup.date}
          label={bulkGeoGroup.label}
          imagesWithoutGps={bulkGeoGroup.images.filter((img) => img.missingGps)}
          onSaved={handleBulkGeoSaved}
        />
      )}
    </div>
  )
}
