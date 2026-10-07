package imaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThumbnailCache_GetSet(t *testing.T) {
	cache := NewThumbnailCache()

	// Set a thumbnail
	cache.Set("/path/to/image.jpg", "data:image/webp;base64,test")

	// Get it back
	result, ok := cache.Get("/path/to/image.jpg")

	assert.True(t, ok)
	assert.Equal(t, "data:image/webp;base64,test", result)
}

func TestThumbnailCache_GetMissing(t *testing.T) {
	cache := NewThumbnailCache()

	_, ok := cache.Get("/nonexistent/path")

	assert.False(t, ok)
}

func TestThumbnailCache_Overwrite(t *testing.T) {
	cache := NewThumbnailCache()

	cache.Set("/path/to/image.jpg", "old-thumbnail")
	cache.Set("/path/to/image.jpg", "new-thumbnail")

	result, _ := cache.Get("/path/to/image.jpg")

	assert.Equal(t, "new-thumbnail", result)
}
