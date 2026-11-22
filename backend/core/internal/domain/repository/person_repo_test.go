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

// TestPersonRepositoryListNamed guards the name-suggestion read: only people
// with a display name come back, in ID order, so an unnamed person never
// appears as a suggestion.
func TestPersonRepositoryListNamed(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	named := domain.Person{Name: "Anna"}
	unnamed := domain.Person{}
	other := domain.Person{Name: "Василиса"}
	for _, person := range []*domain.Person{&named, &unnamed, &other} {
		if err := db.Create(person).Error; err != nil {
			t.Fatalf("create person: %v", err)
		}
	}

	repo := NewPersonRepository(db)
	people, err := repo.ListNamed()
	if err != nil {
		t.Fatalf("list named people: %v", err)
	}

	if len(people) != 2 {
		t.Fatalf("named people = %+v, want 2", people)
	}
	if people[0].ID != named.ID || people[1].ID != other.ID {
		t.Fatalf("named people ids = [%d %d], want [%d %d]", people[0].ID, people[1].ID, named.ID, other.ID)
	}
}
