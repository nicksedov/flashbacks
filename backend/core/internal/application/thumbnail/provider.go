package thumbnail

// Stats holds summary statistics about the thumbnail cache.
type Stats struct {
	TotalSize   int64  `json:"totalSize"`
	TotalFiles  int    `json:"totalFiles"`
	CacheDir    string `json:"cacheDir"`
	Enabled     bool   `json:"enabled"`
	Initialized bool   `json:"initialized"`
}

// ThumbnailProvider is the unified interface for thumbnail generation and caching.
// thumbnail.Service is the canonical implementation (on-disk cache).
type ThumbnailProvider interface {
	Start() error
	GetOrGenerate(filePath string) (string, error)
	HasThumbnail(filePath string) bool
	Invalidate(filePath string) error
	InvalidateAll() error
	Warmup(imagePaths []string) error
	GetStats() (*Stats, error)
	Enable()
	Disable()
	IsEnabled() bool
	UpdateCachePath(newPath string) error
}
