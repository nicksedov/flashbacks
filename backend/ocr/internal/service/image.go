package service

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"

	"github.com/anthonynsimon/bild/effect"
	"github.com/disintegration/imaging"
)

const (
	medianRadius = 1.0 // Radius for median blur (kernel size 3 = radius 1)

	// blurDownscaleThreshold is the scale factor below which the median blur is
	// applied: heavy downscaling of high-resolution sources introduces aliasing
	// noise that the blur removes.
	blurDownscaleThreshold = 0.75
	// blurUpscaleThreshold is the scale factor at or above which the median blur
	// is applied again: strong upscaling amplifies noise. In the near-native
	// band between the two thresholds the image is already clean, so the blur
	// only smears thin strokes and lowers OCR confidence, and is skipped.
	blurUpscaleThreshold = 2.0

	// Image size thresholds for dynamic scaling
	minDimension    = 32                // Minimum dimension in pixels (skip if smaller)
	halfMegapixel   = 524_288           // 0.5 MP
	oneMegapixel    = 2 * halfMegapixel // 1 MP
	twoMegapixels   = 2 * oneMegapixel  // 2 MP
	threeMegapixels = 3 * oneMegapixel  // 3 MP
)

// rotateImage rotates an image by the specified angle in degrees using imaging library.
func rotateImage(img image.Image, angleDeg int) image.Image {
	// Normalize angle to 0-359
	angleDeg = ((angleDeg % 360) + 360) % 360

	if angleDeg == 0 {
		return imaging.Clone(img)
	}

	// Use optimized 90-degree rotations when possible
	switch angleDeg {
	case 90:
		return imaging.Rotate90(img)
	case 180:
		return imaging.Rotate180(img)
	case 270:
		return imaging.Rotate270(img)
	}

	// General rotation for arbitrary angles with white background
	return imaging.Rotate(img, float64(angleDeg), color.White)
}

// encodeImage encodes an image to bytes in the specified format.
// Supported formats: "png", "jpeg" (default).
func encodeImage(img image.Image, format string) ([]byte, error) {
	var buf bytes.Buffer
	var err error

	switch format {
	case "png":
		err = png.Encode(&buf, img)
	default:
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	}

	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// preprocessImage applies preprocessing pipeline: scale, grayscale, median blur.
// Returns (nil, 0, 0, 0) if image is too small to process.
// Returns (processedImage, scaleFactor, width, height) on success.
func preprocessImage(img image.Image) (*image.Gray, float64, int, int) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	pixels := w * h

	// Skip images with any dimension too small to process
	if w <= minDimension || h <= minDimension {
		return nil, 0, 0, 0
	}

	// Calculate target dimensions and scale factor based on megapixels
	newW, newH, scaleFactor := calculateScaleDimensions(w, h, pixels)

	// Step 1: Scale image using cubic interpolation (CatmullRom)
	var scaled image.Image = imaging.Resize(img, newW, newH, imaging.CatmullRom)

	// Step 2: Apply median blur only where it helps — heavily downscaled
	// (aliasing) or strongly upscaled (noise) sources. In the near-native band
	// it smears thin strokes, so it is skipped there.
	if shouldApplyMedianBlur(scaleFactor) {
		scaled = effect.Median(scaled, medianRadius)
	}

	// Step 3: Convert to grayscale, light gray (224..255) treated as pure white
	grayImg := convertToGray(scaled, 224)

	return grayImg, scaleFactor, newW, newH
}

// shouldApplyMedianBlur reports whether the median blur should be applied for a
// given resize scale factor.
func shouldApplyMedianBlur(scaleFactor float64) bool {
	return scaleFactor < blurDownscaleThreshold || scaleFactor >= blurUpscaleThreshold
}

// calculateScaleDimensions determines target dimensions based on megapixel thresholds.
func calculateScaleDimensions(w, h, pixels int) (newW, newH int, scaleFactor float64) {
	switch {
	case pixels < halfMegapixel:
		// Less than 0.5 MP: scale 4x
		scaleFactor = 4.0
		newW, newH = w*4, h*4
	case pixels < oneMegapixel:
		// Less than 1 MP: scale 3x
		scaleFactor = 3.0
		newW, newH = w*3, h*3
	case pixels < twoMegapixels:
		// Less than 2 MP: scale 1.5x
		scaleFactor = 1.5
		newW, newH = w*3/2, h*3/2
	case pixels <= threeMegapixels:
		// 2-3 MP: no scaling
		scaleFactor = 1.0
		newW, newH = w, h
	default:
		// More than 3 MP: scale down to 3 MP
		scaleFactor = math.Sqrt(float64(threeMegapixels) / float64(pixels))
		newW = int(float64(w) * scaleFactor)
		newH = int(float64(h) * scaleFactor)
	}
	return
}

// convertToGray converts any image to *image.Gray.
// Light gray shades (at or above threshold) are preserved as white pixels.
//
// The preprocessing pipeline only ever produces the concrete layouts of the
// imaging and bild libraries (NRGBA from Resize, RGBA from the median blur), so
// those get direct Pix access and skip one color.Color interface box per pixel.
// Any other layout takes the generic color.Color path. Every branch computes the
// identical luminance and threshold, as asserted by
// TestConvertToGrayMatchesGenericReference.
func convertToGray(img image.Image, threshold uint8) *image.Gray {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	grayImg := image.NewGray(image.Rect(0, 0, w, h))

	switch src := img.(type) {
	case *image.NRGBA:
		for y := 0; y < h; y++ {
			row := grayImg.Pix[y*grayImg.Stride : y*grayImg.Stride+w]
			sp := src.PixOffset(bounds.Min.X, bounds.Min.Y+y)
			for x := 0; x < w; x++ {
				// Mirrors color.NRGBA.RGBA(): 8-bit channel -> 16-bit, then
				// premultiplied by alpha.
				a := uint32(src.Pix[sp+3])
				r := uint32(src.Pix[sp])
				r = (r | r<<8) * a / 0xff
				g := uint32(src.Pix[sp+1])
				g = (g | g<<8) * a / 0xff
				b := uint32(src.Pix[sp+2])
				b = (b | b<<8) * a / 0xff
				row[x] = grayPixel(r, g, b, threshold)
				sp += 4
			}
		}
	case *image.RGBA:
		for y := 0; y < h; y++ {
			row := grayImg.Pix[y*grayImg.Stride : y*grayImg.Stride+w]
			sp := src.PixOffset(bounds.Min.X, bounds.Min.Y+y)
			for x := 0; x < w; x++ {
				// RGBA is already alpha-premultiplied; mirrors color.RGBA.RGBA().
				r := uint32(src.Pix[sp])
				r |= r << 8
				g := uint32(src.Pix[sp+1])
				g |= g << 8
				b := uint32(src.Pix[sp+2])
				b |= b << 8
				row[x] = grayPixel(r, g, b, threshold)
				sp += 4
			}
		}
	case *image.Gray:
		for y := 0; y < h; y++ {
			row := grayImg.Pix[y*grayImg.Stride : y*grayImg.Stride+w]
			sp := src.PixOffset(bounds.Min.X, bounds.Min.Y+y)
			for x := 0; x < w; x++ {
				// Mirrors color.Gray.RGBA(): r = g = b = Y << 8 | Y.
				v := uint32(src.Pix[sp])
				v |= v << 8
				row[x] = grayPixel(v, v, v, threshold)
				sp++
			}
		}
	default:
		for y := 0; y < h; y++ {
			row := grayImg.Pix[y*grayImg.Stride : y*grayImg.Stride+w]
			for x := 0; x < w; x++ {
				r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				row[x] = grayPixel(r, g, b, threshold)
			}
		}
	}
	return grayImg
}

// grayPixel applies the RGB-to-luminance formula to 16-bit alpha-premultiplied
// components (as returned by color.Color.RGBA) and snaps shades at or above
// threshold to pure white.
func grayPixel(r, g, b uint32, threshold uint8) uint8 {
	lum := uint8((19595*r + 38470*g + 7471*b + 1<<15) >> 24)
	if lum >= threshold {
		return 0xff
	}
	return lum
}
