package service

import (
	"image"
	"testing"
)

func TestShouldApplyMedianBlur(t *testing.T) {
	tests := []struct {
		name        string
		scaleFactor float64
		want        bool
	}{
		{"heavy downscale applies blur", 0.61, true},
		{"moderate downscale applies blur", 0.70, true},
		{"mild downscale skips blur", 0.89, false},
		{"native size skips blur", 1.0, false},
		{"mild upscale skips blur", 1.5, false},
		{"strong upscale applies blur", 2.0, true},
		{"strong upscale applies blur", 3.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldApplyMedianBlur(tt.scaleFactor); got != tt.want {
				t.Errorf("shouldApplyMedianBlur(%v) = %v, want %v", tt.scaleFactor, got, tt.want)
			}
		})
	}
}

func TestCalculateScaleDimensions(t *testing.T) {
	tests := []struct {
		name       string
		w, h       int
		wantFactor float64
	}{
		{"below 0.5MP is upscaled 4x", 400, 300, 4.0},
		{"between 0.5MP and 1MP is upscaled 3x", 1000, 800, 3.0},
		{"between 1MP and 2MP is upscaled 1.5x", 1280, 960, 1.5},
		{"between 2MP and 3MP is kept at native size", 1800, 1400, 1.0},
		{"at 3MP is kept at native size", 2000, 1500, 1.0},
		{"over 3MP is downscaled", 4000, 3000, 0.512},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, got := calculateScaleDimensions(tt.w, tt.h, tt.w*tt.h)
			if got != tt.wantFactor {
				t.Errorf("calculateScaleDimensions(%d, %d) factor = %v, want %v",
					tt.w, tt.h, got, tt.wantFactor)
			}
		})
	}
}

func TestPreprocessImageSkipsTinyImages(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 20, 20))
	gray, _, _, _ := preprocessImage(img)
	if gray != nil {
		t.Fatalf("preprocessImage() = %v, want nil for tiny image", gray)
	}
}
