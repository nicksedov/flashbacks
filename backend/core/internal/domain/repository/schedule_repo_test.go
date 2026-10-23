package repository

import (
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleRepositoryGetMissingKind(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	repo := NewScheduleRepository(db)

	_, err := repo.Get("unknown_job")
	require.Error(t, err, "a missing kind must report an error instead of a zero schedule")
}

func TestScheduleRepositoryUpsertCreatesAndUpdates(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	repo := NewScheduleRepository(db)

	created := domain.DefaultSchedule(domain.ScheduleGallerySync)
	created.Days = "1,3,5"
	created.StartHour = 4
	require.NoError(t, repo.Upsert(&created))

	stored, err := repo.Get(domain.ScheduleGallerySync)
	require.NoError(t, err)
	assert.Equal(t, "1,3,5", stored.Days)
	assert.Equal(t, 4, stored.StartHour)

	// A second upsert mutates the same row instead of creating a duplicate.
	updated := domain.DefaultSchedule(domain.ScheduleGallerySync)
	updated.Days = "2"
	updated.StartHour = 0
	updated.StartMinute = 15
	require.NoError(t, repo.Upsert(&updated))

	stored, err = repo.Get(domain.ScheduleGallerySync)
	require.NoError(t, err)
	assert.Equal(t, "2", stored.Days)
	assert.Equal(t, 0, stored.StartHour, "zero values must be persisted on update")
	assert.Equal(t, 15, stored.StartMinute)

	var count int64
	require.NoError(t, db.Model(&domain.Schedule{}).Where("kind = ?", string(domain.ScheduleGallerySync)).Count(&count).Error)
	assert.Equal(t, int64(1), count, "the second upsert must update the row, not add a duplicate")

	all, err := repo.GetAll()
	require.NoError(t, err)
	assert.Len(t, all, 2, "the fixture seeds the gallery sync and tag scan kinds")
}

func TestScheduleRepositoryUpsertKeepsEmptyDays(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	repo := NewScheduleRepository(db)

	empty := domain.DefaultSchedule(domain.ScheduleTagScan)
	empty.Days = ""
	require.NoError(t, repo.Upsert(&empty))

	stored, err := repo.Get(domain.ScheduleTagScan)
	require.NoError(t, err)
	assert.Equal(t, "", stored.Days, "an empty weekday set means disabled and must be stored as-is")
}
