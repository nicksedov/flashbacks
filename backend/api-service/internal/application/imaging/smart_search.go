package imaging

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/database"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"

	"gorm.io/gorm"
)

// SmartSearchResult represents a single result from a semantic search.
type SmartSearchResult struct {
	ImageFileID uint
	Path        string
	ModTime     time.Time
	Similarity  float64
	Tags        []string
}

// SmartSearchResponse holds the complete result of a semantic search query.
type SmartSearchResponse struct {
	Images []SmartSearchResult
	Total  int
	Query  string
}

// SearchByEmbedding performs semantic search over image tag embeddings using vector similarity.
// Shared by both the HTTP handler and the MCP server tool.
func SearchByEmbedding(db *gorm.DB, query string, limit int) (SmartSearchResponse, error) {
	if query == "" {
		return SmartSearchResponse{}, fmt.Errorf("query is required")
	}

	// Load embedding settings
	var settings domain.LlmSettings
	if err := db.First(&settings).Error; err != nil {
		return SmartSearchResponse{}, fmt.Errorf("LLM settings not found")
	}

	providerAlias := settings.EmbeddingProviderAlias
	if providerAlias == "" {
		providerAlias = settings.ActiveProvider
	}

	var provider domain.LlmProvider
	if err := db.Where("alias = ?", providerAlias).First(&provider).Error; err != nil {
		return SmartSearchResponse{}, fmt.Errorf("embedding provider '%s' not found", providerAlias)
	}

	modelName := settings.EmbeddingModel
	if modelName == "" {
		modelName = "qwen3-embedding:4b"
	}

	embeddingClient, err := llm.NewEmbeddingClient(provider.Name, provider.ApiUrl, provider.ApiKey, modelName)
	if err != nil {
		return SmartSearchResponse{}, fmt.Errorf("failed to create embedding client: %w", err)
	}

	// Embed the query
	queryEmbeddings, err := embeddingClient.Embed([]string{strings.ToLower(query)})
	if err != nil {
		return SmartSearchResponse{}, fmt.Errorf("failed to embed query: %w", err)
	}
	if len(queryEmbeddings) == 0 {
		return SmartSearchResponse{}, fmt.Errorf("empty embedding result")
	}

	// Convert to pgvector format for the SQL query
	vecStr := llm.Float32SliceToPgVector(queryEmbeddings[0])

	// Determine the per-model child table (safely quoted)
	childTable, err := database.QuotedEmbeddingTableName(modelName)
	if err != nil {
		return SmartSearchResponse{}, fmt.Errorf("invalid embedding table name: %w", err)
	}

	// Check if the child table exists; if not, return empty results
	if !database.EmbeddingTableExists(db, modelName) {
		return SmartSearchResponse{
			Images: []SmartSearchResult{},
			Total:  0,
			Query:  query,
		}, nil
	}

	// HNSW-friendly nearest-neighbor search: ORDER BY distance ASC LIMIT ?
	// Using the <=> (cosine distance) operator with ORDER BY + LIMIT enables
	// the HNSW index for approximate nearest-neighbor search.
	// We over-fetch (limit * 2) then deduplicate by image_file_id to handle
	// multiple tag embeddings per image.
	type searchResult struct {
		ImageFileID uint    `gorm:"column:image_file_id"`
		Distance    float64 `gorm:"column:distance"`
	}

	overFetchLimit := limit * 2
	querySQL := fmt.Sprintf(`
		SELECT te.image_file_id, (m.embedding <=> ?::halfvec) AS distance
		FROM %s m
		INNER JOIN tag_embeddings te ON te.id = m.tag_embeddings_id
		ORDER BY distance ASC
		LIMIT ?
	`, childTable)

	var rawResults []searchResult
	if err := db.Raw(querySQL, vecStr, overFetchLimit).Scan(&rawResults).Error; err != nil {
		return SmartSearchResponse{}, fmt.Errorf("semantic search query failed: %w", err)
	}

	// Deduplicate by image_file_id, keeping the closest distance
	seen := make(map[uint]bool)
	var results []searchResult
	for _, r := range rawResults {
		if !seen[r.ImageFileID] {
			seen[r.ImageFileID] = true
			results = append(results, r)
			if len(results) >= limit {
				break
			}
		}
	}

	if len(results) == 0 {
		return SmartSearchResponse{
			Images: []SmartSearchResult{},
			Total:  0,
			Query:  query,
		}, nil
	}

	// Collect image IDs and build similarity map (convert distance to similarity)
	imageIDs := make([]uint, len(results))
	similarityMap := make(map[uint]float64)
	for i, r := range results {
		imageIDs[i] = r.ImageFileID
		similarityMap[r.ImageFileID] = 1.0 - r.Distance // cosine distance → similarity
	}

	var files []domain.ImageFile
	db.Where("id IN ?", imageIDs).Find(&files)

	fileMap := make(map[uint]domain.ImageFile)
	for _, f := range files {
		fileMap[f.ID] = f
	}

	// Batch-fetch tags for all result images (avoids N+1)
	var allTags []domain.ImageTag
	db.Where("image_file_id IN ?", imageIDs).Find(&allTags)
	tagsMap := make(map[uint][]string)
	for _, t := range allTags {
		tagsMap[t.ImageFileID] = append(tagsMap[t.ImageFileID], t.Tag)
	}

	// Build results preserving similarity ranking order
	images := make([]SmartSearchResult, 0, len(files))
	for _, id := range imageIDs {
		f, ok := fileMap[id]
		if !ok {
			continue
		}

		tagStrs := tagsMap[id]
		if len(tagStrs) > 10 {
			tagStrs = tagStrs[:10]
		}
		sort.Strings(tagStrs)

		images = append(images, SmartSearchResult{
			ImageFileID: f.ID,
			Path:        f.Path,
			ModTime:     f.ModTime,
			Similarity:  similarityMap[id],
			Tags:        tagStrs,
		})
	}

	return SmartSearchResponse{
		Images: images,
		Total:  len(images),
		Query:  query,
	}, nil
}
