package handler

import (
	"cmp"
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

// ServerDeps groups all dependencies for the Server in a single struct,
// enabling Wire-based dependency injection with a single-parameter constructor.
type ServerDeps struct {
	DB                  *gorm.DB
	Config              *config.AppConfig
	ScanManager         *imaging.ScanManager
	OcrManager          *imaging.OcrManager
	LlmOcrService       *imaging.LlmOcrService
	BackgroundSync      *imaging.BackgroundSyncManager
	TagScanManager      *imaging.TagScanManager
	EmbeddingBackfill   *imaging.EmbeddingBackfillManager
	ThumbnailService    *thumbnail.Service
	ThumbnailCache      *imaging.ThumbnailCache
	ThumbnailBatch      *helpers.ThumbnailBatch
	OcrClient           ocr.Client
	ClusterStorage      *geo.ClusterStorage
	GalleryAccess       *helpers.GalleryAccess
	SettingsLoader      *helpers.SettingsLoader
	LlmFactory          *helpers.LLMFactory
	FileMover           *helpers.FileMover
	I18n                *i18n.Service
	GeolocationService  *geocoder.GeolocationService
	Nominatim           *geocoder.NominatimClient
	McpServer           *mcpserver.FlashbacksMCPServer
	Agent               *agent.Agent
	AgentConfig         agent.AgentConfig
	ConversationService *agent.ConversationService
	ExifClient          imaging.ExifClient
}

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

// NewServer creates a new server instance from a dependency struct.
// All dependencies are injected via Wire — no internal construction.
func NewServer(deps ServerDeps) *Server {
	return &Server{
		db:                  deps.DB,
		thumbnailCache:      deps.ThumbnailCache,
		thumbnailService:    deps.ThumbnailService,
		thumbnailBatch:      deps.ThumbnailBatch,
		scanManager:         deps.ScanManager,
		ocrManager:          deps.OcrManager,
		llmOcrService:       deps.LlmOcrService,
		backgroundSync:      deps.BackgroundSync,
		tagScanManager:      deps.TagScanManager,
		embeddingBackfill:   deps.EmbeddingBackfill,
		config:              deps.Config,
		ocrClient:           deps.OcrClient,
		clusterStorage:      deps.ClusterStorage,
		galleryAccess:       deps.GalleryAccess,
		settingsLoader:      deps.SettingsLoader,
		llmFactory:          deps.LlmFactory,
		fileMover:           deps.FileMover,
		i18n:                deps.I18n,
		geolocationService:  deps.GeolocationService,
		nominatim:           deps.Nominatim,
		mcpServer:           deps.McpServer,
		agent:               deps.Agent,
		agentConfig:         deps.AgentConfig,
		conversationService: deps.ConversationService,
		exifClient:          deps.ExifClient,
	}
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
