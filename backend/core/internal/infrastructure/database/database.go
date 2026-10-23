package database

import (
	"fmt"
	"log"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/infrastructure/config"

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
		&domain.LlmInstrumentSettings{},
		&domain.Schedule{},
		&domain.EmbeddingSettings{},
		&domain.OcrLlmRecognition{},
		&domain.ImageTag{},
		&domain.LlmProviderModel{},
		&domain.LlmModelCapability{},
		&domain.LlmModelModality{},
		&domain.Conversation{},
		&domain.ConversationMessage{},
		&domain.TagEmbedding{},
		&domain.ImageProcessingError{},
		&domain.SyncHistory{},
		&domain.TrashItem{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_providers_alias ON llm_providers (alias)")

	// Create composite index for calendar pagination: covers ORDER BY date_taken, image_file_id
	db.Exec("CREATE INDEX IF NOT EXISTS idx_image_metadata_date_taken_file_id ON image_metadata (date_taken, image_file_id)")

	// Create composite index for trash list pagination: covers ORDER BY deleted_at, id
	db.Exec("CREATE INDEX IF NOT EXISTS idx_trash_items_deleted_at_id ON trash_items (deleted_at, id)")

	// Drop the legacy llm_provider_model_caches table. Its data was migrated to
	// the normalized llm_provider_models table in a prior release; the JSON-blob
	// model cache is no longer read or written.
	db.Exec("DROP TABLE IF EXISTS llm_provider_model_caches")

	// Case-insensitive unique login index for self-service registration.
	// Existing case-conflicting logins (e.g. "Admin" vs "admin") must be resolved
	// manually before this index can be created; a warning is logged instead of
	// failing startup so the service can still boot.
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_login_lower ON users (lower(login))").Error; err != nil {
		log.Printf("warning: failed to create unique index idx_users_login_lower (resolve case-conflicting logins first): %v", err)
	}

	// Seed default settings row if not exists
	var count int64
	db.Model(&domain.AppSettings{}).Count(&count)
	if count == 0 {
		db.Create(&domain.AppSettings{ID: 1})
	}

	// Seed default LLM providers if not exist
	var providerCount int64
	db.Model(&domain.LlmProvider{}).Count(&providerCount)
	if providerCount == 0 {
		db.Create([]domain.LlmProvider{
			{Name: "ollama", Alias: "ollama_1", ApiUrl: "http://localhost:11434"},
			{Name: "ollama_cloud", Alias: "ollama_cloud_1", ApiUrl: "https://ollama.com"},
			{Name: "openai", Alias: "openai_1", ApiUrl: "https://api.openai.com"},
		})

		// Seed default instrument settings for the default providers
		var chatProvider, vlProvider domain.LlmProvider
		db.Where("alias = ?", "ollama_1").First(&chatProvider)
		db.Where("alias = ?", "ollama_1").First(&vlProvider)

		db.Create([]domain.LlmInstrumentSettings{
			{Type: domain.InstrumentChat, ProviderID: chatProvider.ID, Model: "minicpm-v"},
			{Type: domain.InstrumentVL, ProviderID: vlProvider.ID, Model: "minicpm-v"},
			{Type: domain.InstrumentEmbedding, ProviderID: chatProvider.ID, Model: "qwen3-embedding:4b"},
			{Type: domain.InstrumentImageEdit, ProviderID: chatProvider.ID, Model: "minicpm-v"},
		})

		// Seed default embedding settings
		var embCount int64
		db.Model(&domain.EmbeddingSettings{}).Count(&embCount)
		if embCount == 0 {
			db.Create(&domain.EmbeddingSettings{ID: 1, Dimension: 1024, BatchSize: 50})
		}
	}

	// Ensure an OCR instrument exists for installs that predate the separate
	// OCR instrument. This runs regardless of provider count so existing
	// deployments get an ocr row after upgrade, copying the provider + model
	// from the current VL instrument. This preserves the pre-upgrade behavior
	// (OCR used the same model as VL) while remaining independently editable.
	var ocrCount int64
	db.Model(&domain.LlmInstrumentSettings{}).Where("type = ?", domain.InstrumentOCR).Count(&ocrCount)
	if ocrCount == 0 {
		var vlInstrument domain.LlmInstrumentSettings
		if err := db.Where("type = ?", domain.InstrumentVL).First(&vlInstrument).Error; err == nil {
			db.Create(&domain.LlmInstrumentSettings{
				Type:       domain.InstrumentOCR,
				ProviderID: vlInstrument.ProviderID,
				Model:      vlInstrument.Model,
			})
		}
	}

	// Seed the unified schedules table: create the row of every scheduled job
	// kind from the legacy schedule columns when it is missing. Idempotent, so a
	// restart never overwrites an edited schedule.
	if err := seedSchedules(db); err != nil {
		log.Printf("warning: failed to seed job schedules: %v", err)
	}

	return db, nil
}

// legacyGallerySyncSchedule mirrors the schedule columns the gallery sync used to
// keep on app_settings. It exists only to backfill the schedules table.
type legacyGallerySyncSchedule struct {
	SyncDays           string `gorm:"column:sync_days"`
	DailySyncHour      int    `gorm:"column:daily_sync_hour"`
	DailySyncMinute    int    `gorm:"column:daily_sync_minute"`
	SyncTimezoneOffset int    `gorm:"column:sync_timezone_offset"`
}

func (legacyGallerySyncSchedule) TableName() string { return "app_settings" }

// legacyTagScanSchedule mirrors the schedule columns the tag scan used to keep on
// tag_scan_settings. It exists only to backfill the schedules table.
type legacyTagScanSchedule struct {
	Enabled        bool   `gorm:"column:enabled"`
	Days           string `gorm:"column:days"`
	StartHour      int    `gorm:"column:start_hour"`
	StartMinute    int    `gorm:"column:start_minute"`
	EndHour        int    `gorm:"column:end_hour"`
	EndMinute      int    `gorm:"column:end_minute"`
	TimezoneOffset int    `gorm:"column:timezone_offset"`
}

func (legacyTagScanSchedule) TableName() string { return "tag_scan_settings" }

// seedSchedules creates the missing schedule row of every job kind, backfilling
// from the legacy columns when they are still present.
func seedSchedules(db *gorm.DB) error {
	kinds := []domain.ScheduleKind{domain.ScheduleGallerySync, domain.ScheduleTagScan}

	for _, kind := range kinds {
		var count int64
		if err := db.Model(&domain.Schedule{}).Where("kind = ?", string(kind)).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		schedule := domain.DefaultSchedule(kind)
		switch kind {
		case domain.ScheduleGallerySync:
			backfillGallerySyncSchedule(db, &schedule)
		case domain.ScheduleTagScan:
			backfillTagScanSchedule(db, &schedule)
		}

		if err := db.Create(&schedule).Error; err != nil {
			return err
		}
	}
	return nil
}

// backfillGallerySyncSchedule copies the legacy app_settings schedule columns
// into schedule when they still exist.
func backfillGallerySyncSchedule(db *gorm.DB, schedule *domain.Schedule) {
	if !db.Migrator().HasTable("app_settings") || !db.Migrator().HasColumn(&legacyGallerySyncSchedule{}, "sync_days") {
		return
	}
	var legacy legacyGallerySyncSchedule
	if err := db.First(&legacy).Error; err != nil {
		return
	}
	schedule.Days = legacy.SyncDays
	if legacy.DailySyncHour > 0 || legacy.DailySyncMinute > 0 {
		schedule.StartHour = legacy.DailySyncHour
		schedule.StartMinute = legacy.DailySyncMinute
	}
	schedule.TimezoneOffset = legacy.SyncTimezoneOffset
}

// backfillTagScanSchedule copies the legacy tag_scan_settings schedule columns
// into schedule when they still exist.
func backfillTagScanSchedule(db *gorm.DB, schedule *domain.Schedule) {
	if !db.Migrator().HasTable("tag_scan_settings") || !db.Migrator().HasColumn(&legacyTagScanSchedule{}, "start_hour") {
		return
	}
	var legacy legacyTagScanSchedule
	if err := db.First(&legacy).Error; err != nil {
		return
	}
	schedule.Enabled = legacy.Enabled
	if legacy.Days != "" {
		schedule.Days = legacy.Days
	}
	schedule.StartHour = legacy.StartHour
	schedule.StartMinute = legacy.StartMinute
	schedule.EndHour = legacy.EndHour
	schedule.EndMinute = legacy.EndMinute
	schedule.TimezoneOffset = legacy.TimezoneOffset
}
