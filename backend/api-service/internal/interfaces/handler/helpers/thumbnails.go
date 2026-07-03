package helpers

import (
	"sync"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/application/thumbnail"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
)

// ThumbnailBatch handles parallel thumbnail generation with service fallback.
type ThumbnailBatch struct {
	svc   *thumbnail.Service
	cache *imaging.ThumbnailCache
}

// NewThumbnailBatch creates a new ThumbnailBatch.
func NewThumbnailBatch(svc *thumbnail.Service, cache *imaging.ThumbnailCache) *ThumbnailBatch {
	return &ThumbnailBatch{svc: svc, cache: cache}
}

// TryGetCached attempts to retrieve a thumbnail from cache without generating.
// Returns the thumbnail data URL and true if cached, empty string and false otherwise.
func (tb *ThumbnailBatch) TryGetCached(filePath string) (string, bool) {
	if tb.svc != nil && tb.svc.HasThumbnail(filePath) {
		thumb, err := tb.svc.GetOrGenerate(filePath)
		if err == nil {
			return thumb, true
		}
	}
	return "", false
}

// Generate generates a single thumbnail with service fallback.
func (tb *ThumbnailBatch) Generate(filePath string) (string, error) {
	if tb.svc != nil {
		return tb.svc.GetOrGenerate(filePath)
	}
	return imaging.GenerateThumbnail(filePath, tb.cache)
}

// GenerateParallel generates thumbnails for multiple paths in parallel,
// calling setFn(index, thumbnail) for each successful generation.
// Uses the thumbnail service with fallback to basic generation.
func (tb *ThumbnailBatch) GenerateParallel(paths []string, setFn func(index int, thumb string)) {
	if len(paths) == 0 {
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, DefaultMaxWorkers)

	for i, path := range paths {
		if path == "" {
			continue
		}
		wg.Add(1)
		go func(idx int, filePath string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			thumb, err := tb.Generate(filePath)
			if err == nil {
				setFn(idx, thumb)
			}
		}(i, path)
	}
	wg.Wait()
}

// GenerateParallelAsync launches thumbnail generation in background goroutines
// without blocking. The setFn callback is called for each successfully generated
// thumbnail. Use this when the HTTP response should not wait for thumbnail generation.
func (tb *ThumbnailBatch) GenerateParallelAsync(paths []string, setFn func(index int, thumb string)) {
	if len(paths) == 0 {
		return
	}

	for i, path := range paths {
		if path == "" {
			continue
		}
		go func(idx int, filePath string) {
			thumb, err := tb.Generate(filePath)
			if err == nil {
				setFn(idx, thumb)
			}
		}(i, path)
	}
}

// GenerateThumbnailsForDTOs extracts paths from a slice of GalleryImageDTO,
// generates thumbnails in parallel, and sets the Thumbnail field on each DTO.
func (tb *ThumbnailBatch) GenerateThumbnailsForDTOs(dtos []dto.GalleryImageDTO) {
	paths := make([]string, len(dtos))
	for i, d := range dtos {
		paths[i] = d.Path
	}
	tb.GenerateParallel(paths, func(idx int, thumb string) {
		dtos[idx].Thumbnail = thumb
	})
}

// GenerateParallelBasic generates thumbnails using only the basic cache-based function
// (no service fallback). Used when the thumbnail service is not available.
func (tb *ThumbnailBatch) GenerateParallelBasic(paths []string, setFn func(index int, thumb string)) {
	if len(paths) == 0 {
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, DefaultMaxWorkers)

	for i, path := range paths {
		if path == "" {
			continue
		}
		wg.Add(1)
		go func(idx int, filePath string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			thumb, err := imaging.GenerateThumbnail(filePath, tb.cache)
			if err == nil {
				setFn(idx, thumb)
			}
		}(i, path)
	}
	wg.Wait()
}
