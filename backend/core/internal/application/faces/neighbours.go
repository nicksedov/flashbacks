package faces

import "context"

// Neighbour is one stored face close enough to a seed face to be a clustering
// candidate.
type Neighbour struct {
	// FaceID is the candidate face.
	FaceID uint
	// Similarity is the cosine similarity to the seed face.
	Similarity float64
}

// NeighbourSource answers nearest-neighbour queries over the stored face
// embeddings.
//
// Core asks for each unassigned face's neighbourhood inside the clustering
// radius instead of reading every embedding, so the clustering cost stops
// growing quadratically with the gallery and the whole vector set never has to
// travel in one request.
type NeighbourSource interface {
	// Supported reports whether the store can answer kNN queries at all: the
	// dialect is PostgreSQL and pgvector is new enough for halfvec HNSW.
	Supported(ctx context.Context) bool
	// Neighbours returns, for each seed face, its nearest faces at or above
	// minSimilarity, capped at limit per seed. Seeds without any candidate are
	// absent from the map.
	Neighbours(ctx context.Context, seeds []uint, minSimilarity float64, limit int) (map[uint][]Neighbour, error)
}
