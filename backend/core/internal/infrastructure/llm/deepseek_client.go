package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/flashbacks/core/internal/infrastructure/imaging"
)

// DeepSeekModelInfo holds known context window sizes for DeepSeek models.
type DeepSeekModelInfo struct {
	ContextWindow int
}

// knownDeepSeekModels maps model ID (or prefix) to context window size.
// DeepSeek publishes these values; the map serves as a fallback when the
// model cache does not contain a contextLength.
var knownDeepSeekModels = map[string]DeepSeekModelInfo{
	"deepseek-chat":          {ContextWindow: 128000},
	"deepseek-reasoner":      {ContextWindow: 128000},
	"deepseek-v4-pro":        {ContextWindow: 1000000},
	"deepseek-v4-flash":      {ContextWindow: 1000000},
	"deepseek-v4-pro-0324":   {ContextWindow: 1000000},
	"deepseek-v4-flash-0324": {ContextWindow: 1000000},
}

// DeepSeekUsage carries the extended token usage information returned by
// the DeepSeek API. These fields go beyond the standard OpenAI usage object.
type DeepSeekUsage struct {
	PromptTokens            int `json:"prompt_tokens"`
	CompletionTokens        int `json:"completion_tokens"`
	PromptCacheHitTokens    int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens   int `json:"prompt_cache_miss_tokens"`
	TotalTokens             int `json:"total_tokens"`
	CompletionTokensDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details,omitempty"`
}

// DeepSeekClient implements ChatClient for DeepSeek's API.
// It extends the standard OpenAI-compatible interface with DeepSeek-specific
// features such as context caching statistics, reasoning token counts, and
// known model context window sizes.
type DeepSeekClient struct {
	*apiClient
	APIKey  string
	Model   string
	Timeout time.Duration
}

// NewDeepSeekClient creates a new DeepSeek API client.
// The baseURL is normalized by stripping any trailing /v1 or /v1/ suffix.
func NewDeepSeekClient(baseURL, apiKey, model string) *DeepSeekClient {
	baseURL = strings.TrimRight(baseURL, "/")
	if before, ok := strings.CutSuffix(baseURL, "/v1"); ok {
		baseURL = before
	}
	timeout := 5 * time.Minute
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	return &DeepSeekClient{
		apiClient: newAPIClient(baseURL, timeout, headers),
		APIKey:    apiKey,
		Model:     model,
		Timeout:   timeout,
	}
}

// ContextWindow returns the known context window size for the configured model.
// Falls back to 128K if the model is not in the registry.
func (c *DeepSeekClient) ContextWindow() int {
	if w := deepSeekKnownContext(c.Model); w > 0 {
		return w
	}
	return 128000
}

// deepSeekKnownContext returns the known context window for a model ID using
// exact then prefix matching, or 0 when the model is unknown.
func deepSeekKnownContext(modelID string) int {
	if info, ok := knownDeepSeekModels[modelID]; ok {
		return info.ContextWindow
	}
	for prefix, info := range knownDeepSeekModels {
		if strings.HasPrefix(modelID, prefix) {
			return info.ContextWindow
		}
	}
	return 0
}

// --- Request / Response types ---

type deepSeekRequest struct {
	Model     string                `json:"model"`
	Messages  []deepSeekChatMessage `json:"messages"`
	MaxTokens int                   `json:"max_tokens,omitempty"`
	Tools     []openAIToolParam     `json:"tools,omitempty"`
	Stream    bool                  `json:"stream,omitempty"`
}

type deepSeekChatMessage struct {
	Role       string               `json:"role"`
	Content    any                  `json:"content,omitempty"`
	ToolCalls  []openAIToolCallResp `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

type deepSeekResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int                   `json:"index"`
		Message      deepSeekChoiceMessage `json:"message"`
		FinishReason string                `json:"finish_reason"`
	} `json:"choices"`
	Usage *DeepSeekUsage `json:"usage,omitempty"`
}

type deepSeekChoiceMessage struct {
	Role      string               `json:"role"`
	Content   any                  `json:"content"` // string or []openAIContent for multimodal
	ToolCalls []openAIToolCallResp `json:"tool_calls,omitempty"`
}

// --- Client interface ---

// Recognize performs image recognition using DeepSeek's VL capabilities.
// Merges systemPrompt into the user message content list to maximize
// compatibility with providers that reject a leading system role or
// require all content fields to be lists (multimodal format).
func (c *DeepSeekClient) Recognize(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
	imgData, mediaType, err := imaging.DownsizeImageForLLM(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to prepare image: %w", err)
	}

	base64Img := fmt.Sprintf("data:%s;base64,%s", mediaType, encodeBase64(imgData))

	// Build user content list, prepending system prompt for compatibility
	// with providers that require all messages to use the multimodal
	// content-list format and/or reject a leading system role.
	userContent := []openAIContent{
		{Type: "text", Text: systemPrompt + "\n\n" + userMessage},
		{Type: "image_url", ImageURL: &openAIImageURL{URL: base64Img}},
	}

	req := deepSeekRequest{
		Model: c.Model,
		Messages: []deepSeekChatMessage{
			{
				Role:    "user",
				Content: userContent,
			},
		},
		MaxTokens: 4000,
	}

	var resp deepSeekResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/chat/completions", req, &resp, nil); err != nil {
		return "", fmt.Errorf("DeepSeek recognize: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	content := resp.Choices[0].Message.Content
	text := extractContentText(content)

	// If the response has multimodal content with embedded images,
	// prepend them as data URLs so callers can extract and save them.
	images := extractContentImages(content)
	for _, img := range images {
		dataURL := fmt.Sprintf("data:%s;base64,%s", img.MIMEType, base64.StdEncoding.EncodeToString(img.Data))
		text = dataURL + "\n" + text
	}

	// Debug: log content type to diagnose missing images.
	switch content.(type) {
	case string:
		// plain text
	case []interface{}:
		logContentDebug(content)
	default:
		log.Printf("DeepSeek recognize: unexpected content type %T", content)
	}

	return text, nil
}

// Chat performs a conversational LLM call with optional tool definitions.
// It implements the ChatClient interface and parses DeepSeek's extended usage info.
func (c *DeepSeekClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	messages := make([]deepSeekChatMessage, len(req.Messages))
	for i, m := range req.Messages {
		msg := deepSeekChatMessage{
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

	dsReq := deepSeekRequest{
		Model:     c.Model,
		Messages:  messages,
		MaxTokens: 4000,
		Tools:     tools,
	}

	var dsResp deepSeekResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/chat/completions", dsReq, &dsResp, nil); err != nil {
		return nil, fmt.Errorf("DeepSeek chat: %w", err)
	}

	if len(dsResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in DeepSeek chat response")
	}

	choice := dsResp.Choices[0]
	chatResp := &ChatResponse{
		Message: ChatMessage{
			Role:    "assistant",
			Content: extractContentText(choice.Message.Content),
		},
	}

	// Parse usage information
	if dsResp.Usage != nil {
		chatResp.Usage = &ChatUsage{
			PromptTokens:          dsResp.Usage.PromptTokens,
			CompletionTokens:      dsResp.Usage.CompletionTokens,
			TotalTokens:           dsResp.Usage.TotalTokens,
			PromptCacheHitTokens:  dsResp.Usage.PromptCacheHitTokens,
			PromptCacheMissTokens: dsResp.Usage.PromptCacheMissTokens,
		}
		if dsResp.Usage.CompletionTokensDetails != nil {
			chatResp.Usage.ReasoningTokens = dsResp.Usage.CompletionTokensDetails.ReasoningTokens
		}
	}

	// Parse tool calls
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

// ListModels returns a list of available models from the DeepSeek API.
// It prefers the rich /models endpoint (context_window, modalities, name) and
// falls back to the OpenAI-compatible /v1/models endpoint when unavailable.
func (c *DeepSeekClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	shortClient := newAPIClient(c.baseURL, 30*time.Second, c.headers)

	if models, err := c.listRichModels(ctx, shortClient); err == nil && len(models) > 0 {
		return models, nil
	}

	var modelsResp openAIModelsResponse
	if err := shortClient.doJSON(ctx, http.MethodGet, "/v1/models", nil, &modelsResp, nil); err != nil {
		return nil, fmt.Errorf("DeepSeek list models: %w", err)
	}

	models := make([]ModelInfo, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		// The plain listing exposes only identifiers; everything else is inferred.
		info := ModelInfo{
			ID:            m.ID,
			Name:          m.ID,
			ContextLength: deepSeekKnownContext(m.ID),
			Capabilities:  inferCapabilitiesByID(m.ID),
			Provenance:    ProvenanceHeuristic,
		}
		info.InputModalities, info.OutputModalities = inferModalities(info.Capabilities)
		if p, ok := lookupModelPricing(m.ID); ok {
			info.InputPricePerMTok = p.InputPerMTok
			info.OutputPricePerMTok = p.OutputPerMTok
		}
		models[i] = info
	}

	return models, nil
}

// deepSeekRichModel is the rich model descriptor returned by DeepSeek GET /models.
type deepSeekRichModel struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	ContextWindow    int      `json:"context_window"`
	MaxOutputTokens  int      `json:"max_output_tokens"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

type deepSeekRichModelsResponse struct {
	Data []deepSeekRichModel `json:"data"`
}

// listRichModels parses the rich DeepSeek /models response into ModelInfo values.
func (c *DeepSeekClient) listRichModels(ctx context.Context, client *apiClient) ([]ModelInfo, error) {
	var resp deepSeekRichModelsResponse
	if err := client.doJSON(ctx, http.MethodGet, "/models", nil, &resp, nil); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(resp.Data))
	for _, m := range resp.Data {
		if m.ID == "" {
			continue
		}
		info := ModelInfo{
			ID:               m.ID,
			Name:             m.Name,
			ContextLength:    m.ContextWindow,
			InputModalities:  mergeModalities(m.InputModalities),
			OutputModalities: mergeModalities(m.OutputModalities),
			Provenance:       ProvenancePartial,
		}
		if len(info.InputModalities) > 0 && info.ContextLength > 0 {
			info.Provenance = ProvenanceRich
		}
		if info.Name == "" {
			info.Name = m.ID
		}
		if info.ContextLength == 0 {
			info.ContextLength = deepSeekKnownContext(m.ID)
		}
		if len(info.InputModalities) == 0 && len(info.OutputModalities) == 0 {
			info.Capabilities = inferCapabilitiesByID(m.ID)
			info.InputModalities, info.OutputModalities = inferModalities(info.Capabilities)
		} else {
			info.Capabilities = capabilitiesFromModalities(info.InputModalities, info.OutputModalities)
		}
		if strings.Contains(strings.ToLower(m.ID), "embedding") {
			info.Capabilities = []string{"embedding"}
		}
		if p, ok := lookupModelPricing(m.ID); ok {
			info.InputPricePerMTok = p.InputPerMTok
			info.OutputPricePerMTok = p.OutputPerMTok
		}
		models = append(models, info)
	}

	return models, nil
}

// encodeBase64 encodes bytes to a base64 string.
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
