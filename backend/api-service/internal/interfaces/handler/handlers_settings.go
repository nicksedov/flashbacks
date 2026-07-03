package handler

import (
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"
	"github.com/flashbacks/api-service/internal/interfaces/middleware"

	"github.com/gin-gonic/gin"
)

// handleGetSettings returns the current application settings
func (s *Server) handleGetSettings(c *gin.Context) {
	settings := s.settingsLoader.AppSettings()
	c.JSON(http.StatusOK, dto.AppSettingsDTO{
		TrashDir:              settings.TrashDir,
		ExifBackupDir:         settings.ExifBackupDir,
		ThumbnailCachePath:    settings.ThumbnailCachePath,
		ThumbnailCacheSize:    settings.ThumbnailCacheSize,
		OcrConcurrentRequests: settings.OcrConcurrentRequests,
		SyncDays:              settings.SyncDays,
		DailySyncHour:         settings.DailySyncHour,
		DailySyncMinute:       settings.DailySyncMinute,
		SyncTimezoneOffset:    settings.SyncTimezoneOffset,
		LastSyncAt:            settings.LastSyncAt,
		LastSyncNew:           settings.LastSyncNew,
		LastSyncUpdated:       settings.LastSyncUpdated,
		LastSyncDeleted:       settings.LastSyncDeleted,
		LastSyncThumbnails:    settings.LastSyncThumbnails,
	})
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

			var galleryFolders []domain.GalleryFolder
			s.db.Find(&galleryFolders)
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

			var galleryFolders []domain.GalleryFolder
			s.db.Find(&galleryFolders)
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

			if s.thumbnailService != nil {
				log.Printf("Updating thumbnail cache path from %s to %s", s.thumbnailService.Stats().CacheDir, normalizedCache)
				if err := s.thumbnailService.UpdateCachePath(normalizedCache); err != nil {
					log.Printf("Failed to update thumbnail cache path: %v", err)
				} else {
					log.Printf("Thumbnail cache path updated successfully. New stats: %+v", s.thumbnailService.Stats())
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

	scheduleChanged := false
	if req.SyncDays != nil {
		settings.SyncDays = *req.SyncDays
		scheduleChanged = true
	}
	if req.DailySyncHour != nil {
		hour := *req.DailySyncHour
		if hour < 0 || hour > 23 {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.ValidationError))
			return
		}
		settings.DailySyncHour = hour
		scheduleChanged = true
	}
	if req.DailySyncMinute != nil {
		minute := *req.DailySyncMinute
		if minute < 0 || minute > 59 {
			c.JSON(http.StatusBadRequest, i18n.ErrorResponse(i18n.ValidationError))
			return
		}
		settings.DailySyncMinute = minute
		scheduleChanged = true
	}
	if req.SyncTimezoneOffset != nil {
		settings.SyncTimezoneOffset = *req.SyncTimezoneOffset
		scheduleChanged = true
	}

	if s.backgroundSync != nil && scheduleChanged {
		syncDays := imaging.ParseSyncDays(settings.SyncDays)
		s.backgroundSync.UpdateSchedule(syncDays, settings.DailySyncHour, settings.DailySyncMinute, settings.SyncTimezoneOffset)
	}

	s.db.Save(&settings)

	c.JSON(http.StatusOK, dto.AppSettingsDTO{
		TrashDir:              settings.TrashDir,
		ExifBackupDir:         settings.ExifBackupDir,
		ThumbnailCachePath:    settings.ThumbnailCachePath,
		ThumbnailCacheSize:    settings.ThumbnailCacheSize,
		OcrConcurrentRequests: settings.OcrConcurrentRequests,
		SyncDays:              settings.SyncDays,
		DailySyncHour:         settings.DailySyncHour,
		DailySyncMinute:       settings.DailySyncMinute,
		SyncTimezoneOffset:    settings.SyncTimezoneOffset,
		LastSyncAt:            settings.LastSyncAt,
		LastSyncNew:           settings.LastSyncNew,
		LastSyncUpdated:       settings.LastSyncUpdated,
		LastSyncDeleted:       settings.LastSyncDeleted,
		LastSyncThumbnails:    settings.LastSyncThumbnails,
	})
}

// handleGetUserSettings returns the current user's settings
func (s *Server) handleGetUserSettings(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, i18n.ErrorResponse(i18n.MsgAuthUnauthorized))
		return
	}

	var settings domain.UserSettings
	if result := s.db.FirstOrCreate(&settings, domain.UserSettings{UserID: user.ID}); result.Error != nil {
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

	var settings domain.UserSettings
	result := s.db.FirstOrCreate(&settings, domain.UserSettings{UserID: user.ID})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, i18n.ErrorResponse(i18n.MsgAuthInternalError))
		return
	}

	if req.Theme != "" {
		settings.Theme = req.Theme
	}
	if req.Language != "" {
		settings.Language = req.Language
	}

	s.db.Save(&settings)

	c.JSON(http.StatusOK, dto.UserSettingsDTO{
		Theme:    settings.Theme,
		Language: settings.Language,
	})
}