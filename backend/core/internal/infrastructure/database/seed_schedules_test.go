package database

import (
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedSchedulesBackfillsLegacyColumns(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	require.NoError(t, db.Exec("DELETE FROM schedules").Error)

	// Simulate a pre-unification database: legacy columns still hold values.
	require.NoError(t, db.Exec("ALTER TABLE app_settings ADD COLUMN sync_days text").Error)
	require.NoError(t, db.Exec("ALTER TABLE app_settings ADD COLUMN daily_sync_hour integer DEFAULT 0").Error)
	require.NoError(t, db.Exec("ALTER TABLE app_settings ADD COLUMN daily_sync_minute integer DEFAULT 0").Error)
	require.NoError(t, db.Exec("ALTER TABLE app_settings ADD COLUMN sync_timezone_offset integer DEFAULT 0").Error)
	require.NoError(t, db.Exec(`UPDATE app_settings SET sync_days = '2,4', daily_sync_hour = 5, daily_sync_minute = 15, sync_timezone_offset = -180 WHERE id = 1`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE tag_scan_settings (
		id integer primary key,
		enabled numeric,
		days text,
		start_hour integer,
		start_minute integer,
		end_hour integer,
		end_minute integer,
		timezone_offset integer
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tag_scan_settings (id, enabled, days, start_hour, start_minute, end_hour, end_minute, timezone_offset)
		VALUES (1, 1, '6,0', 21, 30, 6, 15, -120)`).Error)

	require.NoError(t, seedSchedules(db))

	var syncSchedule domain.Schedule
	require.NoError(t, db.Where("kind = ?", string(domain.ScheduleGallerySync)).First(&syncSchedule).Error)
	assert.Equal(t, "2,4", syncSchedule.Days)
	assert.Equal(t, 5, syncSchedule.StartHour)
	assert.Equal(t, 15, syncSchedule.StartMinute)
	assert.Equal(t, -180, syncSchedule.TimezoneOffset)

	var tagSchedule domain.Schedule
	require.NoError(t, db.Where("kind = ?", string(domain.ScheduleTagScan)).First(&tagSchedule).Error)
	assert.True(t, tagSchedule.Enabled)
	assert.Equal(t, "6,0", tagSchedule.Days)
	assert.Equal(t, 21, tagSchedule.StartHour)
	assert.Equal(t, 30, tagSchedule.StartMinute)
	assert.Equal(t, 6, tagSchedule.EndHour)
	assert.Equal(t, 15, tagSchedule.EndMinute)
	assert.Equal(t, -120, tagSchedule.TimezoneOffset)
}

func TestSeedSchedulesUsesDefaultsWithoutLegacyColumns(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	require.NoError(t, db.Exec("DELETE FROM schedules").Error)

	require.NoError(t, seedSchedules(db))

	var syncSchedule domain.Schedule
	require.NoError(t, db.Where("kind = ?", string(domain.ScheduleGallerySync)).First(&syncSchedule).Error)
	assert.Equal(t, "1,2,3,4,5", syncSchedule.Days)
	assert.Equal(t, 3, syncSchedule.StartHour)
	assert.Equal(t, 30, syncSchedule.StartMinute)

	var tagSchedule domain.Schedule
	require.NoError(t, db.Where("kind = ?", string(domain.ScheduleTagScan)).First(&tagSchedule).Error)
	assert.True(t, tagSchedule.Enabled)
	assert.Equal(t, 22, tagSchedule.StartHour)
	assert.Equal(t, 7, tagSchedule.EndHour)
}

func TestSeedSchedulesKeepsEditedRows(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	var edited domain.Schedule
	require.NoError(t, db.Where("kind = ?", string(domain.ScheduleTagScan)).First(&edited).Error)
	edited.Days = "2"
	require.NoError(t, db.Save(&edited).Error)

	require.NoError(t, seedSchedules(db))

	var reloaded domain.Schedule
	require.NoError(t, db.Where("kind = ?", string(domain.ScheduleTagScan)).First(&reloaded).Error)
	assert.Equal(t, "2", reloaded.Days, "a restart must not overwrite an edited schedule")

	var count int64
	require.NoError(t, db.Model(&domain.Schedule{}).Where("kind = ?", string(domain.ScheduleTagScan)).Count(&count).Error)
	assert.Equal(t, int64(1), count, "the seed must not duplicate an existing kind")
}
