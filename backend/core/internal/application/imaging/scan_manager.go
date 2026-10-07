package imaging

import (
	"fmt"
	"sync"

	"github.com/flashbacks/api-service/internal/domain"

	"gorm.io/gorm"
)

// ScanStatusResponse is the JSON response for GET /api/status
type ScanStatusResponse struct {
	Scanning       bool   `json:"scanning"`
	Progress       string `json:"progress"`
	FilesProcessed int    `json:"filesProcessed"`
}

// FastScanResult holds the result of a fast scan operation
type FastScanResult struct {
	Unchanged    int `json:"unchanged"`    // Files that exist and haven't changed
	Modified     int `json:"modified"`     // Files that were modified (size changed)
	Created      int `json:"created"`      // New files added
	Deleted      int `json:"deleted"`      // Records removed from DB (files no longer exist)
	TotalChecked int `json:"totalChecked"` // Total files checked (modified + created)
}

// FileEventType represents the type of file event detected during scanning
type FileEventType int

const (
	// FileCreated indicates a new file was added to the database
	FileCreated FileEventType = iota
	// FileModified indicates an existing file was updated
	FileModified
	// FileDeleted indicates a file was removed from the database
	FileDeleted
)

// FileEvent represents a file change event detected during scanning
type FileEvent struct {
	Type           FileEventType
	ImageFileID    uint
	Path           string
	ContentChanged bool // true if hash differs (not just modTime)
}

// ScanManager manages asynchronous directory scanning
type ScanManager struct {
	mu              sync.RWMutex
	isScanning      bool
	progress        string
	filesProcessed  int
	db              *gorm.DB
	scanWorkers     int
	OnScanComplete  func()          // called after each scan finishes (if non-nil)
	OnFileProcessed func(FileEvent) // called per-file after scan (async, if non-nil)
	postProcessSem  chan struct{}   // semaphore bounding post-processing goroutines
}

// NewScanManager creates a new ScanManager
func NewScanManager(db *gorm.DB, scanWorkers int) *ScanManager {
	workers := scanWorkers
	if workers <= 0 {
		workers = 1
	}
	return &ScanManager{
		db:             db,
		scanWorkers:    scanWorkers,
		postProcessSem: make(chan struct{}, workers),
	}
}

// getGalleryDirs reads current gallery folder paths from the database
func (sm *ScanManager) getGalleryDirs() []string {
	var folders []domain.GalleryFolder
	sm.db.Find(&folders)
	dirs := make([]string, len(folders))
	for i, f := range folders {
		dirs[i] = f.Path
	}
	return dirs
}

// StartScan launches an asynchronous scan of all gallery directories
func (sm *ScanManager) StartScan() error {
	sm.mu.Lock()
	if sm.isScanning {
		sm.mu.Unlock()
		return fmt.Errorf("scan already in progress")
	}
	sm.isScanning = true
	sm.progress = "Starting scan..."
	sm.filesProcessed = 0
	sm.mu.Unlock()

	go func() {
		progressChan := make(chan string, 200)

		go func() {
			count := 0
			for msg := range progressChan {
				count++
				sm.mu.Lock()
				sm.progress = msg
				sm.filesProcessed = count
				sm.mu.Unlock()
			}
		}()

		// Cleanup missing files first
		sm.mu.Lock()
		sm.progress = "Cleaning up missing files..."
		sm.mu.Unlock()
		cleanupBatch, _ := cleanupMissingFiles(sm.db, progressChan)
		sm.processBatchResult(cleanupBatch)

		// Read gallery dirs from DB at scan time
		scanDirs := sm.getGalleryDirs()

		// Scan all directories
		for _, dir := range scanDirs {
			sm.mu.Lock()
			sm.progress = fmt.Sprintf("Scanning: %s", dir)
			sm.mu.Unlock()
			batch, _ := scanDirectory(sm.db, dir, progressChan, sm.scanWorkers)
			sm.processBatchResult(batch)
		}

		close(progressChan)

		sm.mu.Lock()
		sm.isScanning = false
		sm.progress = "Scan complete"
		sm.mu.Unlock()

		if sm.OnScanComplete != nil {
			sm.OnScanComplete()
		}
	}()

	return nil
}

// ScanSingleDir launches an asynchronous scan of a single directory
func (sm *ScanManager) ScanSingleDir(dirPath string) error {
	sm.mu.Lock()
	if sm.isScanning {
		sm.mu.Unlock()
		return fmt.Errorf("scan already in progress")
	}
	sm.isScanning = true
	sm.progress = fmt.Sprintf("Scanning: %s", dirPath)
	sm.filesProcessed = 0
	sm.mu.Unlock()

	go func() {
		progressChan := make(chan string, 200)

		go func() {
			count := 0
			for msg := range progressChan {
				count++
				sm.mu.Lock()
				sm.progress = msg
				sm.filesProcessed = count
				sm.mu.Unlock()
			}
		}()

		batch, _ := scanDirectory(sm.db, dirPath, progressChan, sm.scanWorkers)
		sm.processBatchResult(batch)

		close(progressChan)

		sm.mu.Lock()
		sm.isScanning = false
		sm.progress = "Scan complete"
		sm.mu.Unlock()

		if sm.OnScanComplete != nil {
			sm.OnScanComplete()
		}
	}()

	return nil
}

// FastScanGallery launches an asynchronous fast scan of all gallery directories
// Only hashes files when record doesn't exist or size differs
// Returns result with scan statistics
func (sm *ScanManager) FastScanGallery() FastScanResult {
	sm.mu.Lock()
	if sm.isScanning {
		sm.mu.Unlock()
		return FastScanResult{}
	}
	sm.isScanning = true
	sm.progress = "Starting fast scan..."
	sm.filesProcessed = 0
	sm.mu.Unlock()

	totalStats := FastScanResult{}

	go func() {
		progressChan := make(chan string, 200)

		go func() {
			count := 0
			for msg := range progressChan {
				count++
				sm.mu.Lock()
				sm.progress = msg
				sm.filesProcessed = count
				sm.mu.Unlock()
			}
		}()

		// Cleanup missing files first
		sm.mu.Lock()
		sm.progress = "Cleaning up missing files..."
		sm.mu.Unlock()
		cleanupBatch, _ := cleanupMissingFiles(sm.db, progressChan)
		sm.processBatchResult(cleanupBatch)

		// Read gallery dirs from DB at scan time
		scanDirs := sm.getGalleryDirs()

		// Fast scan all directories
		for _, dir := range scanDirs {
			sm.mu.Lock()
			sm.progress = fmt.Sprintf("Fast scanning: %s", dir)
			sm.mu.Unlock()
			stats, batch := fastScanGalleryDirectory(sm.db, dir, progressChan, sm.scanWorkers)
			sm.processBatchResult(batch)
			totalStats.Unchanged += stats.Unchanged
			totalStats.Modified += stats.Modified
			totalStats.Created += stats.Created
			totalStats.Deleted += stats.Deleted
			totalStats.TotalChecked += stats.TotalChecked
		}

		close(progressChan)

		sm.mu.Lock()
		sm.isScanning = false
		sm.progress = "Fast scan complete"
		sm.mu.Unlock()

		if sm.OnScanComplete != nil {
			sm.OnScanComplete()
		}
	}()

	return totalStats
}

// FastScanSingleDir launches an asynchronous fast scan of a single directory
// Only hashes files when record doesn't exist or size differs
// Returns result with scan statistics
func (sm *ScanManager) FastScanSingleDir(dirPath string) FastScanResult {
	sm.mu.Lock()
	if sm.isScanning {
		sm.mu.Unlock()
		return FastScanResult{}
	}
	sm.isScanning = true
	sm.progress = fmt.Sprintf("Fast scanning: %s", dirPath)
	sm.filesProcessed = 0
	sm.mu.Unlock()

	stats := FastScanResult{}

	go func() {
		progressChan := make(chan string, 200)

		go func() {
			count := 0
			for msg := range progressChan {
				count++
				sm.mu.Lock()
				sm.progress = msg
				sm.filesProcessed = count
				sm.mu.Unlock()
			}
		}()

		result, batch := fastScanGalleryDirectory(sm.db, dirPath, progressChan, sm.scanWorkers)
		stats = result
		sm.processBatchResult(batch)

		close(progressChan)

		sm.mu.Lock()
		sm.isScanning = false
		sm.progress = "Fast scan complete"
		sm.mu.Unlock()

		if sm.OnScanComplete != nil {
			sm.OnScanComplete()
		}
	}()

	return stats
}

// GetStatus returns the current scan status
func (sm *ScanManager) GetStatus() ScanStatusResponse {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return ScanStatusResponse{
		Scanning:       sm.isScanning,
		Progress:       sm.progress,
		FilesProcessed: sm.filesProcessed,
	}
}

// IsScanning returns whether a scan is currently running
func (sm *ScanManager) IsScanning() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isScanning
}

// processBatchResult emits OnFileProcessed events for each file in the batch result.
// Events are dispatched asynchronously with bounded concurrency via postProcessSem.
func (sm *ScanManager) processBatchResult(batch ScanBatchResult) {
	if sm.OnFileProcessed == nil {
		return
	}
	for _, f := range batch.Created {
		sm.emitEvent(FileEvent{Type: FileCreated, ImageFileID: f.ID, Path: f.Path})
	}
	for _, u := range batch.Updated {
		sm.emitEvent(FileEvent{Type: FileModified, ImageFileID: u.File.ID, Path: u.File.Path, ContentChanged: u.ContentChanged})
	}
	for _, f := range batch.Deleted {
		sm.emitEvent(FileEvent{Type: FileDeleted, ImageFileID: f.ID, Path: f.Path})
	}
}

// emitEvent fires a single FileEvent asynchronously with bounded concurrency.
func (sm *ScanManager) emitEvent(event FileEvent) {
	if sm.postProcessSem != nil {
		sm.postProcessSem <- struct{}{}
	}
	go func() {
		defer func() {
			if sm.postProcessSem != nil {
				<-sm.postProcessSem
			}
		}()
		sm.OnFileProcessed(event)
	}()
}
