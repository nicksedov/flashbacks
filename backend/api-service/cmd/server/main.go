package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/joho/godotenv"

	agentpkg "github.com/flashbacks/api-service/internal/application/agent"
	"github.com/flashbacks/api-service/internal/application/auth"
	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/application/thumbnail"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/config"
	"github.com/flashbacks/api-service/internal/infrastructure/database"
	"github.com/flashbacks/api-service/internal/infrastructure/exifclient"
	"github.com/flashbacks/api-service/internal/infrastructure/geocoder"
	"github.com/flashbacks/api-service/internal/infrastructure/mcpserver"
	"github.com/flashbacks/api-service/internal/infrastructure/ocr"
	"github.com/flashbacks/api-service/internal/interfaces/handler"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"
	"github.com/flashbacks/api-service/internal/interfaces/middleware"
)

// init is invoked before main()
func init() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}
}

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	fmt.Printf("Image Dedup - API Server\n")
	fmt.Printf("========================\n\n")

	// Initialize database
	fmt.Println("Connecting to PostgreSQL database...")
	db, err := database.InitDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	fmt.Println("Database connected successfully!")

	// Initialize EXIF service client
	fmt.Println("Initializing EXIF service client...")
	exifSvcClient := exifclient.NewHTTPExifClient(cfg.ExifServiceURL)
	fmt.Printf("EXIF service client initialized: %s\n", cfg.ExifServiceURL)

	// Initialize OCR classifier client and health check
	var ocrCheckInterval int
	if cfg.OCREnabled {
		fmt.Println("Initializing OCR classifier client...")
		ocrCheckInterval = cfg.OCRCheckInterval
		fmt.Printf("OCR classifier enabled: serviceURL=%s, check interval=%ds\n", cfg.OCRServiceURL, ocrCheckInterval)
	} else {
		fmt.Println("OCR classifier integration disabled")
		ocrCheckInterval = 0
	}

	// Create scan manager (reads gallery folders from DB dynamically)
	scanManager := imaging.NewScanManager(db, cfg.ScanWorkers)

	// Create OCR manager (background classification)
	var ocrManager *imaging.OcrManager
	if cfg.OCREnabled {
		ocrClient := ocr.NewClient(cfg.OCRServiceURL)

		// Read OCR concurrent requests from DB, fallback to env var (default: 4)
		ocrWorkers := cfg.OCRConcurrentRequests
		var appSettings domain.AppSettings
		if result := db.First(&appSettings, 1); result.Error == nil && appSettings.OcrConcurrentRequests > 0 {
			ocrWorkers = appSettings.OcrConcurrentRequests
		}

		ocrManager = imaging.NewOcrManager(db, ocrClient, ocrWorkers)
		fmt.Printf("OCR manager initialized: max concurrent requests=%d\n", ocrWorkers)
	}

	// Initialize thumbnail cache service
	var thumbnailService *thumbnail.Service
	// Initialize thumbnail cache service
	fmt.Println("Initializing thumbnail cache service...")

	// Load thumbnail cache path from database if available
	cachePath := cfg.ThumbnailCachePath
	if cfg.ThumbnailCachePath == "" {
		// Check database for saved cache path
		var appSettings domain.AppSettings
		if result := db.First(&appSettings, 1); result.Error == nil && appSettings.ThumbnailCachePath != "" {
			cachePath = appSettings.ThumbnailCachePath
			fmt.Printf("Using thumbnail cache path from database: %s\n", cachePath)
		}
	}

	tcConfig := &thumbnail.Config{
		CacheDir:      cachePath,
		MaxSize:       cfg.ThumbnailCacheMaxSize,
		Quality:       cfg.ThumbnailCacheQuality,
		Enabled:       cfg.ThumbnailCacheEnabled,
		Format:        "webp",
		PreloadOnScan: cfg.ThumbnailCachePreloadOnScan,
	}
	thumbnailService, err = thumbnail.NewService(tcConfig)
	if err != nil {
		log.Printf("Failed to initialize thumbnail cache: %v", err)
		thumbnailService = nil
	} else {
		if cfg.ThumbnailCacheEnabled {
			fmt.Println("Thumbnail cache service initialized and enabled")
		} else {
			fmt.Println("Thumbnail cache service initialized (disabled)")
		}
	}

	// Start thumbnail service
	if thumbnailService != nil {
		if err := thumbnailService.Start(); err != nil {
			log.Printf("Failed to start thumbnail service: %v", err)
		}
	}

	// Initialize Nominatim client for geocoding (forward search + reverse geocoding)
	nominatimClient := geocoder.NewNominatimClient(nil, "")
	fmt.Println("Nominatim geocoding client initialized")

	// Initialize GeolocationService (cache + rate-limited Nominatim reverse geocoding)
	geolocationService := geocoder.NewGeolocationService(db, nominatimClient)
	fmt.Println("Geolocation service initialized (Nominatim-backed cache)")

	// Create background sync manager
	backgroundSync := imaging.NewBackgroundSyncManager(db, thumbnailService, geolocationService, exifSvcClient)

	// Read schedule from DB (default: weekdays, 03:30, UTC)
	syncHour := 3
	syncMinute := 30
	syncDays := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	syncTimezoneOffset := 0
	var appSettings domain.AppSettings
	if result := db.First(&appSettings, 1); result.Error == nil {
		if appSettings.DailySyncHour > 0 || appSettings.DailySyncMinute > 0 {
			syncHour = appSettings.DailySyncHour
			syncMinute = appSettings.DailySyncMinute
		}
		syncDays = imaging.ParseSyncDays(appSettings.SyncDays)
		syncTimezoneOffset = appSettings.SyncTimezoneOffset
	}

	backgroundSync.Start(syncDays, syncHour, syncMinute, syncTimezoneOffset)
	defer backgroundSync.Stop()
	fmt.Printf("Background sync: days=%v at %02d:%02d, tzOffset=%d min\n", syncDays, syncHour, syncMinute, syncTimezoneOffset)

	// Wire scan complete callback to trigger OCR classification
	scanManager.OnScanComplete = func() {
		if cfg.OCREnabled && ocrManager != nil {
			if err := ocrManager.StartClassification(false); err != nil {
				log.Printf("OCR classification not started: %v", err)
			}
		}
	}

	// Wire per-file post-processor for EXIF extraction, thumbnail generation, and invalidation
	scanManager.OnFileProcessed = func(event imaging.FileEvent) {
		switch event.Type {
		case imaging.FileCreated:
			// Extract EXIF/geo metadata for new files
			backgroundSync.ExtractAndSaveMetadataAsync(event.Path, event.ImageFileID)
			// Generate thumbnail for new files
			if thumbnailService != nil {
				go func() {
					if _, err := thumbnailService.GetOrGenerate(event.Path); err != nil {
						log.Printf("Post-processor: failed to generate thumbnail for %s: %v", event.Path, err)
					}
				}()
			}
		case imaging.FileModified:
			if event.ContentChanged {
				// Re-extract EXIF/geo metadata
				backgroundSync.ExtractAndSaveMetadataAsync(event.Path, event.ImageFileID)
				// Invalidate OCR classification
				backgroundSync.InvalidateOCRClassificationAsync(event.ImageFileID)
				// Invalidate AI tags and embeddings
				backgroundSync.InvalidateTagsAndEmbeddingsAsync(event.ImageFileID)
				// Regenerate thumbnail
				if thumbnailService != nil {
					go func() {
						thumbnailService.Invalidate(event.Path)
						if _, err := thumbnailService.GetOrGenerate(event.Path); err != nil {
							log.Printf("Post-processor: failed to regenerate thumbnail for %s: %v", event.Path, err)
						}
					}()
				}
			}
		case imaging.FileDeleted:
			// Clean up thumbnail cache
			if thumbnailService != nil {
				thumbnailService.Invalidate(event.Path)
			}
		}
	}

	// Initialize i18n service
	i18nSvc, err := i18n.NewService()
	if err != nil {
		log.Fatalf("Failed to initialize i18n service: %v", err)
	}
	fmt.Println("i18n service initialized with English and Russian translations")

	// Initialize authentication components
	sessionConfig := &auth.SessionConfig{
		IdleTimeout:     time.Duration(cfg.SessionIdleHours) * time.Hour,
		AbsoluteTimeout: time.Duration(cfg.SessionAbsoluteDays) * 24 * time.Hour,
		CookieMaxAge:    cfg.SessionIdleHours * 60 * 60,
		TokenLength:     64,
	}

	sessionRepo := auth.NewSessionRepository(db, sessionConfig)
	bootstrap := auth.NewBootstrapService(db, cfg.BootstrapLogin, cfg.BootstrapPassword)
	loginLimiter := auth.NewLoginRateLimiter(10, 15*time.Minute, 30*time.Minute)
	defer loginLimiter.Stop()
	authService := auth.NewAuthService(db, bootstrap, sessionRepo, loginLimiter)
	userService := auth.NewUserService(db, sessionRepo)
	authMiddleware := middleware.NewAuthMiddleware(sessionRepo, authService, i18nSvc)
	csrfProtection := middleware.NewCSRFProtection(i18nSvc)
	authHandlers := handler.NewAuthHandlers(authService, bootstrap, userService, sessionRepo, db, i18nSvc)
	// Start session cleanup job
	sessionCleanup := auth.NewSessionCleanupJob(sessionRepo, 1*time.Hour)
	sessionCleanup.Start()
	defer sessionCleanup.Stop()

	fmt.Println("Authentication system initialized!")

	// Create LLM OCR service
	llmOcrService := imaging.NewLlmOcrService(db)
	defer llmOcrService.Stop()
	fmt.Println("LLM OCR service initialized")

	// Create tag scan manager
	tagScanManager := imaging.NewTagScanManager(db, llmOcrService, cfg.LlmMaxImageMegapixels)

	// Read tag scan schedule from LlmSettings
	tagScanEnabled := true
	tagScanStartHour := 22
	tagScanStartMinute := 0
	tagScanEndHour := 7
	tagScanEndMinute := 0
	tagScanTimezoneOffset := 0
	var llmSettings domain.LlmSettings
	if result := db.First(&llmSettings); result.Error == nil {
		tagScanEnabled = llmSettings.TagScanEnabled
		tagScanStartHour = llmSettings.TagScanStartHour
		tagScanStartMinute = llmSettings.TagScanStartMinute
		tagScanEndHour = llmSettings.TagScanEndHour
		tagScanEndMinute = llmSettings.TagScanEndMinute
		tagScanTimezoneOffset = llmSettings.TagScanTimezoneOffset
	}

	tagScanManager.Start(tagScanEnabled, tagScanStartHour, tagScanStartMinute, tagScanEndHour, tagScanEndMinute, tagScanTimezoneOffset)
	defer tagScanManager.Stop()

	// Set coordinator for AI task synchronization
	llmOcrService.SetCoordinator(tagScanManager)

	fmt.Printf("Tag scan: window %02d:%02d - %02d:%02d, tzOffset=%d, enabled=%v\n", tagScanStartHour, tagScanStartMinute, tagScanEndHour, tagScanEndMinute, tagScanTimezoneOffset, tagScanEnabled)

	// Create embedding backfill manager
	embeddingBackfill := imaging.NewEmbeddingBackfillManager(db)
	fmt.Println("Embedding backfill manager initialized")

	// Wire embedding backfill to tag scan manager (triggered after tags are saved)
	tagScanManager.SetEmbeddingBackfill(embeddingBackfill)

	// Read EXIF backup directory from AppSettings
	var exifBackupDir string
	if result := db.First(&appSettings, 1); result.Error == nil && appSettings.ExifBackupDir != "" {
		exifBackupDir = appSettings.ExifBackupDir
	}

	// Create EXIF agent for MCP tool delegation (write_gps, read_exif_fields, etc.)
	exifAgent := agentpkg.NewExifAgent(cfg.ExifServiceURL, exifBackupDir)

	// Create MCP server
	llmFactory := helpers.NewLLMFactory(db, cfg.LlmMaxImageMegapixels)
	mcpSrv := mcpserver.NewFlashbacksMCPServer(db, llmFactory, llmOcrService, cfg.LlmMaxImageMegapixels, embeddingBackfill, exifAgent)
	fmt.Println("MCP server initialized with image analysis, search, and EXIF tools")

	// Create conversation service and agent
	convService := agentpkg.NewConversationService(db)
	agCfg := agentpkg.DefaultAgentConfig()
	agCfg.MaxConversationTokens = cfg.AgentMaxConversationTokens
	ag := agentpkg.NewAgent(convService, mcpSrv, agCfg)
	fmt.Println("AI agent initialized")

	// Start web server
	server := handler.NewServer(db, scanManager, ocrManager, llmOcrService, backgroundSync, tagScanManager, embeddingBackfill, thumbnailService, cfg, geolocationService, nominatimClient, mcpSrv, ag, agCfg, convService, exifSvcClient)
	router := server.SetupRouter(authMiddleware, csrfProtection, authHandlers)

	// Start OCR health check if enabled
	server.StartOCRHealthCheck()
	defer server.StopOCRHealthCheck()

	fmt.Printf("\nStarting API server on http://%s:%s\n", cfg.ServerHost, cfg.ServerPort)
	fmt.Printf("Scan workers: %d\n", cfg.ScanWorkers)
	fmt.Printf("CORS allowed origins: %s\n", strings.Join(cfg.CORSOrigins, ", "))
	fmt.Printf("Thumbnail cache: enabled=%v, path=%s\n", cfg.ThumbnailCacheEnabled, cachePath)
	fmt.Printf("Background sync: days=%v at %02d:%02d, tzOffset=%d min (configured in UI)\n", syncDays, syncHour, syncMinute, syncTimezoneOffset)
	fmt.Println("Configure gallery folders via the web UI Settings tab.")
	fmt.Println("Press Ctrl+C to stop the server")

	if err := router.Run(fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
