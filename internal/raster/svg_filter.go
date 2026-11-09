// Filters: the filter property with a reference to a filter element
// (feDropShadow), and the CSS functions drop-shadow and brightness.
//
// The structure follows Blink and cc: FilterEffectBuilder::
// BuildFilterOperations and SVGElementResourceClient::UpdateFilterData
// (third_party/blink/renderer/core/paint/filter_effect_builder.cc,
// core/layout/svg/svg_resources.cc), FEDropShadow::CreateImageFilter,
// paint_filter_builder::Build, and RenderSurfaceFilters::BuildImageFilter
// (cc/paint/render_surface_filters.cc). The image filters follow Skia
// (branch chrome/m156): SkDropShadowImageFilter.cpp, SkCropImageFilter.cpp
// and the CPU blur of src/core/SkBlurEngine.cpp (svg_blur.go).
//
// The license of each file of Chromium is the license in its header at
// Chromium 156.0.8078.4. The Blink paths are in third_party/blink/renderer:
//
//   - core/paint/filter_effect_builder.cc: BSD-2-Clause, with the copyright
//     of Apple and Google in the file.
//   - core/layout/svg/svg_resources.cc: LGPL-2.0-or-later.
//   - FEDropShadow::CreateImageFilter:
//     platform/graphics/filters/fe_drop_shadow.cc (LGPL-2.0-or-later).
//   - paint_filter_builder::Build:
//     platform/graphics/filters/paint_filter_builder.cc (BSD-2-Clause, with
//     the copyright of Google in the file).
//   - cc/paint/render_surface_filters.cc: BSD-3-Clause.
//
// See NOTICE in this directory and finding F1 of docs/LICENSE-AUDIT.md.

package raster

import (
	"math"
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

type filterKind uint8

const (
	// filterReference is url(#id): a filter element.
	filterReference filterKind = iota
	// filterDropShadow is the CSS function drop-shadow.
	filterDropShadow
	// filterBrightness is the CSS function brightness.
	filterBrightness
)

// filterOp is one function of a filter property.
type filterOp struct {
	kind filterKind
	// node is the filter element of a reference.
	node *dom.Node
	// dx, dy and sigma are the offset and the standard deviation of a drop
	// shadow in user units.
	dx, dy, sigma float32
	color         css.Color
	// amount is the factor of brightness.
	amount float32
}

// resolveFilter returns the functions of the filter property of an
// element, or nil.
func (p *painter) resolveFilter(n *dom.Node) []filterOp {
	v, ok := p.eng.RasterDeclared(n, "filter")
	if !ok {
		return nil
	}
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "none") {
		return nil
	}
	var ops []filterOp
	for v != "" {
		open := strings.IndexByte(v, '(')
		if open < 0 {
			return nil
		}
		// The matching parenthesis: a color function nests one level.
		depth, end := 0, -1
		for i := open; i < len(v); i++ {
			if v[i] == '(' {
				depth++
			} else if v[i] == ')' {
				depth--
				if depth == 0 {
					end = i
					break
				}
			}
		}
		if end < 0 {
			return nil
		}
		name := strings.ToLower(strings.TrimSpace(v[:open]))
		args := strings.TrimSpace(v[open+1 : end])
		v = strings.TrimSpace(v[end+1:])
		switch name {
		case "url":
			id := parseURLRef("url(" + args + ")")
			f := p.ids[id]
			if f == nil || f.NS != dom.NSSVG || f.Tag != "filter" {
				// A reference to nothing has no effect.
				continue
			}
			ops = append(ops, filterOp{kind: filterReference, node: f})
		case "drop-shadow":
			op, ok := p.parseDropShadow(n, args)
			if !ok {
				return nil
			}
			ops = append(ops, op)
		case "brightness":
			amount := float32(1)
			if args != "" {
				f, ok := parseNumberOrPercent(args)
				if !ok || f < 0 {
					return nil
				}
				amount = f
			}
			ops = append(ops, filterOp{kind: filterBrightness, amount: amount})
		default:
			// A function that is not implemented: the element is painted
			// without the filter.
			return nil
		}
	}
	return ops
}

func parseNumberOrPercent(s string) (float32, bool) {
	s = strings.TrimSpace(s)
	pct := strings.HasSuffix(s, "%")
	if pct {
		s = s[:len(s)-1]
	}
	nums := browser.RasterParseNumbers(s)
	if len(nums) != 1 {
		return 0, false
	}
	if pct {
		return nums[0] / 100, true
	}
	return nums[0], true
}

// parseDropShadow parses the arguments of drop-shadow: a color and two or
// three lengths, in any order of color and lengths. The third length is
// the standard deviation.
func (p *painter) parseDropShadow(n *dom.Node, args string) (filterOp, bool) {
	op := filterOp{kind: filterDropShadow, color: css.Color{A: 1}}
	// Take a color function out first: it has spaces and commas.
	rest := args
	hasColor := false
	if i := strings.Index(strings.ToLower(rest), "rgb"); i >= 0 {
		if j := strings.IndexByte(rest[i:], ')'); j >= 0 {
			if c, ok := parseColor(rest[i : i+j+1]); ok {
				op.color, hasColor = c, true
			}
			rest = rest[:i] + " " + rest[i+j+1:]
		}
	} else if i := strings.Index(strings.ToLower(rest), "hsl"); i >= 0 {
		if j := strings.IndexByte(rest[i:], ')'); j >= 0 {
			if c, ok := parseColor(rest[i : i+j+1]); ok {
				op.color, hasColor = c, true
			}
			rest = rest[:i] + " " + rest[i+j+1:]
		}
	}
	var lengths []float32
	for _, f := range strings.Fields(rest) {
		if l, ok := p.eng.RasterLength(n, f, false); ok {
			lengths = append(lengths, float32(l))
			continue
		}
		if f == "0" {
			lengths = append(lengths, 0)
			continue
		}
		if c, ok := parseColor(f); ok && !hasColor {
			op.color, hasColor = c, true
			continue
		}
		return op, false
	}
	if len(lengths) < 2 || len(lengths) > 3 {
		return op, false
	}
	if !hasColor {
		// The default is the color of the element.
		if c, ok := parseColor(p.eng.RasterLookup(n, "color")); ok {
			op.color = c
		} else {
			op.color = css.Color{A: 1}
		}
	}
	// BuildFilterOperations floors the offset.
	op.dx = floor32(lengths[0])
	op.dy = floor32(lengths[1])
	if len(lengths) == 3 {
		if lengths[2] < 0 {
			return op, false
		}
		op.sigma = lengths[2]
	}
	return op, true
}

// filterImage is an image during filter evaluation: premultiplied RGBA
// pixels for a rectangle of the device.
type filterImage struct {
	pix    []uint8
	bounds IRect
	stride int
}

func newFilterImage(bounds IRect) *filterImage {
	w, h := max(bounds.width(), 0), max(bounds.height(), 0)
	return &filterImage{pix: make([]uint8, w*h*4), bounds: bounds, stride: w * 4}
}

func (f *filterImage) row(y int) []uint8 {
	o := (y - f.bounds.T) * f.stride
	return f.pix[o : o+f.stride]
}

// imageFromLayer copies the drawn region of a layer.
func imageFromLayer(pm *pixmap, region IRect) *filterImage {
	if !region.intersect(pm.bounds()) {
		return newFilterImage(IRect{})
	}
	img := newFilterImage(region)
	for y := region.T; y < region.B; y++ {
		o := y*pm.stride + region.L*4
		copy(img.row(y), pm.pix[o:o+img.stride])
	}
	return img
}

// paintFiltered paints an element into a layer, applies the filter
// functions and draws the result.
func (p *painter) paintFiltered(n *dom.Node, st *svgStyle, ctm Affine, ops []filterOp, paint func(st *svgStyle, ctm Affine)) {
	parent := p.cv.dev()
	layer := p.cv.pushLayer()
	// The filter reads the source outside the clip: a shadow can come from
	// there.
	layer.clip, layer.aa = IRect{0, 0, p.cv.w, p.cv.h}, nil
	paint(st, ctm)
	p.cv.popLayerRaw()
	img := imageFromLayer(layer.pm, layer.dirty)
	p.cv.putPixmap(layer.pm, layer.dirty)

	m := deviceMatrix(ctm)
	// The scale of the filter space: lengths of the filter are multiplied
	// by it. A rotation keeps the lengths.
	sx := float32(math.Hypot(float64(m.sx), float64(m.ky)))
	sy := float32(math.Hypot(float64(m.kx), float64(m.sy)))
	// draw is the way the last image reaches the device.
	type drawMode uint8
	const (
		drawSprite drawMode = iota
		drawLinear
		drawBrightness
	)
	mode := drawSprite
	var amount float32
	toLinear, fromLinear := gammaMaps()
	for i := range ops {
		op := &ops[i]
		// A function that is not the first one reads an 8-bit image.
		switch mode {
		case drawLinear:
			img.mapColors(fromLinear)
		case drawBrightness:
			img.mapColors(brightnessMap(amount))
		}
		mode = drawSprite
		switch op.kind {
		case filterReference:
			var ok bool
			img, ok = p.applyReferenceFilter(n, op.node, img, ctm, sx, sy, toLinear)
			if !ok || img == nil {
				return
			}
			mode = drawLinear
		case filterDropShadow:
			c := op.color
			col := [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A)}
			img = dropShadow(img, float32(op.dx*sx), float32(op.dy*sy), float32(op.sigma*sx), float32(op.sigma*sy), col, false)
		case filterBrightness:
			mode, amount = drawBrightness, op.amount
		}
	}
	region := img.bounds
	if !region.intersect(parent.clip) {
		return
	}
	switch mode {
	case drawSprite:
		p.cv.compositeImage(parent, img, region, nil)
	case drawLinear:
		p.cv.compositeImage(parent, img, region, fromLinear)
	case drawBrightness:
		p.cv.compositeImage(parent, img, region, brightnessMap(amount))
	}
}

// colorFn maps an unpremultiplied color.
type colorFn func(r, g, b, a float32) (float32, float32, float32, float32)

// colorMap is a color filter that maps each color channel on its own and
// keeps alpha. lut is the map of an opaque pixel: the 8-bit result for each
// 8-bit value.
type colorMap struct {
	fn  colorFn
	lut [256]uint8
}

func newColorMap(channel func(float32) float32) *colorMap {
	m := &colorMap{fn: func(r, g, b, a float32) (float32, float32, float32, float32) {
		return channel(r), channel(g), channel(b), a
	}}
	const inv = float32(1.0 / 255)
	for i := range m.lut {
		m.lut[i] = toUnorm(channel(float32(float32(i) * inv)))
	}
	return m
}

var (
	gammaMapsOnce sync.Once
	toLinearMap   *colorMap
	fromLinearMap *colorMap
)

// gammaMaps returns the color filters between sRGB and linear RGB
// (SkColorFilters::SRGBToLinearGamma and LinearToSRGBGamma).
func gammaMaps() (toLinear, fromLinear *colorMap) {
	gammaMapsOnce.Do(func() {
		toLinearMap = newColorMap(srgbToLuma)
		fromLinearMap = newColorMap(srgbFromLuma)
	})
	return toLinearMap, fromLinearMap
}

func brightnessMap(amount float32) *colorMap {
	return newColorMap(func(v float32) float32 { return min(float32(v*amount), 1) })
}

// mapColors applies a color filter to an image, as a color filter stage of
// the raster pipeline does: it unpremultiplies, maps, premultiplies and
// rounds to 8 bits.
func (f *filterImage) mapColors(m *colorMap) {
	const inv = float32(1.0 / 255)
	for i := 0; i+3 < len(f.pix); i += 4 {
		a8 := f.pix[i+3]
		if a8 == 0 {
			continue
		}
		if a8 == 255 {
			f.pix[i], f.pix[i+1], f.pix[i+2] = m.lut[f.pix[i]], m.lut[f.pix[i+1]], m.lut[f.pix[i+2]]
			continue
		}
		a := float32(float32(a8) * inv)
		ia := 1 / a
		r, g, b, a2 := m.fn(float32(float32(float32(f.pix[i])*inv)*ia), float32(float32(float32(f.pix[i+1])*inv)*ia), float32(float32(float32(f.pix[i+2])*inv)*ia), a)
		f.pix[i] = toUnorm(float32(r * a2))
		f.pix[i+1] = toUnorm(float32(g * a2))
		f.pix[i+2] = toUnorm(float32(b * a2))
		f.pix[i+3] = toUnorm(a2)
	}
}

// compositeImage draws a filter result into a device with source-over.
// Without a color filter it is a sprite blit. With one, the blend is in
// the normalized arithmetic of the raster pipeline.
func (c *canvas) compositeImage(dst *device, img *filterImage, region IRect, cm *colorMap) {
	if dst.aa != nil && !region.intersect(dst.aa.bounds) {
		return
	}
	dst.extend(region)
	width := region.width()
	const inv = float32(1.0 / 255)
	for y := region.T; y < region.B; y++ {
		so := (y-img.bounds.T)*img.stride + (region.L-img.bounds.L)*4
		s := img.pix[so : so+width*4 : so+width*4]
		do := y*dst.pm.stride + region.L*4
		d := dst.pm.pix[do : do+width*4 : do+width*4]
		var clipRow []uint8
		if dst.aa != nil {
			clipRow = dst.aa.rowAt(y)[region.L-dst.aa.bounds.L : region.R-dst.aa.bounds.L]
		}
		if cm == nil {
			if clipRow == nil {
				srcOverRow(d, s)
			} else {
				blendRowGeneral(d, s, 255, blendSrcOver, clipRow)
			}
			continue
		}
		for i := 0; i+3 < len(s); i += 4 {
			a8 := s[i+3]
			if a8 == 0 {
				continue
			}
			if a8 == 255 && clipRow == nil {
				// An opaque pixel replaces the destination.
				d[i], d[i+1], d[i+2], d[i+3] = cm.lut[s[i]], cm.lut[s[i+1]], cm.lut[s[i+2]], 255
				continue
			}
			a := float32(float32(a8) * inv)
			ia := 1 / a
			r, g, b, a2 := cm.fn(float32(float32(float32(s[i])*inv)*ia), float32(float32(float32(s[i+1])*inv)*ia), float32(float32(float32(s[i+2])*inv)*ia), a)
			sr, sg, sb := float32(r*a2), float32(g*a2), float32(b*a2)
			dr, dg, db, da := float32(float32(d[i])*inv), float32(float32(d[i+1])*inv), float32(float32(d[i+2])*inv), float32(float32(d[i+3])*inv)
			k := 1 - a2
			or, og, ob, oa := sr+float32(dr*k), sg+float32(dg*k), sb+float32(db*k), a2+float32(da*k)
			if clipRow != nil {
				cv := float32(float32(clipRow[i/4]) * inv)
				or = dr + float32((or-dr)*cv)
				og = dg + float32((og-dg)*cv)
				ob = db + float32((ob-db)*cv)
				oa = da + float32((oa-da)*cv)
			}
			d[i], d[i+1], d[i+2], d[i+3] = toUnorm(or), toUnorm(og), toUnorm(ob), toUnorm(oa)
		}
	}
}

// applyReferenceFilter evaluates a filter element on the source image. It
// returns the result in linear RGB, or the source when the filter has
// nothing that this package implements. ok is false when the element must
// not be painted: a filter without primitives, or an empty filter region.
func (p *painter) applyReferenceFilter(n, filter *dom.Node, src *filterImage, ctm Affine, sx, sy float32, toLinear *colorMap) (*filterImage, bool) {
	var prims []*dom.Node
	for c := filter.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.ElementNode && c.NS == dom.NSSVG && strings.HasPrefix(strings.ToLower(c.Tag), "fe") {
			prims = append(prims, c)
		}
	}
	if len(prims) == 0 {
		return nil, false
	}
	// The filter region: x, y, width and height of the filter element, by
	// default -10%, -10%, 120% and 120% of the bounding box.
	bx, by, bw, bh, hasBox := p.eng.RasterBBox(n)
	units := strings.TrimSpace(filter.AttrOr("filterUnits", "objectBoundingBox"))
	var region Rect
	if units == "userSpaceOnUse" {
		x := p.filterLength(filter, "x", "-10%", lengthWidth)
		y := p.filterLength(filter, "y", "-10%", lengthHeight)
		w := p.filterLength(filter, "width", "120%", lengthWidth)
		h := p.filterLength(filter, "height", "120%", lengthHeight)
		region = Rect{x, y, x + w, y + h}
	} else {
		if !hasBox || !(bw > 0 && bh > 0) {
			return nil, false
		}
		fx := fractionAttr(filter, "x", -0.1)
		fy := fractionAttr(filter, "y", -0.1)
		fw := fractionAttr(filter, "width", 1.2)
		fh := fractionAttr(filter, "height", 1.2)
		x := bx + float32(fx*bw)
		y := by + float32(fy*bh)
		region = Rect{x, y, x + float32(fw*bw), y + float32(fh*bh)}
	}
	if region.isEmpty() {
		return nil, false
	}
	m := deviceMatrix(ctm)
	// SkCropImageFilter rounds the crop out to whole pixels.
	crop := m.mapRect(region).roundOut()

	// Only feDropShadow on the source is implemented. Another primitive
	// makes the filter a pass-through.
	for _, prim := range prims {
		if !strings.EqualFold(prim.Tag, "feDropShadow") {
			return src.clone(), true
		}
	}
	linear := true
	img := src.clone()
	for _, prim := range prims {
		if v := strings.TrimSpace(p.eng.RasterLookup(prim, "color-interpolation-filters")); strings.EqualFold(v, "sRGB") {
			linear = false
		}
	}
	if linear {
		img.mapColors(toLinear)
	}
	for _, prim := range prims {
		dx := attrNumber(prim, "dx", 2)
		dy := attrNumber(prim, "dy", 2)
		stdX, stdY := float32(2), float32(2)
		if nums := browser.RasterParseNumbers(prim.AttrOr("stdDeviation", "")); len(nums) >= 1 {
			stdX, stdY = nums[0], nums[0]
			if len(nums) >= 2 {
				stdY = nums[1]
			}
		}
		col := css.Color{A: 1}
		if v, ok := p.eng.RasterDeclared(prim, "flood-color"); ok {
			if c, ok := parseColor(v); ok {
				col = c
			}
		}
		opacity := float32(1)
		if v, ok := p.eng.RasterDeclared(prim, "flood-opacity"); ok {
			if f, ok := parseOpacity(v); ok {
				opacity = f
			}
		}
		c := [4]float32{float32(col.R) / 255, float32(col.G) / 255, float32(col.B) / 255, float32(float32(col.A) * opacity)}
		if linear {
			c[0], c[1], c[2] = srgbToLuma(c[0]), srgbToLuma(c[1]), srgbToLuma(c[2])
		}
		if stdX < 0 || stdY < 0 {
			stdX, stdY = 0, 0
		}
		img = dropShadow(img, float32(dx*sx), float32(dy*sy), float32(stdX*sx), float32(stdY*sy), c, true)
		img = img.cropped(crop)
	}
	if !linear {
		// The caller converts from linear RGB.
		img.mapColors(toLinear)
	}
	return img, true
}

func (p *painter) filterLength(filter *dom.Node, name, def string, mode lengthMode) float32 {
	v := strings.TrimSpace(filter.AttrOr(name, def))
	if l, ok := p.length(filter, v, mode); ok {
		return l
	}
	l, _ := p.length(filter, def, mode)
	return l
}

// fractionAttr reads a number or a percentage as a fraction of the
// bounding box.
func fractionAttr(n *dom.Node, name string, def float32) float32 {
	v, ok := n.GetAttr(name)
	if !ok {
		return def
	}
	if f, ok := parseNumberOrPercent(v); ok {
		return f
	}
	return def
}

func attrNumber(n *dom.Node, name string, def float32) float32 {
	v, ok := n.GetAttr(name)
	if !ok {
		return def
	}
	if nums := browser.RasterParseNumbers(v); len(nums) == 1 {
		return nums[0]
	}
	return def
}

func (f *filterImage) clone() *filterImage {
	return &filterImage{pix: append([]uint8(nil), f.pix...), bounds: f.bounds, stride: f.stride}
}

// cropped returns the part of an image inside a rectangle.
func (f *filterImage) cropped(crop IRect) *filterImage {
	b := f.bounds
	if !b.intersect(crop) {
		return newFilterImage(IRect{})
	}
	if b == f.bounds {
		return f
	}
	out := newFilterImage(b)
	for y := b.T; y < b.B; y++ {
		o := (b.L - f.bounds.L) * 4
		copy(out.row(y), f.row(y)[o:o+out.stride])
	}
	return out
}

// dropShadow is the filter graph of SkImageFilters::DropShadow: the blurred
// alpha of the input in a color, moved by an offset, below the input.
// color is not premultiplied. floatMerge tells that the input has a pending
// color filter, so that the raster pipeline blends it. Without one the
// input is a sprite.
func dropShadow(src *filterImage, dx, dy, sigmaX, sigmaY float32, color [4]float32, floatMerge bool) *filterImage {
	if src.bounds.isEmpty() {
		return src
	}
	blurred := blurAlpha(src, sigmaX, sigmaY)
	// The bounds of the moved shadow, with room for linear filtering.
	ix, iy := int(math.Floor(float64(dx))), int(math.Floor(float64(dy)))
	fx, fy := dx-float32(ix), dy-float32(iy)
	sb := IRect{blurred.bounds.L + ix, blurred.bounds.T + iy, blurred.bounds.R + ix, blurred.bounds.B + iy}
	if fx != 0 {
		sb.R++
	}
	if fy != 0 {
		sb.B++
	}
	out := newFilterImage(sb.join(src.bounds))
	pr, pg, pb, pa := float32(color[0]*color[3]), float32(color[1]*color[3]), float32(color[2]*color[3]), color[3]
	const inv = float32(1.0 / 255)
	// The shadow pixel for each alpha value.
	var lut [256][4]uint8
	for i := 1; i < 256; i++ {
		a := float32(float32(i) * inv)
		lut[i] = [4]uint8{toUnorm(float32(pr * a)), toUnorm(float32(pg * a)), toUnorm(float32(pb * a)), toUnorm(float32(pa * a))}
	}
	if fx == 0 && fy == 0 {
		// A whole offset copies the alpha values.
		bw := blurred.bounds.width()
		for y := sb.T; y < sb.B; y++ {
			arow := blurred.pix[(y-iy-blurred.bounds.T)*blurred.stride:]
			row := out.row(y)[(sb.L-out.bounds.L)*4:]
			for x := 0; x < bw; x++ {
				if a := arow[x]; a != 0 {
					copy(row[x*4:x*4+4], lut[a][:])
				}
			}
		}
	} else {
		alphaAt := func(x, y int) float32 { return float32(blurred.at(x, y)) }
		for y := sb.T; y < sb.B; y++ {
			row := out.row(y)
			for x := sb.L; x < sb.R; x++ {
				// The source pixels of the shadow, with linear filtering.
				a00 := alphaAt(x-ix-1, y-iy-1)
				a10 := alphaAt(x-ix, y-iy-1)
				a01 := alphaAt(x-ix-1, y-iy)
				a11 := alphaAt(x-ix, y-iy)
				top := float32(a00*fx) + float32(a10*(1-fx))
				bottom := float32(a01*fx) + float32(a11*(1-fx))
				a := float32(top*fy) + float32(bottom*(1-fy))
				if a == 0 {
					continue
				}
				a *= inv
				o := (x - out.bounds.L) * 4
				row[o] = toUnorm(float32(pr * a))
				row[o+1] = toUnorm(float32(pg * a))
				row[o+2] = toUnorm(float32(pb * a))
				row[o+3] = toUnorm(float32(pa * a))
			}
		}
	}
	// The input on top.
	for y := src.bounds.T; y < src.bounds.B; y++ {
		o := (src.bounds.L - out.bounds.L) * 4
		d := out.row(y)[o : o+src.stride]
		s := src.row(y)
		if floatMerge {
			blendRowGeneral(d, s, 255, blendSrcOver, nil)
		} else {
			srcOverRow(d, s)
		}
	}
	return out
}
