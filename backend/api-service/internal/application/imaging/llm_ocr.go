package imaging

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
	"github.com/flashbacks/api-service/internal/infrastructure/llm/prompts"

	"gorm.io/gorm"
)

// LlmRecognitionStatus represents the status of an async recognition task
type LlmRecognitionStatus struct {
	Status    string           // "processing", "completed", "failed"
	Result    *RecognizeResult // non-nil when Status == "completed"
	Error     string           // non-empty when Status == "failed"
	CreatedAt time.Time        // when the task was created
}

// AiTaskStatus represents the status of an async AI action task
type AiTaskStatus struct {
	Status    string          // "processing", "completed", "failed"
	Result    *AiActionResult // non-nil when Status == "completed"
	Error     string          // non-empty when Status == "failed"
	Action    string          // the action type (describe, tags, etc.)
	CreatedAt time.Time       // when the task was created
}

// AiTaskCoordinator coordinates background scanning with AI tasks
type AiTaskCoordinator interface {
	RequestPause()
	Resume()
}

// taskTTL is the time after which completed/failed tasks are automatically cleaned up.
const taskTTL = 30 * time.Minute

// LlmOcrService handles VL LLM-based OCR recognition
type LlmOcrService struct {
	db              *gorm.DB
	processingTasks map[uint]*LlmRecognitionStatus
	taskMu          sync.Mutex
	aiActionTasks   map[string]*AiTaskStatus // key: task ID
	aiTaskMu        sync.Mutex
	coordinator     AiTaskCoordinator
	stopCleanup     chan struct{}
}

// NewLlmOcrService creates a new LLM OCR service
func NewLlmOcrService(db *gorm.DB) *LlmOcrService {
	svc := &LlmOcrService{
		db:              db,
		processingTasks: make(map[uint]*LlmRecognitionStatus),
		aiActionTasks:   make(map[string]*AiTaskStatus),
		stopCleanup:     make(chan struct{}),
	}
	go svc.cleanupLoop()
	return svc
}

// Stop terminates the background TTL cleanup goroutine.
func (s *LlmOcrService) Stop() {
	close(s.stopCleanup)
}

// cleanupLoop periodically removes completed/failed tasks older than taskTTL.
func (s *LlmOcrService) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanupExpiredTasks()
		case <-s.stopCleanup:
			return
		}
	}
}

// cleanupExpiredTasks removes tasks that have completed/failed and are older than taskTTL.
func (s *LlmOcrService) cleanupExpiredTasks() {
	now := time.Now()

	s.taskMu.Lock()
	for id, task := range s.processingTasks {
		if task.Status != "processing" && now.Sub(task.CreatedAt) > taskTTL {
			delete(s.processingTasks, id)
		}
	}
	s.taskMu.Unlock()

	s.aiTaskMu.Lock()
	for id, task := range s.aiActionTasks {
		if task.Status != "processing" && now.Sub(task.CreatedAt) > taskTTL {
			delete(s.aiActionTasks, id)
		}
	}
	s.aiTaskMu.Unlock()
}

// SetCoordinator sets the AI task coordinator for pause/resume coordination
func (s *LlmOcrService) SetCoordinator(c AiTaskCoordinator) {
	s.coordinator = c
}

// RecognizeResult holds the result of LLM recognition
type RecognizeResult struct {
	Success          bool
	MarkdownContent  string
	Language         string
	Provider         string
	Model            string
	ProcessingTimeMs int
	Error            string
}

// RecognizeWithLlm performs OCR using VL LLM
func (s *LlmOcrService) RecognizeWithLlm(imageFileID uint, client llm.Client, provider domain.LlmProvider) (*RecognizeResult, error) {
	// Step 1: Try to get OCR classification for language detection (optional)
	var classificationID uint
	language := "en" // Default to English when no classification exists

	var classification domain.OcrClassification
	if err := s.db.Where("image_file_id = ?", imageFileID).First(&classification).Error; err == nil {
		classificationID = classification.ID
		language = s.detectLanguage(classification)
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to get OCR classification: %w", err)
	}

	// Step 2: Build system prompt
	systemPrompt := buildOcrSystemPrompt(language)

	// Step 3: Get image path
	var imageFile domain.ImageFile
	if err := s.db.First(&imageFile, imageFileID).Error; err != nil {
		return nil, fmt.Errorf("failed to get image file: %w", err)
	}

	// Step 4: Call LLM
	startTime := time.Now()
	markdownContent, err := client.Recognize(context.Background(), imageFile.Path, systemPrompt, "Perform OCR on this image and return markdown content.")
	processingTime := int(time.Since(startTime).Milliseconds())

	if err != nil {
		// Save failed result using UPSERT
		recognition := domain.OcrLlmRecognition{
			ImageFileID:         imageFileID,
			OcrClassificationID: classificationID,
			Language:            language,
			MarkdownContent:     "",
			Provider:            provider.Name,
			Model:               provider.Model,
			ProcessingTimeMs:    processingTime,
			Error:               err.Error(),
			Success:             false,
		}
		s.db.
			Where("image_file_id = ?", imageFileID).
			Assign(recognition).
			FirstOrCreate(&recognition)
		return nil, err
	}

	// Step 5: Save successful result using UPSERT
	recognition := domain.OcrLlmRecognition{
		ImageFileID:         imageFileID,
		OcrClassificationID: classificationID,
		Language:            language,
		MarkdownContent:     markdownContent,
		Provider:            provider.Name,
		Model:               provider.Model,
		ProcessingTimeMs:    processingTime,
		Error:               "",
		Success:             true,
	}
	if err := s.db.
		Where("image_file_id = ?", imageFileID).
		Assign(recognition).
		FirstOrCreate(&recognition).Error; err != nil {
		return nil, fmt.Errorf("failed to save recognition result: %w", err)
	}

	return &RecognizeResult{
		Success:          true,
		MarkdownContent:  markdownContent,
		Language:         language,
		Provider:         provider.Name,
		Model:            provider.Model,
		ProcessingTimeMs: processingTime,
		Error:            "",
	}, nil
}

// StartRecognizeAsync starts LLM recognition in a background goroutine.
// Returns true if a new task was started, false if already processing.
func (s *LlmOcrService) StartRecognizeAsync(imageFileID uint, client llm.Client, provider domain.LlmProvider) bool {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	if task, exists := s.processingTasks[imageFileID]; exists && task.Status == "processing" {
		return false // already processing
	}

	s.processingTasks[imageFileID] = &LlmRecognitionStatus{Status: "processing", CreatedAt: time.Now()}

	go func() {
		if s.coordinator != nil {
			s.coordinator.RequestPause()
			defer s.coordinator.Resume()
		}
		result, err := s.RecognizeWithLlm(imageFileID, client, provider)

		s.taskMu.Lock()
		defer s.taskMu.Unlock()

		if err != nil {
			s.processingTasks[imageFileID] = &LlmRecognitionStatus{
				Status:    "failed",
				Error:     err.Error(),
				CreatedAt: time.Now(),
			}
		} else {
			s.processingTasks[imageFileID] = &LlmRecognitionStatus{
				Status:    "completed",
				Result:    result,
				CreatedAt: time.Now(),
			}
		}
	}()

	return true
}

// GetRecognizeStatus returns the current async task status for an image.
// Returns nil if no task exists for this image.
func (s *LlmOcrService) GetRecognizeStatus(imageFileID uint) *LlmRecognitionStatus {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	task, exists := s.processingTasks[imageFileID]
	if !exists {
		return nil
	}

	// Clean up completed/failed tasks after reading
	if task.Status != "processing" {
		delete(s.processingTasks, imageFileID)
	}

	return task
}

// GetRecognition retrieves LLM OCR recognition for an image
func (s *LlmOcrService) GetRecognition(imageFileID uint) (*domain.OcrLlmRecognition, error) {
	var recognition domain.OcrLlmRecognition
	if err := s.db.Where("image_file_id = ?", imageFileID).Order("id DESC").First(&recognition).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &recognition, nil
}

// detectLanguage determines document language from OCR classification data
func (s *LlmOcrService) detectLanguage(classification domain.OcrClassification) string {
	// Get bounding boxes to analyze words for language detection
	var boxes []domain.OcrBoundingBox
	s.db.Where("classification_id = ?", classification.ID).Find(&boxes)

	if len(boxes) == 0 {
		return "en" // Default to English
	}

	// Simple language detection based on character sets
	russianCount := 0
	englishCount := 0

	for _, box := range boxes {
		word := strings.ToLower(box.Word)
		for _, ch := range word {
			// Cyrillic range
			if ch >= 0x0400 && ch <= 0x04FF {
				russianCount++
			}
			// Latin range
			if ch >= 0x0061 && ch <= 0x007A {
				englishCount++
			}
		}
	}

	if russianCount > englishCount {
		return "ru"
	}
	return "en"
}

// AiActionResult holds the result of an AI action
type AiActionResult struct {
	Success          bool
	Result           string
	Tags             []string
	Provider         string
	Model            string
	ProcessingTimeMs int
	Error            string
}

// ExecuteAiAction performs an AI action (describe, tags, recognizeText, askQuestion)
func (s *LlmOcrService) ExecuteAiAction(imageFileID uint, action string, question string, language string, client llm.Client, provider domain.LlmProvider) (*AiActionResult, error) {
	// Get image file
	var imageFile domain.ImageFile
	if err := s.db.First(&imageFile, imageFileID).Error; err != nil {
		return nil, fmt.Errorf("failed to get image file: %w", err)
	}

	// Build system prompt based on action
	systemPrompt := prompts.BuildActionPrompt(action, question, language)
	userMessage := prompts.BuildActionUserMessage(action)

	// Call LLM
	startTime := time.Now()
	response, err := client.Recognize(context.Background(), imageFile.Path, systemPrompt, userMessage)
	processingTime := int(time.Since(startTime).Milliseconds())

	if err != nil {
		return nil, fmt.Errorf("failed to execute AI action: %w", err)
	}

	result := &AiActionResult{
		Success:          true,
		Provider:         provider.Name,
		Model:            provider.Model,
		ProcessingTimeMs: processingTime,
	}

	// Parse response based on action
	switch action {
	case "tags":
		// Split comma-separated or newline-separated tags
		rawTags := prompts.ParseTags(response)
		tags, err := PostProcessTags(rawTags)
		if err != nil {
			return nil, fmt.Errorf("tag post-processing failed: %w", err)
		}
		result.Tags = tags
		result.Result = strings.Join(tags, ", ")
	default:
		result.Result = response
	}

	return result, nil
}

// hallucinationPrefixes are model hallucination markers that must be discarded.
var hallucinationPrefixes = []string{
	"tag1", "tag2", "tag3",
	"тег1", "тег2", "тег3",
	"русский тег", "russian tag", "english tag",
}

// containsCJK reports whether s contains any CJK Unified Ideograph.
func containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// isHallucinationTag returns true when the tag starts with a known
// hallucination prefix (case-insensitive).
func isHallucinationTag(tag string) bool {
	lower := strings.ToLower(tag)
	for _, prefix := range hallucinationPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// PostProcessTags filters and validates LLM-generated tags.
//
// Filtering rules:
//   - Remove tags containing Chinese (CJK) characters
//   - Remove duplicate tags (case-insensitive comparison)
//   - Remove tags that start with hallucination prefixes
//     ("Русский тег", "Russian tag", "English tag")
//
// Validation:
//   - The remaining count must be in [20, 120].
//     Returns an error otherwise.
func PostProcessTags(raw []string) ([]string, error) {
	seen := make(map[string]struct{}, len(raw))
	result := make([]string, 0, len(raw))

	for _, tag := range raw {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}

		// Rule 1: discard tags with CJK characters
		if containsCJK(trimmed) {
			continue
		}

		// Rule 3: discard hallucination markers
		if isHallucinationTag(trimmed) {
			continue
		}

		// Rule 2: deduplicate (case-insensitive)
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		result = append(result, trimmed)
	}

	// Validate count range
	if len(result) < 20 || len(result) > 120 {
		return nil, fmt.Errorf(
			"tag count out of range: got %d, expected 20–120", len(result))
	}

	return result, nil
}

// StoreCachedTagsResult stores a pre-completed task with cached tags from the database.
// No goroutine or LLM call is involved.
func (s *LlmOcrService) StoreCachedTagsResult(taskID string, tags []string) {
	s.aiTaskMu.Lock()
	defer s.aiTaskMu.Unlock()
	s.aiActionTasks[taskID] = &AiTaskStatus{
		Status:    "completed",
		Action:    "tags",
		CreatedAt: time.Now(),
		Result: &AiActionResult{
			Success: true,
			Tags:    tags,
		},
	}
}

// SaveImageTags persists AI-generated tags to the image_tags table.
// Existing tags for the image are deleted before inserting new ones.
func (s *LlmOcrService) SaveImageTags(imageFileID uint, tags []string) error {
	if err := s.db.Where("image_file_id = ?", imageFileID).Delete(&domain.ImageTag{}).Error; err != nil {
		return fmt.Errorf("failed to delete existing tags: %w", err)
	}
	if len(tags) == 0 {
		return nil
	}
	records := make([]domain.ImageTag, len(tags))
	for i, tag := range tags {
		records[i] = domain.ImageTag{
			ImageFileID: imageFileID,
			Tag:         tag,
		}
	}
	if err := s.db.Create(&records).Error; err != nil {
		return fmt.Errorf("failed to insert tags: %w", err)
	}
	return nil
}

// StartAiActionAsync starts an AI action in a background goroutine.
// Returns a unique task ID that can be used to poll for status.
func (s *LlmOcrService) StartAiActionAsync(taskID string, imageFileID uint, action string, question string, language string, client llm.Client, provider domain.LlmProvider) {
	s.aiTaskMu.Lock()
	s.aiActionTasks[taskID] = &AiTaskStatus{
		Status:    "processing",
		Action:    action,
		CreatedAt: time.Now(),
	}
	s.aiTaskMu.Unlock()

	go func() {
		if s.coordinator != nil {
			s.coordinator.RequestPause()
			defer s.coordinator.Resume()
		}
		result, err := s.ExecuteAiAction(imageFileID, action, question, language, client, provider)

		// Persist tags to DB so future requests can skip the LLM call
		if err == nil && action == "tags" && len(result.Tags) > 0 {
			if saveErr := s.SaveImageTags(imageFileID, result.Tags); saveErr != nil {
				log.Printf("AI action: failed to persist tags for image %d: %v", imageFileID, saveErr)
			}
			// Generate embedding for the newly saved tags
			go GenerateAndSaveEmbedding(s.db, imageFileID, result.Tags)
		}

		s.aiTaskMu.Lock()
		defer s.aiTaskMu.Unlock()

		if err != nil {
			s.aiActionTasks[taskID] = &AiTaskStatus{
				Status:    "failed",
				Error:     err.Error(),
				Action:    action,
				CreatedAt: time.Now(),
			}
		} else {
			s.aiActionTasks[taskID] = &AiTaskStatus{
				Status:    "completed",
				Result:    result,
				Action:    action,
				CreatedAt: time.Now(),
			}
		}
	}()
}

// GetAiActionStatus returns the current status of an async AI action task.
// Returns nil if no task exists for this task ID.
func (s *LlmOcrService) GetAiActionStatus(taskID string) *AiTaskStatus {
	s.aiTaskMu.Lock()
	defer s.aiTaskMu.Unlock()

	task, exists := s.aiActionTasks[taskID]
	if !exists {
		return nil
	}

	// Clean up completed/failed tasks after reading
	if task.Status != "processing" {
		delete(s.aiActionTasks, taskID)
	}

	return task
}
