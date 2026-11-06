package repository

import (
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"
)

// TestPersonRepositoryDelete guards the disband flow: the row is removed and
// the remaining people stay listed in ID order.
func TestPersonRepositoryDelete(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	first := domain.Person{Name: "Anna"}
	second := domain.Person{Name: "Василиса"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first person: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second person: %v", err)
	}

	repo := NewPersonRepository(db)
	if err := repo.Delete(first.ID); err != nil {
		t.Fatalf("delete person: %v", err)
	}

	people, err := repo.List()
	if err != nil {
		t.Fatalf("list people: %v", err)
	}
	if len(people) != 1 || people[0].ID != second.ID {
		t.Fatalf("people after delete = %+v, want only person %d", people, second.ID)
	}

	// Deleting an unknown id stays a no-op instead of surfacing an error.
	if err := repo.Delete(999); err != nil {
		t.Fatalf("delete unknown person: %v", err)
	}
}
