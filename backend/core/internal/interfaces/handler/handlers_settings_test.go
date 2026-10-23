package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	"github.com/flashbacks/core/internal/interfaces/dto"
	"github.com/flashbacks/core/internal/interfaces/handler/helpers"
	"github.com/flashbacks/core/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newSettingsTestServer builds a minimal Server wired for settings routing tests.
func newSettingsTestServer(t *testing.T) (*Server, *gorm.DB, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)

	svc := &Server{
		db:              db,
		settingsLoader:  helpers.NewSettingsLoader(db),
		appSettingsRepo: repository.NewAppSettingsRepository(db),
		scheduleRepo:    repository.NewScheduleRepository(db),
	}
	return svc, db, cleanup
}

func TestUpdateSettingsPersistsGallerySyncSchedule(t *testing.T) {
	svc, db, cleanup := newSettingsTestServer(t)
	defer cleanup()

	days := "2,4"
	hour := 5
	minute := 10
	tzOffset := -180

	w := serveHandler(svc.handleUpdateSettings, http.MethodPut, "/api/settings", dto.UpdateSettingsRequest{
		SyncDays:           &days,
		DailySyncHour:      &hour,
		DailySyncMinute:    &minute,
		SyncTimezoneOffset: &tzOffset,
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var stored domain.Schedule
	require.NoError(t, db.Where("kind = ?", string(domain.ScheduleGallerySync)).First(&stored).Error)
	assert.Equal(t, "2,4", stored.Days)
	assert.Equal(t, 5, stored.StartHour)
	assert.Equal(t, 10, stored.StartMinute)
	assert.Equal(t, -180, stored.TimezoneOffset)

	// The public settings payload keeps its shape and reports the stored schedule.
	w = serveHandler(svc.handleGetSettings, http.MethodGet, "/api/settings", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var resp dto.AppSettingsDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "2,4", resp.SyncDays)
	assert.Equal(t, 5, resp.DailySyncHour)
	assert.Equal(t, 10, resp.DailySyncMinute)
	assert.Equal(t, -180, resp.SyncTimezoneOffset)
}
