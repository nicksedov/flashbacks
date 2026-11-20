package domain_test

import (
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"

	"gorm.io/gorm"
)

// TestCurationColumnsMigrate guards the manual-curation flags: AutoMigrate must
// add both columns to an existing schema so the recognition job can read them
// without a data backfill.
func TestCurationColumnsMigrate(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	assertColumnExists(t, db, &domain.Person{}, "manual")
	assertColumnExists(t, db, &domain.Face{}, "manual_assignment")
}

// TestCurationColumnsDefaultToFalse verifies the additive migration: a row
// inserted without the flags is not treated as user-curated.
func TestCurationColumnsDefaultToFalse(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	person := domain.Person{}
	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}
	face := domain.Face{ImageFileID: 1}
	if err := db.Create(&face).Error; err != nil {
		t.Fatalf("create face: %v", err)
	}

	var storedPerson domain.Person
	if err := db.First(&storedPerson, person.ID).Error; err != nil {
		t.Fatalf("load person: %v", err)
	}
	if storedPerson.Manual {
		t.Errorf("new person manual = true, want false")
	}

	var storedFace domain.Face
	if err := db.First(&storedFace, face.ID).Error; err != nil {
		t.Fatalf("load face: %v", err)
	}
	if storedFace.ManualAssignment {
		t.Errorf("new face manualAssignment = true, want false")
	}
}

// TestPersonNameKey covers the canonical form every uniqueness decision is made
// on: surrounding whitespace is trimmed, internal runs collapse to one space and
// the letters are case-folded, so "Иван", "иван" and " Иван " are one name.
func TestPersonNameKey(t *testing.T) {
	cases := []struct {
		name        string
		wantDisplay string
		wantKey     string
	}{
		{name: "", wantDisplay: "", wantKey: ""},
		{name: "   ", wantDisplay: "", wantKey: ""},
		{name: "  Anna  ", wantDisplay: "Anna", wantKey: "anna"},
		{name: "Anna   Petrova", wantDisplay: "Anna Petrova", wantKey: "anna petrova"},
		{name: "Anna\tPetrova", wantDisplay: "Anna Petrova", wantKey: "anna petrova"},
		{name: "ИВАН", wantDisplay: "ИВАН", wantKey: "иван"},
		{name: " иВаН ", wantDisplay: "иВаН", wantKey: "иван"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			display, key := domain.NormalizePersonName(testCase.name)
			if display != testCase.wantDisplay {
				t.Errorf("display = %q, want %q", display, testCase.wantDisplay)
			}
			if key != testCase.wantKey {
				t.Errorf("key = %q, want %q", key, testCase.wantKey)
			}
			if got := domain.PersonNameKey(testCase.name); got != testCase.wantKey {
				t.Errorf("PersonNameKey = %q, want %q", got, testCase.wantKey)
			}
		})
	}
}

// TestPersonSetNameKeepsKeyInSync guards that the display name and its key are
// written together, which is what the partial unique index relies on.
func TestPersonSetNameKeepsKeyInSync(t *testing.T) {
	person := domain.Person{}
	person.SetName("  Анна   Петрова ")

	if person.Name != "Анна Петрова" {
		t.Errorf("name = %q, want %q", person.Name, "Анна Петрова")
	}
	if person.NameKey != "анна петрова" {
		t.Errorf("nameKey = %q, want %q", person.NameKey, "анна петрова")
	}

	person.SetName("   ")
	if person.Name != "" || person.NameKey != "" {
		t.Errorf("cleared name = (%q, %q), want both empty", person.Name, person.NameKey)
	}
}

// TestPersonNameKeyMigrates guards the additive migration: AutoMigrate must add
// the name_key column to an existing people table so the catalog can write it
// without a manual data migration.
func TestPersonNameKeyMigrates(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	assertColumnExists(t, db, &domain.Person{}, "name_key")
}

func assertColumnExists(t *testing.T, db *gorm.DB, model any, column string) {
	t.Helper()

	types, err := db.Migrator().ColumnTypes(model)
	if err != nil {
		t.Fatalf("read column types: %v", err)
	}
	for _, columnType := range types {
		if columnType.Name() == column {
			return
		}
	}
	t.Fatalf("column %q missing after AutoMigrate", column)
}
