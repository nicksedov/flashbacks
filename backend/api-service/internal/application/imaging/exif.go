package imaging

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/flashbacks/api-service/internal/domain"
)

// HasExifData returns true if any meaningful EXIF field is populated.
func HasExifData(meta *domain.ImageMetadata) bool {
	return meta.CameraModel != "" || meta.LensModel != "" || meta.ISO != 0 ||
		meta.Aperture != "" || meta.ShutterSpeed != "" || meta.FocalLength != "" ||
		meta.DateTaken != nil || meta.Software != "" || meta.GeolocationRef != nil
}

// parseGPSString parses GPS coordinate strings in various formats:
// "41 deg 24' 12.2" N", "41.40338", "41 24 12.2", etc.
func parseGPSString(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	// Remove hemisphere reference if present
	s = strings.TrimRight(s, "NSEWnesw ")

	// Try parsing as a simple float first
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val, true
	}

	// Try parsing "deg min sec" or "deg min'sec\"" format
	// Remove common symbols
	s = strings.ReplaceAll(s, "deg", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.TrimSpace(s)

	parts := strings.Fields(s)
	if len(parts) >= 2 {
		deg, err1 := strconv.ParseFloat(parts[0], 64)
		min, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil {
			sec := 0.0
			if len(parts) >= 3 {
				sec, _ = strconv.ParseFloat(parts[2], 64)
			}
			return deg + min/60.0 + sec/3600.0, true
		}
	}

	return 0, false
}

// parseGPSPosition tries to parse combined GPSPosition field.
// Format is typically "41.40338, 12.13278" or similar.
func parseGPSPosition(s string) (float64, float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}

	// Try comma-separated
	parts := strings.Split(s, ",")
	if len(parts) == 2 {
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lng, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil {
			return lat, lng, true
		}
	}

	// Try space-separated
	parts = strings.Fields(s)
	if len(parts) == 2 {
		lat, err1 := strconv.ParseFloat(parts[0], 64)
		lng, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil {
			return lat, lng, true
		}
	}

	return 0, 0, false
}

// parseOrientation converts orientation string to integer value.
func parseOrientation(s string) int {
	s = strings.TrimSpace(s)

	// Try direct integer
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}

	// Parse text orientations
	lower := strings.ToLower(s)
	if strings.Contains(lower, "rotate") {
		if strings.Contains(lower, "90") {
			if strings.Contains(lower, "cw") || strings.Contains(lower, "normal") {
				return 6 // 90 CW
			}
			return 8 // 90 CCW (or 270 CW)
		}
		if strings.Contains(lower, "180") {
			return 3
		}
	}

	if strings.Contains(lower, "mirror") || strings.Contains(lower, "flip") {
		if strings.Contains(lower, "horizontal") {
			return 2
		}
		if strings.Contains(lower, "vertical") {
			return 4
		}
	}

	if strings.Contains(lower, "normal") || strings.Contains(lower, "horizontal") {
		return 1
	}

	return 0
}

// parseColorSpace converts color space string to a standardized name.
func parseColorSpace(s string) string {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)

	switch {
	case lower == "srgb", strings.Contains(lower, "srgb"):
		return "sRGB"
	case lower == "adobe rgb", strings.Contains(lower, "adobe"):
		return "Adobe RGB"
	case strings.Contains(lower, "uncalibrat"):
		return "Uncalibrated"
	default:
		return s
	}
}

// parseExifDate parses EXIF date format "YYYY:MM:DD HH:MM:SS"
func parseExifDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	if len(dateStr) < 19 {
		return time.Time{}, fmt.Errorf("invalid date format: %s", dateStr)
	}

	// EXIF format: "2024:01:15 14:30:45"
	// Convert to: "2024-01-15 14:30:45"
	dateStr = dateStr[:4] + "-" + dateStr[5:7] + "-" + dateStr[8:]

	return time.Parse("2006-01-02 15:04:05", dateStr)
}

// cleanString removes trailing whitespace and null characters from strings
func cleanString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "\x00")
	return s
}

// formatExposureTimeFloat formats a float exposure time value.
func formatExposureTimeFloat(val float64) string {
	if val <= 0 {
		return "0s"
	}
	if val >= 1 {
		if val == math.Trunc(val) {
			return fmt.Sprintf("%.0fs", val)
		}
		return fmt.Sprintf("%.1fs", val)
	}
	// Show as fraction, e.g. "1/250s"
	denom := int(1.0 / val)
	return fmt.Sprintf("1/%ds", denom)
}
