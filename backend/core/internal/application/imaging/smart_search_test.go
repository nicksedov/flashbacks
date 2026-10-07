package imaging

import (
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"
	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSmartSearchTest(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	return db, cleanup
}

// --- SearchByExactTag tests ---

func TestSearchByExactTag_ExactMatch(t *testing.T) {
	db, cleanup := setupSmartSearchTest(t)
	defer cleanup()

	// Create image files
	img1 := testutil.SeedImageFile(t, db, "/photos/sunset.jpg", "abc", 1000)
	img2 := testutil.SeedImageFile(t, db, "/photos/mountain.jpg", "def", 2000)

	// Add tags
	db.Create(&domain.ImageTag{ImageFileID: img1.ID, Tag: "sunset"})
	db.Create(&domain.ImageTag{ImageFileID: img1.ID, Tag: "nature"})
	db.Create(&domain.ImageTag{ImageFileID: img2.ID, Tag: "mountain"})

	results, err := SearchByExactTag(db, "sunset")

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, img1.ID, results[0].ImageFileID)
	assert.Equal(t, "/photos/sunset.jpg", results[0].Path)
	assert.Equal(t, 1.0, results[0].Similarity)
	assert.Equal(t, MatchExact, results[0].MatchType)
	assert.Contains(t, results[0].Tags, "sunset")
	assert.Contains(t, results[0].Tags, "nature")
}

func TestSearchByExactTag_NoMatch(t *testing.T) {
	db, cleanup := setupSmartSearchTest(t)
	defer cleanup()

	img := testutil.SeedImageFile(t, db, "/photos/sunset.jpg", "abc", 1000)
	db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "sunset"})

	results, err := SearchByExactTag(db, "nonexistent")

	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSearchByExactTag_EmptyQuery(t *testing.T) {
	db, cleanup := setupSmartSearchTest(t)
	defer cleanup()

	results, err := SearchByExactTag(db, "")

	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSearchByExactTag_CaseInsensitive(t *testing.T) {
	db, cleanup := setupSmartSearchTest(t)
	defer cleanup()

	img := testutil.SeedImageFile(t, db, "/photos/sunset.jpg", "abc", 1000)
	db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "Sunset"})

	// Lowercase query should match uppercase tag via ILIKE
	results, err := SearchByExactTag(db, "sunset")

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, img.ID, results[0].ImageFileID)
}

func TestSearchByExactTag_WhitespaceCollapse(t *testing.T) {
	db, cleanup := setupSmartSearchTest(t)
	defer cleanup()

	img := testutil.SeedImageFile(t, db, "/photos/test.jpg", "abc", 1000)
	db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "mountain view"})

	// Multiple spaces in query should collapse and match
	results, err := SearchByExactTag(db, "mountain  view")

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, img.ID, results[0].ImageFileID)
}

func TestSearchByExactTag_TrimWhitespace(t *testing.T) {
	db, cleanup := setupSmartSearchTest(t)
	defer cleanup()

	img := testutil.SeedImageFile(t, db, "/photos/test.jpg", "abc", 1000)
	db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "sunset"})

	// Leading and trailing spaces should be trimmed
	results, err := SearchByExactTag(db, "  sunset  ")

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, img.ID, results[0].ImageFileID)
}

func TestSearchByExactTag_Deduplication(t *testing.T) {
	db, cleanup := setupSmartSearchTest(t)
	defer cleanup()

	img := testutil.SeedImageFile(t, db, "/photos/nature.jpg", "abc", 1000)
	// Same image has the tag multiple times (edge case)
	db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "nature"})
	db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "nature"})

	results, err := SearchByExactTag(db, "nature")

	require.NoError(t, err)
	require.Len(t, results, 1, "should deduplicate by image_file_id")
	assert.Equal(t, img.ID, results[0].ImageFileID)
}

func TestSearchByExactTag_MultipleMatches(t *testing.T) {
	db, cleanup := setupSmartSearchTest(t)
	defer cleanup()

	img1 := testutil.SeedImageFile(t, db, "/photos/sunset1.jpg", "abc", 1000)
	img2 := testutil.SeedImageFile(t, db, "/photos/sunset2.jpg", "def", 2000)
	img3 := testutil.SeedImageFile(t, db, "/photos/other.jpg", "ghi", 3000)

	db.Create(&domain.ImageTag{ImageFileID: img1.ID, Tag: "sunset"})
	db.Create(&domain.ImageTag{ImageFileID: img2.ID, Tag: "sunset"})
	db.Create(&domain.ImageTag{ImageFileID: img3.ID, Tag: "mountain"})

	results, err := SearchByExactTag(db, "sunset")

	require.NoError(t, err)
	require.Len(t, results, 2)
	// All should have similarity 1.0 and MatchExact
	for _, r := range results {
		assert.Equal(t, 1.0, r.Similarity)
		assert.Equal(t, MatchExact, r.MatchType)
	}
}

// --- MergeAndRankResults tests ---

func makeResult(id uint, path string, similarity float64, matchType MatchType) SmartSearchResult {
	return SmartSearchResult{
		ImageFileID: id,
		Path:        path,
		ModTime:     time.Now(),
		Similarity:  similarity,
		Tags:        []string{"tag1", "tag2"},
		MatchType:   matchType,
	}
}

func TestMergeAndRankResults_Empty(t *testing.T) {
	result := MergeAndRankResults(nil, nil)

	assert.Equal(t, 0, result.Total)
	assert.Empty(t, result.Images)
}

func TestMergeAndRankResults_ExactOnly(t *testing.T) {
	exact := []SmartSearchResult{
		makeResult(1, "/a.jpg", 1.0, MatchExact),
		makeResult(2, "/b.jpg", 1.0, MatchExact),
	}

	result := MergeAndRankResults(exact, nil)

	require.Len(t, result.Images, 2)
	assert.Equal(t, 2, result.Total)
	for _, r := range result.Images {
		assert.Equal(t, MatchExact, r.MatchType)
		assert.Equal(t, 1.0, r.Similarity)
	}
}

func TestMergeAndRankResults_EmbeddingOnly(t *testing.T) {
	emb := []SmartSearchResult{
		makeResult(1, "/a.jpg", 0.9, MatchEmbedding),
		makeResult(2, "/b.jpg", 0.7, MatchEmbedding),
		makeResult(3, "/c.jpg", 0.5, MatchEmbedding),
	}

	result := MergeAndRankResults(nil, emb)

	require.Len(t, result.Images, 3)
	assert.Equal(t, 3, result.Total)
	// Should be sorted by similarity descending
	assert.Equal(t, 0.9, result.Images[0].Similarity)
	assert.Equal(t, 0.7, result.Images[1].Similarity)
	assert.Equal(t, 0.5, result.Images[2].Similarity)
}

func TestMergeAndRankResults_Both(t *testing.T) {
	exact := []SmartSearchResult{
		makeResult(1, "/a.jpg", 1.0, MatchExact),
		makeResult(2, "/b.jpg", 1.0, MatchExact),
	}
	emb := []SmartSearchResult{
		makeResult(2, "/b.jpg", 0.8, MatchEmbedding),
		makeResult(3, "/c.jpg", 0.6, MatchEmbedding),
	}

	result := MergeAndRankResults(exact, emb)

	require.Len(t, result.Images, 3)

	// First: "both" (id=2) with boosted similarity
	assert.Equal(t, uint(2), result.Images[0].ImageFileID)
	assert.Equal(t, MatchBoth, result.Images[0].MatchType)
	assert.Equal(t, 1.5, result.Images[0].Similarity)

	// Second: exact-only (id=1)
	assert.Equal(t, uint(1), result.Images[1].ImageFileID)
	assert.Equal(t, MatchExact, result.Images[1].MatchType)
	assert.Equal(t, 1.0, result.Images[1].Similarity)

	// Third: embedding-only (id=3)
	assert.Equal(t, uint(3), result.Images[2].ImageFileID)
	assert.Equal(t, MatchEmbedding, result.Images[2].MatchType)
	assert.Equal(t, 0.6, result.Images[2].Similarity)
}

func TestMergeAndRankResults_DeduplicatesBoth(t *testing.T) {
	exact := []SmartSearchResult{
		makeResult(1, "/a.jpg", 1.0, MatchExact),
	}
	emb := []SmartSearchResult{
		makeResult(1, "/a.jpg", 0.85, MatchEmbedding),
	}

	result := MergeAndRankResults(exact, emb)

	require.Len(t, result.Images, 1, "duplicate should be merged into one result")
	assert.Equal(t, MatchBoth, result.Images[0].MatchType)
	assert.Equal(t, 1.5, result.Images[0].Similarity)
}

func TestMergeAndRankResults_OrderingWithinBoth(t *testing.T) {
	exact := []SmartSearchResult{
		makeResult(1, "/a.jpg", 1.0, MatchExact),
		makeResult(2, "/b.jpg", 1.0, MatchExact),
		makeResult(3, "/c.jpg", 1.0, MatchExact),
	}
	emb := []SmartSearchResult{
		makeResult(2, "/b.jpg", 0.5, MatchEmbedding),
		makeResult(1, "/a.jpg", 0.9, MatchEmbedding),
		makeResult(4, "/d.jpg", 0.7, MatchEmbedding),
	}

	result := MergeAndRankResults(exact, emb)

	require.Len(t, result.Images, 4)

	// Both group ordered by embedding similarity descending: a(0.9) then b(0.5)
	assert.Equal(t, MatchBoth, result.Images[0].MatchType)
	assert.Equal(t, uint(1), result.Images[0].ImageFileID)
	assert.Equal(t, 1.5, result.Images[0].Similarity)

	assert.Equal(t, MatchBoth, result.Images[1].MatchType)
	assert.Equal(t, uint(2), result.Images[1].ImageFileID)
	assert.Equal(t, 1.5, result.Images[1].Similarity)

	// Exact-only: c
	assert.Equal(t, MatchExact, result.Images[2].MatchType)
	assert.Equal(t, uint(3), result.Images[2].ImageFileID)

	// Embedding-only: d
	assert.Equal(t, MatchEmbedding, result.Images[3].MatchType)
	assert.Equal(t, uint(4), result.Images[3].ImageFileID)
}

func TestMergeAndRankResults_AllThreeGroupsPresent(t *testing.T) {
	// Create comprehensive test with all three groups
	exact := []SmartSearchResult{
		makeResult(1, "/both1.jpg", 1.0, MatchExact),
		makeResult(2, "/both2.jpg", 1.0, MatchExact),
		makeResult(3, "/exact_only.jpg", 1.0, MatchExact),
	}

	emb := []SmartSearchResult{
		makeResult(2, "/both2.jpg", 0.85, MatchEmbedding),
		makeResult(1, "/both1.jpg", 0.65, MatchEmbedding),
		makeResult(4, "/emb_only1.jpg", 0.55, MatchEmbedding),
		makeResult(5, "/emb_only2.jpg", 0.35, MatchEmbedding),
	}

	result := MergeAndRankResults(exact, emb)

	require.Len(t, result.Images, 5)

	// Group ordering: both(2), both(1), exact(3), emb(4), emb(5)
	assert.Equal(t, MatchBoth, result.Images[0].MatchType)
	assert.Equal(t, uint(2), result.Images[0].ImageFileID)
	assert.Equal(t, 1.5, result.Images[0].Similarity)

	assert.Equal(t, MatchBoth, result.Images[1].MatchType)
	assert.Equal(t, uint(1), result.Images[1].ImageFileID)
	assert.Equal(t, 1.5, result.Images[1].Similarity)

	assert.Equal(t, MatchExact, result.Images[2].MatchType)
	assert.Equal(t, uint(3), result.Images[2].ImageFileID)
	assert.Equal(t, 1.0, result.Images[2].Similarity)

	assert.Equal(t, MatchEmbedding, result.Images[3].MatchType)
	assert.Equal(t, uint(4), result.Images[3].ImageFileID)
	assert.Equal(t, 0.55, result.Images[3].Similarity)

	assert.Equal(t, MatchEmbedding, result.Images[4].MatchType)
	assert.Equal(t, uint(5), result.Images[4].ImageFileID)
	assert.Equal(t, 0.35, result.Images[4].Similarity)
}

// --- MatchType constants tests ---

func TestMatchType_Constants(t *testing.T) {
	assert.Equal(t, MatchType("exact"), MatchExact)
	assert.Equal(t, MatchType("embedding"), MatchEmbedding)
	assert.Equal(t, MatchType("both"), MatchBoth)
}

// --- sortBySimilarity tests ---

func TestSortBySimilarity_AlreadySorted(t *testing.T) {
	results := []SmartSearchResult{
		{Similarity: 0.9},
		{Similarity: 0.7},
		{Similarity: 0.5},
	}

	sortBySimilarity(results)

	assert.Equal(t, 0.9, results[0].Similarity)
	assert.Equal(t, 0.7, results[1].Similarity)
	assert.Equal(t, 0.5, results[2].Similarity)
}

func TestSortBySimilarity_ReverseOrder(t *testing.T) {
	results := []SmartSearchResult{
		{Similarity: 0.1},
		{Similarity: 0.5},
		{Similarity: 0.9},
	}

	sortBySimilarity(results)

	assert.Equal(t, 0.9, results[0].Similarity)
	assert.Equal(t, 0.5, results[1].Similarity)
	assert.Equal(t, 0.1, results[2].Similarity)
}

func TestSortBySimilarity_Empty(t *testing.T) {
	results := []SmartSearchResult{}
	sortBySimilarity(results)
	// Should not panic
}

func TestSortBySimilarity_SingleElement(t *testing.T) {
	results := []SmartSearchResult{{Similarity: 0.5}}
	sortBySimilarity(results)
	assert.Equal(t, 0.5, results[0].Similarity)
}
