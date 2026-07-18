package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/domain"
	imgutil "github.com/flashbacks/api-service/internal/infrastructure/imaging"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
	"github.com/flashbacks/api-service/internal/infrastructure/llm/prompts"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// dataURLPattern matches base64-encoded data URLs (data:image/...;base64,...).
var dataURLPattern = regexp.MustCompile(`data:([^;]+);base64,([A-Za-z0-9+/=]+)`)

// extractAndSaveDataURL scans text for a base64 data URL and, if found,
// decodes it and writes the result to destPath.
func extractAndSaveDataURL(text, destPath string) (bool, error) {
	matches := dataURLPattern.FindStringSubmatch(text)
	if len(matches) < 3 {
		return false, nil
	}
	mimeType := matches[1]
	b64Data := matches[2]

	decoded, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return false, fmt.Errorf("failed to decode base64 image data: %w", err)
	}

	if err := os.WriteFile(destPath, decoded, 0644); err != nil {
		return false, fmt.Errorf("failed to write enhanced image to %s: %w", destPath, err)
	}

	log.Printf("extractAndSaveDataURL: saved enhanced image (%s, %d bytes) to %s", mimeType, len(decoded), destPath)
	return true, nil
}

// --- Tool input/output types ---

type RecognizeTextInput struct {
	ImagePath string `json:"image_path" jsonschema:"Path to the image file"`
}

type GenerateTagsInput struct {
	ImagePath string `json:"image_path" jsonschema:"Path to the image file"`
}

type AskAboutImageInput struct {
	ImagePath string `json:"image_path" jsonschema:"Path to the image file"`
	Question  string `json:"question" jsonschema:"The question to ask about the image"`
	Language  string `json:"language,omitempty" jsonschema:"Response language code (en, ru). Defaults to en"`
}

type EnhanceImageQualityInput struct {
	ImagePath   string `json:"image_path" jsonschema:"Path to the image file"`
	Instruction string `json:"instruction,omitempty" jsonschema:"Optional specific enhancement instruction (e.g. 'increase resolution', 'reduce blur', 'improve detail')"`
}

type ResizeImageInput struct {
	ImagePath  string `json:"image_path" jsonschema:"Path to the source image file"`
	OutputPath string `json:"output_path,omitempty" jsonschema:"Where to save the resized image (defaults to image_path + '.resized')"`
	MaxWidth   int    `json:"max_width,omitempty" jsonschema:"Maximum width in pixels (aspect ratio preserved)"`
	MaxHeight  int    `json:"max_height,omitempty" jsonschema:"Maximum height in pixels (aspect ratio preserved)"`
}

type ImageAnalysisOutput struct {
	Content          string   `json:"content,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	ProcessingTimeMs int      `json:"processingTimeMs"`
}

// --- Tool definitions for the agent ---

func recognizeTextToolDef() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "recognize_text",
		Description: "Extract and recognize all text from an image (OCR)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"image_path": map[string]any{"type": "string", "description": "Path to the image file"},
			},
			"required": []string{"image_path"},
		},
	}
}

func generateTagsToolDef() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "generate_tags",
		Description: "Generate descriptive tags for an image",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"image_path": map[string]any{"type": "string", "description": "Path to the image file"},
			},
			"required": []string{"image_path"},
		},
	}
}

func askAboutImageToolDef() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "ask_about_image",
		Description: "Answer a specific question about an image",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"image_path": map[string]any{"type": "string", "description": "Path to the image file"},
				"question":   map[string]any{"type": "string", "description": "The question to ask about the image"},
				"language":   map[string]any{"type": "string", "description": "Response language code (en, ru). Defaults to en"},
			},
			"required": []string{"image_path", "question"},
		},
	}
}

func enhanceImageQualityToolDef() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "enhance_image_quality",
		Description: "Improve image quality (automatically analyzes the image first): enhance details, reduce blur, upscale resolution. Creates a backup of the original image before modification. Call directly — no pre-analysis needed.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"image_path":  map[string]any{"type": "string", "description": "Path to the image file"},
				"instruction": map[string]any{"type": "string", "description": "Optional specific enhancement instruction (e.g. 'increase resolution', 'reduce blur', 'improve details')"},
			},
			"required": []string{"image_path"},
		},
	}
}

func resizeImageToolDef() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "resize_image",
		Description: "Resize an image to fit within maximum width and/or height while preserving aspect ratio. At least one of max_width or max_height must be specified. The resized image is saved as JPEG.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"image_path":  map[string]any{"type": "string", "description": "Path to the source image file"},
				"output_path": map[string]any{"type": "string", "description": "Where to save the resized image (defaults to image_path + '.resized')"},
				"max_width":   map[string]any{"type": "integer", "description": "Maximum width in pixels (aspect ratio preserved)"},
				"max_height":  map[string]any{"type": "integer", "description": "Maximum height in pixels (aspect ratio preserved)"},
			},
			"required": []string{"image_path"},
		},
	}
}

// --- Registration ---

func (s *FlashbacksMCPServer) registerImageTools() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "recognize_text",
		Description: "Extract and recognize all text from an image (OCR)",
	}, s.handleRecognizeText)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "generate_tags",
		Description: "Generate descriptive tags for an image",
	}, s.handleGenerateTags)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "ask_about_image",
		Description: "Answer a specific question about an image",
	}, s.handleAskAboutImage)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "enhance_image_quality",
		Description: "Improve image quality (automatically analyzes the image first): enhance details, reduce blur, upscale resolution. Creates a backup of the original image before modification. Call directly — no pre-analysis needed.",
	}, s.handleEnhanceImageQuality)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "resize_image",
		Description: "Resize an image to fit within a target megapixel limit. Dimensions are proportionally scaled and snapped to multiples of 32. The resized image is saved as JPEG next to the original.",
	}, s.handleResizeImage)
}

// --- MCP SDK handlers ---

func (s *FlashbacksMCPServer) handleRecognizeText(ctx context.Context, req *mcp.CallToolRequest, input RecognizeTextInput) (*mcp.CallToolResult, ImageAnalysisOutput, error) {
	result, err := s.runImageAction(input.ImagePath, "recognizeText", "", "en")
	if err != nil {
		return nil, ImageAnalysisOutput{}, err
	}
	output := ImageAnalysisOutput{
		Content:          result.Result,
		Provider:         result.Provider,
		Model:            result.Model,
		ProcessingTimeMs: result.ProcessingTimeMs,
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result.Result}},
	}, output, nil
}

func (s *FlashbacksMCPServer) handleGenerateTags(ctx context.Context, req *mcp.CallToolRequest, input GenerateTagsInput) (*mcp.CallToolResult, ImageAnalysisOutput, error) {
	result, err := s.runImageAction(input.ImagePath, "tags", "", "en")
	if err != nil {
		return nil, ImageAnalysisOutput{}, err
	}
	output := ImageAnalysisOutput{
		Tags:             result.Tags,
		Content:          result.Result,
		Provider:         result.Provider,
		Model:            result.Model,
		ProcessingTimeMs: result.ProcessingTimeMs,
	}
	tagsText := strings.Join(result.Tags, ", ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: tagsText}},
	}, output, nil
}

func (s *FlashbacksMCPServer) handleEnhanceImageQuality(ctx context.Context, req *mcp.CallToolRequest, input EnhanceImageQualityInput) (*mcp.CallToolResult, ImageAnalysisOutput, error) {
	instruction := input.Instruction

	result, err := s.runImageEnhancement(input.ImagePath, instruction, "en")
	if err != nil {
		return nil, ImageAnalysisOutput{}, err
	}
	output := ImageAnalysisOutput{
		Content:          result.Result,
		Provider:         result.Provider,
		Model:            result.Model,
		ProcessingTimeMs: result.ProcessingTimeMs,
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result.Result}},
	}, output, nil
}

func (s *FlashbacksMCPServer) handleAskAboutImage(ctx context.Context, req *mcp.CallToolRequest, input AskAboutImageInput) (*mcp.CallToolResult, ImageAnalysisOutput, error) {
	lang := input.Language
	if lang == "" {
		lang = "en"
	}
	result, err := s.runImageAction(input.ImagePath, "askQuestion", input.Question, lang)
	if err != nil {
		return nil, ImageAnalysisOutput{}, err
	}
	output := ImageAnalysisOutput{
		Content:          result.Result,
		Provider:         result.Provider,
		Model:            result.Model,
		ProcessingTimeMs: result.ProcessingTimeMs,
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result.Result}},
	}, output, nil
}

func (s *FlashbacksMCPServer) handleResizeImage(ctx context.Context, req *mcp.CallToolRequest, input ResizeImageInput) (*mcp.CallToolResult, ImageAnalysisOutput, error) {
	outputPath := input.OutputPath
	if outputPath == "" {
		outputPath = input.ImagePath + ".resized"
	}
	if outputPath == input.ImagePath {
		return nil, ImageAnalysisOutput{}, fmt.Errorf("output_path must differ from image_path to avoid overwriting the source file")
	}

	if err := imgutil.ResizeImage(input.ImagePath, outputPath, input.MaxWidth, input.MaxHeight); err != nil {
		return nil, ImageAnalysisOutput{}, fmt.Errorf("failed to resize image: %w", err)
	}

	result := fmt.Sprintf("Image resized: %s → %s (max %dx%d px, aspect ratio preserved)",
		input.ImagePath, outputPath, input.MaxWidth, input.MaxHeight)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, ImageAnalysisOutput{}, nil
}

// --- Direct execution methods (for agent) ---

func (s *FlashbacksMCPServer) executeRecognizeText(ctx context.Context, args json.RawMessage) (string, error) {
	var input RecognizeTextInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := s.runImageAction(input.ImagePath, "recognizeText", "", "en")
	if err != nil {
		return "", err
	}
	return result.Result, nil
}

func (s *FlashbacksMCPServer) executeGenerateTags(ctx context.Context, args json.RawMessage) (string, error) {
	var input GenerateTagsInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	result, err := s.runImageAction(input.ImagePath, "tags", "", "en")
	if err != nil {
		return "", err
	}
	return strings.Join(result.Tags, ", "), nil
}

func (s *FlashbacksMCPServer) executeAskAboutImage(ctx context.Context, args json.RawMessage) (string, error) {
	var input AskAboutImageInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	lang := input.Language
	if lang == "" {
		lang = "en"
	}
	result, err := s.runImageAction(input.ImagePath, "askQuestion", input.Question, lang)
	if err != nil {
		return "", err
	}
	return result.Result, nil
}

func (s *FlashbacksMCPServer) executeResizeImage(ctx context.Context, args json.RawMessage) (string, error) {
	var input ResizeImageInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	outputPath := input.OutputPath
	if outputPath == "" {
		outputPath = input.ImagePath + ".resized"
	}
	if outputPath == input.ImagePath {
		return "", fmt.Errorf("output_path must differ from image_path to avoid overwriting the source file")
	}

	if err := imgutil.ResizeImage(input.ImagePath, outputPath, input.MaxWidth, input.MaxHeight); err != nil {
		return "", fmt.Errorf("failed to resize image: %w", err)
	}

	return fmt.Sprintf("Image resized: %s → %s (max %dx%d px, aspect ratio preserved)", input.ImagePath, outputPath, input.MaxWidth, input.MaxHeight), nil
}

func (s *FlashbacksMCPServer) executeEnhanceImageQuality(ctx context.Context, args json.RawMessage) (string, error) {
	var input EnhanceImageQualityInput
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	result, err := s.runImageEnhancement(input.ImagePath, input.Instruction, "en")
	if err != nil {
		return "", err
	}
	return result.Result, nil
}

// --- Shared logic ---

type imageActionResult struct {
	Result           string
	Tags             []string
	Provider         string
	Model            string
	ProcessingTimeMs int
}

// enhancementResult is the JSON-serialized result returned by enhance_image_quality.
// The frontend parses this to determine whether a comparison view should be shown.
type enhancementResult struct {
	Message      string `json:"message"`
	EnhancedPath string `json:"enhancedPath,omitempty"`
	Status       string `json:"status"` // "pending_approval" or "no_change"
	Prompt       string `json:"prompt,omitempty"`
}

// runImageAction executes an AI action on an image identified by its path.
// For the "tags" action, it checks the image_tags cache first and saves
// newly generated tags back to the cache.
// backupOriginalImage copies the original image to the exifBackupDir for safekeeping.
func (s *FlashbacksMCPServer) backupOriginalImage(imagePath string) error {
	var settings domain.AppSettings
	if err := s.db.First(&settings, 1).Error; err != nil {
		return fmt.Errorf("app settings not found: %w", err)
	}
	if settings.ExifBackupDir == "" {
		return nil // no backup directory configured, skip
	}

	// Copy the original file to the backup directory
	// Keep the same filename structure to allow restoration
	dest := filepath.Join(settings.ExifBackupDir, filepath.Base(imagePath))
	input, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open original image: %w", err)
	}
	defer input.Close()

	output, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create backup at %s: %w", dest, err)
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return fmt.Errorf("failed to copy image to backup: %w", err)
	}

	log.Printf("enhance_image_quality: backed up original to %s", dest)
	return nil
}

// runImageEnhancement performs AI-powered image quality enhancement.
// Uses a two-step process:
//  1. Pre-analyze the image with a VL LLM to get enhancement recommendations
//  2. Call the image editing LLM with dynamic, image-specific enhancement instructions
//
// The enhanced image is saved to a temporary file (originalPath + ".enhanced").
// The result includes a JSON payload with the temp path so the frontend can
// show a before/after comparison and let the user accept or reject the changes.
func (s *FlashbacksMCPServer) runImageEnhancement(imagePath, instruction, language string) (*imageActionResult, error) {
	if language == "" {
		language = "en"
	}

	// Validate that the image exists on disk and is within a gallery folder.
	// We accept both DB-tracked and temporary files (e.g. .resized outputs from
	// the resize_image tool) as long as they reside under a gallery folder.
	if _, statErr := os.Stat(imagePath); statErr != nil {
		return nil, fmt.Errorf("image not found: %s", imagePath)
	}
	inGallery, galleryErr := s.isPathInGalleryFolder(imagePath)
	if galleryErr != nil {
		return nil, fmt.Errorf("failed to verify gallery access: %w", galleryErr)
	}
	if !inGallery {
		return nil, fmt.Errorf("image not found: %s", imagePath)
	}

	// Step 1: Pre-analyze image with VL LLM to generate enhancement recommendations.
	// This step is mandatory — it provides image-specific guidance to the editing LLM.
	// The instruction parameter is intentionally ignored here: the general enhancement
	// guidelines are already in the system prompt, and the pre-analysis below provides
	// image-specific, actionable recommendations in English.
	vlClient, _, _, vlErr := s.createVLClient()
	if vlErr != nil {
		return nil, fmt.Errorf("pre-analysis VL client unavailable: %w", vlErr)
	}
	preAnalysisPrompt := prompts.BuildEnhancePreAnalysisPrompt(language)
	preAnalysisResult, analyzeErr := vlClient.Recognize(
		context.Background(), imagePath,
		preAnalysisPrompt,
		"Analyze this image for quality issues and provide enhancement recommendations.",
	)
	var recommendations string
	if analyzeErr == nil && preAnalysisResult != "" {
		recommendations = preAnalysisResult
	}

	// Step 2: Call image editing LLM with dynamic, image-specific recommendations
	client, providerName, modelName, err := s.createImgEditClient()
	if err != nil {
		return nil, err
	}

	systemPrompt := prompts.BuildActionPrompt("enhanceQuality", recommendations, language)
	userMessage := prompts.BuildActionUserMessage("enhanceQuality")
	if recommendations != "" {
		userMessage = fmt.Sprintf("Enhance this image by applying the following improvements: %s", recommendations)
	}
	fullPrompt := "System: " + systemPrompt + "\n\nUser: " + userMessage

	// Call LLM — use native image editing API when available (e.g. Alibaba DashScope),
	// otherwise fall back to general-purpose VL recognition.
	startTime := time.Now()
	var response string
	if editor, ok := client.(llm.ImageEditor); ok {
		response, err = editor.EditImage(context.Background(), imagePath, systemPrompt, userMessage)
	} else {
		response, err = client.Recognize(context.Background(), imagePath, systemPrompt, userMessage)
	}
	processingTime := int(time.Since(startTime).Milliseconds())

	if err != nil {
		return nil, fmt.Errorf("image editing LLM call failed: %w", err)
	}

	// Save enhanced image to a temporary file (originalPath + ".enhanced")
	// instead of overwriting the original. The user will decide to accept or reject.
	enhancedPath := imagePath + ".enhanced"
	saved, saveErr := extractAndSaveDataURL(response, enhancedPath)
	if saveErr != nil {
		log.Printf("runImageEnhancement: failed to save enhanced image: %v", saveErr)
	}

	if saved {
		resultJSON, _ := json.Marshal(enhancementResult{
			Message:      "Image enhancement completed. A preview is available for comparison.",
			EnhancedPath: enhancedPath,
			Status:       "pending_approval",
			Prompt:       fullPrompt,
		})
		return &imageActionResult{
			Result:           string(resultJSON),
			Provider:         providerName,
			Model:            modelName,
			ProcessingTimeMs: processingTime,
		}, nil
	}

	// No image data in response — the model returned text only.
	resultJSON, _ := json.Marshal(enhancementResult{
		Message: response + "\n\n⚠️ The model did not return enhanced image data. The original file was not modified.",
		Status:  "no_change",
		Prompt:  fullPrompt,
	})
	log.Printf("runImageEnhancement: LLM returned text-only response (no image data) for %s", imagePath)
	return &imageActionResult{
		Result:           string(resultJSON),
		Provider:         providerName,
		Model:            modelName,
		ProcessingTimeMs: processingTime,
	}, nil
}

func (s *FlashbacksMCPServer) runImageAction(imagePath, action, question, language string) (*imageActionResult, error) {
	if language == "" {
		language = "en"
	}

	// Resolve image file from DB
	var imageFile domain.ImageFile
	if err := s.db.Where("path = ?", imagePath).First(&imageFile).Error; err != nil {
		return nil, fmt.Errorf("image not found: %s", imagePath)
	}

	// For "tags" action: check DB cache first
	if action == "tags" {
		var tags []domain.ImageTag
		if err := s.db.Where("image_file_id = ?", imageFile.ID).Find(&tags).Error; err == nil && len(tags) > 0 {
			tagStrings := make([]string, len(tags))
			for i, t := range tags {
				tagStrings[i] = t.Tag
			}
			return &imageActionResult{
				Tags:   tagStrings,
				Result: strings.Join(tagStrings, ", "),
			}, nil
		}
	}

	// Create LLM client
	client, providerName, modelName, err := s.createVLClient()
	if err != nil {
		return nil, err
	}

	// Build prompts
	systemPrompt := prompts.BuildActionPrompt(action, question, language)
	userMessage := prompts.BuildActionUserMessage(action)

	// Call LLM
	startTime := time.Now()
	response, err := client.Recognize(context.Background(), imageFile.Path, systemPrompt, userMessage)
	processingTime := int(time.Since(startTime).Milliseconds())

	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	result := &imageActionResult{
		Provider:         providerName,
		Model:            modelName,
		ProcessingTimeMs: processingTime,
	}

	if action == "tags" {
		rawTags := prompts.ParseTags(response)
		tags, err := imaging.PostProcessTags(rawTags)
		if err != nil {
			return nil, fmt.Errorf("tag post-processing failed: %w", err)
		}
		result.Tags = tags
		result.Result = strings.Join(tags, ", ")

		// Save generated tags to DB cache for future requests
		if len(tags) > 0 {
			if err := s.llmService.SaveImageTags(imageFile.ID, tags); err != nil {
				// Log but don't fail — tags were generated successfully
				log.Printf("generate_tags: failed to save tags for image %d: %v", imageFile.ID, err)
			}
			// Generate embedding immediately for the just-tagged image
			go imaging.GenerateAndSaveEmbedding(s.db, imageFile.ID, tags)
			// Also trigger batch backfill for any other images missing embeddings
			if s.embeddingBackfill != nil {
				go func() {
					if err := s.embeddingBackfill.Start(); err != nil {
						log.Printf("generate_tags: embedding backfill not started: %v", err)
					}
				}()
			}
		}
	} else {
		result.Result = response
	}

	return result, nil
}
