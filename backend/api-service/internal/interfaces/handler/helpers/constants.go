package helpers

// Date/time format constants used across the application
const (
	DateTimeFormat       = "2006-01-02 15:04:05"
	DateOnlyFormat       = "2006-01-02"
	YearMonthFormat      = "2006-01"
	RFC3339Format        = "2006-01-02T15:04:05Z07:00"
	TrashTimestampFormat = "20060102_150405"
)

// Concurrency constants
const (
	DefaultMaxWorkers = 16
)

// Pagination constants
const (
	DefaultPageSize     = 50
	MaxPageSizeFlexible = 200
)

// Fixed page sizes for handlers that require specific values
var FixedPageSizes = []int{50, 100, 250, 500}
