package dto

import (
	"encoding/json"
	"testing"
)

// TestLlmModelDTOMetadataFields verifies that the model DTO serializes the
// extended metadata fields with their camelCase API keys, so the frontend
// contract exposes modalities and per-1M-token pricing alongside the existing
// context length and capabilities.
func TestLlmModelDTOMetadataFields(t *testing.T) {
	m := LlmModelDTO{
		ID:                 "deepseek-v4-pro",
		Name:               "DeepSeek-V4-Pro",
		Size:               123,
		ContextLength:      1048576,
		Capabilities:       []string{"chat", "tool_calling"},
		Modalities:         []string{"text"},
		InputPricePerMTok:  0.5,
		OutputPricePerMTok: 1.5,
	}

	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	for _, key := range []string{"modalities", "inputPricePerMTok", "outputPricePerMTok", "contextLength", "capabilities"} {
		if _, ok := got[key]; !ok {
			t.Errorf("expected JSON key %q to be present in LlmModelDTO output, got %s", key, string(b))
		}
	}
}
