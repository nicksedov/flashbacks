package llm

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"os"
	"strings"

	"github.com/disintegration/imaging"
)

// roundToMultipleOf32 rounds v to the nearest multiple of 32 (minimum 32).
func roundToMultipleOf32(v int) int {
	n := int(math.Round(float64(v) / 32.0))
	if n < 1 {
		n = 1
	}
	return n * 32
}

// resizeImageForLLM reads an image from the given path and downsizes it if its
// pixel count exceeds maxMegapixels. After scaling, both dimensions are snapped
// to the nearest multiple of 32 to align with vision-model patch grids.
// The returned bytes are JPEG-encoded and the media type is inferred from the
// file extension.
func resizeImageForLLM(imagePath string, maxMegapixels float64) ([]byte, string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width > 0 && height > 0 && maxMegapixels > 0 {
		megapixels := float64(width*height) / 1_000_000.0
		if megapixels > maxMegapixels {
			scale := math.Sqrt(maxMegapixels * 1_000_000.0 / float64(width*height))
			newWidth := int(math.Round(float64(width) * scale))
			if newWidth > 0 {
				// Snap x-dimension to nearest multiple of 32
				newWidth = roundToMultipleOf32(newWidth)
				// Precise scale correction
				scale = float64(newWidth) / float64(width)
				newHeight := int(math.Round(float64(height) * scale))
				if newHeight > 0 {
					// Snap y-dimension to nearest multiple of 32
					newHeight = roundToMultipleOf32(newHeight)
					img = imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", fmt.Errorf("failed to encode image: %w", err)
	}

	mediaType := "image/jpeg"
	ext := strings.ToLower(imagePath)
	switch {
	case strings.HasSuffix(ext, ".png"):
		mediaType = "image/png"
	case strings.HasSuffix(ext, ".gif"):
		mediaType = "image/gif"
	case strings.HasSuffix(ext, ".webp"):
		mediaType = "image/webp"
	case strings.HasSuffix(ext, ".tiff") || strings.HasSuffix(ext, ".tif"):
		mediaType = "image/tiff"
	}

	return buf.Bytes(), mediaType, nil
}
