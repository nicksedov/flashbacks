package imaging

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransformBoundingBoxDimensions_NoRotation(t *testing.T) {
	width, height := transformBoundingBoxDimensions(0, 100, 200)

	assert.Equal(t, 100, width, "width should remain same with no rotation")
	assert.Equal(t, 200, height, "height should remain same with no rotation")
}

func TestTransformBoundingBoxDimensions_90Deg(t *testing.T) {
	width, height := transformBoundingBoxDimensions(90, 100, 200)

	// 90 deg CCW rotation should swap dimensions
	expectedWidth := 200
	expectedHeight := 100

	assert.InDelta(t, expectedWidth, width, 1, "width should be approximately 200")
	assert.InDelta(t, expectedHeight, height, 1, "height should be approximately 100")
}

func TestTransformBoundingBoxDimensions_180Deg(t *testing.T) {
	width, height := transformBoundingBoxDimensions(180, 100, 200)

	// 180 deg rotation should return same dimensions (rectangle is symmetric)
	assert.InDelta(t, 100, width, 1, "width should be approximately 100")
	assert.InDelta(t, 200, height, 1, "height should be approximately 200")
}

func TestTransformBoundingBoxDimensions_270Deg(t *testing.T) {
	width, height := transformBoundingBoxDimensions(270, 100, 200)

	// 270 deg CCW = 90 deg CW, should swap dimensions
	expectedWidth := 200
	expectedHeight := 100

	assert.InDelta(t, expectedWidth, width, 1, "width should be approximately 200")
	assert.InDelta(t, expectedHeight, height, 1, "height should be approximately 100")
}

func TestDetectContentType_PNG(t *testing.T) {
	om, _, _ := setupOcrManager(t)

	contentType := om.detectContentType("/path/to/image.png")

	assert.Equal(t, "image/png", contentType)
}

func TestDetectContentType_JPEG(t *testing.T) {
	om, _, _ := setupOcrManager(t)

	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/image.jpg", "image/jpeg"},
		{"/path/to/image.jpeg", "image/jpeg"},
		{"/path/to/image.webp", "image/jpeg"},
		{"/path/to/image.xyz", "image/jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			contentType := om.detectContentType(tt.path)
			assert.Equal(t, tt.expected, contentType)
		})
	}
}

// Helper to verify trig calculations
func TestTransformBoundingBoxDimensions_NegativeAngle(t *testing.T) {
	// -90 degrees should be normalized to 270
	width1, height1 := transformBoundingBoxDimensions(-90, 100, 200)
	width2, height2 := transformBoundingBoxDimensions(270, 100, 200)

	assert.Equal(t, width2, width1, "-90 should equal 270")
	assert.Equal(t, height2, height1, "-90 should equal 270")
}

func TestTransformBoundingBoxDimensions_LargeAngle(t *testing.T) {
	// 450 degrees should normalize to 90
	width, height := transformBoundingBoxDimensions(450, 100, 200)

	// Should behave like 90 degrees
	expectedWidth := 200
	expectedHeight := 100

	assert.InDelta(t, expectedWidth, width, 1)
	assert.InDelta(t, expectedHeight, height, 1)
}

func TestTransformBoundingBoxDimensions_ZeroDimensions(t *testing.T) {
	width, height := transformBoundingBoxDimensions(90, 0, 0)

	assert.Equal(t, 0, width)
	assert.Equal(t, 0, height)
}

func TestTrigTransformation_45Degrees(t *testing.T) {
	// At 45 degrees, dimensions should change based on trig functions
	width, height := transformBoundingBoxDimensions(45, 100, 100)

	// For a square rotated 45 degrees: new_dim = old_dim * (cos(45) + sin(45))
	// = 100 * (0.707 + 0.707) = 100 * 1.414 = 141.4
	expected := int(math.Round(100.0 * (math.Cos(math.Pi/4) + math.Sin(math.Pi/4))))

	assert.InDelta(t, expected, width, 1)
	assert.InDelta(t, expected, height, 1)
}
