package thumbnail

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/flashbacks/core/internal/domain"
)

// backfillMarkerName is written into the active cache directory once the
// orientation backfill completes, so later starts skip the work.
const backfillMarkerName = ".orientation-backfill.v1"

// ImageFileLister lists indexed image files for the orientation backfill.
type ImageFileLister interface {
	ListAll() ([]domain.ImageFile, error)
}

// RotationBackfill regenerates cached thumbnails for photos that carry a
// non-trivial EXIF orientation, correcting thumbnails cached before
// orientation-aware generation existed.
type RotationBackfill struct {
	provider ThumbnailProvider
	lister   ImageFileLister
}

// NewRotationBackfill creates a RotationBackfill.
func NewRotationBackfill(provider ThumbnailProvider, lister ImageFileLister) *RotationBackfill {
	return &RotationBackfill{provider: provider, lister: lister}
}

// Run performs one backfill pass. It is a no-op when the provider or lister is
// missing, the cache is disabled, or the marker for the active cache already
// exists. Per-file failures are logged and do not abort the pass.
func (b *RotationBackfill) Run(ctx context.Context) error {
	if b == nil || b.provider == nil || b.lister == nil {
		return nil
	}
	if !b.provider.IsEnabled() {
		return nil
	}

	stats, err := b.provider.GetStats()
	if err != nil {
		return err
	}
	if stats == nil || stats.CacheDir == "" {
		return nil
	}

	marker := filepath.Join(stats.CacheDir, backfillMarkerName)
	if _, err := os.Stat(marker); err == nil {
		return nil
	}

	files, err := b.lister.ListAll()
	if err != nil {
		return err
	}

	regenerated := 0
	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !needsOrientationBackfill(file.Path) {
			continue
		}

		if err := b.provider.Invalidate(file.Path); err != nil {
			log.Printf("orientation backfill: invalidate %s: %v", file.Path, err)
			continue
		}
		if _, err := b.provider.GetOrGenerate(file.Path); err != nil {
			log.Printf("orientation backfill: regenerate %s: %v", file.Path, err)
			continue
		}
		regenerated++
	}

	if err := os.WriteFile(marker, []byte("ok\n"), 0644); err != nil {
		return err
	}
	if regenerated > 0 {
		log.Printf("orientation backfill: regenerated %d thumbnails", regenerated)
	}
	return nil
}

// needsOrientationBackfill reports whether the file carries an EXIF orientation
// that requires the thumbnail to be regenerated (3, 6 or 8).
func needsOrientationBackfill(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	switch readExifOrientation(file) {
	case 3, 6, 8:
		return true
	default:
		return false
	}
}
