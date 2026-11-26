package raster

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestEncodeFastRGBAEdgeCases(t *testing.T) {
	cases := []struct {
		name        string
		w, h        int
		knownOpaque bool
		fill        func(img *image.RGBA)
	}{
		{
			name:        "solid_1x70",
			w:           1,
			h:           70,
			knownOpaque: true,
			fill: func(img *image.RGBA) {
				for y := 0; y < 70; y++ {
					img.SetRGBA(0, y, color.RGBA{12, 34, 56, 255})
				}
			},
		},
		{
			name:        "solid_and_stripes_33x80",
			w:           33,
			h:           80,
			knownOpaque: true,
			fill: func(img *image.RGBA) {
				for y := 0; y < 80; y++ {
					for x := 0; x < 33; x++ {
						if y < 25 || (y >= 40 && y < 65) {
							img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
						} else if y == 30 {
							img.SetRGBA(x, y, color.RGBA{uint8(x * 7), uint8(y * 3), uint8((x + y) * 5), 255})
						} else {
							img.SetRGBA(x, y, color.RGBA{20, 40, 60, 255})
						}
					}
				}
			},
		},
		{
			name:        "non_opaque_19x17",
			w:           19,
			h:           17,
			knownOpaque: false,
			fill: func(img *image.RGBA) {
				for y := 0; y < 17; y++ {
					for x := 0; x < 19; x++ {
						a := uint8((x + y) * 7)
						r := uint8((int(a) * ((x * 13) & 255)) / 255)
						g := uint8((int(a) * ((y * 17) & 255)) / 255)
						b := uint8((int(a) * (((x + y) * 11) & 255)) / 255)
						img.SetRGBA(x, y, color.RGBA{r, g, b, a})
					}
				}
			},
		},
		{
			name:        "mixed_widths_128x40",
			w:           128,
			h:           40,
			knownOpaque: true,
			fill: func(img *image.RGBA) {
				for y := 0; y < 40; y++ {
					for x := 0; x < 128; x++ {
						if y%10 < 5 {
							img.SetRGBA(x, y, color.RGBA{240, 240, 240, 255})
						} else if x >= 10 && x < 110 {
							img.SetRGBA(x, y, color.RGBA{uint8((x ^ y) & 255), uint8((x + y) & 255), 99, 255})
						} else {
							img.SetRGBA(x, y, color.RGBA{240, 240, 240, 255})
						}
					}
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, tc.w, tc.h))
			tc.fill(img)
			var buf bytes.Buffer
			if err := encodeFastRGBA(&buf, img, tc.knownOpaque); err != nil {
				t.Fatalf("encodeFastRGBA: %v", err)
			}
			got, err := png.Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("png.Decode: %v", err)
			}
			var wantBuf bytes.Buffer
			if err := png.Encode(&wantBuf, img); err != nil {
				t.Fatalf("stdlib png.Encode: %v", err)
			}
			wantImg, err := png.Decode(bytes.NewReader(wantBuf.Bytes()))
			if err != nil {
				t.Fatalf("stdlib png.Decode: %v", err)
			}
			if got.Bounds() != wantImg.Bounds() {
				t.Fatalf("bounds = %v, want %v", got.Bounds(), wantImg.Bounds())
			}
			for y := 0; y < tc.h; y++ {
				for x := 0; x < tc.w; x++ {
					if got.At(x, y) != wantImg.At(x, y) {
						t.Fatalf("pixel (%d,%d): got %v want %v", x, y, got.At(x, y), wantImg.At(x, y))
					}
				}
			}
		})
	}
}
