package service

import (
	"image"
	"image/color"
	"testing"

	"github.com/otiai10/gosseract/v2"
)

// Benchmarks for the pure (non-CGO) helpers on the /classify hot path.
//
// Run with the project-local CGO flags, otherwise the package cannot link:
//
//	. ./scripts/toolenv.sh
//	cd backend/ocr && go test ./internal/service/ -bench=. -benchmem -count=5
//
// Every benchmark reports allocations so an optimization that trades CPU for
// memory churn (or vice versa) is visible in the before/after diff.

// Package-level sinks prevent the compiler from eliminating the work under
// measurement.
var (
	sinkInt   int
	sinkGray  *image.Gray
	sinkEdges *image.Gray
	sinkLines []houghLine
	sinkInts  []int
	sinkBoxes []BoundingBox
	sinkF64   float64
)

// benchStepEdgeGray builds a gray image with a vertical step edge (left half
// black, right half white) plus a light horizontal band every 64 rows, so Canny
// and the Hough transform have non-trivial structure to work on.
func benchStepEdgeGray(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case x >= w/2:
				img.SetGray(x, y, color.Gray{Y: 255})
			case y%64 == 0:
				img.SetGray(x, y, color.Gray{Y: 200})
			}
		}
	}
	return img
}

// benchOpaqueNRGBA builds a uniform opaque image of the given size.
func benchOpaqueNRGBA(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 128, 128, 128, 255
	}
	return img
}

// benchWords are the synthetic OCR word strings, chosen to exercise every
// branch of countTokens (letters, digits, unit/currency symbols, blanks).
var benchWords = []string{"flashbacks", "45°", "$1,234.56", "3.14", "привет", "  ", "a", "-10"}

// benchOCRBoxes builds a slice of synthetic gosseract word boxes.
func benchOCRBoxes(n int) []gosseract.BoundingBox {
	boxes := make([]gosseract.BoundingBox, n)
	for i := range boxes {
		boxes[i] = gosseract.BoundingBox{
			Box:        image.Rect(i%400, i%300, i%400+90, i%300+24),
			Word:       benchWords[i%len(benchWords)],
			Confidence: float64(60 + i%40),
		}
	}
	return boxes
}

// benchDomainBoxes builds a slice of already-converted BoundingBox values, as
// consumed by the confidence-metric helpers.
func benchDomainBoxes(n int) []BoundingBox {
	boxes := make([]BoundingBox, n)
	for i := range boxes {
		boxes[i] = BoundingBox{
			X:          i % 400,
			Y:          i % 300,
			Width:      90,
			Height:     24,
			Word:       benchWords[i%len(benchWords)],
			Confidence: float64(60+i%40) / 100.0,
		}
	}
	return boxes
}

func BenchmarkCountTokens(b *testing.B) {
	b.ReportAllocs()
	inputs := []string{
		"flashbacks",
		"3.14 kg/m²",
		"$1,234.56",
		"45° 30′ 15″",
		"привет мир",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkInt = countTokens(inputs[i%len(inputs)])
	}
}

func BenchmarkConvertToGray1MP(b *testing.B) {
	b.ReportAllocs()
	src := benchOpaqueNRGBA(1024, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkGray = convertToGray(src, 224)
	}
}

func BenchmarkConvertToGray3MP(b *testing.B) {
	b.ReportAllocs()
	src := benchOpaqueNRGBA(2048, 1536)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkGray = convertToGray(src, 224)
	}
}

func BenchmarkPreprocessImage3MP(b *testing.B) {
	b.ReportAllocs()
	src := benchOpaqueNRGBA(2048, 1536)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkGray, _, _, _ = preprocessImage(src)
	}
}

func BenchmarkCannyEdgeDetection1MP(b *testing.B) {
	b.ReportAllocs()
	gray := benchStepEdgeGray(1024, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkEdges = cannyEdgeDetection(gray, 50, 150)
	}
}

func BenchmarkCannyEdgeDetection3MP(b *testing.B) {
	b.ReportAllocs()
	gray := benchStepEdgeGray(2048, 1536)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkEdges = cannyEdgeDetection(gray, 50, 150)
	}
}

func BenchmarkHoughLineTransform1MP(b *testing.B) {
	b.ReportAllocs()
	gray := benchStepEdgeGray(1024, 1024)
	edges := cannyEdgeDetection(gray, 50, 150)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkLines = houghLineTransform(edges, 64)
	}
}

func BenchmarkDetectSkewAngle1MP(b *testing.B) {
	b.ReportAllocs()
	gray := benchStepEdgeGray(1024, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkInts = detectSkewAngle(gray)
	}
}

func BenchmarkGenerateCandidateAngles(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkInts = generateCandidateAngles(float64(i%90) - 45)
	}
}

func BenchmarkFilterAndConvertBoxes(b *testing.B) {
	b.ReportAllocs()
	boxes := benchOCRBoxes(200)
	c := NewClassifier()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBoxes, sinkInt = c.filterAndConvertBoxes(boxes)
	}
}

func BenchmarkCalculateConfidenceMetrics(b *testing.B) {
	b.ReportAllocs()
	boxes := benchDomainBoxes(200)
	c := NewClassifier()
	total := 0
	for _, box := range boxes {
		total += countTokens(box.Word)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, sinkF64 = c.calculateConfidenceMetrics(boxes, total)
	}
}

func BenchmarkComputeDenseTextConfidence(b *testing.B) {
	b.ReportAllocs()
	boxes := benchDomainBoxes(200)
	total := 0
	for _, box := range boxes {
		total += countTokens(box.Word)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkF64 = computeDenseTextConfidence(boxes, total)
	}
}
