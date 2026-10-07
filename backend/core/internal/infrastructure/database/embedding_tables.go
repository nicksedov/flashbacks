package database

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/flashbacks/api-service/internal/domain"

	"gorm.io/gorm"
)

// validTableName matches a valid PostgreSQL table name for embedding child tables.
var validTableName = regexp.MustCompile(`^tag_embeddings_[a-zA-Z0-9_]+$`)

// maxTableNameLen is the maximum allowed length for a PostgreSQL identifier.
const maxTableNameLen = 63

// quoteIdentifier safely quotes a PostgreSQL identifier (table/column name).
// It wraps the name in double quotes and escapes any embedded double quotes.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// validateEmbeddingTableName validates the table name and returns it quoted.
// Returns an error if the name is invalid.
func validateEmbeddingTableName(modelName string) (string, error) {
	tableName := domain.EmbeddingTableName(modelName)

	// Validate the generated table name to prevent SQL injection
	if !validTableName.MatchString(tableName) {
		return "", fmt.Errorf("invalid embedding table name: %s (from model: %s)", tableName, modelName)
	}
	if len(tableName) > maxTableNameLen {
		return "", fmt.Errorf("embedding table name too long (%d chars, max %d): %s", len(tableName), maxTableNameLen, tableName)
	}
	return quoteIdentifier(tableName), nil
}

// EnsureEmbeddingTable creates the per-model child table and indexes if they don't exist.
// The child table stores vector embeddings for a specific model and references tag_embeddings.
// Uses pgvector's halfvec type (fp16) which supports HNSW indexing up to 4000 dimensions.
func EnsureEmbeddingTable(db *gorm.DB, modelName string, dimension int) error {
	quotedName, err := validateEmbeddingTableName(modelName)
	if err != nil {
		return err
	}

	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			tag_embeddings_id BIGINT NOT NULL REFERENCES tag_embeddings(id) ON DELETE CASCADE,
			dimensity INT NOT NULL DEFAULT %d,
			embedding halfvec(%d) NOT NULL
		)`, quotedName, dimension, dimension)

	if err := db.Exec(createSQL).Error; err != nil {
		return fmt.Errorf("failed to create embedding table %s: %w", quotedName, err)
	}

	// Create HNSW index for fast vector similarity search (halfvec_cosine_ops for fp16)
	indexSQL := fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS idx_%s_vector ON %s USING hnsw (embedding halfvec_cosine_ops)",
		domain.SanitizeModelName(modelName), quotedName)
	if err := db.Exec(indexSQL).Error; err != nil {
		return fmt.Errorf("failed to create HNSW index on %s: %w", quotedName, err)
	}

	// Unique constraint: one embedding per image per model
	constraintSQL := fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_tag_embeddings_id ON %s (tag_embeddings_id)",
		domain.SanitizeModelName(modelName), quotedName)
	if err := db.Exec(constraintSQL).Error; err != nil {
		return fmt.Errorf("failed to create unique index on %s: %w", quotedName, err)
	}

	return nil
}

// EmbeddingTableExists checks if a per-model embedding child table exists in the database.
func EmbeddingTableExists(db *gorm.DB, modelName string) bool {
	tableName := domain.EmbeddingTableName(modelName)
	var exists bool
	db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = ?)", tableName).Scan(&exists)
	return exists
}

// QuotedEmbeddingTableName returns the validated and safely quoted table name
// for use in raw SQL queries. Returns an error if the model name produces an invalid table name.
func QuotedEmbeddingTableName(modelName string) (string, error) {
	return validateEmbeddingTableName(modelName)
}
