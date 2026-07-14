package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	_ "github.com/deepteams/webp"
)

// AlibabaClient implements Client for Alibaba Cloud DashScope (MaaS) API.
//
// Two execution patterns are used for image-to-image editing, depending on the
// model family:
//
//  1. Synchronous (qwen family and non-i2i wan models) — models like
//     qwen-image-2.0-pro and wan2.7-image accept an image+prompt and return the
//     result synchronously via the multimodal generation endpoint.
//
//  2. Asynchronous (wan i2i family) — models like wan2.5-i2i-preview require
//     task submission, returning a task_id, then polling until the task completes.
type AlibabaClient struct {
	*apiClient
	// compatibleClient uses the OpenAI-compatible endpoint (rootURL/compatible-mode/v1)
	// for Chat and ListModels operations.
	compatibleClient   *apiClient
	APIKey             string
	Model              string
	Timeout            time.Duration
	MaxImageMegapixels float64
}

// NewAlibabaClient creates a new Alibaba Cloud DashScope LLM client.
// The baseURL should be the MaaS workspace root URL, e.g.
// https://ws-xxx.ap-southeast-1.maas.aliyuncs.com
//
// Two API base URLs are derived from the root:
//   - root URL: used for DashScope-native APIs (image editing, async tasks)
//   - root URL + /compatible-mode/v1: used for OpenAI-compatible APIs (chat, models)
func NewAlibabaClient(baseURL, apiKey, model string, maxImageMegapixels float64) *AlibabaClient {
	baseURL = strings.TrimRight(baseURL, "/")
	compatibleURL := baseURL + "/compatible-mode/v1"

	timeout := 5 * time.Minute
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	return &AlibabaClient{
		apiClient:          newAPIClient(baseURL, timeout, headers),
		compatibleClient:   newAPIClient(compatibleURL, timeout, headers),
		APIKey:             apiKey,
		Model:              model,
		Timeout:            timeout,
		MaxImageMegapixels: maxImageMegapixels,
	}
}

// ─── Model family detection ────────────────────────────────────────

// isAsyncEditModel reports whether the configured model requires asynchronous
// task submission and polling for image editing.
//
// Only wan i2i (image-to-image) variants use the async endpoint. Other wan
// models (e.g. wan2.7-image) and all qwen models use synchronous generation.
func (c *AlibabaClient) isAsyncEditModel() bool {
	lower := strings.ToLower(c.Model)
	return strings.Contains(lower, "wan") && strings.Contains(lower, "i2i")
}

// ─── Client interface: Recognize (image-to-image) ──────────────────

// alibabaMinMegapixels must be above pixel count required by the Alibaba
// DashScope multimodal generation API (≥ 0.6 MP).
const alibabaMinMegapixels = 1.0

// alibabaMaxMegapixels is the maximum pixel count accepted by the Alibaba
// DashScope multimodal generation API. Images above 4 MP are downsized before
// submission. This is independent of LLM_MAX_IMAGE_MEGAPIXELS.
const alibabaMaxMegapixels = 4.0

// Recognize performs image-to-image editing via the Alibaba DashScope API.
//
// Images are automatically resized to fit within [0.6 MP, 4.0 MP] before
// submission. Upscaled images keep their improved resolution — the result is
// NOT shrunk back to the original (low) dimensions. The result (always PNG
// from the API) is converted to the original file format.
//
// Returns the result as a data URL string (data:image/...;base64,...) that
// callers can extract and save.
func (c *AlibabaClient) Recognize(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
	// Step 1: Prepare the image — downsize to ≤ 4 MP, upscale to ≥ 0.6 MP.
	// orig* = dimensions before any resize, effective* = dimensions after.
	resizedData, origWidth, origHeight, effectiveWidth, effectiveHeight, origExt, err := prepareImageForEditing(imagePath, alibabaMaxMegapixels, alibabaMinMegapixels)
	if err != nil {
		return "", fmt.Errorf("Alibaba recognize: failed to prepare image: %w", err)
	}

	dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(resizedData)

	// Merge system and user prompts into a single editing instruction.
	prompt := strings.TrimSpace(systemPrompt + "\n\n" + userMessage)
	if prompt == "" {
		prompt = "Enhance image quality"
	}

	// Step 2: Call the appropriate API based on model family.
	var resultData []byte
	if c.isAsyncEditModel() {
		resultData, err = c.editImageAsync(ctx, dataURL, prompt)
	} else {
		resultData, err = c.editImageSync(ctx, dataURL, prompt)
	}
	if err != nil {
		return "", err
	}

	// Step 3: Post-process — choose target dimensions:
	// - Upscaled: keep effective dimensions (preserve improved resolution)
	// - Downsized or unchanged: restore original dimensions
	targetWidth, targetHeight := origWidth, origHeight
	if effectiveWidth > origWidth || effectiveHeight > origHeight {
		targetWidth, targetHeight = effectiveWidth, effectiveHeight
	}
	finalData, err := postProcessEditedImage(resultData, targetWidth, targetHeight, origExt)
	if err != nil {
		return "", fmt.Errorf("Alibaba recognize: failed to post-process result: %w", err)
	}

	// Return as data URL so callers can extract and save the image.
	resultDataURL := fmt.Sprintf("data:%s;base64,%s",
		mediaTypeByExt(origExt),
		base64.StdEncoding.EncodeToString(finalData))

	return resultDataURL, nil
}

// ─── Sync flow (qwen family) ───────────────────────────────────────

// generationRequest is the request body for the multimodal generation API
// (qwen family: qwen-image-2.0-pro, etc.).
type generationRequest struct {
	Model      string           `json:"model"`
	Input      generationInput  `json:"input"`
	Parameters generationParams `json:"parameters"`
}

type generationInput struct {
	Messages []generationMessage `json:"messages"`
}

type generationMessage struct {
	Role    string           `json:"role"`
	Content []generationPart `json:"content"`
}

type generationPart struct {
	Image string `json:"image,omitempty"`
	Text  string `json:"text,omitempty"`
}

type generationParams struct {
	N              int    `json:"n"`
	NegativePrompt string `json:"negative_prompt"`
	PromptExtend   bool   `json:"prompt_extend"`
	Watermark      bool   `json:"watermark"`
	Size           string `json:"size,omitempty"`
}

// generationResponse is the synchronous response from the multimodal generation API.
type generationResponse struct {
	Output    generationOutput `json:"output"`
	Usage     *generationUsage `json:"usage,omitempty"`
	RequestID string           `json:"request_id,omitempty"`
	Code      string           `json:"code,omitempty"`
	Message   string           `json:"message,omitempty"`
}

type generationOutput struct {
	Choices []generationChoice `json:"choices"`
}

type generationChoice struct {
	FinishReason string            `json:"finish_reason"`
	Message      generationRespMsg `json:"message"`
}

type generationRespMsg struct {
	Role    string               `json:"role"`
	Content []generationRespPart `json:"content"`
}

type generationRespPart struct {
	Image string `json:"image,omitempty"`
	Text  string `json:"text,omitempty"`
}

type generationUsage struct {
	Width      int `json:"width"`
	Height     int `json:"height"`
	ImageCount int `json:"image_count"`
}

// editImageSync calls the multimodal generation endpoint synchronously.
func (c *AlibabaClient) editImageSync(ctx context.Context, dataURL, prompt string) ([]byte, error) {
	// Determine output size from the resized input image.
	width, height, err := decodeImageDimensions(dataURL)
	if err != nil {
		return nil, fmt.Errorf("Alibaba sync: failed to decode input dimensions: %w", err)
	}

	req := generationRequest{
		Model: c.Model,
		Input: generationInput{
			Messages: []generationMessage{
				{
					Role: "user",
					Content: []generationPart{
						{Image: dataURL},
						{Text: prompt},
					},
				},
			},
		},
		Parameters: generationParams{
			N:              1,
			NegativePrompt: " ",
			PromptExtend:   true,
			Watermark:      false,
			Size:           fmt.Sprintf("%d*%d", width, height),
		},
	}

	var resp generationResponse
	path := "/api/v1/services/aigc/multimodal-generation/generation"
	if err := c.doJSON(ctx, http.MethodPost, path, req, &resp, nil); err != nil {
		return nil, fmt.Errorf("Alibaba sync generation: %w", err)
	}

	if resp.Code != "" && resp.Code != "0" {
		return nil, fmt.Errorf("Alibaba sync generation API error code %s: %s", resp.Code, resp.Message)
	}

	if len(resp.Output.Choices) == 0 {
		return nil, fmt.Errorf("no choices in Alibaba generation response")
	}

	content := resp.Output.Choices[0].Message.Content
	if len(content) == 0 {
		return nil, fmt.Errorf("no content in Alibaba generation response choice")
	}

	// Download the first result image.
	imageURL := content[0].Image
	if imageURL == "" {
		return nil, fmt.Errorf("no image URL in Alibaba generation response")
	}

	log.Printf("Alibaba sync: downloading result from %s", imageURL)
	resultData, _, err := downloadImage(imageURL)
	if err != nil {
		return nil, fmt.Errorf("Alibaba sync: failed to download result: %w", err)
	}

	return resultData, nil
}

// decodeImageDimensions extracts the pixel dimensions from a data URL image.
func decodeImageDimensions(dataURL string) (int, int, error) {
	if !strings.HasPrefix(dataURL, "data:") {
		return 0, 0, fmt.Errorf("not a data URL")
	}
	rest := dataURL[len("data:"):]
	semi := strings.IndexByte(rest, ';')
	if semi < 0 {
		return 0, 0, fmt.Errorf("invalid data URL: no MIME delimiter")
	}
	rest = rest[semi+1:]
	const b64Prefix = "base64,"
	if !strings.HasPrefix(rest, b64Prefix) {
		return 0, 0, fmt.Errorf("invalid data URL: no base64 prefix")
	}
	b64Data := rest[len(b64Prefix):]
	decoded, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid base64: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(decoded))
	if err != nil {
		return 0, 0, fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy(), nil
}

// ─── Async flow (wan family) ───────────────────────────────────────

// imageEditRequest is the request body for the asynchronous image2image API
// (wan family: wan2.5-i2i-preview, etc.).
type imageEditRequest struct {
	Model      string          `json:"model"`
	Input      imageEditInput  `json:"input"`
	Parameters imageEditParams `json:"parameters"`
}

type imageEditInput struct {
	Prompt string   `json:"prompt"`
	Images []string `json:"images"`
}

type imageEditParams struct {
	N            int  `json:"n"`
	PromptExtend bool `json:"prompt_extend"`
}

// asyncSubmitResponse is the response from submitting an async task.
type asyncSubmitResponse struct {
	Output    asyncSubmitOutput `json:"output"`
	RequestID string            `json:"request_id,omitempty"`
	Code      string            `json:"code,omitempty"`
	Message   string            `json:"message,omitempty"`
}

type asyncSubmitOutput struct {
	TaskID     string `json:"task_id"`
	TaskStatus string `json:"task_status"`
}

// taskStatusResponse is the response from polling GET /api/v1/tasks/{task_id}.
type taskStatusResponse struct {
	Output    taskStatusOutput `json:"output"`
	RequestID string           `json:"request_id,omitempty"`
	Usage     *taskUsage       `json:"usage,omitempty"`
}

type taskStatusOutput struct {
	TaskID        string       `json:"task_id"`
	TaskStatus    string       `json:"task_status"`
	SubmitTime    string       `json:"submit_time,omitempty"`
	ScheduledTime string       `json:"scheduled_time,omitempty"`
	EndTime       string       `json:"end_time,omitempty"`
	Results       []taskResult `json:"results,omitempty"`
	TaskMetrics   *taskMetrics `json:"task_metrics,omitempty"`
}

type taskResult struct {
	OrigPrompt   string `json:"orig_prompt"`
	ActualPrompt string `json:"actual_prompt"`
	URL          string `json:"url"`
}

type taskMetrics struct {
	Total     int `json:"TOTAL"`
	Failed    int `json:"FAILED"`
	Succeeded int `json:"SUCCEEDED"`
}

type taskUsage struct {
	ImageCount int `json:"image_count"`
}

// editImageAsync submits an async image2image task and polls until completion.
func (c *AlibabaClient) editImageAsync(ctx context.Context, dataURL, prompt string) ([]byte, error) {
	// Step 1: Submit the task.
	req := imageEditRequest{
		Model: c.Model,
		Input: imageEditInput{
			Prompt: prompt,
			Images: []string{dataURL},
		},
		Parameters: imageEditParams{
			N:            1,
			PromptExtend: true,
		},
	}

	var submitResp asyncSubmitResponse
	submitPath := "/api/v1/services/aigc/image2image/image-synthesis"
	extraHeaders := map[string]string{
		"X-DashScope-Async": "enable",
	}
	if err := c.doJSON(ctx, http.MethodPost, submitPath, req, &submitResp, extraHeaders); err != nil {
		return nil, fmt.Errorf("Alibaba async submit: %w", err)
	}

	if submitResp.Code != "" && submitResp.Code != "0" {
		return nil, fmt.Errorf("Alibaba async submit API error code %s: %s", submitResp.Code, submitResp.Message)
	}

	taskID := submitResp.Output.TaskID
	if taskID == "" {
		return nil, fmt.Errorf("no task_id in async submission response")
	}

	log.Printf("Alibaba async: task submitted id=%s status=%s", taskID, submitResp.Output.TaskStatus)

	// Step 2: Poll until completion.
	resultURL, err := c.pollAsyncTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Step 3: Download the result image.
	log.Printf("Alibaba async: downloading result from %s", resultURL)
	resultData, _, err := downloadImage(resultURL)
	if err != nil {
		return nil, fmt.Errorf("Alibaba async: failed to download result: %w", err)
	}

	return resultData, nil
}

// pollAsyncTask polls the task status endpoint until the task completes.
func (c *AlibabaClient) pollAsyncTask(ctx context.Context, taskID string) (string, error) {
	pollPath := fmt.Sprintf("/api/v1/tasks/%s", taskID)
	pollInterval := 5 * time.Second
	maxRetries := 120 // 10 minutes max

	// Use a short-lived HTTP client for polling (each call is fast).
	pollClient := newAPIClient(c.baseURL, 10*time.Second, c.headers)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("task %s polling cancelled: %w", taskID, ctx.Err())
		case <-time.After(pollInterval):
		}

		var taskResp taskStatusResponse
		if err := pollClient.doJSON(ctx, http.MethodGet, pollPath, nil, &taskResp, nil); err != nil {
			log.Printf("Alibaba async: poll attempt %d/%d failed: %v", attempt, maxRetries, err)
			continue
		}

		status := taskResp.Output.TaskStatus
		log.Printf("Alibaba async: task %s status=%s (attempt %d/%d)", taskID, status, attempt, maxRetries)

		switch status {
		case "SUCCEEDED":
			if len(taskResp.Output.Results) == 0 {
				return "", fmt.Errorf("task %s succeeded but no results returned", taskID)
			}
			url := taskResp.Output.Results[0].URL
			if url == "" {
				return "", fmt.Errorf("task %s succeeded but result has no URL", taskID)
			}
			return url, nil

		case "FAILED":
			msg := ""
			if len(taskResp.Output.Results) > 0 {
				msg = taskResp.Output.Results[0].OrigPrompt
			}
			return "", fmt.Errorf("task %s failed: %s", taskID, msg)

		case "CANCELED":
			return "", fmt.Errorf("task %s was canceled", taskID)

		case "PENDING", "RUNNING":
			// Continue polling.

		default:
			log.Printf("Alibaba async: unknown task status %q, continuing to poll", status)
		}

		// Exponential backoff for the polling interval.
		if pollInterval < 10*time.Second {
			pollInterval += time.Second
		}
	}

	return "", fmt.Errorf("task %s timed out after %d attempts", taskID, maxRetries)
}

// ─── Shared helpers ────────────────────────────────────────────────

// downloadImage downloads an image from an HTTP/HTTPS URL.
func downloadImage(url string) ([]byte, string, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read HTTP response body: %w", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" || strings.HasPrefix(mimeType, "application/octet-stream") {
		mimeType = guessMIMEType(url)
	}

	return data, mimeType, nil
}

// guessMIMEType returns a MIME type based on file extension in the URL.
func guessMIMEType(url string) string {
	lower := strings.ToLower(url)
	switch {
	case strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	default:
		return "image/png"
	}
}

// ─── Client interface: Chat ──────────────────────────────────────

// alibabaRequest is the request body for the DashScope OpenAI-compatible chat API.
type alibabaRequest struct {
	Model    string               `json:"model"`
	Messages []alibabaChatMessage `json:"messages"`
}

type alibabaChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []alibabaContentPart for multimodal
}

type alibabaContentPart struct {
	Type     string           `json:"type"` // "text" or "image_url"
	Text     string           `json:"text,omitempty"`
	ImageURL *alibabaImageURL `json:"image_url,omitempty"`
}

type alibabaImageURL struct {
	URL string `json:"url"`
}

// alibabaResponse handles chat completions response format.
type alibabaResponse struct {
	Choices []alibabaChoice `json:"choices,omitempty"`
	Usage   *alibabaUsage   `json:"usage,omitempty"`
}

type alibabaChoice struct {
	Message      alibabaChoiceMessage `json:"message"`
	FinishReason string               `json:"finish_reason"`
}

type alibabaChoiceMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []any for multimodal
}

type alibabaUsage struct {
	TotalTokens  int `json:"total_tokens"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Chat performs a conversational LLM call via the DashScope OpenAI-compatible API.
func (c *AlibabaClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	messages := make([]alibabaChatMessage, len(req.Messages))
	for i, m := range req.Messages {
		msg := alibabaChatMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		messages[i] = msg
	}

	body := alibabaRequest{
		Model:    c.Model,
		Messages: messages,
	}

	var resp alibabaResponse
	if err := c.compatibleClient.doJSON(ctx, http.MethodPost, "/chat/completions", body, &resp, nil); err != nil {
		return nil, fmt.Errorf("Alibaba chat: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in Alibaba chat response")
	}

	choice := resp.Choices[0]
	chatResp := &ChatResponse{
		Message: ChatMessage{
			Role:    choice.Message.Role,
			Content: alibabaExtractContentText(choice.Message.Content),
		},
	}

	if choice.FinishReason == "tool_calls" {
		chatResp.StopReason = "tool_use"
	} else {
		chatResp.StopReason = "end_turn"
	}

	if resp.Usage != nil {
		chatResp.Usage = &ChatUsage{
			TotalTokens:      resp.Usage.TotalTokens,
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
		}
	}

	return chatResp, nil
}

// alibabaExtractContentText extracts the plain-text portion from a message
// content field, which may be a string or a multimodal content array.
func alibabaExtractContentText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, _ := m["type"].(string); t == "text" {
					if txt, _ := m["text"].(string); txt != "" {
						parts = append(parts, txt)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ─── Client interface: ListModels ─────────────────────────────────

// alibabaModelsResponse handles model listing response.
type alibabaModelsResponse struct {
	Data   []alibabaModel `json:"data"`
	Output struct {
		Models []alibabaModel `json:"models"`
	} `json:"output,omitempty"`
}

type alibabaModel struct {
	ID string `json:"id"`
}

// ListModels returns available models from the Alibaba Cloud OpenAI-compatible API.
func (c *AlibabaClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	shortClient := newAPIClient(c.compatibleClient.baseURL, 30*time.Second, c.compatibleClient.headers)

	var modelsResp alibabaModelsResponse
	if err := shortClient.doJSON(ctx, http.MethodGet, "/models", nil, &modelsResp, nil); err != nil {
		return nil, fmt.Errorf("Alibaba list models: %w", err)
	}

	var models []ModelInfo
	for _, m := range modelsResp.Data {
		models = append(models, ModelInfo{
			ID:   m.ID,
			Name: m.ID,
		})
	}
	if len(models) == 0 {
		for _, m := range modelsResp.Output.Models {
			models = append(models, ModelInfo{
				ID:   m.ID,
				Name: m.ID,
			})
		}
	}

	return models, nil
}

// Ensure AlibabaClient implements both Client and ChatClient.
var _ Client = (*AlibabaClient)(nil)
var _ ChatClient = (*AlibabaClient)(nil)
