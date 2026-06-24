package testutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestDB creates a new SQLite in-memory database with all models migrated.
// Returns the database connection and a cleanup function.
func NewTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Run auto-migration for all domain models
	if err := db.AutoMigrate(
		&domain.ImageFile{},
		&domain.GalleryFolder{},
		&domain.AppSettings{},
		&domain.ImageMetadata{},
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
		&domain.Conversation{},
		&domain.ConversationMessage{},
		&domain.LlmProviderModelCache{},
		&domain.GeolocationCache{},
		&domain.TagEmbedding{},
	); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	// Seed default settings
	db.Create(&domain.AppSettings{ID: 1})
	db.Create(&domain.LlmSettings{ID: 1, ActiveProvider: "ollama_test"})
	db.Create([]domain.LlmProvider{
		{Name: "ollama", Alias: "ollama_test", ApiUrl: "http://localhost:11434", Model: "minicpm-v"},
		{Name: "openai", Alias: "openai_test", ApiUrl: "https://api.openai.com", Model: "gpt-4-vision"},
	})

	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}

	return db, cleanup
}

// SeedUserWithHash creates a user in the test database with a pre-hashed password.
// Use auth.HashPassword from the auth package to generate the hash before calling this.
func SeedUserWithHash(t *testing.T, db *gorm.DB, login, displayName string, role domain.UserRole, isActive bool, passwordHash string) *domain.User {
	t.Helper()

	user := domain.User{
		Login:              login,
		DisplayName:        displayName,
		Role:               role,
		PasswordHash:       passwordHash,
		IsActive:           isActive,
		MustChangePassword: false,
	}

	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	return &user
}

// SeedImageFile creates an ImageFile record in the test database.
func SeedImageFile(t *testing.T, db *gorm.DB, path, hash string, size int64) *domain.ImageFile {
	t.Helper()

	imageFile := domain.ImageFile{
		Path: path,
		Hash: hash,
		Size: size,
	}

	if err := db.Create(&imageFile).Error; err != nil {
		t.Fatalf("failed to seed image file: %v", err)
	}

	return &imageFile
}

// SeedGalleryFolder creates a GalleryFolder record in the test database.
func SeedGalleryFolder(t *testing.T, db *gorm.DB, path string) *domain.GalleryFolder {
	t.Helper()

	folder := domain.GalleryFolder{
		Path: path,
	}

	if err := db.Create(&folder).Error; err != nil {
		t.Fatalf("failed to seed gallery folder: %v", err)
	}

	return &folder
}

// SeedOcrClassification creates an OcrClassification record in the test database.
func SeedOcrClassification(t *testing.T, db *gorm.DB, imageFileID uint, isTextDocument bool) *domain.OcrClassification {
	t.Helper()

	classification := domain.OcrClassification{
		ImageFileID:    imageFileID,
		IsTextDocument: isTextDocument,
	}

	if err := db.Create(&classification).Error; err != nil {
		t.Fatalf("failed to seed OCR classification: %v", err)
	}

	return &classification
}

// SeedAuditLog creates an AuditLog record in the test database.
func SeedAuditLog(t *testing.T, db *gorm.DB, actorUserID *uint, action domain.AuditAction, targetType string, targetID *uint) *domain.AuditLog {
	t.Helper()

	log := domain.AuditLog{
		ActorUserID: actorUserID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
	}

	if err := db.Create(&log).Error; err != nil {
		t.Fatalf("failed to seed audit log: %v", err)
	}

	return &log
}

// SeedUserNoT creates a user in the test database without requiring testing.T.
func SeedUserNoT(db *gorm.DB, login, role string, isActive bool, passwordHash string) *domain.User {
	user := domain.User{
		Login:              login,
		DisplayName:        login,
		Role:               domain.UserRole(role),
		PasswordHash:       passwordHash,
		IsActive:           isActive,
		MustChangePassword: false,
	}

	db.Create(&user)
	return &user
}

// SeedImageFileNoT creates an ImageFile record without requiring testing.T.
func SeedImageFileNoT(db *gorm.DB, path, hash string, size int64) *domain.ImageFile {
	imageFile := domain.ImageFile{
		Path: path,
		Hash: hash,
		Size: size,
	}

	db.Create(&imageFile)
	return &imageFile
}

// SeedGalleryFolderNoT creates a GalleryFolder record without requiring testing.T.
func SeedGalleryFolderNoT(db *gorm.DB, path string) *domain.GalleryFolder {
	folder := domain.GalleryFolder{
		Path: path,
	}

	db.Create(&folder)
	return &folder
}

// SeedOcrClassificationNoT creates an OcrClassification record without requiring testing.T.
func SeedOcrClassificationNoT(db *gorm.DB, imageFileID uint, isTextDocument bool) *domain.OcrClassification {
	classification := domain.OcrClassification{
		ImageFileID:    imageFileID,
		IsTextDocument: isTextDocument,
	}

	db.Create(&classification)
	return &classification
}

// SeedSession creates a session in the test database and returns the session and token hash.
func SeedSession(t *testing.T, db *gorm.DB, userID uint, expired, revoked bool) (*domain.Session, string) {
	t.Helper()

	session := domain.Session{
		UserID:       userID,
		SessionToken: "test-token-hash-" + fmt.Sprintf("%d", userID),
		IPAddress:    "127.0.0.1",
	}

	if expired {
		session.ExpiresAt = time.Now().Add(-1 * time.Hour)
	} else {
		session.ExpiresAt = time.Now().Add(24 * time.Hour)
	}

	if revoked {
		now := time.Now()
		session.RevokedAt = &now
	}

	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("failed to seed session: %v", err)
	}

	return &session, session.SessionToken
}

// SeedAuditLogNoT creates an AuditLog record without requiring testing.T.
func SeedAuditLogNoT(db *gorm.DB, actorUserID uint, action domain.AuditAction, targetType string, targetID *uint, meta string) *domain.AuditLog {
	log := domain.AuditLog{
		ActorUserID: &actorUserID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Meta:        meta,
	}

	db.Create(&log)
	return &log
}
