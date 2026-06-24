package main

import "time"

// ImageTag represents a single AI-generated tag for an image.
// Source: api-service/internal/domain/media.go — ImageTag struct.
type ImageTag struct {
	ID          uint   `gorm:"primaryKey"`
	ImageFileID uint   `gorm:"index;not null"`
	Tag         string `gorm:"not null"`
}

// TagEmbedding is the parent table for per-image embedding metadata.
// Actual vector data is stored in per-model child tables tag_embeddings_<model_name>.
// Source: api-service/internal/domain/media.go — TagEmbedding struct.
type TagEmbedding struct {
	ID          uint `gorm:"primaryKey"`
	ImageFileID uint `gorm:"index;not null"`
	TagCount    int  `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TagEmbeddingModel represents a row in a per-model child table tag_embeddings_<model_name>.
// Source: api-service/internal/domain/media.go — TagEmbeddingModel struct.
type TagEmbeddingModel struct {
	ID              uint   `gorm:"primaryKey"`
	TagEmbeddingsID uint   `gorm:"not null"`
	Dimensity       int    `gorm:"not null"`
	Embedding       string `gorm:"type:halfvec;not null"`
}

// EmbeddingSetupHash stores MD5 hashes of tag content for idempotency tracking
// during the embeddings-builder migration utility. Lives in a separate table
// so it does not pollute the production tag_embeddings schema.
// Source: extracted from tag_embeddings.tag_hash.
type EmbeddingSetupHash struct {
	ID          uint   `gorm:"primaryKey"`
	ImageFileID uint   `gorm:"uniqueIndex;not null"`
	TagHash     string `gorm:"column:tag_hash;default:''"` // MD5 of sorted tag text
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// LlmSettings holds LLM provider and embedding configuration.
// Source: api-service/internal/domain/media.go — LlmSettings struct.
type LlmSettings struct {
	ID                     uint   `gorm:"primaryKey"`
	ActiveProvider         string `gorm:"default:ollama_1;not null"`
	EmbeddingProviderAlias string `gorm:"default:''"`
	EmbeddingModel         string `gorm:"default:'qwen3-embedding:4b'"`
	EmbeddingDimension     int    `gorm:"default:1024"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// LlmProvider represents a configured LLM provider instance.
// Source: api-service/internal/domain/media.go — LlmProvider struct.
type LlmProvider struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"index;not null"`
	Alias     string `gorm:"not null"`
	ApiUrl    string `gorm:"not null"`
	ApiKey    string `gorm:"default:''"`
	Model     string `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
