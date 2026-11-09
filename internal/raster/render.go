// The functions of this file follow parts of Blink, which their comments
// name. The license of each Blink file is the license in its header at
// Chromium 156.0.8078.4. The paths are in third_party/blink/renderer:
//
//   - SVGPreserveAspectRatio::ComputeTransform:
//     core/svg/svg_preserve_aspect_ratio.cc (LGPL-2.0-or-later).
//   - LayoutSVGRoot::ComputeNaturalSizingInfo. At that version the function
//     with this name is LayoutReplaced::ComputeNaturalSizingInfo in
//     core/layout/layout_replaced.cc (LGPL-2.0-or-later). The class
//     LayoutSVGRoot is in core/layout/svg/layout_svg_root.cc
//     (LGPL-2.0-or-later).
//   - BlockSizeFromAspectRatio: core/layout/length_utils.cc (BSD-3-Clause).
//   - LayoutRatioFromSizeF: platform/geometry/physical_size.cc
//     (BSD-3-Clause).
//   - LayoutUnit: platform/geometry/layout_unit.h (BSD-3-Clause, with the
//     copyright of Google in the file).
//
// See NOTICE in this directory and finding F1 of docs/LICENSE-AUDIT.md.

// Package raster paints the SVG that mermaid produces into a bitmap, without
// a browser. Its target is the pixels that headless Chromium paints for the
// same markup.
//
// Chromium rasterizes in software with Skia. The package ports the parts of
// Skia that decide those pixels (the analytic anti-aliasing scan converter,
// the stroker, the dasher, hairlines, the 32-bit blitters) and follows the
// SVG painters of Blink for what is drawn and in which order. Text and HTML
// layout come from internal/browser, glyph outlines from internal/font.
package raster

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// Options control a render.
type Options struct {
	// Scale is the device scale factor: the number of bitmap pixels per CSS
	// pixel. Zero means 1. With the fonts of Linux a scale above 1 also
	// selects the text of Chromium at such a scale factor: outlines without
	// hinting, and the font metrics of subpixel positioning.
	Scale float64
	// Fonts is the font library that lays out and paints the text. Nil is
	// the library of the system. The text must be painted with the library
	// that laid out the SVG: font.Embedded() for an SVG that was made with
	// the embedded fonts. The platform of the library selects the model of
	// the glyph rasterizer: CoreText for macOS, Skia for Linux.
	Fonts *font.Library
	// Background is painted below the SVG. Nil means opaque white. A color
	// with zero alpha leaves the bitmap transparent.
	Background color.Color
	// PageSheets has the text of the style sheets in the head of the page
	// that laid out the SVG. See dom.Document.PageSheets.
	PageSheets []string
}

// Render parses markup as a browser parses inline SVG in an HTML document
// and paints its first svg element. The bitmap has the size that the SVG
// takes in a page at its natural size, with the edges rounded up to whole
// CSS pixels, times the scale.
func Render(svg string, opts Options) (*image.RGBA, error) {
	doc, err := dom.ParseBody(svg)
	if err != nil {
		return nil, err
	}
	doc.PageSheets = opts.PageSheets
	eng := browser.NewWithFonts(opts.Fonts)
	// The page of the screenshot: 16000 CSS pixels wide, without margins.
	eng.ViewportWidth = pageWidth
	eng.BodyMargin = 0
	doc.Engine = eng
	var root *dom.Node
	for c := doc.Body.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.ElementNode && c.NS == dom.NSSVG && c.Tag == "svg" {
			root = c
			break
		}
	}
	if root == nil {
		return nil, errors.New("raster: no svg element")
	}
	scale := opts.Scale
	if !(scale > 0) {
		scale = 1
	}
	sz := rootSize(eng, root)
	pngW := int(math.Round(float64(sz.cssW) * scale))
	pngH := int(math.Round(float64(sz.cssH) * scale))
	if pngW <= 0 || pngH <= 0 {
		return nil, errors.New("raster: empty svg")
	}
	const maxPixels = 1 << 28
	if int64(pngW)*int64(pngH) > maxPixels {
		return nil, errors.New("raster: svg too large")
	}
	p := newPainter(eng, doc, pngW, pngH, scale)
	p.fillBackground(opts.Background)
	p.paintRoot(root, sz)
	p.cv.release()

	base := p.cv.stack[0].pm
	img := &image.RGBA{Pix: base.pix, Stride: base.stride, Rect: image.Rect(0, 0, base.w, base.h)}
	return img, nil
}

// EncodePNG writes the image as a PNG file.
func EncodePNG(w io.Writer, img image.Image) error {
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	return enc.Encode(w, img)
}

// pageWidth is the width in CSS pixels of the page that holds the SVG. The
// oracle uses a wide page so that a percentage width resolves to the
// natural width.
const pageWidth = 16000

// svgRootSize is the layout of the root svg element in a page.
type svgRootSize struct {
	// cssW and cssH are the client size rounded up to whole CSS pixels.
	// The bitmap has scale times as many pixels.
	cssW, cssH int
	// boxW and boxH are the border box in CSS pixels, as LayoutUnits
	// (1/64 px).
	boxW, boxH int64
	// viewBox is the viewBox, when it is valid.
	viewBox    [4]float32
	hasViewBox bool
	par        string
}

const layoutUnit = 64

// luTrunc is the LayoutUnit(float) constructor: it truncates.
func luTrunc(v float32) int64 {
	return int64(float32(v * layoutUnit))
}

// rootSize computes the size of the root svg element as Blink does for a
// block-level replaced element in a page of 16000 CSS pixels:
//
//   - width and height come from the attributes, which are presentation
//     attributes, and from the style attribute. A percentage width resolves
//     against the page width, limited by max-width.
//   - a missing height comes from the aspect ratio of the viewBox
//     (BlockSizeFromAspectRatio), and from the default size 300 x 150
//     without a viewBox.
//
// The layout is in CSS pixels at every scale: the screenshots at scale 2
// show a box that is snapped to whole CSS pixels and then doubled.
func rootSize(eng *browser.Engine, root *dom.Node) svgRootSize {
	var sz svgRootSize
	if vb := browser.RasterParseNumbers(root.AttrOr("viewBox", "")); len(vb) == 4 && vb[2] > 0 && vb[3] > 0 {
		copy(sz.viewBox[:], vb)
		sz.hasViewBox = true
	}
	sz.par = strings.TrimSpace(root.AttrOr("preserveAspectRatio", ""))

	zoom := func(px float64) int64 { return luTrunc(float32(px)) }

	// length resolves a width or height value. percent reports a
	// percentage.
	length := func(prop string) (px float64, percent, ok bool) {
		v, has := eng.RasterDeclared(root, prop)
		if !has {
			if a, hasAttr := root.GetAttr(prop); hasAttr {
				v, has = a, true
			}
		}
		if !has {
			return 0, false, false
		}
		v = strings.TrimSpace(strings.ToLower(v))
		if v == "" || v == "auto" || v == "none" {
			return 0, false, false
		}
		if strings.HasSuffix(v, "%") {
			f, ok := parseFloat(v[:len(v)-1])
			return f, true, ok
		}
		f, ok := eng.RasterLength(root, v, true)
		return f, false, ok
	}

	var natW, natH int64
	hasNatW, hasNatH := false, false
	wPx, wPct, wOK := length("width")
	hPx, hPct, hOK := length("height")
	if wOK && !wPct {
		natW, hasNatW = zoom(wPx), true
	}
	if hOK && !hPct {
		natH, hasNatH = zoom(hPx), true
	}

	// The aspect ratio: the natural size when both sides are known, the
	// viewBox otherwise (LayoutSVGRoot::ComputeNaturalSizingInfo and
	// LayoutRatioFromSizeF).
	var arW, arH int64
	switch {
	case hasNatW && hasNatH && natW > 0 && natH > 0:
		arW, arH = layoutRatio(float32(wPx), float32(hPx))
	case sz.hasViewBox:
		arW, arH = layoutRatio(sz.viewBox[2], sz.viewBox[3])
	}

	// Inline size.
	var w int64
	wKnown := false
	switch {
	case wOK && wPct:
		w = int64(float64(zoom(pageWidth)) * wPx / 100)
		wKnown = true
	case hasNatW:
		w, wKnown = natW, true
	}
	if maxPx, pct, ok := length("max-width"); ok && !pct {
		if m := zoom(maxPx); wKnown && w > m {
			w = m
		}
	}
	// Block size.
	var h int64
	hKnown := false
	if hasNatH {
		h, hKnown = natH, true
	}
	switch {
	case wKnown && hKnown:
	case wKnown:
		if arW > 0 && arH > 0 {
			h = w * arH / arW
		} else {
			h = zoom(150)
		}
	case hKnown:
		if arW > 0 && arH > 0 {
			w = h * arW / arH
		} else {
			w = zoom(300)
		}
	default:
		// No size at all: an aspect ratio stretches to the page, and the
		// default object size applies without one.
		if arW > 0 && arH > 0 {
			w = zoom(pageWidth)
			if maxPx, pct, ok := length("max-width"); ok && !pct {
				if m := zoom(maxPx); w > m {
					w = m
				}
			}
			h = w * arH / arW
		} else {
			w, h = zoom(300), zoom(150)
		}
	}
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	sz.boxW, sz.boxH = w, h
	// getBoundingClientRect is in CSS pixels. The screenshot rounds it up
	// and the bitmap has scale times as many pixels.
	clientW := float64(w) / layoutUnit
	clientH := float64(h) / layoutUnit
	sz.cssW = int(max(1, math.Ceil(clientW)))
	sz.cssH = int(max(1, math.Ceil(clientH)))
	return sz
}

// layoutRatio is LayoutRatioFromSizeF: the aspect ratio as two LayoutUnit
// raw values. A ratio that LayoutUnits cannot hold exactly is approximated
// by a continued fraction.
func layoutRatio(width, height float32) (int64, int64) {
	w, h := luTrunc(width), luTrunc(height)
	if (float32(w)/layoutUnit == width && float32(h)/layoutUnit == height) || !(width > 0 && height > 0) {
		return w, h
	}
	if width == height {
		return layoutUnit, layoutUnit
	}
	initial := width / height
	x := initial
	clamp := func(v int64) int64 {
		if v > math.MaxInt32 {
			return math.MaxInt32
		}
		if v < math.MinInt32 {
			return math.MinInt32
		}
		return v
	}
	var h0, h1, k0, k1 int64 = 0, 1, 1, 0
	for i := 0; i < 16; i++ {
		if math.IsInf(float64(x), 0) || math.IsNaN(float64(x)) {
			break
		}
		estimate := float32(h1) / float32(k1)
		if float32(math.Abs(float64(initial-estimate))) < 0.000001 {
			break
		}
		a := int64(math.Floor(float64(x)))
		a = clamp(a)
		h2 := clamp(clamp(h1*a) + h0)
		k2 := clamp(clamp(k1*a) + k0)
		if h2 == math.MaxInt32 || k2 == math.MaxInt32 {
			break
		}
		h0, k0, h1, k1 = h1, k1, h2, k2
		x = 1 / (x - float32(a))
	}
	if h1 == 0 || k1 == 0 {
		return w, h
	}
	return h1, k1
}

func parseFloat(s string) (float64, bool) {
	v := browser.RasterParseNumbers(strings.TrimSpace(s))
	if len(v) != 1 {
		return 0, false
	}
	return float64(v[0]), true
}

// viewBoxTransform is SVGPreserveAspectRatio::ComputeTransform: the
// transform from the viewBox to a viewport of the given size.
func viewBoxTransform(vb [4]float32, par string, vw, vh float32) Affine {
	lx, ly := float64(vb[0]), float64(vb[1])
	lw, lh := float64(vb[2]), float64(vb[3])
	pw, ph := float64(vw), float64(vh)
	align, slice := parsePAR(par)
	if align == "none" {
		return Affine{A: pw / lw, D: ph / lh}.Translate(-lx, -ly)
	}
	logicalRatio := lw / lh
	physicalRatio := pw / ph
	xAlign := align[:4] // xMin, xMid, xMax
	yAlign := align[4:] // YMin, YMid, YMax
	if (logicalRatio < physicalRatio && !slice) || (logicalRatio >= physicalRatio && slice) {
		m := Affine{A: ph / lh, D: ph / lh}
		switch xAlign {
		case "xMin":
			return m.Translate(-lx, -ly)
		case "xMid":
			return m.Translate(-lx-float64((lw-float64(pw*lh)/ph)/2), -ly)
		default:
			return m.Translate(-lx-(lw-float64(pw*lh)/ph), -ly)
		}
	}
	m := Affine{A: pw / lw, D: pw / lw}
	switch yAlign {
	case "YMin":
		return m.Translate(-lx, -ly)
	case "YMid":
		return m.Translate(-lx, -ly-float64((lh-float64(ph*lw)/pw)/2))
	default:
		return m.Translate(-lx, -ly-(lh-float64(ph*lw)/pw))
	}
}

// parsePAR parses preserveAspectRatio. The default is xMidYMid meet.
func parsePAR(s string) (align string, slice bool) {
	align = "xMidYMid"
	fields := strings.Fields(s)
	if len(fields) > 0 && fields[0] == "defer" {
		fields = fields[1:]
	}
	if len(fields) > 0 {
		switch fields[0] {
		case "none", "xMinYMin", "xMidYMin", "xMaxYMin", "xMinYMid", "xMidYMid", "xMaxYMid", "xMinYMax", "xMidYMax", "xMaxYMax":
			align = fields[0]
		default:
			return "xMidYMid", false
		}
		if len(fields) > 1 && fields[1] == "slice" {
			slice = true
		}
	}
	return align, slice
}

// colorToNRGBA converts a color to straight alpha bytes.
func colorToNRGBA(c color.Color) (r, g, b, a uint8) {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return n.R, n.G, n.B, n.A
}

// cssAlphaByte converts the alpha of a CSS color times an opacity to a byte,
// as SkColor4f::toSkColor rounds it.
func cssAlphaByte(c css.Color, opacity float32) uint8 {
	a := float32(float32(c.A) * opacity)
	if !(a > 0) {
		return 0
	}
	if a >= 1 {
		return 255
	}
	return uint8(float32(a*255) + 0.5)
}
