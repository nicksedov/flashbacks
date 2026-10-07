package imaging

import (
	"testing"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/testutil"
	"github.com/flashbacks/api-service/internal/testutil/fixtures"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryUnclassifiedImages_FullMode(t *testing.T) {
	om, _, cleanup := setupOcrManager(t)
	defer cleanup()

	// Create 10 image files in DB
	tmpDir := fixtures.CreateTempDir(t)
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 10)
	for _, path := range paths {
		testutil.SeedImageFileNoT(om.db, path, "hash-"+path, 1000)
	}

	images, err := om.queryUnclassifiedImages(false)

	require.NoError(t, err)
	assert.Len(t, images, 10, "should return all 10 images in full mode")
}

func TestQueryUnclassifiedImages_Incremental_NewOnly(t *testing.T) {
	om, _, cleanup := setupOcrManager(t)
	defer cleanup()

	// Create 10 image files
	tmpDir := fixtures.CreateTempDir(t)
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 10)
	for _, path := range paths {
		testutil.SeedImageFileNoT(om.db, path, "hash-"+path, 1000)
	}

	// Classify first 5
	for i := 0; i < 5; i++ {
		var img domain.ImageFile
		om.db.Where("path LIKE ?", "%"+paths[i]+"%").First(&img)
		om.db.Create(&domain.OcrClassification{
			ImageFileID:    img.ID,
			IsTextDocument: true,
		})
	}

	// Incremental query should only return unclassified (5-9)
	images, err := om.queryUnclassifiedImages(true)

	require.NoError(t, err)
	assert.Len(t, images, 5, "should return only 5 unclassified images")
}

func TestQueryUnclassifiedImages_Incremental_Stale(t *testing.T) {
	om, _, cleanup := setupOcrManager(t)
	defer cleanup()

	// Create image files
	tmpDir := fixtures.CreateTempDir(t)
	paths := fixtures.CreateMultipleTestJPEGs(t, tmpDir, 6)
	for _, path := range paths {
		testutil.SeedImageFileNoT(om.db, path, "hash-"+path, 1000)
	}

	// Classify first 3, but with old updatedAt
	for i := 0; i < 3; i++ {
		var img domain.ImageFile
		om.db.Where("path LIKE ?", "%"+paths[i]+"%").First(&img)
		classification := &domain.OcrClassification{
			ImageFileID:    img.ID,
			IsTextDocument: true,
		}
		om.db.Create(classification)
		// Make the classification old
		om.db.Model(classification).Update("updated_at", time.Now().Add(-1*time.Hour))
		// Update the image file to be newer
		om.db.Model(&img).Update("updated_at", time.Now())
	}

	// Incremental query should return stale classifications (updated after classification)
	images, err := om.queryUnclassifiedImages(true)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(images), 3, "should return stale images")
}
