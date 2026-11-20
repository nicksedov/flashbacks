package repository

import (
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/infrastructure/cache"
	"github.com/flashbacks/core/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppSettingsRepository_CachesReadsAndInvalidatesOnWrite(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// The test database seeds the app_settings singleton.
	require.NoError(t, db.Model(&domain.AppSettings{}).Where("id = ?", appSettingsSingletonID).
		Update("trash_dir", "/old").Error)

	cacheTTL := cache.NewTTLCache[int, *domain.AppSettings](time.Minute, nil)
	repo := NewAppSettingsRepository(db, cacheTTL)

	first, err := repo.Get()
	require.NoError(t, err)
	assert.Equal(t, "/old", first.TrashDir)
	assert.Equal(t, 1, cacheTTL.Len(), "a read must populate the cache")

	// A write that bypasses the repository is invisible while the entry is
	// fresh: this is the cache hit the spec asks for (no database read).
	require.NoError(t, db.Model(&domain.AppSettings{}).Where("id = ?", appSettingsSingletonID).
		Update("trash_dir", "/direct").Error)
	second, err := repo.Get()
	require.NoError(t, err)
	assert.Equal(t, "/old", second.TrashDir, "the cached value must be served")

	// An Update through the repository invalidates, so the next read is fresh.
	require.NoError(t, repo.Update(map[string]interface{}{"trash_dir": "/via-update"}))
	assert.Equal(t, 0, cacheTTL.Len(), "Update must invalidate the cache")
	third, err := repo.Get()
	require.NoError(t, err)
	assert.Equal(t, "/via-update", third.TrashDir)

	// Save invalidates too.
	third.ExifBackupDir = "/backup"
	require.NoError(t, repo.Save(third))
	assert.Equal(t, 0, cacheTTL.Len(), "Save must invalidate the cache")
	fourth, err := repo.Get()
	require.NoError(t, err)
	assert.Equal(t, "/backup", fourth.ExifBackupDir)
}

func TestAppSettingsRepository_ReturnsCopy(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	require.NoError(t, db.Model(&domain.AppSettings{}).Where("id = ?", appSettingsSingletonID).
		Update("trash_dir", "/old").Error)

	cacheTTL := cache.NewTTLCache[int, *domain.AppSettings](time.Minute, nil)
	repo := NewAppSettingsRepository(db, cacheTTL)

	first, err := repo.Get()
	require.NoError(t, err)
	first.TrashDir = "/mutated"

	second, err := repo.Get()
	require.NoError(t, err)
	assert.Equal(t, "/old", second.TrashDir, "the cached snapshot must not be mutable through the result")
}

func TestUserSettingsRepository_CachesReadsAndInvalidatesOnSave(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	const userID = uint(7)
	cacheTTL := cache.NewTTLCache[uint, *domain.UserSettings](time.Minute, nil)
	repo := NewUserSettingsRepository(db, cacheTTL)

	first, err := repo.FindOrCreateByUserID(userID)
	require.NoError(t, err)
	originalTheme := first.Theme
	assert.Equal(t, 1, cacheTTL.Len(), "the read must populate the cache")

	// A direct database write stays invisible until the entry is evicted.
	require.NoError(t, db.Model(&domain.UserSettings{}).Where("user_id = ?", userID).
		Update("theme", "dark").Error)
	second, err := repo.FindOrCreateByUserID(userID)
	require.NoError(t, err)
	assert.Equal(t, originalTheme, second.Theme, "the cached value must be served")

	second.Theme = "midnight"
	require.NoError(t, repo.Save(second))
	assert.Equal(t, 0, cacheTTL.Len(), "Save must invalidate the cache")

	third, err := repo.FindOrCreateByUserID(userID)
	require.NoError(t, err)
	assert.Equal(t, "midnight", third.Theme)
}
