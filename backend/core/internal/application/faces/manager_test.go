package faces

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
	facesclient "github.com/flashbacks/core/internal/infrastructure/faces"
	"github.com/flashbacks/core/internal/infrastructure/llm"
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
		return analyzedFacesFor(image), nil
	}
	mock.AnalyzeBatchFunc = func(ctx context.Context, images [][]byte) ([]facesclient.BatchResult, error) {
		return batchResults(images), nil
	}
	mock.ClusterFunc = clusterByFirstComponent()
	return &fakeFacesClient{MockFacesClient: mock}
}

// analyzedFacesFor maps fixture file content to a deterministic identity.
func analyzedFacesFor(image []byte) []facesclient.AnalyzedFace {
	if string(image) == "noface" {
		return nil
	}
	vec := make([]float32, 512)
	switch string(image) {
	case "person-a-1", "person-a-2":
		vec[0] = 1
	case "person-b":
		vec[0] = 2
	}
	return []facesclient.AnalyzedFace{mocks.SingleAnalyzedFace(vec)}
}

// batchResults answers one entry per image, mirroring the service contract.
func batchResults(images [][]byte) []facesclient.BatchResult {
	results := make([]facesclient.BatchResult, len(images))
	for i, image := range images {
		results[i] = facesclient.BatchResult{Faces: analyzedFacesFor(image)}
	}
	return results
}

// totalImagesAnalyzed counts the images the fake saw across single and batch
// requests.
func totalImagesAnalyzed(client *fakeFacesClient) int {
	total := client.AnalyzeCallCount
	for _, size := range client.AnalyzeBatchSizes {
		total += size
	}
	return total
}

// batchManager builds a manager whose requests carry at most batchSize images.
func batchManager(db *gorm.DB, client *fakeFacesClient, batchSize int) *Manager {
	return NewManagerWithOptions(db, client, Options{
		BatchSize:      batchSize,
		AnalyzeTimeout: 30 * time.Second,
	})
}

// seedFace stores one detected face with a deterministic embedding, for tests
// that start from an existing unassigned face instead of a full scan.
func seedFace(t *testing.T, db *gorm.DB, imageFileID uint) *domain.Face {
	t.Helper()
	face := domain.Face{ImageFileID: imageFileID, Confidence: 0.9}
	if err := db.Create(&face).Error; err != nil {
		t.Fatalf("create face: %v", err)
	}
	vec := make([]float32, 512)
	vec[0] = 1
	embedding := domain.FaceEmbedding{
		FaceID:    face.ID,
		Dimension: len(vec),
		Embedding: llm.Float32SliceToPgVector(vec),
		Model:     domain.FaceEmbeddingModel,
	}
	if err := db.Create(&embedding).Error; err != nil {
		t.Fatalf("create embedding: %v", err)
	}
	return &face
}

// fakeNeighbours is a controllable NeighbourSource.
type fakeNeighbours struct {
	supported  bool
	err        error
	result     map[uint][]Neighbour
	calls      int
	seeds      []uint
	similarity float64
	limit      int
}

func (f *fakeNeighbours) Supported(context.Context) bool { return f.supported }

func (f *fakeNeighbours) Neighbours(
	_ context.Context, seeds []uint, minSimilarity float64, limit int,
) (map[uint][]Neighbour, error) {
	f.calls++
	f.seeds = seeds
	f.similarity = minSimilarity
	f.limit = limit
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
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
	mock.AnalyzeBatchFunc = func(ctx context.Context, images [][]byte) ([]facesclient.BatchResult, error) {
		results := make([]facesclient.BatchResult, len(images))
		for i, image := range images {
			faces, err := mock.AnalyzeFunc(ctx, image)
			if err != nil {
				return nil, err
			}
			results[i] = facesclient.BatchResult{Faces: faces}
		}
		return results, nil
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
	firstPass := totalImagesAnalyzed(client)
	if firstPass != 2 {
		t.Fatalf("analysed images = %d, want 2 (one per image)", firstPass)
	}

	m.runPass(nil)
	if totalImagesAnalyzed(client) != firstPass {
		t.Errorf("re-analysed %d already scanned image(s)", totalImagesAnalyzed(client)-firstPass)
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

// TestFaceRecognitionManager_AnalyzesInBatches guards the batch contract: five
// images with a batch size of two reach the faces service as three requests
// (2 + 2 + 1), and every image — including one without a face — is marked
// scanned.
func TestFaceRecognitionManager_AnalyzesInBatches(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	for _, content := range []string{"person-a-1", "person-a-2", "person-b", "solo", "noface"} {
		seedImage(t, db, content)
	}
	client := newFakeFacesClient()

	batchManager(db, client, 2).runPass(nil)

	if want := []int{2, 2, 1}; !reflect.DeepEqual(client.AnalyzeBatchSizes, want) {
		t.Fatalf("batch sizes = %v, want %v", client.AnalyzeBatchSizes, want)
	}
	if client.AnalyzeCallCount != 0 {
		t.Errorf("single-image analyses = %d, want 0 (the job must batch)", client.AnalyzeCallCount)
	}

	var scanned int64
	db.Model(&domain.ImageFile{}).
		Where("faces_scan_version = ?", facesScanVersion).
		Count(&scanned)
	if scanned != 5 {
		t.Errorf("scanned images = %d, want 5", scanned)
	}
}

// TestFaceRecognitionManager_FailedBatchRetried guards the retry contract: a
// failed batch request leaves every image of the batch unmarked, so the next
// pass analyses them again.
func TestFaceRecognitionManager_FailedBatchRetried(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	first := seedImage(t, db, "person-a-1")
	second := seedImage(t, db, "person-a-2")

	client := newFakeFacesClient()
	client.AnalyzeBatchFunc = func(context.Context, [][]byte) ([]facesclient.BatchResult, error) {
		return nil, fmt.Errorf("faces service unavailable")
	}
	m := batchManager(db, client, 2)

	m.runPass(nil)

	for _, image := range []*domain.ImageFile{first, second} {
		var stored domain.ImageFile
		if err := db.First(&stored, image.ID).Error; err != nil {
			t.Fatalf("load image %d: %v", image.ID, err)
		}
		if stored.FacesScanVersion == facesScanVersion {
			t.Errorf("image %d was marked scanned although its batch failed", image.ID)
		}
	}

	// The service recovers: the next pass analyses both images again.
	client.AnalyzeBatchFunc = func(_ context.Context, images [][]byte) ([]facesclient.BatchResult, error) {
		return batchResults(images), nil
	}
	m.runPass(nil)

	var scanned int64
	db.Model(&domain.ImageFile{}).
		Where("faces_scan_version = ?", facesScanVersion).
		Count(&scanned)
	if scanned != 2 {
		t.Errorf("scanned images after retry = %d, want 2", scanned)
	}
}

// fakeIndexer records neighbour-index maintenance calls and their order
// relative to clustering.
type fakeIndexer struct {
	calls  int
	events *[]string
}

func (f *fakeIndexer) EnsureNeighbourIndex(context.Context) (bool, error) {
	f.calls++
	*f.events = append(*f.events, "index")
	return true, nil
}

// TestFaceRecognitionManager_EnsuresIndexBeforeClustering guards the index
// policy: one ensure call per pass, after the faces are stored and before
// people are assigned — never once per face insert.
func TestFaceRecognitionManager_EnsuresIndexBeforeClustering(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	seedImage(t, db, "person-a-1")
	seedImage(t, db, "person-a-2")

	client := newFakeFacesClient()
	events := []string{}
	inner := client.ClusterFunc
	client.ClusterFunc = func(ctx context.Context, embeddings [][]float32, minClusterSize int, soloFaces []bool) ([]int, error) {
		events = append(events, "cluster")
		return inner(ctx, embeddings, minClusterSize, soloFaces)
	}
	indexer := &fakeIndexer{events: &events}

	m := NewManagerWithDependencies(db, client, Dependencies{Indexes: indexer}, DefaultOptions())
	m.runPass(nil)

	if indexer.calls != 1 {
		t.Fatalf("EnsureNeighbourIndex calls = %d, want 1 per pass", indexer.calls)
	}
	if want := []string{"index", "cluster"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("call order = %v, want %v (index before person assignment)", events, want)
	}

	var faces int64
	db.Model(&domain.Face{}).Count(&faces)
	if faces != 2 {
		t.Fatalf("stored faces = %d, want 2", faces)
	}
	if indexer.calls != 1 {
		t.Errorf("index ensured %d times for %d faces, want once", indexer.calls, faces)
	}
}

// TestClusterAssignUsesStoreCandidates guards the store path: core asks the
// vector store for candidates at the service's advertised recognition floor and
// sends the links instead of every embedding.
func TestClusterAssignUsesStoreCandidates(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	imageA := seedImage(t, db, "person-a-1")
	imageB := seedImage(t, db, "person-a-2")
	faceA := seedFace(t, db, imageA.ID)
	faceB := seedFace(t, db, imageB.ID)

	client := newFakeFacesClient()
	client.HealthFunc = func(context.Context) (facesclient.Health, error) {
		return facesclient.Health{
			Thresholds: facesclient.Thresholds{RecognitionMinSimilarity: 0.31},
		}, nil
	}
	var gotSeeds []uint
	var gotEdges []facesclient.FaceLink
	client.ClusterGraphFunc = func(
		_ context.Context, faceIDs []uint, edges []facesclient.FaceLink, _ []bool,
	) ([]int, error) {
		gotSeeds = faceIDs
		gotEdges = edges
		return []int{0, 0}, nil
	}
	neighbours := &fakeNeighbours{
		supported: true,
		result: map[uint][]Neighbour{
			faceA.ID: {{FaceID: faceB.ID, Similarity: 0.9}},
			faceB.ID: {{FaceID: faceA.ID, Similarity: 0.9}},
		},
	}

	m := NewManagerWithDependencies(db, client, Dependencies{Neighbours: neighbours}, Options{
		ClusterMode: domain.FacesClusterModeAuto,
	})
	m.clusterAssign(nil, map[uint]int{})

	if neighbours.calls != 1 {
		t.Fatalf("neighbour queries = %d, want 1", neighbours.calls)
	}
	if neighbours.similarity != 0.31 {
		t.Errorf("minSimilarity = %v, want the service's advertised 0.31", neighbours.similarity)
	}
	if neighbours.limit != 16 {
		t.Errorf("neighbour limit = %d, want the default 16", neighbours.limit)
	}
	if len(gotSeeds) != 2 {
		t.Fatalf("cluster graph faces = %d, want 2", len(gotSeeds))
	}
	if len(gotEdges) != 2 || gotEdges[0].FaceID != faceA.ID || gotEdges[0].CandidateID != faceB.ID {
		t.Errorf("candidate edges = %+v, want the two queried links", gotEdges)
	}
	if client.ClusterCallCount != 0 {
		t.Errorf("memory clustering called %d times, want 0 on the store path", client.ClusterCallCount)
	}

	var assignedA, assignedB domain.Face
	db.First(&assignedA, faceA.ID)
	db.First(&assignedB, faceB.ID)
	if assignedA.PersonID == 0 || assignedA.PersonID != assignedB.PersonID {
		t.Errorf("person ids = %d and %d, want one shared person",
			assignedA.PersonID, assignedB.PersonID)
	}
}

// TestClusterAssignFallsBackInMemory guards that clustering still produces
// people when the store cannot answer kNN queries or the query fails.
func TestClusterAssignFallsBackInMemory(t *testing.T) {
	cases := []struct {
		name       string
		neighbours *fakeNeighbours
	}{
		{
			name:       "store reports unsupported",
			neighbours: &fakeNeighbours{supported: false},
		},
		{
			name: "neighbour query fails",
			neighbours: &fakeNeighbours{
				supported: true,
				err:       fmt.Errorf("query face neighbours: connection reset"),
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			db, cleanup := testutil.NewTestDB(t)
			defer cleanup()

			imageA := seedImage(t, db, "person-a-1")
			imageB := seedImage(t, db, "person-a-2")
			seedFace(t, db, imageA.ID)
			seedFace(t, db, imageB.ID)

			client := newFakeFacesClient()
			client.ClusterGraphFunc = func(
				context.Context, []uint, []facesclient.FaceLink, []bool,
			) ([]int, error) {
				t.Error("cluster graph must not be used on the in-memory path")
				return nil, nil
			}

			m := NewManagerWithDependencies(db, client, Dependencies{
				Neighbours: testCase.neighbours,
			}, Options{ClusterMode: domain.FacesClusterModeAuto})
			m.clusterAssign(nil, map[uint]int{})

			if client.ClusterCallCount != 1 {
				t.Fatalf("memory clustering calls = %d, want 1", client.ClusterCallCount)
			}
			var assigned int64
			db.Model(&domain.Face{}).Where("person_id > 0").Count(&assigned)
			if assigned != 2 {
				t.Errorf("faces assigned to a person = %d, want 2", assigned)
			}
		})
	}
}

// TestClusterAssignHonoursMode guards the FACES_CLUSTER_MODE lever: memory
// forces the all-pairs path, store forces the candidate graph.
func TestClusterAssignHonoursMode(t *testing.T) {
	cases := []struct {
		name            string
		mode            string
		supported       bool
		wantGraphCalls  int
		wantMemoryCalls int
	}{
		{
			name:            "memory forces the in-memory path",
			mode:            domain.FacesClusterModeMemory,
			supported:       true,
			wantGraphCalls:  0,
			wantMemoryCalls: 1,
		},
		{
			name:            "store forces the candidate graph",
			mode:            domain.FacesClusterModeStore,
			supported:       false,
			wantGraphCalls:  1,
			wantMemoryCalls: 0,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			db, cleanup := testutil.NewTestDB(t)
			defer cleanup()

			seedFace(t, db, seedImage(t, db, "person-a-1").ID)
			seedFace(t, db, seedImage(t, db, "person-a-2").ID)

			client := newFakeFacesClient()
			client.ClusterGraphFunc = func(
				_ context.Context, faceIDs []uint, _ []facesclient.FaceLink, _ []bool,
			) ([]int, error) {
				labels := make([]int, len(faceIDs))
				return labels, nil
			}

			m := NewManagerWithDependencies(db, client, Dependencies{
				Neighbours: &fakeNeighbours{supported: testCase.supported},
			}, Options{ClusterMode: testCase.mode})
			m.clusterAssign(nil, map[uint]int{})

			if client.ClusterGraphCallCount != testCase.wantGraphCalls {
				t.Errorf("cluster graph calls = %d, want %d",
					client.ClusterGraphCallCount, testCase.wantGraphCalls)
			}
			if client.ClusterCallCount != testCase.wantMemoryCalls {
				t.Errorf("memory clustering calls = %d, want %d",
					client.ClusterCallCount, testCase.wantMemoryCalls)
			}
		})
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
