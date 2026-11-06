package faces

import (
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	"github.com/flashbacks/core/internal/testutil"

	"gorm.io/gorm"
)

// identityVector returns a deterministic 512-dim vector for one identity, in the
// same shape the fake client's embeddings use.
func identityVector(value float32) []float32 {
	vector := make([]float32, 512)
	vector[0] = value
	return vector
}

// markScanned records that the current analyser produced the image's faces, so a
// pass does not re-analyse the fixture.
func markScanned(t *testing.T, db *gorm.DB, imageID uint) {
	t.Helper()
	if err := db.Model(&domain.ImageFile{}).Where("id = ?", imageID).
		Update("faces_scan_version", facesScanVersion).Error; err != nil {
		t.Fatalf("mark image scanned: %v", err)
	}
}

// storeReferences stores exemplars for a person from the given faces.
func storeReferences(t *testing.T, m *Manager, personID uint, faces ...domain.Face) {
	t.Helper()

	references := make([]domain.PersonReference, 0, len(faces))
	for _, face := range faces {
		var embedding domain.FaceEmbedding
		if err := m.db.Where("face_id = ?", face.ID).First(&embedding).Error; err != nil {
			t.Fatalf("load embedding of face %d: %v", face.ID, err)
		}
		vector, err := domain.ParseEmbedding(embedding.Embedding)
		if err != nil {
			t.Fatalf("parse embedding of face %d: %v", face.ID, err)
		}
		references = append(references, domain.PersonReference{
			FaceID:    face.ID,
			Dimension: len(vector),
			Embedding: domain.FormatEmbedding(vector),
			Quality:   face.Confidence,
		})
	}
	if err := m.personRefs.Replace(personID, references); err != nil {
		t.Fatalf("store references of person %d: %v", personID, err)
	}
}

// TestFaceRecognitionManager_DetachedFaceStaysUnassigned guards that clustering
// never picks up a face the user detached.
func TestFaceRecognitionManager_DetachedFaceStaysUnassigned(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	image := seedImage(t, db, "person-b")
	markScanned(t, db, image.ID)
	face := seedFaceWithVector(t, db, image.ID, identityVector(2))
	if err := db.Model(&domain.Face{}).Where("id = ?", face.ID).
		Update("manual_assignment", true).Error; err != nil {
		t.Fatalf("lock face: %v", err)
	}

	m := reconcileManager(db, newFakeFacesClient())
	m.runPass(nil)

	var stored domain.Face
	if err := db.First(&stored, face.ID).Error; err != nil {
		t.Fatalf("reload face: %v", err)
	}
	if stored.PersonID != 0 {
		t.Errorf("detached face person = %d, want 0", stored.PersonID)
	}
	var people int64
	db.Model(&domain.Person{}).Count(&people)
	if people != 0 {
		t.Errorf("people = %d, want 0 (the detached face must not become a person)", people)
	}
}

// TestFaceRecognitionManager_ManualFaceKeepsPerson guards that a user-assigned
// face keeps its person even when a later pass sees a closer identity.
func TestFaceRecognitionManager_ManualFaceKeepsPerson(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	chosen := domain.Person{Name: "Anna", Manual: true}
	if err := db.Create(&chosen).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}

	image := seedImage(t, db, "person-b")
	markScanned(t, db, image.ID)
	face := seedFaceWithVector(t, db, image.ID, identityVector(2))
	if err := db.Model(&domain.Face{}).Where("id = ?", face.ID).Updates(map[string]any{
		"person_id":         chosen.ID,
		"manual_assignment": true,
	}).Error; err != nil {
		t.Fatalf("assign face: %v", err)
	}

	// Another photo of the same identity arrives: it must not steal the face.
	later := seedImage(t, db, "person-b")
	m := reconcileManager(db, newFakeFacesClient())
	m.runPass(nil)

	var stored domain.Face
	if err := db.First(&stored, face.ID).Error; err != nil {
		t.Fatalf("reload face: %v", err)
	}
	if stored.PersonID != chosen.ID {
		t.Errorf("manual face person = %d, want %d", stored.PersonID, chosen.ID)
	}

	var laterFaces []domain.Face
	if err := db.Where("image_file_id = ?", later.ID).Find(&laterFaces).Error; err != nil {
		t.Fatalf("load later faces: %v", err)
	}
	if len(laterFaces) == 0 {
		t.Fatal("expected the later photo to be analysed")
	}
}

// TestFaceRecognitionManager_ManualPersonSurvivesReconcile guards that a curated
// person is not merged away by a reconcile pass, even when another person holds
// the same identity.
func TestFaceRecognitionManager_ManualPersonSurvivesReconcile(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	identity := identityVector(1)

	manual := domain.Person{Name: "Anna", Manual: true, ClusterVersion: "legacy"}
	generated := domain.Person{ClusterVersion: "legacy"}
	if err := db.Create(&manual).Error; err != nil {
		t.Fatalf("create manual person: %v", err)
	}
	if err := db.Create(&generated).Error; err != nil {
		t.Fatalf("create generated person: %v", err)
	}

	manualFaces := make([]domain.Face, 0, 2)
	generatedFaces := make([]domain.Face, 0, 2)
	for index := 0; index < 4; index++ {
		image := seedImage(t, db, "person-a-1")
		markScanned(t, db, image.ID)
		face := seedFaceWithVector(t, db, image.ID, identity)
		personID := manual.ID
		if index >= 2 {
			personID = generated.ID
		}
		if err := db.Model(&domain.Face{}).Where("id = ?", face.ID).
			Update("person_id", personID).Error; err != nil {
			t.Fatalf("assign face: %v", err)
		}
		if index < 2 {
			manualFaces = append(manualFaces, *face)
		} else {
			generatedFaces = append(generatedFaces, *face)
		}
	}

	m := reconcileManager(db, newFakeFacesClient())
	storeReferences(t, m, manual.ID, manualFaces...)
	storeReferences(t, m, generated.ID, generatedFaces...)

	m.runPass(nil)

	// The manual person keeps its faces and is not merged away.
	for _, face := range manualFaces {
		var stored domain.Face
		if err := db.First(&stored, face.ID).Error; err != nil {
			t.Fatalf("reload manual face: %v", err)
		}
		if stored.PersonID != manual.ID {
			t.Errorf("manual face %d person = %d, want %d", face.ID, stored.PersonID, manual.ID)
		}
	}

	var storedManual domain.Person
	if err := db.First(&storedManual, manual.ID).Error; err != nil {
		t.Errorf("manual person was removed: %v", err)
	}
	if !storedManual.Manual {
		t.Errorf("manual person lost its flag")
	}
	if storedManual.Name != "Anna" {
		t.Errorf("manual person name = %q, want Anna", storedManual.Name)
	}
}

// TestFaceRecognitionManager_DisbandedFacesStayUnassigned guards that a pass
// never re-clusters the faces a disbanded person left behind.
func TestFaceRecognitionManager_DisbandedFacesStayUnassigned(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	image := seedImage(t, db, "person-b")
	markScanned(t, db, image.ID)
	face := seedFaceWithVector(t, db, image.ID, identityVector(2))
	// Disbanding locks the faces as unassigned and removes the person.
	if err := db.Model(&domain.Face{}).Where("id = ?", face.ID).Updates(map[string]any{
		"person_id":         0,
		"manual_assignment": true,
	}).Error; err != nil {
		t.Fatalf("disband face: %v", err)
	}

	m := reconcileManager(db, newFakeFacesClient())
	m.runPass(nil)

	var stored domain.Face
	if err := db.First(&stored, face.ID).Error; err != nil {
		t.Fatalf("reload face: %v", err)
	}
	if stored.PersonID != 0 {
		t.Errorf("disbanded face person = %d, want 0", stored.PersonID)
	}
	var people int64
	db.Model(&domain.Person{}).Count(&people)
	if people != 0 {
		t.Errorf("people = %d, want 0 (a disbanded person must not be recreated)", people)
	}
}

// TestFaceRecognitionManager_ReclusterKeepsRenamedPerson guards that a full
// re-cluster keeps a person the user renamed, and also an unnamed person the
// user created from faces.
func TestFaceRecognitionManager_ReclusterKeepsRenamedPerson(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	first := seedImage(t, db, "person-a-1")
	second := seedImage(t, db, "person-b")
	markScanned(t, db, first.ID)
	markScanned(t, db, second.ID)
	faceA := seedFace(t, db, first.ID)
	faceB := seedFace(t, db, second.ID)

	renamed := seedPeople(t, db, "Аня", []uint{faceA.ID})
	created := seedPeople(t, db, "", []uint{faceB.ID})
	for _, person := range []*domain.Person{renamed, created} {
		if err := db.Model(&domain.Person{}).Where("id = ?", person.ID).
			Update("manual", true).Error; err != nil {
			t.Fatalf("mark person manual: %v", err)
		}
	}

	m := NewManagerWithDependencies(
		db, newFakeFacesClient(),
		Dependencies{PersonReferences: repository.NewPersonReferenceRepository(db)},
		Options{ReclusterAll: true},
	)
	m.runPass(nil)

	var storedA, storedB domain.Face
	db.First(&storedA, faceA.ID)
	db.First(&storedB, faceB.ID)
	if storedA.PersonID != renamed.ID {
		t.Errorf("renamed person's face = %d, want %d", storedA.PersonID, renamed.ID)
	}
	if storedB.PersonID != created.ID {
		t.Errorf("created person's face = %d, want %d", storedB.PersonID, created.ID)
	}

	var storedRenamed, storedCreated domain.Person
	if err := db.First(&storedRenamed, renamed.ID).Error; err != nil {
		t.Errorf("renamed person was removed: %v", err)
	}
	if err := db.First(&storedCreated, created.ID).Error; err != nil {
		t.Errorf("created person was removed: %v", err)
	}
}

// TestFaceRecognitionManager_MergedPersonRecognisedLater guards that a merge
// leaves exemplars behind: a later photo of a merged-away identity joins the
// merged person instead of creating a new one.
func TestFaceRecognitionManager_MergedPersonRecognisedLater(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	merged := domain.Person{Name: "Василиса", Manual: true}
	if err := db.Create(&merged).Error; err != nil {
		t.Fatalf("create merged person: %v", err)
	}

	source := seedImage(t, db, "person-a-1")
	markScanned(t, db, source.ID)
	sourceFace := seedFaceWithVector(t, db, source.ID, identityVector(1))
	if err := db.Model(&domain.Face{}).Where("id = ?", sourceFace.ID).Updates(map[string]any{
		"person_id":         merged.ID,
		"manual_assignment": true,
	}).Error; err != nil {
		t.Fatalf("assign source face: %v", err)
	}

	m := reconcileManager(db, newFakeFacesClient())
	storeReferences(t, m, merged.ID, *sourceFace)

	// A later photo of the merged-away identity.
	later := seedImage(t, db, "person-a-2")
	m.runPass(nil)

	var faces []domain.Face
	if err := db.Where("image_file_id = ?", later.ID).Find(&faces).Error; err != nil {
		t.Fatalf("load later faces: %v", err)
	}
	if len(faces) != 1 {
		t.Fatalf("later faces = %d, want 1", len(faces))
	}
	if faces[0].PersonID != merged.ID {
		t.Errorf("later face person = %d, want the merged person %d", faces[0].PersonID, merged.ID)
	}
	var people int64
	db.Model(&domain.Person{}).Count(&people)
	if people != 1 {
		t.Errorf("people = %d, want 1 (no new person for the merged identity)", people)
	}
}

// TestFaceRecognitionManager_CreatedPersonRecognisedLater guards that a person
// created from unassigned faces keeps exemplars, so a later photo of the same
// identity joins it.
func TestFaceRecognitionManager_CreatedPersonRecognisedLater(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	created := domain.Person{Name: "Василиса", Manual: true}
	if err := db.Create(&created).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}

	firstImage := seedImage(t, db, "person-b")
	secondImage := seedImage(t, db, "person-b")
	markScanned(t, db, firstImage.ID)
	markScanned(t, db, secondImage.ID)
	firstFace := seedFaceWithVector(t, db, firstImage.ID, identityVector(2))
	secondFace := seedFaceWithVector(t, db, secondImage.ID, identityVector(2))
	for _, face := range []*domain.Face{firstFace, secondFace} {
		if err := db.Model(&domain.Face{}).Where("id = ?", face.ID).Updates(map[string]any{
			"person_id":         created.ID,
			"manual_assignment": true,
		}).Error; err != nil {
			t.Fatalf("assign face: %v", err)
		}
	}

	m := reconcileManager(db, newFakeFacesClient())
	storeReferences(t, m, created.ID, *firstFace, *secondFace)

	// A later photo of the same identity.
	later := seedImage(t, db, "person-b")
	m.runPass(nil)

	var faces []domain.Face
	if err := db.Where("image_file_id = ?", later.ID).Find(&faces).Error; err != nil {
		t.Fatalf("load later faces: %v", err)
	}
	if len(faces) != 1 {
		t.Fatalf("later faces = %d, want 1", len(faces))
	}
	if faces[0].PersonID != created.ID {
		t.Errorf("later face person = %d, want the created person %d", faces[0].PersonID, created.ID)
	}
	var people int64
	db.Model(&domain.Person{}).Count(&people)
	if people != 1 {
		t.Errorf("people = %d, want 1", people)
	}
}
