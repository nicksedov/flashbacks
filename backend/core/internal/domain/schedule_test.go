package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleParseWeekdays(t *testing.T) {
	tests := []struct {
		input    string
		expected []time.Weekday
	}{
		{"", nil},
		{"0", []time.Weekday{time.Sunday}},
		{"1,2,3,4,5", []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}},
		{"0,6", []time.Weekday{time.Sunday, time.Saturday}},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, ParseWeekdays(tt.input), "input: %q", tt.input)
	}
}

func TestScheduleFormatWeekdays(t *testing.T) {
	tests := []struct {
		input    []time.Weekday
		expected string
	}{
		{nil, ""},
		{[]time.Weekday{}, ""},
		{[]time.Weekday{time.Monday, time.Friday}, "1,5"},
		{[]time.Weekday{time.Sunday, time.Saturday}, "0,6"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, FormatWeekdays(tt.input))
	}
}

func TestScheduleDefaultSchedule(t *testing.T) {
	syncSchedule := DefaultSchedule(ScheduleGallerySync)
	assert.Equal(t, string(ScheduleGallerySync), syncSchedule.Kind)
	assert.True(t, syncSchedule.Enabled)
	assert.Equal(t, "1,2,3,4,5", syncSchedule.Days)
	assert.Equal(t, 3, syncSchedule.StartHour)
	assert.Equal(t, 30, syncSchedule.StartMinute)

	tagSchedule := DefaultSchedule(ScheduleTagScan)
	assert.Equal(t, string(ScheduleTagScan), tagSchedule.Kind)
	assert.True(t, tagSchedule.Enabled)
	assert.Equal(t, 22, tagSchedule.StartHour)
	assert.Equal(t, 7, tagSchedule.EndHour)
}

func TestScheduleNextRunAtInUserTimezone(t *testing.T) {
	// Wednesday 2026-10-21 10:00 UTC; the user is UTC+2 (offset -120).
	now := time.Date(2026, time.October, 21, 10, 0, 0, 0, time.UTC)
	schedule := Schedule{
		Kind:           string(ScheduleGallerySync),
		Enabled:        true,
		Days:           "4", // Thursday only
		StartHour:      5,
		StartMinute:    0,
		TimezoneOffset: -120,
	}

	next := schedule.NextRunAt(now)

	userZone := time.FixedZone("UserTZ", 2*60*60)
	require.True(t, next.After(now), "next run must be in the future")
	assert.Equal(t, time.Thursday, next.In(userZone).Weekday())
	assert.Equal(t, 5, next.In(userZone).Hour())
	assert.Equal(t, 0, next.In(userZone).Minute())
}
