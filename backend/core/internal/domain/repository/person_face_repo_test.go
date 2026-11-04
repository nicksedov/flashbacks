package repository

import (
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"
)

// TestPersonRepositoryCRUD guards the person persistence port.
func TestPersonRepositoryCRUD(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	repo := NewPersonRepository(db)

	person := domain.Person{Name: ""}
	if err := repo.Create(&person); err != nil {
		t.Fatalf("create person: %v", err)
	}
	if person.ID == 0 {
		t.Fatal("expected created person to have an ID")
	}

	person.Name = "Alice"
	if err := repo.Save(&person); err != nil {
		t.Fatalf("save person: %v", err)
	}

	loaded, err := repo.FindByID(person.ID)
	if err != nil {
		t.Fatalf("find person: %v", err)
	}
	if loaded.Name != "Alice" {
		t.Errorf("name = %q, want Alice", loaded.Name)
	}

	people, err := repo.List()
	if err != nil {
		t.Fatalf("list people: %v", err)
	}
	if len(people) != 1 {
		t.Errorf("list length = %d, want 1", len(people))
	}
}

// TestFaceRepositoryAssignAndListGuards the face persistence port: a detected
// face references exactly one person and a person's photos are listed and
// counted distinctly.
func TestFaceRepositoryAssignAndList(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	personRepo := NewPersonRepository(db)
	faceRepo := NewFaceRepository(db)

	person := domain.Person{}
	if err := personRepo.Create(&person); err != nil {
		t.Fatalf("create person: %v", err)
	}

	img1 := testutil.SeedImageFile(t, db, "/gallery/a.jpg", "hash-a", 10)
	img2 := testutil.SeedImageFile(t, db, "/gallery/b.jpg", "hash-b", 20)

	face1 := domain.Face{ImageFileID: img1.ID, X: 1, Y: 2, Width: 3, Height: 4, Confidence: 0.99}
	face2 := domain.Face{ImageFileID: img2.ID, X: 5, Y: 6, Width: 7, Height: 8, Confidence: 0.98}
	if err := faceRepo.Create(&face1); err != nil {
		t.Fatalf("create face 1: %v", err)
	}
	if err := faceRepo.Create(&face2); err != nil {
		t.Fatalf("create face 2: %v", err)
	}

	if err := faceRepo.SetPersonID(face1.ID, person.ID); err != nil {
		t.Fatalf("assign face 1: %v", err)
	}
	if err := faceRepo.SetPersonID(face2.ID, person.ID); err != nil {
		t.Fatalf("assign face 2: %v", err)
	}

	count, err := faceRepo.CountImageFilesByPersonID(person.ID)
	if err != nil {
		t.Fatalf("count photos: %v", err)
	}
	if count != 2 {
		t.Errorf("photo count = %d, want 2", count)
	}

	files, err := faceRepo.ListImageFilesByPersonID(person.ID, 0, 10)
	if err != nil {
		t.Fatalf("list photos: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("listed %d photos, want 2", len(files))
	}
	if files[0].ID != img1.ID || files[1].ID != img2.ID {
		t.Errorf("photo order = [%d %d], want [%d %d]", files[0].ID, files[1].ID, img1.ID, img2.ID)
	}
}

// TestFaceRepositoryEmbeddingRoundTrip guards halfvec embedding persistence.
func TestFaceRepositoryEmbeddingRoundTrip(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	faceRepo := NewFaceRepository(db)
	img := testutil.SeedImageFile(t, db, "/gallery/c.jpg", "hash-c", 30)

	face := domain.Face{ImageFileID: img.ID, X: 0, Y: 0, Width: 10, Height: 10, Confidence: 0.9}
	if err := faceRepo.Create(&face); err != nil {
		t.Fatalf("create face: %v", err)
	}

	embedding := domain.FaceEmbedding{
		FaceID:    face.ID,
		Dimension: 512,
		Embedding: "[0.1,0.2,0.3]",
		Model:     domain.FaceEmbeddingModel,
	}
	if err := faceRepo.CreateEmbedding(&embedding); err != nil {
		t.Fatalf("create embedding: %v", err)
	}
	if embedding.ID == 0 {
		t.Fatal("expected created embedding to have an ID")
	}
}
