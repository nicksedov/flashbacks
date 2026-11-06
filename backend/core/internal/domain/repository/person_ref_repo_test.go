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

// TestPersonReferenceRepositoryCatalogFlows guards the three edit flows the
// catalog service builds on List/Replace/DeleteByPersonIDs: a merge unions the
// target's and sources' exemplars without duplicating a face, a detach drops
// exactly one face from the set, and a disband removes the person's set.
func TestPersonReferenceRepositoryCatalogFlows(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	repo := NewPersonReferenceRepository(db)

	target := domain.Person{}
	source := domain.Person{}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create target: %v", err)
	}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}

	if err := repo.Replace(target.ID, []domain.PersonReference{
		{FaceID: 1, Dimension: 3, Embedding: domain.FormatEmbedding([]float32{1, 0, 0}), Quality: 0.9},
		{FaceID: 2, Dimension: 3, Embedding: domain.FormatEmbedding([]float32{0.8, 0.2, 0}), Quality: 0.8},
	}); err != nil {
		t.Fatalf("replace target references: %v", err)
	}
	if err := repo.Replace(source.ID, []domain.PersonReference{
		{FaceID: 2, Dimension: 3, Embedding: domain.FormatEmbedding([]float32{0.8, 0.2, 0}), Quality: 0.8},
		{FaceID: 3, Dimension: 3, Embedding: domain.FormatEmbedding([]float32{0, 1, 0}), Quality: 0.7},
	}); err != nil {
		t.Fatalf("replace source references: %v", err)
	}

	// Merge: concatenate the lists, filtering duplicates by face id.
	targetRefs, err := repo.List(target.ID)
	if err != nil {
		t.Fatalf("list target references: %v", err)
	}
	sourceRefs, err := repo.List(source.ID)
	if err != nil {
		t.Fatalf("list source references: %v", err)
	}
	// Replace re-inserts the rows, so the merged set is rebuilt without the
	// stored ids (the target's previous rows are deleted by Replace).
	union := make([]domain.PersonReference, 0, len(targetRefs)+len(sourceRefs))
	seen := make(map[uint]bool, len(targetRefs)+len(sourceRefs))
	for _, reference := range append(append([]domain.PersonReference{}, targetRefs...), sourceRefs...) {
		if seen[reference.FaceID] {
			continue
		}
		seen[reference.FaceID] = true
		reference.ID = 0
		union = append(union, reference)
	}
	if err := repo.Replace(target.ID, union); err != nil {
		t.Fatalf("replace merged references: %v", err)
	}
	if err := repo.DeleteByPersonIDs([]uint{source.ID}); err != nil {
		t.Fatalf("delete source references: %v", err)
	}

	merged, err := repo.List(target.ID)
	if err != nil {
		t.Fatalf("list merged references: %v", err)
	}
	if len(merged) != 3 {
		t.Fatalf("merged references = %d, want 3 distinct faces", len(merged))
	}
	for _, reference := range merged {
		if reference.PersonID != target.ID {
			t.Errorf("merged reference person = %d, want %d", reference.PersonID, target.ID)
		}
	}
	if remaining, err := repo.List(source.ID); err != nil {
		t.Fatalf("list source references after merge: %v", err)
	} else if len(remaining) != 0 {
		t.Errorf("source references = %d, want 0", len(remaining))
	}

	// Detach: keep every exemplar except the detached face.
	remaining := make([]domain.PersonReference, 0, len(merged))
	for _, reference := range merged {
		if reference.FaceID == 2 {
			continue
		}
		reference.ID = 0
		remaining = append(remaining, reference)
	}
	if err := repo.Replace(target.ID, remaining); err != nil {
		t.Fatalf("replace after detach: %v", err)
	}
	afterDetach, err := repo.List(target.ID)
	if err != nil {
		t.Fatalf("list after detach: %v", err)
	}
	if len(afterDetach) != 2 {
		t.Fatalf("references after detach = %d, want 2", len(afterDetach))
	}
	for _, reference := range afterDetach {
		if reference.FaceID == 2 {
			t.Errorf("detached face 2 still has a reference")
		}
	}

	// Disband: the person's set is gone.
	if err := repo.DeleteByPersonIDs([]uint{target.ID}); err != nil {
		t.Fatalf("delete target references: %v", err)
	}
	if _, err := repo.Medoid(target.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("medoid after disband = %v, want gorm.ErrRecordNotFound", err)
	}
}
