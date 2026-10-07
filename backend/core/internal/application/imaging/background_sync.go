package imaging

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/flashbacks/api-service/internal/application/thumbnail"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/geocoder"

	"gorm.io/gorm"
)

// SyncStatus holds the current status of the background sync manager.
type SyncStatus struct {
	Running            bool       `json:"running"`
	SyncInProgress     bool       `json:"syncInProgress"`
	NextRunAt          *time.Time `json:"nextRunAt,omitempty"`
	LastSyncAt         *time.Time `json:"lastSyncAt,omitempty"`
	LastSyncNew        int        `json:"lastSyncNew"`
	LastSyncUpdated    int        `json:"lastSyncUpdated"`
	LastSyncDeleted    int        `json:"lastSyncDeleted"`
	LastSyncThumbnails int        `json:"lastSyncThumbnails"`
	ProcessedFiles     int        `json:"processedFiles"`
	TotalFiles         int        `json:"totalFiles"`
}

// BackgroundSyncManager manages background synchronization of gallery files
// and thumbnail cache. It runs on configured weekdays at a configured time (in the user's timezone).
type BackgroundSyncManager struct {
	mu                 sync.Mutex
	running            bool
	stopCh             chan struct{}
	scheduleCh         chan struct{} // Signal to restart the schedule loop
	db                 *gorm.DB
	thumbnailService   thumbnail.ThumbnailProvider
	geolocationService *geocoder.GeolocationService
	exifClient         ExifClient
	syncDays           []time.Weekday
	hour               int
	minute             int
	timezoneOffset     int // minutes from UTC (JS getTimezoneOffset sign: UTC+3 = -180)
	// Status tracking
	nextRunAt          *time.Time
	lastSyncAt         *time.Time
	lastSyncNew        int
	lastSyncUpdated    int
	lastSyncDeleted    int
	lastSyncThumbnails int
	syncInProgress     bool
	processedFiles     int
	totalFiles         int
}

// NewBackgroundSyncManager creates a new background sync manager
func NewBackgroundSyncManager(db *gorm.DB, thumbnailService thumbnail.ThumbnailProvider, geoService *geocoder.GeolocationService, exifClient ExifClient) *BackgroundSyncManager {
	return &BackgroundSyncManager{
		db:                 db,
		thumbnailService:   thumbnailService,
		geolocationService: geoService,
		exifClient:         exifClient,
		syncDays:           []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
		hour:               3,
		minute:             30,
		timezoneOffset:     0,
		stopCh:             make(chan struct{}),
		scheduleCh:         make(chan struct{}),
	}
}

// Start begins the background synchronization loop with the given schedule
func (bsm *BackgroundSyncManager) Start(syncDays []time.Weekday, hour int, minute int, timezoneOffset int) {
	bsm.mu.Lock()
	if bsm.running {
		bsm.mu.Unlock()
		log.Println("Background sync already running")
		return
	}
	bsm.running = true
	bsm.syncDays = syncDays
	bsm.hour = hour
	bsm.minute = minute
	bsm.timezoneOffset = timezoneOffset
	bsm.stopCh = make(chan struct{})
	bsm.scheduleCh = make(chan struct{})

	// Load latest sync stats from sync_history so they survive restarts
	var lastSync domain.SyncHistory
	if result := bsm.db.Order("created_at DESC").First(&lastSync); result.Error == nil {
		bsm.lastSyncAt = &lastSync.CreatedAt
		bsm.lastSyncNew = lastSync.NewFiles
		bsm.lastSyncUpdated = lastSync.UpdatedFiles
		bsm.lastSyncDeleted = lastSync.DeletedFiles
		bsm.lastSyncThumbnails = lastSync.ThumbnailsGenerated
	}
	bsm.mu.Unlock()

	log.Printf("Starting background gallery sync (days=%v at %02d:%02d, tzOffset=%d min)", syncDays, hour, minute, timezoneOffset)
	go bsm.scheduleLoop()
}

// Stop stops the background synchronization
func (bsm *BackgroundSyncManager) Stop() {
	bsm.mu.Lock()
	if !bsm.running {
		bsm.mu.Unlock()
		return
	}
	bsm.running = false
	close(bsm.stopCh)
	bsm.mu.Unlock()

	log.Println("Background gallery sync stopped")
}

// UpdateSchedule updates the schedule at runtime and restarts the loop
func (bsm *BackgroundSyncManager) UpdateSchedule(syncDays []time.Weekday, hour int, minute int, timezoneOffset int) {
	bsm.mu.Lock()
	wasRunning := bsm.running
	bsm.syncDays = syncDays
	bsm.hour = hour
	bsm.minute = minute
	bsm.timezoneOffset = timezoneOffset
	bsm.mu.Unlock()

	log.Printf("Background sync schedule updated: days=%v at %02d:%02d, tzOffset=%d min", syncDays, hour, minute, timezoneOffset)

	// Signal the schedule loop to restart with new settings
	if wasRunning {
		select {
		case bsm.scheduleCh <- struct{}{}:
		default:
		}
	}
}

// GetStatus returns the current sync status.
func (bsm *BackgroundSyncManager) GetStatus() SyncStatus {
	bsm.mu.Lock()
	defer bsm.mu.Unlock()
	return SyncStatus{
		Running:            bsm.running,
		SyncInProgress:     bsm.syncInProgress,
		NextRunAt:          bsm.nextRunAt,
		LastSyncAt:         bsm.lastSyncAt,
		LastSyncNew:        bsm.lastSyncNew,
		LastSyncUpdated:    bsm.lastSyncUpdated,
		LastSyncDeleted:    bsm.lastSyncDeleted,
		LastSyncThumbnails: bsm.lastSyncThumbnails,
		ProcessedFiles:     bsm.processedFiles,
		TotalFiles:         bsm.totalFiles,
	}
}

// scheduleLoop runs the synchronization on configured weekdays at the configured time
func (bsm *BackgroundSyncManager) scheduleLoop() {
	for {
		bsm.mu.Lock()
		syncDays := bsm.syncDays
		hour := bsm.hour
		minute := bsm.minute
		timezoneOffset := bsm.timezoneOffset
		stopCh := bsm.stopCh
		bsm.mu.Unlock()

		if len(syncDays) > 0 {
			// Calculate next run time
			nextRun := bsm.calculateNextRunTime(syncDays, hour, minute, timezoneOffset)

			bsm.mu.Lock()
			bsm.nextRunAt = &nextRun
			bsm.mu.Unlock()

			userTZ := time.FixedZone("UserTZ", -timezoneOffset*60)
			log.Printf("Background sync: next run at %s (user local: %s)", nextRun.Format("2006-01-02 15:04:05"), nextRun.In(userTZ).Format("Mon 15:04"))

			select {
			case <-time.After(time.Until(nextRun)):
				// Time to run sync
				bsm.syncOnce()
			case <-stopCh:
				return
			case <-bsm.scheduleCh:
				// Schedule updated, restart the loop
				continue
			}
		} else {
			log.Println("Background sync: no days selected, waiting for schedule change or stop")

			bsm.mu.Lock()
			bsm.nextRunAt = nil
			bsm.mu.Unlock()

			select {
			case <-stopCh:
				return
			case <-bsm.scheduleCh:
				// Schedule updated, restart the loop
				continue
			}
		}
	}
}

// calculateNextRunTime calculates the next time the sync should run based on the user's timezone.
func (bsm *BackgroundSyncManager) calculateNextRunTime(syncDays []time.Weekday, hour, minute, timezoneOffset int) time.Time {
	userTZ := time.FixedZone("UserTZ", -timezoneOffset*60)
	now := time.Now().In(userTZ)

	daySet := make(map[time.Weekday]bool)
	for _, d := range syncDays {
		daySet[d] = true
	}

	// Check today first: if it's a sync day and the scheduled time hasn't passed.
	// Compare in UTC to avoid timezone-related comparison issues when server and user
	// share the same timezone.
	if daySet[now.Weekday()] {
		candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, userTZ)
		if candidate.UTC().After(now.UTC()) {
			return candidate
		}
	}

	// Check the next 7 days
	for i := 1; i <= 7; i++ {
		future := now.AddDate(0, 0, i)
		if daySet[future.Weekday()] {
			return time.Date(future.Year(), future.Month(), future.Day(), hour, minute, 0, 0, userTZ)
		}
	}

	// Fallback: should not happen if syncDays is non-empty
	return now.Add(24 * time.Hour)
}

// syncOnce performs a single synchronization pass
func (bsm *BackgroundSyncManager) syncOnce() {
	log.Println("Background sync: starting gallery synchronization")

	// Mark sync as in-progress
	bsm.mu.Lock()
	bsm.syncInProgress = true
	bsm.processedFiles = 0
	bsm.totalFiles = 0
	bsm.mu.Unlock()

	defer func() {
		bsm.mu.Lock()
		bsm.syncInProgress = false
		bsm.mu.Unlock()
	}()

	// Get all gallery folders
	var folders []domain.GalleryFolder
	if err := bsm.db.Find(&folders).Error; err != nil {
		log.Printf("Background sync: failed to get gallery folders: %v", err)
		return
	}

	if len(folders) == 0 {
		log.Println("Background sync: no gallery folders configured")
		return
	}

	// Check if thumbnail service is available
	thumbnailEnabled := bsm.thumbnailService != nil && bsm.thumbnailService.IsEnabled()

	newFiles := 0
	updatedFiles := 0
	deletedFiles := 0
	thumbnailGenerated := 0
	totalDiskFiles := 0

	for _, folder := range folders {
		absPath, err := filepath.Abs(folder.Path)
		if err != nil {
			log.Printf("Background sync: failed to get absolute path for %s: %v", folder.Path, err)
			continue
		}

		folderNew, folderUpdated, folderDeleted, folderThumbGenerated, folderTotal := bsm.syncFolder(absPath, thumbnailEnabled)
		newFiles += folderNew
		updatedFiles += folderUpdated
		deletedFiles += folderDeleted
		thumbnailGenerated += folderThumbGenerated
		totalDiskFiles += folderTotal
	}

	// Clean up records for files that no longer exist
	deletedFromCleanup, cleanupTotal := bsm.cleanupMissingFiles()
	deletedFiles += deletedFromCleanup

	// Update total to include cleanup phase
	bsm.mu.Lock()
	bsm.totalFiles = totalDiskFiles + cleanupTotal
	bsm.mu.Unlock()

	log.Printf("Background sync: complete - %d new, %d updated, %d deleted, %d thumbnails generated",
		newFiles, updatedFiles, deletedFiles, thumbnailGenerated)

	// Update status tracking and persist to DB
	now := time.Now()
	bsm.mu.Lock()
	bsm.lastSyncAt = &now
	bsm.lastSyncNew = newFiles
	bsm.lastSyncUpdated = updatedFiles
	bsm.lastSyncDeleted = deletedFiles
	bsm.lastSyncThumbnails = thumbnailGenerated
	bsm.mu.Unlock()

	// Persist last sync stats to sync_history
	bsm.db.Create(&domain.SyncHistory{
		CreatedAt:           now,
		NewFiles:            newFiles,
		UpdatedFiles:        updatedFiles,
		DeletedFiles:        deletedFiles,
		ThumbnailsGenerated: thumbnailGenerated,
	})
}

// syncFolder synchronizes a single folder sequentially
func (bsm *BackgroundSyncManager) syncFolder(folderPath string, thumbnailEnabled bool) (newCount, updatedCount, deletedCount, thumbCount, totalDiskFiles int) {
	log.Printf("Background sync: scanning folder %s", folderPath)

	// Collect all image files from disk
	diskFiles := make(map[string]os.FileInfo)
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Background sync: error accessing %s: %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !domain.IsImageFile(path) {
			return nil
		}
		normalizedPath := filepath.ToSlash(path)
		diskFiles[normalizedPath] = info
		return nil
	})

	if err != nil {
		log.Printf("Background sync: failed to walk folder %s: %v", folderPath, err)
		return
	}

	totalDiskFiles = len(diskFiles)

	// Get all existing DB records for this folder
	var dbFiles []domain.ImageFile
	prefix := folderPath + "/"
	if err := bsm.db.Where("path LIKE ?", prefix+"%").Find(&dbFiles).Error; err != nil {
		log.Printf("Background sync: failed to query DB for folder %s: %v", folderPath, err)
		return
	}

	// Build a map of DB files
	dbFileMap := make(map[string]domain.ImageFile)
	for _, dbFile := range dbFiles {
		dbFileMap[dbFile.Path] = dbFile
	}

	// Process each file on disk sequentially
	for diskPath, diskInfo := range diskFiles {
		// Check if we should stop
		if !bsm.isRunning() {
			log.Println("Background sync: stopped during folder scan")
			return
		}

		dbFile, existsInDB := dbFileMap[diskPath]

		if !existsInDB {
			// New file - add to DB
			hash, err := calculateFileHash(diskPath)
			if err != nil {
				log.Printf("Background sync: failed to hash new file %s: %v", diskPath, err)
				bsm.incrementProcessed()
				continue
			}

			newFile := domain.ImageFile{
				Path:    diskPath,
				Size:    diskInfo.Size(),
				Hash:    hash,
				ModTime: diskInfo.ModTime(),
			}

			if err := bsm.db.Create(&newFile).Error; err != nil {
				log.Printf("Background sync: failed to create record for %s: %v", diskPath, err)
				bsm.incrementProcessed()
				continue
			}

			newCount++
			log.Printf("Background sync: added new file %s", diskPath)

			// Extract EXIF/geo metadata for new file
			bsm.ExtractAndSaveMetadata(diskPath, newFile.ID)

			// Generate thumbnail for new file
			if thumbnailEnabled {
				if bsm.ensureThumbnail(diskPath) {
					thumbCount++
				}
			}
		} else {
			// File exists in DB — two-step change detection:
			// 1. Compare size + modTime (truncated to seconds). If both match → unchanged.
			// 2. Otherwise compute MD5 hash. If hash matches stored → modTime-only bump.
			//    If hash differs → real content change.
			sizeChanged := dbFile.Size != diskInfo.Size()
			dbModTime := dbFile.ModTime.Truncate(time.Second)
			diskModTime := diskInfo.ModTime().Truncate(time.Second)
			modTimeChanged := !dbModTime.Equal(diskModTime)

			if sizeChanged || modTimeChanged {
				hash, err := calculateFileHash(diskPath)
				if err != nil {
					log.Printf("Background sync: failed to hash file %s: %v", diskPath, err)
					bsm.incrementProcessed()
					continue
				}

				hashChanged := dbFile.Hash != hash
				contentChanged := sizeChanged || hashChanged
				dbFile.Size = diskInfo.Size()
				dbFile.Hash = hash
				dbFile.ModTime = diskInfo.ModTime()

				if err := bsm.db.Save(&dbFile).Error; err != nil {
					log.Printf("Background sync: failed to update record for %s: %v", diskPath, err)
					bsm.incrementProcessed()
					continue
				}

				if contentChanged {
					updatedCount++
					log.Printf("Background sync: updated file %s (size:%v, hash:%v)", diskPath, sizeChanged, hashChanged)

					// Re-extract EXIF/geo metadata only if file content actually changed
					bsm.ExtractAndSaveMetadata(diskPath, dbFile.ID)
					// Invalidate OCR classification only if file content actually changed
					bsm.InvalidateOCRClassification(dbFile.ID)
					// Invalidate AI tags and embeddings only if file content actually changed
					bsm.InvalidateTagsAndEmbeddings(dbFile.ID)
					// Regenerate thumbnail for modified file (invalidate old one)
					if thumbnailEnabled {
						bsm.thumbnailService.Invalidate(diskPath)
						if bsm.ensureThumbnail(diskPath) {
							thumbCount++
						}
					}
				} else {
					log.Printf("Background sync: modTime only changed, content unchanged: %s", diskPath)
				}
			} else {
				// File unchanged - ensure thumbnail exists
				if thumbnailEnabled && !bsm.thumbnailService.HasThumbnail(diskPath) {
					if bsm.ensureThumbnail(diskPath) {
						thumbCount++
					}
				}
			}
		}

		bsm.incrementProcessed()
	}

	return
}

// ensureThumbnail generates a thumbnail for a file if it doesn't exist
func (bsm *BackgroundSyncManager) ensureThumbnail(filePath string) bool {
	// Check if file still exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	// Check if thumbnail already exists
	if bsm.thumbnailService.HasThumbnail(filePath) {
		return false
	}

	// Generate thumbnail
	_, err := bsm.thumbnailService.GetOrGenerate(filePath)
	if err != nil {
		log.Printf("Background sync: failed to generate thumbnail for %s: %v", filePath, err)
		return false
	}

	log.Printf("Background sync: generated thumbnail for %s", filePath)
	return true
}

// cleanupMissingFiles removes DB records for files that no longer exist on disk.
// Returns the count of deleted records and the total number of DB records checked.
func (bsm *BackgroundSyncManager) cleanupMissingFiles() (deletedCount int, totalChecked int) {
	var files []domain.ImageFile
	if err := bsm.db.Find(&files).Error; err != nil {
		log.Printf("Background sync: failed to query all files for cleanup: %v", err)
		return 0, 0
	}

	totalChecked = len(files)

	for _, file := range files {
		if !bsm.isRunning() {
			log.Println("Background sync: stopped during cleanup")
			break
		}

		if _, err := os.Stat(file.Path); os.IsNotExist(err) {
			deleteImageFileCascade(bsm.db, file.ID)

			// Clean up thumbnail if it exists
			if bsm.thumbnailService != nil {
				bsm.thumbnailService.Invalidate(file.Path)
			}

			deletedCount++
			log.Printf("Background sync: removed missing file record %s", file.Path)
		}

		bsm.incrementProcessed()
	}

	return
}

// isRunning checks if the sync manager is currently running
func (bsm *BackgroundSyncManager) isRunning() bool {
	bsm.mu.Lock()
	defer bsm.mu.Unlock()
	return bsm.running
}

// incrementProcessed atomically increments the processed file counter.
func (bsm *BackgroundSyncManager) incrementProcessed() {
	bsm.mu.Lock()
	bsm.processedFiles++
	bsm.mu.Unlock()
}

// ExtractAndSaveMetadata extracts EXIF and geo metadata for a file and saves to the database.
func (bsm *BackgroundSyncManager) ExtractAndSaveMetadata(filePath string, imageFileID uint) {
	if bsm.exifClient == nil {
		return
	}
	ctx := context.Background()

	meta, err := bsm.exifClient.ExtractMetadata(ctx, filePath)
	if err != nil {
		log.Printf("Background sync: failed to extract metadata for %s: %v", filePath, err)
		return
	}

	meta.ImageFileID = imageFileID

	// Resolve geolocation via GeolocationService if GPS coordinates are present.
	if bsm.geolocationService != nil {
		lat, lng, hasGPS, _ := bsm.exifClient.ExtractGPS(ctx, filePath)
		if hasGPS {
			geoEntry, err := bsm.geolocationService.ResolveGeolocation(lat, lng)
			if err != nil {
				log.Printf("Background sync: failed to resolve geolocation for %s: %v", filePath, err)
			} else {
				meta.GeolocationRef = &geoEntry.ID
			}
		}
	}

	// Upsert: insert or update
	if err := bsm.db.Where("image_file_id = ?", imageFileID).Assign(meta).FirstOrCreate(&domain.ImageMetadata{}).Error; err != nil {
		log.Printf("Background sync: failed to save metadata for %s: %v", filePath, err)
	} else {
		log.Printf("Background sync: saved EXIF/geo metadata for %s", filePath)
	}
}

// InvalidateOCRClassification deletes existing OCR classification for a file
// so it gets re-classified on the next OCR pass.
func (bsm *BackgroundSyncManager) InvalidateOCRClassification(imageFileID uint) {
	// Delete bounding boxes first (foreign key dependency)
	bsm.db.Where("classification_id IN (SELECT id FROM ocr_classifications WHERE image_file_id = ?)", imageFileID).Delete(&domain.OcrBoundingBox{})
	// Delete the classification
	if result := bsm.db.Where("image_file_id = ?", imageFileID).Delete(&domain.OcrClassification{}); result.Error == nil && result.RowsAffected > 0 {
		log.Printf("Background sync: invalidated OCR classification for image %d", imageFileID)
	}
}

// ExtractAndSaveMetadataAsync extracts EXIF/geo metadata in a background goroutine.
func (bsm *BackgroundSyncManager) ExtractAndSaveMetadataAsync(filePath string, imageFileID uint) {
	go bsm.ExtractAndSaveMetadata(filePath, imageFileID)
}

// InvalidateOCRClassificationAsync invalidates OCR classification in a background goroutine.
func (bsm *BackgroundSyncManager) InvalidateOCRClassificationAsync(imageFileID uint) {
	go bsm.InvalidateOCRClassification(imageFileID)
}

// InvalidateTagsAndEmbeddingsAsync invalidates AI tags and embeddings in a background goroutine.
func (bsm *BackgroundSyncManager) InvalidateTagsAndEmbeddingsAsync(imageFileID uint) {
	go bsm.InvalidateTagsAndEmbeddings(imageFileID)
}

// invalidateTagsAndEmbeddings deletes AI-generated tags and vector embeddings for a file
// so they are re-generated on the next tag scan or embedding backfill pass.
func (bsm *BackgroundSyncManager) InvalidateTagsAndEmbeddings(imageFileID uint) {
	bsm.db.Where("image_file_id = ?", imageFileID).Delete(&domain.ImageTag{})
	bsm.db.Where("image_file_id = ?", imageFileID).Delete(&domain.ImageProcessingError{})
	if result := bsm.db.Where("image_file_id = ?", imageFileID).Delete(&domain.TagEmbedding{}); result.Error == nil && result.RowsAffected > 0 {
		log.Printf("Background sync: invalidated tags and embeddings for image %d", imageFileID)
	}
}

// ParseSyncDays converts a comma-separated string of weekday numbers to a slice of time.Weekday.
// Returns nil if the input is empty.
func ParseSyncDays(s string) []time.Weekday {
	if s == "" {
		return nil
	}
	var days []time.Weekday
	for _, c := range s {
		if c >= '0' && c <= '6' {
			days = append(days, time.Weekday(c-'0'))
		}
	}
	return days
}

// FormatSyncDays converts a slice of time.Weekday to a comma-separated string.
func FormatSyncDays(days []time.Weekday) string {
	if len(days) == 0 {
		return ""
	}
	result := ""
	for i, d := range days {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%d", d)
	}
	return result
}
