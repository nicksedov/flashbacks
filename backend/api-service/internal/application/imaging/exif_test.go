package imaging

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseGPSString_Decimal(t *testing.T) {
	result, ok := parseGPSString("41.40338")

	assert.True(t, ok)
	assert.InDelta(t, 41.40338, result, 0.00001)
}

func TestParseGPSString_DegMinSec(t *testing.T) {
	result, ok := parseGPSString("41 24 12.2")

	assert.True(t, ok)
	// 41 + 24/60 + 12.2/3600 = 41.40339
	assert.InDelta(t, 41.40339, result, 0.00001)
}

func TestParseGPSString_Empty(t *testing.T) {
	_, ok := parseGPSString("")

	assert.False(t, ok)
}

func TestParseGPSString_Invalid(t *testing.T) {
	_, ok := parseGPSString("abc")

	assert.False(t, ok)
}

func TestParseGPSPosition_Comma(t *testing.T) {
	lat, lng, ok := parseGPSPosition("41.4, 12.1")

	assert.True(t, ok)
	assert.InDelta(t, 41.4, lat, 0.00001)
	assert.InDelta(t, 12.1, lng, 0.00001)
}

func TestParseOrientation_Normal(t *testing.T) {
	tests := []string{"1", "Normal", "normal", "Horizontal (normal)"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			result := parseOrientation(input)
			assert.Equal(t, 1, result)
		})
	}
}

func TestParseOrientation_Rotate90CW(t *testing.T) {
	result := parseOrientation("Rotate 90 CW")

	assert.Equal(t, 6, result)
}

func TestParseColorSpace_sRGB(t *testing.T) {
	tests := []string{"sRGB", "srgb", "sRGB IEC61966-2.1"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			result := parseColorSpace(input)
			assert.Equal(t, "sRGB", result)
		})
	}
}

func TestParseExifDate_Valid(t *testing.T) {
	result, err := parseExifDate("2024:01:15 14:30:45")

	assert.NoError(t, err)
	expected := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
	assert.Equal(t, expected, result)
}

func TestFormatExposureTimeFloat_GTE1(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{2.0, "2s"},
		{1.5, "1.5s"},
		{0.5, "1/2s"},
		{0.004, "1/250s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatExposureTimeFloat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanString_TrailingWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello  ", "hello"},
		{"world\x00", "world"},
		{"  test  ", "test"},
		{"normal", "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseGPSString_WithHemisphere(t *testing.T) {
	result, ok := parseGPSString("41 deg 24' 12.2\" N")

	assert.True(t, ok)
	assert.InDelta(t, 41.40339, result, 0.00001)
}

func TestParseOrientation_Rotate180(t *testing.T) {
	result := parseOrientation("Rotate 180")

	assert.Equal(t, 3, result)
}

func TestParseColorSpace_AdobeRGB(t *testing.T) {
	result := parseColorSpace("Adobe RGB")

	assert.Equal(t, "Adobe RGB", result)
}

func TestFormatExposureTimeFloat_Zero(t *testing.T) {
	result := formatExposureTimeFloat(0)

	assert.Equal(t, "0s", result)
}

func TestParseGPSPosition_SpaceSeparated(t *testing.T) {
	lat, lng, ok := parseGPSPosition("41.4 12.1")

	assert.True(t, ok)
	assert.InDelta(t, 41.4, lat, 0.00001)
	assert.InDelta(t, 12.1, lng, 0.00001)
}

func TestParseGPSPosition_Empty(t *testing.T) {
	_, _, ok := parseGPSPosition("")

	assert.False(t, ok)
}

func TestParseExifDate_Invalid(t *testing.T) {
	_, err := parseExifDate("invalid")

	assert.Error(t, err)
}
