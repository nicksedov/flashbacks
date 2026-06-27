package database

import (
	"fmt"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDatabase initializes the database connection and runs migrations.
func InitDatabase(cfg *config.AppConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// PrepareStmt avoids the simple protocol path in the PostgreSQL migrator
		// (GetRows), which triggers a pgx sanitizer bug with QueryExecModeSimpleProtocol.
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Enable pgvector extension BEFORE AutoMigrate needs the vector type
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		return nil, fmt.Errorf("failed to enable pgvector extension: %w", err)
	}

	// Run AutoMigrate
	// Note: image_metadata and geolocation_caches are owned by the exif service.
	if err := db.AutoMigrate(
		&domain.ImageFile{},
		&domain.GalleryFolder{},
		&domain.AppSettings{},
		&domain.User{},
		&domain.UserSettings{},
		&domain.Session{},
		&domain.AuditLog{},
		&domain.OcrClassification{},
		&domain.OcrBoundingBox{},
		&domain.LlmProvider{},
		&domain.LlmSettings{},
		&domain.OcrLlmRecognition{},
		&domain.ImageTag{},
		&domain.LlmProviderModelCache{},
		&domain.Conversation{},
		&domain.ConversationMessage{},
		&domain.TagEmbedding{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_providers_alias ON llm_providers (alias)")

	// Create composite index for calendar pagination: covers ORDER BY date_taken, image_file_id
	db.Exec("CREATE INDEX IF NOT EXISTS idx_image_metadata_date_taken_file_id ON image_metadata (date_taken, image_file_id)")

	// Seed default settings row if not exists
	var count int64
	db.Model(&domain.AppSettings{}).Count(&count)
	if count == 0 {
		db.Create(&domain.AppSettings{ID: 1})
	}

	// Seed default LLM settings row if not exists
	var llmCount int64
	db.Model(&domain.LlmSettings{}).Count(&llmCount)
	if llmCount == 0 {
		db.Create(&domain.LlmSettings{
			ID:             1,
			ActiveProvider: "ollama_1",
		})
	}

	// Seed default LLM providers if not exist
	var providerCount int64
	db.Model(&domain.LlmProvider{}).Count(&providerCount)
	if providerCount == 0 {
		db.Create([]domain.LlmProvider{
			{Name: "ollama", Alias: "ollama_1", ApiUrl: "http://localhost:11434", Model: "minicpm-v"},
			{Name: "ollama_cloud", Alias: "ollama_cloud_1", ApiUrl: "https://ollama.com", Model: "minicpm-v"},
			{Name: "openai", Alias: "openai_1", ApiUrl: "https://api.openai.com", Model: "gpt-4-vision-preview"},
		})
	}

	return db, nil
}
