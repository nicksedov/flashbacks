package database

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// createFaceEmbeddingIndexSQL mirrors the tag-embedding index: halfvec keeps the
// fp16 storage compact and cosine distance is the metric ArcFace embeddings are
// calibrated for. IF NOT EXISTS makes the build idempotent across passes.
const createFaceEmbeddingIndexSQL = `CREATE INDEX IF NOT EXISTS idx_face_embeddings_hnsw
	ON face_embeddings USING hnsw (embedding halfvec_cosine_ops)`

// EnsureFaceEmbeddingIndex creates the cosine HNSW index over face embeddings.
//
// It is called after an analysis pass rather than on every insert, so a scan
// writes to a heap-only table. The call is a no-op that reports false on a store
// that cannot maintain the index (SQLite unit tests).
func EnsureFaceEmbeddingIndex(ctx context.Context, db *gorm.DB) (bool, error) {
	if db.Dialector.Name() != "postgres" {
		return false, nil
	}
	if err := db.WithContext(ctx).Exec(createFaceEmbeddingIndexSQL).Error; err != nil {
		return false, fmt.Errorf("create face embedding index: %w", err)
	}
	return true, nil
}
