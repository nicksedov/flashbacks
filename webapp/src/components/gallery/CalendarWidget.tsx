import { useMemo, useState, useRef, useEffect, useCallback } from "react"
import { useTranslation } from "@/i18n"
import { ChevronLeft, ChevronRight, ChevronDown } from "lucide-react"
import type { CalendarDateRange, CalendarMonthInfo } from "@/types"

const MONTHS = [
  { value: 0, label: "Jan" },
  { value: 1, label: "Feb" },
  { value: 2, label: "Mar" },
  { value: 3, label: "Apr" },
  { value: 4, label: "May" },
  { value: 5, label: "Jun" },
  { value: 6, label: "Jul" },
  { value: 7, label: "Aug" },
  { value: 8, label: "Sep" },
  { value: 9, label: "Oct" },
  { value: 10, label: "Nov" },
  { value: 11, label: "Dec" },
]

interface CalendarWidgetProps {
  dateRange: CalendarDateRange
  monthInfo: CalendarMonthInfo | null
  dayCounts: Map<number, number>
  calendarViewDate: Date
  dateRangeFilter: { start: string | null; end: string | null }
  rangeSelecting: boolean
  onMonthChange: (date: Date) => void
  onDateSelect: (date: string) => Promise<void> | void
  onStartRangeSelect?: (date: string) => void
  onDateRangeSelect: (start: string, end: string) => Promise<void> | void
  onClearFilter: () => void
}

export function CalendarWidget({
  dateRange,
  monthInfo,
  dayCounts,
  calendarViewDate,
  dateRangeFilter,
  rangeSelecting,
  onMonthChange,
  onDateSelect,
  onStartRangeSelect,
  onDateRangeSelect,
  onClearFilter,
}: CalendarWidgetProps) {
  const { t } = useTranslation()

  // Compute which months have images across all years in the date range
  const monthsWithImages = useMemo(() => {
    const monthSet = new Set<number>()
    if (dateRange.minDate && dateRange.maxDate) {
      const minDate = new Date(dateRange.minDate + "T00:00:00")
      const maxDate = new Date(dateRange.maxDate + "T00:00:00")
      for (let y = minDate.getFullYear(); y <= maxDate.getFullYear(); y++) {
        for (let m = 0; m < 12; m++) {
          // A month is "active" if it falls within the data range
          const monthStart = new Date(y, m, 1)
          const monthEnd = new Date(y, m + 1, 0)
          if (monthEnd >= minDate && monthStart <= maxDate) {
            monthSet.add(m)
          }
        }
      }
    }
    return monthSet
  }, [dateRange.minDate, dateRange.maxDate])

  // Compute which years have images
  const yearsWithImages = useMemo(() => {
    const yearSet = new Set<number>()
    if (dateRange.minDate && dateRange.maxDate) {
      const minYear = new Date(dateRange.minDate + "T00:00:00").getFullYear()
      const maxYear = new Date(dateRange.maxDate + "T00:00:00").getFullYear()
      for (let y = minYear; y <= maxYear; y++) {
        yearSet.add(y)
      }
    }
    return yearSet
  }, [dateRange.minDate, dateRange.maxDate])

  // Custom dropdown state
  const [monthOpen, setMonthOpen] = useState(false)
  const [yearOpen, setYearOpen] = useState(false)
  const monthRef = useRef<HTMLDivElement>(null)
  const yearRef = useRef<HTMLDivElement>(null)

  // Close dropdowns on outside click
  const handleOutsideClick = useCallback((e: MouseEvent) => {
    if (monthRef.current && !monthRef.current.contains(e.target as Node)) setMonthOpen(false)
    if (yearRef.current && !yearRef.current.contains(e.target as Node)) setYearOpen(false)
  }, [])

  useEffect(() => {
    document.addEventListener("mousedown", handleOutsideClick)
    return () => document.removeEventListener("mousedown", handleOutsideClick)
  }, [handleOutsideClick])

  const currentYear = calendarViewDate.getFullYear()
  const yearList = useMemo(() => {
    let startYear = currentYear - 5
    let endYear = currentYear + 5
    if (dateRange.minDate && dateRange.maxDate) {
      startYear = new Date(dateRange.minDate + "T00:00:00").getFullYear()
      endYear = new Date(dateRange.maxDate + "T00:00:00").getFullYear()
    }
    const list: number[] = []
    for (let y = startYear; y <= endYear; y++) list.push(y)
    return list
  }, [currentYear, dateRange.minDate, dateRange.maxDate])

  const prevMonth = () => {
    onMonthChange(new Date(calendarViewDate.getFullYear(), calendarViewDate.getMonth() - 1, 1))
  }

  const nextMonth = () => {
    onMonthChange(new Date(calendarViewDate.getFullYear(), calendarViewDate.getMonth() + 1, 1))
  }

  const handleMonthChange = (newMonth: number) => {
    onMonthChange(new Date(calendarViewDate.getFullYear(), newMonth, 1))
  }

  const handleYearChange = (newYear: number) => {
    onMonthChange(new Date(newYear, calendarViewDate.getMonth(), 1))
  }

  const isInSelectedRange = (date: string) => {
    if (!dateRangeFilter.start) return false
    if (!dateRangeFilter.end) return date === dateRangeFilter.start
    return date >= dateRangeFilter.start && date <= dateRangeFilter.end
  }

  const selectFullMonth = () => {
    const year = calendarViewDate.getFullYear()
    const month = calendarViewDate.getMonth()
    const firstDay = `${year}-${String(month + 1).padStart(2, "0")}-01`
    const lastDayDate = new Date(year, month + 1, 0)
    const lastDay = `${year}-${String(month + 1).padStart(2, "0")}-${String(lastDayDate.getDate()).padStart(2, "0")}`

    onDateRangeSelect(firstDay, lastDay)
  }

  // Generate all days of the month as a scrollable strip
  const calendarDays = useMemo(() => {
    const year = calendarViewDate.getFullYear()
    const month = calendarViewDate.getMonth()
    const lastDay = new Date(year, month + 1, 0)
    const daysInMonth = lastDay.getDate()

    const daysWithImages = new Set(monthInfo?.days ?? [])

    const days: { date: string; day: number; hasImages: boolean; imageCount: number }[] = []
    for (let d = 1; d <= daysInMonth; d++) {
      const dateStr = `${year}-${String(month + 1).padStart(2, "0")}-${String(d).padStart(2, "0")}`
      const count = dayCounts.get(d) ?? 0
      days.push({
        date: dateStr,
        day: d,
        hasImages: daysWithImages.has(d),
        imageCount: count,
      })
    }

    return days
  }, [calendarViewDate, monthInfo, dayCounts])

  return (
    <div className="rounded-lg border bg-card p-3">
      {/* Month/Year selector */}
      <div className="flex items-center justify-between mb-2 gap-2">
        <button onClick={prevMonth} className="p-1 hover:bg-muted rounded flex-shrink-0">
          <ChevronLeft className="h-4 w-4" />
        </button>

        <div className="flex items-center gap-2">
          {/* Month dropdown */}
          <div ref={monthRef} className="relative">
            <button
              type="button"
              onClick={() => { setMonthOpen(v => !v); setYearOpen(false) }}
              className="inline-flex items-center gap-1 text-sm font-medium border border-border rounded-md px-2 py-1 bg-popover text-popover-foreground hover:bg-accent transition-colors cursor-pointer"
            >
              {new Date(2000, calendarViewDate.getMonth(), 1).toLocaleDateString(undefined, { month: "long" })}
              <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
            </button>
            {monthOpen && (
              <div
                className="absolute z-50 top-full left-0 mt-1 rounded-md border shadow-lg"
                style={{ backgroundColor: 'var(--color-popover)', color: 'var(--color-popover-foreground)' }}
              >
                {MONTHS.map((m) => (
                  <button
                    key={m.value}
                    type="button"
                    className={`w-full text-left px-2.5 py-1.5 text-xs hover:bg-accent transition-colors ${
                      m.value === calendarViewDate.getMonth() ? "font-semibold bg-accent/50" : ""
                    } ${!monthsWithImages.has(m.value) ? "text-muted-foreground" : ""}`}
                    onClick={() => { handleMonthChange(m.value); setMonthOpen(false) }}
                  >
                    {new Date(2000, m.value, 1).toLocaleDateString(undefined, { month: "long" })}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Year dropdown */}
          <div ref={yearRef} className="relative">
            <button
              type="button"
              onClick={() => { setYearOpen(v => !v); setMonthOpen(false) }}
              className="inline-flex items-center gap-1 text-sm font-medium border border-border rounded-md px-2 py-1 bg-popover text-popover-foreground hover:bg-accent transition-colors cursor-pointer"
            >
              {currentYear}
              <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
            </button>
            {yearOpen && (
              <div
                className="absolute z-50 top-full left-0 mt-1 max-h-80 overflow-y-auto rounded-md border shadow-lg"
                style={{ backgroundColor: 'var(--color-popover)', color: 'var(--color-popover-foreground)' }}
              >
                {yearList.map((year) => (
                  <button
                    key={year}
                    type="button"
                    className={`w-full text-left px-2.5 py-1.5 text-xs hover:bg-accent transition-colors ${
                      year === currentYear ? "font-semibold bg-accent/50" : ""
                    } ${!yearsWithImages.has(year) ? "text-muted-foreground" : ""}`}
                    onClick={() => { handleYearChange(year); setYearOpen(false) }}
                  >
                    {year}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>

        <button onClick={nextMonth} className="p-1 hover:bg-muted rounded flex-shrink-0">
          <ChevronRight className="h-4 w-4" />
        </button>
      </div>

      {/* Horizontal scrollable day strip */}
      <div className="flex gap-0.75 overflow-x-auto pb-1 pt-2 px-1 calendar-days-no-scrollbar">
        {calendarDays.map((day) => {
          const isSelected = isInSelectedRange(day.date)
          const isRangeStart = day.date === dateRangeFilter.start
          const isRangeEnd = day.date === dateRangeFilter.end && dateRangeFilter.end !== null

          return (
            <button
              key={day.date}
              disabled={!day.date}
              className={`
                flex-shrink-0 w-9 h-9 flex flex-col items-center justify-center text-xs rounded-md
                transition-all relative
                ${isSelected
                  ? "bg-primary/80 text-primary-foreground hover:bg-primary/90 font-medium cursor-pointer"
                  : day.hasImages
                    ? "bg-emerald-100/80 bg-card hover:bg-emerald-200/80 text-emerald-600 font-semibold cursor-pointer"
                    : "bg-muted/50 hover:bg-muted text-muted-foreground/70"
                }
                ${isRangeStart || isRangeEnd ? "ring-2 ring-primary ring-offset-2" : ""}
                ${isSelected && !isRangeStart && !isRangeEnd ? "opacity-80" : ""}
              `}
              onClick={() => day.date && onDateSelect(day.date)}
              onDoubleClick={() => day.date && onStartRangeSelect?.(day.date)}
              title={day.hasImages ? `${day.imageCount} ${day.imageCount === 1 ? "image" : "images"}` : "No images"}
            >
              <span className="text-[11px] leading-none">{day.day}</span>
            </button>
          )
        })}
      </div>

      {/* Date filter controls */}
      <div className="mt-2 pt-2 border-t flex items-center justify-between gap-2">
        <div className="flex items-center gap-1.5">
          <button
            onClick={selectFullMonth}
            className="text-xs px-2 py-1 bg-primary text-primary-foreground rounded hover:bg-primary/90 transition-colors"
          >
            {t("gallery.calendar.fullMonth")}
          </button>
          {onStartRangeSelect && !rangeSelecting && (
            <button
              onClick={() => {
                const year = calendarViewDate.getFullYear()
                const month = calendarViewDate.getMonth()
                const firstDay = `${year}-${String(month + 1).padStart(2, "0")}-01`
                onStartRangeSelect(firstDay)
              }}
              className="text-xs px-2 py-1 rounded border border-border bg-popover text-popover-foreground hover:bg-accent transition-colors"
            >
              {t("gallery.calendar.selectRange")}
            </button>
          )}
        </div>

        {(dateRangeFilter.start || dateRangeFilter.end) && (
          <div className="flex items-center gap-2 min-w-0">
            <span className="text-xs text-muted-foreground truncate">
              {rangeSelecting
                ? `${t("gallery.calendar.selectingRange", { start: dateRangeFilter.start ?? "" })}${!dateRangeFilter.end ? ` (${t("gallery.calendar.clickEndDate")})` : ""}`
                : dateRangeFilter.start === dateRangeFilter.end
                  ? dateRangeFilter.start
                  : `${dateRangeFilter.start} — ${dateRangeFilter.end}`}
            </span>
            <button
              onClick={onClearFilter}
              className="text-xs text-primary hover:underline flex-shrink-0"
            >
              {t("gallery.calendar.clearFilter")}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
