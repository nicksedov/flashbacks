package faces

import (
	"context"
	"testing"

	"github.com/flashbacks/core/internal/domain"
	facesclient "github.com/flashbacks/core/internal/infrastructure/faces"
	"github.com/flashbacks/core/internal/testutil"

	"gorm.io/gorm"
)

// clientWithSizeFloors is the fake faces service of the other tests, extended
// with the size floors it now advertises.
func clientWithSizeFloors(facePx, weakPx float64) *fakeFacesClient {
	client := newFakeFacesClient()
	client.HealthFunc = func(context.Context) (facesclient.Health, error) {
		return facesclient.Health{
			Status: "ok",
			Thresholds: facesclient.Thresholds{
				RecognitionMinSimilarity:      0.25,
				RecognitionMargin:             0.05,
				ClusterAssignSimilarity:       0.30,
				ClusterMergeSimilarity:        0.50,
				ClusterMergeSupport:           2,
				ClusterMergeSupportSimilarity: 0.40,
				MinClusterableFacePx:          facePx,
				WeakFacePx:                    weakPx,
			},
		}, nil
	}
	return client
}

// seedSizedFace stores one face of a person with an explicit box size and
// embedding identity.
func seedSizedFace(
	t *testing.T,
	db *gorm.DB,
	imageFileID, personID uint,
	width, height float32,
	manual bool,
) *domain.Face {
	t.Helper()
	face := seedFaceWithVector(t, db, imageFileID, identityVector(1))
	updates := map[string]any{
		"width":             width,
		"height":            height,
		"person_id":         personID,
		"manual_assignment": manual,
	}
	if err := db.Model(&domain.Face{}).Where("id = ?", face.ID).Updates(updates).Error; err != nil {
		t.Fatalf("size face %d: %v", face.ID, err)
	}
	return reloadFace(t, db, face.ID)
}

// reloadFace reads one face back from the database.
func reloadFace(t *testing.T, db *gorm.DB, faceID uint) *domain.Face {
	t.Helper()
	var face domain.Face
	if err := db.First(&face, faceID).Error; err != nil {
		t.Fatalf("reload face %d: %v", faceID, err)
	}
	return &face
}

// countPeople counts the stored people.
func countPeople(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&domain.Person{}).Count(&count).Error; err != nil {
		t.Fatalf("count people: %v", err)
	}
	return count
}

// TestDetachTooSmallFacesUnassignsAutoAssignments guards the core repair: a face
// below the service's floor is detached from its person, a full-size face beside
// it is left alone, and the person survives because it still holds a face.
func TestDetachTooSmallFacesUnassignsAutoAssignments(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	image := seedImage(t, db, "person-a-1")
	person := domain.Person{}
	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}
	big := seedSizedFace(t, db, image.ID, person.ID, 120, 140, false)
	small := seedSizedFace(t, db, image.ID, person.ID, 18, 22, false)

	m := reconcileManager(db, clientWithSizeFloors(32, 64))
	if detached := m.detachTooSmallFaces(); detached != 1 {
		t.Fatalf("detached %d faces, want 1", detached)
	}

	if got := reloadFace(t, db, big.ID).PersonID; got != person.ID {
		t.Errorf("full-size face person = %d, want %d", got, person.ID)
	}
	if got := reloadFace(t, db, small.ID).PersonID; got != 0 {
		t.Errorf("small face person = %d, want 0 (detached)", got)
	}
	if count := countPeople(t, db); count != 1 {
		t.Errorf("people = %d, want 1", count)
	}
}

// TestDetachTooSmallFacesKeepsManualAssignments guards the user's decision: a
// face the user attached by hand stays attached even when it is small, because
// overriding it would silently undo an explicit action.
func TestDetachTooSmallFacesKeepsManualAssignments(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	image := seedImage(t, db, "person-a-1")
	person := domain.Person{}
	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}
	big := seedSizedFace(t, db, image.ID, person.ID, 120, 140, false)
	manual := seedSizedFace(t, db, image.ID, person.ID, 18, 22, true)

	m := reconcileManager(db, clientWithSizeFloors(32, 64))
	if detached := m.detachTooSmallFaces(); detached != 0 {
		t.Fatalf("detached %d faces, want 0", detached)
	}

	if got := reloadFace(t, db, manual.ID).PersonID; got != person.ID {
		t.Errorf("manual face person = %d, want %d", got, person.ID)
	}
	if got := reloadFace(t, db, big.ID).PersonID; got != person.ID {
		t.Errorf("full-size face person = %d, want %d", got, person.ID)
	}
}

// TestDeleteEmptyPeopleRemovesGeneratedPeople guards the cleanup half: a person
// whose every face was detached disappears, while a person the user curated is
// kept even when empty.
func TestDeleteEmptyPeopleRemovesGeneratedPeople(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	image := seedImage(t, db, "person-a-1")
	generated := domain.Person{}
	if err := db.Create(&generated).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}
	seedSizedFace(t, db, image.ID, generated.ID, 18, 22, false)

	curated := domain.Person{Manual: true, Name: "Grandma"}
	if err := db.Create(&curated).Error; err != nil {
		t.Fatalf("create manual person: %v", err)
	}

	m := reconcileManager(db, clientWithSizeFloors(32, 64))
	if detached := m.detachTooSmallFaces(); detached != 1 {
		t.Fatalf("detached %d faces, want 1", detached)
	}
	if removed := m.deleteEmptyPeople(); removed != 1 {
		t.Fatalf("removed %d people, want 1", removed)
	}

	var people []domain.Person
	if err := db.Order("id").Find(&people).Error; err != nil {
		t.Fatalf("list people: %v", err)
	}
	if len(people) != 1 || people[0].ID != curated.ID {
		t.Fatalf("people = %+v, want only the curated person %d", people, curated.ID)
	}
}

// TestDetachTooSmallFacesIsIdempotent guards the repair path being safe to run
// on every full re-cluster: the second run finds nothing left to detach.
func TestDetachTooSmallFacesIsIdempotent(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	image := seedImage(t, db, "person-a-1")
	person := domain.Person{}
	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}
	seedSizedFace(t, db, image.ID, person.ID, 20, 24, false)

	m := reconcileManager(db, clientWithSizeFloors(32, 64))
	if detached := m.detachTooSmallFaces(); detached != 1 {
		t.Fatalf("first run detached %d faces, want 1", detached)
	}
	if detached := m.detachTooSmallFaces(); detached != 0 {
		t.Fatalf("second run detached %d faces, want 0", detached)
	}
}

// TestMinFaceSideIsOrientationInvariant guards the size metric: boxes are
// stored in the un-rotated frame, so a face on a rotated photo has its upright
// height in the box width. The smaller side is the size that stays right.
func TestMinFaceSideIsOrientationInvariant(t *testing.T) {
	if got := minFaceSide(18, 150); got != 18 {
		t.Errorf("minFaceSide(18, 150) = %v, want 18", got)
	}
	if got := minFaceSide(150, 18); got != 18 {
		t.Errorf("minFaceSide(150, 18) = %v, want 18", got)
	}
	if got := minFaceSide(0, 80); got != 80 {
		t.Errorf("minFaceSide(0, 80) = %v, want 80 (unknown side falls back)", got)
	}
}

// TestClusterAssignSkipsFacesBelowTheFloor guards the clustering input: a face
// smaller than the service's floor never reaches the label pass.
func TestClusterAssignSkipsFacesBelowTheFloor(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	image := seedImage(t, db, "person-a-1")
	big := seedSizedFace(t, db, image.ID, 0, 120, 140, false)
	seedSizedFace(t, db, image.ID, 0, 20, 24, false)

	client := clientWithSizeFloors(32, 64)
	var clustered int
	cluster := client.ClusterFunc
	client.ClusterFunc = func(
		ctx context.Context, embeddings [][]float32, minClusterSize int, soloFaces []bool,
	) ([]int, error) {
		clustered = len(embeddings)
		return cluster(ctx, embeddings, minClusterSize, soloFaces)
	}

	m := reconcileManager(db, client)
	m.clusterAssign(make(chan struct{}), map[uint]int{image.ID: 2})

	if clustered != 1 {
		t.Fatalf("clustered %d embeddings, want 1 (the small face was skipped)", clustered)
	}
	if got := reloadFace(t, db, big.ID).PersonID; got == 0 {
		t.Error("expected the full-size face to be clustered into a person")
	}
}

// TestClusterFromCandidatesSendsTheSmallerBoxSide guards the fact core reports:
// the service judges the smaller side, so a rotated box must not look large.
func TestClusterFromCandidatesSendsTheSmallerBoxSide(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	client := clientWithSizeFloors(32, 64)
	var widths []float32
	client.ClusterGraphFunc = func(
		_ context.Context, faceIDs []uint, _ []facesclient.FaceLink, _ []bool,
		facts facesclient.FaceFacts,
	) ([]int, error) {
		widths = facts.Widths
		return make([]int, len(faceIDs)), nil
	}

	m := reconcileManager(db, client)
	members := []faceRef{{faceID: 1, imageFileID: 10, width: 150, height: 18}}
	if _, err := m.clusterFromCandidates(
		context.Background(), members, []bool{true}, nil,
	); err != nil {
		t.Fatalf("cluster from candidates: %v", err)
	}
	if len(widths) != 1 || widths[0] != 18 {
		t.Fatalf("widths = %v, want [18] (the smaller box side)", widths)
	}
}
