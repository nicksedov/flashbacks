package repository

import (
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"
)

// TestGetModelsByProviderID_PreloadsModalities guards the frontend contract:
// the model-details dialog renders input and output modalities separately, which
// are only populated when the repository preloads the association and each row
// retains its direction. Without Preload("Modalities") the field is silently
// dropped for cached models (e.g. Alibaba Qwen), so the UI shows no modality list
// even though the provider API returned one.
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
				{Modality: "Text", Direction: domain.ModalityDirectionInput},
				{Modality: "Image", Direction: domain.ModalityDirectionInput},
				{Modality: "Video", Direction: domain.ModalityDirectionInput},
				{Modality: "Text", Direction: domain.ModalityDirectionOutput},
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

	inputMods := map[string]bool{}
	outputMods := map[string]bool{}
	for _, m := range got[0].Modalities {
		switch m.Direction {
		case domain.ModalityDirectionInput:
			inputMods[m.Modality] = true
		case domain.ModalityDirectionOutput:
			outputMods[m.Modality] = true
		default:
			t.Errorf("unexpected modality direction %q for %q", m.Direction, m.Modality)
		}
	}
	for _, want := range []string{"Text", "Image", "Video"} {
		if !inputMods[want] {
			t.Errorf("expected input modality %q to be preloaded, got %v", want, inputMods)
		}
	}
	if !outputMods["Text"] {
		t.Errorf("expected output modality %q to be preloaded, got %v", "Text", outputMods)
	}

	if len(got[0].Capabilities) == 0 {
		t.Error("expected capabilities to remain preloaded")
	}
}
