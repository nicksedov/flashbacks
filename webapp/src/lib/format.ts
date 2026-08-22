/**
 * Shared date/time formatting helpers.
 * Centralizes the naive-datetime parsing that was previously copy-pasted
 * between SyncHistoryDialog and AdminGeneralTab.
 */

/**
 * Format an ISO datetime for display.
 * Handles backend "naive" datetimes (no timezone offset) by parsing them as
 * local time to avoid double-conversion. Falls back to the raw string on error.
 */
export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return ""
  try {
    let d: Date
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/.test(iso)) {
      const [datePart, timePart] = iso.split("T")
      const [y, m, day] = datePart.split("-").map(Number)
      const [h, min, s] = timePart.split(":").map(Number)
      d = new Date(y, m - 1, day, h, min, s)
    } else {
      d = new Date(iso)
    }
    return d.toLocaleString()
  } catch {
    return iso
  }
}

/**
 * Format a timestamp as a compact relative time ("5m ago", "3h ago", "2d ago").
 * Uses Intl.RelativeTimeFormat when available, with a lightweight fallback.
 */
export function formatRelativeTime(dateStr: string, language = "en"): string {
  const then = new Date(dateStr).getTime()
  if (Number.isNaN(then)) return dateStr
  const diffMs = Date.now() - then
  const diffMin = Math.floor(diffMs / 60000)
  if (diffMin < 1) return "just now"

  const units: Array<[Intl.RelativeTimeFormatUnit, number]> = [
    ["day", Math.floor(diffMin / 1440)],
    ["hour", Math.floor(diffMin / 60)],
    ["minute", diffMin],
  ]
  const rtf = new Intl.RelativeTimeFormat(language, { numeric: "auto" })
  for (const [unit, value] of units) {
    if (unit === "minute" || value >= 1) {
      // Report the largest non-zero unit
      if (unit === "day" && value >= 30) {
        return new Date(dateStr).toLocaleDateString()
      }
      return rtf.format(-value, unit)
    }
  }
  return new Date(dateStr).toLocaleDateString()
}
