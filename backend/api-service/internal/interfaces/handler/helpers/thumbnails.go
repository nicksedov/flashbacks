package helpers

import (
	"sync"

	"github.com/flashbacks/api-service/internal/application/thumbnail"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
)

// ThumbnailBatch handles parallel thumbnail generation via the unified ThumbnailProvider.
type ThumbnailBatch struct {
	provider thumbnail.ThumbnailProvider
}

// NewThumbnailBatch creates a new ThumbnailBatch with the given provider.
func NewThumbnailBatch(provider thumbnail.ThumbnailProvider) *ThumbnailBatch {
	return &ThumbnailBatch{provider: provider}
}

// TryGetCached attempts to retrieve a thumbnail from cache without generating.
// Returns the thumbnail data URL and true if cached, empty string and false otherwise.
func (tb *ThumbnailBatch) TryGetCached(filePath string) (string, bool) {
	if tb.provider != nil && tb.provider.HasThumbnail(filePath) {
		thumb, err := tb.provider.GetOrGenerate(filePath)
		if err == nil {
			return thumb, true
		}
	}
	return "", false
}

// Generate generates a single thumbnail using the provider.
func (tb *ThumbnailBatch) Generate(filePath string) (string, error) {
	if tb.provider != nil {
		return tb.provider.GetOrGenerate(filePath)
	}
	return "", thumbnail.ErrThumbnailCacheDisabled
}

// GenerateParallel generates thumbnails for multiple paths in parallel,
// calling setFn(index, thumbnail) for each successful generation.
func (tb *ThumbnailBatch) GenerateParallel(paths []string, setFn func(index int, thumb string)) {
	if len(paths) == 0 || tb.provider == nil {
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

			thumb, err := tb.provider.GetOrGenerate(filePath)
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
	if len(paths) == 0 || tb.provider == nil {
		return
	}

	for i, path := range paths {
		if path == "" {
			continue
		}
		go func(idx int, filePath string) {
			thumb, err := tb.provider.GetOrGenerate(filePath)
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
