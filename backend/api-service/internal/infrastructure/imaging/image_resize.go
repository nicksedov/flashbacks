package imaging

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
	"strconv"
	"strings"

	"github.com/deepteams/webp"
	imgproc "github.com/disintegration/imaging"
)

// llmMaxMegapixels reads the LLM_MAX_IMAGE_MEGAPIXELS env var (default 2.4 MP)
// used by DownsizeImageForLLM to limit image size before sending to vision models.
func llmMaxMegapixels() float64 {
	if v, ok := os.LookupEnv("LLM_MAX_IMAGE_MEGAPIXELS"); ok {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			return n
		}
	}
	return 2.4
}

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

// DownsizeImageForLLM reads an image from the given path and downsizes it if its
// pixel count exceeds the limit set by the LLM_MAX_IMAGE_MEGAPIXELS env var
// (default 2.4 MP). After scaling, both dimensions are snapped to the nearest
// multiple of 32 to align with vision-model patch grids.
// The returned bytes are JPEG-encoded and the media type is inferred from the
// file extension.
func DownsizeImageForLLM(imagePath string) ([]byte, string, error) {
	maxMegapixels := llmMaxMegapixels()

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
					img = imgproc.Resize(img, newWidth, newHeight, imgproc.Lanczos)
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", fmt.Errorf("failed to encode image: %w", err)
	}

	mediaType := MediaTypeByExt(filepath.Ext(imagePath))

	return buf.Bytes(), mediaType, nil
}

// ResizeImage reads an image from srcPath, resizes it to fit within maxWidth x maxHeight
// while preserving aspect ratio, and writes the result to dstPath as JPEG.
// If maxWidth or maxHeight is 0, that dimension is calculated to maintain the
// aspect ratio of the original. If both are 0, the image is copied as-is.
func ResizeImage(srcPath, dstPath string, maxWidth, maxHeight int) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return &DecodeError{Path: srcPath, Err: err}
	}

	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	if origW <= 0 || origH <= 0 {
		return fmt.Errorf("invalid image dimensions: %dx%d", origW, origH)
	}

	newW, newH := origW, origH
	if maxWidth > 0 || maxHeight > 0 {
		w := maxWidth
		h := maxHeight
		if w <= 0 {
			w = origW
		}
		if h <= 0 {
			h = origH
		}
		// Fit scales the image to fit within w x h preserving aspect ratio
		fitted := imgproc.Fit(img, w, h, imgproc.Lanczos)
		newW = fitted.Bounds().Dx()
		newH = fitted.Bounds().Dy()
		img = fitted
	}

	if newW == origW && newH == origH {
		data, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			return fmt.Errorf("failed to read source image: %w", readErr)
		}
		return os.WriteFile(dstPath, data, 0644)
	}

	resized := img

	out, createErr := os.Create(dstPath)
	if createErr != nil {
		return fmt.Errorf("failed to create output file: %w", createErr)
	}
	defer out.Close()

	if encErr := jpeg.Encode(out, resized, &jpeg.Options{Quality: 85}); encErr != nil {
		return fmt.Errorf("failed to encode resized image: %w", encErr)
	}

	return nil
}

// PrepareImageForEditing reads an image from the given path, downsizes it if its
// pixel count exceeds maxMegapixels (snapping dimensions to multiples of 32),
// and upscales it if the pixel count is below minMegapixels.
//
// Returns the JPEG-encoded image bytes, the original dimensions (before any
// resize), the effective dimensions (after resize, equal to original if no
// resize happened), and the original file extension.
//
// Post-processing rule: use the original dimensions as the target, UNLESS the
// image was upscaled (effective > original) — in that case use the effective
// dimensions so the enhanced image keeps its improved resolution.
func PrepareImageForEditing(imagePath string, maxMegapixels, minMegapixels float64) (resizedData []byte, origWidth, origHeight, effectiveWidth, effectiveHeight int, origExt string, err error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, 0, 0, 0, 0, "", fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, 0, 0, 0, 0, "", &DecodeError{Path: imagePath, Err: err}
	}

	bounds := img.Bounds()
	origWidth = bounds.Dx()
	origHeight = bounds.Dy()
	origExt = strings.ToLower(filepath.Ext(imagePath))

	if origWidth > 0 && origHeight > 0 {
		pixels := float64(origWidth * origHeight)

		// Downsize if the image exceeds the maximum megapixel limit.
		// Done only for API compatibility — the result is restored to
		// original dimensions during post-processing.
		if maxMegapixels > 0 && pixels > maxMegapixels*1_000_000 {
			scale := math.Sqrt(maxMegapixels * 1_000_000.0 / pixels)
			newWidth := int(math.Round(float64(origWidth) * scale))
			if newWidth > 0 {
				newWidth = roundToMultipleOf32(newWidth)
				scale = float64(newWidth) / float64(origWidth)
				newHeight := int(math.Round(float64(origHeight) * scale))
				if newHeight > 0 {
					newHeight = roundToMultipleOf32(newHeight)
					img = imgproc.Resize(img, newWidth, newHeight, imgproc.Lanczos)
				}
			}
		}

		// Upscale if the image is below the minimum pixel threshold required
		// by the target API (e.g. Alibaba DashScope requires ≥ 0.6 MP).
		// Unlike downsizing, upscaled dimensions are KEPT in the final result
		// so the enhanced image retains its improved resolution.
		if minMegapixels > 0 && pixels < minMegapixels*1_000_000 {
			scale := math.Sqrt(minMegapixels * 1_000_000.0 / pixels)
			newWidth := int(math.Round(float64(origWidth) * scale))
			if newWidth > 0 {
				newWidth = roundToMultipleOf32(newWidth)
				scale = float64(newWidth) / float64(origWidth)
				newHeight := int(math.Round(float64(origHeight) * scale))
				if newHeight > 0 {
					newHeight = roundToMultipleOf32(newHeight)
					img = imgproc.Resize(img, newWidth, newHeight, imgproc.Lanczos)
				}
			}
		}
	}

	// Capture effective dimensions after any resize.
	effectiveBounds := img.Bounds()
	effectiveWidth = effectiveBounds.Dx()
	effectiveHeight = effectiveBounds.Dy()

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, 0, 0, 0, 0, "", fmt.Errorf("failed to encode image: %w", err)
	}

	return buf.Bytes(), origWidth, origHeight, effectiveWidth, effectiveHeight, origExt, nil
}

// PostProcessEditedImage resizes the result image (which is always PNG from the
// API) to the target dimensions and converts it to the format indicated by
// targetExt (e.g. ".jpg", ".png", ".webp", ".gif").
func PostProcessEditedImage(data []byte, targetWidth, targetHeight int, targetExt string) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode result image: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != targetWidth || bounds.Dy() != targetHeight {
		img = imgproc.Resize(img, targetWidth, targetHeight, imgproc.Lanczos)
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

// MediaTypeByExt returns the MIME type for a file extension.
func MediaTypeByExt(ext string) string {
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
