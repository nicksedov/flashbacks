package llm

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/deepteams/webp"
	"github.com/disintegration/imaging"
)

// DecodeError wraps an image.Decode failure with the file path.
// Use errors.As to detect permanent image corruption errors.
type DecodeError struct {
	Path string
	Err  error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("failed to decode image %s: %v", e.Path, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }

// IsPermanentError reports whether an error from an AI action is permanent
// (i.e. the image file itself is broken and retrying won't help).
// It checks for DecodeError sentinel and recognized format-specific messages.
func IsPermanentError(err error) bool {
	if err == nil {
		return false
	}
	var de *DecodeError
	if errors.As(err, &de) {
		return true
	}
	// Fallback for errors that arrive via non-standard wrapping paths
	// (e.g. third-party clients that construct their own error messages).
	msg := err.Error()
	return strings.Contains(msg, "invalid JPEG format") ||
		strings.Contains(msg, "missing SOS marker") ||
		strings.Contains(msg, "invalid PNG format") ||
		strings.Contains(msg, "invalid WebP format") ||
		strings.Contains(msg, "image: unknown format") ||
		strings.Contains(msg, "unexpected EOF") ||
		strings.Contains(msg, "unexpected end of file")
}

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
		return nil, "", &DecodeError{Path: imagePath, Err: err}
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

// prepareImageForEditing reads an image from the given path, downsizes it if its
// pixel count exceeds maxMegapixels (snapping dimensions to multiples of 32),
// and returns the resized JPEG bytes along with the original width, height, and
// file extension (including the leading dot, e.g. ".jpg").
func prepareImageForEditing(imagePath string, maxMegapixels float64) (resizedData []byte, origWidth, origHeight int, origExt string, err error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, 0, 0, "", fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, 0, 0, "", &DecodeError{Path: imagePath, Err: err}
	}

	bounds := img.Bounds()
	origWidth = bounds.Dx()
	origHeight = bounds.Dy()
	origExt = strings.ToLower(filepath.Ext(imagePath))

	if origWidth > 0 && origHeight > 0 && maxMegapixels > 0 {
		megapixels := float64(origWidth*origHeight) / 1_000_000.0
		if megapixels > maxMegapixels {
			scale := math.Sqrt(maxMegapixels * 1_000_000.0 / float64(origWidth*origHeight))
			newWidth := int(math.Round(float64(origWidth) * scale))
			if newWidth > 0 {
				newWidth = roundToMultipleOf32(newWidth)
				scale = float64(newWidth) / float64(origWidth)
				newHeight := int(math.Round(float64(origHeight) * scale))
				if newHeight > 0 {
					newHeight = roundToMultipleOf32(newHeight)
					img = imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, 0, 0, "", fmt.Errorf("failed to encode image: %w", err)
	}

	return buf.Bytes(), origWidth, origHeight, origExt, nil
}

// postProcessEditedImage resizes the result image (which is always PNG from the
// API) to the target dimensions and converts it to the format indicated by
// targetExt (e.g. ".jpg", ".png", ".webp", ".gif").
func postProcessEditedImage(data []byte, targetWidth, targetHeight int, targetExt string) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode result image: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != targetWidth || bounds.Dy() != targetHeight {
		img = imaging.Resize(img, targetWidth, targetHeight, imaging.Lanczos)
	}

	var buf bytes.Buffer
	switch strings.ToLower(targetExt) {
	case ".jpg", ".jpeg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92})
	case ".png":
		err = png.Encode(&buf, img)
	case ".gif":
		err = gif.Encode(&buf, img, nil)
	case ".webp":
		err = webp.Encode(&buf, img, &webp.Options{Quality: 92})
	default:
		err = png.Encode(&buf, img)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to encode result image as %s: %w", targetExt, err)
	}

	return buf.Bytes(), nil
}

// mediaTypeByExt returns the MIME type for a file extension.
func mediaTypeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}
