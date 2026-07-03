package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/application/auth"
	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/application/thumbnail"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/config"
	"github.com/flashbacks/api-service/internal/interfaces/handler"
	"github.com/flashbacks/api-service/internal/interfaces/middleware"

	"gorm.io/gorm"
)

// App is the top-level application container. All dependencies are injected
// by Wire; runtime initialization (callbacks, background tasks) is performed
// by Init(). The server is started by Run() and cleaned up by Shutdown().
type App struct {
	DB                *gorm.DB
	Config            *config.AppConfig
	Server            *handler.Server
	ScanManager       *imaging.ScanManager
	OcrManager        *imaging.OcrManager
	LlmOcrService     *imaging.LlmOcrService
	BackgroundSync    *imaging.BackgroundSyncManager
	TagScanManager    *imaging.TagScanManager
	EmbeddingBackfill *imaging.EmbeddingBackfillManager
	ThumbnailService  *thumbnail.Service
	LoginLimiter      *auth.LoginRateLimiter
	SessionCleanup    *auth.SessionCleanupJob

	// HTTP layer — injected by Wire via AuthSet providers.
	AuthMiddleware *middleware.AuthMiddleware
	CSRFProtection *middleware.CSRFProtection
	AuthHandlers   *handler.AuthHandlers
}

// Init performs runtime wiring that cannot be expressed statically:
// callbacks, coordinator assignment, background task scheduling, and
// starting the thumbnail service.
func (app *App) Init() {
	// Read background sync schedule from DB
	syncHour := 3
	syncMinute := 30
	syncDays := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	syncTimezoneOffset := 0
	var appSettings domain.AppSettings
	if result := app.DB.First(&appSettings, 1); result.Error == nil {
		if appSettings.DailySyncHour > 0 || appSettings.DailySyncMinute > 0 {
			syncHour = appSettings.DailySyncHour
			syncMinute = appSettings.DailySyncMinute
		}
		syncDays = imaging.ParseSyncDays(appSettings.SyncDays)
		syncTimezoneOffset = appSettings.SyncTimezoneOffset
	}
	app.BackgroundSync.Start(syncDays, syncHour, syncMinute, syncTimezoneOffset)
	fmt.Printf("Background sync: days=%v at %02d:%02d, tzOffset=%d min\n", syncDays, syncHour, syncMinute, syncTimezoneOffset)

	// Wire scan-complete callback → OCR classification
	app.ScanManager.OnScanComplete = func() {
		if app.Config.OCREnabled && app.OcrManager != nil {
			if err := app.OcrManager.StartClassification(false); err != nil {
				log.Printf("OCR classification not started: %v", err)
			}
		}
	}

	// Wire per-file post-processor for EXIF, thumbnails, and tag/embedding invalidation
	app.ScanManager.OnFileProcessed = func(event imaging.FileEvent) {
		switch event.Type {
		case imaging.FileCreated:
			app.BackgroundSync.ExtractAndSaveMetadataAsync(event.Path, event.ImageFileID)
			if app.ThumbnailService != nil {
				go func() {
					if _, err := app.ThumbnailService.GetOrGenerate(event.Path); err != nil {
						log.Printf("Post-processor: failed to generate thumbnail for %s: %v", event.Path, err)
					}
				}()
			}
		case imaging.FileModified:
			if event.ContentChanged {
				app.BackgroundSync.ExtractAndSaveMetadataAsync(event.Path, event.ImageFileID)
				app.BackgroundSync.InvalidateOCRClassificationAsync(event.ImageFileID)
				app.BackgroundSync.InvalidateTagsAndEmbeddingsAsync(event.ImageFileID)
				if app.ThumbnailService != nil {
					go func() {
						app.ThumbnailService.Invalidate(event.Path)
						if _, err := app.ThumbnailService.GetOrGenerate(event.Path); err != nil {
							log.Printf("Post-processor: failed to regenerate thumbnail for %s: %v", event.Path, err)
						}
					}()
				}
			}
		case imaging.FileDeleted:
			if app.ThumbnailService != nil {
				app.ThumbnailService.Invalidate(event.Path)
			}
		}
	}

	// Start thumbnail service
	if app.ThumbnailService != nil {
		if err := app.ThumbnailService.Start(); err != nil {
			log.Printf("Failed to start thumbnail service: %v", err)
		}
	}

	// Read tag scan schedule from LlmSettings
	tagScanEnabled := true
	tagScanStartHour := 22
	tagScanStartMinute := 0
	tagScanEndHour := 7
	tagScanEndMinute := 0
	tagScanTimezoneOffset := 0
	var llmSettings domain.LlmSettings
	if result := app.DB.First(&llmSettings); result.Error == nil {
		tagScanEnabled = llmSettings.TagScanEnabled
		tagScanStartHour = llmSettings.TagScanStartHour
		tagScanStartMinute = llmSettings.TagScanStartMinute
		tagScanEndHour = llmSettings.TagScanEndHour
		tagScanEndMinute = llmSettings.TagScanEndMinute
		tagScanTimezoneOffset = llmSettings.TagScanTimezoneOffset
	}
	app.TagScanManager.Start(tagScanEnabled, tagScanStartHour, tagScanStartMinute, tagScanEndHour, tagScanEndMinute, tagScanTimezoneOffset)
	fmt.Printf("Tag scan: window %02d:%02d - %02d:%02d, tzOffset=%d, enabled=%v\n", tagScanStartHour, tagScanStartMinute, tagScanEndHour, tagScanEndMinute, tagScanTimezoneOffset, tagScanEnabled)

	// Set coordinator for AI task synchronization
	app.LlmOcrService.SetCoordinator(app.TagScanManager)
	// Wire embedding backfill to tag scan manager
	app.TagScanManager.SetEmbeddingBackfill(app.EmbeddingBackfill)

	// Start session cleanup
	app.SessionCleanup.Start()
}

// PrintBanner prints the startup banner with configuration summary.
func (app *App) PrintBanner() {
	cachePath := app.Config.ThumbnailCachePath
	if cachePath == "" {
		var appSettings domain.AppSettings
		if result := app.DB.First(&appSettings, 1); result.Error == nil && appSettings.ThumbnailCachePath != "" {
			cachePath = appSettings.ThumbnailCachePath
		}
	}

	fmt.Printf("Image Dedup - API Server\n")
	fmt.Printf("========================\n\n")
	fmt.Printf("Starting API server on http://%s:%s\n", app.Config.ServerHost, app.Config.ServerPort)
	fmt.Printf("Scan workers: %d\n", app.Config.ScanWorkers)
	fmt.Printf("CORS allowed origins: %s\n", strings.Join(app.Config.CORSOrigins, ", "))
	fmt.Printf("Thumbnail cache: enabled=%v, path=%s\n", app.Config.ThumbnailCacheEnabled, cachePath)
	fmt.Println("Configure gallery folders via the web UI Settings tab.")
	fmt.Println("Press Ctrl+C to stop the server")
}

// Run sets up the router and starts the HTTP server.
func (app *App) Run() error {
	// Build router
	router := app.Server.SetupRouter(app.AuthMiddleware, app.CSRFProtection, app.AuthHandlers)

	// Start OCR health check
	app.Server.StartOCRHealthCheck()

	return router.Run(fmt.Sprintf("%s:%s", app.Config.ServerHost, app.Config.ServerPort))
}

// Shutdown gracefully stops all background tasks and releases resources.
func (app *App) Shutdown() {
	if app.Server != nil {
		app.Server.StopOCRHealthCheck()
	}
	if app.BackgroundSync != nil {
		app.BackgroundSync.Stop()
	}
	if app.LlmOcrService != nil {
		app.LlmOcrService.Stop()
	}
	if app.TagScanManager != nil {
		app.TagScanManager.Stop()
	}
	if app.LoginLimiter != nil {
		app.LoginLimiter.Stop()
	}
	if app.SessionCleanup != nil {
		app.SessionCleanup.Stop()
	}
	if app.DB != nil {
		if sqlDB, err := app.DB.DB(); err == nil {
			sqlDB.Close()
		}
	}
}
