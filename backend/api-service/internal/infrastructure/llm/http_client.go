package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// apiClient provides a reusable HTTP transport layer for LLM API calls.
// It eliminates duplicated request-building, header-setting, and error-handling
// boilerplate across OllamaClient and OpenAIClient.
type apiClient struct {
	baseURL    string
	httpClient *http.Client
	headers    map[string]string
}

// newAPIClient creates a shared API client with the given base URL, timeout, and default headers.
func newAPIClient(baseURL string, timeout time.Duration, headers map[string]string) *apiClient {
	return &apiClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
		headers:    headers,
	}
}

// doJSON sends an HTTP request with a JSON body and decodes the JSON response into resp.
// If body is nil, no request body is sent (useful for GET requests).
// Returns a descriptive error on HTTP 4xx/5xx responses, including the first 4KB of the body.
func (c *apiClient) doJSON(ctx context.Context, method, path string, body, resp interface{}, extraHeaders map[string]string) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Apply default headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	// Apply per-call extra headers (overrides defaults)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(httpResp.Body, 4096))
		return fmt.Errorf("API error %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	if resp != nil {
		if err := json.NewDecoder(httpResp.Body).Decode(resp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
