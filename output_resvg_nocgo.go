//go:build !cgo || nocgo_rasterizer

package mermaid

import "image"

func rasterizeSVGToImageResvg(svg string, width int, height int) (*image.NRGBA, error) {
	return rasterizeSVGToImageLegacy(svg, width, height)
}
