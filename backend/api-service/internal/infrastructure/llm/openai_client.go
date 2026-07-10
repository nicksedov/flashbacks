package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAIClient implements Client for OpenAI-compatible API
type OpenAIClient struct {
	*apiClient
	APIKey             string
	Model              string
	Timeout            time.Duration
	MaxImageMegapixels float64
}

// NewOpenAIClient creates a new OpenAI client.
// The baseURL is normalized by stripping any trailing /v1 or /v1/ suffix
// so that endpoint paths like /v1/models are not duplicated.
func NewOpenAIClient(baseURL, apiKey, model string, maxImageMegapixels float64) *OpenAIClient {
	baseURL = normalizeOpenAIBaseURL(baseURL)
	timeout := 5 * time.Minute
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	return &OpenAIClient{
		apiClient:          newAPIClient(baseURL, timeout, headers),
		APIKey:             apiKey,
		Model:              model,
		Timeout:            timeout,
		MaxImageMegapixels: maxImageMegapixels,
	}
}

// normalizeOpenAIBaseURL strips trailing slashes and an optional /v1 suffix
// from the base URL. OpenAI-compatible providers may supply a base URL that
// already includes /v1 (e.g. https://host/compatible-mode/v1). Without
// normalization the client would build paths like /v1/v1/models → 404.
func normalizeOpenAIBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if before, ok :=strings.CutSuffix(baseURL, "/v1"); ok  {
		baseURL = before
	}
	return baseURL
}

// openAIRequest represents OpenAI chat completion request
type openAIRequest struct {
	Model     string              `json:"model"`
	Messages  []openAIChatMessage `json:"messages"`
	MaxTokens int                 `json:"max_tokens,omitempty"`
	Tools     []openAIToolParam   `json:"tools,omitempty"`
}

type openAIChatMessage struct {
	Role       string               `json:"role"`
	Content    any                  `json:"content,omitempty"` // string or []openAIContent for multimodal
	ToolCalls  []openAIToolCallResp `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

type openAIToolParam struct {
	Type     string              `json:"type"` // "function"
	Function openAIFunctionParam `json:"function"`
}

type openAIFunctionParam struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIToolCallResp struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIContent struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *openAIImageURL `json:"image_url,omitempty"`
}

type openAIImageURL struct {
	URL string `json:"url"`
}

// openAIResponse represents OpenAI chat completion response
type openAIResponse struct {
	Choices []struct {
		Message      openAIChoiceMessage `json:"message"`
		FinishReason string              `json:"finish_reason"`
	} `json:"choices"`
}

type openAIChoiceMessage struct {
	Content   string               `json:"content"`
	ToolCalls []openAIToolCallResp `json:"tool_calls,omitempty"`
}

// Recognize performs OCR using OpenAI-compatible API
func (c *OpenAIClient) Recognize(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
	// Read and optionally resize image
	imgData, mediaType, err := resizeImageForLLM(imagePath, c.MaxImageMegapixels)
	if err != nil {
		return "", fmt.Errorf("failed to prepare image: %w", err)
	}

	// Encode image to base64 data URL
	base64Img := base64.StdEncoding.EncodeToString(imgData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mediaType, base64Img)

	req := openAIRequest{
		Model: c.Model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemPrompt},
			{
				Role: "user",
				Content: []openAIContent{
					{Type: "text", Text: userMessage},
					{Type: "image_url", ImageURL: &openAIImageURL{URL: dataURL}},
				},
			},
		},
		MaxTokens: 4000,
	}

	var openAIResp openAIResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/chat/completions", req, &openAIResp, nil); err != nil {
		return "", fmt.Errorf("OpenAI recognize: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

// Chat performs a conversational LLM call with optional tool definitions.
// It implements the ChatClient interface.
func (c *OpenAIClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	messages := make([]openAIChatMessage, len(req.Messages))
	for i, m := range req.Messages {
		msg := openAIChatMessage{
			Role:       m.Role,
			ToolCallID: m.ToolCallID,
		}
		if m.Role == "tool" {
			msg.Content = m.Content
		} else if len(m.ToolCalls) > 0 {
			msg.Content = m.Content
			msg.ToolCalls = make([]openAIToolCallResp, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				msg.ToolCalls[j] = openAIToolCallResp{
					ID:   tc.ID,
					Type: "function",
				}
				msg.ToolCalls[j].Function.Name = tc.Name
				msg.ToolCalls[j].Function.Arguments = string(tc.Arguments)
			}
		} else {
			msg.Content = m.Content
		}
		messages[i] = msg
	}

	var tools []openAIToolParam
	for _, t := range req.Tools {
		tools = append(tools, openAIToolParam{
			Type: "function",
			Function: openAIFunctionParam{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	oaiReq := openAIRequest{
		Model:     c.Model,
		Messages:  messages,
		MaxTokens: 4000,
		Tools:     tools,
	}

	var oaiResp openAIResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/chat/completions", oaiReq, &oaiResp, nil); err != nil {
		return nil, fmt.Errorf("OpenAI chat: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in chat response")
	}

	choice := oaiResp.Choices[0]
	chatResp := &ChatResponse{
		Message: ChatMessage{
			Role:    "assistant",
			Content: choice.Message.Content,
		},
	}

	if len(choice.Message.ToolCalls) > 0 {
		chatResp.StopReason = "tool_use"
		for _, tc := range choice.Message.ToolCalls {
			chatResp.Message.ToolCalls = append(chatResp.Message.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
		}
	} else {
		chatResp.StopReason = "end_turn"
	}

	return chatResp, nil
}

// openAIModelsResponse represents OpenAI models list response
type openAIModelsResponse struct {
	Data []openAIModel `json:"data"`
}

type openAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
}

// ListModels returns a list of available models from OpenAI-compatible server
func (c *OpenAIClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Use a shorter timeout for model listing
	shortClient := newAPIClient(c.baseURL, 30*time.Second, c.headers)

	var modelsResp openAIModelsResponse
	if err := shortClient.doJSON(ctx, http.MethodGet, "/v1/models", nil, &modelsResp, nil); err != nil {
		return nil, fmt.Errorf("OpenAI list models: %w", err)
	}

	models := make([]ModelInfo, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = ModelInfo{
			ID:   m.ID,
			Name: m.ID,
		}
	}

	return models, nil
}
