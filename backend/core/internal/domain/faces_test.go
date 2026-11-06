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
