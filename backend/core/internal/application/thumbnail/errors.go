package thumbnail

import "fmt"

// ErrThumbnailNotFound is returned when the thumbnail is missing from the cache.
var ErrThumbnailNotFound = fmt.Errorf("thumbnail not found in cache")

// ErrThumbnailCacheDisabled is returned when the thumbnail cache is disabled.
var ErrThumbnailCacheDisabled = fmt.Errorf("thumbnail cache is disabled")

// ErrInvalidCachePath is returned for an unusable cache directory path.
type ErrInvalidCachePath struct {
	Path string
}

func (e *ErrInvalidCachePath) Error() string {
	return fmt.Sprintf("invalid cache path: %s", e.Path)
}

// ErrCacheWriteFailed is returned when writing to the cache fails.
type ErrCacheWriteFailed struct {
	Path string
	Err  error
}

func (e *ErrCacheWriteFailed) Error() string {
	return fmt.Sprintf("failed to write thumbnail to cache %s: %v", e.Path, e.Err)
}

// ErrCacheReadFailed is returned when reading from the cache fails.
type ErrCacheReadFailed struct {
	Path string
	Err  error
}

func (e *ErrCacheReadFailed) Error() string {
	return fmt.Sprintf("failed to read thumbnail from cache %s: %v", e.Path, e.Err)
}

// ErrCacheInitFailed is returned when the cache cannot be initialized.
type ErrCacheInitFailed struct {
	Path string
	Err  error
}

func (e *ErrCacheInitFailed) Error() string {
	return fmt.Sprintf("failed to initialize thumbnail cache at %s: %v", e.Path, e.Err)
}
