package helpers

import (
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
)

// This file pins the observable output of the gallery DTO builders, which run on
// every GET /api/gallery request. Optimizations to these helpers must keep these
// assertions green without being relaxed.

// galleryFixturePaths vary in depth so FileName/DirPath derivation is exercised.
var galleryFixturePaths = []string{
	"/gallery/2024/album/photo-01.jpg",
	"/gallery/2024/album/sub/photo-02.png",
	"/gallery/trip/day-1/DSC_0003.jpeg",
	"/gallery/trip/day-2/nested/deep/DSC_0004.webp",
}

// makeGalleryFiles builds n deterministic ImageFile fixtures.
func makeGalleryFiles(n int) []domain.ImageFile {
	files := make([]domain.ImageFile, n)
	for i := range files {
		files[i] = domain.ImageFile{
			ID:      uint(i + 1),
			Path:    galleryFixturePaths[i%len(galleryFixturePaths)],
			Size:    1536 + int64(i)*7919,
			Hash:    "deadbeef",
			ModTime: time.Date(2024, 3, 7, 15, 4, 5, i%60, time.UTC),
		}
	}
	return files
}

// TestFormatSizeCharacterization pins the human-readable size formatting and its
// unit boundaries.
func TestFormatSizeCharacterization(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		if got := FormatSize(tt.size); got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

// TestBuildGalleryImageDTOCharacterization pins every derived field of the
// gallery DTO for a single file.
func TestBuildGalleryImageDTOCharacterization(t *testing.T) {
	modTime := time.Date(2024, 3, 7, 15, 4, 5, 0, time.UTC)
	f := domain.ImageFile{
		ID:      42,
		Path:    "/gallery/2024/album/sub/photo.jpg",
		Size:    1536,
		Hash:    "deadbeef",
		ModTime: modTime,
	}

	got := BuildGalleryImageDTO(&f)

	if got.ID != 42 {
		t.Errorf("ID = %d, want 42", got.ID)
	}
	if got.Path != "/gallery/2024/album/sub/photo.jpg" {
		t.Errorf("Path = %q, want the source path", got.Path)
	}
	if got.FileName != "photo.jpg" {
		t.Errorf("FileName = %q, want %q", got.FileName, "photo.jpg")
	}
	if got.DirPath != "/gallery/2024/album/sub" {
		t.Errorf("DirPath = %q, want %q", got.DirPath, "/gallery/2024/album/sub")
	}
	if got.Size != 1536 {
		t.Errorf("Size = %d, want 1536", got.Size)
	}
	if got.SizeHuman != "1.5 KB" {
		t.Errorf("SizeHuman = %q, want %q", got.SizeHuman, "1.5 KB")
	}
	if want := modTime.Format(DateTimeFormat); got.ModTime != want {
		t.Errorf("ModTime = %q, want %q", got.ModTime, want)
	}
}

// TestBuildFileDTOCharacterization pins every derived field of the plain file DTO.
func TestBuildFileDTOCharacterization(t *testing.T) {
	modTime := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	f := domain.ImageFile{
		ID:      7,
		Path:    "deep/nested/file.png",
		Size:    10,
		ModTime: modTime,
	}

	got := BuildFileDTO(&f)

	if got.ID != 7 {
		t.Errorf("ID = %d, want 7", got.ID)
	}
	if got.Path != "deep/nested/file.png" {
		t.Errorf("Path = %q, want the source path", got.Path)
	}
	if got.FileName != "file.png" {
		t.Errorf("FileName = %q, want %q", got.FileName, "file.png")
	}
	if got.DirPath != "deep/nested" {
		t.Errorf("DirPath = %q, want %q", got.DirPath, "deep/nested")
	}
	if want := modTime.Format(DateTimeFormat); got.ModTime != want {
		t.Errorf("ModTime = %q, want %q", got.ModTime, want)
	}
}

// TestBuildGalleryImageDTOsMatchesPerElementBuilder is the regression guard for
// the batch builder: the result must have the same length, preserve input order,
// and be element-wise identical to building each DTO individually.
func TestBuildGalleryImageDTOsMatchesPerElementBuilder(t *testing.T) {
	for _, n := range []int{0, 1, 5, 200} {
		files := makeGalleryFiles(n)

		got := BuildGalleryImageDTOs(files)
		if len(got) != n {
			t.Fatalf("BuildGalleryImageDTOs(len %d) returned %d elements", n, len(got))
		}

		for i := range files {
			want := BuildGalleryImageDTO(&files[i])
			if got[i] != want {
				t.Errorf("n=%d element %d = %+v, want %+v", n, i, got[i], want)
			}
		}
	}
}
