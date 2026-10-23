package handler

import (
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/interfaces/dto"
	"github.com/flashbacks/core/internal/interfaces/handler/helpers"
	"github.com/flashbacks/core/internal/interfaces/i18n"
	"github.com/flashbacks/core/internal/interfaces/middleware"

	"github.com/gin-gonic/gin"
)

// appSettingsDTO builds the settings payload, taking the gallery sync schedule
// fields from the unified schedules table.
func appSettingsDTO(settings domain.AppSettings, syncSchedule domain.Schedule) dto.AppSettingsDTO {
	return dto.AppSettingsDTO{
		TrashDir:              settings.TrashDir,
		ExifBackupDir:         settings.ExifBackupDir,
		ThumbnailCachePath:    settings.ThumbnailCachePath,
		ThumbnailCacheSize:    settings.ThumbnailCacheSize,
		OcrConcurrentRequests: settings.OcrConcurrentRequests,
		SyncDays:              syncSchedule.Days,
		DailySyncHour:         syncSchedule.StartHour,
		DailySyncMinute:       syncSchedule.StartMinute,
		SyncTimezoneOffset:    syncSchedule.TimezoneOffset,
	}
}

// handleGetSettings returns the current application settings
func (s *Server) handleGetSettings(c *gin.Context) {
	settings := s.settingsLoader.AppSettings()
	syncSchedule := s.settingsLoader.Schedule(domain.ScheduleGallerySync)
	c.JSON(http.StatusOK, appSettingsDTO(settings, syncSchedule))
}

// handleUpdateSettings updates the application settings
func (s *Server) handleUpdateSettings(c *gin.Context) {
	var req dto.UpdateSettingsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	settings := s.settingsLoader.AppSettings()

	if req.TrashDir != nil {
		newTrashDir := strings.TrimSpace(*req.TrashDir)
		if newTrashDir != "" {
			absTrash, err := filepath.Abs(newTrashDir)
			if err != nil {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidTrashPath))
				return
			}
			normalizedTrash := filepath.ToSlash(absTrash)

			galleryFolders, _ := s.galleryFolderRepo.FindAll()
			for _, gf := range galleryFolders {
				if helpers.CheckPathsConflict(normalizedTrash, gf.Path) {
					c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageTrashConflict))
					return
				}
			}
			settings.TrashDir = normalizedTrash
		} else {
			settings.TrashDir = ""
		}
	}
	if req.ExifBackupDir != nil {
		newBackupDir := strings.TrimSpace(*req.ExifBackupDir)
		if newBackupDir != "" {
			absBackup, err := filepath.Abs(newBackupDir)
			if err != nil {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidBackupPath))
				return
			}
			normalizedBackup := filepath.ToSlash(absBackup)

			galleryFolders, _ := s.galleryFolderRepo.FindAll()
			for _, gf := range galleryFolders {
				if helpers.CheckPathsConflict(normalizedBackup, gf.Path) {
					c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageBackupConflict))
					return
				}
			}
			settings.ExifBackupDir = normalizedBackup
		} else {
			settings.ExifBackupDir = ""
		}
	}
	if req.ThumbnailCachePath != nil {
		newCachePath := strings.TrimSpace(*req.ThumbnailCachePath)
		if newCachePath != "" {
			absCache, err := filepath.Abs(newCachePath)
			if err != nil {
				c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidTrashPath))
				return
			}
			normalizedCache := filepath.ToSlash(absCache)
			settings.ThumbnailCachePath = normalizedCache

			if s.thumbnailProvider != nil {
				stats, _ := s.thumbnailProvider.GetStats()
				log.Printf("Updating thumbnail cache path from %s to %s", stats.CacheDir, normalizedCache)
				if err := s.thumbnailProvider.UpdateCachePath(normalizedCache); err != nil {
					log.Printf("Failed to update thumbnail cache path: %v", err)
				} else {
					newStats, _ := s.thumbnailProvider.GetStats()
					log.Printf("Thumbnail cache path updated successfully. New stats: %+v", newStats)
				}
			} else {
				log.Printf("Thumbnail service is nil, cannot update cache path")
			}
		} else {
			settings.ThumbnailCachePath = ""
		}
	}

	if req.OcrConcurrentRequests != nil {
		val := *req.OcrConcurrentRequests
		if val < 0 {
			val = 0
		}
		settings.OcrConcurrentRequests = val

		if s.ocrManager != nil {
			s.ocrManager.SetMaxWorkers(val)
		}
	}

	syncSchedule := s.settingsLoader.Schedule(domain.ScheduleGallerySync)
	scheduleChanged := false
	if req.SyncDays != nil {
		syncSchedule.Days = *req.SyncDays
		scheduleChanged = true
	}
	if req.DailySyncHour != nil {
		hour := *req.DailySyncHour
		if hour < 0 || hour > 23 {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.ValidationError))
			return
		}
		syncSchedule.StartHour = hour
		scheduleChanged = true
	}
	if req.DailySyncMinute != nil {
		minute := *req.DailySyncMinute
		if minute < 0 || minute > 59 {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.ValidationError))
			return
		}
		syncSchedule.StartMinute = minute
		scheduleChanged = true
	}
	if req.SyncTimezoneOffset != nil {
		syncSchedule.TimezoneOffset = *req.SyncTimezoneOffset
		scheduleChanged = true
	}

	if scheduleChanged {
		if err := s.scheduleRepo.Upsert(&syncSchedule); err != nil {
			log.Printf("Failed to save gallery sync schedule: %v", err)
			c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
			return
		}
		if s.backgroundSync != nil {
			s.backgroundSync.UpdateSchedule(syncSchedule)
		}
	}

	s.appSettingsRepo.Save(&settings)

	c.JSON(http.StatusOK, appSettingsDTO(settings, syncSchedule))
}

// handleGetUserSettings returns the current user's settings
func (s *Server) handleGetUserSettings(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	settings, err := s.userSettingsRepo.FindOrCreateByUserID(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	c.JSON(http.StatusOK, dto.UserSettingsDTO{
		Theme:    settings.Theme,
		Language: settings.Language,
	})
}

// handleUpdateUserSettings updates the current user's settings
func (s *Server) handleUpdateUserSettings(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	var req dto.UpdateUserSettingsRequest
	if !helpers.BindJSON(c, &req) {
		return
	}

	validThemes := map[string]bool{
		"light-purple":  true,
		"dark-purple":   true,
		"light-green":   true,
		"dark-green":    true,
		"light-blue":    true,
		"dark-blue":     true,
		"light-orange":  true,
		"dark-orange":   true,
		"dark-contrast": true,
	}
	validLanguages := map[string]bool{"en": true, "ru": true}

	if req.Theme != "" && !validThemes[req.Theme] {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidTheme))
		return
	}
	if req.Language != "" && !validLanguages[req.Language] {
		c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.MsgImageInvalidLanguage))
		return
	}

	settings, err := s.userSettingsRepo.FindOrCreateByUserID(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	if req.Theme != "" {
		settings.Theme = req.Theme
	}
	if req.Language != "" {
		settings.Language = req.Language
	}

	s.userSettingsRepo.Save(settings)

	c.JSON(http.StatusOK, dto.UserSettingsDTO{
		Theme:    settings.Theme,
		Language: settings.Language,
	})
}
