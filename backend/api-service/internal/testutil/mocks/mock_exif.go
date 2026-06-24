package mocks

import (
	"context"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
)

// MockExifClient is a test stub implementing the imaging.ExifClient interface.
type MockExifClient struct {
	// Configurable responses
	Metadata *domain.ImageMetadata
	GPSLat   float64
	GPSLng   float64
	GPSOk    bool
	HealthResult *domain.ExifHealthStatus
	// Error responses
	ExtractErr error
	WriteErr   error
	EnrichErr  error
	HealthErr  error
	// Call tracking
	ExtractCalls  int
	WriteCalls    int
	EnrichCalls   int
	HealthCalls   int
}

// NewMockExifClient creates a new mock with default healthy responses.
func NewMockExifClient() *MockExifClient {
	now := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
	return &MockExifClient{
		Metadata: &domain.ImageMetadata{
			Width:       4032,
			Height:      3024,
			CameraModel: "Canon EOS R5",
			LensModel:   "RF 24-70mm F2.8 L IS USM",
			ISO:         400,
			Aperture:    "f/2.8",
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
	return nil, nil // no enrichment in mock
}

func (m *MockExifClient) Health(ctx context.Context) (*domain.ExifHealthStatus, error) {
	m.HealthCalls++
	if m.HealthErr != nil {
		return nil, m.HealthErr
	}
	return m.HealthResult, nil
}
