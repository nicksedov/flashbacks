package dto

// HealthResponse is the JSON response for GET /exif/health
type HealthResponse struct {
	Status            string `json:"status"`
	Version           string `json:"version"`
	ExiftoolAvailable bool   `json:"exiftoolAvailable"`
	DatabaseConnected bool   `json:"databaseConnected"`
	Uptime            string `json:"uptime"`
}

// MetadataResponse is the JSON response for GET /exif/metadata
type MetadataResponse struct {
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	CameraModel  string   `json:"cameraModel"`
	LensModel    string   `json:"lensModel"`
	ISO          int      `json:"iso"`
	Aperture     string   `json:"aperture"`
	ShutterSpeed string   `json:"shutterSpeed"`
	FocalLength  string   `json:"focalLength"`
	DateTaken    string   `json:"dateTaken,omitempty"`
	Orientation  int      `json:"orientation"`
	ColorSpace   string   `json:"colorSpace"`
	Software     string   `json:"software"`
	GPSLatitude  *float64 `json:"gpsLatitude,omitempty"`
	GPSLongitude *float64 `json:"gpsLongitude,omitempty"`
}

// GPSRequest is the JSON request for PUT /exif/gps
type GPSRequest struct {
	Path      string  `json:"path" binding:"required"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	BackupDir string  `json:"backupDir" binding:"required"`
}

// GPSBatchRequest is the JSON request for PUT /exif/gps/batch
type GPSBatchRequest struct {
	Items     []GPSBatchItem `json:"items" binding:"required"`
	BackupDir string         `json:"backupDir" binding:"required"`
}

// GPSBatchItem represents a single item in a batch GPS request
type GPSBatchItem struct {
	Path      string  `json:"path" binding:"required"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// GPSResponse is the JSON response for PUT /exif/gps
type GPSResponse struct {
	Success   bool    `json:"success"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// GPSBatchResponse is the JSON response for PUT /exif/gps/batch
type GPSBatchResponse struct {
	Success     int      `json:"success"`
	Failed      int      `json:"failed"`
	FailedFiles []string `json:"failedFiles,omitempty"`
}

// LocationCandidate represents a suggested location
type LocationCandidate struct {
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	NameLocal  string  `json:"nameLocal"`
	NameEng    string  `json:"nameEng"`
	PhotoCount int     `json:"photoCount"`
}

// LocationCandidatesResponse is the JSON response for GET /exif/location-candidates
type LocationCandidatesResponse struct {
	Candidates []LocationCandidate `json:"candidates"`
}

// ErrorResponse is a generic error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// --- Phase 2: New DTOs ---

// UpsertMetadataRequest is the JSON request for PUT /exif/metadata
type UpsertMetadataRequest struct {
	ImageFileID    uint    `json:"imageFileId"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	CameraModel    string  `json:"cameraModel"`
	LensModel      string  `json:"lensModel"`
	ISO            int     `json:"iso"`
	Aperture       string  `json:"aperture"`
	ShutterSpeed   string  `json:"shutterSpeed"`
	FocalLength    string  `json:"focalLength"`
	DateTaken      *string `json:"dateTaken,omitempty"` // RFC3339 format
	Orientation    int     `json:"orientation"`
	ColorSpace     string  `json:"colorSpace"`
	Software       string  `json:"software"`
	GeolocationRef *uint   `json:"geolocationRef,omitempty"`
}

// UpsertMetadataResponse is the JSON response for PUT /exif/metadata
type UpsertMetadataResponse struct {
	ID             uint    `json:"id"`
	ImageFileID    uint    `json:"imageFileId"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	CameraModel    string  `json:"cameraModel"`
	LensModel      string  `json:"lensModel"`
	ISO            int     `json:"iso"`
	Aperture       string  `json:"aperture"`
	ShutterSpeed   string  `json:"shutterSpeed"`
	FocalLength    string  `json:"focalLength"`
	DateTaken      *string `json:"dateTaken,omitempty"`
	Orientation    int     `json:"orientation"`
	ColorSpace     string  `json:"colorSpace"`
	Software       string  `json:"software"`
	GeolocationRef *uint   `json:"geolocationRef,omitempty"`
}

// DeleteMetadataResponse is the JSON response for DELETE /exif/metadata/:imageFileId
type DeleteMetadataResponse struct {
	Deleted bool `json:"deleted"`
}

// MetadataBatchResponse is the JSON response for GET /exif/metadata/batch
type MetadataBatchResponse struct {
	Metadata []MetadataResponse `json:"metadata"`
}

// CalendarItem represents a single item in the calendar gallery response
type CalendarItem struct {
	ImageFileID    uint    `json:"imageFileId"`
	Path           string  `json:"path"`
	DateTaken      string  `json:"dateTaken,omitempty"`
	GeolocationRef *uint   `json:"geolocationRef,omitempty"`
	GPSLatitude    float64 `json:"gpsLatitude,omitempty"`
	GPSLongitude   float64 `json:"gpsLongitude,omitempty"`
	NameLocal      string  `json:"nameLocal,omitempty"`
	NameEng        string  `json:"nameEng,omitempty"`
}

// CalendarResponse is the JSON response for GET /exif/metadata/calendar
type CalendarResponse struct {
	Items         []CalendarItem `json:"items"`
	NextCursor    string         `json:"nextCursor,omitempty"`
	MinDate       string         `json:"minDate,omitempty"`
	MaxDate       string         `json:"maxDate,omitempty"`
	TotalWithDate int64          `json:"totalWithDate"`
}

// GeoPointItem represents a GPS point for clustering
type GeoPointItem struct {
	ImageFileID  uint    `json:"imageFileId"`
	Path         string  `json:"path"`
	GPSLatitude  float64 `json:"gpsLatitude"`
	GPSLongitude float64 `json:"gpsLongitude"`
	NameLocal    string  `json:"nameLocal,omitempty"`
	NameEng      string  `json:"nameEng,omitempty"`
}

// GeoPointsResponse is the JSON response for GET /exif/metadata/geo-points
type GeoPointsResponse struct {
	Points []GeoPointItem `json:"points"`
}

// GeolocationResponse is the JSON response for GET /exif/geolocation
type GeolocationResponse struct {
	ID           uint    `json:"id"`
	GPSLatitude  float64 `json:"gpsLatitude"`
	GPSLongitude float64 `json:"gpsLongitude"`
	NameLocal    string  `json:"nameLocal"`
	NameEng      string  `json:"nameEng"`
}

// MissingImageItem represents an image missing EXIF data
type MissingImageItem struct {
	ImageFileID uint   `json:"imageFileId"`
	Path        string `json:"path"`
	MissingDate bool   `json:"missingDate"`
	MissingGPS  bool   `json:"missingGps"`
}

// MissingImagesResponse is the JSON response for GET /exif/missing
type MissingImagesResponse struct {
	Images   []MissingImageItem `json:"images"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}
