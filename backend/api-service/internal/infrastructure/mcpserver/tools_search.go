package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Tool input types ---

type SearchByDateInput struct {
	StartDate string `json:"start_date" jsonschema:"Start date in YYYY-MM-DD format"`
	EndDate   string `json:"end_date" jsonschema:"End date in YYYY-MM-DD format"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default 20)"`
}

type SearchByLocationInput struct {
	MinLat float64 `json:"min_lat" jsonschema:"Minimum latitude of bounding box"`
	MaxLat float64 `json:"max_lat" jsonschema:"Maximum latitude of bounding box"`
	MinLng float64 `json:"min_lng" jsonschema:"Minimum longitude of bounding box"`
	MaxLng float64 `json:"max_lng" jsonschema:"Maximum longitude of bounding box"`
	Limit  int     `json:"limit,omitempty" jsonschema:"Maximum number of results (default 20)"`
}

type SearchByPathInput struct {
	Query string `json:"query" jsonschema:"Search query for filename/path (case-insensitive)"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default 20)"`
}

type GetCachedMetadataInput struct {
	ImagePath string `json:"image_path" jsonschema:"Path to the image file"`
}

type SemanticSearchInput struct {
	Query string `json:"query" jsonschema:"Natural language description of what you're looking for"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default 20)"`
}

// --- Tool output types ---

type ImageSearchResult struct {
	ID       uint   `json:"id"`
	Path     string `json:"path"`
	FileName string `json:"fileName"`
	ModTime  string `json:"modTime,omitempty"`
}

type ImageSearchOutput struct {
	Images []ImageSearchResult `json:"images"`
	Total  int                 `json:"total"`
}

type SemanticSearchResult struct {
	ID         uint     `json:"id"`
	Path       string   `json:"path"`
	FileName   string   `json:"fileName"`
	ModTime    string   `json:"modTime,omitempty"`
	Similarity float64  `json:"similarity"`
	Tags       []string `json:"tags"`
}

type SemanticSearchOutput struct {
	Images []SemanticSearchResult `json:"images"`
	Total  int                    `json:"total"`
	Query  string                 `json:"query"`
}

type ImageMetadataOutput struct {
	Path         string  `json:"path"`
	DateTaken    string  `json:"dateTaken,omitempty"`
	GPSLatitude  float64 `json:"gpsLatitude,omitempty"`
	GPSLongitude float64 `json:"gpsLongitude,omitempty"`
	NameLocal    string  `json:"nameLocal,omitempty"`
	NameEng      string  `json:"nameEng,omitempty"`
	CameraModel  string  `json:"cameraModel,omitempty"`
	LensModel    string  `json:"lensModel,omitempty"`
	Width        int     `json:"width,omitempty"`
	Height       int     `json:"height,omitempty"`
}

// --- Tool definitions for the agent ---

func searchByDateToolDef() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "search_by_date",
		Description: "Find images taken within a date range (uses EXIF date taken)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"start_date": map[string]any{"type": "string", "description": "Start date in YYYY-MM-DD format"},
				"end_date":   map[string]any{"type": "string", "description": "End date in YYYY-MM-DD format"},
				"limit":      map[string]any{"type": "integer", "description": "Maximum number of results (default 20)"},
			},
			"required": []string{"start_date", "end_date"},
		},
	}
}

func searchByLocationToolDef() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "search_by_location",
		Description: "Find images taken at specific geographic coordinates (GPS bounding box)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"min_lat": map[string]any{"type": "number", "description": "Minimum latitude of bounding box"},
				"max_lat": map[string]any{"type": "number", "description": "Maximum latitude of bounding box"},
				"min_lng": map[string]any{"type": "number", "description": "Minimum longitude of bounding box"},
				"max_lng": map[string]any{"type": "number", "description": "Maximum longitude of bounding box"},
				"limit":   map[string]any{"type": "integer", "description": "Maximum number of results (default 20)"},
			},
			"required": []string{"min_lat", "max_lat", "min_lng", "max_lng"},
		},
	}
}

func searchByPathToolDef() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "search_by_path",
		Description: "Find images by filename or path pattern (case-insensitive)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Search query for filename/path"},
				"limit": map[string]any{"type": "integer", "description": "Maximum number of results (default 20)"},
			},
			"required": []string{"query"},
		},
	}
}

func getCachedMetadataToolDef() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "get_cached_metadata",
		Description: "Get cached EXIF metadata from the database for a specific image (fast, may be stale). For live file-level EXIF use read_exif_fields or dump_exif_raw.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"image_path": map[string]any{"type": "string", "description": "Path to the image file"},
			},
			"required": []string{"image_path"},
		},
	}
}

func semanticSearchToolDef() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "semantic_search",
		Description: "Find images by natural language description using semantic similarity of AI-generated tags",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Natural language description of what you're looking for"},
				"limit": map[string]any{"type": "integer", "description": "Maximum number of results (default 20)"},
			},
			"required": []string{"query"},
		},
	}
}

// --- Registration ---

func (s *FlashbacksMCPServer) registerSearchTools() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_by_date",
		Description: "Find images taken within a date range",
	}, s.handleSearchByDate)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_by_location",
		Description: "Find images taken at specific geographic coordinates",
	}, s.handleSearchByLocation)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_by_path",
		Description: "Find images by filename or path pattern",
	}, s.handleSearchByPath)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_cached_metadata",
		Description: "Get cached EXIF metadata from the database for a specific image (fast, may be stale). For live file-level EXIF use read_exif_fields or dump_exif_raw.",
	}, s.handleGetCachedMetadata)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "semantic_search",
		Description: "Find images by natural language description using semantic similarity of AI-generated tags. This is the primary tool for finding similar or related images.",
	}, s.handleSemanticSearch)
}

// --- MCP SDK handlers ---

func (s *FlashbacksMCPServer) handleSearchByDate(ctx context.Context, req *mcp.CallToolRequest, input SearchByDateInput) (*mcp.CallToolResult, ImageSearchOutput, error) {
	output, err := s.queryByDate(input.StartDate, input.EndDate, clampLimit(input.Limit))
	if err != nil {
		return nil, ImageSearchOutput{}, err
	}
	text := formatSearchResults(output)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, output, nil
}

func (s *FlashbacksMCPServer) handleSearchByLocation(ctx context.Context, req *mcp.CallToolRequest, input SearchByLocationInput) (*mcp.CallToolResult, ImageSearchOutput, error) {
	output, err := s.queryByLocation(input.MinLat, input.MaxLat, input.MinLng, input.MaxLng, clampLimit(input.Limit))
	if err != nil {
		return nil, ImageSearchOutput{}, err
	}
	text := formatSearchResults(output)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, output, nil
}

func (s *FlashbacksMCPServer) handleSearchByPath(ctx context.Context, req *mcp.CallToolRequest, input SearchByPathInput) (*mcp.CallToolResult, ImageSearchOutput, error) {
	output, err := s.queryByPath(input.Query, clampLimit(input.Limit))
	if err != nil {
		return nil, ImageSearchOutput{}, err
	}
	text := formatSearchResults(output)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, output, nil
}

func (s *FlashbacksMCPServer) handleGetCachedMetadata(ctx context.Context, req *mcp.CallToolRequest, input GetCachedMetadataInput) (*mcp.CallToolResult, ImageMetadataOutput, error) {
	output, err := s.queryImageMetadata(input.ImagePath)
	if err != nil {
		return nil, ImageMetadataOutput{}, err
	}
	text := formatMetadataResult(output)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, output, nil
}

func (s *FlashbacksMCPServer) handleSemanticSearch(ctx context.Context, req *mcp.CallToolRequest, input SemanticSearchInput) (*mcp.CallToolResult, SemanticSearchOutput, error) {
	output, err := s.querySemanticSearch(input.Query, clampLimit(input.Limit))
	if err != nil {
		return nil, SemanticSearchOutput{}, err
	}
	text := formatSemanticSearchResult(output)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, output, nil
}

// --- Direct execution methods (for agent) ---

func (s *FlashbacksMCPServer) executeSearchByDate(ctx context.Context, args json.RawMessage) (string, error) {
	var input SearchByDateInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	output, err := s.queryByDate(input.StartDate, input.EndDate, clampLimit(input.Limit))
	if err != nil {
		return "", err
	}
	return formatSearchResultsJSON(output)
}

func (s *FlashbacksMCPServer) executeSearchByLocation(ctx context.Context, args json.RawMessage) (string, error) {
	var input SearchByLocationInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	output, err := s.queryByLocation(input.MinLat, input.MaxLat, input.MinLng, input.MaxLng, clampLimit(input.Limit))
	if err != nil {
		return "", err
	}
	return formatSearchResultsJSON(output)
}

func (s *FlashbacksMCPServer) executeSearchByPath(ctx context.Context, args json.RawMessage) (string, error) {
	var input SearchByPathInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	output, err := s.queryByPath(input.Query, clampLimit(input.Limit))
	if err != nil {
		return "", err
	}
	return formatSearchResultsJSON(output)
}

func (s *FlashbacksMCPServer) executeGetCachedMetadata(ctx context.Context, args json.RawMessage) (string, error) {
	var input GetCachedMetadataInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	output, err := s.queryImageMetadata(input.ImagePath)
	if err != nil {
		return "", err
	}
	return formatMetadataJSON(output)
}

func (s *FlashbacksMCPServer) executeSemanticSearch(ctx context.Context, args json.RawMessage) (string, error) {
	var input SemanticSearchInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	output, err := s.querySemanticSearch(input.Query, clampLimit(input.Limit))
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// --- Query implementations ---

func (s *FlashbacksMCPServer) queryByDate(startDate, endDate string, limit int) (ImageSearchOutput, error) {
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return ImageSearchOutput{}, fmt.Errorf("invalid start_date format (use YYYY-MM-DD): %w", err)
	}

	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return ImageSearchOutput{}, fmt.Errorf("invalid end_date format (use YYYY-MM-DD): %w", err)
	}

	// End of the end date (23:59:59)
	endTime = endTime.Add(24*time.Hour - time.Second)

	var files []domain.ImageFile
	s.db.Table("image_files").
		Select("image_files.id, image_files.path, image_files.mod_time").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.date_taken >= ? AND image_metadata.date_taken <= ?", startTime, endTime).
		Order("image_metadata.date_taken DESC").
		Limit(limit).
		Find(&files)

	var total int64
	s.db.Table("image_files").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.date_taken >= ? AND image_metadata.date_taken <= ?", startTime, endTime).
		Count(&total)

	return toImageSearchOutput(files, int(total)), nil
}

func (s *FlashbacksMCPServer) queryByLocation(minLat, maxLat, minLng, maxLng float64, limit int) (ImageSearchOutput, error) {
	var files []domain.ImageFile
	s.db.Table("image_files").
		Select("image_files.id, image_files.path, image_files.mod_time").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Joins("INNER JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref").
		Where("geolocation_caches.gps_latitude BETWEEN ? AND ?", minLat, maxLat).
		Where("geolocation_caches.gps_longitude BETWEEN ? AND ?", minLng, maxLng).
		Order("image_files.mod_time DESC").
		Limit(limit).
		Find(&files)

	var total int64
	s.db.Table("image_files").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Joins("INNER JOIN geolocation_caches ON geolocation_caches.id = image_metadata.geolocation_ref").
		Where("geolocation_caches.gps_latitude BETWEEN ? AND ?", minLat, maxLat).
		Where("geolocation_caches.gps_longitude BETWEEN ? AND ?", minLng, maxLng).
		Count(&total)

	return toImageSearchOutput(files, int(total)), nil
}

func (s *FlashbacksMCPServer) queryByPath(query string, limit int) (ImageSearchOutput, error) {
	if query == "" {
		return ImageSearchOutput{}, fmt.Errorf("query is required")
	}

	pattern := "%" + query + "%"

	var files []domain.ImageFile
	s.db.Where("path ILIKE ?", pattern).
		Order("mod_time DESC").
		Limit(limit).
		Find(&files)

	var total int64
	s.db.Model(&domain.ImageFile{}).
		Where("path ILIKE ?", pattern).
		Count(&total)

	return toImageSearchOutput(files, int(total)), nil
}

func (s *FlashbacksMCPServer) queryImageMetadata(imagePath string) (ImageMetadataOutput, error) {
	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", imagePath).First(&imageFile).Error; err != nil {
		return ImageMetadataOutput{}, fmt.Errorf("image not found: %s", imagePath)
	}

	var meta domain.ImageMetadata
	if err := s.db.Where("image_file_id = ?", imageFile.ID).First(&meta).Error; err != nil {
		// No metadata available
		return ImageMetadataOutput{Path: imagePath}, nil
	}

	output := ImageMetadataOutput{
		Path:        imagePath,
		CameraModel: meta.CameraModel,
		LensModel:   meta.LensModel,
		Width:       meta.Width,
		Height:      meta.Height,
	}

	if meta.DateTaken != nil {
		output.DateTaken = meta.DateTaken.Format("2006-01-02 15:04:05")
	}

	// Resolve geolocation from cache
	if meta.GeolocationRef != nil {
		var geoCache domain.GeolocationCache
		if result := s.db.First(&geoCache, *meta.GeolocationRef); result.Error == nil {
			output.GPSLatitude = geoCache.GPSLatitude
			output.GPSLongitude = geoCache.GPSLongitude
			output.NameLocal = geoCache.NameLocal
			output.NameEng = geoCache.NameEng
		}
	}

	return output, nil
}

// --- Helpers ---

func clampLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func toImageSearchOutput(files []domain.ImageFile, total int) ImageSearchOutput {
	images := make([]ImageSearchResult, len(files))
	for i, f := range files {
		images[i] = ImageSearchResult{
			ID:       f.ID,
			Path:     f.Path,
			FileName: fileNameFromPath(f.Path),
			ModTime:  f.ModTime.Format("2006-01-02 15:04:05"),
		}
	}
	return ImageSearchOutput{Images: images, Total: total}
}

func fileNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	parts = strings.Split(path, "\\")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

func formatSearchResults(output ImageSearchOutput) string {
	if len(output.Images) == 0 {
		return "No images found."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d images (showing %d):\n", output.Total, len(output.Images))
	for _, img := range output.Images {
		fmt.Fprintf(&sb, "- [%d] %s", img.ID, img.Path)
		if img.ModTime != "" {
			fmt.Fprintf(&sb, " (modified: %s)", img.ModTime)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatSearchResultsJSON(output ImageSearchOutput) (string, error) {
	data, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func formatMetadataResult(output ImageMetadataOutput) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Image: %s\n", output.Path)
	if output.DateTaken != "" {
		fmt.Fprintf(&sb, "Date taken: %s\n", output.DateTaken)
	}
	if output.GPSLatitude != 0 || output.GPSLongitude != 0 {
		fmt.Fprintf(&sb, "GPS: %.6f, %.6f\n", output.GPSLatitude, output.GPSLongitude)
	}
	if output.NameLocal != "" || output.NameEng != "" {
		fmt.Fprintf(&sb, "Location: %s", output.NameLocal)
		if output.NameEng != "" && output.NameEng != output.NameLocal {
			fmt.Fprintf(&sb, " (%s)", output.NameEng)
		}
		sb.WriteString("\n")
	}
	if output.CameraModel != "" {
		fmt.Fprintf(&sb, "Camera: %s\n", output.CameraModel)
	}
	if output.LensModel != "" {
		fmt.Fprintf(&sb, "Lens: %s\n", output.LensModel)
	}
	if output.Width > 0 && output.Height > 0 {
		fmt.Fprintf(&sb, "Dimensions: %dx%d\n", output.Width, output.Height)
	}
	return sb.String()
}

func formatMetadataJSON(output ImageMetadataOutput) (string, error) {
	data, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// querySemanticSearch performs semantic search using vector similarity.
func (s *FlashbacksMCPServer) querySemanticSearch(query string, limit int) (SemanticSearchOutput, error) {
	result, err := imaging.SearchByEmbedding(s.db, query, limit)
	if err != nil {
		return SemanticSearchOutput{}, err
	}

	images := make([]SemanticSearchResult, 0, len(result.Images))
	for _, img := range result.Images {
		images = append(images, SemanticSearchResult{
			ID:         img.ImageFileID,
			Path:       img.Path,
			FileName:   fileNameFromPath(img.Path),
			ModTime:    img.ModTime.Format("2006-01-02 15:04:05"),
			Similarity: img.Similarity,
			Tags:       img.Tags,
		})
	}

	return SemanticSearchOutput{
		Images: images,
		Total:  len(images),
		Query:  query,
	}, nil
}

func formatSemanticSearchResult(output SemanticSearchOutput) string {
	if len(output.Images) == 0 {
		return fmt.Sprintf("No images found for query: %s", output.Query)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d images for query \"%s\":\n", output.Total, output.Query)
	for _, img := range output.Images {
		fmt.Fprintf(&sb, "- [%d] %s (similarity: %.0f%%)", img.ID, img.Path, img.Similarity*100)
		if len(img.Tags) > 0 {
			fmt.Fprintf(&sb, " tags: %s", strings.Join(img.Tags, ", "))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
