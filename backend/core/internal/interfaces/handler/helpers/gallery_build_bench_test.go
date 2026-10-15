package helpers

import (
	"testing"

	"github.com/flashbacks/core/internal/interfaces/dto"
)

// Benchmarks for the gallery DTO builders on the GET /api/gallery read path.
//
//	cd backend/core && go test ./internal/interfaces/handler/helpers/ -bench=. -benchmem -count=5
//
// Both report allocations so a change that trades CPU for memory churn (or vice
// versa) is visible in the before/after diff.

// sinkGalleryDTOs prevents the compiler from eliminating the work under
// measurement.
var sinkGalleryDTOs []dto.GalleryImageDTO

// sinkFileDTO prevents the compiler from eliminating the work under measurement.
var sinkFileDTO dto.FileDTO

func benchBuildGalleryImageDTOs(b *testing.B, n int) {
	b.Helper()
	b.ReportAllocs()
	files := makeGalleryFiles(n)
	b.ResetTimer()
	for b.Loop() {
		sinkGalleryDTOs = BuildGalleryImageDTOs(files)
	}
}

// BenchmarkBuildGalleryImageDTOsDefaultPage matches the default page size (50).
func BenchmarkBuildGalleryImageDTOsDefaultPage(b *testing.B) {
	benchBuildGalleryImageDTOs(b, 50)
}

// BenchmarkBuildGalleryImageDTOsMaxPage matches MaxPageSizeFlexible (200).
func BenchmarkBuildGalleryImageDTOsMaxPage(b *testing.B) {
	benchBuildGalleryImageDTOs(b, 200)
}

// BenchmarkBuildGalleryImageDTOsFixedLargest matches the largest fixed page size (500).
func BenchmarkBuildGalleryImageDTOsFixedLargest(b *testing.B) {
	benchBuildGalleryImageDTOs(b, 500)
}

func BenchmarkBuildGalleryImageDTOSingle(b *testing.B) {
	b.ReportAllocs()
	files := makeGalleryFiles(1)
	b.ResetTimer()
	for b.Loop() {
		sinkGalleryDTOs = []dto.GalleryImageDTO{BuildGalleryImageDTO(&files[0])}
	}
}

func BenchmarkBuildFileDTO(b *testing.B) {
	b.ReportAllocs()
	files := makeGalleryFiles(1)
	b.ResetTimer()
	for b.Loop() {
		sinkFileDTO = BuildFileDTO(&files[0])
	}
}

func BenchmarkFormatSize(b *testing.B) {
	b.ReportAllocs()
	sizes := []int64{512, 1536, 98304, 1048576, 4294967296}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		sinkFormatSize = FormatSize(sizes[i%len(sizes)])
	}
}

// sinkFormatSize prevents the compiler from eliminating the work under measurement.
var sinkFormatSize string
