package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
	"github.com/flashbacks/api-service/internal/infrastructure/llm/prompts"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

// --- Shared logic ---

type imageActionResult struct {
	Result           string
	Tags             []string
	Provider         string
	Model            string
	ProcessingTimeMs int
}

// runImageAction executes an AI action on an image identified by its path.
// For the "tags" action, it checks the image_tags cache first and saves
// newly generated tags back to the cache.
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
	client, providerName, modelName, err := s.createLLMClient()
	if err != nil {
		return nil, err
	}

	// Build prompts
	systemPrompt := prompts.BuildActionPrompt(action, question, language)
	userMessage := prompts.BuildActionUserMessage(action)

	// Call LLM
	startTime := time.Now()
	response, err := client.Recognize(imageFile.Path, systemPrompt, userMessage)
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
