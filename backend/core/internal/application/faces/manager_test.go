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

// fakeFacesClient builds a deterministic client whose detections and embeddings
// are keyed by the image content (the file bytes), so tests control identity.
type fakeFacesClient struct {
	*mocks.MockFacesClient
}

func newFakeFacesClient() *fakeFacesClient {
	mock := &mocks.MockFacesClient{}
	mock.DetectFunc = func(ctx context.Context, image []byte) ([]facesclient.FaceBox, error) {
		if string(image) == "noface" {
			return nil, nil
		}
		return mocks.SingleFaceBoxes(), nil
	}
	mock.EmbedFunc = func(ctx context.Context, image []byte, box *facesclient.FaceBox) ([]float32, error) {
		vec := make([]float32, 512)
		switch string(image) {
		case "person-a-1", "person-a-2":
			vec[0] = 1
		case "person-b":
			vec[0] = 2
		}
		return vec, nil
	}
	mock.ClusterFunc = func(ctx context.Context, embeddings [][]float32, minClusterSize int) ([]int, error) {
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
	return &fakeFacesClient{MockFacesClient: mock}
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
