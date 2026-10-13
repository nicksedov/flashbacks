package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaCapabilitiesNormalized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"capabilities": ["completion", "thinking", "tools"],
			"model_info": {
				"general.architecture": "gemma4",
				"gemma4.context_length": 262144
			}
		}`))
	}))
	defer server.Close()

	c := NewOllamaClient(server.URL, "", "gemma4:31b", 3.6)
	_, caps, _ := c.fetchModelDetails(context.Background(), "gemma4:31b")

	if !containsFold(caps, "chat") {
		t.Errorf("expected normalized capability %q in %v", "chat", caps)
	}
	if !containsFold(caps, "tool_calling") {
		t.Errorf("expected normalized capability %q in %v", "tool_calling", caps)
	}
	if containsFold(caps, "completion") || containsFold(caps, "thinking") {
		t.Errorf("expected raw names to be dropped, got %v", caps)
	}
}

func TestOllamaVisionModality(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"capabilities": ["completion", "tools", "vision"],
			"model_info": {
				"general.architecture": "gemma4",
				"gemma4.context_length": 262144
			}
		}`))
	}))
	defer server.Close()

	c := NewOllamaClient(server.URL, "", "gemma4:31b", 3.6)
	_, caps, mods := c.fetchModelDetails(context.Background(), "gemma4:31b")

	if !containsFold(caps, "vision") {
		t.Errorf("expected vision capability, got %v", caps)
	}
	if !containsFold(mods, "image") {
		t.Errorf("expected image input modality inferred from vision, got %v", mods)
	}
}

// A model that reports completion/vision/tools/thinking (e.g. gemma4) must not
// receive an embedding capability, so it is never listed under the embedding
// instrument even though its model_info carries an embedding_length field.
func TestOllamaChatVisionModelIsNotEmbeddingCapable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"capabilities": ["completion", "vision", "tools", "thinking"],
			"model_info": {
				"general.architecture": "gemma4",
				"gemma4.context_length": 262144,
				"gemma4.embedding_length": 5376
			}
		}`))
	}))
	defer server.Close()

	c := NewOllamaClient(server.URL, "", "gemma4:31b", 3.6)
	_, caps, _ := c.fetchModelDetails(context.Background(), "gemma4:31b")

	if containsFold(caps, "embedding") {
		t.Errorf("chat/vision model must not be embedding capable, got %v", caps)
	}
	if !containsFold(caps, "chat") {
		t.Errorf("expected chat capability, got %v", caps)
	}
	if !containsFold(caps, "vision") {
		t.Errorf("expected vision capability, got %v", caps)
	}
}
