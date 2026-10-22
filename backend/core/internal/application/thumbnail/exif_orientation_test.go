package thumbnail

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// buildExifTIFF builds a minimal little-endian TIFF/IFD0 block carrying a single
// EXIF Orientation tag.
func buildExifTIFF(orientation int) []byte {
	tiff := make([]byte, 8+2+12+4)
	copy(tiff[0:2], "II")
	binary.LittleEndian.PutUint16(tiff[2:4], 42)
	binary.LittleEndian.PutUint32(tiff[4:8], 8)  // IFD0 offset
	binary.LittleEndian.PutUint16(tiff[8:10], 1) // entry count
	entry := tiff[10:22]
	binary.LittleEndian.PutUint16(entry[0:2], exifOrientationTag)
	binary.LittleEndian.PutUint16(entry[2:4], 3) // SHORT
	binary.LittleEndian.PutUint32(entry[4:8], 1) // component count
	binary.LittleEndian.PutUint16(entry[8:10], uint16(orientation))
	binary.LittleEndian.PutUint32(tiff[22:26], 0) // next IFD
	return tiff
}

// buildExifApp1 builds a JPEG APP1 segment carrying the EXIF TIFF block.
func buildExifApp1(orientation int) []byte {
	payload := append([]byte("Exif\x00\x00"), buildExifTIFF(orientation)...)
	segLen := 2 + len(payload)
	seg := make([]byte, 2+segLen)
	seg[0] = 0xFF
	seg[1] = 0xE1
	binary.BigEndian.PutUint16(seg[2:4], uint16(segLen))
	copy(seg[4:], payload)
	return seg
}

// encodeQuadrantJPEG encodes a landscape image whose quadrant colours make
// rotation observable: top-left red, top-right green, bottom-left blue,
// bottom-right white.
func encodeQuadrantJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var c color.RGBA
			switch {
			case x < w/2 && y < h/2:
				c = color.RGBA{220, 20, 20, 255}
			case x >= w/2 && y < h/2:
				c = color.RGBA{20, 200, 20, 255}
			case x < w/2 && y >= h/2:
				c = color.RGBA{20, 20, 220, 255}
			default:
				c = color.RGBA{245, 245, 245, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// writeJPEGWithOrientation writes a JPEG whose EXIF Orientation is set to the
// given value (0 = no EXIF segment).
func writeJPEGWithOrientation(t *testing.T, dir string, w, h, orientation int) string {
	t.Helper()
	data := encodeQuadrantJPEG(t, w, h)
	if orientation != 0 {
		app1 := buildExifApp1(orientation)
		inserted := make([]byte, 0, len(data)+len(app1))
		inserted = append(inserted, data[:2]...)
		inserted = append(inserted, app1...)
		inserted = append(inserted, data[2:]...)
		data = inserted
	}
	path := filepath.Join(dir, fmt.Sprintf("test_o%d.jpg", orientation))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write jpeg: %v", err)
	}
	return path
}

// writePNGWithOrientation writes a PNG carrying an eXIf chunk.
func writePNGWithOrientation(t *testing.T, dir string, orientation int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 20, 20))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	data := buf.Bytes()

	tiff := buildExifTIFF(orientation)
	chunk := make([]byte, 0, 12+len(tiff))
	length := uint32(len(tiff))
	chunk = append(chunk, byte(length>>24), byte(length>>16), byte(length>>8), byte(length))
	chunk = append(chunk, 'e', 'X', 'I', 'f')
	chunk = append(chunk, tiff...)
	crc := crc32.ChecksumIEEE(append([]byte("eXIf"), tiff...))
	chunk = append(chunk, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))

	// Insert the eXIf chunk before the trailing IEND chunk (last 12 bytes).
	iend := len(data) - 12
	out := make([]byte, 0, len(data)+len(chunk))
	out = append(out, data[:iend]...)
	out = append(out, chunk...)
	out = append(out, data[iend:]...)

	path := filepath.Join(dir, "test_orientation.png")
	if err := os.WriteFile(path, out, 0644); err != nil {
		t.Fatalf("write png: %v", err)
	}
	return path
}

func TestReadExifOrientation(t *testing.T) {
	dir := t.TempDir()

	for _, orientation := range []int{1, 3, 6, 8} {
		orientation := orientation
		t.Run(fmt.Sprintf("jpeg orientation %d", orientation), func(t *testing.T) {
			path := writeJPEGWithOrientation(t, dir, 40, 30, orientation)
			file, err := os.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer file.Close()
			if got := readExifOrientation(file); got != orientation {
				t.Fatalf("orientation = %d, want %d", got, orientation)
			}
		})
	}

	t.Run("jpeg without exif", func(t *testing.T) {
		path := writeJPEGWithOrientation(t, dir, 40, 30, 0)
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer file.Close()
		if got := readExifOrientation(file); got != 0 {
			t.Fatalf("orientation = %d, want 0", got)
		}
	})

	t.Run("png exif", func(t *testing.T) {
		path := writePNGWithOrientation(t, dir, 6)
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer file.Close()
		if got := readExifOrientation(file); got != 6 {
			t.Fatalf("orientation = %d, want 6", got)
		}
	})
}

// generateThumbnailForTest produces the thumbnail image for the given file.
func generateThumbnailForTest(t *testing.T, path string) image.Image {
	t.Helper()
	svc := &Service{cfg: &Config{MaxSize: 320, Quality: 80, Format: "jpeg"}}
	data, err := svc.generateThumbnail(path)
	if err != nil {
		t.Fatalf("generate thumbnail: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	return img
}

// sampleAt returns the colour at a fraction of the image dimensions.
func sampleAt(img image.Image, fx, fy float64) color.RGBA {
	b := img.Bounds()
	x := b.Min.X + int(float64(b.Dx()-1)*fx)
	y := b.Min.Y + int(float64(b.Dy()-1)*fy)
	r, g, bl, _ := img.At(x, y).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255}
}

func isRed(c color.RGBA) bool { return c.R > 150 && c.G < 120 && c.B < 120 }

func isLight(c color.RGBA) bool { return c.R > 180 && c.G > 180 && c.B > 180 }

func TestGenerateThumbnail_Orientation6SwapsDimensions(t *testing.T) {
	dir := t.TempDir()
	base := generateThumbnailForTest(t, writeJPEGWithOrientation(t, dir, 400, 200, 0))
	rotated := generateThumbnailForTest(t, writeJPEGWithOrientation(t, dir, 400, 200, 6))

	if base.Bounds().Dx() <= base.Bounds().Dy() {
		t.Fatalf("base thumbnail should be landscape, got %dx%d", base.Bounds().Dx(), base.Bounds().Dy())
	}
	if rotated.Bounds().Dx() >= rotated.Bounds().Dy() {
		t.Fatalf("rotated thumbnail should be portrait, got %dx%d", rotated.Bounds().Dx(), rotated.Bounds().Dy())
	}
	if rotated.Bounds().Dx() != base.Bounds().Dy() || rotated.Bounds().Dy() != base.Bounds().Dx() {
		t.Fatalf("dimensions not swapped: base %dx%d, rotated %dx%d",
			base.Bounds().Dx(), base.Bounds().Dy(), rotated.Bounds().Dx(), rotated.Bounds().Dy())
	}
}

func TestGenerateThumbnail_Orientation8SwapsDimensions(t *testing.T) {
	dir := t.TempDir()
	base := generateThumbnailForTest(t, writeJPEGWithOrientation(t, dir, 400, 200, 0))
	rotated := generateThumbnailForTest(t, writeJPEGWithOrientation(t, dir, 400, 200, 8))

	if base.Bounds().Dx() <= base.Bounds().Dy() {
		t.Fatalf("base thumbnail should be landscape, got %dx%d", base.Bounds().Dx(), base.Bounds().Dy())
	}
	if rotated.Bounds().Dx() >= rotated.Bounds().Dy() {
		t.Fatalf("rotated thumbnail should be portrait, got %dx%d", rotated.Bounds().Dx(), rotated.Bounds().Dy())
	}
	if rotated.Bounds().Dx() != base.Bounds().Dy() || rotated.Bounds().Dy() != base.Bounds().Dx() {
		t.Fatalf("dimensions not swapped: base %dx%d, rotated %dx%d",
			base.Bounds().Dx(), base.Bounds().Dy(), rotated.Bounds().Dx(), rotated.Bounds().Dy())
	}
}

func TestGenerateThumbnail_Orientation3Rotates180(t *testing.T) {
	dir := t.TempDir()
	base := generateThumbnailForTest(t, writeJPEGWithOrientation(t, dir, 400, 200, 0))
	rotated := generateThumbnailForTest(t, writeJPEGWithOrientation(t, dir, 400, 200, 3))

	if base.Bounds().Dx() != rotated.Bounds().Dx() || base.Bounds().Dy() != rotated.Bounds().Dy() {
		t.Fatalf("180° rotation must preserve dimensions: base %dx%d, rotated %dx%d",
			base.Bounds().Dx(), base.Bounds().Dy(), rotated.Bounds().Dx(), rotated.Bounds().Dy())
	}

	if !isRed(sampleAt(base, 0.25, 0.25)) {
		t.Fatalf("base top-left should be red, got %v", sampleAt(base, 0.25, 0.25))
	}
	if !isLight(sampleAt(rotated, 0.25, 0.25)) {
		t.Fatalf("rotated top-left should be light after 180°, got %v", sampleAt(rotated, 0.25, 0.25))
	}
	if !isRed(sampleAt(rotated, 0.75, 0.75)) {
		t.Fatalf("rotated bottom-right should be red after 180°, got %v", sampleAt(rotated, 0.75, 0.75))
	}
}

func TestGenerateThumbnail_Orientation1Unrotated(t *testing.T) {
	dir := t.TempDir()
	noExif := generateThumbnailForTest(t, writeJPEGWithOrientation(t, dir, 400, 200, 0))
	oriented := generateThumbnailForTest(t, writeJPEGWithOrientation(t, dir, 400, 200, 1))

	if noExif.Bounds() != oriented.Bounds() {
		t.Fatalf("orientation 1 must not change dimensions: %v vs %v", noExif.Bounds(), oriented.Bounds())
	}
	if !isRed(sampleAt(oriented, 0.25, 0.25)) {
		t.Fatalf("orientation 1 top-left should stay red, got %v", sampleAt(oriented, 0.25, 0.25))
	}
}
