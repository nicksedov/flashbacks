package imaging

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/flashbacks/api-service/internal/application/background"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/database"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"

	"gorm.io/gorm"
)

// EmbeddingBackfillProgress holds the current backfill progress.
type EmbeddingBackfillProgress struct {
	Total     int    `json:"total"`
	Processed int    `json:"processed"`
	Remaining int    `json:"remaining"`
	LastError string `json:"lastError"`
}

// EmbeddingBackfillStatus holds the current status of the embedding backfill.
type EmbeddingBackfillStatus struct {
	Running  bool                      `json:"running"`
	Progress EmbeddingBackfillProgress `json:"progress"`
}

// EmbeddingBackfillManager generates embeddings for images that have tags but no embeddings.
type EmbeddingBackfillManager struct {
	*background.Manager
	mu       sync.Mutex
	db       *gorm.DB
	progress EmbeddingBackfillProgress
}

// NewEmbeddingBackfillManager creates a new embedding backfill manager.
func NewEmbeddingBackfillManager(db *gorm.DB) *EmbeddingBackfillManager {
	return &EmbeddingBackfillManager{
		Manager: background.New("embedding-backfill"),
		db:      db,
	}
}

// Start begins the embedding backfill process in a goroutine.
func (m *EmbeddingBackfillManager) Start() error {
	if !m.TryStart() {
		return fmt.Errorf("embedding backfill already running")
	}
	m.mu.Lock()
	m.progress = EmbeddingBackfillProgress{}
	m.mu.Unlock()

	go m.run()
	return nil
}

// GetStatus returns the current backfill status.
func (m *EmbeddingBackfillManager) GetStatus() EmbeddingBackfillStatus {
	m.mu.Lock()
	progress := m.progress
	m.mu.Unlock()
	return EmbeddingBackfillStatus{
		Running:  m.IsRunning(),
		Progress: progress,
	}
}

// run is the main backfill loop.
func (m *EmbeddingBackfillManager) run() {
	log.Println("Embedding backfill: started")
	defer func() {
		m.MarkStopped()
		log.Println("Embedding backfill: finished")
	}()

	// Create embedding client with dimension
	embeddingClient, modelName, dimension, err := resolveEmbeddingClient(m.db)
	if err != nil {
		m.setError(fmt.Sprintf("Failed to create embedding client: %v", err))
		return
	}

	// Ensure the per-model child table exists
	if err := database.EnsureEmbeddingTable(m.db, modelName, dimension); err != nil {
		m.setError(fmt.Sprintf("Failed to ensure embedding table: %v", err))
		return
	}

	childTable, err := database.QuotedEmbeddingTableName(modelName)
	if err != nil {
		m.setError(fmt.Sprintf("Invalid embedding table name: %v", err))
		return
	}

	// Count images that need embeddings for this model
	var total int64
	m.db.Table("image_files").
		Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id").
		Joins("LEFT JOIN tag_embeddings ON tag_embeddings.image_file_id = image_files.id").
		Joins(fmt.Sprintf("LEFT JOIN %s ON %s.tag_embeddings_id = tag_embeddings.id", childTable, childTable)).
		Where(fmt.Sprintf("%s.id IS NULL", childTable)).
		Distinct("image_files.id").
		Count(&total)

	if total == 0 {
		log.Println("Embedding backfill: all images already have embeddings for this model")
		return
	}

	m.mu.Lock()
	m.progress.Total = int(total)
	m.progress.Remaining = int(total)
	m.mu.Unlock()

	// Read batch size from embedding settings (default 50)
	batchSize := 50
	var embSettings domain.EmbeddingSettings
	if err := m.db.First(&embSettings).Error; err == nil && embSettings.BatchSize > 0 {
		batchSize = embSettings.BatchSize
	}

	cursor := uint(0)

	for {
		select {
		case <-m.Done():
			log.Println("Embedding backfill: stopped by user")
			return
		default:
		}

		type imageWithTags struct {
			ImageFileID uint
			Tags        string
		}

		var results []imageWithTags
		err := m.db.Table("image_files").
			Select("image_files.id as image_file_id").
			Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id").
			Joins("LEFT JOIN tag_embeddings ON tag_embeddings.image_file_id = image_files.id").
			Joins(fmt.Sprintf("LEFT JOIN %s ON %s.tag_embeddings_id = tag_embeddings.id", childTable, childTable)).
			Where(fmt.Sprintf("%s.id IS NULL AND image_files.id > ?", childTable), cursor).
			Group("image_files.id").
			Order("image_files.id ASC").
			Limit(batchSize).
			Find(&results).Error

		if err != nil || len(results) == 0 {
			log.Printf("Embedding backfill: no more images to process (cursor=%d)", cursor)
			break
		}

		imageIDs := make([]uint, len(results))
		for i, r := range results {
			imageIDs[i] = r.ImageFileID
		}

		// Batch-fetch tags for all images in this batch (avoids N+1)
		var allTags []domain.ImageTag
		m.db.Where("image_file_id IN ?", imageIDs).Find(&allTags)
		tagsByImage := make(map[uint][]string)
		for _, t := range allTags {
			tagsByImage[t.ImageFileID] = append(tagsByImage[t.ImageFileID], t.Tag)
		}

		tagTexts := make([]string, len(imageIDs))
		for i, imgID := range imageIDs {
			tagStrs := tagsByImage[imgID]
			for j := range tagStrs {
				tagStrs[j] = strings.ToLower(tagStrs[j])
			}
			sort.Strings(tagStrs)
			tagTexts[i] = strings.Join(tagStrs, ", ")
		}

		embeddings, err := embeddingClient.Embed(context.Background(), tagTexts)
		if err != nil {
			m.setError(fmt.Sprintf("Embedding API failed: %v", err))
			log.Printf("Embedding backfill: embedding API error: %v", err)
			cursor = imageIDs[len(imageIDs)-1]
			continue
		}

		for i, imgID := range imageIDs {
			if i >= len(embeddings) {
				break
			}
			vecStr := llm.Float32SliceToPgVector(embeddings[i])
			tagCount := strings.Count(tagTexts[i], ",") + 1
			if tagTexts[i] == "" {
				tagCount = 0
			}

			if err := upsertEmbedding(m.db, imgID, childTable, dimension, vecStr, tagCount); err != nil {
				log.Printf("Embedding backfill: failed to save embedding for image %d: %v", imgID, err)
			}
		}

		cursor = imageIDs[len(imageIDs)-1]
		m.mu.Lock()
		m.progress.Processed += len(imageIDs)
		m.progress.Remaining = m.progress.Total - m.progress.Processed
		m.mu.Unlock()

		log.Printf("Embedding backfill: processed %d/%d images", m.progress.Processed, m.progress.Total)
	}
}

// upsertEmbedding upserts the parent TagEmbedding record and the child table row.
func upsertEmbedding(db *gorm.DB, imageFileID uint, childTable string, dimension int, vecStr string, tagCount int) error {
	var parent domain.TagEmbedding
	result := db.Where("image_file_id = ?", imageFileID).First(&parent)
	if result.Error == gorm.ErrRecordNotFound {
		parent = domain.TagEmbedding{ImageFileID: imageFileID, TagCount: tagCount}
		if err := db.Create(&parent).Error; err != nil {
			return fmt.Errorf("failed to create parent embedding record: %w", err)
		}
	} else if result.Error != nil {
		return fmt.Errorf("failed to query parent embedding record: %w", result.Error)
	} else {
		db.Model(&parent).Update("tag_count", tagCount)
	}

	// Atomic upsert: insert or update child embedding row
	if err := db.Exec(fmt.Sprintf(
		"INSERT INTO %s (tag_embeddings_id, dimensity, embedding) VALUES (?, ?, ?::halfvec) "+
			"ON CONFLICT (tag_embeddings_id) DO UPDATE SET dimensity = EXCLUDED.dimensity, embedding = EXCLUDED.embedding",
		childTable), parent.ID, dimension, vecStr).Error; err != nil {
		return fmt.Errorf("failed to upsert child embedding row: %w", err)
	}

	return nil
}

// resolveEmbeddingClient loads LLM settings and creates an EmbeddingClient.
// Returns the client, model name, embedding dimension, and any error.
func resolveEmbeddingClient(db *gorm.DB) (llm.EmbeddingClient, string, int, error) {
	var embInstrument domain.LlmInstrumentSettings
	if err := db.Where("type = ?", domain.InstrumentEmbedding).Preload("Provider").First(&embInstrument).Error; err != nil {
		return nil, "", 0, fmt.Errorf("embedding instrument not configured")
	}
	provider := embInstrument.Provider
	modelName := embInstrument.Model
	if modelName == "" {
		modelName = "qwen3-embedding:4b"
	}

	var embSettings domain.EmbeddingSettings
	dimension := 1024
	if err := db.First(&embSettings).Error; err == nil && embSettings.Dimension > 0 {
		dimension = embSettings.Dimension
	}

	client, err := llm.NewEmbeddingClient(provider.Name, provider.ApiUrl, provider.ApiKey, modelName)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to create embedding client: %w", err)
	}
	return client, modelName, dimension, nil
}

// setError updates the last error in progress.
func (m *EmbeddingBackfillManager) setError(msg string) {
	m.mu.Lock()
	m.progress.LastError = msg
	m.mu.Unlock()
}

// GenerateAndSaveEmbedding generates and saves an embedding for a single image's tags.
// Called from the real-time hook after tags are saved.
func GenerateAndSaveEmbedding(db *gorm.DB, imageFileID uint, tags []string) {
	if len(tags) == 0 {
		return
	}

	embeddingClient, modelName, dimension, err := resolveEmbeddingClient(db)
	if err != nil {
		log.Printf("Embedding hook: %v", err)
		return
	}

	if err := database.EnsureEmbeddingTable(db, modelName, dimension); err != nil {
		log.Printf("Embedding hook: failed to ensure embedding table: %v", err)
		return
	}

	tagStrs := make([]string, len(tags))
	for i, t := range tags {
		tagStrs[i] = strings.ToLower(t)
	}
	sort.Strings(tagStrs)
	text := strings.Join(tagStrs, ", ")

	embeddings, err := embeddingClient.Embed(context.Background(), []string{text})
	if err != nil {
		log.Printf("Embedding hook: embed API failed for image %d: %v", imageFileID, err)
		return
	}

	if len(embeddings) == 0 {
		return
	}

	vecStr := llm.Float32SliceToPgVector(embeddings[0])
	childTable, err := database.QuotedEmbeddingTableName(modelName)
	if err != nil {
		log.Printf("Embedding hook: invalid table name: %v", err)
		return
	}

	if err := upsertEmbedding(db, imageFileID, childTable, dimension, vecStr, len(tags)); err != nil {
		log.Printf("Embedding hook: failed to save embedding for image %d: %v", imageFileID, err)
	}
}
