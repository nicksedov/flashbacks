// Package faces implements the background face recognition job.
package faces

import (
	"context"
	"fmt"
	"log"
	"os"
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

// minClusterSize is the default HDBSCAN minClusterSize used by the job.
const minClusterSize = 5

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

// runPass processes every unprocessed image, then clusters and assigns people.
func (m *Manager) runPass(stopCh <-chan struct{}) {
	var total int64
	m.db.Table("image_files").
		Joins("LEFT JOIN faces ON faces.image_file_id = image_files.id").
		Where("faces.id IS NULL").
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
		err := m.db.Table("image_files").
			Select("image_files.*").
			Joins("LEFT JOIN faces ON faces.image_file_id = image_files.id").
			Where("faces.id IS NULL AND image_files.id > ?", cursor).
			Order("image_files.id ASC").
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

	m.clusterAssign(stopCh)
}

// processImage detects and embeds faces on a single image, persisting each.
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	boxes, err := m.client.Detect(ctx, data)
	if err != nil {
		m.setError(fmt.Sprintf("detect %s: %v", imageFile.Path, err))
		return
	}

	for _, box := range boxes {
		vec, err := m.client.Embed(ctx, data, &box)
		if err != nil {
			m.setError(fmt.Sprintf("embed %s: %v", imageFile.Path, err))
			continue
		}
		m.persistFace(imageFile.ID, box, vec)
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
// person (one person per non-noise cluster).
func (m *Manager) clusterAssign(stopCh <-chan struct{}) {
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
		faceIDs = append(faceIDs, face.ID)
		embeddings = append(embeddings, vec)
	}
	if len(embeddings) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	labels, err := m.client.Cluster(ctx, embeddings, m.clusterSize)
	if err != nil {
		m.setError(fmt.Sprintf("cluster: %v", err))
		return
	}

	personByLabel := make(map[int]uint, len(labels))
	for i, label := range labels {
		if i >= len(faceIDs) || label == -1 {
			continue
		}
		personID, ok := personByLabel[label]
		if !ok {
			coverFaceID := faceIDs[i]
			person := domain.Person{CoverFaceID: &coverFaceID}
			if err := m.db.Create(&person).Error; err != nil {
				m.setError(fmt.Sprintf("create person: %v", err))
				continue
			}
			personID = person.ID
			personByLabel[label] = personID
		}
		if err := m.db.Model(&domain.Face{}).Where("id = ?", faceIDs[i]).Update("person_id", personID).Error; err != nil {
			m.setError(fmt.Sprintf("assign person: %v", err))
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
