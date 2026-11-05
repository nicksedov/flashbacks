package repository

import (
	"errors"
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"

	"gorm.io/gorm"
)

// TestPersonReferenceRepositoryReplaceAndMedoid guards the exemplar port:
// replacing the set drops the previous one, and the medoid is the exemplar
// closest to the set's centroid.
func TestPersonReferenceRepositoryReplaceAndMedoid(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	person := domain.Person{}
	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}

	repo := NewPersonReferenceRepository(db)
	references := []domain.PersonReference{
		{FaceID: 1, Dimension: 3, Embedding: domain.FormatEmbedding([]float32{1, 0, 0}), Quality: 0.9},
		{FaceID: 2, Dimension: 3, Embedding: domain.FormatEmbedding([]float32{0.9, 0.1, 0}), Quality: 0.8},
		{FaceID: 3, Dimension: 3, Embedding: domain.FormatEmbedding([]float32{0, 1, 0}), Quality: 0.7},
	}
	if err := repo.Replace(person.ID, references); err != nil {
		t.Fatalf("replace references: %v", err)
	}

	stored, err := repo.List(person.ID)
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(stored) != 3 {
		t.Fatalf("stored %d references, want 3", len(stored))
	}
	for _, reference := range stored {
		if reference.PersonID != person.ID {
			t.Errorf("reference person = %d, want %d", reference.PersonID, person.ID)
		}
	}

	medoid, err := repo.Medoid(person.ID)
	if err != nil {
		t.Fatalf("medoid: %v", err)
	}
	// Centroid = (1.9, 1.1, 0); face 2 (0.9, 0.1, 0) is the closest exemplar.
	if medoid.FaceID != 2 {
		t.Errorf("medoid face = %d, want 2", medoid.FaceID)
	}

	// Replacing the set drops the previous exemplars instead of accumulating.
	if err := repo.Replace(person.ID, references[2:]); err != nil {
		t.Fatalf("replace references again: %v", err)
	}
	stored, err = repo.List(person.ID)
	if err != nil {
		t.Fatalf("list references after replace: %v", err)
	}
	if len(stored) != 1 || stored[0].FaceID != 3 {
		t.Fatalf("references after replace = %+v, want face 3 only", stored)
	}

	grouped, err := repo.ListAll()
	if err != nil {
		t.Fatalf("list all references: %v", err)
	}
	if len(grouped[person.ID]) != 1 {
		t.Errorf("grouped references = %d, want 1", len(grouped[person.ID]))
	}

	if err := repo.DeleteByPersonIDs([]uint{person.ID}); err != nil {
		t.Fatalf("delete references: %v", err)
	}
	if _, err := repo.Medoid(person.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("medoid of a cleared person = %v, want gorm.ErrRecordNotFound", err)
	}
}
