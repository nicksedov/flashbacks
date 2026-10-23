package thumbnail

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// ThumbnailCacheSubdirs is the number of subdirectories used to spread files evenly (single-level layout)
	ThumbnailCacheSubdirs = 256
	// ThumbnailFormat is the format of the stored thumbnails
	ThumbnailFormat = "webp"
	// ThumbnailQuality is the thumbnail compression quality (0-100)
	ThumbnailQuality = 80
	// ThumbnailMaxSize is the maximum thumbnail dimension in pixels
	ThumbnailMaxSize = 320
)

// CacheKey derives the cache key from the file path.
func CacheKey(filePath string) string {
	// Normalize the path: replace backslashes with forward slashes for consistency
	normalizedPath := filepath.ToSlash(filePath)
	// MD5 hash of the path to get a fixed-length key
	hash := md5.Sum([]byte(normalizedPath))
	return hex.EncodeToString(hash[:])
}

// CachePath returns the absolute thumbnail path in the single-level cache layout.
func CachePath(cacheDir, filePath string) string {
	cacheKey := CacheKey(filePath)
	subdir := cacheKey[:2] // First 2 characters of the hash (00-ff = 256 buckets)
	fileName := cacheKey + "." + ThumbnailFormat

	return filepath.Join(cacheDir, subdir, fileName)
}

// CachePathRelative returns the thumbnail path relative to the cache root.
func CachePathRelative(filePath string) string {
	cacheKey := CacheKey(filePath)
	subdir := cacheKey[:2]
	fileName := cacheKey + "." + ThumbnailFormat

	// Return the relative path without the cache directory prefix
	return filepath.Join(subdir, fileName)
}

// CacheDirPath returns the cache subdirectory for the file hash.
func CacheDirPath(cacheDir, filePath string) string {
	cacheKey := CacheKey(filePath)
	subdir := cacheKey[:2]

	return filepath.Join(cacheDir, subdir)
}

// ThumbnailCacheStorage manages the on-disk thumbnail cache.
type ThumbnailCacheStorage struct {
	cacheDir string
	mu       sync.RWMutex
	enabled  bool
}

// NewThumbnailCacheStorage creates a new thumbnail cache storage.
func NewThumbnailCacheStorage(cacheDir string) (*ThumbnailCacheStorage, error) {
	if cacheDir == "" {
		return nil, &ErrInvalidCachePath{Path: cacheDir}
	}

	// Make a relative path absolute
	if !filepath.IsAbs(cacheDir) {
		absPath, err := filepath.Abs(cacheDir)
		if err != nil {
			return nil, &ErrInvalidCachePath{Path: cacheDir}
		}
		cacheDir = absPath
	}

	storage := &ThumbnailCacheStorage{
		cacheDir: cacheDir,
		enabled:  true,
	}

	// Initialize the subdirectory structure
	if err := storage.initStructure(); err != nil {
		return nil, &ErrCacheInitFailed{Path: cacheDir, Err: err}
	}

	return storage, nil
}

// initStructure creates the single-level structure of 256 subdirectories.
func (tcs *ThumbnailCacheStorage) initStructure() error {
	tcs.mu.Lock()
	defer tcs.mu.Unlock()

	// Create the cache root directory
	if err := os.MkdirAll(tcs.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create 256 subdirectories to spread the files evenly
	// Names: 00, 01, 02, ..., ff (hexadecimal)
	for i := 0; i < ThumbnailCacheSubdirs; i++ {
		subdirName := fmt.Sprintf("%02x", i)
		subdirPath := filepath.Join(tcs.cacheDir, subdirName)

		if err := os.MkdirAll(subdirPath, 0755); err != nil {
			return fmt.Errorf("failed to create subdirectory %s: %w", subdirPath, err)
		}
	}

	return nil
}

// Enable turns the thumbnail cache on.
func (tcs *ThumbnailCacheStorage) Enable() {
	tcs.mu.Lock()
	defer tcs.mu.Unlock()
	tcs.enabled = true
}

// Disable turns the thumbnail cache off.
func (tcs *ThumbnailCacheStorage) Disable() {
	tcs.mu.Lock()
	defer tcs.mu.Unlock()
	tcs.enabled = false
}

// IsEnabled reports whether the cache is enabled.
func (tcs *ThumbnailCacheStorage) IsEnabled() bool {
	tcs.mu.RLock()
	defer tcs.mu.RUnlock()
	return tcs.enabled
}

// GetPath returns the absolute thumbnail path for the given image path.
func (tcs *ThumbnailCacheStorage) GetPath(filePath string) string {
	if !tcs.enabled {
		return ""
	}
	return CachePath(tcs.cacheDir, filePath)
}

// GetPathRelative returns the thumbnail path relative to the cache root.
func (tcs *ThumbnailCacheStorage) GetPathRelative(filePath string) string {
	if !tcs.enabled {
		return ""
	}
	return CachePathRelative(filePath)
}

// Exists reports whether the thumbnail exists in the cache.
func (tcs *ThumbnailCacheStorage) Exists(filePath string) bool {
	if !tcs.enabled {
		return false
	}

	path := CachePath(tcs.cacheDir, filePath)
	tcs.mu.RLock()
	defer tcs.mu.RUnlock()

	_, err := os.Stat(path)
	return err == nil
}

// Get returns the thumbnail path when it exists, an empty string otherwise.
func (tcs *ThumbnailCacheStorage) Get(filePath string) string {
	if !tcs.enabled {
		return ""
	}

	path := CachePath(tcs.cacheDir, filePath)
	tcs.mu.RLock()
	defer tcs.mu.RUnlock()

	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// Set stores the thumbnail in the cache.
func (tcs *ThumbnailCacheStorage) Set(filePath string, thumbnailData []byte) error {
	if !tcs.enabled {
		return ErrThumbnailCacheDisabled
	}

	tcs.mu.Lock()
	defer tcs.mu.Unlock()

	cachePath := CachePath(tcs.cacheDir, filePath)

	// Make sure the subdirectory exists
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ErrCacheWriteFailed{Path: filePath, Err: err}
	}

	// Write the thumbnail file
	if err := os.WriteFile(cachePath, thumbnailData, 0644); err != nil {
		return &ErrCacheWriteFailed{Path: filePath, Err: err}
	}

	return nil
}

// Delete removes the thumbnail from the cache.
func (tcs *ThumbnailCacheStorage) Delete(filePath string) error {
	if !tcs.enabled {
		return nil
	}

	tcs.mu.Lock()
	defer tcs.mu.Unlock()

	cachePath := CachePath(tcs.cacheDir, filePath)

	if err := os.Remove(cachePath); err != nil {
		if os.IsNotExist(err) {
			return nil // File is already gone
		}
		return &ErrCacheReadFailed{Path: filePath, Err: err}
	}

	return nil
}

// ClearAll wipes the whole thumbnail cache.
func (tcs *ThumbnailCacheStorage) ClearAll() error {
	tcs.mu.Lock()
	defer tcs.mu.Unlock()

	// Remove the whole cache tree
	return os.RemoveAll(tcs.cacheDir)
}

// Stats returns the cache statistics.
func (tcs *ThumbnailCacheStorage) Stats() (int, int64, error) {
	tcs.mu.RLock()
	defer tcs.mu.RUnlock()

	var count int
	var totalSize int64

	err := filepath.Walk(tcs.cacheDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil // Keep walking on errors
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), "."+ThumbnailFormat) {
			count++
			totalSize += info.Size()
		}

		return nil
	})

	if err != nil {
		return 0, 0, &ErrCacheReadFailed{Path: tcs.cacheDir, Err: err}
	}

	return count, totalSize, nil
}

// ListFiles returns the list of all files in the cache.
func (tcs *ThumbnailCacheStorage) ListFiles() ([]string, error) {
	tcs.mu.RLock()
	defer tcs.mu.RUnlock()

	var files []string

	err := filepath.Walk(tcs.cacheDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), "."+ThumbnailFormat) {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, &ErrCacheReadFailed{Path: tcs.cacheDir, Err: err}
	}

	return files, nil
}
