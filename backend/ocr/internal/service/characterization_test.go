package service

import (
	"bytes"
	"image"
	"image/color"
	"math/rand"
	"testing"
	"unicode"
)

// This file pins the observable behaviour of the pure (non-CGO) helpers in the
// OCR service package BEFORE any performance work. Every optimization applied
// afterwards must keep these assertions green without being relaxed; a failure
// here means the change altered behaviour, not just cost.

// TestCountTokensCharacterization pins the token-counting semantics for every
// branch of countTokens (letters/digits, separators between digits, sign before
// a digit, unit symbols, currency symbols, quotation marks).
func TestCountTokensCharacterization(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"latin word", "hello", 5},
		{"alphanumeric", "abc123", 6},
		{"cyrillic word", "привет", 6},
		{"decimal point between digits", "3.14", 4},
		{"thousands comma between digits", "1,000", 5},
		{"ratio slash between digits", "3/4", 3},
		{"plus before digit", "+5", 2},
		{"ascii minus before digit", "-10", 3},
		{"unicode minus before digit", "−10", 3},
		// NOTE: UnitSymbols.R16 (tokens.go) is not sorted by Lo, and
		// unicode.Is assumes a sorted RangeTable. As a result the lookup
		// silently misses some unit symbols: the degree sign (U+00B0) is
		// NOT counted today, while '%' (U+0025, first entry) and '‰'
		// (U+2030) are. These expectations pin the OBSERVED behaviour and
		// must not be "corrected" as part of a performance change.
		{"percent after digit is detected", "50%", 3},
		{"degree after digit is not detected (unsorted RangeTable)", "45°", 2},
		{"degree without preceding digit", "°", 0},
		{"per-mille after digit is detected", "5‰", 2},
		{"currency before digits", "$100", 4},
		{"currency after digits", "50€", 3},
		{"quotes around word", `"abc"`, 5},
		{"dashes without digits", "---", 0},
		{"separated words", "a b", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countTokens(tt.in); got != tt.want {
				t.Errorf("countTokens(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestUnitSymbolLookupEvidence documents the root cause of the degree-sign
// mis-detection pinned in TestCountTokensCharacterization: the UnitSymbols
// RangeTable is not sorted by Lo, so unicode.Is matches only the symbols that
// happen to be reachable in scan order. This is evidence for a separate
// correctness fix; it asserts current library-level behaviour, not intent.
func TestUnitSymbolLookupEvidence(t *testing.T) {
	observed := map[string]struct {
		r    rune
		want bool
	}{
		"percent U+0025 (first R16 entry)": {'%', true},
		"per-mille U+2030":                 {'‰', true},
		"degree U+00B0 (out of order)":     {'°', false},
	}

	for name, tc := range observed {
		if got := unicode.Is(UnitSymbols, tc.r); got != tc.want {
			t.Errorf("%s: unicode.Is(UnitSymbols, %q) = %v, want %v (current observed behaviour)",
				name, tc.r, got, tc.want)
		}
	}
}

// TestConvertToGrayCharacterization pins the luminance conversion and the
// "light gray becomes white" threshold boundary (first value >= threshold is
// snapped to 255, the value just below stays as-is).
func TestConvertToGrayCharacterization(t *testing.T) {
	const w, h = 8, 1
	levels := []uint8{0, 100, 223, 224, 225, 255, 10, 200}
	want := []uint8{0, 100, 223, 255, 255, 255, 10, 200}

	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x, v := range levels {
		src.SetNRGBA(x, 0, color.NRGBA{R: v, G: v, B: v, A: 255})
	}

	gray := convertToGray(src, 224)
	if got := gray.Bounds().Dx(); got != w {
		t.Fatalf("convertToGray width = %d, want %d", got, w)
	}
	if got := gray.Bounds().Dy(); got != h {
		t.Fatalf("convertToGray height = %d, want %d", got, h)
	}

	for x, exp := range want {
		if got := gray.GrayAt(x, 0).Y; got != exp {
			t.Errorf("convertToGray pixel x=%d = %d, want %d", x, got, exp)
		}
	}
}

// TestConvertToGrayNonZeroOriginCharacterization verifies that the destination
// image is written from (0,0) even when the source has a non-zero origin.
func TestConvertToGrayNonZeroOriginCharacterization(t *testing.T) {
	src := image.NewNRGBA(image.Rect(5, 7, 9, 9))

	gray := convertToGray(src, 224)
	if got := gray.Bounds(); got != image.Rect(0, 0, 4, 2) {
		t.Fatalf("convertToGray bounds = %v, want (0,0)-(4,2)", got)
	}
}

// TestPreprocessImageCharacterization pins the scale factor and destination
// dimensions for a sub-0.5MP source, and that a uniform source stays uniform
// through the resize + median-blur + grayscale pipeline.
func TestPreprocessImageCharacterization(t *testing.T) {
	const w, h = 64, 48

	black := image.NewGray(image.Rect(0, 0, w, h))
	gray, scale, outW, outH := preprocessImage(black)
	if gray == nil {
		t.Fatal("preprocessImage returned nil for a 64x48 image")
	}
	if scale != 4.0 {
		t.Errorf("preprocessImage scale = %v, want 4.0", scale)
	}
	if outW != 256 || outH != 192 {
		t.Errorf("preprocessImage size = %dx%d, want 256x192", outW, outH)
	}
	if got := gray.Bounds().Dx(); got != 256 {
		t.Errorf("preprocessed image width = %d, want 256", got)
	}
	if got := gray.GrayAt(0, 0).Y; got != 0 {
		t.Errorf("black source pixel = %d, want 0", got)
	}
	if got := gray.GrayAt(255, 191).Y; got != 0 {
		t.Errorf("black source pixel = %d, want 0", got)
	}

	white := image.NewGray(image.Rect(0, 0, w, h))
	for i := range white.Pix {
		white.Pix[i] = 255
	}
	grayWhite, _, _, _ := preprocessImage(white)
	if grayWhite == nil {
		t.Fatal("preprocessImage returned nil for a white 64x48 image")
	}
	if got := grayWhite.GrayAt(128, 96).Y; got != 255 {
		t.Errorf("white source pixel = %d, want 255", got)
	}
}

// TestCannyEdgeDetectionCharacterization pins Canny output for a synthetic
// vertical step edge. A single step produces exactly two strong columns on
// every interior row and nothing on the top/bottom border rows.
func TestCannyEdgeDetectionCharacterization(t *testing.T) {
	const w, h = 32, 8

	gray := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x >= 16 {
				gray.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}

	edges := cannyEdgeDetection(gray, 50, 150)
	if got := edges.Bounds(); got != image.Rect(0, 0, w, h) {
		t.Fatalf("canny bounds = %v, want (0,0)-(%d,%d)", got, w, h)
	}

	want := make(map[[2]int]uint8)
	for y := 1; y < h-1; y++ {
		want[[2]int{15, y}] = 255
		want[[2]int{16, y}] = 255
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			got := edges.GrayAt(x, y).Y
			exp := want[[2]int{x, y}]
			if got != exp {
				t.Errorf("canny pixel (%d,%d) = %d, want %d", x, y, got, exp)
			}
		}
	}
}

// TestDetectSkewAngleFallbackCharacterization pins the fallback candidate set
// returned when no Hough line reaches the vote threshold (a short 8px-tall
// image can accumulate at most 6 votes, below the minimum threshold of 50).
func TestDetectSkewAngleFallbackCharacterization(t *testing.T) {
	const w, h = 32, 8

	gray := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x >= 16 {
				gray.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}

	got := detectSkewAngle(gray)
	want := []int{90, 180, 270}
	if len(got) != len(want) {
		t.Fatalf("detectSkewAngle() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("detectSkewAngle() = %v, want %v", got, want)
		}
	}
}

// TestDetectSkewAngleDeterminism asserts the pipeline is deterministic, so the
// golden values above remain a valid guard for later refactors.
func TestDetectSkewAngleDeterminism(t *testing.T) {
	const w, h = 64, 64
	gray := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		gray.SetGray(20, y, color.Gray{Y: 255})
		gray.SetGray(21, y, color.Gray{Y: 255})
	}

	first := detectSkewAngle(gray)
	second := detectSkewAngle(gray)

	if len(first) != len(second) {
		t.Fatalf("detectSkewAngle not deterministic: %v vs %v", first, second)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("detectSkewAngle not deterministic: %v vs %v", first, second)
		}
	}
}

// TestWeightedMedianCharacterization pins the weighted-median helper used to
// pick the dominant skew.
func TestWeightedMedianCharacterization(t *testing.T) {
	tests := []struct {
		name    string
		values  []float64
		weights []float64
		want    float64
	}{
		{"empty", nil, nil, 0},
		{"single", []float64{2.5}, []float64{1}, 2.5},
		{"uniform three", []float64{1, 2, 3}, []float64{1, 1, 1}, 2},
		{"weighted towards first", []float64{1, 9}, []float64{10, 1}, 1},
		{"weighted towards last", []float64{1, 9}, []float64{1, 10}, 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := weightedMedian(tt.values, tt.weights); got != tt.want {
				t.Errorf("weightedMedian(%v, %v) = %v, want %v", tt.values, tt.weights, got, tt.want)
			}
		})
	}
}

// TestGenerateCandidateAnglesCharacterization pins the candidate angle set
// derivation (0 is always excluded because phase 1 already tested it).
func TestGenerateCandidateAnglesCharacterization(t *testing.T) {
	tests := []struct {
		name string
		skew float64
		want []int
	}{
		{"zero skew", 0, []int{90, 180, 270}},
		{"positive skew", 5, []int{5, 95, 185, 275}},
		{"negative skew", -5, []int{355, 85, 175, 265}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateCandidateAngles(tt.skew)
			if len(got) != len(tt.want) {
				t.Fatalf("generateCandidateAngles(%v) = %v, want %v", tt.skew, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("generateCandidateAngles(%v) = %v, want %v", tt.skew, got, tt.want)
				}
			}
		})
	}
}

// referenceConvertToGray is the straightforward, interface-based implementation
// of convertToGray. It is the source of truth the optimized implementation is
// checked against.
func referenceConvertToGray(img image.Image, threshold uint8) *image.Gray {
	bounds := img.Bounds()
	grayImg := image.NewGray(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			lum := uint8((19595*r + 38470*g + 7471*b + 1<<15) >> 24)
			if lum >= threshold {
				lum = 0xff
			}
			grayImg.SetGray(x-bounds.Min.X, y-bounds.Min.Y, color.Gray{Y: lum})
		}
	}
	return grayImg
}

// TestConvertToGrayMatchesGenericReference guards the optimized convertToGray:
// every supported concrete layout, including alpha-carrying and sub-image
// inputs, must produce byte-identical output to the generic implementation for
// every threshold.
func TestConvertToGrayMatchesGenericReference(t *testing.T) {
	const w, h = 37, 19
	rnd := rand.New(rand.NewSource(20261015))

	nrgba := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(nrgba.Pix); i += 4 {
		nrgba.Pix[i] = uint8(rnd.Intn(256))
		nrgba.Pix[i+1] = uint8(rnd.Intn(256))
		nrgba.Pix[i+2] = uint8(rnd.Intn(256))
		// Deliberately include fully transparent and partially transparent
		// pixels, since NRGBA premultiplies by alpha inside RGBA().
		nrgba.Pix[i+3] = uint8(rnd.Intn(4) * 85)
	}

	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(rgba.Pix); i++ {
		rgba.Pix[i] = uint8(rnd.Intn(256))
	}

	gray := image.NewGray(image.Rect(0, 0, w, h))
	for i := 0; i < len(gray.Pix); i++ {
		gray.Pix[i] = uint8(rnd.Intn(256))
	}

	// YCbCr exercises the generic fallback branch.
	ycbcr := image.NewYCbCr(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	for i := range ycbcr.Y {
		ycbcr.Y[i] = uint8(rnd.Intn(256))
	}
	for i := range ycbcr.Cb {
		ycbcr.Cb[i] = uint8(rnd.Intn(256))
		ycbcr.Cr[i] = uint8(rnd.Intn(256))
	}

	// A sub-image with a non-zero origin exercises the bounds-offset handling.
	backing := image.NewNRGBA(image.Rect(0, 0, 2*w, 2*h))
	for i := 0; i < len(backing.Pix); i++ {
		backing.Pix[i] = uint8(rnd.Intn(256))
	}
	subImage := backing.SubImage(image.Rect(5, 7, 5+w, 7+h)).(*image.NRGBA)

	cases := []struct {
		name string
		img  image.Image
	}{
		{"NRGBA", nrgba},
		{"RGBA", rgba},
		{"Gray", gray},
		{"YCbCr420", ycbcr},
		{"NRGBASubImage", subImage},
	}

	for _, threshold := range []uint8{0, 1, 100, 224, 255} {
		for _, tc := range cases {
			got := convertToGray(tc.img, threshold)
			want := referenceConvertToGray(tc.img, threshold)

			if got.Stride != want.Stride {
				t.Errorf("convertToGray(%s, %d) stride = %d, want %d",
					tc.name, threshold, got.Stride, want.Stride)
			}
			if len(got.Pix) != len(want.Pix) {
				t.Fatalf("convertToGray(%s, %d) pix length = %d, want %d",
					tc.name, threshold, len(got.Pix), len(want.Pix))
			}
			if !bytes.Equal(got.Pix, want.Pix) {
				for i := range want.Pix {
					if got.Pix[i] != want.Pix[i] {
						t.Fatalf("convertToGray(%s, %d) differs at byte %d: got %d, want %d",
							tc.name, threshold, i, got.Pix[i], want.Pix[i])
					}
				}
			}
		}
	}
}
