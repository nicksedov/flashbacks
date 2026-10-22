package repository

import (
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"
)

func TestImageFileRepository_ListAll(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	repo := NewImageFileRepository(db)
	now := time.Now()
	files := []domain.ImageFile{
		{Path: "/gallery/a.jpg", Size: 1, Hash: "h1", ModTime: now},
		{Path: "/gallery/b.jpg", Size: 2, Hash: "h2", ModTime: now},
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatalf("seed image files: %v", err)
	}

	got, err := repo.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 files, got %d", len(got))
	}
	if got[0].Path != "/gallery/a.jpg" || got[1].Path != "/gallery/b.jpg" {
		t.Fatalf("expected insertion order, got %q then %q", got[0].Path, got[1].Path)
	}
}
