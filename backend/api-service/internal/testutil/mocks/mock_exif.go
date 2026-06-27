package mocks

import (
	"context"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
)

// MockExifClient is a test stub implementing the imaging.ExifClient interface.
type MockExifClient struct {
	// Configurable responses
	Metadata     *domain.ImageMetadata
	GPSLat       float64
	GPSLng       float64
	GPSOk        bool
	HealthResult *domain.ExifHealthStatus
	// Phase 3: New operation results
	GeoCache       *domain.GeolocationCache
	CalendarResult *domain.CalendarResult
	GeoPoints      []domain.GeoPoint
	MissingResult  *domain.MissingImagesResult
	LocCandidates  []domain.LocationCandidate
	// Error responses
	ExtractErr error
	WriteErr   error
	EnrichErr  error
	HealthErr  error
	// Phase 3: New operation errors
	GetMetaErr       error
	UpsertErr        error
	DeleteErr        error
	BatchErr         error
	CalendarErr      error
	GeoPointsErr     error
	MissingErr       error
	LocCandidatesErr error
	GeolocationErr   error
	// Call tracking
	ExtractCalls    int
	WriteCalls      int
	EnrichCalls     int
	HealthCalls     int
	GetMetaCalls    int
	UpsertCalls     int
	DeleteCalls     int
	BatchCalls      int
	CalendarCalls   int
	GeoPointsCalls  int
	MissingCalls    int
	LocCandCalls    int
	GeoResolveCalls int
}

// NewMockExifClient creates a new mock with default healthy responses.
func NewMockExifClient() *MockExifClient {
	now := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
	return &MockExifClient{
		Metadata: &domain.ImageMetadata{
			Width:        4032,
			Height:       3024,
			CameraModel:  "Canon EOS R5",
			LensModel:    "RF 24-70mm F2.8 L IS USM",
			ISO:          400,
			Aperture:     "f/2.8",
			ShutterSpeed: "1/250s",
			FocalLength:  "50mm",
			DateTaken:    &now,
			Orientation:  1,
			ColorSpace:   "sRGB",
			Software:     "Adobe Photoshop 25.0",
		},
		GPSLat: 55.7558,
		GPSLng: 37.6173,
		GPSOk:  true,
		HealthResult: &domain.ExifHealthStatus{
			Status:            "healthy",
			Version:           "1.0.0",
			ExiftoolAvailable: true,
			DatabaseConnected: true,
			Uptime:            "1h0m",
		},
	}
}

func (m *MockExifClient) ExtractMetadata(ctx context.Context, filePath string) (*domain.ImageMetadata, error) {
	m.ExtractCalls++
	if m.ExtractErr != nil {
		return nil, m.ExtractErr
	}
	result := *m.Metadata
	return &result, nil
}

func (m *MockExifClient) ExtractGPS(ctx context.Context, filePath string) (float64, float64, bool, error) {
	m.ExtractCalls++
	if m.ExtractErr != nil {
		return 0, 0, false, m.ExtractErr
	}
	return m.GPSLat, m.GPSLng, m.GPSOk, nil
}

func (m *MockExifClient) WriteGPS(ctx context.Context, filePath string, lat, lng float64, backupDir string, meta *domain.ImageMetadata) error {
	m.WriteCalls++
	return m.WriteErr
}

func (m *MockExifClient) EnrichMissingMetadata(ctx context.Context, filePath string, meta *domain.ImageMetadata) (map[string]interface{}, error) {
	m.EnrichCalls++
	if m.EnrichErr != nil {
		return nil, m.EnrichErr
	}
	return nil, nil
}

func (m *MockExifClient) Health(ctx context.Context) (*domain.ExifHealthStatus, error) {
	m.HealthCalls++
	if m.HealthErr != nil {
		return nil, m.HealthErr
	}
	return m.HealthResult, nil
}

// --- Phase 3: New methods ---

func (m *MockExifClient) GetMetadataByImageID(ctx context.Context, imageFileID uint) (*domain.ImageMetadata, error) {
	m.GetMetaCalls++
	if m.GetMetaErr != nil {
		return nil, m.GetMetaErr
	}
	result := *m.Metadata
	return &result, nil
}

func (m *MockExifClient) UpsertMetadata(ctx context.Context, meta *domain.ImageMetadata) (*domain.ImageMetadata, error) {
	m.UpsertCalls++
	if m.UpsertErr != nil {
		return nil, m.UpsertErr
	}
	return meta, nil
}

func (m *MockExifClient) DeleteMetadata(ctx context.Context, imageFileID uint) error {
	m.DeleteCalls++
	return m.DeleteErr
}

func (m *MockExifClient) GetMetadataBatch(ctx context.Context, imageFileIDs []uint) (map[uint]*domain.ImageMetadata, error) {
	m.BatchCalls++
	if m.BatchErr != nil {
		return nil, m.BatchErr
	}
	result := make(map[uint]*domain.ImageMetadata, len(imageFileIDs))
	for _, id := range imageFileIDs {
		copy := *m.Metadata
		result[id] = &copy
	}
	return result, nil
}

func (m *MockExifClient) GetCalendarItems(ctx context.Context, params domain.CalendarParams) (*domain.CalendarResult, error) {
	m.CalendarCalls++
	if m.CalendarErr != nil {
		return nil, m.CalendarErr
	}
	if m.CalendarResult != nil {
		return m.CalendarResult, nil
	}
	return &domain.CalendarResult{}, nil
}

func (m *MockExifClient) GetGeoPoints(ctx context.Context, bounds domain.GeoBounds) ([]domain.GeoPoint, error) {
	m.GeoPointsCalls++
	if m.GeoPointsErr != nil {
		return nil, m.GeoPointsErr
	}
	return m.GeoPoints, nil
}

func (m *MockExifClient) GetMissingImages(ctx context.Context, page, pageSize int) (*domain.MissingImagesResult, error) {
	m.MissingCalls++
	if m.MissingErr != nil {
		return nil, m.MissingErr
	}
	if m.MissingResult != nil {
		return m.MissingResult, nil
	}
	return &domain.MissingImagesResult{}, nil
}

func (m *MockExifClient) GetLocationCandidates(ctx context.Context, date string) ([]domain.LocationCandidate, error) {
	m.LocCandCalls++
	if m.LocCandidatesErr != nil {
		return nil, m.LocCandidatesErr
	}
	return m.LocCandidates, nil
}

func (m *MockExifClient) ResolveGeolocation(ctx context.Context, lat, lng float64) (*domain.GeolocationCache, error) {
	m.GeoResolveCalls++
	if m.GeolocationErr != nil {
		return nil, m.GeolocationErr
	}
	if m.GeoCache != nil {
		return m.GeoCache, nil
	}
	return &domain.GeolocationCache{
		GPSLatitude:  lat,
		GPSLongitude: lng,
		NameLocal:    "Москва",
		NameEng:      "Moscow",
	}, nil
}
