// embeddings-builder is a standalone CLI utility that populates the
// tag_embeddings table from existing image_tags records.
//
// Idempotent by design: tracks tag content changes via MD5 hashes stored
// in a separate embedding_setup_hashes table. On restart, skips images
// whose embeddings already exist and whose tag content has not changed.
//
// Usage:
//
//	go run ./cmd/embeddings-builder --dry-run          # count images needing embeddings
//	go run ./cmd/embeddings-builder                    # process all with default batch size (100)
//	go run ./cmd/embeddings-builder --batch-size 50    # smaller batches if API timeouts occur
//	go run ./cmd/embeddings-builder --force            # recompute all embeddings regardless of existing data
package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// sanitizeModelName converts an embedding model name to a valid PostgreSQL table name suffix.
var nonAlphanumUnderscore = regexp.MustCompile(`[^a-zA-Z0-9_]`)
var multiUnderscore = regexp.MustCompile(`_+`)

func sanitizeModelName(modelName string) string {
	s := nonAlphanumUnderscore.ReplaceAllString(modelName, "_")
	s = multiUnderscore.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

func embeddingTableName(modelName string) string {
	return "tag_embeddings_" + sanitizeModelName(modelName)
}

// hashTags computes an MD5 hex digest of the sorted, lowercased tag text.
func hashTags(tagText string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(tagText)))
}

func main() {
	batchSize := flag.Int("batch-size", 100, "Number of images per embedding API call")
	dryRun := flag.Bool("dry-run", false, "Count images needing embeddings without processing")
	force := flag.Bool("force", false, "Recompute all embeddings, ignoring existing data and tag hashes")
	flag.Parse()

	// Load environment and connect to PostgreSQL
	loadEnv()
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	log.Println("Connected to database")

	// Ensure the embedding_setup_hashes table exists for idempotency tracking
	ensureSetupHashesTable(db)

	// Resolve embedding provider and model from llm_settings + llm_providers
	embeddingClient, modelName, _, err := resolveEmbeddingClient(db)
	if err != nil {
		log.Fatalf("Failed to create embedding client: %v", err)
	}
	log.Printf("Embedding provider ready — model: %s", modelName)

	// Probe the embedding model to detect actual dimension.
	log.Println("Probing embedding model to detect vector dimension...")
	probe, err := embeddingClient.Embed([]string{"dimension probe"})
	if err != nil || len(probe) == 0 {
		log.Fatalf("Failed to probe embedding model: %v", err)
	}
	actualDim := len(probe[0])
	log.Printf("Embedding model produces %d-dimensional vectors", actualDim)

	// Ensure the per-model child table exists with the correct dimension
	childTable := embeddingTableName(modelName)
	log.Printf("Ensuring child table %s exists with halfvec(%d)...", childTable, actualDim)

	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			tag_embeddings_id BIGINT NOT NULL REFERENCES tag_embeddings(id) ON DELETE CASCADE,
			dimensity INT NOT NULL DEFAULT %d,
			embedding halfvec(%d) NOT NULL
		)`, childTable, actualDim, actualDim)
	if err := db.Exec(createSQL).Error; err != nil {
		log.Fatalf("Failed to create child table %s: %v", childTable, err)
	}

	// Create HNSW index on the child table
	db.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_vector ON %s USING hnsw (embedding halfvec_cosine_ops)", childTable, childTable))
	// Unique constraint on tag_embeddings_id
	db.Exec(fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_tag_embeddings_id ON %s (tag_embeddings_id)", childTable, childTable))
	log.Println("Child table and indexes ensured")

	if *force {
		log.Println("--force: will recompute ALL embeddings regardless of existing data")
	}

	// Count total images that need embeddings for this model.
	// Without --force, an image is "needing" embeddings when:
	//   1. No child table row exists for this model (missing embedding), OR
	//   2. A tag_hash exists in embedding_setup_hashes (previously processed;
	//      actual hash comparison happens in filterUnchanged() later).
	total := countNeedingEmbeddings(db, childTable, *force)

	if total == 0 {
		log.Println("All images already have up-to-date embeddings — nothing to do")
		return
	}

	log.Printf("Found %d image(s) needing embeddings", total)

	if *dryRun {
		fmt.Printf("\n--dry-run: %d image(s) would be processed (batch size: %d)\n", total, *batchSize)
		return
	}

	// Cursor-based batch loop
	var processed, skipped, failed int
	cursor := uint(0)
	startTime := time.Now()

	for {
		imageIDs, err := fetchBatch(db, childTable, cursor, *batchSize, *force)
		if err != nil {
			log.Fatalf("Failed to fetch image batch: %v", err)
		}
		if len(imageIDs) == 0 {
			break
		}

		// Batch-fetch tags for all images in this batch (avoids N+1)
		tagsByImage := batchFetchTags(db, imageIDs)

		// Build tag texts and hashes for each image in the batch
		tagTexts := make([]string, len(imageIDs))
		tagHashes := make([]string, len(imageIDs))
		for i, imgID := range imageIDs {
			tagStrs := tagsByImage[imgID]
			for j := range tagStrs {
				tagStrs[j] = strings.ToLower(tagStrs[j])
			}
			sort.Strings(tagStrs)
			tagTexts[i] = strings.Join(tagStrs, ", ")
			tagHashes[i] = hashTags(tagTexts[i])
		}

		// Without --force, filter out images whose tag hash hasn't changed.
		// Images appear in the batch when:
		//   - child table row is missing (needs initial embedding), OR
		//   - a tag_hash was stored (hash comparison detects actual tag changes).
		// Images whose stored hash matches the current computed hash are skipped.
		if !*force {
			filtered := filterUnchanged(db, imageIDs, tagHashes)
			if len(filtered) == 0 {
				skipped += len(imageIDs)
				cursor = imageIDs[len(imageIDs)-1]
				continue
			}
			skipped += len(imageIDs) - len(filtered)
			// Rebuild arrays with only the filtered set
			newIDs := make([]uint, len(filtered))
			newTexts := make([]string, len(filtered))
			newHashes := make([]string, len(filtered))
			for i, idx := range filtered {
				newIDs[i] = imageIDs[idx]
				newTexts[i] = tagTexts[idx]
				newHashes[i] = tagHashes[idx]
			}
			imageIDs, tagTexts, tagHashes = newIDs, newTexts, newHashes
		}

		// Call embedding API for the whole batch
		embeddings, err := embeddingClient.Embed(tagTexts)
		if err != nil {
			log.Printf("Embedding API failed for batch (cursor=%d): %v — skipping batch", cursor, err)
			failed += len(imageIDs)
			cursor = imageIDs[len(imageIDs)-1]
			continue
		}

		// Upsert embeddings for each image
		for i, imgID := range imageIDs {
			if i >= len(embeddings) {
				break
			}
			vecStr := float32SliceToPgVector(embeddings[i])
			tagCount := 0
			if tagTexts[i] != "" {
				tagCount = strings.Count(tagTexts[i], ",") + 1
			}

			if err := upsertEmbedding(db, imgID, childTable, actualDim, vecStr, tagCount); err != nil {
				log.Printf("Failed to save embedding for image %d: %v", imgID, err)
				failed++
				continue
			}
			// Save the tag hash for future idempotency checks
			saveTagHash(db, imgID, tagHashes[i])
		}

		processed += len(imageIDs)
		cursor = imageIDs[len(imageIDs)-1]
		elapsed := time.Since(startTime).Round(time.Second)
		pct := float64(processed+skipped) / float64(total) * 100
		log.Printf("Progress: %d/%d (%.1f%%) — processed %d, skipped %d, failed %d — elapsed %s",
			processed+skipped, total, pct, processed, skipped, failed, elapsed)
	}

	log.Printf("Done — processed %d, skipped %d, failed %d in %s",
		processed, skipped, failed, time.Since(startTime).Round(time.Second))
}

// ensureSetupHashesTable creates the embedding_setup_hashes table if it does not exist.
func ensureSetupHashesTable(db *gorm.DB) {
	sql := `
		CREATE TABLE IF NOT EXISTS embedding_setup_hashes (
			id BIGSERIAL PRIMARY KEY,
			image_file_id BIGINT NOT NULL UNIQUE REFERENCES image_files(id) ON DELETE CASCADE,
			tag_hash VARCHAR(32) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`
	if err := db.Exec(sql).Error; err != nil {
		log.Fatalf("Failed to create embedding_setup_hashes table: %v", err)
	}
	log.Println("embedding_setup_hashes table ensured")
}

// countNeedingEmbeddings returns the number of images that need embedding computation.
// Without --force, an image needs embeddings when:
//  1. No child table row exists for this model (missing embedding), OR
//  2. A tag_hash exists in embedding_setup_hashes (image was previously processed)
//     — the actual hash comparison happens later in filterUnchanged().
func countNeedingEmbeddings(db *gorm.DB, childTable string, force bool) int64 {
	var total int64
	if force {
		// With --force, count all images that have tags
		db.Table("image_files").
			Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id").
			Distinct("image_files.id").
			Count(&total)
	} else {
		// Without --force, count images where:
		//   - child table row is missing, OR
		//   - a tag_hash was stored (may be outdated due to tag changes)
		db.Table("image_files").
			Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id").
			Joins("LEFT JOIN tag_embeddings ON tag_embeddings.image_file_id = image_files.id").
			Joins(fmt.Sprintf("LEFT JOIN %s ON %s.tag_embeddings_id = tag_embeddings.id", childTable, childTable)).
			Joins("LEFT JOIN embedding_setup_hashes ON embedding_setup_hashes.image_file_id = image_files.id").
			Where(fmt.Sprintf("%s.id IS NULL OR embedding_setup_hashes.tag_hash != ''", childTable)).
			Distinct("image_files.id").
			Count(&total)
	}
	return total
}

// fetchBatch retrieves the next batch of image IDs that need embeddings.
// Without --force, includes images that are missing child records or have a
// stored tag_hash (to detect tag changes via hash comparison).
func fetchBatch(db *gorm.DB, childTable string, cursor uint, batchSize int, force bool) ([]uint, error) {
	var imageIDs []uint
	q := db.Table("image_files").
		Select("DISTINCT image_files.id").
		Joins("INNER JOIN image_tags ON image_tags.image_file_id = image_files.id")

	if !force {
		q = q.Joins("LEFT JOIN tag_embeddings ON tag_embeddings.image_file_id = image_files.id").
			Joins(fmt.Sprintf("LEFT JOIN %s ON %s.tag_embeddings_id = tag_embeddings.id", childTable, childTable)).
			Joins("LEFT JOIN embedding_setup_hashes ON embedding_setup_hashes.image_file_id = image_files.id").
			Where(fmt.Sprintf("(%s.id IS NULL OR embedding_setup_hashes.tag_hash != '') AND image_files.id > ?", childTable), cursor)
	} else {
		q = q.Where("image_files.id > ?", cursor)
	}

	err := q.Order("image_files.id ASC").
		Limit(batchSize).
		Pluck("image_files.id", &imageIDs).Error
	return imageIDs, err
}

// batchFetchTags retrieves all tags for the given image IDs in a single query.
// Returns a map of imageFileID -> []tagString.
func batchFetchTags(db *gorm.DB, imageIDs []uint) map[uint][]string {
	var tags []ImageTag
	db.Where("image_file_id IN ?", imageIDs).Find(&tags)
	tagsByImage := make(map[uint][]string)
	for _, t := range tags {
		tagsByImage[t.ImageFileID] = append(tagsByImage[t.ImageFileID], t.Tag)
	}
	return tagsByImage
}

// filterUnchanged returns indices of images whose tag hash differs from the stored hash.
// Images without an embedding_setup_hashes record (hash is empty) are always included
// (they need an initial embedding).
// Images with a matching hash are excluded — their embedding is already up-to-date.
func filterUnchanged(db *gorm.DB, imageIDs []uint, tagHashes []string) []int {
	// Fetch existing hash records for this batch
	var records []EmbeddingSetupHash
	db.Where("image_file_id IN ?", imageIDs).Find(&records)
	hashByImage := make(map[uint]string)
	for _, r := range records {
		hashByImage[r.ImageFileID] = r.TagHash
	}

	var indices []int
	for i, imgID := range imageIDs {
		storedHash := hashByImage[imgID]
		if storedHash == "" || storedHash != tagHashes[i] {
			indices = append(indices, i)
		}
	}
	return indices
}

// saveTagHash creates or updates the tag hash record in embedding_setup_hashes.
func saveTagHash(db *gorm.DB, imageFileID uint, tagHash string) {
	var record EmbeddingSetupHash
	result := db.Where("image_file_id = ?", imageFileID).First(&record)
	if result.Error == gorm.ErrRecordNotFound {
		record = EmbeddingSetupHash{ImageFileID: imageFileID, TagHash: tagHash}
		if err := db.Create(&record).Error; err != nil {
			log.Printf("Failed to create tag hash record for image %d: %v", imageFileID, err)
		}
	} else if result.Error == nil {
		db.Model(&record).Update("tag_hash", tagHash)
	}
}

// upsertEmbedding atomically upserts the parent TagEmbedding record and the child table row.
func upsertEmbedding(db *gorm.DB, imageFileID uint, childTable string, dimension int, vecStr string, tagCount int) error {
	// Upsert parent record: find existing or create new
	var parent TagEmbedding
	result := db.Where("image_file_id = ?", imageFileID).First(&parent)
	if result.Error == gorm.ErrRecordNotFound {
		parent = TagEmbedding{ImageFileID: imageFileID, TagCount: tagCount}
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

// resolveEmbeddingClient reads llm_settings and llm_providers from the DB
// and creates an EmbeddingClient for the configured provider and model.
// Returns the client, model name, embedding dimension, and any error.
func resolveEmbeddingClient(db *gorm.DB) (EmbeddingClient, string, int, error) {
	var settings LlmSettings
	if err := db.First(&settings).Error; err != nil {
		return nil, "", 0, fmt.Errorf("llm_settings not found: %w", err)
	}

	providerAlias := settings.EmbeddingProviderAlias
	if providerAlias == "" {
		providerAlias = settings.ActiveProvider
	}

	var provider LlmProvider
	if err := db.Where("alias = ?", providerAlias).First(&provider).Error; err != nil {
		return nil, "", 0, fmt.Errorf("embedding provider '%s' not found in llm_providers: %w", providerAlias, err)
	}

	modelName := settings.EmbeddingModel
	if modelName == "" {
		modelName = "qwen3-embedding:4b"
	}

	dimension := settings.EmbeddingDimension
	if dimension == 0 {
		dimension = 1024
	}

	client, err := newEmbeddingClient(provider.Name, provider.ApiUrl, provider.ApiKey, modelName)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to create embedding client: %w", err)
	}
	return client, modelName, dimension, nil
}
