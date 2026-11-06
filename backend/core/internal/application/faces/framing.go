package faces

import (
	"image"
	_ "image/jpeg" // decoders for the formats the gallery stores
	_ "image/png"
	"io"
	"os"

	"github.com/flashbacks/core/internal/application/thumbnail"
)

// ImageFrame is the pixel frame a stored face box is expressed in: the raw
// stored pixels of the file plus the EXIF Orientation a browser applies when it
// renders that same file.
//
// Face detection reads the stored bytes as they are (the analyser does not
// honour EXIF), so boxes are stored in the raw frame. `/api/image` hands the
// untouched file to the browser, and the browser rotates it upright, so a box
// has to be mapped through the orientation before it is drawn over the photo.
type ImageFrame struct {
	// Width and Height are the raw stored pixel size, before any rotation.
	Width  int
	Height int
	// Orientation is the EXIF Orientation (1-8); 1 means "as stored".
	Orientation int
}

// ReadImageFrame probes an image file for its raw pixel size and EXIF
// Orientation.
//
// Anything unreadable — a missing file, an unknown or corrupt format, a missing
// or invalid orientation tag — yields the identity frame (orientation 1), which
// NormalizeBox treats as "leave the box alone". A probe never fails a request:
// the worst case is the unrescaled box the client drew before.
func ReadImageFrame(path string) ImageFrame {
	frame := ImageFrame{Orientation: 1}

	file, err := os.Open(path)
	if err != nil {
		return frame
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return frame
	}
	frame.Width, frame.Height = config.Width, config.Height

	// The orientation tag lives in the headers, so the probe has to rewind after
	// DecodeConfig consumed them.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return frame
	}
	if orientation := thumbnail.ReadExifOrientation(file); orientation >= 2 && orientation <= 8 {
		frame.Orientation = orientation
	}
	return frame
}

// NormalizeBox maps a box stored in the raw frame into the upright frame the
// browser renders for the same file. It follows the standard EXIF Orientation
// mapping: 3 rotates 180°, 6 rotates 90° clockwise, 8 rotates 270° clockwise,
// and 2/4/5/7 mirror; the width and height swap for 5-8, where the stored image
// is sideways.
//
// The identity frame and an unknown orientation return the box unchanged, so a
// caller can always normalize without checking the probe result first.
func (f ImageFrame) NormalizeBox(x, y, width, height float32) (float32, float32, float32, float32) {
	if f.Orientation < 2 || f.Orientation > 8 || f.Width <= 0 || f.Height <= 0 {
		return x, y, width, height
	}

	rawWidth, rawHeight := float32(f.Width), float32(f.Height)

	// Every EXIF orientation is an axis-aligned rotation or mirror, so mapping two
	// opposite corners and re-ordering them is enough for the whole box.
	mapPoint := func(px, py float32) (float32, float32) {
		switch f.Orientation {
		case 2: // mirror horizontally
			return rawWidth - px, py
		case 3: // rotate 180°
			return rawWidth - px, rawHeight - py
		case 4: // mirror vertically
			return px, rawHeight - py
		case 5: // transpose
			return py, px
		case 6: // rotate 90° clockwise
			return rawHeight - py, px
		case 7: // transverse
			return rawHeight - py, rawWidth - px
		default: // 8: rotate 270° clockwise
			return py, rawWidth - px
		}
	}

	left, top := mapPoint(x, y)
	right, bottom := mapPoint(x+width, y+height)
	return min(left, right), min(top, bottom), absDiff(right, left), absDiff(bottom, top)
}

// absDiff returns the absolute difference of two box edges.
func absDiff(first, second float32) float32 {
	if first < second {
		return second - first
	}
	return first - second
}
