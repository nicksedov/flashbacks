package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/flashbacks/core/internal/application/auth"
	"github.com/flashbacks/core/internal/application/imaging"
	"github.com/flashbacks/core/internal/application/thumbnail"
	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	"github.com/flashbacks/core/internal/infrastructure/config"
	"github.com/flashbacks/core/internal/infrastructure/di"
	"github.com/flashbacks/core/internal/infrastructure/registrybootstrap"
	"github.com/flashbacks/core/internal/interfaces/handler"
	"github.com/flashbacks/core/internal/interfaces/middleware"

	"gorm.io/gorm"
)

// App is the top-level application container. All dependencies are injected
// by Wire; runtime initialization (callbacks, background tasks) is performed
// by Init(). The server is started by Run() and cleaned up by Shutdown().
type App struct {
	DB                *gorm.DB
	Config            *config.AppConfig
	ScheduleRepo      repository.ScheduleRepository
	Server            *handler.Server
	ScanManager       *imaging.ScanManager
	OcrManager        *imaging.OcrManager
	LlmOcrService     *imaging.LlmOcrService
	BackgroundSync    *imaging.BackgroundSyncManager
	TagScanManager    *imaging.TagScanManager
	EmbeddingBackfill *imaging.EmbeddingBackfillManager
	ThumbnailService  thumbnail.ThumbnailProvider
	LoginLimiter      di.LoginLimiter
	SessionCleanup    *auth.SessionCleanupJob
	Registry          *registrybootstrap.Bootstrap

	// HTTP layer — injected by Wire via AuthSet providers.
	AuthMiddleware *middleware.AuthMiddleware
	CSRFProtection *middleware.CSRFProtection
	AuthHandlers   *handler.AuthHandlers
}

// Init performs runtime wiring that cannot be expressed statically:
// callbacks, coordinator assignment, background task scheduling, and
// starting the thumbnail service.
func (app *App) Init() {
	// Read the job schedules from the unified schedules table
	gallerySchedule := app.scheduleOrDefault(domain.ScheduleGallerySync)
	app.BackgroundSync.Start(gallerySchedule)
	fmt.Printf("Background sync: days=%v at %02d:%02d, tzOffset=%d min\n",
		gallerySchedule.Weekdays(), gallerySchedule.StartHour, gallerySchedule.StartMinute, gallerySchedule.TimezoneOffset)

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

	// Read the tagging schedule from the unified schedules table
	tagScanSchedule := app.scheduleOrDefault(domain.ScheduleTagScan)
	app.TagScanManager.Start(tagScanSchedule)
	fmt.Printf("Tag scan: days=%v, window %02d:%02d - %02d:%02d, tzOffset=%d, enabled=%v\n",
		tagScanSchedule.Weekdays(), tagScanSchedule.StartHour, tagScanSchedule.StartMinute, tagScanSchedule.EndHour, tagScanSchedule.EndMinute, tagScanSchedule.TimezoneOffset, tagScanSchedule.Enabled)

	// Set coordinator for AI task synchronization
	app.LlmOcrService.SetCoordinator(app.TagScanManager)
	// Wire embedding backfill to tag scan manager
	app.TagScanManager.SetEmbeddingBackfill(app.EmbeddingBackfill)

	// Start session cleanup
	app.SessionCleanup.Start()

	// Advertise this instance once the database is ready.
	if app.Registry != nil && app.Registry.Enabled() {
		app.Registry.Register(context.Background(), "core", registrybootstrap.AdvertiseHost(),
			registrybootstrap.PortInt(app.Config.ServerPort, 5170), "/api", "/api/auth/status", registrybootstrap.Version())
	}
}

// scheduleOrDefault loads the schedule of a job kind, falling back to the
// built-in defaults when the row is missing or unreadable.
func (app *App) scheduleOrDefault(kind domain.ScheduleKind) domain.Schedule {
	if app.ScheduleRepo != nil {
		if schedule, err := app.ScheduleRepo.Get(kind); err == nil && schedule != nil {
			return *schedule
		}
	}
	return domain.DefaultSchedule(kind)
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
	if app.LoginLimiter.LoginRateLimiter != nil {
		app.LoginLimiter.Stop()
	}
	if app.SessionCleanup != nil {
		app.SessionCleanup.Stop()
	}
	if app.Registry != nil {
		ctx, cancel := context.WithTimeout(context.Background(), registrybootstrap.ShutdownTimeout)
		app.Registry.Deregister(ctx)
		cancel()
		app.Registry.Close()
	}
	if app.DB != nil {
		if sqlDB, err := app.DB.DB(); err == nil {
			sqlDB.Close()
		}
	}
}
