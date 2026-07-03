package imaging

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/database"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"

	"gorm.io/gorm"
)

// MatchType indicates how a search result was matched.
type MatchType string

const (
	MatchExact     MatchType = "exact"
	MatchEmbedding MatchType = "embedding"
	MatchBoth      MatchType = "both"
)

// SmartSearchResult represents a single result from a semantic search.
type SmartSearchResult struct {
	ImageFileID uint
	Path        string
	ModTime     time.Time
	Similarity  float64
	Tags        []string
	MatchType   MatchType
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
	queryEmbeddings, err := embeddingClient.Embed(context.Background(), []string{strings.ToLower(query)})
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

	// Load file records and tags in batch
	fullResults := loadFilesAndTags(db, imageIDs, similarityMap, MatchEmbedding)

	return SmartSearchResponse{
		Images: fullResults,
		Total:  len(fullResults),
		Query:  query,
	}, nil
}

// SearchByExactTag finds images whose tags match the query after normalization.
// Query normalization: trim leading/trailing whitespace, collapse multiple spaces into single.
// Comparison is case-insensitive via LOWER().
func SearchByExactTag(db *gorm.DB, query string) ([]SmartSearchResult, error) {
	// Normalize: trim + collapse consecutive whitespace to single space
	normalized := strings.Join(strings.Fields(query), " ")
	if normalized == "" {
		return nil, nil
	}

	var tags []domain.ImageTag
	db.Where("LOWER(tag) = LOWER(?)", normalized).Find(&tags)

	if len(tags) == 0 {
		return nil, nil
	}

	// Deduplicate by image_file_id
	seen := make(map[uint]bool)
	imageIDs := make([]uint, 0, len(tags))
	for _, t := range tags {
		if !seen[t.ImageFileID] {
			seen[t.ImageFileID] = true
			imageIDs = append(imageIDs, t.ImageFileID)
		}
	}

	// Exact tag matches get Similarity = 1.0
	similarityMap := make(map[uint]float64)
	for _, id := range imageIDs {
		similarityMap[id] = 1.0
	}

	return loadFilesAndTags(db, imageIDs, similarityMap, MatchExact), nil
}

// loadFilesAndTags batch-loads ImageFile records and tags for the given image IDs,
// then builds SmartSearchResult slices ordered by imageIDs.
func loadFilesAndTags(db *gorm.DB, imageIDs []uint, similarityMap map[uint]float64, matchType MatchType) []SmartSearchResult {
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

	// Build results preserving imageIDs order
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

		sim := similarityMap[id]

		images = append(images, SmartSearchResult{
			ImageFileID: f.ID,
			Path:        f.Path,
			ModTime:     f.ModTime,
			Similarity:  sim,
			Tags:        tagStrs,
			MatchType:   matchType,
		})
	}

	return images
}

// MergeAndRankResults combines exact-match and embedding search results,
// ranks them by match type priority, and removes duplicates.
//
// Ranking (descending priority):
//  1. Both (exact tag + embedding) — Similarity = 1.5 (boosted)
//  2. Exact tag only — Similarity = 1.0
//  3. Embedding only — Similarity = original (0.0–1.0)
func MergeAndRankResults(exactResults, embeddingResults []SmartSearchResult) SmartSearchResponse {
	if len(exactResults) == 0 && len(embeddingResults) == 0 {
		return SmartSearchResponse{
			Images: []SmartSearchResult{},
			Total:  0,
		}
	}

	// Build index of embedding results by ImageFileID
	embeddingByID := make(map[uint]SmartSearchResult)
	for _, r := range embeddingResults {
		embeddingByID[r.ImageFileID] = r
	}

	// Build index of exact results by ImageFileID
	exactByID := make(map[uint]SmartSearchResult)
	for _, r := range exactResults {
		exactByID[r.ImageFileID] = r
	}

	// Group results into three categories
	var bothResults []SmartSearchResult
	var exactOnly []SmartSearchResult
	var embeddingOnly []SmartSearchResult

	// Find intersections (both)
	seenBoth := make(map[uint]bool)
	// Track original embedding similarity for sorting before boosting
	bothEmbSim := make(map[uint]float64)
	for _, r := range exactResults {
		if emb, ok := embeddingByID[r.ImageFileID]; ok {
			r.MatchType = MatchBoth
			r.Similarity = emb.Similarity // temporarily use embedding sim for sorting
			bothResults = append(bothResults, r)
			seenBoth[r.ImageFileID] = true
			bothEmbSim[r.ImageFileID] = emb.Similarity
		}
	}

	// Exact-only
	for _, r := range exactResults {
		if !seenBoth[r.ImageFileID] {
			r.MatchType = MatchExact
			r.Similarity = 1.0
			exactOnly = append(exactOnly, r)
		}
	}

	// Embedding-only
	for _, r := range embeddingResults {
		if !seenBoth[r.ImageFileID] {
			r.MatchType = MatchEmbedding
			embeddingOnly = append(embeddingOnly, r)
		}
	}

	// Sort "both" group by embedding similarity (descending)
	sortBySimilarity(bothResults)
	// Boost similarity for "both" results after sorting
	for i := range bothResults {
		bothResults[i].Similarity = 1.5
	}
	// Sort embedding-only by similarity (descending)
	sortBySimilarity(embeddingOnly)

	// Merge all three groups: both → exact_only → embedding_only
	all := make([]SmartSearchResult, 0, len(bothResults)+len(exactOnly)+len(embeddingOnly))
	all = append(all, bothResults...)
	all = append(all, exactOnly...)
	all = append(all, embeddingOnly...)

	// Extract the query from embedding results (or exact results as fallback)
	query := ""
	if len(embeddingResults) > 0 {
		// Query is not tracked per-result; we just return results
	}

	return SmartSearchResponse{
		Images: all,
		Total:  len(all),
		Query:  query,
	}
}

// sortBySimilarity sorts results by Similarity in descending order (highest first).
func sortBySimilarity(results []SmartSearchResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
}
