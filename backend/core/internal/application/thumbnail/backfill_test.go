package thumbnail

import (
	"context"
	"testing"

	"github.com/flashbacks/core/internal/domain"
)

type fakeProvider struct {
	cacheDir    string
	invalidated []string
	generated   []string
}

func (f *fakeProvider) Start() error { return nil }

func (f *fakeProvider) GetOrGenerate(path string) (string, error) {
	f.generated = append(f.generated, path)
	return "data:image/webp;base64,eA==", nil
}

func (f *fakeProvider) HasThumbnail(string) bool { return false }

func (f *fakeProvider) Invalidate(path string) error {
	f.invalidated = append(f.invalidated, path)
	return nil
}

func (f *fakeProvider) InvalidateAll() error { return nil }

func (f *fakeProvider) Warmup([]string) error { return nil }

func (f *fakeProvider) GetStats() (*Stats, error) {
	return &Stats{CacheDir: f.cacheDir, Enabled: true, Initialized: true}, nil
}

func (f *fakeProvider) Enable() {}

func (f *fakeProvider) Disable() {}

func (f *fakeProvider) IsEnabled() bool { return true }

func (f *fakeProvider) UpdateCachePath(string) error { return nil }

type fakeLister struct {
	files []domain.ImageFile
}

func (f *fakeLister) ListAll() ([]domain.ImageFile, error) { return f.files, nil }

func newFileLister(paths ...string) *fakeLister {
	lister := &fakeLister{}
	for _, path := range paths {
		lister.files = append(lister.files, domain.ImageFile{Path: path})
	}
	return lister
}

func TestRotationBackfill_RegeneratesRotated(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeJPEGWithOrientation(t, dir, 40, 30, 3),
		writeJPEGWithOrientation(t, dir, 40, 30, 6),
		writeJPEGWithOrientation(t, dir, 40, 30, 8),
		writeJPEGWithOrientation(t, dir, 40, 30, 1),
	}

	provider := &fakeProvider{cacheDir: t.TempDir()}
	backfill := NewRotationBackfill(provider, newFileLister(paths...))

	if err := backfill.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(provider.generated) != 3 {
		t.Fatalf("expected 3 regenerated thumbnails, got %d: %v", len(provider.generated), provider.generated)
	}
}

func TestRotationBackfill_Idempotent(t *testing.T) {
	dir := t.TempDir()
	paths := []string{writeJPEGWithOrientation(t, dir, 40, 30, 6)}

	provider := &fakeProvider{cacheDir: t.TempDir()}
	backfill := NewRotationBackfill(provider, newFileLister(paths...))

	if err := backfill.Run(context.Background()); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first := len(provider.generated)
	if first != 1 {
		t.Fatalf("expected 1 generated thumbnail on the first run, got %d", first)
	}

	if err := backfill.Run(context.Background()); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(provider.generated) != first {
		t.Fatalf("expected no regeneration on the second run, got %d total", len(provider.generated))
	}
}

func TestRotationBackfill_SkipsUnrotated(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeJPEGWithOrientation(t, dir, 40, 30, 1),
		writeJPEGWithOrientation(t, dir, 40, 30, 0),
	}

	provider := &fakeProvider{cacheDir: t.TempDir()}
	backfill := NewRotationBackfill(provider, newFileLister(paths...))

	if err := backfill.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(provider.invalidated) != 0 || len(provider.generated) != 0 {
		t.Fatalf("expected no work for unrotated photos, invalidated=%v generated=%v",
			provider.invalidated, provider.generated)
	}
}
