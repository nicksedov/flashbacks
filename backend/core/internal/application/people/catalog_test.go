package people

import (
	"errors"
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	"github.com/flashbacks/core/internal/testutil"

	"gorm.io/gorm"
)

// catalogFixture wires the catalog service to an in-memory SQLite database with
// the real repositories, the same way the service runs in production.
type catalogFixture struct {
	db         *gorm.DB
	catalog    *Catalog
	persons    repository.PersonRepository
	faces      repository.FaceRepository
	references repository.PersonReferenceRepository
}

func newCatalogFixture(t *testing.T) *catalogFixture {
	t.Helper()

	db, cleanup := testutil.NewTestDB(t)
	t.Cleanup(cleanup)

	persons := repository.NewPersonRepository(db)
	faces := repository.NewFaceRepository(db)
	references := repository.NewPersonReferenceRepository(db)
	images := repository.NewImageFileRepository(db)

	return &catalogFixture{
		db:         db,
		catalog:    NewCatalog(db, persons, faces, references, images),
		persons:    persons,
		faces:      faces,
		references: references,
	}
}

func (f *catalogFixture) createPerson(t *testing.T, name string) *domain.Person {
	t.Helper()

	person := domain.Person{Name: name}
	if err := f.db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}
	return &person
}

func (f *catalogFixture) createImage(t *testing.T, path string) *domain.ImageFile {
	t.Helper()
	return testutil.SeedImageFile(t, f.db, path, "hash-"+path, 10)
}

func (f *catalogFixture) createFace(
	t *testing.T, imageID, personID uint, confidence float32, vector []float32,
) *domain.Face {
	t.Helper()

	face := domain.Face{
		ImageFileID: imageID,
		PersonID:    personID,
		X:           1,
		Y:           2,
		Width:       30,
		Height:      40,
		Confidence:  confidence,
	}
	if err := f.db.Create(&face).Error; err != nil {
		t.Fatalf("create face: %v", err)
	}
	if len(vector) > 0 {
		embedding := domain.FaceEmbedding{
			FaceID:    face.ID,
			Dimension: len(vector),
			Embedding: domain.FormatEmbedding(vector),
			Model:     domain.FaceEmbeddingModel,
		}
		if err := f.db.Create(&embedding).Error; err != nil {
			t.Fatalf("create embedding: %v", err)
		}
	}
	return &face
}

func (f *catalogFixture) setReferences(t *testing.T, personID uint, faceIDs ...uint) {
	t.Helper()

	references := make([]domain.PersonReference, 0, len(faceIDs))
	for _, faceID := range faceIDs {
		references = append(references, domain.PersonReference{
			FaceID:    faceID,
			Dimension: 3,
			Embedding: domain.FormatEmbedding([]float32{1, 0, 0}),
			Quality:   0.9,
		})
	}
	if err := f.references.Replace(personID, references); err != nil {
		t.Fatalf("replace references of person %d: %v", personID, err)
	}
}

func (f *catalogFixture) referenceFaceIDs(t *testing.T, personID uint) []uint {
	t.Helper()

	references, err := f.references.List(personID)
	if err != nil {
		t.Fatalf("list references of person %d: %v", personID, err)
	}
	ids := make([]uint, 0, len(references))
	for _, reference := range references {
		ids = append(ids, reference.FaceID)
	}
	return ids
}

func TestCatalogRename(t *testing.T) {
	t.Run("trims the name and marks the person manual", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		person := fixture.createPerson(t, "")

		updated, err := fixture.catalog.Rename(person.ID, "  Anna  ")
		if err != nil {
			t.Fatalf("rename: %v", err)
		}
		if updated.Name != "Anna" {
			t.Errorf("name = %q, want %q", updated.Name, "Anna")
		}
		if !updated.Manual {
			t.Errorf("person.manual = false, want true")
		}
	})

	t.Run("clears an existing name", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		person := fixture.createPerson(t, "Василиса")

		updated, err := fixture.catalog.Rename(person.ID, "   ")
		if err != nil {
			t.Fatalf("rename: %v", err)
		}
		if updated.Name != "" {
			t.Errorf("name = %q, want empty", updated.Name)
		}
	})

	t.Run("rejects an over-long name", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		person := fixture.createPerson(t, "Anna")

		long := make([]rune, maxNameLength+1)
		for index := range long {
			long[index] = 'а'
		}
		if _, err := fixture.catalog.Rename(person.ID, string(long)); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("rename error = %v, want ErrInvalidInput", err)
		}

		stored, err := fixture.persons.FindByID(person.ID)
		if err != nil {
			t.Fatalf("reload person: %v", err)
		}
		if stored.Name != "Anna" {
			t.Errorf("name after rejected rename = %q, want Anna", stored.Name)
		}
	})

	t.Run("rejects an unknown person", func(t *testing.T) {
		fixture := newCatalogFixture(t)

		if _, err := fixture.catalog.Rename(999, "Anna"); !errors.Is(err, ErrPersonNotFound) {
			t.Fatalf("rename error = %v, want ErrPersonNotFound", err)
		}
	})

	t.Run("allows two people to share a name", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		first := fixture.createPerson(t, "")
		second := fixture.createPerson(t, "")

		if _, err := fixture.catalog.Rename(first.ID, "Anna"); err != nil {
			t.Fatalf("rename first: %v", err)
		}
		if _, err := fixture.catalog.Rename(second.ID, "Anna"); err != nil {
			t.Fatalf("rename second: %v", err)
		}

		people, err := fixture.persons.List()
		if err != nil {
			t.Fatalf("list people: %v", err)
		}
		if len(people) != 2 {
			t.Fatalf("people = %d, want 2", len(people))
		}
		for _, person := range people {
			if person.Name != "Anna" {
				t.Errorf("person %d name = %q, want Anna", person.ID, person.Name)
			}
		}
	})
}

// TestCatalogListUnassignedEmpty guards the empty state of the unassigned
// listing: no claimable face yields an empty page, not an error.
func TestCatalogListUnassignedEmpty(t *testing.T) {
	fixture := newCatalogFixture(t)
	image := fixture.createImage(t, "/library/a.jpg")
	person := fixture.createPerson(t, "Anna")
	fixture.createFace(t, image.ID, person.ID, 0.9, nil)

	page, err := fixture.catalog.ListUnassigned(1, 50)
	if err != nil {
		t.Fatalf("list unassigned: %v", err)
	}
	if page.TotalFaces != 0 || len(page.Faces) != 0 {
		t.Fatalf("unassigned page = %+v, want empty", page)
	}
	if page.HasNextPage {
		t.Errorf("hasNextPage = true, want false")
	}
}

func TestCatalogMerge(t *testing.T) {
	t.Run("moves faces and counts a shared photo once", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		sharedImage := fixture.createImage(t, "/library/shared.jpg")
		otherImage := fixture.createImage(t, "/library/other.jpg")

		target := fixture.createPerson(t, "")
		source := fixture.createPerson(t, "")

		targetFace := fixture.createFace(t, sharedImage.ID, target.ID, 0.9, nil)
		sourceSharedFace := fixture.createFace(t, sharedImage.ID, source.ID, 0.8, nil)
		sourceFace := fixture.createFace(t, otherImage.ID, source.ID, 0.7, nil)
		fixture.setReferences(t, target.ID, targetFace.ID)
		fixture.setReferences(t, source.ID, targetFace.ID, sourceSharedFace.ID, sourceFace.ID)

		name := "  Василиса  "
		merged, err := fixture.catalog.Merge(target.ID, []uint{source.ID}, &name)
		if err != nil {
			t.Fatalf("merge: %v", err)
		}
		if merged.Name != "Василиса" {
			t.Errorf("merged name = %q, want Василиса", merged.Name)
		}
		if !merged.Manual {
			t.Errorf("merged person manual = false, want true")
		}

		// The source is gone and its faces reference the target.
		if _, err := fixture.persons.FindByID(source.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("source person still exists: %v", err)
		}
		for _, faceID := range []uint{sourceSharedFace.ID, sourceFace.ID} {
			face, err := fixture.faces.FindByID(faceID)
			if err != nil {
				t.Fatalf("reload face %d: %v", faceID, err)
			}
			if face.PersonID != target.ID {
				t.Errorf("face %d person = %d, want %d", faceID, face.PersonID, target.ID)
			}
			if !face.ManualAssignment {
				t.Errorf("face %d manualAssignment = false, want true", faceID)
			}
		}

		// The photo showing both identities counts once.
		photoCount, err := fixture.faces.CountImageFilesByPersonID(target.ID)
		if err != nil {
			t.Fatalf("count photos: %v", err)
		}
		if photoCount != 2 {
			t.Errorf("merged photo count = %d, want 2", photoCount)
		}

		// The exemplars are the union, without duplicates, and now belong to the
		// target.
		stored, err := fixture.references.List(target.ID)
		if err != nil {
			t.Fatalf("list merged references: %v", err)
		}
		seen := make(map[uint]bool, len(stored))
		for _, reference := range stored {
			if reference.PersonID != target.ID {
				t.Errorf("reference person = %d, want %d", reference.PersonID, target.ID)
			}
			if seen[reference.FaceID] {
				t.Errorf("duplicate reference for face %d", reference.FaceID)
			}
			seen[reference.FaceID] = true
		}
		if len(stored) != 3 {
			t.Errorf("merged references = %d, want 3", len(stored))
		}
		if remaining, err := fixture.references.List(source.ID); err != nil {
			t.Fatalf("list source references: %v", err)
		} else if len(remaining) != 0 {
			t.Errorf("source references = %d, want 0", len(remaining))
		}
	})

	t.Run("keeps the target name when no name is sent", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		target := fixture.createPerson(t, "Anna")
		source := fixture.createPerson(t, "Василиса")

		merged, err := fixture.catalog.Merge(target.ID, []uint{source.ID}, nil)
		if err != nil {
			t.Fatalf("merge: %v", err)
		}
		if merged.Name != "Anna" {
			t.Errorf("merged name = %q, want Anna", merged.Name)
		}
	})

	t.Run("falls back to the first source cover", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		image := fixture.createImage(t, "/library/a.jpg")

		target := fixture.createPerson(t, "")
		source := fixture.createPerson(t, "")
		sourceFace := fixture.createFace(t, image.ID, source.ID, 0.7, nil)
		cover := sourceFace.ID
		if err := fixture.db.Model(&domain.Person{}).Where("id = ?", source.ID).
			Update("cover_face_id", cover).Error; err != nil {
			t.Fatalf("set source cover: %v", err)
		}

		merged, err := fixture.catalog.Merge(target.ID, []uint{source.ID}, nil)
		if err != nil {
			t.Fatalf("merge: %v", err)
		}
		if merged.CoverFaceID == nil || *merged.CoverFaceID != cover {
			t.Errorf("merged cover = %v, want %d", merged.CoverFaceID, cover)
		}
	})

	t.Run("rejects an empty source list", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		target := fixture.createPerson(t, "")

		if _, err := fixture.catalog.Merge(target.ID, nil, nil); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("merge error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("rejects the target among the sources", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		target := fixture.createPerson(t, "")

		if _, err := fixture.catalog.Merge(target.ID, []uint{target.ID}, nil); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("merge error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("rejects more than fifty sources", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		target := fixture.createPerson(t, "")

		sources := make([]uint, maxMergeSources+1)
		for index := range sources {
			sources[index] = uint(index) + 2
		}
		if _, err := fixture.catalog.Merge(target.ID, sources, nil); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("merge error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("rejects an unknown source without changing anything", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		image := fixture.createImage(t, "/library/a.jpg")
		target := fixture.createPerson(t, "Anna")
		fixture.createFace(t, image.ID, target.ID, 0.9, nil)

		if _, err := fixture.catalog.Merge(target.ID, []uint{999}, nil); !errors.Is(err, ErrPersonNotFound) {
			t.Fatalf("merge error = %v, want ErrPersonNotFound", err)
		}

		stored, err := fixture.persons.FindByID(target.ID)
		if err != nil {
			t.Fatalf("reload target: %v", err)
		}
		if stored.Manual {
			t.Errorf("failed merge marked the target manual")
		}
		photoCount, err := fixture.faces.CountImageFilesByPersonID(target.ID)
		if err != nil {
			t.Fatalf("count photos: %v", err)
		}
		if photoCount != 1 {
			t.Errorf("target photo count = %d, want 1", photoCount)
		}
	})
}

func TestCatalogCreateFromFaces(t *testing.T) {
	t.Run("builds a named person from unassigned faces", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		image := fixture.createImage(t, "/library/a.jpg")
		otherImage := fixture.createImage(t, "/library/b.jpg")
		first := fixture.createFace(t, image.ID, 0, 0.9, []float32{1, 0, 0})
		second := fixture.createFace(t, otherImage.ID, 0, 0.8, []float32{0, 1, 0})

		person, err := fixture.catalog.CreateFromFaces("Василиса", []uint{first.ID, second.ID})
		if err != nil {
			t.Fatalf("create person: %v", err)
		}
		if person.Name != "Василиса" {
			t.Errorf("name = %q, want Василиса", person.Name)
		}
		if !person.Manual {
			t.Errorf("manual = false, want true")
		}

		for _, faceID := range []uint{first.ID, second.ID} {
			face, err := fixture.faces.FindByID(faceID)
			if err != nil {
				t.Fatalf("reload face %d: %v", faceID, err)
			}
			if face.PersonID != person.ID {
				t.Errorf("face %d person = %d, want %d", faceID, face.PersonID, person.ID)
			}
			if !face.ManualAssignment {
				t.Errorf("face %d manualAssignment = false, want true", faceID)
			}
		}

		// Exemplars seed from the faces' embeddings.
		if ids := fixture.referenceFaceIDs(t, person.ID); len(ids) != 2 {
			t.Errorf("references = %v, want both faces", ids)
		}
		if person.CoverFaceID == nil {
			t.Errorf("cover = nil, want a face")
		}
	})

	t.Run("creates an unnamed person", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		image := fixture.createImage(t, "/library/a.jpg")
		face := fixture.createFace(t, image.ID, 0, 0.9, []float32{1, 0, 0})

		person, err := fixture.catalog.CreateFromFaces("", []uint{face.ID})
		if err != nil {
			t.Fatalf("create person: %v", err)
		}
		if person.Name != "" {
			t.Errorf("name = %q, want empty", person.Name)
		}
	})

	t.Run("rejects an empty face list", func(t *testing.T) {
		fixture := newCatalogFixture(t)

		if _, err := fixture.catalog.CreateFromFaces("Anna", nil); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("create error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("rejects an unknown face", func(t *testing.T) {
		fixture := newCatalogFixture(t)

		if _, err := fixture.catalog.CreateFromFaces("Anna", []uint{999}); !errors.Is(err, ErrFaceNotFound) {
			t.Fatalf("create error = %v, want ErrFaceNotFound", err)
		}
	})

	t.Run("rejects an already-assigned face", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		image := fixture.createImage(t, "/library/a.jpg")
		person := fixture.createPerson(t, "Anna")
		face := fixture.createFace(t, image.ID, person.ID, 0.9, []float32{1, 0, 0})

		if _, err := fixture.catalog.CreateFromFaces("", []uint{face.ID}); !errors.Is(err, ErrFaceNotFound) {
			t.Fatalf("create error = %v, want ErrFaceNotFound", err)
		}

		people, err := fixture.persons.List()
		if err != nil {
			t.Fatalf("list people: %v", err)
		}
		if len(people) != 1 {
			t.Errorf("people after rejected create = %d, want 1", len(people))
		}
	})
}

func TestCatalogDisband(t *testing.T) {
	fixture := newCatalogFixture(t)
	firstImage := fixture.createImage(t, "/library/a.jpg")
	secondImage := fixture.createImage(t, "/library/b.jpg")
	person := fixture.createPerson(t, "Василиса")

	faces := []*domain.Face{
		fixture.createFace(t, firstImage.ID, person.ID, 0.9, []float32{1, 0, 0}),
		fixture.createFace(t, firstImage.ID, person.ID, 0.8, []float32{1, 0, 0}),
		fixture.createFace(t, secondImage.ID, person.ID, 0.7, []float32{1, 0, 0}),
	}
	fixture.setReferences(t, person.ID, faces[0].ID, faces[1].ID)

	if err := fixture.catalog.Disband(person.ID); err != nil {
		t.Fatalf("disband: %v", err)
	}

	if _, err := fixture.persons.FindByID(person.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("person still exists after disband: %v", err)
	}
	for _, created := range faces {
		face, err := fixture.faces.FindByID(created.ID)
		if err != nil {
			t.Fatalf("disband deleted face %d: %v", created.ID, err)
		}
		if face.PersonID != 0 {
			t.Errorf("face %d person = %d, want 0", created.ID, face.PersonID)
		}
		if !face.ManualAssignment {
			t.Errorf("face %d manualAssignment = false, want true", created.ID)
		}
	}
	if _, err := fixture.faces.FindByID(faces[0].ID); err != nil {
		t.Fatalf("reload face: %v", err)
	}
	for _, image := range []*domain.ImageFile{firstImage, secondImage} {
		var stored domain.ImageFile
		if err := fixture.db.First(&stored, image.ID).Error; err != nil {
			t.Errorf("disband deleted photo %d: %v", image.ID, err)
		}
	}
	if ids := fixture.referenceFaceIDs(t, person.ID); len(ids) != 0 {
		t.Errorf("references after disband = %v, want none", ids)
	}

	if err := fixture.catalog.Disband(999); !errors.Is(err, ErrPersonNotFound) {
		t.Errorf("disband unknown person = %v, want ErrPersonNotFound", err)
	}
}

func TestCatalogAssignFace(t *testing.T) {
	t.Run("assigns a face to a person and locks it", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		image := fixture.createImage(t, "/library/a.jpg")
		person := fixture.createPerson(t, "Василиса")
		face := fixture.createFace(t, image.ID, 0, 0.9, []float32{1, 0, 0})

		updated, err := fixture.catalog.AssignFace(face.ID, person.ID)
		if err != nil {
			t.Fatalf("assign face: %v", err)
		}
		if updated.PersonID != person.ID {
			t.Errorf("personId = %d, want %d", updated.PersonID, person.ID)
		}
		if !updated.ManualAssignment {
			t.Errorf("manualAssignment = false, want true")
		}
		if ids := fixture.referenceFaceIDs(t, person.ID); len(ids) != 1 || ids[0] != face.ID {
			t.Errorf("person references = %v, want [%d]", ids, face.ID)
		}
	})

	t.Run("detaches a face and drops its reference", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		image := fixture.createImage(t, "/library/a.jpg")
		person := fixture.createPerson(t, "Василиса")
		other := fixture.createFace(t, image.ID, person.ID, 0.8, []float32{0, 1, 0})
		face := fixture.createFace(t, image.ID, person.ID, 0.9, []float32{1, 0, 0})
		fixture.setReferences(t, person.ID, other.ID, face.ID)

		updated, err := fixture.catalog.AssignFace(face.ID, 0)
		if err != nil {
			t.Fatalf("detach face: %v", err)
		}
		if updated.PersonID != 0 {
			t.Errorf("personId = %d, want 0", updated.PersonID)
		}
		if !updated.ManualAssignment {
			t.Errorf("manualAssignment = false, want true")
		}
		if ids := fixture.referenceFaceIDs(t, person.ID); len(ids) != 1 || ids[0] != other.ID {
			t.Errorf("references after detach = %v, want [%d]", ids, other.ID)
		}
	})

	t.Run("promotes a new cover when the cover face leaves", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		image := fixture.createImage(t, "/library/a.jpg")
		person := fixture.createPerson(t, "Василиса")
		cover := fixture.createFace(t, image.ID, person.ID, 0.9, []float32{1, 0, 0})
		promoted := fixture.createFace(t, image.ID, person.ID, 0.8, []float32{0, 1, 0})
		if err := fixture.db.Model(&domain.Person{}).Where("id = ?", person.ID).
			Update("cover_face_id", cover.ID).Error; err != nil {
			t.Fatalf("set cover: %v", err)
		}

		if _, err := fixture.catalog.AssignFace(cover.ID, 0); err != nil {
			t.Fatalf("detach cover face: %v", err)
		}

		stored, err := fixture.persons.FindByID(person.ID)
		if err != nil {
			t.Fatalf("reload person: %v", err)
		}
		if stored.CoverFaceID == nil || *stored.CoverFaceID != promoted.ID {
			t.Errorf("cover after detach = %v, want %d", stored.CoverFaceID, promoted.ID)
		}
	})

	t.Run("rejects an unknown person", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		image := fixture.createImage(t, "/library/a.jpg")
		face := fixture.createFace(t, image.ID, 0, 0.9, nil)

		if _, err := fixture.catalog.AssignFace(face.ID, 999); !errors.Is(err, ErrPersonNotFound) {
			t.Fatalf("assign error = %v, want ErrPersonNotFound", err)
		}
		stored, err := fixture.faces.FindByID(face.ID)
		if err != nil {
			t.Fatalf("reload face: %v", err)
		}
		if stored.PersonID != 0 {
			t.Errorf("face person = %d, want 0", stored.PersonID)
		}
	})

	t.Run("rejects an unknown face", func(t *testing.T) {
		fixture := newCatalogFixture(t)
		person := fixture.createPerson(t, "Василиса")

		if _, err := fixture.catalog.AssignFace(999, person.ID); !errors.Is(err, ErrFaceNotFound) {
			t.Fatalf("assign error = %v, want ErrFaceNotFound", err)
		}
	})
}

func TestCatalogListUnassigned(t *testing.T) {
	fixture := newCatalogFixture(t)
	person := fixture.createPerson(t, "Anna")

	// A face whose photo vanished must not be listed: there is no thumbnail to
	// show for it.
	vanishedImage := fixture.createImage(t, "/library/vanished.jpg")
	fixture.createFace(t, vanishedImage.ID, 0, 0.6, nil)
	if err := fixture.db.Delete(&domain.ImageFile{}, vanishedImage.ID).Error; err != nil {
		t.Fatalf("delete image: %v", err)
	}

	liveImage := fixture.createImage(t, "/library/live.jpg")
	live := []*domain.Face{
		fixture.createFace(t, liveImage.ID, 0, 0.95, nil),
		fixture.createFace(t, liveImage.ID, 0, 0.85, nil),
		fixture.createFace(t, liveImage.ID, 0, 0.75, nil),
	}
	fixture.createFace(t, liveImage.ID, person.ID, 0.99, nil)

	first, err := fixture.catalog.ListUnassigned(1, 2)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if first.TotalFaces != 3 {
		t.Errorf("total faces = %d, want 3", first.TotalFaces)
	}
	if len(first.Faces) != 2 {
		t.Errorf("first page length = %d, want 2", len(first.Faces))
	}
	if !first.HasNextPage {
		t.Errorf("first page hasNextPage = false, want true")
	}
	if first.TotalPages != 2 {
		t.Errorf("totalPages = %d, want 2", first.TotalPages)
	}
	if first.Faces[0].Face.ID != live[0].ID {
		t.Errorf("first face = %d, want %d", first.Faces[0].Face.ID, live[0].ID)
	}
	if first.Faces[0].Path != liveImage.Path {
		t.Errorf("path = %q, want %q", first.Faces[0].Path, liveImage.Path)
	}
	if first.Faces[0].FileName != "live.jpg" {
		t.Errorf("fileName = %q, want live.jpg", first.Faces[0].FileName)
	}
	if first.Faces[0].Face.Confidence != 0.95 {
		t.Errorf("confidence = %v, want 0.95", first.Faces[0].Face.Confidence)
	}
	if first.Faces[0].Face.Width != 30 || first.Faces[0].Face.Height != 40 {
		t.Errorf("box = %vx%v, want 30x40", first.Faces[0].Face.Width, first.Faces[0].Face.Height)
	}

	second, err := fixture.catalog.ListUnassigned(2, 2)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(second.Faces) != 1 || second.Faces[0].Face.ID != live[2].ID {
		t.Fatalf("second page = %+v, want face %d", second.Faces, live[2].ID)
	}
	if second.HasNextPage {
		t.Errorf("second page hasNextPage = true, want false")
	}

	empty, err := fixture.catalog.ListUnassigned(1, 50)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if empty.TotalFaces != 3 {
		t.Errorf("total faces = %d, want 3", empty.TotalFaces)
	}
}
