package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewDeepSeekClient_NormalizesBaseURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://api.deepseek.com", "https://api.deepseek.com"},
		{"https://api.deepseek.com/v1", "https://api.deepseek.com"},
		{"https://api.deepseek.com/v1/", "https://api.deepseek.com"},
		{"https://api.deepseek.com/", "https://api.deepseek.com"},
		{"https://custom.deepseek.com/path", "https://custom.deepseek.com/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			c := NewDeepSeekClient(tt.input, "test-key", "deepseek-chat")
			if c.baseURL != tt.expected {
				t.Errorf("expected baseURL %q, got %q", tt.expected, c.baseURL)
			}
		})
	}
}

func TestDeepSeekContextWindow_ExactMatch(t *testing.T) {
	c := NewDeepSeekClient("https://api.deepseek.com", "test-key", "deepseek-chat")
	if w := c.ContextWindow(); w != 128000 {
		t.Errorf("expected 128000, got %d", w)
	}

	c2 := NewDeepSeekClient("https://api.deepseek.com", "test-key", "deepseek-v4-pro")
	if w := c2.ContextWindow(); w != 1000000 {
		t.Errorf("expected 1000000, got %d", w)
	}
}

func TestDeepSeekContextWindow_PrefixMatch(t *testing.T) {
	c := NewDeepSeekClient("https://api.deepseek.com", "test-key", "deepseek-chat-v2")
	if w := c.ContextWindow(); w != 128000 {
		t.Errorf("expected 128000 (prefix match), got %d", w)
	}
}

func TestDeepSeekContextWindow_UnknownModel(t *testing.T) {
	c := NewDeepSeekClient("https://api.deepseek.com", "test-key", "unknown-model")
	if w := c.ContextWindow(); w != 128000 {
		t.Errorf("expected 128000 (fallback), got %d", w)
	}
}

func TestDeepSeekContextWindow_AllKnownModels(t *testing.T) {
	for modelID, info := range knownDeepSeekModels {
		c := NewDeepSeekClient("https://api.deepseek.com", "test-key", modelID)
		if w := c.ContextWindow(); w != info.ContextWindow {
			t.Errorf("model %q: expected context window %d, got %d", modelID, info.ContextWindow, w)
		}
	}
}

func TestDeepSeekListModels_AttachesKnownContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": [
				{"id": "deepseek-chat", "object": "model", "created": 1700000000},
				{"id": "deepseek-v4-pro", "object": "model", "created": 1700000001},
				{"id": "custom-model", "object": "model", "created": 1700000002}
			]
		}`))
	}))
	defer server.Close()

	c := NewDeepSeekClient(server.URL, "test-key", "deepseek-chat")
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}

	// Known model should have context length
	if models[0].ID != "deepseek-chat" || models[0].ContextLength != 128000 {
		t.Errorf("expected deepseek-chat context 128000, got %d", models[0].ContextLength)
	}
	if models[1].ID != "deepseek-v4-pro" || models[1].ContextLength != 1000000 {
		t.Errorf("expected deepseek-v4-pro context 1000000, got %d", models[1].ContextLength)
	}
	// Unknown model should have 0 context length
	if models[2].ContextLength != 0 {
		t.Errorf("expected custom-model context 0, got %d", models[2].ContextLength)
	}
}

func TestDeepSeekChat_ParsesUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "chat_123",
			"object": "chat.completion",
			"created": 1700000000,
			"model": "deepseek-chat",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Hello! How can I help you?"
					},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 25,
				"completion_tokens": 10,
				"prompt_cache_hit_tokens": 15,
				"prompt_cache_miss_tokens": 10,
				"total_tokens": 35,
				"completion_tokens_details": {
					"reasoning_tokens": 3
				}
			}
		}`))
	}))
	defer server.Close()

	c := NewDeepSeekClient(server.URL, "test-key", "deepseek-chat")
	resp, err := c.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "Hi"},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Message.Content != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %q", resp.Message.Content)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("expected end_turn, got %q", resp.StopReason)
	}

	// Check extended usage
	if resp.Usage == nil {
		t.Fatal("expected usage to be parsed")
	}
	if resp.Usage.PromptTokens != 25 {
		t.Errorf("expected PromptTokens=25, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 10 {
		t.Errorf("expected CompletionTokens=10, got %d", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 35 {
		t.Errorf("expected TotalTokens=35, got %d", resp.Usage.TotalTokens)
	}
	if resp.Usage.PromptCacheHitTokens != 15 {
		t.Errorf("expected PromptCacheHitTokens=15, got %d", resp.Usage.PromptCacheHitTokens)
	}
	if resp.Usage.PromptCacheMissTokens != 10 {
		t.Errorf("expected PromptCacheMissTokens=10, got %d", resp.Usage.PromptCacheMissTokens)
	}
	if resp.Usage.ReasoningTokens != 3 {
		t.Errorf("expected ReasoningTokens=3, got %d", resp.Usage.ReasoningTokens)
	}
}

func TestDeepSeekChat_ParsesToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "chat_456",
			"object": "chat.completion",
			"created": 1700000000,
			"model": "deepseek-chat",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "",
						"tool_calls": [
							{
								"id": "call_abc",
								"type": "function",
								"function": {
									"name": "get_weather",
									"arguments": "{\"location\":\"Paris\"}"
								}
							}
						]
					},
					"finish_reason": "tool_calls"
				}
			],
			"usage": {
				"prompt_tokens": 30,
				"completion_tokens": 15,
				"total_tokens": 45
			}
		}`))
	}))
	defer server.Close()

	c := NewDeepSeekClient(server.URL, "test-key", "deepseek-chat")
	resp, err := c.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "What's the weather in Paris?"},
		},
		Tools: []ToolDefinition{
			{Name: "get_weather", Description: "Get weather for a location"},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.StopReason != "tool_use" {
		t.Errorf("expected tool_use, got %q", resp.StopReason)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Name != "get_weather" {
		t.Errorf("expected get_weather, got %q", resp.Message.ToolCalls[0].Name)
	}
	if resp.Usage == nil {
		t.Fatal("expected usage to be parsed")
	}
	if resp.Usage.PromptTokens != 30 {
		t.Errorf("expected PromptTokens=30, got %d", resp.Usage.PromptTokens)
	}
}

func TestDeepSeekChat_NoUsageInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "chat_789",
			"object": "chat.completion",
			"created": 1700000000,
			"model": "deepseek-chat",
			"choices": [
				{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "No usage data"
					},
					"finish_reason": "stop"
				}
			]
		}`))
	}))
	defer server.Close()

	c := NewDeepSeekClient(server.URL, "test-key", "deepseek-chat")
	resp, err := c.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "No usage"},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if resp.Usage != nil {
		t.Error("expected nil usage when not provided in response")
	}
}
