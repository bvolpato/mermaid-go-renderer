package mermaid

import (
	"bytes"
	"context"
	"image"
	"image/draw"
	"image/png"
	"time"
)

func withTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// cropWhitespace trims white border pixels from a PNG image.
func cropWhitespace(data []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			r8, g8, b8, a8 := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
			if a8 < 10 || (r8 > 250 && g8 > 250 && b8 > 250) {
				continue
			}
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}

	if maxX <= minX || maxY <= minY {
		return data, nil
	}

	pad := 10
	minX = max(bounds.Min.X, minX-pad)
	minY = max(bounds.Min.Y, minY-pad)
	maxX = min(bounds.Max.X-1, maxX+pad)
	maxY = min(bounds.Max.Y-1, maxY+pad)

	cropRect := image.Rect(0, 0, maxX-minX+1, maxY-minY+1)
	cropped := image.NewRGBA(cropRect)
	draw.Draw(cropped, cropRect, img, image.Pt(minX, minY), draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
