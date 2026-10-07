package imaging

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm/prompts"
	"github.com/flashbacks/api-service/internal/testutil"
	"github.com/flashbacks/api-service/internal/testutil/fixtures"
	"github.com/flashbacks/api-service/internal/testutil/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectLanguage_English(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create a classification with English words
	classification := &domain.OcrClassification{
		IsTextDocument: true,
	}
	service.db.Create(classification)

	// Create bounding boxes with English text
	for _, word := range []string{"hello", "world", "test"} {
		service.db.Create(&domain.OcrBoundingBox{
			ClassificationID: classification.ID,
			Word:             word,
		})
	}

	language := service.detectLanguage(*classification)

	assert.Equal(t, "en", language, "should detect English")
}

func TestDetectLanguage_Russian(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create a classification
	classification := &domain.OcrClassification{
		IsTextDocument: true,
	}
	service.db.Create(classification)

	// Create bounding boxes with Russian text
	for _, word := range []string{"привет", "мир", "тест"} {
		service.db.Create(&domain.OcrBoundingBox{
			ClassificationID: classification.ID,
			Word:             word,
		})
	}

	language := service.detectLanguage(*classification)

	assert.Equal(t, "ru", language, "should detect Russian")
}

func TestParseTags_CommaSeparated(t *testing.T) {
	input := "cat, dog, bird"

	tags := prompts.ParseTags(input)

	assert.Equal(t, []string{"cat", "dog", "bird"}, tags)
}

// --- PostProcessTags tests ---

// generateValidTags returns n unique English tags for testing.
func generateValidTags(n int) []string {
	base := []string{
		"sunset", "mountain", "river", "forest", "ocean",
		"cityscape", "portrait", "wildlife", "architecture", "flower",
		"sky", "cloud", "rain", "snow", "desert",
		"beach", "lake", "waterfall", "valley", "island",
		"bridge", "castle", "temple", "garden", "meadow",
		"canyon", "cave", "cliff", "reef", "aurora",
	}
	if n > len(base) {
		out := make([]string, n)
		copy(out, base)
		for i := len(base); i < n; i++ {
			out[i] = fmt.Sprintf("tag_%d", i)
		}
		return out
	}
	return base[:n]
}

func TestPostProcessTags_ValidCount(t *testing.T) {
	raw := generateValidTags(30)
	result, err := PostProcessTags(raw)

	require.NoError(t, err)
	assert.Len(t, result, 30)
}

func TestPostProcessTags_FiltersChineseCharacters(t *testing.T) {
	raw := generateValidTags(25)
	raw = append(raw, "\u5c71\u6c34", "\u98ce\u666f", "beautiful \u82b1") // Chinese tags, mixed

	result, err := PostProcessTags(raw)

	require.NoError(t, err)
	assert.Len(t, result, 25)
	for _, tag := range result {
		assert.NotContains(t, tag, "\u5c71")
	}
}

func TestPostProcessTags_RemovesDuplicates_CaseInsensitive(t *testing.T) {
	raw := generateValidTags(22)
	raw = append(raw, "Sunset", "SUNSET", "MOUNTAIN") // duplicates of existing tags

	result, err := PostProcessTags(raw)

	require.NoError(t, err)
	assert.Len(t, result, 22)
}

func TestPostProcessTags_FiltersHallucinationPrefixes(t *testing.T) {
	raw := generateValidTags(25)
	raw = append(raw,
		"\u0420\u0443\u0441\u0441\u043a\u0438\u0439 \u0442\u0435\u0433: \u043a\u043e\u0442", // "Русский тег: кот"
		"English tag: cat",
		"Russian tag: dog",
	)

	result, err := PostProcessTags(raw)

	require.NoError(t, err)
	assert.Len(t, result, 25)
	for _, tag := range result {
		lower := strings.ToLower(tag)
		assert.False(t, strings.HasPrefix(lower, "\u0440\u0443\u0441\u0441\u043a\u0438\u0439 \u0442\u0435\u0433"), "unexpected русский тег: %s", tag)
		assert.False(t, strings.HasPrefix(lower, "english tag"), "unexpected english tag: %s", tag)
		assert.False(t, strings.HasPrefix(lower, "russian tag"), "unexpected russian tag: %s", tag)
	}
}

func TestPostProcessTags_TooFewTags_ReturnsError(t *testing.T) {
	raw := generateValidTags(10)

	_, err := PostProcessTags(raw)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag count out of range")
}

func TestPostProcessTags_TooManyTags_ReturnsError(t *testing.T) {
	raw := generateValidTags(130)

	_, err := PostProcessTags(raw)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag count out of range")
}

func TestPostProcessTags_BoundaryMin20(t *testing.T) {
	raw := generateValidTags(20)

	result, err := PostProcessTags(raw)

	require.NoError(t, err)
	assert.Len(t, result, 20)
}

func TestPostProcessTags_BoundaryMax120(t *testing.T) {
	raw := generateValidTags(120)

	result, err := PostProcessTags(raw)

	require.NoError(t, err)
	assert.Len(t, result, 120)
}

func TestPostProcessTags_EmptyAndWhitespaceSkipped(t *testing.T) {
	raw := generateValidTags(25)
	raw = append(raw, "", "   ", "\t")

	result, err := PostProcessTags(raw)

	require.NoError(t, err)
	assert.Len(t, result, 25)
}

func TestPostProcessTags_CombinedFilters(t *testing.T) {
	// 22 valid + 3 Chinese + 3 duplicates + 3 hallucination = 22 kept
	raw := generateValidTags(22)
	raw = append(raw,
		"\u5c71", "\u6c34", "\u82b1", // Chinese — filtered
		"Sunset", "Ocean", "FOREST", // duplicates — filtered
		"\u0420\u0443\u0441\u0441\u043a\u0438\u0439 \u0442\u0435\u0433 abc", "English tag xyz", "Russian tag qrs", // hallucination — filtered
	)

	result, err := PostProcessTags(raw)

	require.NoError(t, err)
	assert.Len(t, result, 22)
}

func TestLlmOcrService_Recognize_Success(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	// Create OCR classification with bounding boxes (for language detection)
	classification := &domain.OcrClassification{
		ImageFileID:    imgFile.ID,
		IsTextDocument: true,
	}
	service.db.Create(classification)

	// Add bounding boxes with English text
	for _, word := range []string{"hello", "world"} {
		service.db.Create(&domain.OcrBoundingBox{
			ClassificationID: classification.ID,
			Word:             word,
		})
	}

	// Mock LLM to return markdown
	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse("# Document Title\n\nContent goes here."),
	}
	provider := domain.LlmProvider{Name: "ollama", Alias: "ollama-provider", ApiUrl: "http://localhost:11434"}

	result, err := service.RecognizeWithLlm(imgFile.ID, mockClient, provider, "minicpm-v")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Contains(t, result.MarkdownContent, "# Document Title")
	assert.Equal(t, "en", result.Language)
	assert.Equal(t, "ollama", result.Provider)
	assert.Equal(t, "minicpm-v", result.Model)

	// Verify recognition saved to DB
	saved, err := service.GetRecognition(imgFile.ID)
	require.NoError(t, err)
	assert.NotNil(t, saved)
	assert.True(t, saved.Success)
}

func TestLlmOcrService_Recognize_LLMError(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	// Create OCR classification with bounding boxes
	classification := &domain.OcrClassification{
		ImageFileID:    imgFile.ID,
		IsTextDocument: true,
	}
	service.db.Create(classification)

	for _, word := range []string{"hello", "world"} {
		service.db.Create(&domain.OcrBoundingBox{
			ClassificationID: classification.ID,
			Word:             word,
		})
	}

	// Mock LLM to return error
	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.LlmErrorResponse(assert.AnError),
	}
	provider := domain.LlmProvider{Name: "ollama", Alias: "ollama-provider", ApiUrl: "http://localhost:11434"}

	_, err := service.RecognizeWithLlm(imgFile.ID, mockClient, provider, "minicpm-v")

	require.Error(t, err)

	// Verify failed recognition was saved to DB
	saved, dbErr := service.GetRecognition(imgFile.ID)
	require.NoError(t, dbErr)
	assert.NotNil(t, saved)
	assert.False(t, saved.Success)
	assert.NotEmpty(t, saved.Error)
}

func TestLlmOcrService_Recognize_MissingClassification(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file without any OCR classification
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	// Mock LLM to return markdown
	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse("# Recognized without classification"),
	}
	provider := domain.LlmProvider{Name: "ollama", Alias: "ollama-provider", ApiUrl: "http://localhost:11434"}

	result, err := service.RecognizeWithLlm(imgFile.ID, mockClient, provider, "minicpm-v")

	require.NoError(t, err, "should succeed even without OCR classification")
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "en", result.Language, "should default to English")
	assert.Contains(t, result.MarkdownContent, "Recognized without classification")

	// Verify recognition saved to DB with OcrClassificationID = 0
	saved, dbErr := service.GetRecognition(imgFile.ID)
	require.NoError(t, dbErr)
	require.NotNil(t, saved)
	assert.Equal(t, uint(0), saved.OcrClassificationID)
}

func TestLlmOcrService_GetRecognition_HasData(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create a recognition
	recognition := &domain.OcrLlmRecognition{
		ImageFileID:     1,
		MarkdownContent: "# Test",
		Language:        "en",
	}
	service.db.Create(recognition)

	// Retrieve it
	result, err := service.GetRecognition(1)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "# Test", result.MarkdownContent)
}

func TestLlmOcrService_StartRecognizeAsync_NewTask(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Mock LLM client (function will fail since no image file, but task should still launch)
	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse("recognized content"),
	}
	// Start recognition (will fail since no image file, but should launch task)
	provider := domain.LlmProvider{Name: "ollama", Alias: "ollama-provider", ApiUrl: "http://localhost:11434"}
	result := service.StartRecognizeAsync(1, mockClient, provider, "minicpm-v")

	assert.True(t, result, "should return true for new task")
}

func TestLlmOcrService_StartRecognizeAsync_Duplicate(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create a processing task
	service.taskMu.Lock()
	service.processingTasks[1] = &LlmRecognitionStatus{Status: "processing"}
	service.taskMu.Unlock()

	// Mock LLM client (function will fail since no image file, but task launch attempt should be prevented)
	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse("recognized content"),
	}
	// Try to start duplicate
	provider := domain.LlmProvider{Name: "ollama", Alias: "ollama-provider", ApiUrl: "http://localhost:11434"}
	result := service.StartRecognizeAsync(1, mockClient, provider, "minicpm-v")

	assert.False(t, result, "should return false for duplicate task")
}

func TestLlmOcrService_GetRecognizeStatus_Processing(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create a processing task
	service.taskMu.Lock()
	service.processingTasks[1] = &LlmRecognitionStatus{Status: "processing"}
	service.taskMu.Unlock()

	status := service.GetRecognizeStatus(1)

	assert.NotNil(t, status)
	assert.Equal(t, "processing", status.Status)
}

func TestLlmOcrService_ExecuteAiAction_Describe(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse("A beautiful landscape image with mountains and a lake."),
	}
	provider := domain.LlmProvider{Name: "ollama", Alias: "ollama-provider", ApiUrl: "http://localhost:11434"}
	result, err := service.ExecuteAiAction(imgFile.ID, "describe", "", "en", mockClient, provider, "minicpm-v")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Result, "landscape")
	assert.Equal(t, "ollama", result.Provider)
}

func TestLlmOcrService_ExecuteAiAction_Tags(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	// Build a valid tag string with 20+ tags to pass PostProcessTags validation
	validTags := strings.Join(generateValidTags(25), ", ")
	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse(validTags),
	}
	provider := domain.LlmProvider{Name: "ollama", Alias: "ollama-provider", ApiUrl: "http://localhost:11434"}

	result, err := service.ExecuteAiAction(imgFile.ID, "tags", "", "en", mockClient, provider, "minicpm-v")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Len(t, result.Tags, 25)
	assert.Equal(t, "sunset", result.Tags[0])
}

func TestLlmOcrService_StoreCachedTagsResult(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	tags := []string{"cat", "dog", "bird"}
	service.StoreCachedTagsResult("task-1", tags)

	status := service.GetAiActionStatus("task-1")
	require.NotNil(t, status)
	assert.Equal(t, "completed", status.Status)
	assert.Equal(t, "tags", status.Action)
	require.NotNil(t, status.Result)
	assert.Equal(t, tags, status.Result.Tags)

	// Status should be cleaned up after first read
	assert.Nil(t, service.GetAiActionStatus("task-1"))
}

func TestLlmOcrService_SaveImageTags(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	imgFile := testutil.SeedImageFileNoT(service.db, "/tmp/test.jpg", "hash123", 1000)

	// Save initial tags
	err := service.SaveImageTags(imgFile.ID, []string{"cat", "dog"})
	require.NoError(t, err)

	var tags []domain.ImageTag
	service.db.Where("image_file_id = ?", imgFile.ID).Find(&tags)
	assert.Len(t, tags, 2)

	// Save new tags - should replace old ones
	err = service.SaveImageTags(imgFile.ID, []string{"bird", "fish", "snake"})
	require.NoError(t, err)

	service.db.Where("image_file_id = ?", imgFile.ID).Find(&tags)
	assert.Len(t, tags, 3)
	tagValues := make([]string, len(tags))
	for i, tag := range tags {
		tagValues[i] = tag.Tag
	}
	assert.Equal(t, []string{"bird", "fish", "snake"}, tagValues)
}

func TestLlmOcrService_SaveImageTags_EmptyTags(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	imgFile := testutil.SeedImageFileNoT(service.db, "/tmp/test.jpg", "hash123", 1000)

	// Save initial tags
	err := service.SaveImageTags(imgFile.ID, []string{"cat", "dog"})
	require.NoError(t, err)

	// Save empty tags - should delete old ones and not insert new
	err = service.SaveImageTags(imgFile.ID, []string{})
	require.NoError(t, err)

	var tags []domain.ImageTag
	service.db.Where("image_file_id = ?", imgFile.ID).Find(&tags)
	assert.Empty(t, tags)
}

func TestLlmOcrService_StartAiActionAsync_SavesTagsToDB(t *testing.T) {
	service, cleanup := setupLlmOcrService(t)
	defer cleanup()

	// Create image file
	tmpDir := fixtures.CreateTempDir(t)
	imgPath := fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)
	imgFile := testutil.SeedImageFileNoT(service.db, imgPath, "hash123", 1000)

	// Build a valid tag string with 20+ tags to pass PostProcessTags validation
	validTags := strings.Join(generateValidTags(25), ", ")
	mockClient := &mocks.MockLlmClient{
		RecognizeFunc: mocks.TextResponse(validTags),
	}
	model := "minicpm-v"
	provider := domain.LlmProvider{Name: "ollama", Alias: "ollama-provider", ApiUrl: "http://localhost:11434"}
	service.StartAiActionAsync("task-1", imgFile.ID, "tags", "", "en", mockClient, provider, model)

	// Wait for async completion
	require.Eventually(t, func() bool {
		s := service.GetAiActionStatus("task-1")
		return s != nil && s.Status != "processing"
	}, 5*time.Second, 50*time.Millisecond)

	// Verify tags were saved to DB
	var tags []domain.ImageTag
	service.db.Where("image_file_id = ?", imgFile.ID).Find(&tags)
	assert.Len(t, tags, 25)
}

func setupLlmOcrService(t *testing.T) (*LlmOcrService, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	service := NewLlmOcrService(db)
	return service, cleanup
}
