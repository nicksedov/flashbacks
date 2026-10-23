package imaging

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/flashbacks/core/internal/application/background"
	"github.com/flashbacks/core/internal/domain"
	imgutil "github.com/flashbacks/core/internal/infrastructure/imaging"
	"github.com/flashbacks/core/internal/infrastructure/llm"

	"gorm.io/gorm"
)

// TagScanProgress holds the current scanning progress
type TagScanProgress struct {
	Total        int
	Scanned      int
	Skipped      int // files permanently skipped (corrupt/unreadable)
	Remaining    int
	CurrentImage string
	LastError    string
}

// TagScanStatus holds the current status of the tag scan manager
type TagScanStatus struct {
	Running    bool
	Paused     bool
	Enabled    bool
	Schedule   string
	WindowOpen bool
	Progress   TagScanProgress
}

// actionTags is the AI action used by the tag scan background process.
const actionTags = "tags"

// TagScanManager manages background tag scanning of gallery images
type TagScanManager struct {
	*background.Manager
	mu                 sync.Mutex
	paused             bool
	scheduleCh         chan struct{}
	resumeCh           chan struct{}
	pauseDepth         int
	db                 *gorm.DB
	llmOcrService      *LlmOcrService
	schedule           domain.Schedule
	cursor             uint
	progress           TagScanProgress
	maxImageMegapixels float64
	embeddingBackfill  *EmbeddingBackfillManager
}

// NewTagScanManager creates a new tag scan manager
func NewTagScanManager(db *gorm.DB, llmOcrService *LlmOcrService, maxImageMegapixels float64) *TagScanManager {
	return &TagScanManager{
		Manager:            background.New("tag-scan"),
		db:                 db,
		llmOcrService:      llmOcrService,
		schedule:           domain.DefaultSchedule(domain.ScheduleTagScan),
		scheduleCh:         make(chan struct{}),
		resumeCh:           make(chan struct{}, 1),
		maxImageMegapixels: maxImageMegapixels,
	}
}

// SetEmbeddingBackfill sets the embedding backfill manager to trigger after tags are saved.
func (tsm *TagScanManager) SetEmbeddingBackfill(eb *EmbeddingBackfillManager) {
	tsm.mu.Lock()
	tsm.embeddingBackfill = eb
	tsm.mu.Unlock()
}

// Start begins the tag scanning loop with the given schedule
func (tsm *TagScanManager) Start(schedule domain.Schedule) {
	if !tsm.TryStart() {
		log.Println("Tag scanning already running")
		return
	}
	tsm.mu.Lock()
	tsm.schedule = schedule
	tsm.scheduleCh = make(chan struct{})
	tsm.mu.Unlock()

	log.Printf("Starting background tag scanning (days=%v, window %02d:%02d - %02d:%02d, tzOffset=%d, enabled=%v)",
		schedule.Weekdays(), schedule.StartHour, schedule.StartMinute, schedule.EndHour, schedule.EndMinute, schedule.TimezoneOffset, schedule.Enabled)
	go tsm.scheduleLoop()
}

// Stop stops the tag scanning
func (tsm *TagScanManager) Stop() {
	tsm.Manager.Stop()
	log.Println("Background tag scanning stopped")
}

// IsRunning returns whether the tag scanning is currently running
func (tsm *TagScanManager) IsRunning() bool {
	return tsm.Manager.IsRunning()
}

// UpdateSchedule updates the schedule at runtime and restarts the loop
func (tsm *TagScanManager) UpdateSchedule(schedule domain.Schedule) {
	tsm.mu.Lock()
	wasRunning := tsm.Manager.IsRunning()
	tsm.schedule = schedule
	tsm.mu.Unlock()

	log.Printf("Tag scanning schedule updated: days=%v, window %02d:%02d - %02d:%02d, tzOffset=%d, enabled=%v",
		schedule.Weekdays(), schedule.StartHour, schedule.StartMinute, schedule.EndHour, schedule.EndMinute, schedule.TimezoneOffset, schedule.Enabled)

	if wasRunning {
		select {
		case tsm.scheduleCh <- struct{}{}:
		default:
		}
	}
}

// RequestPause requests the scanner to pause (for AI task coordination)
func (tsm *TagScanManager) RequestPause() {
	tsm.mu.Lock()
	if !tsm.Manager.IsRunning() {
		tsm.mu.Unlock()
		return
	}
	tsm.pauseDepth++
	if tsm.pauseDepth > 1 {
		// Already paused, just increment counter
		tsm.mu.Unlock()
		return
	}
	tsm.paused = true
	// Drain any stale resume signal before pausing
	select {
	case <-tsm.resumeCh:
	default:
	}
	tsm.mu.Unlock()

	log.Println("Tag scan: pause requested")
}

// Resume resumes the scanner after an AI task completes
func (tsm *TagScanManager) Resume() {
	tsm.mu.Lock()
	if tsm.pauseDepth <= 0 {
		tsm.mu.Unlock()
		return
	}
	tsm.pauseDepth--
	shouldResume := tsm.pauseDepth == 0
	if shouldResume {
		tsm.paused = false
	}
	tsm.mu.Unlock()

	if shouldResume {
		select {
		case tsm.resumeCh <- struct{}{}:
		default:
		}
	}
}

// GetStatus returns the current tag scan status
func (tsm *TagScanManager) GetStatus() TagScanStatus {
	tsm.mu.Lock()
	running := tsm.Manager.IsRunning()
	paused := tsm.paused
	schedule := tsm.schedule
	windowLabel := fmt.Sprintf("%02d:%02d - %02d:%02d", schedule.StartHour, schedule.StartMinute, schedule.EndHour, schedule.EndMinute)
	progress := tsm.progress
	windowOpen := schedule.IsWindowOpenAt(time.Now())
	needsTotalQuery := running && progress.Total == 0
	tsm.mu.Unlock()

	// Query DB OUTSIDE the lock to avoid blocking Stop() and other operations
	if needsTotalQuery {
		var total int64
		tsm.db.Table("image_files").
			Joins("LEFT JOIN image_tags ON image_files.id = image_tags.image_file_id").
			Scopes(withoutPermanentErrors).
			Where("image_tags.id IS NULL").
			Count(&total)
		progress.Total = int(total)
		progress.Remaining = int(total)
	}

	return TagScanStatus{
		Running:    running,
		Paused:     paused,
		Enabled:    schedule.Enabled,
		Schedule:   windowLabel,
		WindowOpen: windowOpen,
		Progress:   progress,
	}
}

// scheduleLoop runs the tag scanning within the configured time window
func (tsm *TagScanManager) scheduleLoop() {
	log.Println("Tag scan: scheduleLoop started")
	for {
		tsm.mu.Lock()
		schedule := tsm.schedule
		stopCh := tsm.Done()
		tsm.mu.Unlock()

		if !schedule.Enabled || len(schedule.Weekdays()) == 0 {
			if !schedule.Enabled {
				log.Println("Tag scan: disabled, waiting for schedule change")
			} else {
				log.Println("Tag scan: no weekdays selected, waiting for schedule change")
			}
			// Wait for schedule change or stop
			select {
			case <-stopCh:
				return
			case <-tsm.scheduleCh:
				log.Println("Tag scan: schedule change received, re-evaluating")
				continue
			}
		}

		// If currently inside the scanning window, start scanning immediately
		inWindow := schedule.IsWindowOpenAt(time.Now())

		if inWindow {
			log.Println("Tag scan: inside window, starting scanWindow")
			tsm.scanWindow()
			log.Println("Tag scan: scanWindow completed, waiting before re-check")
			// After scanWindow completes, the window may still be open but we've
			// finished a pass. Wait briefly then re-check to avoid a tight loop
			// while still being responsive to schedule changes.
			select {
			case <-time.After(30 * time.Second):
				log.Println("Tag scan: 30s wait complete, re-evaluating")
			case <-stopCh:
				return
			case <-tsm.scheduleCh:
				log.Println("Tag scan: schedule change received during wait")
			}
			continue
		}

		// Outside the window - calculate when it next opens
		nextWindowOpen := tsm.calculateNextWindowOpen()
		log.Printf("Tag scan: outside window, next window opens at %s (server time)", nextWindowOpen.Format("15:04:05"))

		select {
		case <-time.After(time.Until(nextWindowOpen)):
			// Window opened, start scanning
			log.Println("Tag scan: window opened, starting scanWindow")
			tsm.scanWindow()
		case <-stopCh:
			return
		case <-tsm.scheduleCh:
			// Schedule updated, restart the loop
			log.Println("Tag scan: schedule change received, re-evaluating")
			continue
		}
	}
}

// calculateNextWindowOpen calculates when the scanning window next opens on a
// selected weekday. This should only be called when we are currently OUTSIDE
// the window.
func (tsm *TagScanManager) calculateNextWindowOpen() time.Time {
	tsm.mu.Lock()
	schedule := tsm.schedule
	tsm.mu.Unlock()

	return schedule.NextWindowOpenAt(time.Now())
}

// scanWindow runs scanning while within the configured time window
func (tsm *TagScanManager) scanWindow() {
	log.Println("Tag scan: scanWindow entered, counting untagged images")
	// Count total untagged images (excluding files with permanent errors)
	var total int64
	tsm.db.Table("image_files").
		Joins("LEFT JOIN image_tags ON image_files.id = image_tags.image_file_id").
		Scopes(withoutPermanentErrors).
		Where("image_tags.id IS NULL").
		Count(&total)

	if total == 0 {
		log.Println("Tag scan: all images already tagged, exiting")
		return
	}

	tsm.mu.Lock()
	tsm.progress = TagScanProgress{
		Total:     int(total),
		Scanned:   0,
		Remaining: int(total),
	}
	tsm.cursor = 0 // Reset cursor for new scan pass
	tsm.mu.Unlock()

	log.Printf("Tag scan: starting window scan, %d untagged images, cursor reset to 0", total)

	for {
		// Read the schedule under the lock, then evaluate the window outside it
		// to avoid deadlock (Go sync.Mutex is not reentrant).
		tsm.mu.Lock()
		schedule := tsm.schedule
		stopCh := tsm.Done()
		resumeCh := tsm.resumeCh
		tsm.mu.Unlock()

		if !schedule.IsWindowOpenAt(time.Now()) {
			log.Println("Tag scan: window closed, stopping scan")
			break
		}

		// Check for pause request
		tsm.mu.Lock()
		paused := tsm.paused
		tsm.mu.Unlock()

		if paused {
			log.Println("Tag scan: paused")
			<-resumeCh
			log.Println("Tag scan: resumed")
			continue
		}

		// Check for stop
		select {
		case <-stopCh:
			log.Println("Tag scan: stopped during window scan")
			return
		default:
		}

		// Find next untagged image (excluding files with permanent processing errors)
		var imageFile domain.ImageFile
		err := tsm.db.Table("image_files").
			Select("image_files.*").
			Joins("LEFT JOIN image_tags ON image_files.id = image_tags.image_file_id").
			Scopes(withoutPermanentErrors).
			Where("image_tags.id IS NULL AND image_files.id > ?", tsm.cursor).
			Order("image_files.id ASC").
			First(&imageFile).Error

		if err != nil {
			// No more untagged images
			log.Printf("Tag scan: no more untagged images found (cursor=%d), err=%v", tsm.cursor, err)
			break
		}

		log.Printf("Tag scan: found untagged image ID=%d, path=%s", imageFile.ID, imageFile.Path)

		// Update progress
		tsm.mu.Lock()
		tsm.progress.CurrentImage = imageFile.Path
		tsm.mu.Unlock()

		// Process image
		log.Printf("Tag scan: calling processImage for ID=%d", imageFile.ID)
		tsm.processImage(imageFile)
		log.Printf("Tag scan: processImage returned for ID=%d", imageFile.ID)

		// Update cursor and progress
		tsm.mu.Lock()
		tsm.cursor = imageFile.ID
		tsm.progress.Scanned++
		tsm.progress.Remaining--
		tsm.mu.Unlock()

		log.Printf("Tag scan: progress updated, scanned=%d, remaining=%d", tsm.progress.Scanned, tsm.progress.Remaining)
	}

	log.Printf("Tag scan: window scan complete, %d images scanned in this pass", tsm.progress.Scanned)
}

// processImage processes a single image, generating tags via LLM
func (tsm *TagScanManager) processImage(imageFile domain.ImageFile) {
	log.Printf("Tag scan: processImage started for ID=%d, path=%s", imageFile.ID, imageFile.Path)

	// Get VL instrument settings for the model and provider
	var vlInstrument domain.LlmInstrumentSettings
	if err := tsm.db.Where("type = ?", domain.InstrumentVL).Preload("Provider").First(&vlInstrument).Error; err != nil {
		log.Printf("Tag scan: failed to load VL instrument settings: %v", err)
		tsm.mu.Lock()
		tsm.progress.LastError = "Failed to load LLM instrument settings"
		tsm.mu.Unlock()
		return
	}
	provider := vlInstrument.Provider
	model := vlInstrument.Model

	log.Printf("Tag scan: provider=%s, model=%s, url=%s",
		provider.Name, model, provider.ApiUrl)

	// Create LLM client using model from instrument settings
	log.Printf("Tag scan: creating LLM client for provider=%s, url=%s, model=%s",
		provider.Name, provider.ApiUrl, model)
	client, err := llm.NewClient(provider.Name, provider.ApiUrl, provider.ApiKey, model, tsm.maxImageMegapixels)
	if err != nil {
		log.Printf("Tag scan: failed to create LLM client: %v", err)
		tsm.mu.Lock()
		tsm.progress.LastError = fmt.Sprintf("Failed to create LLM client: %v", err)
		tsm.mu.Unlock()
		return
	}
	log.Println("Tag scan: LLM client created successfully")

	// Execute AI action "tags"
	log.Printf("Tag scan: calling ExecuteAiAction for ID=%d, action=tags", imageFile.ID)
	result, err := tsm.llmOcrService.ExecuteAiAction(imageFile.ID, "tags", "", "en", client, provider, model)
	if err != nil {
		log.Printf("Tag scan: failed to generate tags for %s: %v", imageFile.Path, err)
		tsm.mu.Lock()
		tsm.progress.LastError = fmt.Sprintf("Failed to tag %s: %v", imageFile.Path, err)
		tsm.mu.Unlock()

		// If the error is permanent (e.g. corrupt image file), save a processing
		// error record so the file won't be retried on subsequent scan passes.
		if imgutil.IsPermanentError(err) {
			log.Printf("Tag scan: marking %s as permanently failed (corrupt file)", imageFile.Path)
			tsm.mu.Lock()
			tsm.progress.Skipped++
			tsm.mu.Unlock()
			rec := domain.ImageProcessingError{
				ImageFileID: imageFile.ID,
				Action:      actionTags,
				Error:       err.Error(),
			}
			if dbErr := tsm.db.Where("image_file_id = ?", imageFile.ID).
				Assign(rec).
				FirstOrCreate(&rec).Error; dbErr != nil {
				log.Printf("Tag scan: failed to save processing error: %v", dbErr)
			}
		}
		return
	}
	log.Printf("Tag scan: ExecuteAiAction returned %d tags for ID=%d", len(result.Tags), imageFile.ID)

	// Save tags: delete existing tags first, then insert new ones
	tsm.db.Where("image_file_id = ?", imageFile.ID).Delete(&domain.ImageTag{})

	if len(result.Tags) > 0 {
		tags := make([]domain.ImageTag, len(result.Tags))
		for i, tag := range result.Tags {
			tags[i] = domain.ImageTag{
				ImageFileID: imageFile.ID,
				Tag:         tag,
			}
		}
		if err := tsm.db.Create(&tags).Error; err != nil {
			log.Printf("Tag scan: failed to save tags for %s: %v", imageFile.Path, err)
			tsm.mu.Lock()
			tsm.progress.LastError = fmt.Sprintf("Failed to save tags for %s: %v", imageFile.Path, err)
			tsm.mu.Unlock()
			return
		}
		log.Printf("Tag scan: saved %d tags for %s", len(result.Tags), imageFile.Path)

		// Generate embedding immediately for the just-tagged image
		go GenerateAndSaveEmbedding(tsm.db, imageFile.ID, result.Tags)

		// Also trigger batch backfill for any other images missing embeddings
		tsm.mu.Lock()
		eb := tsm.embeddingBackfill
		tsm.mu.Unlock()
		if eb != nil {
			go func() {
				if err := eb.Start(); err != nil {
					log.Printf("Tag scan: embedding backfill not started: %v", err)
				}
			}()
		}
	}
}

// withoutPermanentErrors is a GORM scope that excludes images with a permanent
// processing error (e.g. corrupt/unreadable JPEGs) from tag scan queries.
func withoutPermanentErrors(db *gorm.DB) *gorm.DB {
	return db.
		Joins("LEFT JOIN image_processing_errors ON image_processing_errors.image_file_id = image_files.id").
		Where("image_processing_errors.id IS NULL")
}
