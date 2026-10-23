package thumbnail

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/deepteams/webp"
	"github.com/disintegration/imaging"
)

// Config holds the ThumbnailService configuration.
type Config struct {
	CacheDir      string
	MaxSize       int // Maximum thumbnail dimension in pixels
	Quality       int // Compression quality (0-100)
	Enabled       bool
	Format        string // "webp" or "jpeg"
	CacheTTL      time.Duration
	PreloadOnScan bool
}

// Service manages thumbnail caching.
type Service struct {
	cfg         *Config
	storage     *ThumbnailCacheStorage
	mu          sync.RWMutex
	stats       ThumbnailStats
	initialized bool
}

// ThumbnailStats describes thumbnail cache statistics.
type ThumbnailStats struct {
	TotalSize   int64  `json:"totalSize"`
	TotalFiles  int    `json:"totalFiles"`
	CacheDir    string `json:"cacheDir"`
	Enabled     bool   `json:"enabled"`
	Initialized bool   `json:"initialized"`
}

// DefaultCacheDir returns the default thumbnail cache directory (Linux XDG compatible).
func DefaultCacheDir() string {
	home := os.Getenv("HOME")
	// In Docker/Alpine HOME may be "/" or empty - fall back to /tmp in that case
	if home == "" || home == "/" {
		return filepath.Join(os.TempDir(), "image-tool", "thumbnails")
	}
	return filepath.Join(home, ".cache", "image-tool", "thumbnails")
}

// NewService creates a new ThumbnailService.
func NewService(cfg *Config) (*Service, error) {
	if cfg == nil {
		cfg = &Config{
			CacheDir:      DefaultCacheDir(),
			MaxSize:       ThumbnailMaxSize,
			Quality:       ThumbnailQuality,
			Enabled:       true,
			Format:        ThumbnailFormat,
			CacheTTL:      30 * 24 * time.Hour, // 30 days
			PreloadOnScan: true,
		}
	}

	if cfg.CacheDir == "" {
		cfg.CacheDir = DefaultCacheDir()
	}

	storage, err := NewThumbnailCacheStorage(cfg.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create thumbnail storage: %w", err)
	}

	s := &Service{
		cfg:         cfg,
		storage:     storage,
		initialized: true,
	}

	// Collect statistics during initialization
	s.updateStats()

	return s, nil
}

// Start initializes the service (called on application startup).
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.Enabled {
		return nil
	}

	// Make sure the cache directory structure exists
	if err := s.storage.initStructure(); err != nil {
		return fmt.Errorf("failed to initialize thumbnail cache: %w", err)
	}

	s.initialized = true
	s.updateStats()
	return nil
}

// Stop stops the service (called on application shutdown).
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.initialized = false
}

// IsEnabled reports whether the thumbnail cache is enabled and initialized.
func (s *Service) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Enabled && s.initialized
}

// GetThumbnailPath returns the cached thumbnail path for the given file ("" when absent).
func (s *Service) GetThumbnailPath(filePath string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.cfg.Enabled || !s.initialized {
		return ""
	}

	path := s.storage.Get(filePath)
	return path
}

// HasThumbnail reports whether a thumbnail for the file exists in the cache.
func (s *Service) HasThumbnail(filePath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.cfg.Enabled || !s.initialized {
		return false
	}

	return s.storage.Exists(filePath)
}

// GetOrGenerate obtains a thumbnail from cache or generates it.
// Uses double-checked locking: RLock for cache hit, Lock only for cache miss.
func (s *Service) GetOrGenerate(filePath string) (string, error) {
	if !s.cfg.Enabled || !s.initialized {
		return "", ErrThumbnailCacheDisabled
	}

	// Fast path: check cache under read lock
	s.mu.RLock()
	cachedPath := s.storage.Get(filePath)
	s.mu.RUnlock()

	if cachedPath != "" {
		data, err := os.ReadFile(cachedPath)
		if err != nil {
			// Cache file lost; fall through to write-lock path
			s.mu.Lock()
			s.storage.Delete(filePath)
			s.mu.Unlock()
		} else {
			s.mu.Lock()
			s.stats.TotalFiles++
			s.mu.Unlock()
			return s.encodeToDataURL(data), nil
		}
	}

	// Slow path: acquire write lock for generation
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.Enabled || !s.initialized {
		return "", ErrThumbnailCacheDisabled
	}

	// Double-check after acquiring write lock
	cachedPath = s.storage.Get(filePath)
	if cachedPath != "" {
		data, err := os.ReadFile(cachedPath)
		if err == nil {
			s.stats.TotalFiles++
			return s.encodeToDataURL(data), nil
		}
		s.storage.Delete(filePath)
	}

	// Generate new thumbnail
	encodedData, err := s.generateThumbnail(filePath)
	if err != nil {
		return "", err
	}

	// Save to cache
	if err := s.storage.Set(filePath, encodedData); err != nil {
		return "", err
	}

	s.stats.TotalFiles++
	s.stats.TotalSize += int64(len(encodedData))

	return s.encodeToDataURL(encodedData), nil
}

// encodeToDataURL encodes binary data as a data URL with the configured MIME type.
func (s *Service) encodeToDataURL(data []byte) string {
	mimeType := "image/webp"
	switch s.cfg.Format {
	case "jpeg":
		mimeType = "image/jpeg"
	case "png":
		mimeType = "image/png"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// GenerateThumbnail generates a thumbnail for the file without caching it.
func (s *Service) GenerateThumbnail(filePath string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.generateThumbnail(filePath)
}

// generateThumbnail is the internal thumbnail generation routine.
func (s *Service) generateThumbnail(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	orientation := readExifOrientation(file)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to rewind image %s: %w", filePath, err)
	}

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image %s: %w", filePath, err)
	}

	img = applyOrientation(img, orientation)

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var newWidth, newHeight int
	if width >= height {
		newWidth = s.cfg.MaxSize
		newHeight = 0
	} else {
		newWidth = 0
		newHeight = s.cfg.MaxSize
	}

	thumbnail := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	var buf bytes.Buffer
	switch s.cfg.Format {
	case "webp":
		if err := webp.Encode(&buf, thumbnail, &webp.Options{Quality: float32(s.cfg.Quality)}); err != nil {
			buf.Reset()
			if err := jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: s.cfg.Quality}); err != nil {
				return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
			}
		}
	case "jpeg":
		if err := jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: s.cfg.Quality}); err != nil {
			return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
		}
	case "png":
		if err := png.Encode(&buf, thumbnail); err != nil {
			return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// Invalidate removes the thumbnail of the given file from the cache.
func (s *Service) Invalidate(filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.Enabled || !s.initialized {
		return nil
	}

	return s.storage.Delete(filePath)
}

// saveThumbnailToCache stores a thumbnail in the cache (used by scan integration).
func (s *Service) saveThumbnailToCache(filePath string, thumbnailData []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.Enabled || !s.initialized {
		return ErrThumbnailCacheDisabled
	}

	return s.storage.Set(filePath, thumbnailData)
}

// InvalidateAll removes every thumbnail from the cache.
func (s *Service) InvalidateAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.Enabled {
		return nil
	}

	if err := s.storage.ClearAll(); err != nil {
		return err
	}

	s.stats = ThumbnailStats{}
	return nil
}

// Warmup pre-generates thumbnails for the given files.
func (s *Service) Warmup(imagePaths []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.Enabled || !s.initialized {
		return nil
	}

	for _, path := range imagePaths {
		if !s.storage.Exists(path) {
			encodedData, err := s.generateThumbnail(path)
			if err != nil {
				continue // Skip files that failed to generate
			}

			if err := s.storage.Set(path, encodedData); err != nil {
				continue
			}

			s.stats.TotalFiles++
			s.stats.TotalSize += int64(len(encodedData))
		}
	}

	return nil
}

// updateStats refreshes the in-memory cache statistics.
func (s *Service) updateStats() {
	if !s.initialized {
		return
	}

	count, size, err := s.storage.Stats()
	if err != nil {
		return
	}

	s.stats = ThumbnailStats{
		TotalFiles:  count,
		TotalSize:   size,
		CacheDir:    s.cfg.CacheDir,
		Enabled:     s.cfg.Enabled,
		Initialized: true,
	}
}

// Stats returns the current cache statistics.
func (s *Service) Stats() ThumbnailStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		log.Printf("Stats: service not initialized")
		return ThumbnailStats{}
	}

	// Always refresh from disk to ensure accuracy
	s.updateStats()

	log.Printf("Stats: returning %+v", s.stats)
	return s.stats
}

// GetStats returns the current cache statistics as a *Stats value,
// satisfying the ThumbnailProvider interface.
func (s *Service) GetStats() (*Stats, error) {
	st := s.Stats()
	return &Stats{
		TotalSize:   st.TotalSize,
		TotalFiles:  st.TotalFiles,
		CacheDir:    st.CacheDir,
		Enabled:     st.Enabled,
		Initialized: st.Initialized,
	}, nil
}

// UpdateCachePath switches the cache directory and moves the existing thumbnail files.
func (s *Service) UpdateCachePath(newPath string) error {
	if newPath == "" {
		return &ErrInvalidCachePath{Path: newPath}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldPath := s.cfg.CacheDir
	log.Printf("UpdateCachePath: old=%s, new=%s", oldPath, newPath)

	// Same path: only refresh the statistics
	if oldPath == newPath {
		log.Printf("UpdateCachePath: paths are the same, updating stats only")
		s.updateStats()
		log.Printf("UpdateCachePath: stats after update: %+v", s.stats)
		return nil
	}

	// Move files from the old storage to the new one
	if err := s.moveCacheTo(newPath); err != nil {
		log.Printf("UpdateCachePath: moveCacheTo failed: %v", err)
		return err
	}

	// Create the new storage and swap it in
	newStorage, err := NewThumbnailCacheStorage(newPath)
	if err != nil {
		log.Printf("UpdateCachePath: NewThumbnailCacheStorage failed: %v", err)
		return &ErrCacheInitFailed{Path: newPath, Err: err}
	}

	s.storage = newStorage
	s.cfg.CacheDir = newPath
	s.cfg.Enabled = true
	s.initialized = true

	s.updateStats()
	log.Printf("UpdateCachePath: success, stats: %+v", s.stats)
	return nil
}

// moveCacheTo moves the cached files to newPath.
func (s *Service) moveCacheTo(newPath string) error {
	oldPath := s.cfg.CacheDir

	// Nothing to move when the old path is empty
	if oldPath == "" {
		return nil
	}

	// Check whether the old directory exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		// Old directory missing: just create the new one
		if err := os.MkdirAll(newPath, 0755); err != nil {
			return &ErrCacheInitFailed{Path: newPath, Err: err}
		}
		return nil
	}

	// List the files in the current cache
	files, err := s.storage.ListFiles()
	if err != nil {
		// Failed to list files: continue with an empty set, the cache may be
		// empty or its directory structure may not exist yet
		files = []string{}
	}

	// Move each file
	for _, srcPath := range files {
		// Read the contents
		data, err := os.ReadFile(srcPath)
		if err != nil {
			// Skip unreadable files
			continue
		}

		// Write to the new location
		dstPath := filepath.Join(newPath, filepath.Base(filepath.Dir(srcPath)), filepath.Base(srcPath))
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			continue
		}

		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			continue
		}

		// Remove the old file after the copy succeeded
		os.Remove(srcPath)
	}

	// Remove the old cache tree (it should be empty after the move)
	s.cleanupEmptyDirs(oldPath)

	return nil
}

// cleanupEmptyDirs removes empty subdirectories.
func (s *Service) cleanupEmptyDirs(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			// Try to remove the empty directory
			os.Remove(path)
		}

		return nil
	})
}

// Enable turns the thumbnail cache on.
func (s *Service) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.Enabled = true
	if s.storage != nil {
		s.storage.Enable()
	}
}

// Disable turns the thumbnail cache off.
func (s *Service) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.Enabled = false
	if s.storage != nil {
		s.storage.Disable()
	}
}
