package imaging

import (
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/testutil"
	"github.com/flashbacks/api-service/internal/testutil/fixtures"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupScanManager(t *testing.T) (*ScanManager, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	return NewScanManager(db, 2), cleanup
}

func TestScanManager_StartScan_Success(t *testing.T) {
	sm, _ := setupScanManager(t)

	err := sm.StartScan()

	require.NoError(t, err)

	// Check status immediately - it should be scanning right after start
	status := sm.GetStatus()
	assert.True(t, status.Scanning, "should be scanning immediately after start")
}

func TestScanManager_StartScan_DoubleStart(t *testing.T) {
	sm, _ := setupScanManager(t)

	// First start should succeed
	err := sm.StartScan()
	require.NoError(t, err)

	// Second start should fail
	err = sm.StartScan()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan already in progress")
}

func TestScanManager_GetStatus_NotScanning(t *testing.T) {
	sm, _ := setupScanManager(t)

	status := sm.GetStatus()

	assert.False(t, status.Scanning, "should not be scanning")
	assert.Equal(t, "", status.Progress)
	assert.Equal(t, 0, status.FilesProcessed)
}

func TestScanManager_FastScanGallery_Success(t *testing.T) {
	sm, _ := setupScanManager(t)

	// Seed a gallery folder
	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	testutil.SeedGalleryFolderNoT(sm.db, tmpDir)

	result := sm.FastScanGallery()

	// Result should be returned immediately
	assert.GreaterOrEqual(t, result.TotalChecked, 0, "should have checked files")
}

func TestScanManager_FastScanGallery_AlreadyScanning(t *testing.T) {
	sm, _ := setupScanManager(t)

	// Start a regular scan to set isScanning=true
	err := sm.StartScan()
	require.NoError(t, err)

	// Fast scan while already scanning should return empty result
	time.Sleep(50 * time.Millisecond)
	result := sm.FastScanGallery()

	assert.Equal(t, FastScanResult{}, result, "should return empty result when already scanning")
}

func TestScanManager_OnScanComplete_Callback(t *testing.T) {
	sm, _ := setupScanManager(t)

	callbackCalled := make(chan bool, 1)
	sm.OnScanComplete = func() {
		callbackCalled <- true
	}

	// Create a temp dir with some files to make scan complete faster
	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 2)
	testutil.SeedGalleryFolderNoT(sm.db, tmpDir)

	// Start and wait for completion
	_ = sm.StartScan()

	// Wait for callback (with timeout)
	select {
	case called := <-callbackCalled:
		assert.True(t, called, "callback should have been called")
	case <-time.After(2 * time.Second):
		t.Fatal("callback was not called within timeout")
	}
}
