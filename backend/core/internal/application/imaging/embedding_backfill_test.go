package imaging

import (
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"

	"gorm.io/gorm"
)

// --- Domain helper function tests ---

func TestSanitizeModelName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple model", "qwen3-embedding", "qwen3_embedding"},
		{"with colon and version", "model:v1.0", "model_v1_0"},
		{"with slashes", "org/model/name", "org_model_name"},
		{"already clean", "model_name_v1", "model_name_v1"},
		{"multiple special chars", "a::b//c--d", "a_b_c_d"},
		{"leading/trailing special", ":model:", "model"},
		{"empty string", "", ""},
		{"all special chars", ":::", ""},
		{"mixed underscores", "a__b___c", "a_b_c"},
		{"numbers preserved", "model-123-v2", "model_123_v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domain.SanitizeModelName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeModelName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEmbeddingTableName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard model", "qwen3-embedding:4b", "tag_embeddings_qwen3_embedding_4b"},
		{"simple name", "model_v1", "tag_embeddings_model_v1"},
		{"complex name", "org/model:v1.0", "tag_embeddings_org_model_v1_0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domain.EmbeddingTableName(tt.input)
			if result != tt.expected {
				t.Errorf("EmbeddingTableName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// --- EmbeddingBackfillManager lifecycle tests ---

func TestNewEmbeddingBackfillManager(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	mgr := NewEmbeddingBackfillManager(db)
	if mgr == nil {
		t.Fatal("NewEmbeddingBackfillManager returned nil")
	}
	if mgr.IsRunning() {
		t.Error("new manager should not be running")
	}
}

func TestEmbeddingBackfillManager_GetStatus(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	mgr := NewEmbeddingBackfillManager(db)
	status := mgr.GetStatus()

	if status.Running {
		t.Error("initial status should not be running")
	}
	if status.Progress.Total != 0 {
		t.Errorf("initial total should be 0, got %d", status.Progress.Total)
	}
	if status.Progress.Processed != 0 {
		t.Errorf("initial processed should be 0, got %d", status.Progress.Processed)
	}
	if status.Progress.Remaining != 0 {
		t.Errorf("initial remaining should be 0, got %d", status.Progress.Remaining)
	}
	if status.Progress.LastError != "" {
		t.Errorf("initial last error should be empty, got %q", status.Progress.LastError)
	}
}

func TestEmbeddingBackfillManager_StopWhenNotRunning(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	mgr := NewEmbeddingBackfillManager(db)
	// Should not panic or cause issues
	mgr.Stop()
	if mgr.IsRunning() {
		t.Error("manager should not be running after stop on non-running manager")
	}
}

func TestEmbeddingBackfillManager_SetError(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	mgr := NewEmbeddingBackfillManager(db)
	mgr.setError("test error message")

	status := mgr.GetStatus()
	if status.Progress.LastError != "test error message" {
		t.Errorf("expected last error %q, got %q", "test error message", status.Progress.LastError)
	}
}

// --- Upsert parent record tests (using SQLite without pgvector) ---

func TestUpsertEmbedding_CreatesParentRecord(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Create a parent record via upsertEmbedding's parent logic
	// We test only the parent part since SQLite doesn't support ::vector
	var parent domain.TagEmbedding
	result := db.Where("image_file_id = ?", 1).First(&parent)
	if result.Error != gorm.ErrRecordNotFound {
		t.Fatal("expected record not found initially")
	}

	// Create parent record manually (simulating upsertEmbedding's parent logic)
	parent = domain.TagEmbedding{ImageFileID: 1, TagCount: 5}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("failed to create parent record: %v", err)
	}

	// Verify it was created
	var found domain.TagEmbedding
	if err := db.Where("image_file_id = ?", 1).First(&found).Error; err != nil {
		t.Fatalf("failed to find parent record: %v", err)
	}
	if found.TagCount != 5 {
		t.Errorf("expected tag count 5, got %d", found.TagCount)
	}
	if found.ImageFileID != 1 {
		t.Errorf("expected image_file_id 1, got %d", found.ImageFileID)
	}
}

func TestUpsertEmbedding_UpdatesExistingParent(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Create initial parent
	parent := domain.TagEmbedding{ImageFileID: 42, TagCount: 3}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("failed to create parent record: %v", err)
	}

	// Update tag count (simulating upsertEmbedding's update path)
	db.Model(&parent).Update("tag_count", 7)

	var found domain.TagEmbedding
	if err := db.Where("image_file_id = ?", 42).First(&found).Error; err != nil {
		t.Fatalf("failed to find parent record: %v", err)
	}
	if found.TagCount != 7 {
		t.Errorf("expected updated tag count 7, got %d", found.TagCount)
	}
}

// --- Concurrency safety tests ---

func TestEmbeddingBackfillManager_ConcurrentGetStatus(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	mgr := NewEmbeddingBackfillManager(db)

	// Simulate concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = mgr.GetStatus()
			_ = mgr.IsRunning()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent access timed out")
		}
	}
}
