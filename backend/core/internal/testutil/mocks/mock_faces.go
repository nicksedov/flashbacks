package mocks

import (
	"context"

	"github.com/flashbacks/core/internal/infrastructure/faces"
)

// MockFacesClient is a mock implementation of faces.Client for testing.
type MockFacesClient struct {
	DetectFunc       func(ctx context.Context, image []byte) ([]faces.FaceBox, error)
	AnalyzeFunc      func(ctx context.Context, image []byte) ([]faces.AnalyzedFace, error)
	AnalyzeBatchFunc func(ctx context.Context, images [][]byte) ([]faces.BatchResult, error)
	EmbedFunc        func(ctx context.Context, image []byte, box *faces.FaceBox) ([]float32, error)
	ClusterFunc      func(ctx context.Context, embeddings [][]float32, minClusterSize int, soloFaces []bool) ([]int, error)
	ClusterGraphFunc func(ctx context.Context, faceIDs []uint, edges []faces.FaceLink, soloFaces []bool) ([]int, error)
	HealthFunc       func(ctx context.Context) (faces.Health, error)
	ServiceURLValue  string
	DetectCallCount  int
	AnalyzeCallCount int
	// AnalyzeBatchCallCount counts batch requests; AnalyzeBatchSizes records the
	// number of images each request carried.
	AnalyzeBatchCallCount int
	AnalyzeBatchSizes     []int
	EmbedCallCount        int
	ClusterCallCount      int
	ClusterGraphCallCount int
	HealthCallCount       int
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

// Analyze implements faces.Client.
func (m *MockFacesClient) Analyze(ctx context.Context, image []byte) ([]faces.AnalyzedFace, error) {
	m.AnalyzeCallCount++
	if m.AnalyzeFunc != nil {
		return m.AnalyzeFunc(ctx, image)
	}
	return nil, nil
}

// AnalyzeBatch implements faces.Client.
func (m *MockFacesClient) AnalyzeBatch(ctx context.Context, images [][]byte) ([]faces.BatchResult, error) {
	m.AnalyzeBatchCallCount++
	m.AnalyzeBatchSizes = append(m.AnalyzeBatchSizes, len(images))
	if m.AnalyzeBatchFunc != nil {
		return m.AnalyzeBatchFunc(ctx, images)
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
func (m *MockFacesClient) Cluster(ctx context.Context, embeddings [][]float32, minClusterSize int, soloFaces []bool) ([]int, error) {
	m.ClusterCallCount++
	if m.ClusterFunc != nil {
		return m.ClusterFunc(ctx, embeddings, minClusterSize, soloFaces)
	}
	return nil, nil
}

// ClusterGraph implements faces.Client.
func (m *MockFacesClient) ClusterGraph(ctx context.Context, faceIDs []uint, edges []faces.FaceLink, soloFaces []bool) ([]int, error) {
	m.ClusterGraphCallCount++
	if m.ClusterGraphFunc != nil {
		return m.ClusterGraphFunc(ctx, faceIDs, edges, soloFaces)
	}
	return nil, nil
}

// Health implements faces.Client.
func (m *MockFacesClient) Health(ctx context.Context) (faces.Health, error) {
	m.HealthCallCount++
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}
	return faces.Health{}, nil
}

// SingleFaceBoxes returns one deterministic face box.
func SingleFaceBoxes() []faces.FaceBox {
	return []faces.FaceBox{{X: 10, Y: 20, Width: 30, Height: 40, Confidence: 0.99}}
}

// SingleAnalyzedFace returns one deterministic analysed face (box + embedding).
func SingleAnalyzedFace(embedding []float32) faces.AnalyzedFace {
	return faces.AnalyzedFace{FaceBox: SingleFaceBoxes()[0], Embedding: embedding}
}

// Vector512 returns a deterministic 512-d embedding.
func Vector512() []float32 {
	vec := make([]float32, 512)
	for i := range vec {
		vec[i] = 0.01
	}
	return vec
}
