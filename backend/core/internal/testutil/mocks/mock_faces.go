package mocks

import (
	"context"

	"github.com/flashbacks/core/internal/infrastructure/faces"
)

// MockFacesClient is a mock implementation of faces.Client for testing.
type MockFacesClient struct {
	DetectFunc       func(ctx context.Context, image []byte) ([]faces.FaceBox, error)
	EmbedFunc        func(ctx context.Context, image []byte, box *faces.FaceBox) ([]float32, error)
	ClusterFunc      func(ctx context.Context, embeddings [][]float32, minClusterSize int) ([]int, error)
	ServiceURLValue  string
	DetectCallCount  int
	EmbedCallCount   int
	ClusterCallCount int
}

// ServiceURL implements faces.Client.
func (m *MockFacesClient) ServiceURL() string {
	return m.ServiceURLValue
}

// Detect implements faces.Client.
func (m *MockFacesClient) Detect(ctx context.Context, image []byte) ([]faces.FaceBox, error) {
	m.DetectCallCount++
	if m.DetectFunc != nil {
		return m.DetectFunc(ctx, image)
	}
	return nil, nil
}

// Embed implements faces.Client.
func (m *MockFacesClient) Embed(ctx context.Context, image []byte, box *faces.FaceBox) ([]float32, error) {
	m.EmbedCallCount++
	if m.EmbedFunc != nil {
		return m.EmbedFunc(ctx, image, box)
	}
	return nil, nil
}

// Cluster implements faces.Client.
func (m *MockFacesClient) Cluster(ctx context.Context, embeddings [][]float32, minClusterSize int) ([]int, error) {
	m.ClusterCallCount++
	if m.ClusterFunc != nil {
		return m.ClusterFunc(ctx, embeddings, minClusterSize)
	}
	return nil, nil
}

// SingleFaceBoxes returns one deterministic face box.
func SingleFaceBoxes() []faces.FaceBox {
	return []faces.FaceBox{{X: 10, Y: 20, Width: 30, Height: 40, Confidence: 0.99}}
}

// Vector512 returns a deterministic 512-d embedding.
func Vector512() []float32 {
	vec := make([]float32, 512)
	for i := range vec {
		vec[i] = 0.01
	}
	return vec
}
