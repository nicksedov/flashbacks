package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"

	"gorm.io/gorm"
)

// TestProviderCatalogFreshnessColumnsMigrated guards task 3.1: a freshly
// migrated database carries both freshness columns.
func TestProviderCatalogFreshnessColumnsMigrated(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	for _, column := range []string{"models_fetched_at", "models_source_rich"} {
		if !db.Migrator().HasColumn(&domain.LlmProvider{}, column) {
			t.Errorf("llm_providers is missing column %q", column)
		}
	}
}

// seedProvider creates a provider row directly so each test starts from a known
// database state.
func seedProvider(t *testing.T, db *gorm.DB, alias string) *domain.LlmProvider {
	t.Helper()
	provider := domain.LlmProvider{
		Name:   "alibaba",
		Alias:  alias,
		ApiUrl: "https://dashscope.aliyuncs.com",
	}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	return &provider
}

// TestMergeProviderCatalogToleratesDuplicateCapability guards task 3.2: a
// duplicate capability row must not abort the whole provider write.
func TestMergeProviderCatalogToleratesDuplicateCapability(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	repo := NewLlmCatalogRepository(db)
	provider := seedProvider(t, db, "alibaba_duplicate_test")

	models := []domain.LlmProviderModel{{
		ModelID:   "qwen3.5-flash",
		ModelName: "Qwen3.5-Flash",
		Capabilities: []domain.LlmModelCapability{
			{Capability: "vision"},
			{Capability: "vision"},
		},
		Modalities: []domain.LlmModelModality{
			{Modality: "image", Direction: domain.ModalityDirectionInput},
			{Modality: "image", Direction: domain.ModalityDirectionInput},
		},
	}}

	if err := repo.MergeProviderCatalog(provider.ID, models, time.Now().UTC(), true); err != nil {
		t.Fatalf("merge with a duplicate capability must not fail: %v", err)
	}

	stored, err := repo.GetProviderCatalog(provider.ID)
	if err != nil {
		t.Fatalf("get provider catalog: %v", err)
	}
	if len(stored.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(stored.Models))
	}
	if len(stored.Models[0].Capabilities) != 1 {
		t.Errorf("capabilities = %v, want the duplicate collapsed to one row", stored.Models[0].Capabilities)
	}
	if len(stored.Models[0].Modalities) != 1 {
		t.Errorf("modalities = %v, want the duplicate collapsed to one row", stored.Models[0].Modalities)
	}
}

// TestMergeProviderCatalogPersistsFreshness guards task 3.3: the refresh
// timestamp and provenance flag are written with the rows, and a failed write
// leaves both untouched.
func TestMergeProviderCatalogPersistsFreshness(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	repo := NewLlmCatalogRepository(db)
	provider := seedProvider(t, db, "alibaba_freshness_test")

	firstRefresh := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	if err := repo.MergeProviderCatalog(provider.ID, []domain.LlmProviderModel{{
		ModelID:       "qwen3.5-flash",
		ModelName:     "Qwen3.5-Flash",
		ContextLength: 1000000,
	}}, firstRefresh, true); err != nil {
		t.Fatalf("initial merge: %v", err)
	}

	stored, err := repo.GetProviderCatalog(provider.ID)
	if err != nil {
		t.Fatalf("get provider catalog: %v", err)
	}
	if stored.FetchedAt == nil || !stored.FetchedAt.Equal(firstRefresh) {
		t.Fatalf("fetchedAt = %v, want %v", stored.FetchedAt, firstRefresh)
	}
	if !stored.SourceRich {
		t.Error("expected the rich provenance flag to be persisted")
	}

	// Force the final provider update to fail so the transaction must roll back.
	callbackName := "test:fail_provider_update"
	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "llm_providers" {
			_ = tx.AddError(errors.New("forced provider update failure"))
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}

	secondRefresh := firstRefresh.Add(24 * time.Hour)
	err = repo.MergeProviderCatalog(provider.ID, []domain.LlmProviderModel{
		{
			ModelID:       "qwen3.5-flash",
			ModelName:     "Qwen3.5-Flash",
			ContextLength: 2000000,
		},
		{ModelID: "brand-new-model", ModelName: "Brand New"},
	}, secondRefresh, false)
	if err == nil {
		t.Fatal("expected the forced write failure to be reported")
	}

	if err := db.Callback().Update().Remove(callbackName); err != nil {
		t.Fatalf("remove callback: %v", err)
	}

	stored, err = repo.GetProviderCatalog(provider.ID)
	if err != nil {
		t.Fatalf("get provider catalog after failed write: %v", err)
	}
	if stored.FetchedAt == nil || !stored.FetchedAt.Equal(firstRefresh) {
		t.Errorf("fetchedAt = %v, want the previous %v untouched", stored.FetchedAt, firstRefresh)
	}
	if !stored.SourceRich {
		t.Error("expected the previous provenance flag to remain untouched")
	}
	if len(stored.Models) != 1 {
		t.Fatalf("models = %v, want the previous single model untouched", stored.Models)
	}
	if stored.Models[0].ContextLength != 1000000 {
		t.Errorf("context length = %d, want the previous 1000000 untouched", stored.Models[0].ContextLength)
	}
}
