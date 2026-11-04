package faces

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/flashbacks/core/internal/domain"
	facesclient "github.com/flashbacks/core/internal/infrastructure/faces"
	"github.com/flashbacks/core/internal/testutil"
	"github.com/flashbacks/core/internal/testutil/mocks"

	"gorm.io/gorm"
)

// fakeFacesClient builds a deterministic client whose analysis and cluster
// labels are keyed by the image content (the file bytes), so tests control
// identity.
type fakeFacesClient struct {
	*mocks.MockFacesClient
}

func newFakeFacesClient() *fakeFacesClient {
	mock := &mocks.MockFacesClient{}
	mock.AnalyzeFunc = func(ctx context.Context, image []byte) ([]facesclient.AnalyzedFace, error) {
		if string(image) == "noface" {
			return nil, nil
		}
		vec := make([]float32, 512)
		switch string(image) {
		case "person-a-1", "person-a-2":
			vec[0] = 1
		case "person-b":
			vec[0] = 2
		}
		return []facesclient.AnalyzedFace{mocks.SingleAnalyzedFace(vec)}, nil
	}
	mock.ClusterFunc = clusterByFirstComponent()
	return &fakeFacesClient{MockFacesClient: mock}
}

// clusterByFirstComponent labels every embedding by its first component, which
// the fakes use to encode identity (0 is noise).
func clusterByFirstComponent() func(context.Context, [][]float32, int, []bool) ([]int, error) {
	return func(_ context.Context, embeddings [][]float32, _ int, _ []bool) ([]int, error) {
		labels := make([]int, len(embeddings))
		for i, e := range embeddings {
			if len(e) > 0 && e[0] > 0 {
				labels[i] = int(e[0]) - 1
			} else {
				labels[i] = -1
			}
		}
		return labels, nil
	}
}

func seedImage(t *testing.T, db *gorm.DB, content string) *domain.ImageFile {
	t.Helper()
	path := filepath.Join(t.TempDir(), content+".jpg")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return testutil.SeedImageFile(t, db, path, content, int64(len(content)))
}

func newTestManager(t *testing.T, db *gorm.DB) *Manager {
	t.Helper()
	return NewManager(db, newFakeFacesClient())
}

// TestFaceRecognitionManager_FaceAssignedToPerson guards the core scenario: a
// photo with exactly one face yields a face record referencing exactly one
// person.
func TestFaceRecognitionManager_FaceAssignedToPerson(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	image := seedImage(t, db, "person-a-1")
	m := newTestManager(t, db)
	m.runPass(nil)

	var faces []domain.Face
	if err := db.Where("image_file_id = ?", image.ID).Find(&faces).Error; err != nil {
		t.Fatalf("query faces: %v", err)
	}
	if len(faces) != 1 {
		t.Fatalf("face count = %d, want 1", len(faces))
	}
	if faces[0].PersonID == 0 {
		t.Fatal("expected the detected face to reference a person")
	}

	var people int64
	db.Model(&domain.Person{}).Count(&people)
	if people != 1 {
		t.Errorf("person count = %d, want 1", people)
	}
}

// TestFaceRecognitionManager_SamePersonGrouped guards that the same face across
// two photos shares one person.
func TestFaceRecognitionManager_SamePersonGrouped(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	imgA1 := seedImage(t, db, "person-a-1")
	imgA2 := seedImage(t, db, "person-a-2")
	imgB := seedImage(t, db, "person-b")

	m := newTestManager(t, db)
	m.runPass(nil)

	var faceA1, faceA2, faceB domain.Face
	db.Where("image_file_id = ?", imgA1.ID).First(&faceA1)
	db.Where("image_file_id = ?", imgA2.ID).First(&faceA2)
	db.Where("image_file_id = ?", imgB.ID).First(&faceB)

	if faceA1.PersonID == 0 || faceA2.PersonID == 0 || faceB.PersonID == 0 {
		t.Fatal("expected every face to be assigned to a person")
	}
	if faceA1.PersonID != faceA2.PersonID {
		t.Errorf("same-person faces got different people: %d vs %d", faceA1.PersonID, faceA2.PersonID)
	}
	if faceA1.PersonID == faceB.PersonID {
		t.Errorf("different-person faces got the same person %d", faceA1.PersonID)
	}
}

// TestFaceRecognitionManager_NoFaceSkipped guards that a photo without a
// detectable face creates no face record and the job advances.
func TestFaceRecognitionManager_NoFaceSkipped(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	imgNoFace := seedImage(t, db, "noface")
	imgFace := seedImage(t, db, "person-a-1")

	m := newTestManager(t, db)
	m.runPass(nil)

	var noFaceCount int64
	db.Model(&domain.Face{}).Where("image_file_id = ?", imgNoFace.ID).Count(&noFaceCount)
	if noFaceCount != 0 {
		t.Errorf("face count for no-face photo = %d, want 0", noFaceCount)
	}

	var faceCount int64
	db.Model(&domain.Face{}).Where("image_file_id = ?", imgFace.ID).Count(&faceCount)
	if faceCount != 1 {
		t.Errorf("face count for face photo = %d, want 1", faceCount)
	}

	status := m.GetStatus()
	if status.Processed != 2 {
		t.Errorf("processed = %d, want 2", status.Processed)
	}
}

// TestFaceRecognitionManager_UsesCalibratedClusterMinimum guards that the job
// asks the faces service to group on similarity alone: a person must form from
// two faces. A higher minimum labels every face of a small gallery as noise and
// leaves the People view empty although detection and embedding succeeded.
func TestFaceRecognitionManager_UsesCalibratedClusterMinimum(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	client := newFakeFacesClient()
	clusterCalls := 0
	receivedMinimum := -1
	var receivedSolo []bool
	inner := client.ClusterFunc
	client.ClusterFunc = func(ctx context.Context, embeddings [][]float32, minClusterSize int, soloFaces []bool) ([]int, error) {
		clusterCalls++
		receivedMinimum = minClusterSize
		receivedSolo = soloFaces
		return inner(ctx, embeddings, minClusterSize, soloFaces)
	}

	imgA1 := seedImage(t, db, "person-a-1")
	imgA2 := seedImage(t, db, "person-a-2")

	NewManager(db, client).runPass(nil)

	if clusterCalls == 0 {
		t.Fatal("expected the job to cluster the detected faces")
	}
	if receivedMinimum != 2 {
		t.Errorf("minClusterSize = %d, want 2 (a person photographed twice must cluster)", receivedMinimum)
	}
	if len(receivedSolo) != 2 || !receivedSolo[0] || !receivedSolo[1] {
		t.Errorf("soloFaces = %v, want both faces of one-face photos marked solo", receivedSolo)
	}

	var faceA1, faceA2 domain.Face
	db.Where("image_file_id = ?", imgA1.ID).First(&faceA1)
	db.Where("image_file_id = ?", imgA2.ID).First(&faceA2)
	if faceA1.PersonID == 0 || faceA2.PersonID == 0 {
		t.Fatal("expected both faces to be assigned to a person")
	}
	if faceA1.PersonID != faceA2.PersonID {
		t.Errorf("same-person faces got different people: %d vs %d", faceA1.PersonID, faceA2.PersonID)
	}
}

// newGroupAwareFacesClient detects two faces on the "group" fixture and one on
// every other image, embedding all of them as the same identity.
func newGroupAwareFacesClient() *mocks.MockFacesClient {
	mock := &mocks.MockFacesClient{}
	mock.AnalyzeFunc = func(ctx context.Context, image []byte) ([]facesclient.AnalyzedFace, error) {
		vec := make([]float32, 512)
		vec[0] = 1
		if string(image) == "group" {
			return []facesclient.AnalyzedFace{
				{
					FaceBox:   facesclient.FaceBox{X: 10, Y: 10, Width: 40, Height: 40, Confidence: 0.99},
					Embedding: vec,
				},
				{
					FaceBox:   facesclient.FaceBox{X: 60, Y: 10, Width: 40, Height: 40, Confidence: 0.98},
					Embedding: vec,
				},
			}, nil
		}
		return []facesclient.AnalyzedFace{mocks.SingleAnalyzedFace(vec)}, nil
	}
	mock.ClusterFunc = clusterByFirstComponent()
	return mock
}

// TestFaceRecognitionManager_CoverPrefersSoloPhoto guards that a person's cover
// is taken from a photo where they are the only face: a group shot processed
// first must not win just because its face has the lowest ID.
func TestFaceRecognitionManager_CoverPrefersSoloPhoto(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	group := seedImage(t, db, "group")
	solo := seedImage(t, db, "solo")

	NewManager(db, newGroupAwareFacesClient()).runPass(nil)

	var person domain.Person
	if err := db.First(&person).Error; err != nil {
		t.Fatalf("load person: %v", err)
	}
	if person.CoverFaceID == nil {
		t.Fatal("expected the person to have a cover face")
	}
	var cover domain.Face
	if err := db.First(&cover, *person.CoverFaceID).Error; err != nil {
		t.Fatalf("load cover face: %v", err)
	}
	if cover.ImageFileID != solo.ID {
		t.Errorf("cover image = %d, want the solo photo %d (group photo is %d)",
			cover.ImageFileID, solo.ID, group.ID)
	}
}

// TestFaceRecognitionManager_AnalysesEachImageOnce guards the scan version: an
// image is analysed once, images without faces are not analysed again, and a new
// analyser version replaces the stored faces instead of adding to them.
func TestFaceRecognitionManager_AnalysesEachImageOnce(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	faceImage := seedImage(t, db, "person-a-1")
	noFaceImage := seedImage(t, db, "noface")

	client := newFakeFacesClient()
	m := NewManager(db, client)

	m.runPass(nil)
	firstPass := client.AnalyzeCallCount
	if firstPass != 2 {
		t.Fatalf("analyses = %d, want 2 (one per image)", firstPass)
	}

	m.runPass(nil)
	if client.AnalyzeCallCount != firstPass {
		t.Errorf("re-analysed %d already scanned image(s)", client.AnalyzeCallCount-firstPass)
	}

	// A new analyser version re-scans the image and replaces its faces.
	if err := db.Model(&domain.ImageFile{}).
		Where("id IN ?", []uint{faceImage.ID, noFaceImage.ID}).
		Update("faces_scan_version", "analyze-0").Error; err != nil {
		t.Fatalf("reset scan version: %v", err)
	}
	m.runPass(nil)

	var faces int64
	db.Model(&domain.Face{}).Where("image_file_id = ?", faceImage.ID).Count(&faces)
	if faces != 1 {
		t.Errorf("face count after re-scan = %d, want 1 (faces must be replaced, not duplicated)", faces)
	}
}

// TestFaceRecognitionManager_CoverFallsBackToGroupPhoto guards the fallback: a
// person photographed only in group shots still gets a cover, and it stays
// deterministic (the lowest face ID).
func TestFaceRecognitionManager_CoverFallsBackToGroupPhoto(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	seedImage(t, db, "group")

	NewManager(db, newGroupAwareFacesClient()).runPass(nil)

	var person domain.Person
	if err := db.First(&person).Error; err != nil {
		t.Fatalf("load person: %v", err)
	}
	if person.CoverFaceID == nil {
		t.Fatal("expected the person to have a cover face")
	}

	var faces []domain.Face
	if err := db.Where("person_id = ?", person.ID).Order("id").Find(&faces).Error; err != nil {
		t.Fatalf("load faces: %v", err)
	}
	if len(faces) != 2 {
		t.Fatalf("face count = %d, want 2", len(faces))
	}
	if *person.CoverFaceID != faces[0].ID {
		t.Errorf("cover face = %d, want the lowest face ID %d", *person.CoverFaceID, faces[0].ID)
	}
}

// TestFaceRecognitionManager_RefreshesStoredCover guards the migration path:
// people created by the earlier "first face wins" logic get their cover moved
// to a solo photo on the next pass.
func TestFaceRecognitionManager_RefreshesStoredCover(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	group := seedImage(t, db, "group")
	solo := seedImage(t, db, "solo")

	person := domain.Person{}
	if err := db.Create(&person).Error; err != nil {
		t.Fatalf("create person: %v", err)
	}

	groupFace := domain.Face{ImageFileID: group.ID, PersonID: person.ID, Confidence: 0.99}
	otherGroupFace := domain.Face{ImageFileID: group.ID, PersonID: person.ID, Confidence: 0.98}
	soloFace := domain.Face{ImageFileID: solo.ID, PersonID: person.ID, Confidence: 0.5}
	for _, face := range []*domain.Face{&groupFace, &otherGroupFace, &soloFace} {
		if err := db.Create(face).Error; err != nil {
			t.Fatalf("create face: %v", err)
		}
	}
	// The legacy cover points at the group photo.
	person.CoverFaceID = &groupFace.ID
	if err := db.Save(&person).Error; err != nil {
		t.Fatalf("save person: %v", err)
	}

	m := NewManager(db, newGroupAwareFacesClient())
	m.refreshPersonCovers(m.imageFaceCounts())

	var refreshed domain.Person
	if err := db.First(&refreshed, person.ID).Error; err != nil {
		t.Fatalf("reload person: %v", err)
	}
	if refreshed.CoverFaceID == nil || *refreshed.CoverFaceID != soloFace.ID {
		t.Errorf("cover face = %v, want the solo photo face %d", refreshed.CoverFaceID, soloFace.ID)
	}
}

// TestFaceRecognitionManager_StatusProgress guards the status counters.
func TestFaceRecognitionManager_StatusProgress(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	seedImage(t, db, "person-a-1")
	seedImage(t, db, "person-a-2")
	seedImage(t, db, "noface")

	m := newTestManager(t, db)
	m.runPass(nil)

	status := m.GetStatus()
	if status.Total != 3 {
		t.Errorf("total = %d, want 3", status.Total)
	}
	if status.Processed != 3 {
		t.Errorf("processed = %d, want 3", status.Processed)
	}
}
