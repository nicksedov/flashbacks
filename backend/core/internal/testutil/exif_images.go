package testutil

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"os"
	"testing"
)

// exifOrientationTag is the EXIF tag id for Orientation (IFD0 tag 0x0112).
const exifOrientationTag = 0x0112

// WriteJPEGWithOrientation writes a JPEG of the given raw pixel size carrying the
// EXIF Orientation tag set to orientation (1-8, or 0 to omit the tag). It mirrors
// what a camera writes when it stores a portrait photo as landscape pixels.
func WriteJPEGWithOrientation(t *testing.T, path string, width, height, orientation int) {
	t.Helper()

	source := image.NewRGBA(image.Rect(0, 0, width, height))
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	data := encoded.Bytes()

	if orientation < 1 || orientation > 8 {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write jpeg %s: %v", path, err)
		}
		return
	}

	// The APP1 segment must follow the two-byte SOI marker and precede the rest of
	// the stream, otherwise a reader stops at SOS without seeing it.
	segment := buildExifApp1(orientation)
	out := make([]byte, 0, len(data)+len(segment))
	out = append(out, data[:2]...)
	out = append(out, segment...)
	out = append(out, data[2:]...)

	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write jpeg %s: %v", path, err)
	}
}

// buildExifTIFF builds a minimal little-endian TIFF/IFD0 block carrying a single
// EXIF Orientation tag.
func buildExifTIFF(orientation int) []byte {
	tiff := make([]byte, 8+2+12+4)
	copy(tiff[0:8], []byte{'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00})
	binary.LittleEndian.PutUint16(tiff[8:10], 1) // one IFD entry

	entry := tiff[10:22]
	binary.LittleEndian.PutUint16(entry[0:2], exifOrientationTag)
	binary.LittleEndian.PutUint16(entry[2:4], 3) // SHORT
	binary.LittleEndian.PutUint32(entry[4:8], 1) // component count
	binary.LittleEndian.PutUint16(entry[8:10], uint16(orientation))
	binary.LittleEndian.PutUint32(tiff[22:26], 0) // next IFD
	return tiff
}

// buildExifApp1 wraps the TIFF block in a JPEG APP1 "Exif" segment.
func buildExifApp1(orientation int) []byte {
	payload := append([]byte("Exif\x00\x00"), buildExifTIFF(orientation)...)
	length := 2 + len(payload)

	segment := make([]byte, 0, 4+len(payload))
	segment = append(segment, 0xFF, 0xE1, byte(length>>8), byte(length))
	segment = append(segment, payload...)
	return segment
}
