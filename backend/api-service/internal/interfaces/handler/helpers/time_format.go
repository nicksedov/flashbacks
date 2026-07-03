package helpers

import (
	"time"
)

// FormatTimeInUserTZ formats a time pointer as an ISO 8601 string in the user's configured timezone.
// This prevents browser double-conversion: the string is already in the user's local time.
func FormatTimeInUserTZ(t *time.Time, timezoneOffset int) string {
	if t == nil {
		return ""
	}
	userTZ := time.FixedZone("UserTZ", -timezoneOffset*60)
	return t.In(userTZ).Format(DateTimeFormat)
}