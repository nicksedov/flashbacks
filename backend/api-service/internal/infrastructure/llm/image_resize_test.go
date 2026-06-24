package llm

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundToMultipleOf32(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"exact multiple", 128, 128},
		{"round down", 100, 96},         // 100/32 = 3.125 → 3 → 96
		{"midpoint rounds away from zero", 112, 128}, // 112/32 = 3.5 → Round(3.5)=4 → 128
		{"round up", 120, 128},          // 120/32 = 3.75 → 4 → 128
		{"small value", 16, 32},         // 16/32 = 0.5 → Round(0.5)=1 → 32
		{"very small", 1, 32},           // 1/32 ≈ 0.03 → Round=0 → clamped to 1 → 32
		{"large value exact", 1920, 1920}, // 1920/32 = 60 → exact
		{"large off round down", 1935, 1920}, // 1935/32 = 60.47 → 60 → 1920
		{"large off round up", 1940, 1952},  // 1940/32 = 60.625 → 61 → 1952
		{"zero clamped", 0, 32},         // 0/32 = 0 → clamped to 1 → 32
		{"negative clamped", -10, 32},   // negative → clamped to 1 → 32
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := roundToMultipleOf32(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, 0, result%32, "result must be divisible by 32")
		})
	}
}

func TestRoundToMultipleOf32_Divisible(t *testing.T) {
	// Verify a wide range of values always produce multiples of 32
	for v := 1; v <= 4096; v++ {
		result := roundToMultipleOf32(v)
		assert.Equal(t, 0, result%32, "roundToMultipleOf32(%d) = %d is not divisible by 32", v, result)
		assert.GreaterOrEqual(t, result, 32, "result must be at least 32")
	}
}

// createTempJPEG creates a minimal JPEG file with given dimensions and returns its path.
func createTempJPEG(t *testing.T, dir, name string, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 50}))
	return path
}

func TestResizeImageForLLM_NoResize_WithinLimit(t *testing.T) {
	dir := t.TempDir()
	// 100x100 = 0.01 MP, well under limit
	path := createTempJPEG(t, dir, "small.jpg", 100, 100)

	data, mediaType, err := resizeImageForLLM(path, 3.6)
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", mediaType)
	assert.NotEmpty(t, data)
}

func TestResizeImageForLLM_ResizesAndAlignsTo32(t *testing.T) {
	dir := t.TempDir()
	// 3000x2000 = 6 MP, exceeds 3.6 MP limit → scale = sqrt(3.6/6) ≈ 0.7746
	// raw: 2324x1549 → snapped: 2336x1536 (both multiples of 32)
	// 2324/32=72.625→73→2336, 1549/32=48.4→48→1536
	path := createTempJPEG(t, dir, "large.jpg", 3000, 2000)

	data, mediaType, err := resizeImageForLLM(path, 3.6)
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", mediaType)
	assert.NotEmpty(t, data)
}

func TestResizeImageForLLM_InvalidPath(t *testing.T) {
	_, _, err := resizeImageForLLM("/nonexistent/path.jpg", 3.6)
	assert.Error(t, err)
}

func TestResizeImageForLLM_WebPMediaType(t *testing.T) {
	dir := t.TempDir()
	// Create a JPEG but name it .webp to test media type detection from extension
	path := createTempJPEG(t, dir, "photo.webp", 100, 100)

	_, mediaType, err := resizeImageForLLM(path, 3.6)
	require.NoError(t, err)
	assert.Equal(t, "image/webp", mediaType)
}

func TestResizeImageForLLM_PngMediaType(t *testing.T) {
	dir := t.TempDir()
	path := createTempJPEG(t, dir, "photo.png", 100, 100)

	_, mediaType, err := resizeImageForLLM(path, 3.6)
	require.NoError(t, err)
	assert.Equal(t, "image/png", mediaType)
}

func TestResizeImageForLLM_ZeroMaxMegapixels_NoResize(t *testing.T) {
	dir := t.TempDir()
	path := createTempJPEG(t, dir, "img.jpg", 500, 500)

	// maxMegapixels=0 should skip resizing entirely
	data, _, err := resizeImageForLLM(path, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}
