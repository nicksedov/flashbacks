package mocks

import (
	"context"
	"io"

	"github.com/flashbacks/api-service/internal/infrastructure/healthcheck"
	"github.com/flashbacks/api-service/internal/infrastructure/ocr"
	shareddomain "github.com/flashbacks/shared/domain"
)

// MockOcrClient is a mock implementation of ocr.Client for testing.
type MockOcrClient struct {
	ClassifyFunc    func(ctx context.Context, image io.Reader, contentType string, params *ocr.ClassifyParams) (*shareddomain.ClassifyResponse, error)
	HealthFunc      func(ctx context.Context) error
	GetStatusFunc   func() healthcheck.HealthStatus
	StartHealthFunc func(intervalSeconds int)
	StopHealthFunc  func()

	// Counters
	ClassifyCallCount int
	HealthCallCount   int
}

// CheckHealth implements ocr.Client.
func (m *MockOcrClient) CheckHealth(ctx context.Context) error {
	m.HealthCallCount++
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}
	return nil
}

// GetStatus implements ocr.Client.
func (m *MockOcrClient) GetStatus() healthcheck.HealthStatus {
	if m.GetStatusFunc != nil {
		return m.GetStatusFunc()
	}
	return healthcheck.HealthStatus{Status: healthcheck.StatusHealthy}
}

// StartHealthCheck implements ocr.Client.
func (m *MockOcrClient) StartHealthCheck(intervalSeconds int) {
	if m.StartHealthFunc != nil {
		m.StartHealthFunc(intervalSeconds)
	}
}

// StopHealthCheck implements ocr.Client.
func (m *MockOcrClient) StopHealthCheck() {
	if m.StopHealthFunc != nil {
		m.StopHealthFunc()
	}
}

// Classify implements ocr.Client.
func (m *MockOcrClient) Classify(ctx context.Context, image io.Reader, contentType string, params *ocr.ClassifyParams) (*shareddomain.ClassifyResponse, error) {
	m.ClassifyCallCount++
	if m.ClassifyFunc != nil {
		return m.ClassifyFunc(ctx, image, contentType, params)
	}
	return nil, nil
}

// TextDocumentResponse returns a mock OCR response for a text document with bounding boxes.
func TextDocumentResponse(meanConfidence, weightedConfidence float64, tokenCount int, angle int) *shareddomain.ClassifyResponse {
	return &shareddomain.ClassifyResponse{
		IsTextDocument:     true,
		MeanConfidence:     meanConfidence,
		WeightedConfidence: weightedConfidence,
		TokenCount:         tokenCount,
		Angle:              angle,
		ScaleFactor:        1.0,
		BoundingBoxWidth:   200,
		BoundingBoxHeight:  50,
		Boxes: []shareddomain.BoundingBox{
			{X: 10, Y: 10, Width: 100, Height: 20, Word: "Hello", Confidence: 0.95},
			{X: 10, Y: 35, Width: 80, Height: 20, Word: "World", Confidence: 0.90},
		},
	}
}

// NonTextResponse returns a mock OCR response for a non-text document (photo).
func NonTextResponse() *shareddomain.ClassifyResponse {
	return &shareddomain.ClassifyResponse{
		IsTextDocument:     false,
		MeanConfidence:     0.1,
		WeightedConfidence: 0.1,
		TokenCount:         0,
		Angle:              0,
		ScaleFactor:        1.0,
		Boxes:              []shareddomain.BoundingBox{},
	}
}

// ErrorResponse is a helper that returns an error from Classify.
func ErrorResponse(err error) func(ctx context.Context, image io.Reader, contentType string, params *ocr.ClassifyParams) (*shareddomain.ClassifyResponse, error) {
	return func(ctx context.Context, image io.Reader, contentType string, params *ocr.ClassifyParams) (*shareddomain.ClassifyResponse, error) {
		return nil, err
	}
}
