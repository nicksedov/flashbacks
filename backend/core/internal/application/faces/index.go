package faces

import "context"

// IndexMaintainer keeps the vector index the neighbour queries rely on.
//
// The index is built once per analysis pass instead of on every face insert, so
// writes during a scan hit a heap-only table and stay fast.
type IndexMaintainer interface {
	// EnsureNeighbourIndex creates the cosine HNSW index over face embeddings if
	// it does not exist yet. It reports false when the store cannot maintain
	// such an index (SQLite unit tests), which matches
	// NeighbourSource.Supported reporting false.
	EnsureNeighbourIndex(ctx context.Context) (bool, error)
}
