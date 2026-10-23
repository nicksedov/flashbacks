package domain

import "time"

// ScheduleKind identifies the recurring background job a schedule configures.
type ScheduleKind string

const (
	// ScheduleGallerySync is the gallery synchronization job.
	ScheduleGallerySync ScheduleKind = "gallery_sync"
	// ScheduleTagScan is the automatic image tagging job.
	ScheduleTagScan ScheduleKind = "tag_scan"
)

// Schedule is the stored schedule of a recurring background job. Every scheduled
// job reads and writes this row instead of keeping its own schedule columns.
type Schedule struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Kind    string `gorm:"uniqueIndex;not null" json:"kind"`
	Enabled bool   `gorm:"default:true" json:"enabled"`
	// Days holds comma-separated weekday numbers (time.Weekday: 0=Sunday,
	// 1=Monday, ..., 6=Saturday). An empty string means the job never runs.
	Days string `gorm:"default:'1,2,3,4,5'" json:"days"`
	// StartHour/StartMinute are the run time (or window opening) in the user's
	// local time.
	StartHour   int `gorm:"default:0" json:"startHour"`
	StartMinute int `gorm:"default:0" json:"startMinute"`
	// EndHour/EndMinute close the window for jobs that scan a time range. When
	// both bounds equal the start, the job is a single run at the start time.
	EndHour        int       `gorm:"default:0" json:"endHour"`
	EndMinute      int       `gorm:"default:0" json:"endMinute"`
	TimezoneOffset int       `gorm:"default:0" json:"timezoneOffset"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// TableName overrides the default GORM table name to "schedules".
func (Schedule) TableName() string {
	return "schedules"
}

// ParseWeekdays converts a comma-separated weekday string to a slice of
// time.Weekday. Returns nil if the input is empty.
func ParseWeekdays(s string) []time.Weekday {
	if s == "" {
		return nil
	}
	var days []time.Weekday
	for _, c := range s {
		if c >= '0' && c <= '6' {
			days = append(days, time.Weekday(c-'0'))
		}
	}
	return days
}

// FormatWeekdays converts a slice of time.Weekday to a comma-separated string.
func FormatWeekdays(days []time.Weekday) string {
	if len(days) == 0 {
		return ""
	}
	result := ""
	for i, d := range days {
		if i > 0 {
			result += ","
		}
		result += string(rune('0' + int(d)))
	}
	return result
}

// Weekdays returns the weekdays this schedule runs on.
func (s Schedule) Weekdays() []time.Weekday {
	return ParseWeekdays(s.Days)
}

// IsWeekdaySelected reports whether weekday is part of the schedule.
func (s Schedule) IsWeekdaySelected(weekday time.Weekday) bool {
	for _, d := range s.Weekdays() {
		if d == weekday {
			return true
		}
	}
	return false
}

// userZone returns the user's timezone. TimezoneOffset follows the JS
// getTimezoneOffset convention (UTC+3 = -180), so the zone offset is its
// negation.
func (s Schedule) userZone() *time.Location {
	return time.FixedZone("UserTZ", -s.TimezoneOffset*60)
}

// localTime converts an instant to the user's local wall clock.
func (s Schedule) localTime(now time.Time) time.Time {
	return now.In(s.userZone())
}

// IsWindowOpenAt reports whether the schedule's window is open at now. The
// window opens only on selected weekdays; with no weekdays selected it never
// opens. Equal start and end bounds mean a 24-hour window.
func (s Schedule) IsWindowOpenAt(now time.Time) bool {
	days := s.Weekdays()
	if len(days) == 0 {
		return false
	}

	localNow := s.localTime(now)
	if !s.IsWeekdaySelected(localNow.Weekday()) {
		return false
	}

	currentMinutes := localNow.Hour()*60 + localNow.Minute()
	startMinutes := s.StartHour*60 + s.StartMinute
	endMinutes := s.EndHour*60 + s.EndMinute

	if startMinutes == endMinutes {
		return true
	}
	if startMinutes < endMinutes {
		return currentMinutes >= startMinutes && currentMinutes <= endMinutes
	}
	// Overnight window (e.g. 22:00 - 07:00).
	return currentMinutes >= startMinutes || currentMinutes <= endMinutes
}

// NextRunAt returns the next instant the job runs at its start time on a
// selected weekday. With no weekdays selected it returns a far-future instant so
// callers park instead of spinning.
func (s Schedule) NextRunAt(now time.Time) time.Time {
	return s.nextOpeningAt(now)
}

// NextWindowOpenAt returns the next instant the schedule's window opens on a
// selected weekday, looking a full week ahead.
func (s Schedule) NextWindowOpenAt(now time.Time) time.Time {
	return s.nextOpeningAt(now)
}

// nextOpeningAt implements the shared "next start on a selected weekday" search.
func (s Schedule) nextOpeningAt(now time.Time) time.Time {
	days := s.Weekdays()
	if len(days) == 0 {
		return now.AddDate(100, 0, 0)
	}

	userZone := s.userZone()
	localNow := now.In(userZone)
	for offset := 0; offset < 8; offset++ {
		day := localNow.AddDate(0, 0, offset)
		if !s.IsWeekdaySelected(day.Weekday()) {
			continue
		}
		candidate := time.Date(day.Year(), day.Month(), day.Day(), s.StartHour, s.StartMinute, 0, 0, userZone)
		if !candidate.After(localNow) {
			continue
		}
		return candidate
	}

	// Fallback: a selected day always exists, so this is unreachable in practice.
	return now.AddDate(0, 0, 7)
}

// DefaultSchedule returns the built-in schedule for a job kind.
func DefaultSchedule(kind ScheduleKind) Schedule {
	switch kind {
	case ScheduleGallerySync:
		return Schedule{
			Kind:        string(kind),
			Enabled:     true,
			Days:        "1,2,3,4,5",
			StartHour:   3,
			StartMinute: 30,
		}
	case ScheduleTagScan:
		return Schedule{
			Kind:        string(kind),
			Enabled:     true,
			Days:        "1,2,3,4,5",
			StartHour:   22,
			StartMinute: 0,
			EndHour:     7,
			EndMinute:   0,
		}
	default:
		return Schedule{
			Kind:    string(kind),
			Enabled: true,
			Days:    "1,2,3,4,5",
		}
	}
}
