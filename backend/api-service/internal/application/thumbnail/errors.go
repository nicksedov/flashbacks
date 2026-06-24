package thumbnail

import "fmt"

// ErrThumbnailNotFound ошибка - миниатюра не найдена в кэше
var ErrThumbnailNotFound = fmt.Errorf("thumbnail not found in cache")

// ErrThumbnailCacheDisabled ошибка - кэш миниатюр отключен
var ErrThumbnailCacheDisabled = fmt.Errorf("thumbnail cache is disabled")

// ErrInvalidCachePath ошибка - неверный путь кэша
type ErrInvalidCachePath struct {
	Path string
}

func (e *ErrInvalidCachePath) Error() string {
	return fmt.Sprintf("invalid cache path: %s", e.Path)
}

// ErrCacheWriteFailed ошибка записи в кэш
type ErrCacheWriteFailed struct {
	Path string
	Err  error
}

func (e *ErrCacheWriteFailed) Error() string {
	return fmt.Sprintf("failed to write thumbnail to cache %s: %v", e.Path, e.Err)
}

// ErrCacheReadFailed ошибка чтения из кэша
type ErrCacheReadFailed struct {
	Path string
	Err  error
}

func (e *ErrCacheReadFailed) Error() string {
	return fmt.Sprintf("failed to read thumbnail from cache %s: %v", e.Path, e.Err)
}

// ErrCacheInitFailed ошибка инициализации кэша
type ErrCacheInitFailed struct {
	Path string
	Err  error
}

func (e *ErrCacheInitFailed) Error() string {
	return fmt.Sprintf("failed to initialize thumbnail cache at %s: %v", e.Path, e.Err)
}
