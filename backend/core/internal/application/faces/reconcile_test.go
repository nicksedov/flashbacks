package faces

import (
	"context"
	"testing"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	"github.com/flashbacks/core/internal/infrastructure/llm"
	"github.com/flashbacks/core/internal/testutil"

	"gorm.io/gorm"
)

// reconcileManager builds a manager that stores and reads person references.
func reconcileManager(db *gorm.DB, client *fakeFacesClient) *Manager {
	return NewManagerWithDependencies(db, client, Dependencies{
		PersonReferences: repository.NewPersonReferenceRepository(db),
	}, DefaultOptions())
}

// seedFaceWithVector stores one face with an explicit embedding and marks its
// photo as scanned, so a pass does not try to analyse the fixture file.
func seedFaceWithVector(t *testing.T, db *gorm.DB, imageFileID uint, vector []float32) *domain.Face {
	t.Helper()
	face := domain.Face{ImageFileID: imageFileID, Confidence: 0.9}
	if err := db.Create(&face).Error; err != nil {
		t.Fatalf("create face: %v", err)
	}
	embedding := domain.FaceEmbedding{
		FaceID:    face.ID,
		Dimension: len(vector),
		Embedding: llm.Float32SliceToPgVector(vector),
		Model:     domain.FaceEmbeddingModel,
	}
	if err := db.Create(&embedding).Error; err != nil {
		t.Fatalf("create embedding: %v", err)
	}
	return &face
}

// TestFaceRecognitionManager_IdentityStableAcrossPasses guards that a known
// identity analysed in a later pass joins the person the first pass created
// instead of minting a new one.
func TestFaceRecognitionManager_IdentityStableAcrossPasses(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	first := seedImage(t, db, "person-a-1")
	m := reconcileManager(db, newFakeFacesClient())
	m.runPass(nil)

	var peopleAfterFirst []domain.Person
	if err := db.Order("id").Find(&peopleAfterFirst).Error; err != nil {
		t.Fatalf("list people: %v", err)
	}
	if len(peopleAfterFirst) != 1 {
		t.Fatalf("people after the first pass = %d, want 1", len(peopleAfterFirst))
	}
	personID := peopleAfterFirst[0].ID

	references, err := m.personRefs.List(personID)
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(references) == 0 {
		t.Fatal("expected the first pass to store a reference of the new person")
	}

	// A second photo of the same identity arrives in a later pass.
	second := seedImage(t, db, "person-a-2")
	m.runPass(nil)

	var faces []domain.Face
	if err := db.Where("image_file_id IN ?", []uint{first.ID, second.ID}).
		Order("id").Find(&faces).Error; err != nil {
		t.Fatalf("load faces: %v", err)
	}
	if len(faces) != 2 {
		t.Fatalf("face count = %d, want 2", len(faces))
	}
	if faces[0].PersonID != personID || faces[1].PersonID != personID {
		t.Errorf("person ids = %d and %d, want the person %d from the first pass",
			faces[0].PersonID, faces[1].PersonID, personID)
	}

	var people int64
	db.Model(&domain.Person{}).Count(&people)
	if people != 1 {
		t.Errorf("person count after two passes = %d, want 1", people)
	}
}

// TestFaceRecognitionManager_PeopleReconciled guards that people holding the
// faces of one identity converge into one person, keeping the oldest id.
func TestFaceRecognitionManager_PeopleReconciled(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	older := domain.Person{ClusterVersion: "legacy"}
	younger := domain.Person{ClusterVersion: "legacy"}
	if err := db.Create(&older).Error; err != nil {
		t.Fatalf("create older person: %v", err)
	}
	if err := db.Create(&younger).Error; err != nil {
		t.Fatalf("create younger person: %v", err)
	}

	// Four photos of one identity, stored as two people by an older
	// clustering generation.
	identity := make([]float32, 512)
	identity[0] = 1
	for index := 0; index < 4; index++ {
		image := seedImage(t, db, "person-a-1")
		face := seedFaceWithVector(t, db, image.ID, identity)
		personID := older.ID
		if index >= 2 {
			personID = younger.ID
		}
		if err := db.Model(&domain.Face{}).Where("id = ?", face.ID).
			Update("person_id", personID).Error; err != nil {
			t.Fatalf("assign face: %v", err)
		}
		if err := db.Model(&domain.ImageFile{}).Where("id = ?", image.ID).
			Update("faces_scan_version", facesScanVersion).Error; err != nil {
			t.Fatalf("mark image scanned: %v", err)
		}
	}

	m := reconcileManager(db, newFakeFacesClient())
	m.runPass(nil)

	var people []domain.Person
	if err := db.Order("id").Find(&people).Error; err != nil {
		t.Fatalf("list people: %v", err)
	}
	if len(people) != 1 {
		t.Fatalf("people after reconcile = %d, want 1", len(people))
	}
	if people[0].ID != older.ID {
		t.Errorf("surviving person = %d, want the oldest id %d", people[0].ID, older.ID)
	}
	if people[0].ClusterVersion != personClusterVersion {
		t.Errorf("cluster version = %q, want %q", people[0].ClusterVersion, personClusterVersion)
	}

	var assigned int64
	db.Model(&domain.Face{}).Where("person_id = ?", older.ID).Count(&assigned)
	if assigned != 4 {
		t.Errorf("faces on the surviving person = %d, want 4", assigned)
	}
}

// seedPersonWithFaces stores one person holding *count* faces with *vector*.
func seedPersonWithFaces(t *testing.T, db *gorm.DB, vector []float32, count int) *domain.Person {
	t.Helper()
	person := domain.Person{ClusterVersion: "legacy"}
	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}
	for index := 0; index < count; index++ {
		image := seedImage(t, db, "person-a-1")
		face := seedFaceWithVector(t, db, image.ID, vector)
		if err := db.Model(&domain.Face{}).Where("id = ?", face.ID).
			Update("person_id", person.ID).Error; err != nil {
			t.Fatalf("assign face: %v", err)
		}
		if err := db.Model(&domain.ImageFile{}).Where("id = ?", image.ID).
			Update("faces_scan_version", facesScanVersion).Error; err != nil {
			t.Fatalf("mark image scanned: %v", err)
		}
	}
	return &person
}

// sparseVector builds a 512-d vector whose first components are the given ones.
func sparseVector(values ...float32) []float32 {
	vector := make([]float32, 512)
	copy(vector, values)
	return vector
}

// TestFaceRecognitionManager_ReconcileDoesNotChainLookalikes guards that a
// person who looks like two different people cannot merge both of them into one
// gallery-wide person.
func TestFaceRecognitionManager_ReconcileDoesNotChainLookalikes(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// cos(A,B)=0.9, cos(B,C)~0.52, cos(A,C)=0.1: B is a bridge, not a shared
	// identity.
	first := seedPersonWithFaces(t, db, sparseVector(1, 0), 2)
	seedPersonWithFaces(t, db, sparseVector(0.9, 0.435), 2)
	third := seedPersonWithFaces(t, db, sparseVector(0.1, 0.995), 2)

	m := reconcileManager(db, newFakeFacesClient())
	m.reconcilePeople()

	var people []domain.Person
	if err := db.Order("id").Find(&people).Error; err != nil {
		t.Fatalf("list people: %v", err)
	}
	if len(people) != 2 {
		t.Fatalf("people after reconcile = %d, want 2 (only the mutual best pair merges)", len(people))
	}

	var merged int64
	if err := db.Model(&domain.Face{}).Where("person_id = ?", first.ID).
		Count(&merged).Error; err != nil {
		t.Fatalf("count merged faces: %v", err)
	}
	if merged != 4 {
		t.Errorf("faces on the merged person = %d, want 4", merged)
	}

	var bridge int64
	if err := db.Model(&domain.Face{}).Where("person_id = ?", third.ID).
		Count(&bridge).Error; err != nil {
		t.Fatalf("count bridge faces: %v", err)
	}
	if bridge != 2 {
		t.Errorf("faces on the lookalike person = %d, want 2 (unmerged)", bridge)
	}
}

// TestFaceRecognitionManager_ResetDropsReferences guards that a full re-cluster
// does not leave references pointing at people it deleted.
func TestFaceRecognitionManager_ResetDropsReferences(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	person := domain.Person{}
	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}
	repo := repository.NewPersonReferenceRepository(db)
	if err := repo.Replace(person.ID, []domain.PersonReference{{
		FaceID:    1,
		Dimension: 2,
		Embedding: domain.FormatEmbedding([]float32{1, 0}),
	}}); err != nil {
		t.Fatalf("store reference: %v", err)
	}

	m := reconcileManager(db, newFakeFacesClient())
	m.resetGeneratedPeople()

	grouped, err := repo.ListAll()
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(grouped) != 0 {
		t.Errorf("references after reset = %d, want 0", len(grouped))
	}
}

// TestFaceRecognitionManager_AmbiguousFaceUnassigned guards that a weak match
// to every existing person stays unassigned instead of guessing.
func TestFaceRecognitionManager_AmbiguousFaceUnassigned(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	person := domain.Person{ClusterVersion: personClusterVersion}
	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}

	reference := make([]float32, 512)
	reference[0] = 1
	personRepo := repository.NewPersonReferenceRepository(db)
	if err := personRepo.Replace(person.ID, []domain.PersonReference{{
		FaceID:    1,
		Dimension: len(reference),
		Embedding: domain.FormatEmbedding(reference),
	}}); err != nil {
		t.Fatalf("store reference: %v", err)
	}

	// A face weakly similar to the reference (cosine ~0.1) and the clustering
	// service labels it as noise.
	distant := make([]float32, 512)
	distant[0] = 0.1
	distant[1] = 0.99499
	image := seedImage(t, db, "person-b")
	face := seedFaceWithVector(t, db, image.ID, distant)
	if err := db.Model(&domain.ImageFile{}).Where("id = ?", image.ID).
		Update("faces_scan_version", facesScanVersion).Error; err != nil {
		t.Fatalf("mark image scanned: %v", err)
	}

	client := newFakeFacesClient()
	client.ClusterFunc = func(
		context.Context, [][]float32, int, []bool,
	) ([]int, error) {
		return []int{-1}, nil
	}
	m := reconcileManager(db, client)
	m.clusterAssign(nil, map[uint]int{})

	var stored domain.Face
	if err := db.First(&stored, face.ID).Error; err != nil {
		t.Fatalf("load face: %v", err)
	}
	if stored.PersonID != 0 {
		t.Errorf("person id = %d, want the face to stay unassigned", stored.PersonID)
	}

	var people int64
	db.Model(&domain.Person{}).Count(&people)
	if people != 1 {
		t.Errorf("person count = %d, want the existing person only", people)
	}
}
