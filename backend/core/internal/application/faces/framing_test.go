package faces

import (
	"path/filepath"
	"testing"

	"github.com/flashbacks/core/internal/testutil"
)

// TestImageFrameNormalizeBox pins every EXIF orientation to the upright box a
// browser would show for the same file: the stored photo is 400x200 landscape
// pixels, and values 5-8 hand back a portrait box.
func TestImageFrameNormalizeBox(t *testing.T) {
	// The stored (raw frame) box: a face in the upper-left area.
	const (
		boxX      float32 = 40
		boxY      float32 = 20
		boxWidth  float32 = 100
		boxHeight float32 = 50
	)

	cases := []struct {
		name  string
		wantX float32
		wantY float32
		wantW float32
		wantH float32
	}{
		{name: "orientation 1 keeps the box", wantX: 40, wantY: 20, wantW: 100, wantH: 50},
		{name: "orientation 2 mirrors horizontally", wantX: 260, wantY: 20, wantW: 100, wantH: 50},
		{name: "orientation 3 rotates 180", wantX: 260, wantY: 130, wantW: 100, wantH: 50},
		{name: "orientation 4 mirrors vertically", wantX: 40, wantY: 130, wantW: 100, wantH: 50},
		{name: "orientation 5 transposes", wantX: 20, wantY: 40, wantW: 50, wantH: 100},
		{name: "orientation 6 rotates 90 clockwise", wantX: 130, wantY: 40, wantW: 50, wantH: 100},
		{name: "orientation 7 is transverse", wantX: 130, wantY: 260, wantW: 50, wantH: 100},
		{name: "orientation 8 rotates 270 clockwise", wantX: 20, wantY: 260, wantW: 50, wantH: 100},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frame := ImageFrame{Width: 400, Height: 200, Orientation: index + 1}
			x, y, width, height := frame.NormalizeBox(boxX, boxY, boxWidth, boxHeight)

			if x != tc.wantX || y != tc.wantY || width != tc.wantW || height != tc.wantH {
				t.Fatalf(
					"box = (%v, %v, %v, %v), want (%v, %v, %v, %v)",
					x, y, width, height, tc.wantX, tc.wantY, tc.wantW, tc.wantH,
				)
			}
		})
	}
}

// TestImageFrameNormalizeBox_IdentityFrame guards the fallback both callers rely
// on: a frame that could not be probed leaves stored coordinates untouched.
func TestImageFrameNormalizeBox_IdentityFrame(t *testing.T) {
	frame := ImageFrame{} // orientation 0 and zero size: nothing is known
	x, y, width, height := frame.NormalizeBox(1, 2, 3, 4)
	if x != 1 || y != 2 || width != 3 || height != 4 {
		t.Fatalf("box = (%v, %v, %v, %v), want the stored (1, 2, 3, 4)", x, y, width, height)
	}
}

// TestReadImageFrame proves the probe reports the raw stored size plus the EXIF
// Orientation, and degrades to the identity frame instead of failing.
func TestReadImageFrame(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing file", func(t *testing.T) {
		frame := ReadImageFrame(filepath.Join(dir, "missing.jpg"))
		if frame.Orientation != 1 || frame.Width != 0 || frame.Height != 0 {
			t.Fatalf("frame = %+v, want the identity frame", frame)
		}
	})

	t.Run("rotated file", func(t *testing.T) {
		path := filepath.Join(dir, "portrait.jpg")
		testutil.WriteJPEGWithOrientation(t, path, 400, 200, 6)

		frame := ReadImageFrame(path)
		if frame.Width != 400 || frame.Height != 200 || frame.Orientation != 6 {
			t.Fatalf("frame = %+v, want 400x200 orientation 6", frame)
		}
	})

	t.Run("file without an orientation tag", func(t *testing.T) {
		path := filepath.Join(dir, "plain.jpg")
		testutil.WriteJPEGWithOrientation(t, path, 40, 30, 0)

		frame := ReadImageFrame(path)
		if frame.Width != 40 || frame.Height != 30 || frame.Orientation != 1 {
			t.Fatalf("frame = %+v, want 40x30 orientation 1", frame)
		}
	})
}
