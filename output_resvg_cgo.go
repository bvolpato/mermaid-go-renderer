//go:build cgo

package mermaid

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/xo/resvg"
)

func rasterizeSVGToImageResvg(svg string, width int, height int) (*image.NRGBA, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid image size %dx%d", width, height)
	}
	img, err := resvg.Render(
		[]byte(svg),
		resvg.WithWidth(width),
		resvg.WithHeight(height),
		resvg.WithScaleMode(resvg.ScaleNone),
		resvg.WithLoadSystemFonts(true),
		resvg.WithBackground(color.White),
	)
	if err != nil {
		return nil, fmt.Errorf("resvg render: %w", err)
	}
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	if isZenUMLSVG(svg) && imageNonWhitePixels(out) == 0 {
		if fallbackSVG, ok := buildZenUMLRasterFallbackSVG(svg, width, height); ok {
			if fallbackImg, fallbackErr := resvg.Render(
				[]byte(fallbackSVG),
				resvg.WithWidth(width),
				resvg.WithHeight(height),
				resvg.WithScaleMode(resvg.ScaleNone),
				resvg.WithLoadSystemFonts(true),
				resvg.WithBackground(color.White),
			); fallbackErr == nil {
				draw.Draw(out, out.Bounds(), fallbackImg, fallbackImg.Bounds().Min, draw.Src)
			}
		}
	}
	viewBox, hasViewBox := parseSVGViewBox(svg)
	overlaySVGForeignObjectText(out, svg, width, height, viewBox, hasViewBox)
	return out, nil
}
