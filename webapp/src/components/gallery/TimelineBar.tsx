import { useMemo } from "react"
import { useTranslation } from "@/i18n"
import type { CalendarDateGroup, CalendarDateRange, TimelineDateMarker } from "@/types"

interface TimelineBarProps {
  dateRange: CalendarDateRange
  groups: CalendarDateGroup[]
  allDates: TimelineDateMarker[]
  dateRangeFilter: { start: string | null; end: string | null }
  loadedDates: Set<string>
  onNavigateToDate: (date: string) => void
}

const HEADER_HEIGHT = 56

export function TimelineBar({
  dateRange,
  groups,
  allDates,
  dateRangeFilter,
  loadedDates,
  onNavigateToDate,
}: TimelineBarProps) {
  const { t } = useTranslation()

  const buildTooltip = (date: string, imageCount: number, isLoaded: boolean, isVisible: boolean, isFiltered: boolean) => {
    const parts = [`${date} (${t("gallery.calendar.tooltipImages", { count: imageCount })})`]
    if (isVisible) parts.push(t("gallery.calendar.tooltipVisible"))
    else if (isLoaded) parts.push(t("gallery.calendar.tooltipLoaded"))
    if (isFiltered) parts.push(t("gallery.calendar.tooltipOutsideFilter"))
    return parts.join(" - ")
  }
  const visibleDateRange = useMemo(() => {
    if (groups.length === 0) return { start: null, end: null }
    return {
      start: groups[0].date,
      end: groups[groups.length - 1].date,
    }
  }, [groups])

  // Determine which dates are active based on filter
  const activeDateSet = useMemo(() => {
    const set = new Set<string>()
    if (dateRangeFilter.start && dateRangeFilter.end) {
      // When a date range filter is active, only dates within it are active
      allDates.forEach((d) => {
        if (d.date >= dateRangeFilter.start! && d.date <= dateRangeFilter.end!) {
          set.add(d.date)
        }
      })
    }
    return set
  }, [allDates, dateRangeFilter.start, dateRangeFilter.end])

  // Dates currently visible on the page (from loaded groups)
  const visibleDatesSet = useMemo(() => {
    const set = new Set<string>()
    groups.forEach((g) => set.add(g.date))
    return set
  }, [groups])

  const hasActiveFilter = dateRangeFilter.start !== null && dateRangeFilter.end !== null

  if (!dateRange.minDate || !dateRange.maxDate || allDates.length === 0) {
    return null
  }

  return (
    <div
      className="fixed right-0 w-16 z-10 hidden lg:flex flex-col justify-center"
      style={{
        pointerEvents: "none",
        top: `${HEADER_HEIGHT}px`,
        bottom: 0,
      }}
    >
      <div
        className="rounded-l-lg border-r border-y border-l-0 bg-gray-400/40 p-2 mx-0"
        style={{
          pointerEvents: "auto",
          height: "calc(100vh - 2rem)",
          maxHeight: "calc(100vh - 2rem)",
        }}
      >
        <div
          className="relative flex-1"
          style={{ height: "calc(100% - 2rem)" }}
          onClick={(e) => {
            const rect = e.currentTarget.getBoundingClientRect()
            const clickY = e.clientY - rect.top
            const clickPercent = clickY / rect.height

            const totalDays = daysBetween(dateRange.minDate, dateRange.maxDate)
            const targetOffset = Math.floor(clickPercent * totalDays)
            const targetDate = new Date(dateRange.minDate + "T00:00:00")
            targetDate.setDate(targetDate.getDate() + targetOffset)

            const dateStr = `${targetDate.getFullYear()}-${String(targetDate.getMonth() + 1).padStart(2, "0")}-${String(targetDate.getDate()).padStart(2, "0")}`

            // Find the closest actual date from allDates
            const closestDate = findClosestDate(allDates.map((d) => d.date), dateStr)
            if (closestDate) {
              onNavigateToDate(closestDate)
            }
          }}
        >
          {/* Timeline track */}
          <div className="absolute left-1/2 -translate-x-1/2 w-0.5 h-full bg-muted" />

          {/* Year/Month scale markers */}
          {(() => {
            const markers = []
            const start = new Date(dateRange.minDate + "T00:00:00")
            const end = new Date(dateRange.maxDate + "T00:00:00")
            const totalDays = daysBetween(dateRange.minDate, dateRange.maxDate)

            // Generate year markers
            let current = new Date(start.getFullYear(), 0, 1)
            if (current < start) current = new Date(start.getFullYear() + 1, 0, 1)

            while (current <= end) {
              const offset = daysBetween(dateRange.minDate, current.toISOString().split("T")[0])
              const topPercent = totalDays > 0 ? (offset / totalDays) * 100 : 0

              markers.push(
                <div
                  key={`year-${current.getFullYear()}`}
                  className="absolute left-0 right-0 border-t border-muted/30"
                  style={{ top: `${topPercent}%` }}
                >
                  <span
                    className="absolute left-0 text-[9px] font-semibold text-foreground whitespace-nowrap -translate-x-1/2 px-1 rounded bg-background/80"
                    style={{ left: "50%" }}
                  >
                    {current.getFullYear()}
                  </span>
                </div>
              )

              current.setFullYear(current.getFullYear() + 1)
              current = new Date(current.getFullYear(), 0, 1)
            }

            return markers
          })()}

          {/* Visible range indicator */}
          {visibleDateRange.start && visibleDateRange.end && (
            (() => {
              const totalDays = daysBetween(dateRange.minDate, dateRange.maxDate)
              const startOffset = daysBetween(dateRange.minDate, visibleDateRange.start)
              const endOffset = daysBetween(dateRange.minDate, visibleDateRange.end)
              const topPercent = totalDays > 0 ? (startOffset / totalDays) * 100 : 0
              const heightPercent = totalDays > 0 ? ((endOffset - startOffset) / totalDays) * 100 : 0

              return (
                <div
                  className="absolute left-1/2 -translate-x-1/2 w-3 bg-blue-500/50 rounded-sm"
                  style={{
                    top: `${topPercent}%`,
                    height: `${Math.max(heightPercent, 5)}%`,
                  }}
                />
              )
            })()
          )}

          {/* Date markers for ALL dates (not just visible groups) */}
          {allDates
            .filter((dateMarker) => dateMarker.imageCount > 0)
            .map((dateMarker) => {
            const offset = daysBetween(dateRange.minDate, dateMarker.date)
            const totalDays = daysBetween(dateRange.minDate, dateRange.maxDate)
            const topPercent = totalDays > 0 ? (offset / totalDays) * 100 : 0

            // Determine marker state
            const isLoaded = loadedDates.has(dateMarker.date)
            const isVisible = visibleDatesSet.has(dateMarker.date)
            const isFiltered = hasActiveFilter && !activeDateSet.has(dateMarker.date)
            const isActive = !isFiltered

            // Style based on state - always visible, semi-transparent so labels show through
            if (!isActive) {
              // Inactive (outside filter range) - dimmed but visible
              return (
                <div
                  key={dateMarker.date}
                  className="absolute left-1/2 -translate-x-1/2 rounded-full transition-all bg-gray-400 cursor-not-allowed"
                  style={{ top: `${topPercent}%`, width: "8px", height: "8px", opacity: 0.4 }}
                  title={buildTooltip(dateMarker.date, dateMarker.imageCount, isLoaded, false, isFiltered)}
                  onClick={(e) => {
                    e.stopPropagation()
                  }}
                />
              )
            } else if (isVisible) {
              // Currently visible on page - larger and fully opaque
              return (
                <div
                  key={dateMarker.date}
                  className="absolute left-1/2 -translate-x-1/2 rounded-full transition-all bg-blue-500 cursor-pointer hover:scale-125"
                  style={{ top: `${topPercent}%`, width: "12px", height: "12px", opacity: 1 }}
                  title={buildTooltip(dateMarker.date, dateMarker.imageCount, isLoaded, true, false)}
                  onClick={(e) => {
                    e.stopPropagation()
                    onNavigateToDate(dateMarker.date)
                  }}
                />
              )
            } else if (isLoaded) {
              // Loaded but not currently visible - solid but smaller
              return (
                <div
                  key={dateMarker.date}
                  className="absolute left-1/2 -translate-x-1/2 rounded-full transition-all bg-blue-500 cursor-pointer hover:scale-150"
                  style={{ top: `${topPercent}%`, width: "8px", height: "8px", opacity: 0.75 }}
                  title={buildTooltip(dateMarker.date, dateMarker.imageCount, isLoaded, false, isFiltered)}
                  onClick={(e) => {
                    e.stopPropagation()
                    onNavigateToDate(dateMarker.date)
                  }}
                />
              )
            } else {
              // Active but not loaded yet - more transparent but still visible
              return (
                <div
                  key={dateMarker.date}
                  className="absolute left-1/2 -translate-x-1/2 rounded-full transition-all bg-blue-400 cursor-pointer hover:scale-125"
                  style={{ top: `${topPercent}%`, width: "8px", height: "8px", opacity: 0.5 }}
                  title={buildTooltip(dateMarker.date, dateMarker.imageCount, isLoaded, false, isFiltered)}
                  onClick={(e) => {
                    e.stopPropagation()
                    onNavigateToDate(dateMarker.date)
                  }}
                />
              )
            }
          })}
        </div>
      </div>
    </div>
  )
}

function daysBetween(date1: string, date2: string): number {
  const d1 = new Date(date1 + "T00:00:00")
  const d2 = new Date(date2 + "T00:00:00")
  return Math.floor((d2.getTime() - d1.getTime()) / (1000 * 60 * 60 * 24))
}

/**
 * Find the closest date from the list that is <= target date
 */
function findClosestDate(dates: string[], target: string): string | null {
  // Sort dates and find the one closest to but not after target
  let closest: string | null = null
  let minDiff = Infinity

  for (const date of dates) {
    const diff = daysBetween(target, date)
    // We want the date closest to target from below (date <= target)
    if (diff >= 0 && diff < minDiff) {
      minDiff = diff
      closest = date
    }
  }

  // If no date found before target, return the earliest date
  if (!closest && dates.length > 0) {
    return dates[0]
  }

  return closest
}
