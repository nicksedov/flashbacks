package repository

import (
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"
)

// TestGetModelsByProviderID_PreloadsModalities guards the frontend contract:
// the model-details dialog renders `modalities`, which is only populated when
// the repository preloads the association. Without Preload("Modalities") the
// field is silently dropped for cached models (e.g. Alibaba Qwen), so the UI
// shows no modality list even though the provider API returned one.
func TestGetModelsByProviderID_PreloadsModalities(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	repo := NewLlmRepository(db)

	provider := domain.LlmProvider{
		Name:   "alibaba",
		Alias:  "alibaba_test",
		ApiUrl: "https://dashscope.aliyuncs.com",
	}
	if err := repo.CreateProvider(&provider); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	models := []domain.LlmProviderModel{
		{
			ModelID:   "qwen3.8-max",
			ModelName: "Qwen3.8-Max",
			Capabilities: []domain.LlmModelCapability{
				{Capability: "chat"},
				{Capability: "vision"},
			},
			Modalities: []domain.LlmModelModality{
				{Modality: "Text"},
				{Modality: "Image"},
				{Modality: "Video"},
			},
		},
	}
	if err := repo.ReplaceProviderModels(provider.ID, models); err != nil {
		t.Fatalf("replace provider models: %v", err)
	}

	got, err := repo.GetModelsByProviderID(provider.ID)
	if err != nil {
		t.Fatalf("get models by provider id: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 model, got %d", len(got))
	}

	modalities := make([]string, len(got[0].Modalities))
	for i, m := range got[0].Modalities {
		modalities[i] = m.Modality
	}
	for _, want := range []string{"Text", "Image", "Video"} {
		found := false
		for _, m := range modalities {
			if m == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected modality %q to be preloaded, got %v", want, modalities)
		}
	}

	if len(got[0].Capabilities) == 0 {
		t.Error("expected capabilities to remain preloaded")
	}
}
