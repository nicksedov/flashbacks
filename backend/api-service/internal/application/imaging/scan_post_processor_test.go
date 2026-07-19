package imaging

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"
	"github.com/flashbacks/api-service/internal/testutil/fixtures"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostProcessor_ScanDirectory_ReturnsCreatedInBatch(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	progressChan := make(chan string, 100)
	batch, err := scanDirectory(db, tmpDir, progressChan, 2)
	require.NoError(t, err)

	assert.Len(t, batch.Created, 3, "batch should contain 3 created files")
	assert.Empty(t, batch.Updated, "no files should be updated")
	assert.Empty(t, batch.Deleted, "no files should be deleted")

	for _, f := range batch.Created {
		assert.NotZero(t, f.ID, "created file should have a populated ID")
		assert.NotEmpty(t, f.Path, "created file should have a path")
	}
}

func TestPostProcessor_ScanDirectory_ReturnsModifiedInBatch(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)

	// First scan to populate DB
	progressChan := make(chan string, 100)
	_, err := scanDirectory(db, tmpDir, progressChan, 1)
	require.NoError(t, err)

	// Modify the file with different content
	fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 200, 200)

	// Second scan — should detect modification
	progressChan2 := make(chan string, 100)
	batch, err := scanDirectory(db, tmpDir, progressChan2, 1)
	require.NoError(t, err)

	assert.Empty(t, batch.Created, "no new files expected")
	require.Len(t, batch.Updated, 1, "1 updated file expected")
	assert.True(t, batch.Updated[0].ContentChanged, "content should be detected as changed (different hash)")
	assert.NotZero(t, batch.Updated[0].File.ID)
}

func TestPostProcessor_FastScan_ReturnsDeletedInBatch(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 5)

	// First scan to populate DB
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	// Delete 2 files from disk
	os.Remove(paths[0])
	os.Remove(paths[1])

	// Fast scan — should detect deletions
	progressChan2 := make(chan string, 100)
	result, batch := fastScanGalleryDirectory(db, tmpDir, progressChan2, 1)

	assert.Equal(t, 2, result.Deleted)
	assert.Len(t, batch.Deleted, 2, "batch should contain 2 deleted files")

	for _, f := range batch.Deleted {
		assert.NotZero(t, f.ID)
		assert.NotEmpty(t, f.Path)
	}
}

func TestPostProcessor_CleanupMissingFiles_ReturnsDeletedInBatch(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	// First scan to populate DB
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	// Delete 1 file from disk
	os.Remove(paths[0])

	// Cleanup — should return deleted records
	progressChan2 := make(chan string, 100)
	batch, err := cleanupMissingFiles(db, progressChan2)
	require.NoError(t, err)

	require.Len(t, batch.Deleted, 1, "batch should contain 1 deleted file")
	assert.NotZero(t, batch.Deleted[0].ID)
}

func TestPostProcessor_ProcessBatchResult_EmitsEvents(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	sm := NewScanManager(db, 4)

	var mu sync.Mutex
	var events []FileEvent
	sm.OnFileProcessed = func(event FileEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	}

	batch := ScanBatchResult{
		Created: []domain.ImageFile{{ID: 1, Path: "/a.jpg"}, {ID: 2, Path: "/b.jpg"}},
		Updated: []updatedFile{{File: domain.ImageFile{ID: 3, Path: "/c.jpg"}, ContentChanged: true}},
		Deleted: []domain.ImageFile{{ID: 4, Path: "/d.jpg"}},
	}

	sm.processBatchResult(batch)

	// Wait for async goroutines to complete
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n >= 4 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, events, 4)

	var created, modified, deleted int
	for _, e := range events {
		switch e.Type {
		case FileCreated:
			created++
		case FileModified:
			modified++
			assert.True(t, e.ContentChanged)
		case FileDeleted:
			deleted++
		}
	}
	assert.Equal(t, 2, created)
	assert.Equal(t, 1, modified)
	assert.Equal(t, 1, deleted)
}

func TestPostProcessor_ProcessBatchResult_NilCallback(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	sm := NewScanManager(db, 2)
	// OnFileProcessed is nil by default

	batch := ScanBatchResult{
		Created: []domain.ImageFile{{ID: 1, Path: "/a.jpg"}},
	}

	// Should not panic
	sm.processBatchResult(batch)
}

func TestPostProcessor_ProcessBatchResult_BoundedConcurrency(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	sm := NewScanManager(db, 2) // semaphore capacity = 2

	var maxConcurrent atomic.Int32
	var currentConcurrent atomic.Int32
	var wg sync.WaitGroup

	const itemCount = 20
	wg.Add(itemCount)

	sm.OnFileProcessed = func(event FileEvent) {
		cur := currentConcurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		currentConcurrent.Add(-1)
		wg.Done()
	}

	var items []domain.ImageFile
	for i := 0; i < itemCount; i++ {
		items = append(items, domain.ImageFile{ID: uint(i + 1), Path: fmt.Sprintf("/img_%02d.jpg", i)})
	}

	sm.processBatchResult(ScanBatchResult{Created: items})

	wg.Wait()

	observed := maxConcurrent.Load()
	assert.LessOrEqual(t, int(observed), 2, "max concurrent goroutines should not exceed semaphore capacity of 2")
	assert.GreaterOrEqual(t, int(observed), 1, "at least 1 goroutine should have run")
}

func TestPostProcessor_FastScan_ReturnsCreatedAndModifiedInBatch(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMinimalJPEG(t, tmpDir, "existing.jpg", 100, 100)

	// First scan to populate DB
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	// Add a new file and modify existing one
	fixtures.CreateMinimalJPEG(t, tmpDir, "new.jpg", 150, 150)
	fixtures.CreateMinimalJPEG(t, tmpDir, "existing.jpg", 200, 200)

	// Fast scan
	progressChan2 := make(chan string, 100)
	result, batch := fastScanGalleryDirectory(db, tmpDir, progressChan2, 1)

	assert.GreaterOrEqual(t, result.Created, 1)
	assert.GreaterOrEqual(t, result.Modified, 1)

	assert.GreaterOrEqual(t, len(batch.Created), 1, "batch should have at least 1 created file")
	assert.GreaterOrEqual(t, len(batch.Updated), 1, "batch should have at least 1 updated file")
}

func TestPostProcessor_ScanDirectory_NoChangesReturnsEmptyBatch(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	// First scan
	progressChan := make(chan string, 100)
	_, err := scanDirectory(db, tmpDir, progressChan, 1)
	require.NoError(t, err)

	// Second scan — all cached, no changes
	progressChan2 := make(chan string, 100)
	batch, err := scanDirectory(db, tmpDir, progressChan2, 1)
	require.NoError(t, err)

	assert.Empty(t, batch.Created, "no new files on second scan")
	assert.Empty(t, batch.Updated, "no modified files on second scan")
	assert.Empty(t, batch.Deleted, "no deleted files on second scan")
}
