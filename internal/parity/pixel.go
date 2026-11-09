package parity

import (
	"fmt"
	"image"
	"image/png"
	"os"
)

// PixelDiff is the result of comparing two PNG files pixel by pixel.
type PixelDiff struct {
	// SameSize is false when the images have different dimensions.
	SameSize bool
	Width    int
	Height   int
	// Different is the number of pixels with any channel difference.
	Different int
	// MaxDelta is the largest difference of one channel, 0 to 255.
	MaxDelta int
}

// Identical reports whether the two images are the same, pixel for pixel.
func (d PixelDiff) Identical() bool { return d.SameSize && d.Different == 0 }

func (d PixelDiff) String() string {
	if !d.SameSize {
		return "different size"
	}
	if d.Different == 0 {
		return fmt.Sprintf("identical (%dx%d)", d.Width, d.Height)
	}
	return fmt.Sprintf("%d of %d pixels differ, largest channel difference %d", d.Different, d.Width*d.Height, d.MaxDelta)
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// ComparePNG compares two PNG files pixel by pixel.
func ComparePNG(gotPath, wantPath string) (PixelDiff, error) {
	got, err := loadPNG(gotPath)
	if err != nil {
		return PixelDiff{}, err
	}
	want, err := loadPNG(wantPath)
	if err != nil {
		return PixelDiff{}, err
	}
	gb, wb := got.Bounds(), want.Bounds()
	d := PixelDiff{Width: wb.Dx(), Height: wb.Dy()}
	if gb.Dx() != wb.Dx() || gb.Dy() != wb.Dy() {
		return d, nil
	}
	d.SameSize = true
	for y := 0; y < wb.Dy(); y++ {
		for x := 0; x < wb.Dx(); x++ {
			r1, g1, b1, a1 := got.At(gb.Min.X+x, gb.Min.Y+y).RGBA()
			r2, g2, b2, a2 := want.At(wb.Min.X+x, wb.Min.Y+y).RGBA()
			delta := max(absDiff(r1, r2), absDiff(g1, g2), absDiff(b1, b2), absDiff(a1, a2)) >> 8
			if delta > 0 || r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				d.Different++
				if int(delta) > d.MaxDelta {
					d.MaxDelta = int(delta)
				}
			}
		}
	}
	return d, nil
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
