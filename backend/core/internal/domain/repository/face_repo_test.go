package repository

import (
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"
)

// TestFaceRepositoryListUnassigned guards the unassigned-faces listing: only
// faces with person_id 0 are returned, ordered by ID, and pagination slices that
// order without skipping or repeating a face.
func TestFaceRepositoryListUnassigned(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	image := testutil.SeedImageFile(t, db, "/library/a.jpg", "hash-a", 10)
	person := domain.Person{Name: "Anna"}
	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}

	faces := []domain.Face{
		{ImageFileID: image.ID},
		{ImageFileID: image.ID, PersonID: person.ID},
		{ImageFileID: image.ID},
		{ImageFileID: image.ID, PersonID: person.ID},
		{ImageFileID: image.ID},
	}
	for index := range faces {
		if err := db.Create(&faces[index]).Error; err != nil {
			t.Fatalf("create face %d: %v", index, err)
		}
	}

	repo := NewFaceRepository(db)

	count, err := repo.CountUnassigned()
	if err != nil {
		t.Fatalf("count unassigned: %v", err)
	}
	if count != 3 {
		t.Errorf("unassigned count = %d, want 3", count)
	}

	firstPage, err := repo.ListUnassigned(0, 2)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(firstPage) != 2 {
		t.Fatalf("first page length = %d, want 2", len(firstPage))
	}
	if firstPage[0].ID != faces[0].ID || firstPage[1].ID != faces[2].ID {
		t.Errorf("first page ids = %d,%d, want %d,%d",
			firstPage[0].ID, firstPage[1].ID, faces[0].ID, faces[2].ID)
	}
	for _, face := range firstPage {
		if face.PersonID != 0 {
			t.Errorf("face %d personId = %d, want 0", face.ID, face.PersonID)
		}
	}

	secondPage, err := repo.ListUnassigned(2, 2)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(secondPage) != 1 || secondPage[0].ID != faces[4].ID {
		t.Fatalf("second page = %+v, want only face %d", secondPage, faces[4].ID)
	}

	// A person's face is never reported as unassigned.
	for _, face := range append(firstPage, secondPage...) {
		if face.PersonID != 0 {
			t.Errorf("assigned face %d leaked into the unassigned listing", face.ID)
		}
	}
}

// TestFaceRepositoryListUnassignedEmpty guards the empty state: no unassigned
// face yields an empty page rather than an error.
func TestFaceRepositoryListUnassignedEmpty(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	repo := NewFaceRepository(db)
	faces, err := repo.ListUnassigned(0, 10)
	if err != nil {
		t.Fatalf("list unassigned: %v", err)
	}
	if len(faces) != 0 {
		t.Errorf("unassigned faces = %d, want 0", len(faces))
	}
	count, err := repo.CountUnassigned()
	if err != nil {
		t.Fatalf("count unassigned: %v", err)
	}
	if count != 0 {
		t.Errorf("unassigned count = %d, want 0", count)
	}
}
