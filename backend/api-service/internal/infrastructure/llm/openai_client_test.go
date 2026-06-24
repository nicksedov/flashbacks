package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeOpenAIBaseURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain host without v1",
			input: "https://api.openai.com",
			want:  "https://api.openai.com",
		},
		{
			name:  "host with /v1 suffix",
			input: "https://ws-2dc7yd9a748q8pau.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1",
			want:  "https://ws-2dc7yd9a748q8pau.ap-southeast-1.maas.aliyuncs.com/compatible-mode",
		},
		{
			name:  "host with /v1/ trailing slash",
			input: "https://example.com/compatible-mode/v1/",
			want:  "https://example.com/compatible-mode",
		},
		{
			name:  "host with trailing slash only",
			input: "https://api.openai.com/",
			want:  "https://api.openai.com",
		},
		{
			name:  "host with multiple trailing slashes",
			input: "https://api.openai.com///",
			want:  "https://api.openai.com",
		},
		{
			name:  "path segment containing v1 but not /v1",
			input: "https://example.com/apiv1",
			want:  "https://example.com/apiv1",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeOpenAIBaseURL(tt.input)
			if got != tt.want {
				t.Errorf("normalizeOpenAIBaseURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewOpenAIClient_NormalizesBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		wantBaseURL string
	}{
		{
			name:        "plain host preserved",
			baseURL:     "https://api.openai.com",
			wantBaseURL: "https://api.openai.com",
		},
		{
			name:        "/v1 suffix stripped",
			baseURL:     "https://example.com/compatible-mode/v1",
			wantBaseURL: "https://example.com/compatible-mode",
		},
		{
			name:        "/v1/ trailing slash stripped",
			baseURL:     "https://example.com/compatible-mode/v1/",
			wantBaseURL: "https://example.com/compatible-mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewOpenAIClient(tt.baseURL, "key", "model", 3.6)
			if client.apiClient.baseURL != tt.wantBaseURL {
				t.Errorf("baseURL = %q, want %q", client.apiClient.baseURL, tt.wantBaseURL)
			}
		})
	}
}

func TestOpenAIClientListModels_WithV1BaseURL(t *testing.T) {
	// This test simulates the exact bug: a base URL that already includes /v1
	// The test server checks that /v1 is NOT doubled
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		if receivedPath == "/v1/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := openAIModelsResponse{
			Data: []openAIModel{{ID: "qwen3.7-plus", Object: "model", Created: 1234567890}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Simulate: user provides base URL = server.URL + "/v1"
	baseURLWithV1 := server.URL + "/v1"
	client := NewOpenAIClient(baseURLWithV1, "test-key", "qwen3.7-plus", 3.6)

	models, err := client.ListModels()
	if err != nil {
		t.Fatalf("ListModels() should succeed, got error: %v", err)
	}

	if receivedPath != "/v1/models" {
		t.Errorf("expected path /v1/models, got %s (double /v1 bug not fixed)", receivedPath)
	}

	if len(models) != 1 || models[0].ID != "qwen3.7-plus" {
		t.Errorf("unexpected models: %+v", models)
	}
}
