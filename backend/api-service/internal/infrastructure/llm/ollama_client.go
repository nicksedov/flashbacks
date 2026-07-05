package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// OllamaClient implements Client for Ollama API
type OllamaClient struct {
	*apiClient
	APIKey             string
	Model              string
	Timeout            time.Duration
	MaxImageMegapixels float64
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, apiKey, model string, maxImageMegapixels float64) *OllamaClient {
	timeout := 5 * time.Minute // VL models can be slow
	headers := map[string]string{}
	if apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}
	return &OllamaClient{
		apiClient:          newAPIClient(baseURL, timeout, headers),
		APIKey:             apiKey,
		Model:              model,
		Timeout:            timeout,
		MaxImageMegapixels: maxImageMegapixels,
	}
}

// ollamaRequest represents Ollama chat request
type ollamaRequest struct {
	Model    string            `json:"model"`
	Messages []ollamaMessage   `json:"messages"`
	Stream   bool              `json:"stream"`
	Tools    []ollamaToolParam `json:"tools,omitempty"`
}

type ollamaMessage struct {
	Role      string              `json:"role"`
	Content   string              `json:"content"`
	Images    []string            `json:"images,omitempty"`     // Base64 encoded images
	ToolCalls []ollamaToolCallMsg `json:"tool_calls,omitempty"` // Populated when assistant requests tool use
}

type ollamaToolParam struct {
	Type     string             `json:"type"` // "function"
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ollamaToolCallMsg struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

// ollamaResponse represents Ollama chat response
type ollamaResponse struct {
	Message ollamaResponseMessage `json:"message"`
	Done    bool                  `json:"done"`
}

type ollamaResponseMessage struct {
	Content   string              `json:"content"`
	ToolCalls []ollamaToolCallMsg `json:"tool_calls,omitempty"`
}

// Recognize performs OCR using Ollama
func (c *OllamaClient) Recognize(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
	// Read and optionally resize image
	imgData, _, err := resizeImageForLLM(imagePath, c.MaxImageMegapixels)
	if err != nil {
		return "", fmt.Errorf("failed to prepare image: %w", err)
	}

	// Encode image to base64
	base64Img := base64.StdEncoding.EncodeToString(imgData)

	req := ollamaRequest{
		Model: c.Model,
		Messages: []ollamaMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage, Images: []string{base64Img}},
		},
		Stream: false,
	}

	var ollamaResp ollamaResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/chat", req, &ollamaResp, nil); err != nil {
		return "", fmt.Errorf("Ollama recognize: %w", err)
	}

	return ollamaResp.Message.Content, nil
}

// Chat performs a conversational LLM call with optional tool definitions.
// It implements the ChatClient interface.
func (c *OllamaClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	messages := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		msg := ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		// If this is an assistant message with tool calls, populate tool_calls
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				var args map[string]any
				if err := json.Unmarshal(tc.Arguments, &args); err != nil {
					args = map[string]any{"raw": string(tc.Arguments)}
				}
				tcMsg := ollamaToolCallMsg{}
				tcMsg.Function.Name = tc.Name
				tcMsg.Function.Arguments = args
				msg.ToolCalls = append(msg.ToolCalls, tcMsg)
			}
		}
		messages[i] = msg
	}

	var tools []ollamaToolParam
	for _, t := range req.Tools {
		tools = append(tools, ollamaToolParam{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	oReq := ollamaRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   false,
		Tools:    tools,
	}

	var oResp ollamaResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/chat", oReq, &oResp, nil); err != nil {
		return nil, fmt.Errorf("Ollama chat: %w", err)
	}

	chatResp := &ChatResponse{
		Message: ChatMessage{
			Role:    "assistant",
			Content: oResp.Message.Content,
		},
	}

	if len(oResp.Message.ToolCalls) > 0 {
		chatResp.StopReason = "tool_use"
		for i, tc := range oResp.Message.ToolCalls {
			argsJSON, err := json.Marshal(tc.Function.Arguments)
			if err != nil {
				argsJSON = []byte("{}")
			}
			chatResp.Message.ToolCalls = append(chatResp.Message.ToolCalls, ToolCall{
				ID:        fmt.Sprintf("call_%d", i),
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(argsJSON),
			})
		}
	} else {
		chatResp.StopReason = "end_turn"
	}

	return chatResp, nil
}

// ollamaTagsResponse represents Ollama tags response
type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
	Digest     string `json:"digest"`
}

// ListModels returns a list of available models from Ollama server.
// It also fetches context length for each model via /api/show concurrently.
func (c *OllamaClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	var tagsResp ollamaTagsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/tags", nil, &tagsResp, nil); err != nil {
		return nil, fmt.Errorf("Ollama list models: %w", err)
	}

	models := make([]ModelInfo, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		models[i] = ModelInfo{
			ID:   m.Name,
			Name: m.Name,
			Size: m.Size,
		}
	}

	// Fetch context length concurrently for each model (bounded to 4 goroutines)
	const maxConcurrency = 4
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := range models {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctxLen := c.fetchContextLength(ctx, models[idx].Name)
			if ctxLen > 0 {
				mu.Lock()
				models[idx].ContextLength = ctxLen
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	return models, nil
}

// fetchContextLength calls /api/show to extract context_length for a model.
func (c *OllamaClient) fetchContextLength(ctx context.Context, modelName string) int {
	// Use a short-lived client with 30s timeout for this call
	shortClient := newAPIClient(c.baseURL, 30*time.Second, c.headers)

	var showResp struct {
		ModelInfo  map[string]any `json:"model_info"`
		Parameters string         `json:"parameters"`
	}
	err := shortClient.doJSON(ctx, http.MethodPost, "/api/show", map[string]string{"name": modelName}, &showResp, nil)
	if err != nil {
		return 0
	}

	// Try model_info map for known keys.
	// The key format is "{architecture}.context_length" where architecture
	// varies per model (llama, qwen3, gemma2, mistral, etc.), so we search
	// for any key ending with ".context_length" + the bare "context_length".
	for _, key := range []string{"llama.context_length", "transformer.context_length", "context_length"} {
		if v, ok := showResp.ModelInfo[key]; ok {
			if n, ok := v.(float64); ok && n > 0 {
				return int(n)
			}
		}
	}
	// Fallback: match any key ending with ".context_length" (architecture-agnostic)
	for k, v := range showResp.ModelInfo {
		if len(k) > 16 && k[len(k)-16:] == ".context_length" {
			if n, ok := v.(float64); ok && n > 0 {
				return int(n)
			}
		}
	}

	return 0
}
