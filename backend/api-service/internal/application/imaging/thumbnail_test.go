package imaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetImageMimeType_JPG(t *testing.T) {
	tests := []string{".jpg", ".JPG", ".jpeg", ".JPEG"}

	for _, ext := range tests {
		t.Run(ext, func(t *testing.T) {
			result := GetImageMimeType("test" + ext)
			assert.Equal(t, "image/jpeg", result)
		})
	}
}

func TestGetImageMimeType_PNG(t *testing.T) {
	result := GetImageMimeType("test.png")

	assert.Equal(t, "image/png", result)
}

func TestGetImageMimeType_WebP(t *testing.T) {
	result := GetImageMimeType("test.webp")

	assert.Equal(t, "image/webp", result)
}

func TestGetImageMimeType_Unknown(t *testing.T) {
	result := GetImageMimeType("test.xyz")

	// Should default to image/jpeg
	assert.Equal(t, "image/jpeg", result)
}

func TestGetImageMimeType_AllExtensions(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".jpg", "image/jpeg"},
		{".png", "image/png"},
		{".gif", "image/gif"},
		{".bmp", "image/bmp"},
		{".webp", "image/webp"},
		{".tiff", "image/tiff"},
		{".tif", "image/tiff"},
		{".xyz", "image/jpeg"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := GetImageMimeType("file" + tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

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
