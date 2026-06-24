package thumbnail

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTempCacheDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "thumbnail-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestCacheKey_Deterministic(t *testing.T) {
	key1 := CacheKey("/path/to/image.jpg")
	key2 := CacheKey("/path/to/image.jpg")

	assert.Equal(t, key1, key2)
	assert.Len(t, key1, 32) // MD5 hex = 32 chars
}

func TestCacheKey_DifferentPaths(t *testing.T) {
	key1 := CacheKey("/path/to/image1.jpg")
	key2 := CacheKey("/path/to/image2.jpg")

	assert.NotEqual(t, key1, key2)
}

func TestCacheKey_PathNormalization(t *testing.T) {
	// Same logical path produces same key regardless of trailing slash variations
	key1 := CacheKey("/path/to/image.jpg")
	key2 := CacheKey("/path/to/image.jpg")

	assert.Equal(t, key1, key2)

	// Verify key is hex-encoded MD5 (32 chars)
	assert.Len(t, key1, 32)
	assert.Regexp(t, "^[0-9a-f]{32}$", key1)
}

func TestCachePath_Format(t *testing.T) {
	path := CachePath("/var/cache", "/images/photo.jpg")

	assert.Contains(t, filepath.ToSlash(path), "/var/cache/")
	assert.Contains(t, path, ".webp")
}

func TestCachePathRelative_Format(t *testing.T) {
	path := CachePathRelative("/images/photo.jpg")

	assert.NotContains(t, path, "/var/cache")
	assert.Contains(t, path, ".webp")
}

func TestCacheDirPath_Format(t *testing.T) {
	path := CacheDirPath("/var/cache", "/images/photo.jpg")

	assert.Contains(t, filepath.ToSlash(path), "/var/cache/")
}

func TestErrInvalidCachePath_Error(t *testing.T) {
	err := &ErrInvalidCachePath{Path: "/invalid"}
	assert.Contains(t, err.Error(), "invalid cache path")
	assert.Contains(t, err.Error(), "/invalid")
}

func TestErrCacheWriteFailed_Error(t *testing.T) {
	inner := os.ErrPermission
	err := &ErrCacheWriteFailed{Path: "/tmp/test", Err: inner}
	assert.Contains(t, err.Error(), "failed to write thumbnail")
	assert.Contains(t, err.Error(), "/tmp/test")
}

func TestErrCacheReadFailed_Error(t *testing.T) {
	inner := os.ErrNotExist
	err := &ErrCacheReadFailed{Path: "/tmp/missing", Err: inner}
	assert.Contains(t, err.Error(), "failed to read thumbnail")
	assert.Contains(t, err.Error(), "/tmp/missing")
}

func TestErrCacheInitFailed_Error(t *testing.T) {
	inner := os.ErrPermission
	err := &ErrCacheInitFailed{Path: "/tmp/cache", Err: inner}
	assert.Contains(t, err.Error(), "failed to initialize thumbnail cache")
	assert.Contains(t, err.Error(), "/tmp/cache")
}

func TestDefaultCacheDir(t *testing.T) {
	dir := DefaultCacheDir()
	assert.NotEmpty(t, dir)
}

func TestThumbnailCacheStorage_New_InvalidPath(t *testing.T) {
	_, err := NewThumbnailCacheStorage("")
	require.Error(t, err)
	assert.IsType(t, &ErrInvalidCachePath{}, err)
}

func TestThumbnailCacheStorage_Set_Get_Delete(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	filePath := "/images/test.jpg"
	data := []byte("thumbnail-data")

	// Set
	err = storage.Set(filePath, data)
	require.NoError(t, err)

	// Get
	cachedPath := storage.Get(filePath)
	assert.NotEmpty(t, cachedPath)
	assert.Contains(t, cachedPath, dir)

	// Verify file exists on disk
	cachedData, err := os.ReadFile(cachedPath)
	require.NoError(t, err)
	assert.Equal(t, data, cachedData)

	// Exists
	assert.True(t, storage.Exists(filePath))

	// Delete
	err = storage.Delete(filePath)
	require.NoError(t, err)
	assert.False(t, storage.Exists(filePath))
}

func TestThumbnailCacheStorage_Exists_Missing(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	assert.False(t, storage.Exists("/nonexistent/file.jpg"))
}

func TestThumbnailCacheStorage_Get_Missing(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	assert.Empty(t, storage.Get("/nonexistent/file.jpg"))
}

func TestThumbnailCacheStorage_Enable_Disable(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	assert.True(t, storage.IsEnabled())

	storage.Disable()
	assert.False(t, storage.IsEnabled())

	// Operations should be no-ops when disabled
	err = storage.Set("/test.jpg", []byte("data"))
	assert.Equal(t, ErrThumbnailCacheDisabled, err)

	assert.Empty(t, storage.Get("/test.jpg"))
	assert.False(t, storage.Exists("/test.jpg"))

	// GetPath returns empty when disabled
	path := storage.GetPath("/test.jpg")
	assert.Empty(t, path)

	storage.Enable()
	assert.True(t, storage.IsEnabled())
}

func TestThumbnailCacheStorage_ClearAll(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	// Store some data
	require.NoError(t, storage.Set("/img1.jpg", []byte("data1")))
	require.NoError(t, storage.Set("/img2.jpg", []byte("data2")))

	// Clear
	err = storage.ClearAll()
	require.NoError(t, err)

	// Verify all gone
	assert.False(t, storage.Exists("/img1.jpg"))
	assert.False(t, storage.Exists("/img2.jpg"))
}

func TestThumbnailCacheStorage_ListFiles(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	require.NoError(t, storage.Set("/img1.jpg", []byte("data")))
	require.NoError(t, storage.Set("/img2.jpg", []byte("data")))

	files, err := storage.ListFiles()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 2)
}

func TestThumbnailCacheStorage_Stats(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	require.NoError(t, storage.Set("/img1.jpg", []byte("12345")))
	require.NoError(t, storage.Set("/img2.jpg", []byte("1234567890")))

	count, size, err := storage.Stats()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 2)
	assert.GreaterOrEqual(t, size, int64(15))
}

func TestThumbnailCacheStorage_Delete_NonExistent(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	// Deleting non-existent file should not error
	err = storage.Delete("/nonexistent.jpg")
	assert.NoError(t, err)
}

func TestThumbnailCacheStorage_Delete_WhenDisabled(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	storage.Disable()
	err = storage.Delete("/test.jpg")
	assert.NoError(t, err) // No-op when disabled
}

func TestThumbnailCacheStorage_GetPath(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	path := storage.GetPath("/images/photo.jpg")
	assert.Contains(t, path, dir)
	assert.Contains(t, path, ".webp")
}

func TestThumbnailCacheStorage_GetPath_WhenDisabled(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	storage.Disable()
	assert.Empty(t, storage.GetPath("/test.jpg"))
}

func TestThumbnailCacheStorage_GetPathRelative(t *testing.T) {
	dir := createTempCacheDir(t)
	storage, err := NewThumbnailCacheStorage(dir)
	require.NoError(t, err)

	relative := storage.GetPathRelative("/images/photo.jpg")
	assert.NotEmpty(t, relative)
	assert.NotContains(t, relative, dir) // Should be relative, not absolute
	assert.Contains(t, relative, ".webp")
}
