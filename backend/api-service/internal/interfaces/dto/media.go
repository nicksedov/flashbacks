package dto

import "time"

// --- Duplicates API ---

// DuplicatesResponse is the JSON response for GET /api/duplicates
type DuplicatesResponse struct {
	Groups      []DuplicateGroupDTO `json:"groups"`
	TotalFiles  int                 `json:"totalFiles"`
	PageFiles   int                 `json:"pageFiles"`
	TotalGroups int                 `json:"totalGroups"`
	ScannedDirs []string            `json:"scannedDirs"`
	CurrentPage int                 `json:"currentPage"`
	PageSize    int                 `json:"pageSize"`
	TotalPages  int                 `json:"totalPages"`
	HasPrevPage bool                `json:"hasPrevPage"`
	HasNextPage bool                `json:"hasNextPage"`
	PageSizes   []int               `json:"pageSizes"`
}

// DuplicateGroupDTO represents a duplicate group in JSON responses
type DuplicateGroupDTO struct {
	Index     int       `json:"index"`
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
	SizeHuman string    `json:"sizeHuman"`
	Files     []FileDTO `json:"files"`
	Thumbnail string    `json:"thumbnail"`
}

// FileDTO represents a file in JSON responses
type FileDTO struct {
	ID       uint   `json:"id"`
	Path     string `json:"path"`
	FileName string `json:"fileName"`
	DirPath  string `json:"dirPath"`
	ModTime  string `json:"modTime"`
}

// --- Scan API ---

// ScanResponse is the JSON response for POST /api/scan
// Message is a i18n key string (e.g., "scan.started")
type ScanResponse struct {
	Message string `json:"message"`
}

// FastScanResponse is the JSON response for POST /api/fast-scan
type FastScanResponse struct {
	Message   string `json:"message"`
	Unchanged int    `json:"unchanged"` // Files that exist and haven't changed
	Modified  int    `json:"modified"`  // Files that were modified (size changed)
	Created   int    `json:"created"`   // New files added
	Deleted   int    `json:"deleted"`   // Records removed from DB (files no longer exist)
	Total     int    `json:"total"`     // Total checked (modified + created)
}

// --- Delete Files API ---

// DeleteFilesRequest represents the request for direct file deletion
type DeleteFilesRequest struct {
	FilePaths []string `json:"filePaths"`
	TrashDir  string   `json:"trashDir"`
}

// DeleteFilesResponse represents the response from file deletion
type DeleteFilesResponse struct {
	Success     int      `json:"success"`
	Failed      int      `json:"failed"`
	FailedFiles []string `json:"failedFiles,omitempty"`
}

// --- Folder Patterns API ---

// FolderPattern represents a unique combination of folders containing duplicates
type FolderPattern struct {
	ID             string   `json:"id"`
	Folders        []string `json:"folders"`
	DuplicateCount int      `json:"duplicateCount"`
	TotalFiles     int      `json:"totalFiles"`
}

// FolderPatternsResponse represents the response for folder patterns
type FolderPatternsResponse struct {
	Patterns                   []FolderPattern `json:"patterns"`
	SingleFolderDuplicateCount int             `json:"singleFolderDuplicateCount"` // Duplicates all in one folder (not suitable for batch dedup)
}

// --- Batch Delete API ---

// BatchDeleteRequest represents a request for batch deletion
type BatchDeleteRequest struct {
	Rules    []BatchDeleteRule `json:"rules"`
	TrashDir string            `json:"trashDir"`
}

// BatchDeleteRule specifies which folder to keep for a pattern
type BatchDeleteRule struct {
	PatternID  string `json:"patternId"`
	KeepFolder string `json:"keepFolder"`
}

// BatchDeleteResponse represents the response from batch deletion
type BatchDeleteResponse struct {
	RulesApplied int      `json:"rulesApplied"`
	FilesDeleted int      `json:"filesDeleted"`
	Failed       int      `json:"failed"`
	FailedFiles  []string `json:"failedFiles,omitempty"`
}

// --- Thumbnail API ---

// ThumbnailResponse is the JSON response for GET /api/thumbnail
type ThumbnailResponse struct {
	Thumbnail string `json:"thumbnail"`
}

// InvalidateThumbnailRequest запрос на удаление миниатюры
type InvalidateThumbnailRequest struct {
	FilePath string `json:"filePath" binding:"required"`
}

// WarmupThumbnailsRequest запрос на предварительную генерацию миниатюр
type WarmupThumbnailsRequest struct {
	FilePaths []string `json:"filePaths" binding:"required"`
}

// --- Gallery Folders API ---

// GalleryFolderDTO represents a gallery folder in JSON responses
type GalleryFolderDTO struct {
	ID        uint   `json:"id"`
	Path      string `json:"path"`
	FileCount int    `json:"fileCount"`
	CreatedAt string `json:"createdAt"`
}

// GalleryFoldersResponse is the JSON response for GET /api/folders
type GalleryFoldersResponse struct {
	Folders      []GalleryFolderDTO `json:"folders"`
	TotalFolders int                `json:"totalFolders"`
}

// AddFolderRequest represents the request for adding a gallery folder
type AddFolderRequest struct {
	Path string `json:"path" binding:"required"`
}

// AddFolderResponse is the JSON response for POST /api/folders
// Message is a i18n key string (e.g., "folder.added")
type AddFolderResponse struct {
	Message     string           `json:"message"`
	Folder      GalleryFolderDTO `json:"folder"`
	ScanStarted bool             `json:"scanStarted"`
}

// RemoveFolderResponse is the JSON response for DELETE /api/folders/:id
// Message is a i18n key string (e.g., "folder.removed")
type RemoveFolderResponse struct {
	Message      string `json:"message"`
	FilesRemoved int    `json:"filesRemoved"`
}

// --- Gallery Images API ---

// GalleryImageDTO represents an image in the gallery browser
type GalleryImageDTO struct {
	ID          uint   `json:"id"`
	Path        string `json:"path"`
	FileName    string `json:"fileName"`
	DirPath     string `json:"dirPath"`
	Size        int64  `json:"size"`
	SizeHuman   string `json:"sizeHuman"`
	ModTime     string `json:"modTime"`
	Thumbnail   string `json:"thumbnail,omitempty"`
	MissingDate bool   `json:"missingDate,omitempty"`
	MissingGps  bool   `json:"missingGps,omitempty"`
}

// GalleryImagesResponse is the JSON response for GET /api/gallery
type GalleryImagesResponse struct {
	Images      []GalleryImageDTO `json:"images"`
	TotalImages int               `json:"totalImages"`
	CurrentPage int               `json:"currentPage"`
	PageSize    int               `json:"pageSize"`
	TotalPages  int               `json:"totalPages"`
	HasNextPage bool              `json:"hasNextPage"`
}

// --- App Settings API ---

// AppSettingsDTO is the JSON response for GET /api/settings
type AppSettingsDTO struct {
	TrashDir              string     `json:"trashDir"`
	ExifBackupDir         string     `json:"exifBackupDir"`
	ThumbnailCachePath    string     `json:"thumbnailCachePath,omitempty"`
	ThumbnailCacheSize    int        `json:"thumbnailCacheSize,omitempty"`
	OcrConcurrentRequests int        `json:"ocrConcurrentRequests,omitempty"`
	SyncDays              string     `json:"syncDays"`
	DailySyncHour         int        `json:"dailySyncHour"`
	DailySyncMinute       int        `json:"dailySyncMinute"`
	SyncTimezoneOffset    int        `json:"syncTimezoneOffset"`
	LastSyncAt            *time.Time `json:"lastSyncAt,omitempty"`
	LastSyncNew           int        `json:"lastSyncNew"`
	LastSyncUpdated       int        `json:"lastSyncUpdated"`
	LastSyncDeleted       int        `json:"lastSyncDeleted"`
	LastSyncThumbnails    int        `json:"lastSyncThumbnails"`
}

// SyncStatusResponse is the JSON response for GET /api/sync-status.
// NextRunAt and LastSyncAt are formatted as ISO 8601 strings in the user's timezone
// to avoid browser timezone double-conversion.
type SyncStatusResponse struct {
	Running            bool   `json:"running"`
	SyncInProgress     bool   `json:"syncInProgress"`
	NextRunAt          string `json:"nextRunAt,omitempty"`
	LastSyncAt         string `json:"lastSyncAt,omitempty"`
	LastSyncNew        int    `json:"lastSyncNew"`
	LastSyncUpdated    int    `json:"lastSyncUpdated"`
	LastSyncDeleted    int    `json:"lastSyncDeleted"`
	LastSyncThumbnails int    `json:"lastSyncThumbnails"`
	ProcessedFiles     int    `json:"processedFiles"`
	TotalFiles         int    `json:"totalFiles"`
}

// UserSettingsDTO is the JSON response for user settings
type UserSettingsDTO struct {
	Theme    string `json:"theme"`
	Language string `json:"language"`
}

// UpdateSettingsRequest is the JSON request for PUT /api/settings
type UpdateSettingsRequest struct {
	TrashDir              *string `json:"trashDir"`
	ExifBackupDir         *string `json:"exifBackupDir"`
	ThumbnailCachePath    *string `json:"thumbnailCachePath,omitempty"`
	OcrConcurrentRequests *int    `json:"ocrConcurrentRequests,omitempty"`
	SyncDays              *string `json:"syncDays,omitempty"`
	DailySyncHour         *int    `json:"dailySyncHour,omitempty"`
	DailySyncMinute       *int    `json:"dailySyncMinute,omitempty"`
	SyncTimezoneOffset    *int    `json:"syncTimezoneOffset,omitempty"`
}

// UpdateUserSettingsRequest is the JSON request for PUT /api/user-settings
type UpdateUserSettingsRequest struct {
	Theme    string `json:"theme"`
	Language string `json:"language"`
}

// --- Trash API ---

// TrashInfoResponse is the JSON response for GET /api/trash-info
type TrashInfoResponse struct {
	FileCount      int    `json:"fileCount"`
	TotalSize      int64  `json:"totalSize"`
	TotalSizeHuman string `json:"totalSizeHuman"`
}

// CleanTrashResponse is the JSON response for POST /api/trash-clean
type CleanTrashResponse struct {
	Deleted int `json:"deleted"`
	Failed  int `json:"failed"`
}

// --- Image Metadata API ---

// ImageMetadataDTO represents image EXIF metadata and geolocation in JSON responses
type ImageMetadataDTO struct {
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	Dimensions   string   `json:"dimensions"`
	CameraModel  string   `json:"cameraModel"`
	LensModel    string   `json:"lensModel"`
	ISO          int      `json:"iso"`
	Aperture     string   `json:"aperture"`
	ShutterSpeed string   `json:"shutterSpeed"`
	FocalLength  string   `json:"focalLength"`
	DateTaken    string   `json:"dateTaken"`
	Orientation  int      `json:"orientation"`
	ColorSpace   string   `json:"colorSpace"`
	Software     string   `json:"software"`
	GPSLatitude  *float64 `json:"gpsLatitude"`
	GPSLongitude *float64 `json:"gpsLongitude"`
	NameLocal    string   `json:"nameLocal"`
	NameEng      string   `json:"nameEng"`
	HasGPS       bool     `json:"hasGps"`
	HasExif      bool     `json:"hasExif"`
}

// ImageMetadataResponse is the JSON response for GET /api/image-metadata
type ImageMetadataResponse struct {
	Found    bool              `json:"found"`
	Metadata *ImageMetadataDTO `json:"metadata,omitempty"`
}

// --- Gallery Calendar API ---

// CalendarDateGroup represents a group of images for a single date
type CalendarDateGroup struct {
	Date       string            `json:"date"`  // "YYYY-MM-DD"
	Label      string            `json:"label"` // Human-readable label
	ImageCount int               `json:"imageCount"`
	Images     []GalleryImageDTO `json:"images"`
}

// CalendarDateRange represents the min/max date range for all images with EXIF dates
type CalendarDateRange struct {
	MinDate       string `json:"minDate"` // "YYYY-MM-DD" or empty
	MaxDate       string `json:"maxDate"` // "YYYY-MM-DD" or empty
	TotalWithDate int    `json:"totalWithDate"`
}

// CalendarMonthInfo represents which days in a month have images
type CalendarMonthInfo struct {
	Year  int   `json:"year"`
	Month int   `json:"month"` // 1-12
	Days  []int `json:"days"`  // Days that have images (1-31)
}

// TimelineDateMarker represents a single date with image count for the timeline
type TimelineDateMarker struct {
	Date       string `json:"date"`       // "YYYY-MM-DD"
	ImageCount int    `json:"imageCount"` // Number of images on this date
	Page       int    `json:"page"`       // Page number (1-based) where this date first appears (deprecated)
	Cursor     string `json:"cursor"`     // Cursor pointing to the start of this date
}

// CalendarAllDatesResponse is the JSON response for GET /api/gallery/calendar/dates
type CalendarAllDatesResponse struct {
	MinDate string               `json:"minDate"` // "YYYY-MM-DD" or empty
	MaxDate string               `json:"maxDate"` // "YYYY-MM-DD" or empty
	Dates   []TimelineDateMarker `json:"dates"`   // All dates that have images
}

// GalleryCalendarResponse is the JSON response for GET /api/gallery/calendar
type GalleryCalendarResponse struct {
	Groups      []CalendarDateGroup `json:"groups"`
	TotalImages int                 `json:"totalImages"`
	TotalGroups int                 `json:"totalGroups"`
	HasMore     bool                `json:"hasMore"`
	DateRange   CalendarDateRange   `json:"dateRange"`
	// Month info for the calendar widget (current page's months)
	Months []CalendarMonthInfo `json:"months"`
	// Cursor-based pagination support
	NextCursor *string `json:"nextCursor,omitempty"`
}

// CalendarSeekRequest is the request for GET /api/gallery/calendar/seek
type CalendarSeekRequest struct {
	Date string `form:"date" binding:"required"` // "YYYY-MM-DD"
}

// CalendarSeekResponse is the response for GET /api/gallery/calendar/seek
type CalendarSeekResponse struct {
	Cursor     string `json:"cursor"`     // Cursor pointing to the requested date
	ActualDate string `json:"actualDate"` // The actual date found (may differ if requested date has no images)
	ImageCount int    `json:"imageCount"` // Number of images on this date
}

// --- Gallery Geolocation / Map Clustering API ---

// GeoCluster represents a single cluster in the map view response
type GeoCluster struct {
	ID         string   `json:"id"`
	Latitude   float64  `json:"latitude"`
	Longitude  float64  `json:"longitude"`
	Count      int      `json:"count"`
	ImagePaths []string `json:"imagePaths,omitempty"` // Only used internally, not sent to frontend
}

// GeoClustersResponse is the JSON response for GET /api/gallery/clusters
type GeoClustersResponse struct {
	Clusters    []GeoCluster `json:"clusters"`
	TotalImages int          `json:"totalImages"`
}

// GeoImagesResponse is the JSON response for GET /api/gallery/geo-images
type GeoImagesResponse struct {
	Images      []GalleryImageDTO `json:"images"`
	TotalImages int               `json:"totalImages"`
	CurrentPage int               `json:"currentPage"`
	PageSize    int               `json:"pageSize"`
	TotalPages  int               `json:"totalPages"`
	HasNextPage bool              `json:"hasNextPage"`
}

// --- OCR Status API ---

// OCRStatus represents the current status of OCR classifier service
type OCRStatus struct {
	Enabled    bool   `json:"enabled"`
	Health     string `json:"health"`
	LastCheck  string `json:"lastCheck,omitempty"`
	Error      string `json:"error,omitempty"`
	ServiceURL string `json:"serviceUrl,omitempty"`
}

// OCRStatusResponse is the JSON response for GET /api/ocr/status
type OCRStatusResponse struct {
	Status OCRStatus `json:"status"`
}

// --- OCR Classification API ---

// OcrClassificationStatusResponse for GET /api/ocr/classify-status
type OcrClassificationStatusResponse struct {
	Processing     bool   `json:"processing"`
	Progress       string `json:"progress"`
	FilesProcessed int    `json:"filesProcessed"`
	TotalFiles     int    `json:"totalFiles"`
}

// OcrDocumentDTO represents an image classified as a text document
type OcrDocumentDTO struct {
	ID                 uint    `json:"id"`
	ImageFileID        uint    `json:"imageFileId"`
	Path               string  `json:"path"`
	FileName           string  `json:"fileName"`
	DirPath            string  `json:"dirPath"`
	Size               int64   `json:"size"`
	SizeHuman          string  `json:"sizeHuman"`
	ModTime            string  `json:"modTime"`
	Thumbnail          string  `json:"thumbnail,omitempty"`
	ThumbnailCachePath string  `json:"thumbnailCachePath,omitempty"`
	MeanConfidence     float32 `json:"meanConfidence"`
	WeightedConfidence float32 `json:"weightedConfidence"`
	TokenCount         int     `json:"tokenCount"`
	Angle              int     `json:"angle"`
	ScaleFactor        float32 `json:"scaleFactor"`
}

// OcrDocumentsResponse for GET /api/ocr/documents
type OcrDocumentsResponse struct {
	Documents   []OcrDocumentDTO `json:"documents"`
	Total       int              `json:"total"`
	CurrentPage int              `json:"currentPage"`
	PageSize    int              `json:"pageSize"`
	TotalPages  int              `json:"totalPages"`
	HasNextPage bool             `json:"hasNextPage"`
}

// BoundingBoxDTO for OCR bounding box data
type BoundingBoxDTO struct {
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Word       string  `json:"word"`
	Confidence float32 `json:"confidence"`
}

// OcrDataResponse for GET /api/ocr/data
type OcrDataResponse struct {
	ImagePath         string           `json:"imagePath"`
	Angle             int              `json:"angle"`
	ScaleFactor       float32          `json:"scaleFactor"`
	IsTextDocument    bool             `json:"isTextDocument"`
	BoundingBoxWidth  int              `json:"boundingBoxWidth"`
	BoundingBoxHeight int              `json:"boundingBoxHeight"`
	Boxes             []BoundingBoxDTO `json:"boxes"`
}

// --- LLM Settings API ---

// LlmProviderDTO for per-provider LLM settings responses
type LlmProviderDTO struct {
	ID           uint          `json:"id"`
	Alias        string        `json:"alias"`
	Name         string        `json:"name"` // "ollama", "ollama_cloud", "openai"
	ApiUrl       string        `json:"apiUrl"`
	ApiKey       string        `json:"apiKey"` // Masked in responses
	Model        string        `json:"model"`
	CachedModels []LlmModelDTO `json:"cachedModels"`
}

// LlmSettingsResponse for GET /api/llm/settings
type LlmSettingsResponse struct {
	ID                     uint             `json:"id"`
	ActiveProvider         string           `json:"activeProvider"` // References LlmProvider.Alias
	TagScanEnabled         bool             `json:"tagScanEnabled"`
	TagScanStartHour       int              `json:"tagScanStartHour"`
	TagScanStartMinute     int              `json:"tagScanStartMinute"`
	TagScanEndHour         int              `json:"tagScanEndHour"`
	TagScanEndMinute       int              `json:"tagScanEndMinute"`
	TagScanTimezoneOffset  int              `json:"tagScanTimezoneOffset"`
	EmbeddingProviderAlias string           `json:"embeddingProviderAlias"`
	EmbeddingModel         string           `json:"embeddingModel"`
	EmbeddingDimension     int              `json:"embeddingDimension"`
	EmbeddingBatchSize     int              `json:"embeddingBatchSize"`
	Providers              []LlmProviderDTO `json:"providers"`
}

// UpdateLlmSettingsRequest for PUT /api/llm/settings (active provider + tag scan only)
type UpdateLlmSettingsRequest struct {
	ActiveProvider         *string `json:"activeProvider"` // References LlmProvider.Alias
	TagScanEnabled         *bool   `json:"tagScanEnabled,omitempty"`
	TagScanStartHour       *int    `json:"tagScanStartHour,omitempty"`
	TagScanStartMinute     *int    `json:"tagScanStartMinute,omitempty"`
	TagScanEndHour         *int    `json:"tagScanEndHour,omitempty"`
	TagScanEndMinute       *int    `json:"tagScanEndMinute,omitempty"`
	TagScanTimezoneOffset  *int    `json:"tagScanTimezoneOffset,omitempty"`
	EmbeddingProviderAlias *string `json:"embeddingProviderAlias,omitempty"`
	EmbeddingModel         *string `json:"embeddingModel,omitempty"`
	EmbeddingDimension     *int    `json:"embeddingDimension,omitempty"`
	EmbeddingBatchSize     *int    `json:"embeddingBatchSize,omitempty"`
}

// ProbeEmbeddingDimensionRequest for POST /api/llm/embedding/probe
type ProbeEmbeddingDimensionRequest struct {
	ProviderAlias string `json:"providerAlias" binding:"required"`
	Model         string `json:"model" binding:"required"`
}

// ProbeEmbeddingDimensionResponse for POST /api/llm/embedding/probe
type ProbeEmbeddingDimensionResponse struct {
	Dimension int `json:"dimension"`
}

// CreateLlmProviderRequest for POST /api/llm/providers
type CreateLlmProviderRequest struct {
	Alias  string `json:"alias" binding:"required"`
	Name   string `json:"name" binding:"required"` // "ollama", "ollama_cloud", "openai"
	ApiUrl string `json:"apiUrl"`
	ApiKey string `json:"apiKey"`
	Model  string `json:"model"`
}

// UpdateLlmProviderRequest for PUT /api/llm/providers/:alias
type UpdateLlmProviderRequest struct {
	ApiUrl *string `json:"apiUrl"`
	ApiKey *string `json:"apiKey"`
	Model  *string `json:"model"`
	Alias  *string `json:"alias"` // New alias value (rename)
}

// TagScanStatusResponse for GET /api/tag-scan/status
type TagScanStatusResponse struct {
	Running      bool   `json:"running"`
	Paused       bool   `json:"paused"`
	Enabled      bool   `json:"enabled"`
	Schedule     string `json:"schedule"`
	WindowOpen   bool   `json:"windowOpen"`
	Scanned      int    `json:"scanned"`
	Remaining    int    `json:"remaining"`
	Total        int    `json:"total"`
	CurrentImage string `json:"currentImage,omitempty"`
	LastError    string `json:"lastError,omitempty"`
}

// LlmModelDTO represents an available LLM model
type LlmModelDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Size          int64  `json:"size,omitempty"`
	ContextLength int    `json:"contextLength,omitempty"`
}

// LlmModelsResponse for GET /api/llm/models
type LlmModelsResponse struct {
	Success  bool          `json:"success"`
	Models   []LlmModelDTO `json:"models"`
	Error    string        `json:"error,omitempty"`
	Provider string        `json:"provider"`
}

// --- LLM OCR API ---

// LlmOcrRequest for POST /api/llm/recognize
type LlmOcrRequest struct {
	ImagePath string `json:"imagePath" binding:"required"`
	Force     bool   `json:"force"`
}

// LlmOcrResponse for POST /api/llm/recognize
type LlmOcrResponse struct {
	Success          bool   `json:"success"`
	MarkdownContent  string `json:"markdownContent,omitempty"`
	Language         string `json:"language"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	ProcessingTimeMs int    `json:"processingTimeMs"`
	Error            string `json:"error,omitempty"`
}

// LlmOcrDataResponse for GET /api/llm/recognition
type LlmOcrDataResponse struct {
	Found            bool   `json:"found"`
	MarkdownContent  string `json:"markdownContent,omitempty"`
	Language         string `json:"language,omitempty"`
	Provider         string `json:"provider,omitempty"`
	Model            string `json:"model,omitempty"`
	ProcessingTimeMs int    `json:"processingTimeMs,omitempty"`
	Success          bool   `json:"success,omitempty"`
	Error            string `json:"error,omitempty"`
	CreatedAt        string `json:"createdAt,omitempty"`
}

// LlmRecognizeStatusResponse for GET /api/llm/recognize-status
type LlmRecognizeStatusResponse struct {
	Status           string `json:"status"` // "processing", "completed", "failed", "not_found"
	MarkdownContent  string `json:"markdownContent,omitempty"`
	Language         string `json:"language,omitempty"`
	Provider         string `json:"provider,omitempty"`
	Model            string `json:"model,omitempty"`
	ProcessingTimeMs int    `json:"processingTimeMs,omitempty"`
	Error            string `json:"error,omitempty"`
}

// --- AI Assistant API ---

// AiActionType represents the type of AI action
type AiActionType string

const (
	AiActionDescribe      AiActionType = "describe"
	AiActionTags          AiActionType = "tags"
	AiActionRecognizeText AiActionType = "recognizeText"
	AiActionAskQuestion   AiActionType = "askQuestion"
)

// AiActionRequest for POST /api/ai/action
type AiActionRequest struct {
	ImagePath string       `json:"imagePath" binding:"required"`
	Action    AiActionType `json:"action" binding:"required"`
	Question  string       `json:"question,omitempty"` // Only for askQuestion
	Language  string       `json:"language,omitempty"` // UI language code (e.g. "en", "ru")
	Force     bool         `json:"force,omitempty"`    // Force regeneration, skip cached results; defaults to false
}

// AiActionStartResponse for POST /api/ai/action (async start)
type AiActionStartResponse struct {
	TaskID string       `json:"taskId"`
	Action AiActionType `json:"action"`
	Status string       `json:"status"` // "processing"
}

// AiActionStatusResponse for GET /api/ai/status/:taskId
type AiActionStatusResponse struct {
	TaskID           string       `json:"taskId"`
	Status           string       `json:"status"` // "processing", "completed", "failed"
	Action           AiActionType `json:"action"`
	Result           string       `json:"result,omitempty"`
	Tags             []string     `json:"tags,omitempty"`
	Error            string       `json:"error,omitempty"`
	Provider         string       `json:"provider,omitempty"`
	Model            string       `json:"model,omitempty"`
	ProcessingTimeMs int          `json:"processingTimeMs,omitempty"`
}

// AiActionResponse for POST /api/ai/action
type AiActionResponse struct {
	Success          bool         `json:"success"`
	Action           AiActionType `json:"action"`
	Result           string       `json:"result,omitempty"`
	Tags             []string     `json:"tags,omitempty"` // Only for tags action
	Error            string       `json:"error,omitempty"`
	Provider         string       `json:"provider,omitempty"`
	Model            string       `json:"model,omitempty"`
	ProcessingTimeMs int          `json:"processingTimeMs,omitempty"`
}

// ImageTagsResponse for GET /api/image-tags
type ImageTagsResponse struct {
	Tags []string `json:"tags"`
}

// --- Geocode / GPS API ---

// GeocodeSearchResult represents a single location from the Nominatim geocoding API.
type GeocodeSearchResult struct {
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	DisplayName string  `json:"displayName"`
	Type        string  `json:"type"`
}

// GeocodeSearchResponse is the JSON response for GET /api/geocode/search
type GeocodeSearchResponse struct {
	Results []GeocodeSearchResult `json:"results"`
}

// UpdateGpsRequest is the JSON request for PUT /api/image-metadata/gps
type UpdateGpsRequest struct {
	Path string  `json:"path" binding:"required"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
}

// UpdateGpsResponse is the JSON response for PUT /api/image-metadata/gps
type UpdateGpsResponse struct {
	Success   bool    `json:"success"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	NameLocal string  `json:"nameLocal"`
	NameEng   string  `json:"nameEng"`
}

// LocationCandidate represents a suggested location from same-day photos.
type LocationCandidate struct {
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	NameLocal  string  `json:"nameLocal"`
	NameEng    string  `json:"nameEng"`
	PhotoCount int     `json:"photoCount"`
	Thumbnail  string  `json:"thumbnail,omitempty"`
}

// LocationCandidatesResponse is the JSON response for GET /api/image-metadata/location-candidates
type LocationCandidatesResponse struct {
	Candidates []LocationCandidate `json:"candidates"`
}

// BatchUpdateGpsRequest is the JSON request for PUT /api/image-metadata/gps/batch
type BatchUpdateGpsRequest struct {
	Paths []string `json:"paths" binding:"required"`
	Lat   float64  `json:"lat"`
	Lng   float64  `json:"lng"`
}

// BatchUpdateGpsResponse is the JSON response for PUT /api/image-metadata/gps/batch
type BatchUpdateGpsResponse struct {
	Success     int      `json:"success"`
	Failed      int      `json:"failed"`
	Skipped     int      `json:"skipped"`
	FailedFiles []string `json:"failedFiles,omitempty"`
	NameLocal   string   `json:"nameLocal"`
	NameEng     string   `json:"nameEng"`
	Lat         float64  `json:"lat"`
	Lng         float64  `json:"lng"`
}
