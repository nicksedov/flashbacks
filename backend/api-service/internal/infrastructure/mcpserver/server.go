package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
		resizeImageToolDef(),
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
	case "resize_image":
		return s.executeResizeImage(ctx, arguments)
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

// createVLClient creates a VL (vision-language) LLM client from the VL instrument settings.
// Does NOT fall back to chat instrument — requires an explicit VL instrument configuration.
func (s *FlashbacksMCPServer) createVLClient() (llm.Client, string, string, error) {
	var instrument struct {
		Model      string `json:"model"`
		ProviderID uint   `json:"providerId"`
	}
	if err := s.db.Table("llm_instrument_settings").
		Select("model, provider_id").
		Where("type = ?", "vl").
		First(&instrument).Error; err != nil {
		return nil, "", "", fmt.Errorf("VL instrument not configured — please set a VL LLM provider in Admin Settings > Analysis Tools")
	}

	var provider struct {
		Name   string `json:"name"`
		ApiUrl string `json:"apiUrl"`
		ApiKey string `json:"apiKey"`
	}
	if err := s.db.Table("llm_providers").
		Select("name, api_url, api_key").
		Where("id = ?", instrument.ProviderID).
		First(&provider).Error; err != nil {
		return nil, "", "", fmt.Errorf("LLM provider not found")
	}

	client, err := llm.NewClient(provider.Name, provider.ApiUrl, provider.ApiKey, instrument.Model, s.maxMegapixels)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create LLM client: %w", err)
	}

	return client, provider.Name, instrument.Model, nil
}

// createImgEditClient creates an LLM client from the image_edit instrument settings.
func (s *FlashbacksMCPServer) createImgEditClient() (llm.Client, string, string, error) {
	var instrument struct {
		Model      string `json:"model"`
		ProviderID uint   `json:"providerId"`
	}
	if err := s.db.Table("llm_instrument_settings").
		Select("model, provider_id").
		Where("type = ?", "image_edit").
		First(&instrument).Error; err != nil {
		return nil, "", "", fmt.Errorf("image_edit instrument not configured")
	}

	var provider struct {
		Name   string `json:"name"`
		ApiUrl string `json:"apiUrl"`
		ApiKey string `json:"apiKey"`
	}
	if err := s.db.Table("llm_providers").
		Select("name, api_url, api_key").
		Where("id = ?", instrument.ProviderID).
		First(&provider).Error; err != nil {
		return nil, "", "", fmt.Errorf("LLM provider not found")
	}

	client, err := llm.NewClient(provider.Name, provider.ApiUrl, provider.ApiKey, instrument.Model, s.maxMegapixels)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create LLM client: %w", err)
	}

	return client, provider.Name, instrument.Model, nil
}

// isPathInGalleryFolder checks whether the given file path resides under any
// configured gallery folder. It queries the gallery_folders table directly
// to avoid importing the handler helpers package.
func (s *FlashbacksMCPServer) isPathInGalleryFolder(path string) (bool, error) {
	var folders []struct {
		Path string `json:"path"`
	}
	if err := s.db.Table("gallery_folders").Select("path").Find(&folders).Error; err != nil {
		return false, fmt.Errorf("failed to query gallery folders: %w", err)
	}

	for _, f := range folders {
		if path == f.Path ||
			strings.HasPrefix(path, f.Path+"/") ||
			strings.HasPrefix(path, f.Path+"\\") {
			return true, nil
		}
	}
	return false, nil
}
