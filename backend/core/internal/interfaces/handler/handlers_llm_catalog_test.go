package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	catalog "github.com/flashbacks/core/internal/application/llm"
	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	"github.com/flashbacks/core/internal/infrastructure/llm"
	"github.com/flashbacks/core/internal/interfaces/dto"
	"github.com/flashbacks/core/internal/interfaces/handler/helpers"
	"github.com/flashbacks/core/internal/testutil"
)

// stubCatalogLister is a controllable model lister for handler tests.
type stubCatalogLister struct {
	models []llm.ModelInfo
	err    error
}

func (s *stubCatalogLister) ListModels(context.Context, domain.LlmProvider, string) ([]llm.ModelInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.models, nil
}

// newCatalogHandlerServer builds a Server whose models endpoint is backed by the
// real catalog service over an in-memory database.
func newCatalogHandlerServer(t *testing.T, lister catalog.ModelLister) (*Server, repository.LlmCatalogRepository, func()) {
	t.Helper()

	db, cleanup := testutil.NewTestDB(t)
	repo := repository.NewLlmCatalogRepository(db)
	svc := &Server{
		db:             db,
		settingsLoader: helpers.NewSettingsLoader(db),
		llmRepo:        repository.NewLlmRepository(db),
		llmCatalog:     catalog.NewService(repo, lister, catalog.Config{TTL: time.Hour}),
	}
	return svc, repo, cleanup
}

// decodeModelsResponse decodes the models endpoint payload.
func decodeModelsResponse(t *testing.T, body []byte) dto.LlmModelsResponse {
	t.Helper()
	var response dto.LlmModelsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode models response: %v", err)
	}
	return response
}

// TestHandleGetLlmModels_ServesStoredCatalog guards
// llm-model-catalog.response-equals-stored and
// llm-model-catalog.response-exposes-freshness at the HTTP boundary.
func TestHandleGetLlmModels_ServesStoredCatalog(t *testing.T) {
	lister := &stubCatalogLister{models: []llm.ModelInfo{{
		ID:               "qwen3.5-flash",
		Name:             "Qwen3.5-Flash",
		ContextLength:    1000000,
		Capabilities:     []string{"chat", "tool_calling", "vision"},
		InputModalities:  []string{"text", "image"},
		OutputModalities: []string{"text"},
	}}}
	svc, repo, cleanup := newCatalogHandlerServer(t, lister)
	defer cleanup()

	w := serveHandler(svc.handleGetLlmModels, http.MethodGet,
		"/api/llm/models?provider=ollama_test&refresh=true", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	response := decodeModelsResponse(t, w.Body.Bytes())
	if !response.Success {
		t.Fatalf("expected success, got error %q", response.Error)
	}
	if response.FetchedAt == "" {
		t.Error("expected a fetchedAt timestamp in the response")
	}
	if response.Stale {
		t.Error("a successful refresh must not be reported as stale")
	}
	if len(response.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(response.Models))
	}
	if response.Models[0].ContextLength != 1000000 {
		t.Errorf("context length = %d, want the stored 1000000", response.Models[0].ContextLength)
	}

	// The served metadata must equal the persisted catalog.
	provider, err := repo.GetProviderByAlias("ollama_test")
	if err != nil {
		t.Fatalf("load provider: %v", err)
	}
	stored, err := repo.GetProviderCatalog(provider.ID)
	if err != nil {
		t.Fatalf("load provider catalog: %v", err)
	}
	if len(stored.Models) != len(response.Models) {
		t.Fatalf("served %d models, stored %d", len(response.Models), len(stored.Models))
	}
	if stored.Models[0].ContextLength != response.Models[0].ContextLength {
		t.Errorf("stored context length = %d, served %d",
			stored.Models[0].ContextLength, response.Models[0].ContextLength)
	}
	if len(stored.Models[0].Capabilities) != len(response.Models[0].Capabilities) {
		t.Errorf("stored capabilities = %v, served %v",
			stored.Models[0].Capabilities, response.Models[0].Capabilities)
	}

	// A non-forcing read must answer from the stored catalog.
	w = serveHandler(svc.handleGetLlmModels, http.MethodGet,
		"/api/llm/models?provider=ollama_test", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on the stored read, got %d: %s", w.Code, w.Body.String())
	}
	storedResponse := decodeModelsResponse(t, w.Body.Bytes())
	if len(storedResponse.Models) != len(response.Models) {
		t.Fatalf("stored read served %d models, refresh served %d",
			len(storedResponse.Models), len(response.Models))
	}
}

// TestHandleGetLlmModels_StaleOnFailedRefresh guards
// llm-model-catalog.write-failure-reported at the HTTP boundary.
func TestHandleGetLlmModels_StaleOnFailedRefresh(t *testing.T) {
	lister := &stubCatalogLister{models: []llm.ModelInfo{{ID: "acme-chat-7b"}}}
	svc, repo, cleanup := newCatalogHandlerServer(t, lister)
	defer cleanup()

	provider, err := repo.GetProviderByAlias("ollama_test")
	if err != nil {
		t.Fatalf("load provider: %v", err)
	}
	if err := repo.MergeProviderCatalog(provider.ID, []domain.LlmProviderModel{{
		ModelID:   "stored-model",
		ModelName: "Stored Model",
	}}, time.Now().UTC(), true); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	lister.err = errors.New("provider unreachable")
	w := serveHandler(svc.handleGetLlmModels, http.MethodGet,
		"/api/llm/models?provider=ollama_test&refresh=true", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 while a catalog is stored, got %d: %s", w.Code, w.Body.String())
	}

	response := decodeModelsResponse(t, w.Body.Bytes())
	if !response.Success {
		t.Fatalf("expected the stored catalog to be served, got error %q", response.Error)
	}
	if !response.Stale {
		t.Error("expected stale: true after a failed refresh")
	}
	if response.RefreshError == "" {
		t.Error("expected refreshError to carry the failure text")
	}
	if len(response.Models) != 1 || response.Models[0].ID != "stored-model" {
		t.Fatalf("served models = %+v, want the previously stored catalog", response.Models)
	}
}

// TestHandleGetLlmModels_RefreshFailureWithoutCatalog guards task 4.2: a failed
// refresh with no stored catalog answers with a failure, not an empty success.
func TestHandleGetLlmModels_RefreshFailureWithoutCatalog(t *testing.T) {
	lister := &stubCatalogLister{err: errors.New("provider unreachable")}
	svc, _, cleanup := newCatalogHandlerServer(t, lister)
	defer cleanup()

	w := serveHandler(svc.handleGetLlmModels, http.MethodGet,
		"/api/llm/models?provider=ollama_test&refresh=true", nil)

	response := decodeModelsResponse(t, w.Body.Bytes())
	if response.Success {
		t.Fatal("expected success: false when the refresh fails and no catalog exists")
	}
	if response.Error == "" {
		t.Error("expected an error message in the response")
	}
}
