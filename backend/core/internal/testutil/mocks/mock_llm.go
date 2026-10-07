package mocks

import (
	"context"

	"github.com/flashbacks/api-service/internal/infrastructure/llm"
)

// MockLlmClient is a mock implementation of llm.Client for testing.
type MockLlmClient struct {
	RecognizeFunc  func(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error)
	ListModelsFunc func(ctx context.Context) ([]llm.ModelInfo, error)

	// Counters
	RecognizeCallCount  int
	ListModelsCallCount int
}

// Recognize implements llm.Client.
func (m *MockLlmClient) Recognize(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
	m.RecognizeCallCount++
	if m.RecognizeFunc != nil {
		return m.RecognizeFunc(ctx, imagePath, systemPrompt, userMessage)
	}
	return "", nil
}

// ListModels implements llm.Client.
func (m *MockLlmClient) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	m.ListModelsCallCount++
	if m.ListModelsFunc != nil {
		return m.ListModelsFunc(ctx)
	}
	return []llm.ModelInfo{
		{ID: "minicpm-v", Name: "MiniCPM-V"},
	}, nil
}

// TextResponse returns a function that simulates LLM returning a text description.
func TextResponse(text string) func(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
	return func(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
		return text, nil
	}
}

// TagsResponse returns a function that simulates LLM returning comma-separated tags.
func TagsResponse(tags []string) func(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
	tagStr := ""
	for i, tag := range tags {
		if i > 0 {
			tagStr += ", "
		}
		tagStr += tag
	}
	return func(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
		return tagStr, nil
	}
}

// LlmErrorResponse returns a function that simulates an LLM error.
func LlmErrorResponse(err error) func(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
	return func(ctx context.Context, imagePath string, systemPrompt string, userMessage string) (string, error) {
		return "", err
	}
}
