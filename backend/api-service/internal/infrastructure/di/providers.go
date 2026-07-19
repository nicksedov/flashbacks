// Package di provides Wire provider sets for dependency injection.
package di

import (
	"log"
	"time"

	"github.com/flashbacks/api-service/internal/application/agent"
	"github.com/flashbacks/api-service/internal/application/auth"
	"github.com/flashbacks/api-service/internal/application/geo"
	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/application/thumbnail"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/domain/repository"
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

	"github.com/google/wire"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────────────────────────────────────
// Core providers (infrastructure, configuration, database, i18n)
// ──────────────────────────────────────────────────────────────────────────────

// CoreSet groups fundamental infrastructure providers.
var CoreSet = wire.NewSet(
	ProvideConfig,
	ProvideDB,
	ProvideI18nService,
	ProvideExifClient,
	ProvideNominatimClient,
	ProvideGeolocationService,
	ProvideOcrClient,
	ProvideThumbnailConfig,
	ProvideThumbnailProvider,
	ProvideLLMFactory,
	ProvideMaxMegapixels,
	ProvideImageFileRepo,
	ProvideGalleryFolderRepo,
	ProvideLlmRepo,
	ProvideMetadataRepo,
	ProvideOcrRepo,
	ProvideImageTagRepo,
	ProvideAppSettingsRepo,
	ProvideUserSettingsRepo,
	ProvideSyncHistoryRepo,
)

// ProvideConfig loads application configuration from environment variables.
func ProvideConfig() *config.AppConfig {
	return config.LoadConfig()
}

// ProvideDB initializes the database connection and runs auto-migration.
func ProvideDB(cfg *config.AppConfig) (*gorm.DB, error) {
	return database.InitDatabase(cfg)
}

// ProvideI18nService creates the i18n translation service.
func ProvideI18nService() (*i18n.Service, error) {
	return i18n.NewService()
}

// ProvideExifClient creates the EXIF service HTTP client.
func ProvideExifClient(cfg *config.AppConfig) imaging.ExifClient {
	return exifclient.NewHTTPExifClient(cfg.ExifServiceURL)
}

// ProvideNominatimClient creates the Nominatim geocoding client.
func ProvideNominatimClient() *geocoder.NominatimClient {
	return geocoder.NewNominatimClient(nil, "")
}

// ProvideGeolocationService creates the geolocation service backed by Nominatim.
func ProvideGeolocationService(db *gorm.DB, nominatim *geocoder.NominatimClient) *geocoder.GeolocationService {
	return geocoder.NewGeolocationService(db, nominatim)
}

// ProvideOcrClient creates the OCR classifier client (nil if OCR is disabled).
func ProvideOcrClient(cfg *config.AppConfig) ocr.Client {
	if cfg.OCREnabled {
		return ocr.NewClient(cfg.OCRServiceURL)
	}
	return nil
}

// ProvideMaxMegapixels extracts the LLM max image megapixels from config.
func ProvideMaxMegapixels(cfg *config.AppConfig) float64 {
	return cfg.LlmMaxImageMegapixels
}

// ProvideThumbnailConfig builds the thumbnail service configuration.
func ProvideThumbnailConfig(db *gorm.DB, cfg *config.AppConfig) *thumbnail.Config {
	cachePath := cfg.ThumbnailCachePath
	if cachePath == "" {
		var appSettings domain.AppSettings
		if result := db.First(&appSettings, 1); result.Error == nil && appSettings.ThumbnailCachePath != "" {
			cachePath = appSettings.ThumbnailCachePath
		}
	}
	return &thumbnail.Config{
		CacheDir:      cachePath,
		MaxSize:       cfg.ThumbnailCacheMaxSize,
		Quality:       cfg.ThumbnailCacheQuality,
		Enabled:       cfg.ThumbnailCacheEnabled,
		Format:        "webp",
		PreloadOnScan: cfg.ThumbnailCachePreloadOnScan,
	}
}

// ProvideThumbnailProvider creates the thumbnail service as a ThumbnailProvider.
func ProvideThumbnailProvider(tcConfig *thumbnail.Config) thumbnail.ThumbnailProvider {
	svc, err := thumbnail.NewService(tcConfig)
	if err != nil {
		log.Printf("Failed to initialize thumbnail cache: %v", err)
		return nil
	}
	return svc
}

// ProvideLLMFactory creates the LLM client factory.
func ProvideLLMFactory(db *gorm.DB, maxMegapixels float64) *helpers.LLMFactory {
	return helpers.NewLLMFactory(db, maxMegapixels)
}

// ──────────────────────────────────────────────────────────────────────────────
// Repository providers (data access layer)
// ──────────────────────────────────────────────────────────────────────────────

// ProvideImageFileRepo creates the image file repository.
func ProvideImageFileRepo(db *gorm.DB) repository.ImageFileRepository {
	return repository.NewImageFileRepository(db)
}

// ProvideGalleryFolderRepo creates the gallery folder repository.
func ProvideGalleryFolderRepo(db *gorm.DB) repository.GalleryFolderRepository {
	return repository.NewGalleryFolderRepository(db)
}

// ProvideLlmRepo creates the LLM repository.
func ProvideLlmRepo(db *gorm.DB) repository.LlmRepository {
	return repository.NewLlmRepository(db)
}

// ProvideMetadataRepo creates the image metadata repository.
func ProvideMetadataRepo(db *gorm.DB) repository.MetadataRepository {
	return repository.NewMetadataRepository(db)
}

// ProvideOcrRepo creates the OCR data repository.
func ProvideOcrRepo(db *gorm.DB) repository.OcrRepository {
	return repository.NewOcrRepository(db)
}

// ProvideImageTagRepo creates the image tag repository.
func ProvideImageTagRepo(db *gorm.DB) repository.ImageTagRepository {
	return repository.NewImageTagRepository(db)
}

// ProvideAppSettingsRepo creates the app settings repository.
func ProvideAppSettingsRepo(db *gorm.DB) repository.AppSettingsRepository {
	return repository.NewAppSettingsRepository(db)
}

// ProvideUserSettingsRepo creates the user settings repository.
func ProvideUserSettingsRepo(db *gorm.DB) repository.UserSettingsRepository {
	return repository.NewUserSettingsRepository(db)
}

// ProvideSyncHistoryRepo creates the sync history repository.
func ProvideSyncHistoryRepo(db *gorm.DB) repository.SyncHistoryRepository {
	return repository.NewSyncHistoryRepository(db)
}

// ──────────────────────────────────────────────────────────────────────────────
// Application service providers (scanning, OCR, LLM, background, thumbnail)
// ──────────────────────────────────────────────────────────────────────────────

// AppSet groups business-logic service providers.
var AppSet = wire.NewSet(
	ProvideScanManager,
	ProvideOcrManager,
	ProvideLlmOcrService,
	ProvideBackgroundSyncManager,
	ProvideTagScanManager,
	ProvideEmbeddingBackfillManager,
	ProvideClusterStorage,
	ProvideGalleryAccess,
	ProvideSettingsLoader,
	ProvideFileMover,
	ProvideThumbnailBatch,
)

// ProvideScanManager creates the scan manager.
func ProvideScanManager(db *gorm.DB, cfg *config.AppConfig) *imaging.ScanManager {
	return imaging.NewScanManager(db, cfg.ScanWorkers)
}

// ProvideOcrManager creates the OCR manager (nil if OCR is disabled).
func ProvideOcrManager(db *gorm.DB, ocrClient ocr.Client, cfg *config.AppConfig) *imaging.OcrManager {
	if cfg.OCREnabled && ocrClient != nil {
		ocrWorkers := cfg.OCRConcurrentRequests
		var appSettings domain.AppSettings
		if result := db.First(&appSettings, 1); result.Error == nil && appSettings.OcrConcurrentRequests > 0 {
			ocrWorkers = appSettings.OcrConcurrentRequests
		}
		return imaging.NewOcrManager(db, ocrClient, ocrWorkers)
	}
	return nil
}

// ProvideLlmOcrService creates the LLM-based OCR service.
func ProvideLlmOcrService(db *gorm.DB) *imaging.LlmOcrService {
	return imaging.NewLlmOcrService(db)
}

// ProvideBackgroundSyncManager creates the background sync manager.
func ProvideBackgroundSyncManager(
	db *gorm.DB,
	thumbnailService thumbnail.ThumbnailProvider,
	geolocationService *geocoder.GeolocationService,
	exifClient imaging.ExifClient,
) *imaging.BackgroundSyncManager {
	return imaging.NewBackgroundSyncManager(db, thumbnailService, geolocationService, exifClient)
}

// ProvideTagScanManager creates the tag scan manager.
func ProvideTagScanManager(
	db *gorm.DB,
	llmOcrService *imaging.LlmOcrService,
	maxMegapixels float64,
) *imaging.TagScanManager {
	return imaging.NewTagScanManager(db, llmOcrService, maxMegapixels)
}

// ProvideEmbeddingBackfillManager creates the embedding backfill manager.
func ProvideEmbeddingBackfillManager(db *gorm.DB) *imaging.EmbeddingBackfillManager {
	return imaging.NewEmbeddingBackfillManager(db)
}

// ProvideClusterStorage creates the geo-cluster storage.
func ProvideClusterStorage() *geo.ClusterStorage {
	return geo.NewClusterStorage()
}

// ProvideGalleryAccess creates the gallery path validator.
func ProvideGalleryAccess(db *gorm.DB) *helpers.GalleryAccess {
	return helpers.NewGalleryAccess(db)
}

// ProvideSettingsLoader creates the settings loader.
func ProvideSettingsLoader(db *gorm.DB) *helpers.SettingsLoader {
	return helpers.NewSettingsLoader(db)
}

// ProvideFileMover creates the file mover helper.
func ProvideFileMover(db *gorm.DB) *helpers.FileMover {
	return helpers.NewFileMover(db)
}

// ProvideThumbnailBatch creates the thumbnail batch generator.
func ProvideThumbnailBatch(provider thumbnail.ThumbnailProvider) *helpers.ThumbnailBatch {
	return helpers.NewThumbnailBatch(provider)
}

// ──────────────────────────────────────────────────────────────────────────────
// Authentication providers (session, bootstrap, rate limiting, middleware)
// ──────────────────────────────────────────────────────────────────────────────

// AuthSet groups authentication-related providers.
var AuthSet = wire.NewSet(
	ProvideSessionConfig,
	ProvideSessionRepository,
	ProvideBootstrapService,
	ProvideLoginRateLimiter,
	ProvideAuthService,
	ProvideUserService,
	ProvideAuthMiddleware,
	ProvideCSRFProtection,
	ProvideAuthHandlers,
	ProvideSessionCleanupJob,
)

// ProvideSessionConfig creates the session configuration from app config.
func ProvideSessionConfig(cfg *config.AppConfig) *auth.SessionConfig {
	return &auth.SessionConfig{
		IdleTimeout:     time.Duration(cfg.SessionIdleHours) * time.Hour,
		AbsoluteTimeout: time.Duration(cfg.SessionAbsoluteDays) * 24 * time.Hour,
		CookieMaxAge:    cfg.SessionIdleHours * 60 * 60,
		TokenLength:     64,
	}
}

// ProvideSessionRepository creates the session repository.
func ProvideSessionRepository(db *gorm.DB, sessionConfig *auth.SessionConfig) *auth.SessionRepository {
	return auth.NewSessionRepository(db, sessionConfig)
}

// ProvideBootstrapService creates the bootstrap service.
func ProvideBootstrapService(db *gorm.DB, cfg *config.AppConfig) *auth.BootstrapService {
	return auth.NewBootstrapService(db, cfg.BootstrapLogin, cfg.BootstrapPassword)
}

// ProvideLoginRateLimiter creates the login rate limiter.
func ProvideLoginRateLimiter() *auth.LoginRateLimiter {
	return auth.NewLoginRateLimiter(10, 15*time.Minute, 30*time.Minute)
}

// ProvideAuthService creates the authentication service.
func ProvideAuthService(
	db *gorm.DB,
	bootstrap *auth.BootstrapService,
	sessionRepo *auth.SessionRepository,
	loginLimiter *auth.LoginRateLimiter,
) *auth.AuthService {
	return auth.NewAuthService(db, bootstrap, sessionRepo, loginLimiter)
}

// ProvideUserService creates the user service.
func ProvideUserService(db *gorm.DB, sessionRepo *auth.SessionRepository) *auth.UserService {
	return auth.NewUserService(db, sessionRepo)
}

// ProvideAuthMiddleware creates the authentication middleware.
func ProvideAuthMiddleware(
	sessionRepo *auth.SessionRepository,
	authService *auth.AuthService,
	i18nSvc *i18n.Service,
) *middleware.AuthMiddleware {
	return middleware.NewAuthMiddleware(sessionRepo, authService, i18nSvc)
}

// ProvideCSRFProtection creates the CSRF protection middleware.
func ProvideCSRFProtection(i18nSvc *i18n.Service) *middleware.CSRFProtection {
	return middleware.NewCSRFProtection(i18nSvc)
}

// ProvideAuthHandlers creates the auth HTTP handlers.
func ProvideAuthHandlers(
	authService *auth.AuthService,
	bootstrap *auth.BootstrapService,
	userService *auth.UserService,
	sessionRepo *auth.SessionRepository,
	db *gorm.DB,
	i18nSvc *i18n.Service,
) *handler.AuthHandlers {
	return handler.NewAuthHandlers(authService, bootstrap, userService, sessionRepo, db, i18nSvc)
}

// ProvideSessionCleanupJob creates the session cleanup background job.
func ProvideSessionCleanupJob(sessionRepo *auth.SessionRepository) *auth.SessionCleanupJob {
	return auth.NewSessionCleanupJob(sessionRepo, 1*time.Hour)
}

// ──────────────────────────────────────────────────────────────────────────────
// Agent providers (EXIF agent, MCP server, conversation, AI agent)
// ──────────────────────────────────────────────────────────────────────────────

// AgentSet groups AI agent-related providers.
var AgentSet = wire.NewSet(
	ProvideExifBackupDir,
	ProvideExifAgent,
	ProvideMCPServer,
	ProvideConversationService,
	ProvideAgentConfig,
	ProvideAgent,
)

// ProvideExifBackupDir reads the EXIF backup directory from AppSettings.
func ProvideExifBackupDir(db *gorm.DB) string {
	var appSettings domain.AppSettings
	if result := db.First(&appSettings, 1); result.Error == nil && appSettings.ExifBackupDir != "" {
		return appSettings.ExifBackupDir
	}
	return ""
}

// ProvideExifAgent creates the EXIF sub-agent.
func ProvideExifAgent(cfg *config.AppConfig, backupDir string) *agent.ExifAgent {
	return agent.NewExifAgent(cfg.ExifServiceURL, backupDir)
}

// ProvideMCPServer creates the MCP server with all registered tools.
func ProvideMCPServer(
	db *gorm.DB,
	llmFactory *helpers.LLMFactory,
	llmOcrService *imaging.LlmOcrService,
	maxMegapixels float64,
	embeddingBackfill *imaging.EmbeddingBackfillManager,
	exifAgent *agent.ExifAgent,
) *mcpserver.FlashbacksMCPServer {
	return mcpserver.NewFlashbacksMCPServer(db, llmFactory, llmOcrService, maxMegapixels, embeddingBackfill, exifAgent)
}

// ProvideConversationService creates the conversation persistence service.
func ProvideConversationService(db *gorm.DB) *agent.ConversationService {
	return agent.NewConversationService(db)
}

// ProvideAgentConfig creates the agent configuration.
func ProvideAgentConfig(cfg *config.AppConfig) agent.AgentConfig {
	agCfg := agent.DefaultAgentConfig()
	agCfg.MaxConversationTokens = cfg.AgentMaxConversationTokens
	return agCfg
}

// ProvideAgent creates the AI agent.
func ProvideAgent(
	convService *agent.ConversationService,
	mcpSrv *mcpserver.FlashbacksMCPServer,
	agCfg agent.AgentConfig,
) *agent.Agent {
	return agent.NewAgent(convService, mcpSrv, agCfg)
}

// ──────────────────────────────────────────────────────────────────────────────
// Handler provider (Server and its dependencies)
// ──────────────────────────────────────────────────────────────────────────────

// HandlerSet groups HTTP handler providers.
var HandlerSet = wire.NewSet(
	wire.Struct(new(handler.ServerDeps), "*"),
	ProvideServer,
)

// ProvideServer creates the HTTP server from its dependency struct.
func ProvideServer(deps handler.ServerDeps) *handler.Server {
	return handler.NewServer(deps)
}

// ──────────────────────────────────────────────────────────────────────────────
// Application aggregate (combines all sets for the injector)
// ──────────────────────────────────────────────────────────────────────────────

// ApplicationSet is the top-level provider set combining all sub-sets.
var ApplicationSet = wire.NewSet(
	CoreSet,
	AppSet,
	AuthSet,
	AgentSet,
	HandlerSet,
)
