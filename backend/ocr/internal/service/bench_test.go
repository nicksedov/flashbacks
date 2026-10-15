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

// benchOpaqueNRGBA builds a uniform opaque image of the given size, matching the
// layout produced by imaging.Resize (the no-blur preprocessing path).
func benchOpaqueNRGBA(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 128, 128, 128, 255
	}
	return img
}

// benchOpaqueRGBA builds a uniform opaque image of the given size, matching the
// layout produced by bild's median blur (the blur preprocessing path).
func benchOpaqueRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
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
	for i := 0; b.Loop(); i++ {
		sinkInt = countTokens(inputs[i%len(inputs)])
	}
}

func BenchmarkConvertToGray1MP(b *testing.B) {
	b.ReportAllocs()
	src := benchOpaqueNRGBA(1024, 1024)
	b.ResetTimer()
	for b.Loop() {
		sinkGray = convertToGray(src, 224)
	}
}

func BenchmarkConvertToGray3MP(b *testing.B) {
	b.ReportAllocs()
	src := benchOpaqueNRGBA(2048, 1536)
	b.ResetTimer()
	for b.Loop() {
		sinkGray = convertToGray(src, 224)
	}
}

func BenchmarkConvertToGrayBlurPathRGBA3MP(b *testing.B) {
	b.ReportAllocs()
	src := benchOpaqueRGBA(2048, 1536)
	b.ResetTimer()
	for b.Loop() {
		sinkGray = convertToGray(src, 224)
	}
}

func BenchmarkPreprocessImage3MP(b *testing.B) {
	b.ReportAllocs()
	src := benchOpaqueNRGBA(2048, 1536)
	b.ResetTimer()
	for b.Loop() {
		sinkGray, _, _, _ = preprocessImage(src)
	}
}

func BenchmarkCannyEdgeDetection1MP(b *testing.B) {
	b.ReportAllocs()
	gray := benchStepEdgeGray(1024, 1024)
	b.ResetTimer()
	for b.Loop() {
		sinkEdges = cannyEdgeDetection(gray, 50, 150)
	}
}

func BenchmarkCannyEdgeDetection3MP(b *testing.B) {
	b.ReportAllocs()
	gray := benchStepEdgeGray(2048, 1536)
	b.ResetTimer()
	for b.Loop() {
		sinkEdges = cannyEdgeDetection(gray, 50, 150)
	}
}

func BenchmarkHoughLineTransform1MP(b *testing.B) {
	b.ReportAllocs()
	gray := benchStepEdgeGray(1024, 1024)
	edges := cannyEdgeDetection(gray, 50, 150)
	b.ResetTimer()
	for b.Loop() {
		sinkLines = houghLineTransform(edges, 64)
	}
}

func BenchmarkDetectSkewAngle1MP(b *testing.B) {
	b.ReportAllocs()
	gray := benchStepEdgeGray(1024, 1024)
	b.ResetTimer()
	for b.Loop() {
		sinkInts = detectSkewAngle(gray)
	}
}

func BenchmarkGenerateCandidateAngles(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		sinkInts = generateCandidateAngles(float64(i%90) - 45)
	}
}

func BenchmarkFilterAndConvertBoxes(b *testing.B) {
	b.ReportAllocs()
	boxes := benchOCRBoxes(200)
	c := NewClassifier()
	b.ResetTimer()
	for b.Loop() {
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
	for b.Loop() {
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
	for b.Loop() {
		sinkF64 = computeDenseTextConfidence(boxes, total)
	}
}

// benchHysteresisInput builds a suppressed-gradient buffer with a single strong
// seed on interior row 1 and, when chain > 0, a weak run of that length growing
// away from the seed. The weak run models a long faint edge touching a strong
// one.
//
// Measured result: the straight chain costs only ~2x the seed-only case, because
// the scan runs in increasing x/y, so the promoted front advances within a single
// pass. These benchmarks exist to keep that property true; extra passes (and a
// quadratic cost) would require adversarially wrapped geometry.
func benchHysteresisInput(w, h, chain int) []float64 {
	buf := make([]float64, w*h)
	row := w // interior row 1
	if chain > w-2 {
		chain = w - 2
	}
	for x := 1; x <= chain; x++ {
		buf[row+x] = 75.0 // weak: low <= 75 < high
	}
	buf[row+1] = 300.0 // strong seed
	return buf
}

func BenchmarkHysteresisSeedOnly(b *testing.B) {
	b.ReportAllocs()
	buf := benchHysteresisInput(512, 512, 0)
	b.ResetTimer()
	for b.Loop() {
		sinkEdges = hysteresisThresholding(buf, 512, 512, 50, 150)
	}
}

func BenchmarkHysteresisWeakChain(b *testing.B) {
	b.ReportAllocs()
	buf := benchHysteresisInput(512, 512, 510)
	b.ResetTimer()
	for b.Loop() {
		sinkEdges = hysteresisThresholding(buf, 512, 512, 50, 150)
	}
}
