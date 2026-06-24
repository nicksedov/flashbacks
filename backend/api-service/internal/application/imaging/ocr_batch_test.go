package imaging

import (
	"testing"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"

	"github.com/stretchr/testify/assert"
)

func setupClassificationBatch(t *testing.T) (*ClassificationBatch, func()) {
	t.Helper()
	db, cleanup := testutil.NewTestDB(t)
	return NewClassificationBatch(db), cleanup
}

func TestClassificationBatch_Add(t *testing.T) {
	batch, _ := setupClassificationBatch(t)

	result := OcrResult{
		Image: domain.ImageFile{ID: 1},
		Classification: &domain.OcrClassification{
			ImageFileID:    1,
			IsTextDocument: true,
			MeanConfidence: 0.95,
		},
	}

	batch.Add(result)

	assert.Equal(t, 1, len(batch.Classifications), "batch size should be 1")
}

func TestClassificationBatch_Add_Nil(t *testing.T) {
	batch, _ := setupClassificationBatch(t)

	result := OcrResult{
		Image:          domain.ImageFile{ID: 1},
		Classification: nil,
	}

	batch.Add(result)

	assert.Equal(t, 0, len(batch.Classifications), "nil classification should be ignored")
}

func TestClassificationBatch_IsFull(t *testing.T) {
	batch, _ := setupClassificationBatch(t)

	// Add batchSize (20) results
	for i := 0; i < batchSize; i++ {
		batch.Add(OcrResult{
			Image: domain.ImageFile{ID: uint(i)},
			Classification: &domain.OcrClassification{
				ImageFileID:    uint(i),
				IsTextDocument: true,
			},
		})
	}

	assert.True(t, batch.IsFull(), "batch should be full at 20 items")
}

func TestClassificationBatch_Save(t *testing.T) {
	batch, _ := setupClassificationBatch(t)

	// Add 5 classifications
	for i := 0; i < 5; i++ {
		batch.Add(OcrResult{
			Image: domain.ImageFile{ID: uint(i)},
			Classification: &domain.OcrClassification{
				ImageFileID:    uint(i),
				IsTextDocument: true,
				MeanConfidence: 0.95,
			},
		})
	}

	batch.Save()

	// Verify saved to DB
	var count int64
	batch.db.Model(&domain.OcrClassification{}).Count(&count)
	assert.Equal(t, int64(5), count, "should have saved 5 classifications")

	// Verify batch is cleared
	assert.Equal(t, 0, len(batch.Classifications), "batch should be cleared after save")
}

func TestClassificationBatch_Save_Empty(t *testing.T) {
	batch, _ := setupClassificationBatch(t)

	// Save without adding anything
	batch.Save()

	// Should not panic or error
	var count int64
	batch.db.Model(&domain.OcrClassification{}).Count(&count)
	assert.Equal(t, int64(0), count, "should not create any records")
}
