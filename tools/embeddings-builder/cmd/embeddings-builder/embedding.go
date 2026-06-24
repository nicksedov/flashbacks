// Package main — embedding.go
//
// Standalone embedding client for the embeddings-builder utility.
// Adapted from api-service/internal/infrastructure/llm/ — intentionally duplicated
// to keep this module free of the main api-service's dependencies.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider type constants (from api-service llm/client.go).
const (
	ProviderOllama      = "ollama"
	ProviderOllamaCloud = "ollama_cloud"
	ProviderOpenAI      = "openai"
)

// ---------------------------------------------------------------------------
// EmbeddingClient interface
// ---------------------------------------------------------------------------

// EmbeddingClient generates vector embeddings for text.
type EmbeddingClient interface {
	Embed(texts []string) ([][]float32, error)
}

// ---------------------------------------------------------------------------
// Ollama embedding client
// ---------------------------------------------------------------------------

// ollamaEmbedClient is a minimal Ollama client for embedding only.
type ollamaEmbedClient struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

func newOllamaEmbedClient(baseURL, apiKey, model string) *ollamaEmbedClient {
	return &ollamaEmbedClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		Timeout: 5 * time.Minute,
	}
}

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func (c *ollamaEmbedClient) Embed(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	body, err := json.Marshal(ollamaEmbedRequest{Model: c.Model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	client := &http.Client{Timeout: c.Timeout}
	req, err := http.NewRequest("POST", c.BaseURL+"/api/embed", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama embed API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var embedResp ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embed response: %w", err)
	}
	if len(embedResp.Embeddings) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(texts), len(embedResp.Embeddings))
	}
	return embedResp.Embeddings, nil
}

// ---------------------------------------------------------------------------
// OpenAI embedding client
// ---------------------------------------------------------------------------

// openAIEmbedClient is a minimal OpenAI-compatible client for embedding only.
type openAIEmbedClient struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

func newOpenAIEmbedClient(baseURL, apiKey, model string) *openAIEmbedClient {
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return &openAIEmbedClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		Timeout: 5 * time.Minute,
	}
}

type openAIEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (c *openAIEmbedClient) Embed(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	body, err := json.Marshal(openAIEmbedRequest{Model: c.Model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	client := &http.Client{Timeout: c.Timeout}
	req, err := http.NewRequest("POST", c.BaseURL+"/v1/embeddings", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI embed API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var embedResp openAIEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embed response: %w", err)
	}
	if len(embedResp.Data) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(texts), len(embedResp.Data))
	}

	embeddings := make([][]float32, len(embedResp.Data))
	for i, d := range embedResp.Data {
		embeddings[i] = d.Embedding
	}
	return embeddings, nil
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

// newEmbeddingClient creates an embedding client based on provider type.
func newEmbeddingClient(provider, baseURL, apiKey, model string) (EmbeddingClient, error) {
	switch provider {
	case ProviderOllama:
		return newOllamaEmbedClient(baseURL, "", model), nil
	case ProviderOllamaCloud:
		if apiKey == "" {
			return nil, fmt.Errorf("API key is required for Ollama Cloud provider")
		}
		return newOllamaEmbedClient(baseURL, apiKey, model), nil
	case ProviderOpenAI:
		if apiKey == "" {
			return nil, fmt.Errorf("API key is required for OpenAI provider")
		}
		return newOpenAIEmbedClient(baseURL, apiKey, model), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", provider)
	}
}

// ---------------------------------------------------------------------------
// Vector helper
// ---------------------------------------------------------------------------

// float32SliceToPgVector converts a float32 slice to pgvector format string.
func float32SliceToPgVector(vec []float32) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
