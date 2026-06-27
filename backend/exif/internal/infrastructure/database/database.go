package database

import (
	"fmt"

	"exif/internal/domain"
	"exif/internal/infrastructure/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Init connects to PostgreSQL, runs AutoMigrate for owned tables, and opens the shared database.
// The EXIF service owns the schema for image_metadata and geolocation_caches tables.
func Init(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// PrepareStmt avoids the simple protocol path in the PostgreSQL migrator.
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run AutoMigrate for owned tables (image_metadata, geolocation_caches).
	// Schema ownership was transferred from api-service to exif service.
	if err := db.AutoMigrate(&domain.ImageMetadata{}, &domain.GeolocationCache{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database tables: %w", err)
	}

	return db, nil
}
