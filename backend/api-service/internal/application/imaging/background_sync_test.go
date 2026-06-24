package imaging

import (
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"
	"github.com/flashbacks/api-service/internal/testutil/fixtures"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBackgroundSyncManager(t *testing.T) (*BackgroundSyncManager, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	// Create manager without thumbnail service, geocoder, or exif client (nil)
	manager := NewBackgroundSyncManager(db, nil, nil, nil)
	return manager, cleanup
}

func TestBackgroundSyncManager_Start(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	bsm.Start([]time.Weekday{time.Monday, time.Friday}, 14, 0, 0)

	time.Sleep(50 * time.Millisecond)
	assert.True(t, bsm.isRunning(), "sync should be running")

	bsm.Stop()
}

func TestBackgroundSyncManager_Stop(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	bsm.Start([]time.Weekday{time.Monday}, 14, 0, 0)
	bsm.Stop()

	time.Sleep(50 * time.Millisecond)
	assert.False(t, bsm.isRunning(), "sync should not be running")
}

func TestBackgroundSyncManager_isRunning(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	assert.False(t, bsm.isRunning(), "should not be running initially")

	bsm.Start([]time.Weekday{time.Monday}, 14, 0, 0)
	assert.True(t, bsm.isRunning(), "should be running after start")

	bsm.Stop()
	assert.False(t, bsm.isRunning(), "should not be running after stop")
}

func TestBackgroundSyncManager_CalculateNextRunTime_FutureToday(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	now := time.Now()
	// Schedule for a time 5 minutes in the future today
	scheduleHour := now.Hour()
	scheduleMinute := now.Minute() + 5
	if scheduleMinute >= 60 {
		scheduleMinute -= 60
		scheduleHour = (scheduleHour + 1) % 24
	}

	// Use today's weekday
	syncDays := []time.Weekday{now.Weekday()}
	nextRun := bsm.calculateNextRunTime(syncDays, scheduleHour, scheduleMinute, 0)

	assert.True(t, nextRun.After(now), "next run should be in the future")
	assert.Equal(t, scheduleHour, nextRun.Hour())
	assert.Equal(t, scheduleMinute, nextRun.Minute())
}

func TestBackgroundSyncManager_CalculateNextRunTime_PastToday(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	now := time.Now()
	// Schedule for a time 2 hours in the past
	scheduleHour := (now.Hour() + 22) % 24
	scheduleMinute := now.Minute()

	// Only today's weekday - should skip to next week
	syncDays := []time.Weekday{now.Weekday()}
	nextRun := bsm.calculateNextRunTime(syncDays, scheduleHour, scheduleMinute, 0)

	// Should be next week (7 days from now)
	assert.True(t, nextRun.After(now), "next run should be in the future")
	assert.Equal(t, scheduleHour, nextRun.Hour())
	assert.Equal(t, scheduleMinute, nextRun.Minute())
}

func TestBackgroundSyncManager_CalculateNextRunTime_DifferentDay(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	// Schedule for a specific day and time
	now := time.Now()
	// Find the next Monday from now
	daysUntilMonday := (int(time.Monday) - int(now.Weekday()) + 7) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7 // next Monday, not today
	}

	syncDays := []time.Weekday{time.Monday}
	nextRun := bsm.calculateNextRunTime(syncDays, 10, 0, 0)

	assert.True(t, nextRun.After(now), "next run should be in the future")
	assert.Equal(t, time.Monday, nextRun.Weekday())
	assert.Equal(t, 10, nextRun.Hour())
	assert.Equal(t, 0, nextRun.Minute())
}

func TestBackgroundSyncManager_CalculateNextRunTime_EmptyDays(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	// With empty sync days, should not be called - but test the behavior
	// It falls through to the fallback (now + 24h)
	nextRun := bsm.calculateNextRunTime(nil, 3, 30, 0)

	now := time.Now()
	assert.True(t, nextRun.After(now), "fallback should be in the future")
}

func TestBackgroundSyncManager_CalculateNextRunTime_WithTimezone(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	// Test with UTC+3 (Moscow): timezoneOffset = -180 in JS convention
	// User wants sync at 03:30 local Moscow time
	tzOffset := -180 // UTC+3 in JS getTimezoneOffset convention

	now := time.Now()
	userTZ := time.FixedZone("UserTZ", -tzOffset*60) // = FixedZone("UserTZ", 10800) = UTC+3
	nowUser := now.In(userTZ)

	// Schedule for 5 min from now in user's local time
	scheduleHour := nowUser.Hour()
	scheduleMinute := nowUser.Minute() + 5
	if scheduleMinute >= 60 {
		scheduleMinute -= 60
		scheduleHour = (scheduleHour + 1) % 24
	}

	syncDays := []time.Weekday{nowUser.Weekday()}
	nextRun := bsm.calculateNextRunTime(syncDays, scheduleHour, scheduleMinute, tzOffset)

	// The returned time should be in the user's timezone
	assert.True(t, nextRun.After(now), "next run should be in the future")
	assert.Equal(t, scheduleHour, nextRun.Hour())
	assert.Equal(t, scheduleMinute, nextRun.Minute())
}

func TestBackgroundSyncManager_GetStatus(t *testing.T) {
	bsm, _ := setupBackgroundSyncManager(t)

	status := bsm.GetStatus()
	assert.False(t, status.Running)
	assert.Nil(t, status.NextRunAt)
	assert.Nil(t, status.LastSyncAt)
}

func TestBackgroundSyncManager_SyncFolder_NewFiles(t *testing.T) {
	bsm, cleanup := setupBackgroundSyncManager(t)
	defer cleanup()

	// Set running=true so syncFolder doesn't exit early
	bsm.mu.Lock()
	bsm.running = true
	bsm.mu.Unlock()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	// Sync folder with empty DB
	newCount, updatedCount, deletedCount, thumbCount, totalDisk := bsm.syncFolder(tmpDir, false)

	assert.Equal(t, 3, newCount, "should add 3 new files")
	assert.Equal(t, 0, updatedCount)
	assert.Equal(t, 0, deletedCount)
	assert.Equal(t, 0, thumbCount) // Thumbnails disabled
	assert.Equal(t, 3, totalDisk, "should report 3 total disk files")
}

func TestBackgroundSyncManager_SyncFolder_Unchanged(t *testing.T) {
	bsm, cleanup := setupBackgroundSyncManager(t)
	defer cleanup()

	// Set running=true so syncFolder doesn't exit early
	bsm.mu.Lock()
	bsm.running = true
	bsm.mu.Unlock()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	// First sync to populate DB
	bsm.syncFolder(tmpDir, false)

	// Second sync - all should be unchanged
	newCount, updatedCount, _, _, totalDisk := bsm.syncFolder(tmpDir, false)

	assert.Equal(t, 0, newCount)
	assert.Equal(t, 0, updatedCount, "no files should be updated")
	assert.Equal(t, 3, totalDisk, "should report 3 total disk files")
}

func TestBackgroundSyncManager_SyncFolder_DeletedFiles(t *testing.T) {
	bsm, cleanup := setupBackgroundSyncManager(t)
	defer cleanup()

	// Set running=true so syncFolder doesn't exit early
	bsm.mu.Lock()
	bsm.running = true
	bsm.mu.Unlock()

	tmpDir := fixtures.CreateTempDir(t)
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 5)

	// First sync to populate DB
	bsm.syncFolder(tmpDir, false)

	// Delete 2 files from disk
	require.GreaterOrEqual(t, len(paths), 2)
	fixtures.DeleteTestFile(t, paths[0])
	fixtures.DeleteTestFile(t, paths[1])

	// Cleanup should remove them
	deletedCount, cleanupTotal := bsm.cleanupMissingFiles()

	assert.Equal(t, 2, deletedCount, "should delete 2 missing file records")
	assert.Equal(t, 5, cleanupTotal, "should have checked 5 total DB records")
}

func TestBackgroundSyncManager_InvalidateTagsAndEmbeddings(t *testing.T) {
	bsm, cleanup := setupBackgroundSyncManager(t)
	defer cleanup()

	// Create an ImageFile
	img := testutil.SeedImageFile(t, bsm.db, "/tmp/test/invalidate.jpg", "abc123", 1024)

	// Seed tags and embeddings
	bsm.db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "landscape"})
	bsm.db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "nature"})
	bsm.db.Create(&domain.TagEmbedding{ImageFileID: img.ID, TagCount: 2})

	// Verify they exist
	var tagCountBefore, embCountBefore int64
	bsm.db.Model(&domain.ImageTag{}).Where("image_file_id = ?", img.ID).Count(&tagCountBefore)
	bsm.db.Model(&domain.TagEmbedding{}).Where("image_file_id = ?", img.ID).Count(&embCountBefore)
	require.Equal(t, int64(2), tagCountBefore)
	require.Equal(t, int64(1), embCountBefore)

	// Invalidate
	bsm.InvalidateTagsAndEmbeddings(img.ID)

	// Verify deleted
	var tagCountAfter, embCountAfter int64
	bsm.db.Model(&domain.ImageTag{}).Where("image_file_id = ?", img.ID).Count(&tagCountAfter)
	bsm.db.Model(&domain.TagEmbedding{}).Where("image_file_id = ?", img.ID).Count(&embCountAfter)
	assert.Equal(t, int64(0), tagCountAfter, "tags should be invalidated")
	assert.Equal(t, int64(0), embCountAfter, "embeddings should be invalidated")
}

func TestBackgroundSyncManager_CleanupMissingFiles_CascadeDeletesChildren(t *testing.T) {
	bsm, cleanup := setupBackgroundSyncManager(t)
	defer cleanup()

	// Set running=true so cleanupMissingFiles doesn't exit early
	bsm.mu.Lock()
	bsm.running = true
	bsm.mu.Unlock()

	// Create an ImageFile with a non-existent path and seed children
	img := testutil.SeedImageFile(t, bsm.db, "/tmp/nonexistent/deleted.jpg", "hash", 512)
	bsm.db.Create(&domain.ImageMetadata{ImageFileID: img.ID, Width: 100, Height: 100})
	bsm.db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "tag1"})
	bsm.db.Create(&domain.TagEmbedding{ImageFileID: img.ID, TagCount: 1})

	// Run cleanup
	deleted, cleanupTotal := bsm.cleanupMissingFiles()
	assert.Equal(t, 1, deleted)
	assert.Equal(t, 1, cleanupTotal, "should have checked 1 total DB record")

	// Verify parent and all children are gone
	var imgCount, metaCount, tagCount, embCount int64
	bsm.db.Model(&domain.ImageFile{}).Where("id = ?", img.ID).Count(&imgCount)
	bsm.db.Model(&domain.ImageMetadata{}).Where("image_file_id = ?", img.ID).Count(&metaCount)
	bsm.db.Model(&domain.ImageTag{}).Where("image_file_id = ?", img.ID).Count(&tagCount)
	bsm.db.Model(&domain.TagEmbedding{}).Where("image_file_id = ?", img.ID).Count(&embCount)
	assert.Equal(t, int64(0), imgCount, "ImageFile should be deleted")
	assert.Equal(t, int64(0), metaCount, "ImageMetadata should be deleted")
	assert.Equal(t, int64(0), tagCount, "ImageTag should be deleted")
	assert.Equal(t, int64(0), embCount, "TagEmbedding should be deleted")
}

func TestParseSyncDays(t *testing.T) {
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
		result := ParseSyncDays(tt.input)
		assert.Equal(t, tt.expected, result, "input: %q", tt.input)
	}
}

func TestFormatSyncDays(t *testing.T) {
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
		result := FormatSyncDays(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}
