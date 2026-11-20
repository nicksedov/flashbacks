package database

import (
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"

	"gorm.io/gorm"
)

func mustCreate(t *testing.T, db *gorm.DB, value any) {
	t.Helper()
	if err := db.Create(value).Error; err != nil {
		t.Fatalf("create %T: %v", value, err)
	}
}

// TestMigratePersonNamesMergesLegacyDuplicates guards the startup repair: two
// people whose names are equal after normalization become one person under the
// lowest id, with both faces, the adopted cover and the union of the exemplars.
func TestMigratePersonNamesMergesLegacyDuplicates(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	firstImage := testutil.SeedImageFile(t, db, "/library/a.jpg", "hash-a", 10)
	secondImage := testutil.SeedImageFile(t, db, "/library/b.jpg", "hash-b", 10)

	// Legacy rows written before the repair: both carry an empty key, so only
	// the repair can see that the names are equal.
	older := domain.Person{Name: "Иван"}
	younger := domain.Person{Name: " иВАН "}
	mustCreate(t, db, &older)
	mustCreate(t, db, &younger)

	olderFace := domain.Face{
		ImageFileID: firstImage.ID, PersonID: older.ID,
		Width: 40, Height: 40, Confidence: 0.9,
	}
	youngerFace := domain.Face{
		ImageFileID: secondImage.ID, PersonID: younger.ID,
		Width: 40, Height: 40, Confidence: 0.8,
	}
	mustCreate(t, db, &olderFace)
	mustCreate(t, db, &youngerFace)

	cover := youngerFace.ID
	if err := db.Model(&domain.Person{}).Where("id = ?", younger.ID).
		Update("cover_face_id", cover).Error; err != nil {
		t.Fatalf("set younger cover: %v", err)
	}
	mustCreate(t, db, &domain.PersonReference{
		PersonID: older.ID, FaceID: olderFace.ID,
		Dimension: 3, Embedding: "[1,0,0]", Quality: 0.9,
	})
	mustCreate(t, db, &domain.PersonReference{
		PersonID: younger.ID, FaceID: youngerFace.ID,
		Dimension: 3, Embedding: "[0,1,0]", Quality: 0.8,
	})

	migratePersonNames(db)

	var people []domain.Person
	if err := db.Order("id").Find(&people).Error; err != nil {
		t.Fatalf("list people: %v", err)
	}
	if len(people) != 1 {
		t.Fatalf("people after repair = %d, want 1", len(people))
	}
	survivor := people[0]
	if survivor.ID != older.ID {
		t.Fatalf("survivor = %d, want the lowest id %d", survivor.ID, older.ID)
	}
	if survivor.Name != "Иван" {
		t.Errorf("survivor name = %q, want the original display name Иван", survivor.Name)
	}
	if survivor.NameKey != "иван" {
		t.Errorf("survivor key = %q, want иван", survivor.NameKey)
	}
	if !survivor.Manual {
		t.Errorf("survivor manual = false, want true so the job leaves it alone")
	}
	if survivor.CoverFaceID == nil || *survivor.CoverFaceID != cover {
		t.Errorf("survivor cover = %v, want the adopted cover %d", survivor.CoverFaceID, cover)
	}

	for _, faceID := range []uint{olderFace.ID, youngerFace.ID} {
		var face domain.Face
		if err := db.First(&face, faceID).Error; err != nil {
			t.Fatalf("reload face %d: %v", faceID, err)
		}
		if face.PersonID != older.ID {
			t.Errorf("face %d person = %d, want %d", faceID, face.PersonID, older.ID)
		}
		if !face.ManualAssignment {
			t.Errorf("face %d manualAssignment = false, want true", faceID)
		}
	}

	var references []domain.PersonReference
	if err := db.Where("person_id = ?", older.ID).Find(&references).Error; err != nil {
		t.Fatalf("list survivor references: %v", err)
	}
	if len(references) != 2 {
		t.Errorf("survivor exemplars = %d, want both faces", len(references))
	}
}

// TestMigratePersonNamesIsIdempotent guards that a second start on a repaired
// catalog changes nothing.
func TestMigratePersonNamesIsIdempotent(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	named := domain.Person{}
	named.SetName("Анна")
	mustCreate(t, db, &named)

	migratePersonNames(db)
	migratePersonNames(db)

	var stored domain.Person
	if err := db.First(&stored, named.ID).Error; err != nil {
		t.Fatalf("reload person: %v", err)
	}
	if stored.Name != "Анна" || stored.NameKey != "анна" {
		t.Errorf("person = (%q, %q), want (Анна, анна)", stored.Name, stored.NameKey)
	}
	if stored.Manual {
		t.Errorf("repair marked a clean person manual")
	}

	var people int64
	if err := db.Model(&domain.Person{}).Count(&people).Error; err != nil {
		t.Fatalf("count people: %v", err)
	}
	if people != 1 {
		t.Errorf("people = %d, want 1", people)
	}
}

// TestMigratePersonNamesKeepsUnnamedPeople guards that the repair leaves every
// unnamed person alone while it folds named duplicates.
func TestMigratePersonNamesKeepsUnnamedPeople(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	older := domain.Person{Name: "Иван"}
	younger := domain.Person{Name: "иван"}
	mustCreate(t, db, &older)
	mustCreate(t, db, &younger)
	for index := 0; index < 3; index++ {
		mustCreate(t, db, &domain.Person{})
	}

	migratePersonNames(db)

	var named int64
	if err := db.Model(&domain.Person{}).Where("name <> ''").Count(&named).Error; err != nil {
		t.Fatalf("count named people: %v", err)
	}
	if named != 1 {
		t.Errorf("named people = %d, want 1", named)
	}

	var unnamed int64
	if err := db.Model(&domain.Person{}).Where("name = ''").Count(&unnamed).Error; err != nil {
		t.Fatalf("count unnamed people: %v", err)
	}
	if unnamed != 3 {
		t.Errorf("unnamed people = %d, want 3", unnamed)
	}
}

// TestMigratePersonNamesCreatesUniqueIndex guards the storage-level guarantee:
// after the repair a duplicate key cannot be written even by code that bypasses
// the catalog, while unnamed people stay unbounded.
func TestMigratePersonNamesCreatesUniqueIndex(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// An earlier build indexed name_key twice; the plain btree must be gone once
	// the partial unique index exists.
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_people_name_key ON people (name_key)").Error; err != nil {
		t.Fatalf("create legacy index: %v", err)
	}

	migratePersonNames(db)

	if db.Migrator().HasIndex(&domain.Person{}, "idx_people_name_key") {
		t.Errorf("redundant index idx_people_name_key still exists")
	}
	if !db.Migrator().HasIndex(&domain.Person{}, "idx_people_name_key_unique") {
		t.Errorf("unique index idx_people_name_key_unique is missing")
	}

	first := domain.Person{}
	first.SetName("Иван")
	mustCreate(t, db, &first)

	second := domain.Person{}
	second.SetName(" иван ")
	if err := db.Create(&second).Error; err == nil {
		t.Fatalf("duplicate name key accepted, want a unique violation")
	}

	for index := 0; index < 3; index++ {
		mustCreate(t, db, &domain.Person{})
	}
}
