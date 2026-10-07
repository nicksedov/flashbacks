package fixtures

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// CreateTempDir creates a temporary directory and registers cleanup with t.Cleanup.
func CreateTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "image-toolkit-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// CreateTestFile creates a file with the given content in the specified directory.
func CreateTestFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	return path
}

// CreateMinimalJPEG creates a minimal valid JPEG file with the given dimensions.
func CreateMinimalJPEG(t *testing.T, dir, name string, width, height int) string {
	t.Helper()
	path := filepath.Join(dir, name)

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create a simple image
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with a solid color
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create JPEG file: %v", err)
	}
	defer file.Close()

	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 75}); err != nil {
		t.Fatalf("failed to encode JPEG: %v", err)
	}

	return path
}

// CreateMultipleTestJPEGs creates N JPEG files with sequential names.
func CreateMultipleTestJPEGs(t *testing.T, dir string, count int) []string {
	t.Helper()
	paths := make([]string, count)
	for i := 0; i < count; i++ {
		name := "test_" + string(rune('0'+i)) + ".jpg"
		paths[i] = CreateMinimalJPEG(t, dir, name, 100, 100)
	}
	return paths
}

// DeleteTestFile deletes a file if it exists.
func DeleteTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to delete test file: %v", err)
	}
}
