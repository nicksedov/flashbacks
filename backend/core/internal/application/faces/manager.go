// Package faces implements the background face recognition job.
package faces

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flashbacks/core/internal/application/background"
	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/domain/repository"
	facesclient "github.com/flashbacks/core/internal/infrastructure/faces"
	"github.com/flashbacks/core/internal/infrastructure/llm"

	"gorm.io/gorm"
)

// facesScanVersion identifies the current face-analysis pipeline. Images whose
// stored faces come from another version are analysed again — once — which is
// how an improved analyser reaches photos that already have faces.
const facesScanVersion = "analyze-3"

// minClusterSize is the minimum number of faces a person needs before the
// clustering service groups them into a person. Two is the calibrated floor:
// an identity photographed twice must still become a person, while the
// similarity floor inside the faces service keeps different people apart.
// A higher floor (5 labels every face of a small gallery as noise) leaves the
// People view empty even though faces were detected and embedded.
const minClusterSize = 2

// fallbackRecognitionSimilarity is the calibrated relaxed floor used when the
// faces service does not advertise its thresholds (an older service).
const fallbackRecognitionSimilarity = 0.25

// personClusterVersion identifies the reconciliation generation that produced a
// person. A person carrying another version is reconciled on the next pass, so
// an algorithm change reaches a gallery that is already fully analysed without
// a wipe.
const personClusterVersion = "cluster-graph-v2"

// Calibrated fallbacks used when the faces service does not advertise the
// clustering thresholds (an older service). They mirror the service defaults.
const (
	fallbackAssignmentSimilarity   = 0.30
	fallbackRecognitionMargin      = 0.05
	fallbackMergeSimilarity        = 0.50
	fallbackMergeSupport           = 2
	fallbackMergeSupportSimilarity = 0.40
)

// exemplarLimit is how many reference vectors one person keeps: enough to hold
// the modes of one identity (age, beard, glasses) without a full re-cluster.
const exemplarLimit = 5

// Options configures how the recognition job talks to the faces service.
type Options struct {
	// BatchSize is the maximum number of images one analysis request carries.
	BatchSize int
	// BatchConcurrency is how many analysis requests may be in flight at once,
	// one per model session on the faces service (one session per GPU).
	BatchConcurrency int
	// AnalyzeTimeout bounds a single batch analysis request.
	AnalyzeTimeout time.Duration
	// ClusterMode selects the clustering strategy: auto, store or memory.
	ClusterMode string
	// ClusterNeighbours caps the candidates per face in the store path.
	ClusterNeighbours int
	// ClusterTimeout bounds index maintenance and one clustering decision; a
	// large gallery (tens of thousands of faces) needs minutes, not seconds.
	ClusterTimeout time.Duration
	// ReclusterAll discards the people a previous clustering produced before a
	// pass clusters again, so a changed clustering algorithm reaches a gallery
	// that is already fully analysed. Names a user typed are kept.
	ReclusterAll bool
}

// memoryClusterLimit caps the in-memory all-pairs fallback: sklearn computes a
// full NxN distance matrix, so a very large gallery would exhaust memory
// instead of merely being slow.
const memoryClusterLimit = 5000

// Dependencies are the optional stores the job consults when a clustering
// strategy needs them.
type Dependencies struct {
	// Neighbours answers face nearest-neighbour queries; nil keeps the
	// in-memory clustering path.
	Neighbours NeighbourSource
	// Indexes ensures the neighbour index after an analysis pass; nil skips it.
	Indexes IndexMaintainer
	// PersonReferences stores the exemplar vectors that let a later pass
	// recognise a person it created earlier; nil keeps the legacy behaviour.
	PersonReferences repository.PersonReferenceRepository
}

// DefaultOptions returns the documented defaults (eight images per request,
// five minutes per request, automatic clustering with sixteen candidates).
func DefaultOptions() Options {
	return Options{
		BatchSize:         8,
		BatchConcurrency:  2,
		AnalyzeTimeout:    300 * time.Second,
		ClusterMode:       domain.FacesClusterModeAuto,
		ClusterNeighbours: 16,
		ClusterTimeout:    10 * time.Minute,
	}
}

func (o Options) normalized() Options {
	defaults := DefaultOptions()
	if o.BatchSize <= 0 {
		o.BatchSize = defaults.BatchSize
	}
	if o.BatchConcurrency <= 0 {
		o.BatchConcurrency = defaults.BatchConcurrency
	}
	if o.AnalyzeTimeout <= 0 {
		o.AnalyzeTimeout = defaults.AnalyzeTimeout
	}
	if o.ClusterMode != domain.FacesClusterModeStore && o.ClusterMode != domain.FacesClusterModeMemory {
		o.ClusterMode = defaults.ClusterMode
	}
	if o.ClusterNeighbours <= 0 {
		o.ClusterNeighbours = defaults.ClusterNeighbours
	}
	if o.ClusterTimeout <= 0 {
		o.ClusterTimeout = defaults.ClusterTimeout
	}
	return o
}

// Progress tracks how many photos the job has processed.
type Progress struct {
	Total     int    `json:"total"`
	Processed int    `json:"processed"`
	LastError string `json:"lastError"`
}

// Status is the public status of the face recognition job.
type Status struct {
	Running   bool     `json:"running"`
	Processed int      `json:"processed"`
	Total     int      `json:"total"`
	Progress  Progress `json:"progress"`
}

// Manager runs the face recognition background job: it detects faces on
// unprocessed gallery photos, embeds them, persists faces and embeddings, then
// clusters the embeddings and assigns each face to a person. It follows the
// TagScanManager pattern (schedule window + resumable cursor over unprocessed
// images).
type Manager struct {
	*background.Manager
	mu                sync.Mutex
	db                *gorm.DB
	client            facesclient.Client
	schedule          domain.Schedule
	scheduleCh        chan struct{}
	progress          Progress
	clusterSize       int
	batchSize         int
	batchConcurrency  int
	analyzeTimeout    time.Duration
	clusterMode       string
	clusterNeighbours int
	clusterTimeout    time.Duration
	reclusterAll      bool
	// clusterPending is true while a pass still owes a clustering decision:
	// on start (the gallery may be fully analysed already) and after every
	// pass that analysed images. It is cleared once clustering succeeds.
	clusterPending bool
	neighbours     NeighbourSource
	indexes        IndexMaintainer
	personRefs     repository.PersonReferenceRepository
}

// batchCaps bound the per-statement work on a gallery of tens of thousands of
// faces, so one clustering pass does not build a single huge query.
const (
	// embeddingBatch is how many face ids one embedding lookup carries.
	embeddingBatch = 1000
	// assignmentBatch is how many face ids one person assignment carries.
	assignmentBatch = 500
	// neighbourBatch is how many seeds one neighbour query asks for.
	neighbourBatch = 4000
)

// NewManager creates a face recognition manager with the default batching
// options and no optional stores.
func NewManager(db *gorm.DB, client facesclient.Client) *Manager {
	return NewManagerWithDependencies(db, client, Dependencies{}, DefaultOptions())
}

// NewManagerWithOptions creates a face recognition manager with explicit
// batching options and no optional stores.
func NewManagerWithOptions(db *gorm.DB, client facesclient.Client, options Options) *Manager {
	return NewManagerWithDependencies(db, client, Dependencies{}, options)
}

// NewManagerWithDependencies creates a face recognition manager with explicit
// stores and batching options.
func NewManagerWithDependencies(
	db *gorm.DB, client facesclient.Client, dependencies Dependencies, options Options,
) *Manager {
	options = options.normalized()
	return &Manager{
		Manager:           background.New("face-recognition"),
		db:                db,
		client:            client,
		schedule:          domain.DefaultSchedule(domain.ScheduleFaceRecognition),
		scheduleCh:        make(chan struct{}),
		clusterSize:       minClusterSize,
		batchSize:         options.BatchSize,
		batchConcurrency:  options.BatchConcurrency,
		analyzeTimeout:    options.AnalyzeTimeout,
		clusterMode:       options.ClusterMode,
		clusterNeighbours: options.ClusterNeighbours,
		clusterTimeout:    options.ClusterTimeout,
		reclusterAll:      options.ReclusterAll,
		clusterPending:    true,
		neighbours:        dependencies.Neighbours,
		indexes:           dependencies.Indexes,
		personRefs:        dependencies.PersonReferences,
	}
}

// Start begins the schedule-driven recognition loop.
func (m *Manager) Start(schedule domain.Schedule) {
	if !m.TryStart() {
		log.Println("Face recognition already running")
		return
	}
	m.mu.Lock()
	m.schedule = schedule
	m.scheduleCh = make(chan struct{})
	m.mu.Unlock()

	go m.scheduleLoop()
}

// Stop stops the recognition loop.
func (m *Manager) Stop() {
	m.Manager.Stop()
}

// IsRunning reports whether the job is currently running.
func (m *Manager) IsRunning() bool {
	return m.Manager.IsRunning()
}

// UpdateSchedule updates the schedule at runtime.
func (m *Manager) UpdateSchedule(schedule domain.Schedule) {
	m.mu.Lock()
	wasRunning := m.Manager.IsRunning()
	m.schedule = schedule
	m.mu.Unlock()

	if wasRunning {
		select {
		case m.scheduleCh <- struct{}{}:
		default:
		}
	}
}

// GetStatus returns the current recognition status.
func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	running := m.Manager.IsRunning()
	progress := m.progress
	m.mu.Unlock()
	return Status{Running: running, Processed: progress.Processed, Total: progress.Total, Progress: progress}
}

// scheduleLoop runs the job within its configured window.
func (m *Manager) scheduleLoop() {
	for {
		m.mu.Lock()
		schedule := m.schedule
		stopCh := m.Done()
		m.mu.Unlock()

		if !schedule.Enabled || len(schedule.Weekdays()) == 0 {
			select {
			case <-stopCh:
				return
			case <-m.scheduleCh:
				continue
			}
		}

		if schedule.IsWindowOpenAt(time.Now()) {
			m.runPass(stopCh)
			select {
			case <-time.After(30 * time.Second):
			case <-stopCh:
				return
			case <-m.scheduleCh:
			}
			continue
		}

		next := schedule.NextWindowOpenAt(time.Now())
		select {
		case <-time.After(time.Until(next)):
		case <-stopCh:
			return
		case <-m.scheduleCh:
			continue
		}
	}
}

// runPass analyses every image the current analyser has not seen yet, then
// clusters and assigns people.
//
// Batches are collected sequentially (one resumable cursor) but sent to the
// faces service by a small worker pool, so a faces service with one session per
// GPU keeps several requests in flight. Results are persisted on this goroutine,
// which keeps the database writes ordered.
func (m *Manager) runPass(stopCh <-chan struct{}) {
	var total int64
	m.db.Model(&domain.ImageFile{}).
		Where("faces_scan_version <> ?", facesScanVersion).
		Count(&total)
	m.mu.Lock()
	if total == 0 {
		// A previous pass may have stored faces and then failed (or been
		// stopped) before clustering. The pending flag keeps retrying that
		// step, but only while a pass still owes a decision: retrying merely
		// because unassigned faces exist would re-cluster every thirty seconds
		// forever, since the faces no person claims stay unassigned on purpose.
		pending := m.clusterPending
		reclusterAll := m.reclusterAll
		m.reclusterAll = false
		m.progress = Progress{}
		m.mu.Unlock()
		if !pending && !reclusterAll {
			return
		}
		if reclusterAll {
			// A finished gallery keeps its faces, so a clustering change is
			// only visible after the stored people are discarded. The flag is
			// one-shot: the pass rebuilds the people, and the next pass keeps
			// them instead of deleting everything every thirty seconds.
			m.resetGeneratedPeople()
		}
	} else {
		m.clusterPending = true
		m.progress = Progress{Total: int(total)}
		m.mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Cancel in-flight requests as soon as the job is asked to stop.
	go func() {
		select {
		case <-stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	batches := make(chan []batchImage)
	outcomes := make(chan batchOutcome)

	var workers sync.WaitGroup
	for i := 0; i < m.batchConcurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			m.dispatchBatches(ctx, batches, outcomes)
		}()
	}
	go func() {
		defer close(batches)
		m.collectBatches(stopCh, batches)
	}()
	go func() {
		workers.Wait()
		close(outcomes)
	}()

	for outcome := range outcomes {
		if ctx.Err() != nil {
			// Drained after a stop: the pass is being abandoned.
			continue
		}
		m.applyBatch(outcome)
	}

	select {
	case <-stopCh:
		return
	default:
	}

	// The new faces are stored, so the neighbour index can be (re)built before
	// clustering reads it. Building it after the analysis loop means inserts
	// during the scan hit a heap-only table.
	m.ensureNeighbourIndex()

	// Detection is finished, so the per-image face counts are final and serve
	// both the covers of the new clusters and the refresh of stored ones.
	counts := m.imageFaceCounts()
	m.clusterAssign(stopCh, counts)
	// People produced by another clustering generation are reconciled once the
	// new people exist: the reconcile step merges the fragments of one identity
	// and builds the references a later pass assigns against.
	if m.peopleNeedReconcile() {
		m.reconcilePeople()
	}
	m.refreshPersonCovers(counts)
}

// collectBatches reads unprocessed images from the cursor and queues full
// batches for the dispatchers. It stops early when the pass is cancelled.
func (m *Manager) collectBatches(stopCh <-chan struct{}, out chan<- []batchImage) {
	cursor := uint(0)
	exhausted := false
	for !exhausted {
		select {
		case <-stopCh:
			return
		default:
		}

		batch := make([]batchImage, 0, m.batchSize)
		for len(batch) < m.batchSize {
			var imageFile domain.ImageFile
			err := m.db.Where("faces_scan_version <> ? AND id > ?", facesScanVersion, cursor).
				Order("id ASC").
				First(&imageFile).Error
			if err != nil {
				exhausted = true
				break
			}
			cursor = imageFile.ID

			select {
			case <-stopCh:
				return
			default:
			}

			data, err := os.ReadFile(imageFile.Path)
			m.mu.Lock()
			m.progress.Processed++
			m.mu.Unlock()
			if err != nil {
				// The scan version stays untouched, so the next pass retries it.
				m.setError(fmt.Sprintf("read %s: %v", imageFile.Path, err))
				continue
			}
			batch = append(batch, batchImage{file: imageFile, data: data})
		}

		if len(batch) == 0 {
			continue
		}
		select {
		case out <- batch:
		case <-stopCh:
			return
		}
	}
}

// batchOutcome is the result of one dispatched analysis request.
type batchOutcome struct {
	images  []batchImage
	results []facesclient.BatchResult
	err     error
}

// dispatchBatches consumes queued batches until the channel closes or the pass
// is cancelled.
func (m *Manager) dispatchBatches(
	ctx context.Context, in <-chan []batchImage, out chan<- batchOutcome,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch, ok := <-in:
			if !ok {
				return
			}
			m.dispatchBatch(ctx, batch, out)
		}
	}
}

// dispatchBatch sends one batch and reports its outcome. Every worker may have
// a request in flight, which is what lets the faces service use both GPUs.
func (m *Manager) dispatchBatch(
	ctx context.Context, images []batchImage, out chan<- batchOutcome,
) {
	payload := make([][]byte, len(images))
	for i, image := range images {
		payload[i] = image.data
	}

	requestCtx, cancel := context.WithTimeout(ctx, m.analyzeTimeout)
	defer cancel()

	results, err := m.client.AnalyzeBatch(requestCtx, payload)
	select {
	case out <- batchOutcome{images: images, results: results, err: err}:
	case <-ctx.Done():
	}
}

// ensureNeighbourIndex asks the store to (re)build the face neighbour index
// once per pass, before person assignment reads it.
func (m *Manager) ensureNeighbourIndex() {
	if m.indexes == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), m.clusterTimeout)
	defer cancel()
	if _, err := m.indexes.EnsureNeighbourIndex(ctx); err != nil {
		m.setError(fmt.Sprintf("ensure face embedding index: %v", err))
	}
}

// batchImage is one photo queued for a batch analysis request.
type batchImage struct {
	file domain.ImageFile
	data []byte
}

// applyBatch stores the entries of one analysed batch.
//
// A request-level failure leaves the whole batch unmarked so the next pass
// analyses it again. A per-entry failure is recorded and also left unmarked,
// so one unreadable photo never blocks the images that succeeded.
func (m *Manager) applyBatch(outcome batchOutcome) {
	images := outcome.images
	if outcome.err != nil {
		m.setError(fmt.Sprintf("analyze batch of %d: %v", len(images), outcome.err))
		return
	}

	for i, image := range images {
		if i >= len(outcome.results) {
			m.setError(fmt.Sprintf("analyze batch: service returned %d entries for %d images",
				len(outcome.results), len(images)))
			return
		}
		if outcome.results[i].Error != "" {
			// The service reached this image and reported a failure no retry
			// will fix (an undecodable file, an unresolvable crop). Recording
			// it and marking the image scanned keeps a handful of such photos
			// from being re-analysed on every pass forever, which on a real
			// gallery delayed clustering and kept the job constantly busy. A
			// request-level failure stays retryable, as the batch contract
			// requires.
			m.setError(fmt.Sprintf("analyze %s: %s", image.file.Path, outcome.results[i].Error))
			m.markScanned(image.file.ID)
			continue
		}
		m.replaceFaces(image.file.ID, outcome.results[i].Faces)
		m.markScanned(image.file.ID)
	}
}

// faceRef identifies one face of a cluster together with the photo it was
// detected on; choosing a cover needs both.
type faceRef struct {
	faceID      uint
	imageFileID uint
	confidence  float32
	width       float32
}

// imageFaceCounts reports how many faces every photo contains. A photo with a
// single face is the best cover for its person: nothing else shares the tile.
func (m *Manager) imageFaceCounts() map[uint]int {
	type row struct {
		ImageFileID uint
		FaceCount   int
	}
	var rows []row
	counts := make(map[uint]int)
	if err := m.db.Table("faces").
		Select("image_file_id, count(*) AS face_count").
		Group("image_file_id").
		Scan(&rows).Error; err != nil {
		m.setError(fmt.Sprintf("count faces per image: %v", err))
		return counts
	}
	for _, r := range rows {
		counts[r.ImageFileID] = r.FaceCount
	}
	return counts
}

// chooseCover picks the cover face of one person: a face detected on a solo
// photo beats a face cropped out of a group shot, then the most confident
// detection wins, and the lowest face ID (members are ordered by ID) keeps the
// choice deterministic.
func chooseCover(members []faceRef, facesPerImage map[uint]int) uint {
	best := members[0]
	for _, candidate := range members[1:] {
		bestSolo := facesPerImage[best.imageFileID] <= 1
		candidateSolo := facesPerImage[candidate.imageFileID] <= 1
		switch {
		case candidateSolo && !bestSolo:
			best = candidate
		case candidateSolo == bestSolo && candidate.confidence > best.confidence:
			best = candidate
		}
	}
	return best.faceID
}

// refreshPersonCovers re-evaluates stored covers, so people clustered by an
// earlier version of the job (one that always took the first face) also end up
// with a solo photo as their cover.
func (m *Manager) refreshPersonCovers(facesPerImage map[uint]int) {
	var faces []domain.Face
	if err := m.db.Where("person_id > 0").Order("id").Find(&faces).Error; err != nil {
		m.setError(fmt.Sprintf("load assigned faces: %v", err))
		return
	}
	if len(faces) == 0 {
		return
	}

	byPerson := make(map[uint][]faceRef, len(faces))
	for _, face := range faces {
		byPerson[face.PersonID] = append(byPerson[face.PersonID], faceRef{
			faceID:      face.ID,
			imageFileID: face.ImageFileID,
			confidence:  face.Confidence,
		})
	}

	var people []domain.Person
	if err := m.db.Order("id").Find(&people).Error; err != nil {
		m.setError(fmt.Sprintf("load people: %v", err))
		return
	}
	for _, person := range people {
		members := byPerson[person.ID]
		if len(members) == 0 {
			continue
		}
		cover := chooseCover(members, facesPerImage)
		if person.CoverFaceID != nil && *person.CoverFaceID == cover {
			continue
		}
		if err := m.db.Model(&domain.Person{}).Where("id = ?", person.ID).
			Update("cover_face_id", cover).Error; err != nil {
			m.setError(fmt.Sprintf("refresh cover of person %d: %v", person.ID, err))
		}
	}
}

// replaceFaces drops the faces stored for an image and stores the analysed ones,
// so re-analysing a photo replaces its faces instead of accumulating them.
func (m *Manager) replaceFaces(imageFileID uint, faces []facesclient.AnalyzedFace) {
	faceIDs := m.db.Model(&domain.Face{}).Select("id").Where("image_file_id = ?", imageFileID)
	if err := m.db.Where("face_id IN (?)", faceIDs).Delete(&domain.FaceEmbedding{}).Error; err != nil {
		m.setError(fmt.Sprintf("clear embeddings of image %d: %v", imageFileID, err))
		return
	}
	if err := m.db.Where("image_file_id = ?", imageFileID).Delete(&domain.Face{}).Error; err != nil {
		m.setError(fmt.Sprintf("clear faces of image %d: %v", imageFileID, err))
		return
	}
	for _, face := range faces {
		if len(face.Embedding) == 0 {
			continue
		}
		m.persistFace(imageFileID, face.FaceBox, face.Embedding)
	}
}

// markScanned records that the current analyser produced this image's faces.
func (m *Manager) markScanned(imageFileID uint) {
	if err := m.db.Model(&domain.ImageFile{}).Where("id = ?", imageFileID).
		Update("faces_scan_version", facesScanVersion).Error; err != nil {
		m.setError(fmt.Sprintf("mark image %d scanned: %v", imageFileID, err))
	}
}

// persistFace stores a detected face and its embedding.
func (m *Manager) persistFace(imageFileID uint, box facesclient.FaceBox, vec []float32) {
	face := domain.Face{
		ImageFileID: imageFileID,
		X:           box.X,
		Y:           box.Y,
		Width:       box.Width,
		Height:      box.Height,
		Confidence:  box.Confidence,
	}
	if err := m.db.Create(&face).Error; err != nil {
		m.setError(fmt.Sprintf("persist face: %v", err))
		return
	}

	embedding := domain.FaceEmbedding{
		FaceID:    face.ID,
		Dimension: len(vec),
		Embedding: llm.Float32SliceToPgVector(vec),
		Model:     domain.FaceEmbeddingModel,
	}
	if err := m.db.Create(&embedding).Error; err != nil {
		m.setError(fmt.Sprintf("persist embedding: %v", err))
		return
	}
	if err := m.db.Model(&face).Update("embedding_id", embedding.ID).Error; err != nil {
		m.setError(fmt.Sprintf("link embedding: %v", err))
	}
}

// clusterAssign clusters every unassigned embedding and assigns each face to a
// person (one person per non-noise cluster), picking each person's cover with
// chooseCover.
func (m *Manager) clusterAssign(stopCh <-chan struct{}, facesPerImage map[uint]int) {
	select {
	case <-stopCh:
		return
	default:
	}

	var unassigned []domain.Face
	if err := m.db.Where("person_id = ?", 0).Order("id").Find(&unassigned).Error; err != nil {
		m.setError(fmt.Sprintf("load unassigned faces: %v", err))
		return
	}
	if len(unassigned) == 0 {
		return
	}

	faceIDs := make([]uint, 0, len(unassigned))
	for _, face := range unassigned {
		faceIDs = append(faceIDs, face.ID)
	}
	vectors := m.loadEmbeddings(faceIDs)

	members := make([]faceRef, 0, len(unassigned))
	embeddings := make([][]float32, 0, len(unassigned))
	for _, face := range unassigned {
		vec := vectors[face.ID]
		if len(vec) == 0 {
			continue
		}
		members = append(members, faceRef{
			faceID:      face.ID,
			imageFileID: face.ImageFileID,
			confidence:  face.Confidence,
			width:       face.Width,
		})
		embeddings = append(embeddings, vec)
	}
	if len(embeddings) == 0 {
		return
	}

	solo := make([]bool, len(members))
	for i, member := range members {
		solo[i] = facesPerImage[member.imageFileID] <= 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.clusterTimeout)
	defer cancel()

	memberVectors := make(map[uint][]float32, len(members))
	for index, member := range members {
		memberVectors[member.faceID] = embeddings[index]
	}

	// Step 1 — assign: match every unassigned face against the reference sets
	// of the people already discovered, before anything is sent for clustering.
	candidates, candidateErr := m.candidateMap(ctx, members)
	var references *referenceVectors
	var thresholds clusteringThresholds
	var personByFace map[uint]uint
	if m.personRefs != nil {
		references = m.loadReferenceVectors()
		if references != nil {
			thresholds = m.thresholds(ctx)
			personByFace = m.candidatePersonMap(candidates)
		}
	}
	known := m.assignKnownPeople(members, embeddings, candidates, references, personByFace, thresholds)

	remainingMembers := make([]faceRef, 0, len(members))
	remainingEmbeddings := make([][]float32, 0, len(members))
	remainingSolo := make([]bool, 0, len(members))
	for index, member := range members {
		if known[member.faceID] {
			continue
		}
		remainingMembers = append(remainingMembers, member)
		remainingEmbeddings = append(remainingEmbeddings, embeddings[index])
		remainingSolo = append(remainingSolo, solo[index])
	}

	if len(remainingMembers) == 0 {
		m.mu.Lock()
		m.clusterPending = false
		m.mu.Unlock()
		return
	}

	// Step 2 — cluster: label the rest and treat each cluster as a candidate
	// for a new person.
	labels, err := m.clusterLabels(
		ctx, remainingMembers, remainingEmbeddings, remainingSolo, candidates, candidateErr,
	)
	if err != nil {
		m.setError(fmt.Sprintf("cluster: %v", err))
		return
	}

	byLabel := make(map[int][]faceRef, len(labels))
	for i, label := range labels {
		if i >= len(remainingMembers) || label == -1 {
			continue
		}
		byLabel[label] = append(byLabel[label], remainingMembers[i])
	}

	labelIDs := make([]int, 0, len(byLabel))
	for label := range byLabel {
		labelIDs = append(labelIDs, label)
	}
	sort.Ints(labelIDs)

	// Step 3 — reconcile: extend the person a cluster matches instead of
	// minting a new one, and store the exemplars of every new person.
	assigned := 0
	created := 0
	for _, label := range labelIDs {
		cluster := byLabel[label]
		faceIDs := make([]uint, 0, len(cluster))
		for _, member := range cluster {
			faceIDs = append(faceIDs, member.faceID)
		}

		if references != nil {
			if personID, matched := m.matchClusterToPerson(
				cluster, memberVectors, references, candidates, personByFace, thresholds,
			); matched {
				m.assignFacesToPerson(personID, faceIDs)
				assigned += len(cluster)
				continue
			}
		}

		coverFaceID := chooseCover(cluster, facesPerImage)
		person := domain.Person{
			CoverFaceID:    &coverFaceID,
			ClusterVersion: personClusterVersion,
		}
		if err := m.db.Create(&person).Error; err != nil {
			m.setError(fmt.Sprintf("create person: %v", err))
			continue
		}
		m.assignFacesToPerson(person.ID, faceIDs)
		m.replaceReferences(person.ID, cluster, memberVectors, facesPerImage)
		created++
		assigned += len(cluster)
	}
	log.Printf(
		"Face recognition: clustered %d faces into %d people (%d new, %d assigned)",
		len(remainingMembers), len(labelIDs), created, assigned,
	)
	m.mu.Lock()
	m.clusterPending = false
	m.mu.Unlock()
}

// loadEmbeddings returns the stored vectors of *faceIDs*, one query per batch
// rather than one per face: a gallery of tens of thousands of faces would
// otherwise issue tens of thousands of single-row reads per pass.
func (m *Manager) loadEmbeddings(faceIDs []uint) map[uint][]float32 {
	type embeddingRow struct {
		FaceID    uint
		Embedding string
	}
	vectors := make(map[uint][]float32, len(faceIDs))
	for start := 0; start < len(faceIDs); start += embeddingBatch {
		end := start + embeddingBatch
		if end > len(faceIDs) {
			end = len(faceIDs)
		}
		var rows []embeddingRow
		if err := m.db.Model(&domain.FaceEmbedding{}).
			Select("face_id, embedding").
			Where("face_id IN ?", faceIDs[start:end]).
			Find(&rows).Error; err != nil {
			m.setError(fmt.Sprintf("load embeddings: %v", err))
			continue
		}
		for _, row := range rows {
			if vec := parsePgVector(row.Embedding); len(vec) > 0 {
				vectors[row.FaceID] = vec
			}
		}
	}
	return vectors
}

// resetGeneratedPeople discards the people a previous clustering produced so a
// finished gallery can be clustered again with the current algorithm. People a
// user named are kept, together with their faces.
func (m *Manager) resetGeneratedPeople() {
	generated := m.db.Model(&domain.Person{}).Select("id").Where("name = ?", "")
	if err := m.db.Model(&domain.Face{}).
		Where("person_id IN (?)", generated).
		Update("person_id", 0).Error; err != nil {
		m.setError(fmt.Sprintf("reset face assignments: %v", err))
		return
	}
	if err := m.db.Where("name = ?", "").Delete(&domain.Person{}).Error; err != nil {
		m.setError(fmt.Sprintf("reset people: %v", err))
		return
	}
	log.Println("Face recognition: cleared generated people for a full re-cluster")
}

// clusterLabels decides the person labels of the unassigned faces.
//
// The store path sends one candidate graph built from kNN answers; the memory
// path keeps the all-pairs call the service has always accepted. A failing
// store query degrades to the memory path for small galleries, because the
// graph is an optimisation and people must still be produced. For a large
// gallery the all-pairs matrix would exhaust memory, so the faces stay
// unassigned and the next pass retries the store path.
func (m *Manager) clusterLabels(
	ctx context.Context,
	members []faceRef,
	embeddings [][]float32,
	solo []bool,
	candidates map[uint][]Neighbour,
	candidateErr error,
) ([]int, error) {
	if m.useStoreCandidates(ctx) {
		if candidateErr == nil {
			labels, err := m.clusterFromCandidates(ctx, members, solo, candidates)
			if err == nil {
				return labels, nil
			}
			candidateErr = err
		}
		m.setError(fmt.Sprintf("cluster from store: %v", candidateErr))
		if len(members) > memoryClusterLimit {
			return nil, fmt.Errorf(
				"store clustering failed for %d faces; in-memory fallback is capped at %d: %w",
				len(members), memoryClusterLimit, candidateErr,
			)
		}
	}
	return m.client.Cluster(ctx, embeddings, m.clusterSize, solo)
}

// useStoreCandidates reports whether this pass should ask the vector store for
// candidates instead of sending every embedding.
func (m *Manager) useStoreCandidates(ctx context.Context) bool {
	if m.neighbours == nil {
		return false
	}
	switch m.clusterMode {
	case domain.FacesClusterModeMemory:
		return false
	case domain.FacesClusterModeStore:
		return true
	default:
		return m.neighbours.Supported(ctx)
	}
}

// candidateMap reads each face's neighbourhood inside the service's
// recognition radius. The links serve both the person-reference assignment and
// the clustering decision, so one kNN query covers the pass.
func (m *Manager) candidateMap(
	ctx context.Context, members []faceRef,
) (map[uint][]Neighbour, error) {
	if !m.useStoreCandidates(ctx) {
		return nil, nil
	}
	seeds := make([]uint, len(members))
	for i, member := range members {
		seeds[i] = member.faceID
	}

	similarity := m.recognitionSimilarity(ctx)
	candidates := make(map[uint][]Neighbour, len(seeds))
	for start := 0; start < len(seeds); start += neighbourBatch {
		end := min(start+neighbourBatch, len(seeds))
		batch, err := m.neighbours.Neighbours(
			ctx, seeds[start:end], similarity, m.clusterNeighbours,
		)
		if err != nil {
			return nil, err
		}
		for seed, neighbours := range batch {
			candidates[seed] = neighbours
		}
	}
	return candidates, nil
}

// clusterFromCandidates asks the service to label the candidate graph.
func (m *Manager) clusterFromCandidates(
	ctx context.Context,
	members []faceRef,
	solo []bool,
	candidates map[uint][]Neighbour,
) ([]int, error) {
	seeds := make([]uint, len(members))
	for i, member := range members {
		seeds[i] = member.faceID
	}

	edges := make([]facesclient.FaceLink, 0, len(candidates))
	facts := facesclient.FaceFacts{
		PhotoIDs:  make([]uint, len(members)),
		Widths:    make([]float32, len(members)),
		Qualities: make([]float32, len(members)),
	}
	for index, member := range members {
		facts.PhotoIDs[index] = member.imageFileID
		facts.Widths[index] = member.width
		facts.Qualities[index] = member.confidence
	}
	for _, seed := range seeds {
		for _, neighbour := range candidates[seed] {
			edges = append(edges, facesclient.FaceLink{
				FaceID:      seed,
				CandidateID: neighbour.FaceID,
				Similarity:  neighbour.Similarity,
			})
		}
	}
	return m.client.ClusterGraph(ctx, seeds, edges, solo, facts)
}

// recognitionSimilarity asks the faces service for its recognition floor, so the
// candidate radius stays owned by the component that calibrated it. A service
// that does not report the field falls back to the calibrated 0.25.
func (m *Manager) recognitionSimilarity(ctx context.Context) float64 {
	health, err := m.client.Health(ctx)
	if err != nil {
		log.Printf("Face recognition: read faces health: %v", err)
		return fallbackRecognitionSimilarity
	}
	if health.Thresholds.RecognitionMinSimilarity <= 0 {
		return fallbackRecognitionSimilarity
	}
	return health.Thresholds.RecognitionMinSimilarity
}

// setError records the last error without aborting the pass.
func (m *Manager) setError(msg string) {
	m.mu.Lock()
	m.progress.LastError = msg
	m.mu.Unlock()
	log.Printf("Face recognition: %s", msg)
}

// parsePgVector parses a pgvector string like "[0.1,0.2,0.3]" into float32s.
func parsePgVector(s string) []float32 {
	trimmed := strings.TrimSpace(s)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	out := make([]float32, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 32)
		if err != nil {
			return nil
		}
		out = append(out, float32(value))
	}
	return out
}
