package imaging

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/ocr"
	"github.com/flashbacks/api-service/internal/testutil"
	"github.com/flashbacks/api-service/internal/testutil/fixtures"
	"github.com/flashbacks/api-service/internal/testutil/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupOcrManager(t *testing.T) (*OcrManager, *mocks.MockOcrClient, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	mockOcr := &mocks.MockOcrClient{}
	om := NewOcrManager(db, mockOcr, 2)
	return om, mockOcr, cleanup
}

func TestOcrManager_StartClassification_Success(t *testing.T) {
	om, _, _ := setupOcrManager(t)

	err := om.StartClassification(false)

	require.NoError(t, err)

	// Check status immediately - it should be processing right after start
	status := om.GetStatus()
	assert.True(t, status.Processing, "should be processing immediately after start")
}

func TestOcrManager_StartClassification_AlreadyProcessing(t *testing.T) {
	om, _, _ := setupOcrManager(t)

	err := om.StartClassification(false)
	require.NoError(t, err)

	err = om.StartClassification(false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "OCR classification already in progress")
}

func TestOcrManager_StopClassification(t *testing.T) {
	om, _, _ := setupOcrManager(t)

	err := om.StartClassification(false)
	require.NoError(t, err)

	om.StopClassification()

	time.Sleep(50 * time.Millisecond)
	om.mu.RLock()
	assert.True(t, om.stopRequested, "stop should be requested")
	om.mu.RUnlock()
}

func TestOcrManager_GetEffectiveWorkers_Auto(t *testing.T) {
	om, _, _ := setupOcrManager(t)
	om.maxWorkers = 0

	workers := om.GetEffectiveWorkers()

	assert.Equal(t, runtime.NumCPU(), workers, "should use runtime.NumCPU() when maxWorkers=0")
}

func TestOcrManager_GetEffectiveWorkers_Explicit(t *testing.T) {
	om, _, _ := setupOcrManager(t)
	om.maxWorkers = 4

	workers := om.GetEffectiveWorkers()

	assert.Equal(t, 4, workers, "should use explicit maxWorkers value")
}

func TestOcrManager_GetEffectiveWorkers_Capped(t *testing.T) {
	om, _, _ := setupOcrManager(t)
	om.maxWorkers = 1000

	workers := om.GetEffectiveWorkers()

	assert.Equal(t, runtime.NumCPU(), workers, "should cap at runtime.NumCPU()")
}

func TestOcrManager_ProcessUnclassified_NoImages(t *testing.T) {
	om, mockOcr, _ := setupOcrManager(t)

	mockOcr.ClassifyFunc = func(ctx context.Context, image io.Reader, contentType string, params *ocr.ClassifyParams) (*ocr.ClassifyResponse, error) {
		return mocks.TextDocumentResponse(0.95, 0.90, 100, 0), nil
	}

	om.processUnclassified(false)

	status := om.GetStatus()
	// After completion with no images, should be not processing
	assert.False(t, status.Processing, "should not be processing after completion")
}

func TestOcrManager_Process_Unclassified_WithImages(t *testing.T) {
	om, mockOcr, cleanup := setupOcrManager(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)
	for _, path := range paths {
		testutil.SeedImageFileNoT(om.db, path, "hash-"+path, 1000)
	}

	mockOcr.ClassifyFunc = func(ctx context.Context, image io.Reader, contentType string, params *ocr.ClassifyParams) (*ocr.ClassifyResponse, error) {
		return mocks.TextDocumentResponse(0.95, 0.90, 100, 0), nil
	}

	// Call processUnclassified synchronously to avoid goroutine timing issues
	om.processUnclassified(false)

	var count int64
	om.db.Model(&domain.OcrClassification{}).Count(&count)
	assert.GreaterOrEqual(t, count, int64(1), "should have saved at least 1 classification")
}

func TestOcrManager_Process_OCRClientError(t *testing.T) {
	om, mockOcr, cleanup := setupOcrManager(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	path := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 1)[0]
	testutil.SeedImageFileNoT(om.db, path, "hash-error-test", 1000)

	mockOcr.ClassifyFunc = func(ctx context.Context, image io.Reader, contentType string, params *ocr.ClassifyParams) (*ocr.ClassifyResponse, error) {
		return nil, assert.AnError
	}

	// Call processUnclassified synchronously to avoid goroutine timing issues
	om.processUnclassified(false)

	// After synchronous processing, should not be in processing state
	assert.False(t, om.IsProcessing(), "should not be processing after synchronous call")
}

func TestOcrManager_ConsumeResults_BatchSaving(t *testing.T) {
	om, _, cleanup := setupOcrManager(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	// Create 25 image files with Windows-safe names
	imageFiles := make([]*domain.ImageFile, 25)
	for i := 0; i < 25; i++ {
		name := fmt.Sprintf("test_%02d.jpg", i)
		path := fixtures.CreateMinimalJPEG(t, tmpDir, name, 100, 100)
		imageFiles[i] = testutil.SeedImageFileNoT(om.db, path, "hash-batch-test", 1000)
	}

	results := make(chan OcrResult, 25)
	var wg sync.WaitGroup
	done := make(chan struct{})

	go om.consumeResults(results, &wg, done)

	for i := 0; i < 25; i++ {
		results <- OcrResult{
			Image: *imageFiles[i],
			Classification: &domain.OcrClassification{
				ImageFileID:    imageFiles[i].ID,
				IsTextDocument: true,
				MeanConfidence: 0.95,
			},
		}
	}

	wg.Wait()
	close(results)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("consumeResults did not complete within timeout")
	}

	var count int64
	om.db.Model(&domain.OcrClassification{}).Count(&count)
	assert.Equal(t, int64(25), count, "should have saved all 25 classifications")
}
