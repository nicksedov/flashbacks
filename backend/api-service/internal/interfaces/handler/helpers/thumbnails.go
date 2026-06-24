package helpers

import (
	"sync"

	"github.com/flashbacks/api-service/internal/application/imaging"
	"github.com/flashbacks/api-service/internal/application/thumbnail"
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
