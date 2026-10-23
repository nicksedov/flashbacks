package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	modelllm "github.com/flashbacks/core/internal/infrastructure/llm"
	"github.com/flashbacks/core/internal/testutil"
)

const (
	testProviderAlias = "ollama_test"
	testTTL           = time.Hour
)

// testClock is a controllable clock for TTL tests.
type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time { return c.now }

func (c *testClock) advance(d time.Duration) { c.now = c.now.Add(d) }

// fakeLister returns a canned model list and counts how often it was asked.
type fakeLister struct {
	models []modelllm.ModelInfo
	err    error
	calls  int
}

func (f *fakeLister) ListModels(context.Context, domain.LlmProvider, string) ([]modelllm.ModelInfo, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.models, nil
}

// failingCatalogRepo fails every catalog merge while delegating reads.
type failingCatalogRepo struct {
	repository.LlmCatalogRepository
	err error
}

func (r *failingCatalogRepo) MergeProviderCatalog(uint, []domain.LlmProviderModel, time.Time, bool) error {
	return r.err
}

// catalogTestEnv bundles a service under test with its controllable pieces.
type catalogTestEnv struct {
	service *Service
	repo    repository.LlmCatalogRepository
	lister  *fakeLister
	clock   *testClock
	cleanup func()
}

// newCatalogTestEnv builds a catalog service on an in-memory SQLite database.
func newCatalogTestEnv(t *testing.T, models []modelllm.ModelInfo, wrapRepo func(repository.LlmCatalogRepository) repository.LlmCatalogRepository) *catalogTestEnv {
	t.Helper()

	db, cleanup := testutil.NewTestDB(t)
	repo := repository.NewLlmCatalogRepository(db)

	clock := &testClock{now: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}
	lister := &fakeLister{models: models}

	var store repository.LlmCatalogRepository = repo
	if wrapRepo != nil {
		store = wrapRepo(repo)
	}

	service := NewService(store, lister, Config{TTL: testTTL, Now: clock.Now})

	return &catalogTestEnv{
		service: service,
		repo:    repo,
		lister:  lister,
		clock:   clock,
		cleanup: cleanup,
	}
}

// provider returns the seeded provider row used by the tests.
func (e *catalogTestEnv) provider(t *testing.T) domain.LlmProvider {
	t.Helper()
	provider, err := e.repo.GetProviderByAlias(testProviderAlias)
	if err != nil {
		t.Fatalf("load provider: %v", err)
	}
	return *provider
}

// seedCatalog writes an initial stored catalog for the test provider.
func (e *catalogTestEnv) seedCatalog(t *testing.T, models []domain.LlmProviderModel, rich bool) {
	t.Helper()
	provider := e.provider(t)
	if err := e.repo.MergeProviderCatalog(provider.ID, models, e.clock.Now(), rich); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
}

// TestCatalogPrefersRichSource guards
// llm-model-catalog.native-source-preferred: values reported by the provider are
// served as-is instead of being replaced by registry defaults.
func TestCatalogPrefersRichSource(t *testing.T) {
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{{
		ID:                 "qwen-vl-ocr-plus",
		Name:               "Qwen VL OCR Plus",
		ContextLength:      64000,
		Capabilities:       []string{"image_edit"},
		InputModalities:    []string{"text", "image"},
		OutputModalities:   []string{"text"},
		InputPricePerMTok:  1.5,
		OutputPricePerMTok: 2.5,
		Provenance:         modelllm.ProvenanceRich,
	}}, nil)
	defer env.cleanup()

	result, err := env.service.Get(context.Background(), testProviderAlias, true)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(result.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(result.Models))
	}

	model := result.Models[0]
	if model.ContextLength != 64000 {
		t.Errorf("context length = %d, want the provider's 64000", model.ContextLength)
	}
	if len(model.Capabilities) != 1 || model.Capabilities[0] != "image_edit" {
		t.Errorf("capabilities = %v, want the provider's [image_edit]", model.Capabilities)
	}
	if model.InputPricePerMTok != 1.5 || model.OutputPricePerMTok != 2.5 {
		t.Errorf("prices = %v/%v, want the provider's 1.5/2.5", model.InputPricePerMTok, model.OutputPricePerMTok)
	}
}

// TestCatalogRegistryFillsListing guards
// llm-model-catalog.registry-fills-partial-listing: a bare model listing is
// completed from the curated model-family registry.
func TestCatalogRegistryFillsListing(t *testing.T) {
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{{
		ID:   "qwen-vl-ocr-plus",
		Name: "Qwen VL OCR Plus",
	}}, nil)
	defer env.cleanup()

	result, err := env.service.Get(context.Background(), testProviderAlias, true)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	model := result.Models[0]

	if !contains(model.Capabilities, "vision") {
		t.Errorf("capabilities = %v, want vision from the registry", model.Capabilities)
	}
	if !contains(model.InputModalities, "image") {
		t.Errorf("input modalities = %v, want image from the registry", model.InputModalities)
	}
	if model.ContextLength != 128000 {
		t.Errorf("context length = %d, want the registry's 128000", model.ContextLength)
	}
}

// TestCatalogUnknownModelDefault guards
// llm-model-catalog.unknown-model-safe-default: nothing describing the model
// still yields a usable chat entry.
func TestCatalogUnknownModelDefault(t *testing.T) {
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{{ID: "acme-chat-7b"}}, nil)
	defer env.cleanup()

	result, err := env.service.Get(context.Background(), testProviderAlias, true)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	model := result.Models[0]

	if !contains(model.Capabilities, "chat") || !contains(model.Capabilities, "tool_calling") {
		t.Errorf("capabilities = %v, want chat and tool_calling", model.Capabilities)
	}
	if contains(model.Capabilities, "vision") {
		t.Errorf("capabilities = %v, want no vision", model.Capabilities)
	}
	if len(model.InputModalities) != 1 || model.InputModalities[0] != "text" {
		t.Errorf("input modalities = %v, want [text]", model.InputModalities)
	}
}

// TestModelsResponseEqualsStored guards
// llm-model-catalog.response-equals-stored: the served list is the stored one.
func TestModelsResponseEqualsStored(t *testing.T) {
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{
		{
			ID:               "qwen3.5-flash",
			Name:             "Qwen3.5-Flash",
			ContextLength:    1000000,
			Capabilities:     []string{"chat", "tool_calling", "vision"},
			InputModalities:  []string{"text", "image"},
			OutputModalities: []string{"text"},
			Provenance:       modelllm.ProvenanceRich,
		},
		{
			ID:               "acme-chat-7b",
			Name:             "Acme Chat",
			ContextLength:    32768,
			Capabilities:     []string{"chat"},
			InputModalities:  []string{"text"},
			OutputModalities: []string{"text"},
			Provenance:       modelllm.ProvenanceRich,
		},
	}, nil)
	defer env.cleanup()

	first, err := env.service.Get(context.Background(), testProviderAlias, true)
	if err != nil {
		t.Fatalf("initial Get: %v", err)
	}

	provider := env.provider(t)
	stored, err := env.repo.GetProviderCatalog(provider.ID)
	if err != nil {
		t.Fatalf("get provider catalog: %v", err)
	}

	storedInfos := modelInfosFromStored(stored.Models)
	if len(first.Models) != len(storedInfos) {
		t.Fatalf("served %d models, stored %d", len(first.Models), len(storedInfos))
	}
	for i := range storedInfos {
		assertSameModel(t, first.Models[i], storedInfos[i])
	}

	// A second, non-forcing read must return exactly the same values.
	second, err := env.service.Get(context.Background(), testProviderAlias, false)
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if len(second.Models) != len(first.Models) {
		t.Fatalf("second read served %d models, first %d", len(second.Models), len(first.Models))
	}
	for i := range first.Models {
		assertSameModel(t, first.Models[i], second.Models[i])
	}
}

// TestModelsResponseExposesFetchedAt guards
// llm-model-catalog.response-exposes-freshness.
func TestModelsResponseExposesFetchedAt(t *testing.T) {
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{{ID: "acme-chat-7b"}}, nil)
	defer env.cleanup()

	result, err := env.service.Get(context.Background(), testProviderAlias, true)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if result.FetchedAt == nil {
		t.Fatal("expected a fetchedAt timestamp")
	}
	if !result.FetchedAt.Equal(env.clock.Now()) {
		t.Errorf("fetchedAt = %v, want %v", result.FetchedAt, env.clock.Now())
	}
}

// TestCatalogFreshNotRefetched guards
// llm-model-catalog.fresh-catalog-not-refetched.
func TestCatalogFreshNotRefetched(t *testing.T) {
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{{ID: "acme-chat-7b"}}, nil)
	defer env.cleanup()

	if _, err := env.service.Get(context.Background(), testProviderAlias, true); err != nil {
		t.Fatalf("initial Get: %v", err)
	}
	if env.lister.calls != 1 {
		t.Fatalf("lister calls = %d, want 1", env.lister.calls)
	}

	env.clock.advance(testTTL / 2)
	result, err := env.service.Get(context.Background(), testProviderAlias, false)
	if err != nil {
		t.Fatalf("fresh Get: %v", err)
	}
	if env.lister.calls != 1 {
		t.Errorf("lister calls = %d, want the provider not to be queried again", env.lister.calls)
	}
	if result.Stale {
		t.Error("a fresh catalog must not be reported as stale")
	}
}

// TestCatalogExpiredRefetched guards
// llm-model-catalog.expired-catalog-refetched.
func TestCatalogExpiredRefetched(t *testing.T) {
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{{ID: "acme-chat-7b"}}, nil)
	defer env.cleanup()

	if _, err := env.service.Get(context.Background(), testProviderAlias, true); err != nil {
		t.Fatalf("initial Get: %v", err)
	}

	env.clock.advance(testTTL + time.Minute)
	if _, err := env.service.Get(context.Background(), testProviderAlias, false); err != nil {
		t.Fatalf("expired Get: %v", err)
	}
	if env.lister.calls != 2 {
		t.Errorf("lister calls = %d, want the expired catalog to be re-collected", env.lister.calls)
	}
}

// TestCatalogExplicitRefreshIgnoresTTL guards
// llm-model-catalog.explicit-refresh-ignores-ttl.
func TestCatalogExplicitRefreshIgnoresTTL(t *testing.T) {
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{{ID: "acme-chat-7b"}}, nil)
	defer env.cleanup()

	if _, err := env.service.Get(context.Background(), testProviderAlias, true); err != nil {
		t.Fatalf("initial Get: %v", err)
	}
	if _, err := env.service.Get(context.Background(), testProviderAlias, true); err != nil {
		t.Fatalf("explicit refresh Get: %v", err)
	}
	if env.lister.calls != 2 {
		t.Errorf("lister calls = %d, want an explicit refresh even though the catalog is fresh", env.lister.calls)
	}
}

// TestMergeKeepsVision guards
// llm-model-catalog.degraded-refresh-keeps-vision.
func TestMergeKeepsVision(t *testing.T) {
	env := newCatalogTestEnv(t, nil, nil)
	defer env.cleanup()

	env.seedCatalog(t, []domain.LlmProviderModel{{
		ModelID:   "qwen3.5-flash",
		ModelName: "Qwen3.5-Flash",
		Capabilities: []domain.LlmModelCapability{
			{Capability: "chat"},
			{Capability: "vision"},
		},
	}}, true)

	provider := env.provider(t)
	if err := env.repo.MergeProviderCatalog(provider.ID, []domain.LlmProviderModel{{
		ModelID:   "qwen3.5-flash",
		ModelName: "Qwen3.5-Flash",
	}}, env.clock.Now(), false); err != nil {
		t.Fatalf("degraded merge: %v", err)
	}

	stored, err := env.repo.GetProviderCatalog(provider.ID)
	if err != nil {
		t.Fatalf("get provider catalog: %v", err)
	}
	if len(stored.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(stored.Models))
	}
	if !hasStoredCapability(stored.Models[0], "vision") {
		t.Errorf("capabilities = %v, want the stored vision capability kept", stored.Models[0].Capabilities)
	}
}

// TestMergeKeepsContextLength guards
// llm-model-catalog.degraded-refresh-keeps-context.
func TestMergeKeepsContextLength(t *testing.T) {
	env := newCatalogTestEnv(t, nil, nil)
	defer env.cleanup()

	env.seedCatalog(t, []domain.LlmProviderModel{{
		ModelID:       "qwen3.5-flash",
		ModelName:     "Qwen3.5-Flash",
		ContextLength: 1000000,
	}}, true)

	provider := env.provider(t)
	if err := env.repo.MergeProviderCatalog(provider.ID, []domain.LlmProviderModel{{
		ModelID:   "qwen3.5-flash",
		ModelName: "Qwen3.5-Flash",
	}}, env.clock.Now(), false); err != nil {
		t.Fatalf("degraded merge: %v", err)
	}

	stored, err := env.repo.GetProviderCatalog(provider.ID)
	if err != nil {
		t.Fatalf("get provider catalog: %v", err)
	}
	if stored.Models[0].ContextLength != 1000000 {
		t.Errorf("context length = %d, want the stored 1000000 kept", stored.Models[0].ContextLength)
	}
}

// TestMergeKeepsMissingModels guards
// llm-model-catalog.degraded-refresh-keeps-models.
func TestMergeKeepsMissingModels(t *testing.T) {
	env := newCatalogTestEnv(t, nil, nil)
	defer env.cleanup()

	env.seedCatalog(t, []domain.LlmProviderModel{
		{ModelID: "model-a", ModelName: "Model A"},
		{ModelID: "model-b", ModelName: "Model B"},
	}, true)

	provider := env.provider(t)
	if err := env.repo.MergeProviderCatalog(provider.ID, []domain.LlmProviderModel{
		{ModelID: "model-a", ModelName: "Model A"},
	}, env.clock.Now(), false); err != nil {
		t.Fatalf("degraded merge: %v", err)
	}

	stored, err := env.repo.GetProviderCatalog(provider.ID)
	if err != nil {
		t.Fatalf("get provider catalog: %v", err)
	}
	if len(stored.Models) != 2 {
		t.Fatalf("expected the missing model to be kept, got %d models", len(stored.Models))
	}

	// A rich refresh is allowed to delete models the provider dropped.
	if err := env.repo.MergeProviderCatalog(provider.ID, []domain.LlmProviderModel{
		{ModelID: "model-a", ModelName: "Model A"},
	}, env.clock.Now(), true); err != nil {
		t.Fatalf("rich merge: %v", err)
	}
	stored, err = env.repo.GetProviderCatalog(provider.ID)
	if err != nil {
		t.Fatalf("get provider catalog: %v", err)
	}
	if len(stored.Models) != 1 {
		t.Fatalf("expected the dropped model to be deleted by a rich refresh, got %d models", len(stored.Models))
	}
}

// TestRefreshCorrectsImageEditClassification guards the regression where a wan
// image model stored as a chat model stayed missing from the image edit
// instrument dropdown: a refresh that reports the model's modalities must
// replace the stored classification, because the image-edit capability is
// derived from provider facts rather than from identifier heuristics.
func TestRefreshCorrectsImageEditClassification(t *testing.T) {
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{
		modelllm.EnrichModelInfo(modelllm.ModelInfo{
			ID:               "wan2.7-image",
			Name:             "Wan 2.7 Image",
			InputModalities:  []string{"text", "image"},
			OutputModalities: []string{"image"},
			Provenance:       modelllm.ProvenancePartial,
		}),
	}, nil)
	defer env.cleanup()

	env.seedCatalog(t, []domain.LlmProviderModel{{
		ModelID:   "wan2.7-image",
		ModelName: "Wan 2.7 Image",
		Capabilities: []domain.LlmModelCapability{
			{Capability: "chat"},
			{Capability: "tool_calling"},
		},
		Modalities: []domain.LlmModelModality{
			{Modality: "text", Direction: domain.ModalityDirectionInput},
			{Modality: "text", Direction: domain.ModalityDirectionOutput},
		},
	}}, true)

	result, err := env.service.Get(context.Background(), testProviderAlias, true)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(result.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(result.Models))
	}

	model := result.Models[0]
	if !contains(model.Capabilities, "image_edit") {
		t.Errorf("capabilities = %v, want image_edit after the refresh", model.Capabilities)
	}
	if contains(model.Capabilities, "chat") {
		t.Errorf("capabilities = %v, want the stale chat capability replaced", model.Capabilities)
	}
}

// TestDegradedRefreshKeepsStoredModalities guards the production regression where
// an identifier-only listing (the Alibaba compatible-mode fallback) replaced the
// provider-reported modalities of every stored model with text only.
func TestDegradedRefreshKeepsStoredModalities(t *testing.T) {
	// The Alibaba compatible-mode listing exposes only the identifier, so the
	// client fills the rest from the registry and the ID heuristics — exactly the
	// values that must never overwrite what the provider reported earlier.
	listing := modelllm.EnrichModelInfo(modelllm.ModelInfo{
		ID:         "qwen3.5-flash",
		Name:       "Qwen3.5-Flash",
		Provenance: modelllm.ProvenanceHeuristic,
	})
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{listing}, nil)
	defer env.cleanup()

	env.seedCatalog(t, []domain.LlmProviderModel{{
		ModelID:       "qwen3.5-flash",
		ModelName:     "Qwen3.5-Flash",
		ContextLength: 1000000,
		Capabilities: []domain.LlmModelCapability{
			{Capability: "chat"},
			{Capability: "vision"},
		},
		Modalities: []domain.LlmModelModality{
			{Modality: "text", Direction: domain.ModalityDirectionInput},
			{Modality: "image", Direction: domain.ModalityDirectionInput},
			{Modality: "video", Direction: domain.ModalityDirectionInput},
			{Modality: "text", Direction: domain.ModalityDirectionOutput},
		},
	}}, true)

	result, err := env.service.Get(context.Background(), testProviderAlias, true)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(result.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(result.Models))
	}

	model := result.Models[0]
	if !contains(model.Capabilities, "vision") {
		t.Errorf("capabilities = %v, want the stored vision capability kept", model.Capabilities)
	}
	for _, modality := range []string{"text", "image", "video"} {
		if !contains(model.InputModalities, modality) {
			t.Errorf("input modalities = %v, want the stored %q kept", model.InputModalities, modality)
		}
	}
	if model.ContextLength != 1000000 {
		t.Errorf("context length = %d, want the stored 1000000 kept", model.ContextLength)
	}
	if result.Stale {
		t.Error("a degraded refresh that kept the catalog is not a failure")
	}
}

// TestRefreshWriteFailureReported guards
// llm-model-catalog.write-failure-reported.
func TestRefreshWriteFailureReported(t *testing.T) {
	writeErr := errors.New("catalog write failed")
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{{ID: "acme-chat-7b"}}, func(inner repository.LlmCatalogRepository) repository.LlmCatalogRepository {
		return &failingCatalogRepo{LlmCatalogRepository: inner, err: writeErr}
	})
	defer env.cleanup()

	env.seedCatalog(t, []domain.LlmProviderModel{{
		ModelID:   "acme-chat-7b",
		ModelName: "Acme Chat",
	}}, true)

	result, err := env.service.Get(context.Background(), testProviderAlias, true)
	if err != nil {
		t.Fatalf("Get must not fail while a catalog is stored: %v", err)
	}
	if !result.Stale {
		t.Error("expected the failed refresh to be reported as stale")
	}
	if result.RefreshError == nil {
		t.Error("expected the refresh failure text to be reported")
	}
}

// TestRefreshFailureKeepsCatalog guards
// llm-model-catalog.write-failure-keeps-catalog.
func TestRefreshFailureKeepsCatalog(t *testing.T) {
	writeErr := errors.New("catalog write failed")
	env := newCatalogTestEnv(t, []modelllm.ModelInfo{{ID: "acme-chat-7b"}}, func(inner repository.LlmCatalogRepository) repository.LlmCatalogRepository {
		return &failingCatalogRepo{LlmCatalogRepository: inner, err: writeErr}
	})
	defer env.cleanup()

	env.seedCatalog(t, []domain.LlmProviderModel{{
		ModelID:   "stored-model",
		ModelName: "Stored Model",
	}}, true)

	result, err := env.service.Get(context.Background(), testProviderAlias, true)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(result.Models) != 1 || result.Models[0].ID != "stored-model" {
		t.Fatalf("served models = %+v, want the previously stored catalog", result.Models)
	}

	provider := env.provider(t)
	stored, err := env.repo.GetProviderCatalog(provider.ID)
	if err != nil {
		t.Fatalf("get provider catalog: %v", err)
	}
	if len(stored.Models) != 1 || stored.Models[0].ModelID != "stored-model" {
		t.Fatalf("stored models = %+v, want them unchanged", stored.Models)
	}
}

// TestRefreshFailureWithoutCatalogReportsError verifies that a failed refresh
// with no stored catalog surfaces an error instead of an empty success.
func TestRefreshFailureWithoutCatalogReportsError(t *testing.T) {
	env := newCatalogTestEnv(t, nil, nil)
	defer env.cleanup()

	env.lister.err = errors.New("provider unreachable")

	if _, err := env.service.Get(context.Background(), testProviderAlias, true); err == nil {
		t.Fatal("expected an error when the refresh fails and no catalog is stored")
	}
}

// TestCatalogUnknownProvider verifies unknown aliases surface ErrProviderNotFound.
func TestCatalogUnknownProvider(t *testing.T) {
	env := newCatalogTestEnv(t, nil, nil)
	defer env.cleanup()

	_, err := env.service.Get(context.Background(), "does-not-exist", true)
	if !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("err = %v, want ErrProviderNotFound", err)
	}
}

// ── helpers ───────────────────────────────────────────────────────

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func hasStoredCapability(model domain.LlmProviderModel, want string) bool {
	for _, capability := range model.Capabilities {
		if capability.Capability == want {
			return true
		}
	}
	return false
}

func assertSameModel(t *testing.T, got, want modelllm.ModelInfo) {
	t.Helper()
	if got.ID != want.ID || got.Name != want.Name {
		t.Errorf("model identity = %q/%q, want %q/%q", got.ID, got.Name, want.ID, want.Name)
	}
	if got.ContextLength != want.ContextLength {
		t.Errorf("model %s context length = %d, want %d", got.ID, got.ContextLength, want.ContextLength)
	}
	if got.InputPricePerMTok != want.InputPricePerMTok || got.OutputPricePerMTok != want.OutputPricePerMTok {
		t.Errorf("model %s prices = %v/%v, want %v/%v",
			got.ID, got.InputPricePerMTok, got.OutputPricePerMTok,
			want.InputPricePerMTok, want.OutputPricePerMTok)
	}
	if len(got.Capabilities) != len(want.Capabilities) {
		t.Errorf("model %s capabilities = %v, want %v", got.ID, got.Capabilities, want.Capabilities)
	}
	if len(got.InputModalities) != len(want.InputModalities) {
		t.Errorf("model %s input modalities = %v, want %v", got.ID, got.InputModalities, want.InputModalities)
	}
	if len(got.OutputModalities) != len(want.OutputModalities) {
		t.Errorf("model %s output modalities = %v, want %v", got.ID, got.OutputModalities, want.OutputModalities)
	}
}
