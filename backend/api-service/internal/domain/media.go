package domain

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	shareddomain "github.com/flashbacks/shared/domain"
)

// ImageFile represents an image file in the database
type ImageFile struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"uniqueIndex;not null" json:"path"`
	Size      int64     `gorm:"not null;index:idx_size_hash" json:"size"`
	Hash      string    `gorm:"not null;index:idx_size_hash" json:"hash"`
	ModTime   time.Time `gorm:"not null" json:"modTime"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ExifHealthStatus is re-exported from the shared domain package.
// The canonical definition lives in github.com/flashbacks/shared/domain.
type ExifHealthStatus = shareddomain.ExifHealthStatus

// DuplicateGroup represents a group of duplicate images
type DuplicateGroup struct {
	Hash  string
	Size  int64
	Files []ImageFile
}

// SupportedExtensions contains all supported image file extensions
var SupportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
	".webp": true,
}

// IsImageFile checks if a file is a supported image based on extension
func IsImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return SupportedExtensions[ext]
}

// ImageMetadata is re-exported from the shared domain package.
// The canonical definition lives in github.com/flashbacks/shared/domain.
type ImageMetadata = shareddomain.ImageMetadata

// GeolocationCache is re-exported from the shared domain package.
// The canonical definition lives in github.com/flashbacks/shared/domain.
type GeolocationCache = shareddomain.GeolocationCache

// --- Phase 3: API response types (used by ExifClient and mocks) ---

// CalendarParams holds parameters for the calendar gallery query.
type CalendarParams struct {
	StartDate *time.Time
	EndDate   *time.Time
	Cursor    *time.Time
	CursorID  *uint
	PageSize  int
}

// CalendarResult holds the calendar gallery response.
type CalendarResult struct {
	Items         []CalendarItem
	NextCursor    string
	MinDate       string
	MaxDate       string
	TotalWithDate int64
}

// CalendarItem is re-exported from the shared domain package.
// The canonical definition lives in github.com/flashbacks/shared/domain.
type CalendarItem = shareddomain.CalendarItem

// GeoBounds defines a geographic bounding box.
type GeoBounds struct {
	MinLat float64
	MaxLat float64
	MinLng float64
	MaxLng float64
}

// GeoPoint represents a GPS point for clustering.
type GeoPoint struct {
	ImageFileID  uint    `json:"imageFileId"`
	GPSLatitude  float64 `json:"gpsLatitude"`
	GPSLongitude float64 `json:"gpsLongitude"`
	NameLocal    string  `json:"nameLocal"`
	NameEng      string  `json:"nameEng"`
}

// MissingImagesResult holds paginated missing images response.
type MissingImagesResult struct {
	Images   []MissingImageItem
	Total    int64
	Page     int
	PageSize int
}

// MissingImageItem is re-exported from the shared domain package.
// The canonical definition lives in github.com/flashbacks/shared/domain.
type MissingImageItem = shareddomain.MissingImageItem

// LocationCandidate is re-exported from the shared domain package.
// The canonical definition lives in github.com/flashbacks/shared/domain.
type LocationCandidate = shareddomain.LocationCandidate

// GalleryFolder represents a configured gallery folder in the database
type GalleryFolder struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"uniqueIndex;not null" json:"path"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AppSettings stores global application settings (singleton, ID=1)
// Contains application-level settings: trash directory, EXIF backup directory, and thumbnail cache configuration
type AppSettings struct {
	ID                    uint   `gorm:"primaryKey" json:"id"`
	TrashDir              string `gorm:"default:''" json:"trashDir"`
	ExifBackupDir         string `gorm:"default:''" json:"exifBackupDir"`
	ThumbnailCachePath    string `gorm:"default:''" json:"thumbnailCachePath"`
	ThumbnailCacheSize    int    `gorm:"default:0" json:"thumbnailCacheSize"`
	OcrConcurrentRequests int    `gorm:"default:4" json:"ocrConcurrentRequests"`
	// SyncDays: comma-separated weekday numbers (time.Weekday: 0=Sunday,1=Monday,...,6=Saturday)
	// Empty string means sync is disabled for all days.
	SyncDays        string `gorm:"default:'1,2,3,4,5'" json:"syncDays"`
	DailySyncHour   int    `gorm:"default:3" json:"dailySyncHour"`
	DailySyncMinute int    `gorm:"default:30" json:"dailySyncMinute"`
	// SyncTimezoneOffset: user's timezone offset in minutes from UTC (same sign as JS getTimezoneOffset: UTC+3 = -180)
	SyncTimezoneOffset int       `gorm:"default:0" json:"syncTimezoneOffset"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// SyncHistory stores a record of a completed background sync operation.
type SyncHistory struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	CreatedAt           time.Time `gorm:"autoCreateTime" json:"createdAt"`
	NewFiles            int       `gorm:"default:0" json:"newFiles"`
	UpdatedFiles        int       `gorm:"default:0" json:"updatedFiles"`
	DeletedFiles        int       `gorm:"default:0" json:"deletedFiles"`
	ThumbnailsGenerated int       `gorm:"default:0" json:"thumbnailsGenerated"`
}

// OcrClassification stores OCR classification results for an image
type OcrClassification struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	ImageFileID        uint      `gorm:"uniqueIndex;not null" json:"imageFileId"`
	IsTextDocument     bool      `gorm:"not null;default:false;index:idx_is_text_doc" json:"isTextDocument"`
	MeanConfidence     float32   `json:"meanConfidence"`
	WeightedConfidence float32   `json:"weightedConfidence"`
	TokenCount         int       `json:"tokenCount"`
	Angle              int       `json:"angle"`
	ScaleFactor        float32   `json:"scaleFactor"`
	BoundingBoxWidth   int       `json:"boundingBoxWidth"`
	BoundingBoxHeight  int       `json:"boundingBoxHeight"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// OcrBoundingBox stores bounding box data for OCR text regions
type OcrBoundingBox struct {
	ID               uint    `gorm:"primaryKey" json:"id"`
	ClassificationID uint    `gorm:"index;not null" json:"classificationId"`
	X                int     `json:"x"`
	Y                int     `json:"y"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	Word             string  `json:"word"`
	Confidence       float32 `json:"confidence"`
}

// LlmProvider stores per-provider LLM connection settings
// Name is the provider type ("ollama", "ollama_cloud", "openai", "deepseek"), Alias is a unique user-defined identifier
// NOTE: Model is no longer stored on LlmProvider. Use LlmInstrumentSettings.Model instead.
type LlmProvider struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"index;not null" json:"name"` // "ollama", "ollama_cloud", "openai", "deepseek"
	// Alias uniqueness is managed by a manual CREATE UNIQUE INDEX in database.go
	// (not by GORM uniqueIndex) to avoid a naming mismatch between GORM v1.30.0's
	// NamingStrategy.UniqueName ("uni_llm_providers_alias") and the existing DB
	// constraint name ("idx_llm_providers_alias").
	Alias     string    `gorm:"not null" json:"alias"`
	ApiUrl    string    `gorm:"not null" json:"apiUrl"`
	ApiKey    string    `gorm:"default:''" json:"apiKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// LlmProviderModel stores a single model per provider row.
// Replaces the JSON-blob LlmProviderModelCache with normalized relational storage.
type LlmProviderModel struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	LlmProviderID uint      `gorm:"not null;index" json:"llmProviderId"`
	ModelID       string    `gorm:"not null" json:"modelId"`        // API model identifier (e.g. "deepseek-v4-flash")
	ModelName     string    `gorm:"not null" json:"modelName"`      // Display name
	Size          int64     `gorm:"default:0" json:"size"`          // Model file size in bytes (0 = unknown)
	ContextLength int       `gorm:"default:0" json:"contextLength"` // Context window in tokens (0 = unknown)
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`

	// GORM relations
	Provider     LlmProvider          `gorm:"foreignKey:LlmProviderID;constraint:OnDelete:CASCADE" json:"-"`
	Capabilities []LlmModelCapability `gorm:"foreignKey:ModelID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName overrides the default GORM table name.
func (LlmProviderModel) TableName() string {
	return "llm_provider_models"
}

// LlmModelCapability stores a capability for a model (e.g. "chat", "tool_calling", "vision", "embedding").
type LlmModelCapability struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ModelID    uint   `gorm:"not null;index" json:"modelId"`
	Capability string `gorm:"size:50;not null" json:"capability"`
}

// TableName overrides the default GORM table name.
func (LlmModelCapability) TableName() string {
	return "llm_model_capabilities"
}

// LlmProviderModelCache stores cached model lists per provider.
// Deprecated: Use LlmProviderModel and LlmModelCapability instead.
// Kept for migration compatibility; will be removed after migration.
type LlmProviderModelCache struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	ProviderAlias string    `gorm:"uniqueIndex;not null" json:"providerAlias"`
	ModelsJSON    string    `gorm:"type:text;not null" json:"modelsJson"` // JSON array of {id, name, size?}
	FetchedAt     time.Time `json:"fetchedAt"`
}

// InstrumentType defines the type of an LLM instrument.
type InstrumentType string

const (
	InstrumentChat      InstrumentType = "chat"
	InstrumentVL        InstrumentType = "vl"
	InstrumentEmbedding InstrumentType = "embedding"
	InstrumentImageEdit InstrumentType = "image_edit"
)

// LlmInstrumentSettings stores the provider + model assignment per instrument type.
// One row per type (chat, vl, embedding, image_edit).
type LlmInstrumentSettings struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	Type       InstrumentType `gorm:"uniqueIndex;not null;size:50" json:"type"`
	ProviderID uint           `gorm:"not null" json:"providerId"`
	Model      string         `gorm:"not null;default:''" json:"model"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`

	// GORM belongs-to relation
	Provider LlmProvider `gorm:"foreignKey:ProviderID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName overrides the default GORM table name for LlmInstrumentSettings.
func (LlmInstrumentSettings) TableName() string {
	return "llm_instrument_settings"
}

// TagScanSettings stores tag scan scheduling configuration (singleton, ID=1).
type TagScanSettings struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Enabled        bool      `gorm:"default:true" json:"enabled"`
	StartHour      int       `gorm:"default:22" json:"startHour"`
	StartMinute    int       `gorm:"default:0" json:"startMinute"`
	EndHour        int       `gorm:"default:7" json:"endHour"`
	EndMinute      int       `gorm:"default:0" json:"endMinute"`
	TimezoneOffset int       `gorm:"default:0" json:"timezoneOffset"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// TableName overrides the default GORM table name for TagScanSettings.
func (TagScanSettings) TableName() string {
	return "tag_scan_settings"
}

// EmbeddingSettings stores embedding engine parameters (singleton, ID=1).
type EmbeddingSettings struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Dimension int       `gorm:"default:1024" json:"dimension"`
	BatchSize int       `gorm:"default:50" json:"batchSize"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName overrides the default GORM table name for EmbeddingSettings.
func (EmbeddingSettings) TableName() string {
	return "embedding_settings"
}

// ImageTag stores AI-generated tags for an image
type ImageTag struct {
	ID          uint   `gorm:"primaryKey"`
	ImageFileID uint   `gorm:"index;not null"`
	Tag         string `gorm:"not null"`
}

// TagEmbedding is the parent table for per-image embedding metadata.
// Actual vector data is stored in per-model child tables tag_embeddings_<model_name>.
type TagEmbedding struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ImageFileID uint      `gorm:"index;not null" json:"imageFileId"`
	TagCount    int       `gorm:"not null" json:"tagCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TagEmbeddingModel represents a row in a per-model child table tag_embeddings_<model_name>.
// Not managed by GORM AutoMigrate; table lifecycle is handled via raw SQL in the database package.
type TagEmbeddingModel struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	TagEmbeddingsID uint   `gorm:"not null" json:"tagEmbeddingsId"` // FK to tag_embeddings.id
	Dimensity       int    `gorm:"not null" json:"dimensity"`
	Embedding       string `gorm:"type:halfvec;not null" json:"-"` // pgvector halfvec (fp16)
}

// nonAlphanumericUnderscore matches any character that is not a letter, digit, or underscore.
var nonAlphanumericUnderscore = regexp.MustCompile(`[^a-zA-Z0-9_]`)
var multipleUnderscores = regexp.MustCompile(`_+`)

// SanitizeModelName converts an embedding model name to a valid PostgreSQL table name suffix.
// Replaces ':', '/', '-', '.', and any other non-alphanumeric/underscore chars with '_'.
func SanitizeModelName(modelName string) string {
	sanitized := nonAlphanumericUnderscore.ReplaceAllString(modelName, "_")
	sanitized = multipleUnderscores.ReplaceAllString(sanitized, "_")
	return strings.Trim(sanitized, "_")
}

// EmbeddingTableName returns the per-model child table name for a given embedding model.
func EmbeddingTableName(modelName string) string {
	return "tag_embeddings_" + SanitizeModelName(modelName)
}

// ImageProcessingError tracks permanent processing errors for an image file
// (e.g. corrupt/truncated JPEGs that cannot be decoded). Images with an entry
// in this table are excluded from tag scan processing to prevent endless retries.
type ImageProcessingError struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ImageFileID uint      `gorm:"uniqueIndex;not null" json:"imageFileId"`
	Action      string    `gorm:"not null;default:'tags'" json:"action"`
	Error       string    `gorm:"type:text;not null" json:"error"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// OcrLlmRecognition stores VL LLM OCR recognition results
type OcrLlmRecognition struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	ImageFileID         uint      `gorm:"uniqueIndex;not null" json:"imageFileId"`
	OcrClassificationID uint      `gorm:"index" json:"ocrClassificationId"`
	Language            string    `gorm:"not null" json:"language"` // "en", "ru", etc.
	MarkdownContent     string    `gorm:"type:text;not null" json:"markdownContent"`
	Provider            string    `json:"provider"`         // Which provider was used
	Model               string    `json:"model"`            // Which model was used
	ProcessingTimeMs    int       `json:"processingTimeMs"` // Processing time in milliseconds
	Error               string    `json:"error"`            // Error message if failed
	Success             bool      `gorm:"default:false" json:"success"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}
