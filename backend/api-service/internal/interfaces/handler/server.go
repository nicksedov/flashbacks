package handler

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/flashbacks/api-service/internal/application/agent"
	"github.com/flashbacks/api-service/internal/application/geo"
	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/application/thumbnail"
	"github.com/flashbacks/api-service/internal/infrastructure/config"
	"github.com/flashbacks/api-service/internal/infrastructure/geocoder"
	"github.com/flashbacks/api-service/internal/infrastructure/mcpserver"
	"github.com/flashbacks/api-service/internal/infrastructure/ocr"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"
	"github.com/flashbacks/api-service/internal/interfaces/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Server holds the application state
type Server struct {
	db                  *gorm.DB
	thumbnailCache      *imaging.ThumbnailCache
	thumbnailService    *thumbnail.Service
	thumbnailBatch      *helpers.ThumbnailBatch
	scanManager         *imaging.ScanManager
	ocrManager          *imaging.OcrManager
	llmOcrService       *imaging.LlmOcrService
	backgroundSync      *imaging.BackgroundSyncManager
	tagScanManager      *imaging.TagScanManager
	embeddingBackfill   *imaging.EmbeddingBackfillManager
	config              *config.AppConfig
	ocrClient           ocr.Client
	clusterStorage      *geo.ClusterStorage
	galleryAccess       *helpers.GalleryAccess
	settingsLoader      *helpers.SettingsLoader
	llmFactory          *helpers.LLMFactory
	fileMover           *helpers.FileMover
	i18n                *i18n.Service
	geolocationService  *geocoder.GeolocationService
	nominatim           *geocoder.NominatimClient
	mcpServer           *mcpserver.FlashbacksMCPServer
	agent               *agent.Agent
	agentConfig         agent.AgentConfig
	conversationService *agent.ConversationService
	exifClient          imaging.ExifClient
}

// NewServer creates a new server instance
func NewServer(db *gorm.DB, scanManager *imaging.ScanManager, ocrManager *imaging.OcrManager, llmOcrService *imaging.LlmOcrService, backgroundSync *imaging.BackgroundSyncManager, tagScanManager *imaging.TagScanManager, embeddingBackfill *imaging.EmbeddingBackfillManager, thumbnailService *thumbnail.Service, cfg *config.AppConfig, geolocationService *geocoder.GeolocationService, nominatimClient *geocoder.NominatimClient, mcpSrv *mcpserver.FlashbacksMCPServer, ag *agent.Agent, agCfg agent.AgentConfig, convService *agent.ConversationService, exifClient imaging.ExifClient) *Server {
	var ocrClient ocr.Client
	if cfg.OCREnabled {
		ocrClient = ocr.NewClient(cfg.OCRServiceURL)
	}

	// Initialize i18n service
	i18nSvc, err := i18n.NewService()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize i18n service: %v", err))
	}

	s := &Server{
		db:                  db,
		thumbnailCache:      imaging.NewThumbnailCache(),
		thumbnailService:    thumbnailService,
		scanManager:         scanManager,
		ocrManager:          ocrManager,
		llmOcrService:       llmOcrService,
		backgroundSync:      backgroundSync,
		tagScanManager:      tagScanManager,
		embeddingBackfill:   embeddingBackfill,
		config:              cfg,
		ocrClient:           ocrClient,
		clusterStorage:      geo.NewClusterStorage(),
		i18n:                i18nSvc,
		geolocationService:  geolocationService,
		nominatim:           nominatimClient,
		mcpServer:           mcpSrv,
		agent:               ag,
		agentConfig:         agCfg,
		conversationService: convService,
		exifClient:          exifClient,
	}
	s.thumbnailBatch = helpers.NewThumbnailBatch(thumbnailService, s.thumbnailCache)
	s.galleryAccess = helpers.NewGalleryAccess(db)
	s.settingsLoader = helpers.NewSettingsLoader(db)
	s.llmFactory = helpers.NewLLMFactory(db, cfg.LlmMaxImageMegapixels)
	s.fileMover = helpers.NewFileMover(db)
	return s
}

// StartOCRHealthCheck starts the OCR health check in background
func (s *Server) StartOCRHealthCheck() {
	if s.ocrClient != nil && s.config.OCREnabled {
		s.ocrClient.StartHealthCheck(s.config.OCRCheckInterval)
	}
}

// StopOCRHealthCheck stops the OCR health check
func (s *Server) StopOCRHealthCheck() {
	if s.ocrClient != nil {
		s.ocrClient.StopHealthCheck()
	}
}

// formatSize formats file size in human readable format
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// pathsConflict checks if two normalized (forward-slash) paths are the same,
// or if one is a parent/child of the other.
// Returns a non-empty reason string if there is a conflict, empty string otherwise.
func pathsConflict(a, b string) string {
	// Normalize: trim trailing slashes, lowercase for case-insensitive FS
	na := strings.TrimRight(strings.ToLower(a), "/")
	nb := strings.TrimRight(strings.ToLower(b), "/")

	if na == nb {
		return "same"
	}
	if strings.HasPrefix(na, nb+"/") {
		return "child" // a is child of b
	}
	if strings.HasPrefix(nb, na+"/") {
		return "parent" // a is parent of b
	}
	return ""
}

// sortPatternsByCount sorts patterns by duplicate count descending
func sortPatternsByCount(patterns []dto.FolderPattern) {
	slices.SortFunc(patterns, func(a, b dto.FolderPattern) int {
		return cmp.Compare(b.DuplicateCount, a.DuplicateCount)
	})
}

// createPatternID creates a unique ID from sorted folder paths
func createPatternID(folders []string) string {
	return strings.Join(folders, "|")
}

// respondSuccess sends a success response with the message translated to the user's language
func (s *Server) respondSuccess(c *gin.Context, code int, msg i18n.MessageKey, data ...interface{}) {
	lang := middleware.GetLanguage(c)
	resp := i18n.SuccessResponseResolved(s.i18n, msg, lang, data...)
	c.JSON(code, resp)
}

// respondError sends an error response with the message translated to the user's language
func (s *Server) respondError(c *gin.Context, code int, msg i18n.MessageKey) {
	lang := middleware.GetLanguage(c)
	c.JSON(code, i18n.ErrorResponseResolved(s.i18n, msg, lang))
}

// respondValidationError sends a validation error response with the message translated
func (s *Server) respondValidationError(c *gin.Context, code int, msg i18n.MessageKey) {
	lang := middleware.GetLanguage(c)
	c.JSON(code, i18n.ValidationErrorResolved(s.i18n, msg, lang))
}

// respondJSON sends a raw JSON response (for complex responses not fitting standard patterns)
func (s *Server) respondJSON(c *gin.Context, code int, data interface{}) {
	c.JSON(code, data)
}
