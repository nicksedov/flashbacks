package imaging

import (
	"context"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
)

// ExifClient abstracts EXIF operations for dependency injection.
// Two implementations: HTTPExifClient (production) and MockExifClient (tests).
type ExifClient interface {
	// ExtractMetadata reads EXIF metadata from an image file.
	ExtractMetadata(ctx context.Context, filePath string) (*domain.ImageMetadata, error)

	// ExtractGPS reads GPS coordinates from an image file's EXIF.
	ExtractGPS(ctx context.Context, filePath string) (lat, lng float64, ok bool, err error)

	// WriteGPS writes GPS coordinates to an image file's EXIF.
	// backupDir is the directory where the EXIF service stores a backup of the original file before modification.
	WriteGPS(ctx context.Context, filePath string, lat, lng float64, backupDir string, meta *domain.ImageMetadata) error

	// EnrichMissingMetadata fills empty fields in existing metadata from the file.
	EnrichMissingMetadata(ctx context.Context, filePath string, meta *domain.ImageMetadata) (map[string]interface{}, error)

	// Health checks the EXIF service health status.
	Health(ctx context.Context) (*domain.ExifHealthStatus, error)
}

// ExifServiceStatus is the status returned to the frontend.
type ExifServiceStatus struct {
	Enabled    bool      `json:"enabled"`
	Health     string    `json:"health"`
	LastCheck  time.Time `json:"lastCheck"`
	Error      string    `json:"error"`
	ServiceURL string    `json:"serviceURL"`
}
