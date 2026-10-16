package imaging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/flashbacks/core/internal/domain"

	"gorm.io/gorm"
)

// Scan phase values reported by GET /api/status.
const (
	// ScanPhaseCleanup indicates missing files are being removed from the DB.
	ScanPhaseCleanup = "cleanup"
	// ScanPhaseScanning indicates gallery directories are being scanned.
	ScanPhaseScanning = "scanning"
	// ScanPhaseComplete indicates the scan has finished.
	ScanPhaseComplete = "complete"
)

// ScanStatusResponse is the JSON response for GET /api/status.
//
// Phase is a machine value (cleanup|scanning|complete) the frontend localizes;
// Progress is a human-readable English string kept for compatibility. The
// counters report real files (not progress messages): TotalFiles is the number
// of image files discovered by a metadata-only pre-count, FilesProcessed counts
// files examined (hashed or skipped as unchanged), and FilesAdded/FilesDeleted/
// FilesUpdated count records created, deleted, and updated during the scan.
type ScanStatusResponse struct {
	Scanning         bool   `json:"scanning"`
	Phase            string `json:"phase"`
	Progress         string `json:"progress"`
	CurrentDirectory string `json:"currentDirectory"`
	TotalFiles       int    `json:"totalFiles"`
	FilesProcessed   int    `json:"filesProcessed"`
	FilesAdded       int    `json:"filesAdded"`
	FilesDeleted     int    `json:"filesDeleted"`
	FilesUpdated     int    `json:"filesUpdated"`
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
	mu               sync.RWMutex
	isScanning       bool
	phase            string
	progress         string
	currentDirectory string
	totalFiles       int
	filesProcessed   int
	filesAdded       int
	filesDeleted     int
	filesUpdated     int
	db               *gorm.DB
	scanWorkers      int
	OnScanComplete   func()          // called after each scan finishes (if non-nil)
	OnFileProcessed  func(FileEvent) // called per-file after scan (async, if non-nil)
	postProcessSem   chan struct{}   // semaphore bounding post-processing goroutines
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

// beginScan resets scan state for a new scan. The caller MUST hold sm.mu.
func (sm *ScanManager) beginScan(phase, progress, dir string) {
	sm.isScanning = true
	sm.phase = phase
	sm.progress = progress
	sm.currentDirectory = dir
	sm.totalFiles = 0
	sm.filesProcessed = 0
	sm.filesAdded = 0
	sm.filesDeleted = 0
	sm.filesUpdated = 0
}

// startProgressReader drains scan progress messages, updating the human-readable
// progress string and the real files-processed counter. The returned function
// blocks until the reader goroutine has drained the channel, so callers can read
// final counters after it returns.
func (sm *ScanManager) startProgressReader(progressChan <-chan string) func() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range progressChan {
			sm.mu.Lock()
			sm.progress = msg
			if isFileScannedMessage(msg) {
				sm.filesProcessed++
			}
			sm.mu.Unlock()
		}
	}()
	return wg.Wait
}

// isFileScannedMessage reports whether a progress message represents an image
// file that the scanner examined (hashed or skipped as unchanged). Directory
// headers and error lines are not counted, so FilesProcessed reflects files
// rather than the number of progress messages.
func isFileScannedMessage(msg string) bool {
	return strings.HasPrefix(msg, "Processed: ") ||
		strings.HasPrefix(msg, "Skipping (cached): ") ||
		strings.HasPrefix(msg, "Skipped (unchanged): ")
}

// countImageFiles returns the number of image files under dir using a
// metadata-only walk (no hashing), used to report the total for a scan.
func countImageFiles(dir string) int {
	count := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if domain.IsImageFile(path) {
			count++
		}
		return nil
	})
	return count
}

// countImageFilesInDirs returns the summed image-file count across dirs.
func countImageFilesInDirs(dirs []string) int {
	total := 0
	for _, dir := range dirs {
		total += countImageFiles(dir)
	}
	return total
}

// StartScan launches an asynchronous scan of all gallery directories
func (sm *ScanManager) StartScan() error {
	sm.mu.Lock()
	if sm.isScanning {
		sm.mu.Unlock()
		return fmt.Errorf("scan already in progress")
	}
	sm.beginScan(ScanPhaseCleanup, "Cleaning up missing files...", "")
	sm.mu.Unlock()

	go func() {
		progressChan := make(chan string, 200)
		waitReader := sm.startProgressReader(progressChan)

		// Cleanup missing files first
		cleanupBatch, _ := cleanupMissingFiles(sm.db, progressChan)
		sm.processBatchResult(cleanupBatch)

		// Read gallery dirs from DB at scan time
		scanDirs := sm.getGalleryDirs()

		// Pre-count the total number of image files so the caller can show
		// "scanned X of Y" while the scan runs.
		sm.mu.Lock()
		sm.totalFiles = countImageFilesInDirs(scanDirs)
		sm.phase = ScanPhaseScanning
		sm.mu.Unlock()

		// Scan all directories
		for _, dir := range scanDirs {
			sm.mu.Lock()
			sm.currentDirectory = dir
			sm.progress = fmt.Sprintf("Scanning: %s", dir)
			sm.mu.Unlock()
			batch, _ := scanDirectory(sm.db, dir, progressChan, sm.scanWorkers)
			sm.processBatchResult(batch)
		}

		close(progressChan)
		waitReader()

		sm.mu.Lock()
		sm.isScanning = false
		sm.phase = ScanPhaseComplete
		sm.progress = "Scan complete"
		sm.currentDirectory = ""
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
	sm.beginScan(ScanPhaseScanning, fmt.Sprintf("Scanning: %s", dirPath), dirPath)
	sm.mu.Unlock()

	go func() {
		progressChan := make(chan string, 200)
		waitReader := sm.startProgressReader(progressChan)

		sm.mu.Lock()
		sm.totalFiles = countImageFiles(dirPath)
		sm.mu.Unlock()

		batch, _ := scanDirectory(sm.db, dirPath, progressChan, sm.scanWorkers)
		sm.processBatchResult(batch)

		close(progressChan)
		waitReader()

		sm.mu.Lock()
		sm.isScanning = false
		sm.phase = ScanPhaseComplete
		sm.progress = "Scan complete"
		sm.currentDirectory = ""
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
	sm.beginScan(ScanPhaseCleanup, "Cleaning up missing files...", "")
	sm.mu.Unlock()

	totalStats := FastScanResult{}

	go func() {
		progressChan := make(chan string, 200)
		waitReader := sm.startProgressReader(progressChan)

		// Cleanup missing files first
		cleanupBatch, _ := cleanupMissingFiles(sm.db, progressChan)
		sm.processBatchResult(cleanupBatch)

		// Read gallery dirs from DB at scan time
		scanDirs := sm.getGalleryDirs()

		sm.mu.Lock()
		sm.totalFiles = countImageFilesInDirs(scanDirs)
		sm.phase = ScanPhaseScanning
		sm.mu.Unlock()

		// Fast scan all directories
		for _, dir := range scanDirs {
			sm.mu.Lock()
			sm.currentDirectory = dir
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
		waitReader()

		sm.mu.Lock()
		sm.isScanning = false
		sm.phase = ScanPhaseComplete
		sm.progress = "Fast scan complete"
		sm.currentDirectory = ""
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
	sm.beginScan(ScanPhaseScanning, fmt.Sprintf("Fast scanning: %s", dirPath), dirPath)
	sm.mu.Unlock()

	stats := FastScanResult{}

	go func() {
		progressChan := make(chan string, 200)
		waitReader := sm.startProgressReader(progressChan)

		sm.mu.Lock()
		sm.totalFiles = countImageFiles(dirPath)
		sm.mu.Unlock()

		result, batch := fastScanGalleryDirectory(sm.db, dirPath, progressChan, sm.scanWorkers)
		stats = result
		sm.processBatchResult(batch)

		close(progressChan)
		waitReader()

		sm.mu.Lock()
		sm.isScanning = false
		sm.phase = ScanPhaseComplete
		sm.progress = "Fast scan complete"
		sm.currentDirectory = ""
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
		Scanning:         sm.isScanning,
		Phase:            sm.phase,
		Progress:         sm.progress,
		CurrentDirectory: sm.currentDirectory,
		TotalFiles:       sm.totalFiles,
		FilesProcessed:   sm.filesProcessed,
		FilesAdded:       sm.filesAdded,
		FilesDeleted:     sm.filesDeleted,
		FilesUpdated:     sm.filesUpdated,
	}
}

// IsScanning returns whether a scan is currently running
func (sm *ScanManager) IsScanning() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isScanning
}

// processBatchResult accumulates the real per-scan counters and emits
// OnFileProcessed events for each file in the batch result. Events are dispatched
// asynchronously with bounded concurrency via postProcessSem.
func (sm *ScanManager) processBatchResult(batch ScanBatchResult) {
	sm.mu.Lock()
	sm.filesAdded += len(batch.Created)
	sm.filesUpdated += len(batch.Updated)
	sm.filesDeleted += len(batch.Deleted)
	sm.mu.Unlock()

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
