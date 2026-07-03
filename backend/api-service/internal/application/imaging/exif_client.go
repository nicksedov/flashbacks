package imaging

import (
	"context"

	"github.com/flashbacks/api-service/internal/domain"
)

// ExifClient abstracts EXIF operations for dependency injection.
// Two implementations: HTTPExifClient (production) and MockExifClient (tests).
type ExifClient interface {
	ExtractMetadata(ctx context.Context, filePath string) (*domain.ImageMetadata, error)
	ExtractGPS(ctx context.Context, filePath string) (lat, lng float64, ok bool, err error)
	WriteGPS(ctx context.Context, filePath string, lat, lng float64, backupDir string, meta *domain.ImageMetadata) error
	EnrichMissingMetadata(ctx context.Context, filePath string, meta *domain.ImageMetadata) (map[string]interface{}, error)
	Health(ctx context.Context) (*domain.ExifHealthStatus, error)

	// --- Phase 3: New metadata operations ---
	GetMetadataByImageID(ctx context.Context, imageFileID uint) (*domain.ImageMetadata, error)
	UpsertMetadata(ctx context.Context, meta *domain.ImageMetadata) (*domain.ImageMetadata, error)
	DeleteMetadata(ctx context.Context, imageFileID uint) error
	GetMetadataBatch(ctx context.Context, imageFileIDs []uint) (map[uint]*domain.ImageMetadata, error)
	GetCalendarItems(ctx context.Context, params domain.CalendarParams) (*domain.CalendarResult, error)
	GetGeoPoints(ctx context.Context, bounds domain.GeoBounds) ([]domain.GeoPoint, error)
	GetMissingImages(ctx context.Context, page, pageSize int) (*domain.MissingImagesResult, error)
	GetLocationCandidates(ctx context.Context, date string) ([]domain.LocationCandidate, error)
	ResolveGeolocation(ctx context.Context, lat, lng float64) (*domain.GeolocationCache, error)
}
