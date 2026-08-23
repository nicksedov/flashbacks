package helpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"
	"github.com/flashbacks/api-service/internal/testutil/fixtures"
)

func TestMoveToTrashOrDeleteRecordsTrashItem(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	srcDir := fixtures.CreateTempDir(t)
	trashDir := fixtures.CreateTempDir(t)

	srcPath := fixtures.CreateTestFile(t, srcDir, "photo.jpg", []byte("jpeg-bytes"))
	// Seed the source file record so the mover can capture hash/mod time.
	testutil.SeedImageFile(t, db, filepath.ToSlash(srcPath), "hash-abc", 10)

	fm := NewFileMover(db)
	if err := fm.MoveToTrashOrDelete(srcPath, trashDir); err != nil {
		t.Fatalf("MoveToTrashOrDelete failed: %v", err)
	}

	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Fatalf("expected source file to be gone after move, stat err = %v", err)
	}

	var item domain.TrashItem
	if err := db.First(&item).Error; err != nil {
		t.Fatalf("expected a TrashItem row to be created: %v", err)
	}

	if item.OriginalPath != filepath.ToSlash(srcPath) {
		t.Errorf("OriginalPath = %q, want %q", item.OriginalPath, filepath.ToSlash(srcPath))
	}
	if item.TrashPath == "" {
		t.Error("TrashPath is empty")
	}
	if item.FileName != "photo.jpg" {
		t.Errorf("FileName = %q, want photo.jpg", item.FileName)
	}
	if item.Size != 10 {
		t.Errorf("Size = %d, want 10", item.Size)
	}
	if item.DeletedAt.IsZero() {
		t.Error("DeletedAt is zero, want a non-zero deletion timestamp")
	}
	if item.OriginalHash != "hash-abc" {
		t.Errorf("OriginalHash = %q, want hash-abc", item.OriginalHash)
	}

	// The file should now be present inside the trash dir.
	if _, err := os.Stat(item.TrashPath); err != nil {
		t.Errorf("expected file to exist at trash path %q: %v", item.TrashPath, err)
	}
}

func TestMoveToTrashOrDeletePermanentDeleteRecordsNothing(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	srcDir := fixtures.CreateTempDir(t)
	srcPath := fixtures.CreateTestFile(t, srcDir, "photo.jpg", []byte("jpeg-bytes"))
	testutil.SeedImageFile(t, db, filepath.ToSlash(srcPath), "hash-abc", 11)

	fm := NewFileMover(db)
	// Empty trashDir means permanent delete - no TrashItem should be recorded.
	if err := fm.MoveToTrashOrDelete(srcPath, ""); err != nil {
		t.Fatalf("MoveToTrashOrDelete failed: %v", err)
	}

	var count int64
	db.Model(&domain.TrashItem{}).Count(&count)
	if count != 0 {
		t.Errorf("expected no TrashItem rows on permanent delete, got %d", count)
	}
}
