// Helpers for the API's weekday representation: comma-separated weekday
// numbers using Go's time.Weekday numbering (0=Sunday ... 6=Saturday).

/** Parses the weekday string into flags indexed 0=Sunday ... 6=Saturday. */
export function parseDaysString(value: string): boolean[] {
  const days = new Array<boolean>(7).fill(false)
  if (!value) return days
  for (const chunk of value.split(",")) {
    const day = Number(chunk.trim())
    if (Number.isInteger(day) && day >= 0 && day <= 6) days[day] = true
  }
  return days
}

/** Serializes weekday flags (0=Sunday ... 6=Saturday) into the API's weekday string. */
export function daysToString(days: boolean[]): string {
  return days
    .map((checked, index) => (checked ? String(index) : ""))
    .filter(Boolean)
    .join(",")
}
