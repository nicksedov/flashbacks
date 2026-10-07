package imaging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"
	"github.com/flashbacks/api-service/internal/testutil/fixtures"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateFileHash_Success(t *testing.T) {
	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateTestFile(t, tmpDir, "test.txt", []byte("hello world"))
	testPath := filepath.Join(tmpDir, "test.txt")

	hash, err := calculateFileHash(testPath)

	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 32) // MD5 produces 32 hex characters
}

func TestCalculateFileHash_FileNotFound(t *testing.T) {
	_, err := calculateFileHash("/nonexistent/path/file.txt")

	require.Error(t, err)
}

func TestCalculateFileHash_EmptyFile(t *testing.T) {
	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateTestFile(t, tmpDir, "empty.txt", []byte(""))
	testPath := filepath.Join(tmpDir, "empty.txt")

	hash, err := calculateFileHash(testPath)

	require.NoError(t, err)
	assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", hash) // Known MD5 of empty string
}

func TestScanDirectory_NewFiles(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	// Create 3 JPEG files
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	progressChan := make(chan string, 100)

	_, err := scanDirectory(db, tmpDir, progressChan, 2)

	require.NoError(t, err)

	var count int64
	db.Model(&domain.ImageFile{}).Count(&count)
	assert.Equal(t, int64(3), count, "should have 3 image records")
}

func TestScanDirectory_CachedFiles(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)

	// First scan - should create record
	progressChan := make(chan string, 100)
	_, err := scanDirectory(db, tmpDir, progressChan, 1)
	require.NoError(t, err)

	// Second scan - should skip (cached)
	progressChan2 := make(chan string, 100)
	_, err = scanDirectory(db, tmpDir, progressChan2, 1)
	require.NoError(t, err)

	// Count should still be 1
	var count int64
	db.Model(&domain.ImageFile{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestScanDirectory_ModifiedFiles(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 100, 100)

	// First scan
	progressChan := make(chan string, 100)
	_, err := scanDirectory(db, tmpDir, progressChan, 1)
	require.NoError(t, err)

	// Get initial size
	var imageFile domain.ImageFile
	db.First(&imageFile)
	initialSize := imageFile.Size

	// Modify the file (create a new one with different size)
	fixtures.CreateMinimalJPEG(t, tmpDir, "test.jpg", 200, 200)

	// Second scan - should detect modification
	progressChan2 := make(chan string, 100)
	_, err = scanDirectory(db, tmpDir, progressChan2, 1)
	require.NoError(t, err)

	// Size should have changed
	db.First(&imageFile)
	assert.NotEqual(t, initialSize, imageFile.Size, "file size should have changed after modification")
	assert.Greater(t, imageFile.Size, int64(0), "size should be positive")
}

func TestScanDirectory_NoImageFiles(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateTestFile(t, tmpDir, "test.txt", []byte("hello"))
	fixtures.CreateTestFile(t, tmpDir, "document.pdf", []byte("pdf content"))

	progressChan := make(chan string, 100)
	_, err := scanDirectory(db, tmpDir, progressChan, 1)

	require.NoError(t, err)

	var count int64
	db.Model(&domain.ImageFile{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestScanDirectory_InvalidPath(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// scanDirectory gracefully handles invalid paths by reporting errors
	// through the progress channel rather than returning an error
	progressChan := make(chan string, 100)
	_, err := scanDirectory(db, "/nonexistent/path/that/does/not/exist", progressChan, 1)

	// scanDirectory returns nil even for invalid root (errors reported via channel)
	require.NoError(t, err)

	// But the progress channel should contain an error message
	foundError := false
	close(progressChan)
	for msg := range progressChan {
		if msg != "" {
			foundError = true
			break
		}
	}
	assert.True(t, foundError, "progress channel should contain error message for invalid path")
}

func TestFastScanGalleryDirectory_Unchanged(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMultipleTestJPEGs(t, tmpDir, 3)

	// First scan to populate DB
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	// Fast scan - all should be unchanged
	progressChan2 := make(chan string, 100)
	result, _ := fastScanGalleryDirectory(db, tmpDir, progressChan2, 1)

	assert.Equal(t, 3, result.Unchanged)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 0, result.Modified)
}

func TestFastScanGalleryDirectory_NewAndModified(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)
	fixtures.CreateMinimalJPEG(t, tmpDir, "existing.jpg", 100, 100)

	// First scan
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	// Add a new file and modify existing one
	fixtures.CreateMinimalJPEG(t, tmpDir, "new.jpg", 150, 150)
	fixtures.CreateMinimalJPEG(t, tmpDir, "existing.jpg", 200, 200)

	// Fast scan
	progressChan2 := make(chan string, 100)
	result, _ := fastScanGalleryDirectory(db, tmpDir, progressChan2, 1)

	assert.GreaterOrEqual(t, result.Created, 1, "should detect at least 1 new file")
	assert.GreaterOrEqual(t, result.Modified, 1, "should detect at least 1 modified file")
}

func TestFastScanGalleryDirectory_MissingFiles(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)

	// Create 5 JPEG files
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 5)

	// Scan to populate DB
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	// Delete 2 files
	os.Remove(paths[0])
	os.Remove(paths[1])

	// Fast scan - should detect missing files
	progressChan2 := make(chan string, 100)
	result, _ := fastScanGalleryDirectory(db, tmpDir, progressChan2, 1)

	assert.Equal(t, 2, result.Deleted, "should detect 2 deleted files")
}

func TestFindDuplicates_SingleGroup(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)

	// Create 3 files with same content (will have same hash)
	for i := 0; i < 3; i++ {
		name := "file" + string(rune('0'+i)) + ".jpg"
		fixtures.CreateTestFile(t, tmpDir, name, []byte("duplicate content"))
	}

	// Scan directory
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	groups, err := findDuplicates(db)

	require.NoError(t, err)
	assert.Len(t, groups, 1, "should find 1 duplicate group")
	assert.Len(t, groups[0].Files, 3, "group should have 3 files")
}

func TestFindDuplicates_NoDuplicates(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)

	// Create 3 files with different content
	for i := 0; i < 3; i++ {
		name := "file" + string(rune('0'+i)) + ".jpg"
		fixtures.CreateTestFile(t, tmpDir, name, []byte("unique content "+string(rune('0'+i))))
	}

	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	groups, err := findDuplicates(db)

	require.NoError(t, err)
	assert.Empty(t, groups, "should find no duplicates")
}

func TestFindDuplicatesPaginated_FirstPage(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)

	// Create 10 groups of duplicates
	for g := 0; g < 10; g++ {
		content := "group-" + string(rune('0'+g%10)) + string(rune('A'+g/10))
		for f := 0; f < 2; f++ {
			name := "g" + string(rune('0'+g)) + "f" + string(rune('0'+f)) + ".jpg"
			fixtures.CreateTestFile(t, tmpDir, name, []byte(content))
		}
	}

	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	groups, totalGroups, totalFiles, err := FindDuplicatesPaginated(db, 0, 3)

	require.NoError(t, err)
	assert.Len(t, groups, 3, "should return 3 groups on first page")
	assert.GreaterOrEqual(t, totalGroups, 10, "should have at least 10 total groups")
	assert.Greater(t, totalFiles, 0, "should have files counted")
}

func TestFindDuplicatesPaginated_BeyondEnd(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)

	// Create 2 groups
	for g := 0; g < 2; g++ {
		content := "paginated-group-" + string(rune('0'+g))
		for f := 0; f < 2; f++ {
			name := "pg" + string(rune('0'+g)) + "f" + string(rune('0'+f)) + ".jpg"
			fixtures.CreateTestFile(t, tmpDir, name, []byte(content))
		}
	}

	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	groups, totalGroups, _, err := FindDuplicatesPaginated(db, 20, 5)

	require.NoError(t, err)
	assert.Empty(t, groups, "should return empty slice beyond end")
	assert.Equal(t, 2, totalGroups, "should still report correct total")
}

func TestCleanupMissingFiles_RemovesDeleted(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	tmpDir := fixtures.CreateTempDir(t)

	// Create 5 files
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 5)

	// Scan to populate DB
	progressChan := make(chan string, 100)
	scanDirectory(db, tmpDir, progressChan, 1)

	var countBefore int64
	db.Model(&domain.ImageFile{}).Count(&countBefore)
	assert.Equal(t, int64(5), countBefore)

	// Delete 2 files
	os.Remove(paths[0])
	os.Remove(paths[1])

	// Cleanup
	progressChan2 := make(chan string, 100)
	_, err := cleanupMissingFiles(db, progressChan2)

	require.NoError(t, err)

	var countAfter int64
	db.Model(&domain.ImageFile{}).Count(&countAfter)
	assert.Equal(t, int64(3), countAfter, "should have 3 records after cleanup")
}

func TestDeleteImageFileCascade_RemovesAllChildren(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Create an ImageFile
	img := testutil.SeedImageFile(t, db, "/tmp/test/cascade.jpg", "abc123", 1024)

	// Seed child records
	db.Create(&domain.ImageMetadata{ImageFileID: img.ID, Width: 800, Height: 600})
	db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "landscape"})
	db.Create(&domain.ImageTag{ImageFileID: img.ID, Tag: "nature"})
	db.Create(&domain.OcrLlmRecognition{ImageFileID: img.ID, Language: "en", MarkdownContent: "text"})
	classification := testutil.SeedOcrClassification(t, db, img.ID, true)
	db.Create(&domain.OcrBoundingBox{ClassificationID: classification.ID, X: 0, Y: 0, Width: 100, Height: 50, Word: "hello"})
	db.Create(&domain.TagEmbedding{ImageFileID: img.ID, TagCount: 2})

	// Verify children exist
	var tagCount, metaCount, ocrCount, bbCount, embCount, recogCount int64
	db.Model(&domain.ImageTag{}).Where("image_file_id = ?", img.ID).Count(&tagCount)
	db.Model(&domain.ImageMetadata{}).Where("image_file_id = ?", img.ID).Count(&metaCount)
	db.Model(&domain.OcrClassification{}).Where("image_file_id = ?", img.ID).Count(&ocrCount)
	db.Model(&domain.OcrBoundingBox{}).Count(&bbCount)
	db.Model(&domain.TagEmbedding{}).Where("image_file_id = ?", img.ID).Count(&embCount)
	db.Model(&domain.OcrLlmRecognition{}).Where("image_file_id = ?", img.ID).Count(&recogCount)
	require.Equal(t, int64(2), tagCount)
	require.Equal(t, int64(1), metaCount)
	require.Equal(t, int64(1), ocrCount)
	require.Equal(t, int64(1), bbCount)
	require.Equal(t, int64(1), embCount)
	require.Equal(t, int64(1), recogCount)

	// Execute cascade delete
	deleteImageFileCascade(db, img.ID)

	// Verify all children and parent are deleted
	db.Model(&domain.ImageFile{}).Where("id = ?", img.ID).Count(&metaCount)
	assert.Equal(t, int64(0), metaCount, "ImageFile should be deleted")
	db.Model(&domain.ImageTag{}).Where("image_file_id = ?", img.ID).Count(&tagCount)
	assert.Equal(t, int64(0), tagCount, "ImageTags should be deleted")
	db.Model(&domain.ImageMetadata{}).Where("image_file_id = ?", img.ID).Count(&metaCount)
	assert.Equal(t, int64(0), metaCount, "ImageMetadata should be deleted")
	db.Model(&domain.OcrClassification{}).Where("image_file_id = ?", img.ID).Count(&ocrCount)
	assert.Equal(t, int64(0), ocrCount, "OcrClassification should be deleted")
	db.Model(&domain.OcrBoundingBox{}).Count(&bbCount)
	assert.Equal(t, int64(0), bbCount, "OcrBoundingBox should be deleted")
	db.Model(&domain.TagEmbedding{}).Where("image_file_id = ?", img.ID).Count(&embCount)
	assert.Equal(t, int64(0), embCount, "TagEmbedding should be deleted")
	db.Model(&domain.OcrLlmRecognition{}).Where("image_file_id = ?", img.ID).Count(&recogCount)
	assert.Equal(t, int64(0), recogCount, "OcrLlmRecognition should be deleted")
}
