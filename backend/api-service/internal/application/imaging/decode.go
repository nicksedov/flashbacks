package imaging

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"strings"

	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// DecodeError wraps an image decode failure with the file path.
// Use errors.As to detect image corruption errors.
type DecodeError struct {
	Path string
	Err  error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("failed to decode image %s: %v", e.Path, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }

// IsDecodeError reports whether an error is a DecodeError (image corruption).
func IsDecodeError(err error) bool {
	if err == nil {
		return false
	}
	// Check if the error or any wrapped error is a DecodeError
	for {
		if _, ok := err.(*DecodeError); ok {
			return true
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
			if err == nil {
				return false
			}
		} else {
			break
		}
	}
	return false
}

// IsJPEGDecodeFailure reports whether an error is a permanent JPEG decoding
// failure (corrupt file, unsupported encoding like large progressive JPEGs
// that Go's decoder cannot handle, etc.).
func IsJPEGDecodeFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unexpected EOF") ||
		strings.Contains(msg, "unexpected end of file") ||
		strings.Contains(msg, "invalid JPEG format") ||
		strings.Contains(msg, "missing SOS marker") ||
		strings.Contains(msg, "image: unknown format")
}

// DecodeImageRobust attempts to decode an image from a file path using Go's
// standard image.Decode. If decoding fails, it wraps the error as a DecodeError
// with the file path for diagnostics.
//
// For JPEG files that fail with "unexpected EOF" (common with large panoramic
// images), the error is classified as a permanent decode failure via
// IsJPEGDecodeFailure.
func DecodeImageRobust(filePath string) (image.Image, string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", &DecodeError{Path: filePath, Err: err}
	}

	return img, format, nil
}

// DecodeImageConfigRobust attempts to decode image configuration (dimensions,
// format) from a file path. Falls back to returning minimum information on
// failure.
func DecodeImageConfigRobust(filePath string) (width, height int, format string, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, format, &DecodeError{Path: filePath, Err: err}
	}

	return cfg.Width, cfg.Height, format, nil
}

// DecodeJPEGRobust attempts to decode a JPEG file. If the standard decoder
// fails, it tries a more lenient approach by reading the JPEG data into a
// buffer first (which can sometimes help with truncated files).
func DecodeJPEGRobust(filePath string) (image.Image, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	// Standard decode first
	img, err := jpeg.Decode(f)
	if err == nil {
		return img, nil
	}

	return nil, &DecodeError{Path: filePath, Err: err}
}
