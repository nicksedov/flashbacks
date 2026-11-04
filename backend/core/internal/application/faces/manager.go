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
	mu          sync.Mutex
	db          *gorm.DB
	client      facesclient.Client
	schedule    domain.Schedule
	scheduleCh  chan struct{}
	progress    Progress
	clusterSize int
}

// NewManager creates a face recognition manager.
func NewManager(db *gorm.DB, client facesclient.Client) *Manager {
	return &Manager{
		Manager:     background.New("face-recognition"),
		db:          db,
		client:      client,
		schedule:    domain.DefaultSchedule(domain.ScheduleFaceRecognition),
		scheduleCh:  make(chan struct{}),
		clusterSize: minClusterSize,
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
func (m *Manager) runPass(stopCh <-chan struct{}) {
	var total int64
	m.db.Model(&domain.ImageFile{}).
		Where("faces_scan_version <> ?", facesScanVersion).
		Count(&total)
	if total == 0 {
		return
	}

	m.mu.Lock()
	m.progress = Progress{Total: int(total)}
	m.mu.Unlock()

	cursor := uint(0)
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		var imageFile domain.ImageFile
		err := m.db.Where("faces_scan_version <> ? AND id > ?", facesScanVersion, cursor).
			Order("id ASC").
			First(&imageFile).Error
		if err != nil {
			break
		}

		m.processImage(stopCh, imageFile)
		cursor = imageFile.ID

		m.mu.Lock()
		m.progress.Processed++
		m.mu.Unlock()
	}

	// Detection is finished, so the per-image face counts are final and serve
	// both the covers of the new clusters and the refresh of stored ones.
	counts := m.imageFaceCounts()
	m.clusterAssign(stopCh, counts)
	m.refreshPersonCovers(counts)
}

// faceRef identifies one face of a cluster together with the photo it was
// detected on; choosing a cover needs both.
type faceRef struct {
	faceID      uint
	imageFileID uint
	confidence  float32
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

// processImage analyses a single image in one service call and stores the faces.
func (m *Manager) processImage(stopCh <-chan struct{}, imageFile domain.ImageFile) {
	select {
	case <-stopCh:
		return
	default:
	}

	data, err := os.ReadFile(imageFile.Path)
	if err != nil {
		m.setError(fmt.Sprintf("read %s: %v", imageFile.Path, err))
		return
	}

	// One analyse call per image: embedding a face by re-detecting the whole
	// image costs a full inference each, which a group photo with dozens of
	// faces cannot afford.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	faces, err := m.client.Analyze(ctx, data)
	if err != nil {
		// The scan version stays untouched, so the image is retried next pass.
		m.setError(fmt.Sprintf("analyze %s: %v", imageFile.Path, err))
		return
	}

	m.replaceFaces(imageFile.ID, faces)
	m.markScanned(imageFile.ID)
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

	members := make([]faceRef, 0, len(unassigned))
	embeddings := make([][]float32, 0, len(unassigned))
	for _, face := range unassigned {
		var embedding domain.FaceEmbedding
		if err := m.db.Where("face_id = ?", face.ID).First(&embedding).Error; err != nil {
			continue
		}
		vec := parsePgVector(embedding.Embedding)
		if len(vec) == 0 {
			continue
		}
		members = append(members, faceRef{
			faceID:      face.ID,
			imageFileID: face.ImageFileID,
			confidence:  face.Confidence,
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	labels, err := m.client.Cluster(ctx, embeddings, m.clusterSize, solo)
	if err != nil {
		m.setError(fmt.Sprintf("cluster: %v", err))
		return
	}

	byLabel := make(map[int][]faceRef, len(labels))
	for i, label := range labels {
		if i >= len(members) || label == -1 {
			continue
		}
		byLabel[label] = append(byLabel[label], members[i])
	}

	labelIDs := make([]int, 0, len(byLabel))
	for label := range byLabel {
		labelIDs = append(labelIDs, label)
	}
	sort.Ints(labelIDs)

	for _, label := range labelIDs {
		cluster := byLabel[label]
		coverFaceID := chooseCover(cluster, facesPerImage)
		person := domain.Person{CoverFaceID: &coverFaceID}
		if err := m.db.Create(&person).Error; err != nil {
			m.setError(fmt.Sprintf("create person: %v", err))
			continue
		}
		for _, member := range cluster {
			if err := m.db.Model(&domain.Face{}).Where("id = ?", member.faceID).
				Update("person_id", person.ID).Error; err != nil {
				m.setError(fmt.Sprintf("assign person: %v", err))
			}
		}
	}
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
