package imaging

import (
	"fmt"
	"testing"

	"github.com/flashbacks/api-service/internal/domain"
	imgutil "github.com/flashbacks/api-service/internal/infrastructure/imaging"
	"github.com/flashbacks/api-service/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- IsPermanentError tests ---

func TestIsPermanentError_DetectsDecodeErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "DecodeError sentinel with JPEG SOS marker",
			err:      &imgutil.DecodeError{Path: "/photo.jpg", Err: fmt.Errorf("image: invalid JPEG format: missing SOS marker")},
			expected: true,
		},
		{
			name:     "DecodeError sentinel wrapped in fmt.Errorf",
			err:      fmt.Errorf("failed to prepare image: %w", &imgutil.DecodeError{Path: "/photo.jpg", Err: fmt.Errorf("image: invalid JPEG format: missing SOS marker")}),
			expected: true,
		},
		{
			name:     "invalid JPEG format fallback (non-sentinel path)",
			err:      fmt.Errorf("image: invalid JPEG format: unexpected marker"),
			expected: true,
		},
		{
			name:     "unknown image format fallback",
			err:      fmt.Errorf("image: unknown format"),
			expected: true,
		},
		{
			name:     "transient error should not be permanent",
			err:      fmt.Errorf("failed to execute AI action: LLM provider timeout"),
			expected: false,
		},
		{
			name:     "API error should not be permanent",
			err:      fmt.Errorf("Ollama recognize: 500 Internal Server Error"),
			expected: false,
		},
		{
			name:     "tag count error should not be permanent",
			err:      fmt.Errorf("tag count out of range: got 5, expected 20-120"),
			expected: false,
		},
		{
			name:     "wrap non-permanent in permanent-looking message",
			err:      fmt.Errorf("failed to execute AI action: API rate limited"),
			expected: false,
		},
		{
			name:     "PNG format error fallback",
			err:      fmt.Errorf("invalid PNG format: bad CRC"),
			expected: true,
		},
		{
			name:     "file not found is not permanent (file may reappear)",
			err:      fmt.Errorf("failed to open image: open /path/photo.jpg: no such file or directory"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := imgutil.IsPermanentError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- ImageProcessingError DB tests ---

func TestImageProcessingError_SavesAndExcludesFromTagScan(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	// Seed an image file
	img := testutil.SeedImageFile(t, db, "/test/corrupt.jpg", "abc123", 1000)

	// Initially no processing error should exist
	var count int64
	db.Model(&domain.ImageProcessingError{}).Where("image_file_id = ?", img.ID).Count(&count)
	assert.Equal(t, int64(0), count)

	// Save a processing error
	err := db.Create(&domain.ImageProcessingError{
		ImageFileID: img.ID,
		Action:      "tags",
		Error:       "failed to decode image: invalid JPEG format: missing SOS marker",
	}).Error
	require.NoError(t, err)

	// Verify it was saved
	var rec domain.ImageProcessingError
	err = db.Where("image_file_id = ?", img.ID).First(&rec).Error
	require.NoError(t, err)
	assert.Equal(t, "tags", rec.Action)
	assert.Contains(t, rec.Error, "invalid JPEG format")
	assert.False(t, rec.CreatedAt.IsZero())

	// Verify the tag scan query excludes this file (LEFT JOIN WHERE NULL should return no rows)
	var imageFile domain.ImageFile
	err = db.Table("image_files").
		Select("image_files.*").
		Joins("LEFT JOIN image_tags ON image_files.id = image_tags.image_file_id").
		Joins("LEFT JOIN image_processing_errors ON image_processing_errors.image_file_id = image_files.id").
		Where("image_tags.id IS NULL AND image_processing_errors.id IS NULL AND image_files.id > ?", uint(0)).
		Order("image_files.id ASC").
		First(&imageFile).Error
	assert.Error(t, err, "should not find any untagged files since corrupt file has a processing error")

	// Verify the total count query also excludes it
	var total int64
	db.Table("image_files").
		Joins("LEFT JOIN image_tags ON image_files.id = image_tags.image_file_id").
		Joins("LEFT JOIN image_processing_errors ON image_processing_errors.image_file_id = image_files.id").
		Where("image_tags.id IS NULL AND image_processing_errors.id IS NULL").
		Count(&total)
	assert.Equal(t, int64(0), total, "corrupt file should not be counted as untagged")
}

func TestImageProcessingError_UniquePerImage(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	img := testutil.SeedImageFile(t, db, "/test/corrupt.jpg", "abc123", 1000)

	// Create first error
	err := db.Create(&domain.ImageProcessingError{
		ImageFileID: img.ID,
		Action:      "tags",
		Error:       "first error",
	}).Error
	require.NoError(t, err)

	// UPSERT: update the existing record
	rec := domain.ImageProcessingError{
		ImageFileID: img.ID,
		Action:      "tags",
		Error:       "updated error",
	}
	err = db.Where("image_file_id = ?", img.ID).
		Assign(rec).
		FirstOrCreate(&rec).Error
	require.NoError(t, err)

	// Should have only one record with updated error
	var count int64
	db.Model(&domain.ImageProcessingError{}).Where("image_file_id = ?", img.ID).Count(&count)
	assert.Equal(t, int64(1), count)

	var saved domain.ImageProcessingError
	db.Where("image_file_id = ?", img.ID).First(&saved)
	assert.Equal(t, "updated error", saved.Error)
}

func TestImageProcessingError_DeletedOnCascade(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	img := testutil.SeedImageFile(t, db, "/test/corrupt.jpg", "abc123", 1000)

	// Save processing error
	db.Create(&domain.ImageProcessingError{
		ImageFileID: img.ID,
		Action:      "tags",
		Error:       "corrupt file",
	})

	// Cascade delete the image file
	deleteImageFileCascade(db, img.ID)

	// Verify processing error was also deleted
	var count int64
	db.Model(&domain.ImageProcessingError{}).Where("image_file_id = ?", img.ID).Count(&count)
	assert.Equal(t, int64(0), count, "processing error should be cascade-deleted with image file")
}
