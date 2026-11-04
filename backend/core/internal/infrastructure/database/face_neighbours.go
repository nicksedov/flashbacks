package database

import (
	"context"
	"fmt"

	"github.com/flashbacks/core/internal/application/faces"

	"gorm.io/gorm"
)

// neighbourQuery probes the HNSW index once per seed face through a lateral
// join: the seed's nearest faces inside the similarity radius are answered by
// the index, so the gallery never has to be read into memory.
const neighbourQuery = `
SELECT src.face_id AS face_id, n.face_id AS candidate_id, n.similarity AS similarity
FROM unnest(?::bigint[]) AS src(face_id)
JOIN face_embeddings seed ON seed.face_id = src.face_id
CROSS JOIN LATERAL (
	SELECT e.face_id, 1 - (e.embedding <=> seed.embedding) AS similarity
	FROM face_embeddings e
	WHERE e.face_id <> seed.face_id
	  AND 1 - (e.embedding <=> seed.embedding) >= ?
	ORDER BY e.embedding <=> seed.embedding
	LIMIT ?
) AS n`

// pgvectorWithHalfvecQuery reports whether pgvector is new enough for the
// halfvec HNSW index (0.7.0 introduced the halfvec type).
const pgvectorWithHalfvecQuery = `
SELECT EXISTS (
	SELECT 1 FROM pg_extension
	WHERE extname = 'vector'
	  AND (split_part(extversion, '.', 1)::int > 0 OR split_part(extversion, '.', 2)::int >= 7)
)`

// FaceNeighbours answers face nearest-neighbour queries from PostgreSQL and
// pgvector.
type FaceNeighbours struct {
	db *gorm.DB
}

// NewFaceNeighbours creates the pgvector-backed neighbour source.
func NewFaceNeighbours(db *gorm.DB) *FaceNeighbours {
	return &FaceNeighbours{db: db}
}

// Supported reports whether the store can answer kNN queries: the dialect must
// be PostgreSQL and pgvector must be at least 0.7 (halfvec support). SQLite
// unit tests therefore take the in-memory clustering path.
func (f *FaceNeighbours) Supported(ctx context.Context) bool {
	if f.db.Dialector.Name() != "postgres" {
		return false
	}
	var supported bool
	if err := f.db.WithContext(ctx).Raw(pgvectorWithHalfvecQuery).Scan(&supported).Error; err != nil {
		return false
	}
	return supported
}

// Neighbours returns, per seed face, the faces within minSimilarity capped at
// limit, grouped by seed. Seeds without candidates are absent from the map.
func (f *FaceNeighbours) Neighbours(
	ctx context.Context, seeds []uint, minSimilarity float64, limit int,
) (map[uint][]faces.Neighbour, error) {
	grouped := make(map[uint][]faces.Neighbour, len(seeds))
	if len(seeds) == 0 {
		return grouped, nil
	}
	if limit <= 0 {
		return nil, fmt.Errorf("neighbour limit must be positive, got %d", limit)
	}

	seedIDs := make([]int64, len(seeds))
	for i, seed := range seeds {
		seedIDs[i] = int64(seed)
	}

	var rows []struct {
		FaceID      uint
		CandidateID uint
		Similarity  float64
	}
	if err := f.db.WithContext(ctx).Raw(neighbourQuery, seedIDs, minSimilarity, limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query face neighbours: %w", err)
	}
	for _, row := range rows {
		grouped[row.FaceID] = append(grouped[row.FaceID], faces.Neighbour{
			FaceID:     row.CandidateID,
			Similarity: row.Similarity,
		})
	}
	return grouped, nil
}

// EnsureNeighbourIndex creates the cosine HNSW index over face embeddings, once
// per analysis pass rather than on every insert.
func (f *FaceNeighbours) EnsureNeighbourIndex(ctx context.Context) (bool, error) {
	return EnsureFaceEmbeddingIndex(ctx, f.db)
}

// FaceNeighbours implements the application ports.
var (
	_ faces.NeighbourSource = (*FaceNeighbours)(nil)
	_ faces.IndexMaintainer = (*FaceNeighbours)(nil)
)
