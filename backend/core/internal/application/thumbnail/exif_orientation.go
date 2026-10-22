package thumbnail

import (
	"encoding/binary"
	"image"
	"io"
	"os"

	"github.com/disintegration/imaging"
)

const (
	// exifOrientationTag is the EXIF tag id for Orientation (IFD0 tag 0x0112).
	exifOrientationTag = 0x0112
	// pngSignature is the 8-byte PNG file heading.
	pngSignature = "\x89PNG\r\n\x1a\n"
	// maxPNGChunkLen bounds the size of a chunk we are willing to read into
	// memory, protecting against corrupt length fields.
	maxPNGChunkLen = 1 << 20
)

// readExifOrientation returns the EXIF Orientation value (1-8) stored in the
// image file, or 0 when the file has no orientation tag or cannot be parsed.
// It supports JPEG (APP1 "Exif" segment) and PNG (eXIf chunk) and reads only the
// format headers, so the file is left at an arbitrary offset afterwards.
func readExifOrientation(file *os.File) int {
	if file == nil {
		return 0
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0
	}

	var head [8]byte
	n, err := io.ReadFull(file, head[:])
	if err != nil && n < 2 {
		return 0
	}

	switch {
	case n >= 2 && head[0] == 0xFF && head[1] == 0xD8:
		return readJPEGOrientation(file)
	case n >= 8 && string(head[:8]) == pngSignature:
		return readPNGOrientation(file)
	default:
		return 0
	}
}

// readJPEGOrientation walks the JPEG segment headers from the SOI marker and
// returns the orientation from the first APP1 "Exif" segment, stopping at SOS.
func readJPEGOrientation(file *os.File) int {
	if _, err := file.Seek(2, io.SeekStart); err != nil {
		return 0
	}

	for {
		prefix, err := readByte(file)
		if err != nil {
			return 0
		}
		if prefix != 0xFF {
			return 0
		}

		marker, err := readByte(file)
		if err != nil {
			return 0
		}
		// A marker may be preceded by any number of 0xFF fill bytes.
		for marker == 0xFF {
			marker, err = readByte(file)
			if err != nil {
				return 0
			}
		}

		switch {
		case marker == 0xD9 || marker == 0xDA: // EOI or start of scan
			return 0
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7):
			continue // standalone marker without a payload
		}

		size, err := readSegmentLength(file)
		if err != nil {
			return 0
		}
		payloadLen := size - 2
		if payloadLen < 0 {
			return 0
		}

		if marker == 0xE1 { // APP1
			payload := make([]byte, payloadLen)
			if _, err := io.ReadFull(file, payload); err != nil {
				return 0
			}
			if len(payload) >= 6 && string(payload[:6]) == "Exif\x00\x00" {
				return parseTIFFOrientation(payload[6:])
			}
			continue
		}

		if _, err := file.Seek(int64(payloadLen), io.SeekCurrent); err != nil {
			return 0
		}
	}
}

// readPNGOrientation walks PNG chunks from the signature and returns the
// orientation from the eXIf chunk, stopping at IEND.
func readPNGOrientation(file *os.File) int {
	if _, err := file.Seek(int64(len(pngSignature)), io.SeekStart); err != nil {
		return 0
	}

	var hdr [8]byte
	for {
		if _, err := io.ReadFull(file, hdr[:]); err != nil {
			return 0
		}
		length := int(binary.BigEndian.Uint32(hdr[:4]))
		chunkType := string(hdr[4:8])

		switch chunkType {
		case "eXIf":
			if length <= 0 || length > maxPNGChunkLen {
				return 0
			}
			data := make([]byte, length)
			if _, err := io.ReadFull(file, data); err != nil {
				return 0
			}
			return parseTIFFOrientation(data)
		case "IEND":
			return 0
		}

		// Skip the chunk data plus the 4-byte CRC.
		if _, err := file.Seek(int64(length)+4, io.SeekCurrent); err != nil {
			return 0
		}
	}
}

// parseTIFFOrientation reads the Orientation tag from an EXIF TIFF block
// (as carried by a JPEG APP1 payload or a PNG eXIf chunk).
func parseTIFFOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 0
	}

	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}

	if order.Uint16(tiff[2:4]) != 42 {
		return 0
	}

	ifdOffset := int(order.Uint32(tiff[4:8]))
	if ifdOffset < 8 || ifdOffset+2 > len(tiff) {
		return 0
	}

	count := int(order.Uint16(tiff[ifdOffset : ifdOffset+2]))
	entry := ifdOffset + 2
	for i := 0; i < count; i++ {
		if entry+12 > len(tiff) {
			return 0
		}
		tag := order.Uint16(tiff[entry : entry+2])
		if tag == exifOrientationTag {
			typ := order.Uint16(tiff[entry+2 : entry+4])
			valueCount := order.Uint32(tiff[entry+4 : entry+8])
			// Orientation is a single SHORT (type 3).
			if typ != 3 || valueCount < 1 {
				return 0
			}
			value := order.Uint16(tiff[entry+8 : entry+10])
			if value >= 1 && value <= 8 {
				return int(value)
			}
			return 0
		}
		entry += 12
	}

	return 0
}

// applyOrientation returns the image transformed to the upright result a browser
// produces when it renders the same file, following the standard EXIF
// Orientation mapping: `3` rotates 180°, `6` rotates 90° clockwise, `8` rotates
// 270° clockwise, and the mirrored values (`2` horizontal, `4` vertical, `5`
// transpose, `7` transverse) are flipped as well. Unknown values are returned
// unchanged.
//
// The imaging helpers rotate counter-clockwise, so the clockwise 90° required by
// orientation 6 is imaging.Rotate270, and the clockwise 270° required by
// orientation 8 is imaging.Rotate90. Using them the other way round flips the
// result by 180°.
func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		// Transpose: reflect across the main diagonal.
		return imaging.FlipH(imaging.Rotate270(img))
	case 6:
		return imaging.Rotate270(img)
	case 7:
		// Transverse: reflect across the anti-diagonal.
		return imaging.FlipH(imaging.Rotate90(img))
	case 8:
		return imaging.Rotate90(img)
	default:
		return img
	}
}

func readByte(file *os.File) (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(file, b[:])
	return b[0], err
}

func readSegmentLength(file *os.File) (int, error) {
	var b [2]byte
	if _, err := io.ReadFull(file, b[:]); err != nil {
		return 0, err
	}
	return int(binary.BigEndian.Uint16(b[:])), nil
}
