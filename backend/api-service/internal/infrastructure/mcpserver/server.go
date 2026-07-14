package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flashbacks/api-service/internal/application/agent"
	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
	"github.com/flashbacks/api-service/internal/interfaces/handler/helpers"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

// FlashbacksMCPServer wraps the official MCP SDK server with domain-specific tools.
type FlashbacksMCPServer struct {
	server            *mcp.Server
	db                *gorm.DB
	llmFactory        *helpers.LLMFactory
	llmService        *imaging.LlmOcrService
	maxMegapixels     float64
	embeddingBackfill *imaging.EmbeddingBackfillManager
	exifAgent         *agent.ExifAgent
}

// NewFlashbacksMCPServer creates and configures the MCP server with all tools.
func NewFlashbacksMCPServer(db *gorm.DB, llmFactory *helpers.LLMFactory, llmService *imaging.LlmOcrService, maxMegapixels float64, embeddingBackfill *imaging.EmbeddingBackfillManager, exifAgent *agent.ExifAgent) *FlashbacksMCPServer {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "image-toolkit",
		Version: "1.0.0",
	}, nil)

	s := &FlashbacksMCPServer{
		server:            srv,
		db:                db,
		llmFactory:        llmFactory,
		llmService:        llmService,
		maxMegapixels:     maxMegapixels,
		embeddingBackfill: embeddingBackfill,
		exifAgent:         exifAgent,
	}

	s.registerImageTools()
	s.registerSearchTools()

	return s
}

// Server returns the underlying MCP server instance.
func (s *FlashbacksMCPServer) Server() *mcp.Server {
	return s.server
}

// HTTPHandler returns an http.Handler that serves MCP over streamable HTTP.
func (s *FlashbacksMCPServer) HTTPHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.server
	}, nil)
}

// ToolDefinitions returns all registered tool definitions for use by the agent.
func (s *FlashbacksMCPServer) ToolDefinitions() []llm.ToolDefinition {
	tools := []llm.ToolDefinition{
		recognizeTextToolDef(),
		generateTagsToolDef(),
		askAboutImageToolDef(),
		enhanceImageQualityToolDef(),
		searchByDateToolDef(),
		searchByLocationToolDef(),
		searchByPathToolDef(),
		getCachedMetadataToolDef(),
		semanticSearchToolDef(),
	}

	// Append EXIF agent tools if available
	if s.exifAgent != nil {
		tools = append(tools, s.exifAgent.ToolDefinitions()...)
	}

	return tools
}

// ExecuteTool runs a tool by name with the given arguments.
func (s *FlashbacksMCPServer) ExecuteTool(ctx context.Context, name string, arguments json.RawMessage) (string, error) {
	// Delegate EXIF tools to the ExifAgent
	if agent.IsExifTool(name) {
		if s.exifAgent == nil {
			return "", fmt.Errorf("EXIF agent not available")
		}
		return s.exifAgent.ExecuteTool(ctx, name, arguments)
	}

	switch name {
	case "recognize_text":
		return s.executeRecognizeText(ctx, arguments)
	case "generate_tags":
		return s.executeGenerateTags(ctx, arguments)
	case "ask_about_image":
		return s.executeAskAboutImage(ctx, arguments)
	case "enhance_image_quality":
		return s.executeEnhanceImageQuality(ctx, arguments)
	case "search_by_date":
		return s.executeSearchByDate(ctx, arguments)
	case "search_by_location":
		return s.executeSearchByLocation(ctx, arguments)
	case "search_by_path":
		return s.executeSearchByPath(ctx, arguments)
	case "get_cached_metadata":
		return s.executeGetCachedMetadata(ctx, arguments)
	case "semantic_search":
		return s.executeSemanticSearch(ctx, arguments)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// createVLClient creates a VL (vision-language) LLM client from the VlProvider setting.
// Does NOT fall back to ActiveProvider — requires an explicit VL provider configuration.
func (s *FlashbacksMCPServer) createVLClient() (llm.Client, string, string, error) {
	var settings struct {
		VlProvider string `json:"vlProvider"`
	}
	if err := s.db.Table("llm_settings").Select("vl_provider").First(&settings).Error; err != nil {
		return nil, "", "", fmt.Errorf("LLM settings not found")
	}

	alias := settings.VlProvider
	if alias == "" {
		return nil, "", "", fmt.Errorf("VL provider is not configured — please set a VL LLM provider in Admin Settings > Analysis Tools")
	}

	var provider struct {
		Name   string `json:"name"`
		ApiUrl string `json:"apiUrl"`
		ApiKey string `json:"apiKey"`
		Model  string `json:"model"`
	}
	if err := s.db.Table("llm_providers").
		Select("name, api_url, api_key, model").
		Where("alias = ?", alias).
		First(&provider).Error; err != nil {
		return nil, "", "", fmt.Errorf("LLM provider '%s' not found", alias)
	}

	client, err := llm.NewClient(provider.Name, provider.ApiUrl, provider.ApiKey, provider.Model, s.maxMegapixels)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create LLM client: %w", err)
	}

	return client, provider.Name, provider.Model, nil
}

// createImgEditClient creates an LLM client from the ImgEditProvider setting.
// Falls back to VlProvider, then ActiveProvider if ImgEditProvider is not configured.
func (s *FlashbacksMCPServer) createImgEditClient() (llm.Client, string, string, error) {
	var settings struct {
		ActiveProvider  string `json:"activeProvider"`
		VlProvider      string `json:"vlProvider"`
		ImgEditProvider string `json:"imgEditProvider"`
	}
	if err := s.db.Table("llm_settings").Select("active_provider, vl_provider, img_edit_provider").First(&settings).Error; err != nil {
		return nil, "", "", fmt.Errorf("LLM settings not found")
	}

	alias := settings.ImgEditProvider
	if alias == "" {
		alias = settings.VlProvider
	}
	if alias == "" {
		alias = settings.ActiveProvider
	}

	var provider struct {
		Name   string `json:"name"`
		ApiUrl string `json:"apiUrl"`
		ApiKey string `json:"apiKey"`
		Model  string `json:"model"`
	}
	if err := s.db.Table("llm_providers").
		Select("name, api_url, api_key, model").
		Where("alias = ?", alias).
		First(&provider).Error; err != nil {
		return nil, "", "", fmt.Errorf("LLM provider '%s' not found", alias)
	}

	client, err := llm.NewClient(provider.Name, provider.ApiUrl, provider.ApiKey, provider.Model, s.maxMegapixels)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create LLM client: %w", err)
	}

	return client, provider.Name, provider.Model, nil
}
