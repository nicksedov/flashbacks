package domain

import "time"

// ImageMetadata stores extracted EXIF metadata for an image.
// Geolocation is resolved via GeolocationRef -> GeolocationCache (Nominatim-backed).
type ImageMetadata struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ImageFileID    uint       `gorm:"uniqueIndex;not null" json:"imageFileId"`
	Width          int        `json:"width"`
	Height         int        `json:"height"`
	CameraModel    string     `json:"cameraModel"`
	LensModel      string     `json:"lensModel"`
	ISO            int        `json:"iso"`
	Aperture       string     `json:"aperture"`
	ShutterSpeed   string     `json:"shutterSpeed"`
	FocalLength    string     `json:"focalLength"`
	DateTaken      *time.Time `json:"dateTaken"`
	Orientation    int        `json:"orientation"`
	ColorSpace     string     `json:"colorSpace"`
	Software       string     `json:"software"`
	GeolocationRef *uint      `gorm:"index" json:"geolocationRef"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// GeolocationCache stores reverse-geocoded location names for unique GPS coordinate pairs.
// Populated by Nominatim reverse geocoding; referenced by ImageMetadata.GeolocationRef.
type GeolocationCache struct {
	ID           uint    `gorm:"primaryKey" json:"id"`
	GPSLatitude  float64 `gorm:"uniqueIndex:idx_geo_lat_lng;not null" json:"gpsLatitude"`
	GPSLongitude float64 `gorm:"uniqueIndex:idx_geo_lat_lng;not null" json:"gpsLongitude"`
	NameLocal    string  `gorm:"type:text" json:"nameLocal"`
	NameEng      string  `gorm:"type:text" json:"nameEng"`
}

// ExifHealthStatus represents the EXIF service health check result.
type ExifHealthStatus struct {
	Status            string `json:"status"`
	Version           string `json:"version"`
	ExiftoolAvailable bool   `json:"exiftoolAvailable"`
	DatabaseConnected bool   `json:"databaseConnected"`
	Uptime            string `json:"uptime"`
}

// LocationCandidate represents a location suggestion for GPS assignment.
type LocationCandidate struct {
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	NameLocal  string  `json:"nameLocal"`
	NameEng    string  `json:"nameEng"`
	PhotoCount int     `json:"photoCount"`
}

// CalendarItem represents a single item in the calendar gallery view.
type CalendarItem struct {
	ImageFileID    uint    `json:"imageFileId"`
	DateTaken      string  `json:"dateTaken"`
	GeolocationRef *uint   `json:"geolocationRef"`
	GPSLatitude    float64 `json:"gpsLatitude"`
	GPSLongitude   float64 `json:"gpsLongitude"`
	NameLocal      string  `json:"nameLocal"`
	NameEng        string  `json:"nameEng"`
}

// MissingImageItem represents an image that is missing EXIF metadata.
type MissingImageItem struct {
	ImageFileID uint   `json:"imageFileId"`
	Path        string `json:"path"`
	MissingDate bool   `json:"missingDate"`
	MissingGPS  bool   `json:"missingGps"`
}
