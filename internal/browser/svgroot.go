// Ported from Chromium 156.0.8078.4 (Blink), in part: the geometry between
// the user space of SVG content and the page. The functions of this file
// follow the functions of Blink that their comments name:
//
//   - LayoutSVGRoot::LocalToBorderBoxTransform:
//     core/layout/svg/layout_svg_root.cc and layout_svg_root.h
//     (LGPL-2.0-or-later).
//   - SVGPreserveAspectRatio::ComputeTransform:
//     core/svg/svg_preserve_aspect_ratio.cc (LGPL-2.0-or-later).
//   - SVGSVGElement::LocalCoordinateSpaceTransform:
//     core/svg/svg_svg_element.cc (LGPL-2.0-or-later).
//   - SVGGraphicsElement::getScreenCTM: core/svg/svg_graphics_element.cc
//     (LGPL-2.0-or-later).
//   - SVGLayoutSupport::CalculateScreenFontSizeScalingFactor:
//     core/layout/svg/svg_layout_support.cc (LGPL-2.0-or-later).
//   - LayoutObject::LocalToAbsoluteQuad: core/layout/layout_object.h
//     (LGPL-2.0-or-later).
//   - TransformState: core/layout/geometry/transform_state.cc (BSD-2-Clause,
//     with the copyright of Apple in the file).
//   - LayoutRatioFromSizeF: platform/geometry/physical_size.cc
//     (BSD-3-Clause).
//   - LayoutUnit: platform/geometry/layout_unit.h (BSD-3-Clause, with the
//     copyright of Google in the file).
//   - gfx::Transform, gfx::QuadF and gfx::SizeF: ui/gfx/geometry of
//     Chromium (BSD-3-Clause).
//   - base::ClampedNumeric: base/numerics of Chromium (BSD-3-Clause).
//
// The Blink paths are in third_party/blink/renderer. The license of each
// file is the license in its header at that version. See NOTICE in this
// directory.

package browser

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// This file has the geometry between the user space of SVG content and the
// page. It follows Blink:
//
//   - LayoutSVGRoot: the CSS box of the outermost svg element, which is a
//     replaced element, and LocalToBorderBoxTransform, which maps the
//     viewBox into that box (SVGPreserveAspectRatio::ComputeTransform).
//   - LayoutObject::LocalToAbsoluteQuad with TransformState: the mapping of
//     a box to the page for getBoundingClientRect.
//   - SVGGraphicsElement::getScreenCTM.
//   - SVGLayoutSupport::CalculateScreenFontSizeScalingFactor: the factor
//     between the font size of SVG text and the font size on the screen.
//     Blink lays out SVG text with the font of the screen size.

// outermostSVG returns the svg element that is the root of the SVG fragment
// of n: the last SVG ancestor, when it is an svg element.
func outermostSVG(n *dom.Node) *dom.Node {
	var root *dom.Node
	for p := n; p != nil && p.Type == dom.ElementNode && p.NS == dom.NSSVG; p = p.Parent {
		root = p
	}
	if root != nil && root.Tag == "svg" {
		return root
	}
	return nil
}

// svgRootBox is the layout of an outermost svg element.
type svgRootBox struct {
	// x, y, w and h are the content box in the page.
	x, y, w, h lu
	// toBorderBox is LayoutSVGRoot::LocalToBorderBoxTransform: it maps the
	// user space of the svg to its box. It is the identity without a
	// viewBox.
	toBorderBox affine
}

// luFromFloat is the LayoutUnit(float) constructor: it truncates.
func luFromFloat(v float32) lu {
	return lu(math.Trunc(float64(v * luPerPx)))
}

// mulDiv is LayoutUnit::MulDiv: v * m / d, truncated.
func (v lu) mulDiv(m, d lu) lu {
	if d == 0 {
		return 0
	}
	return lu(int64(v) * int64(m) / int64(d))
}

// layoutRatioFromSizeF is LayoutRatioFromSizeF of Blink
// (platform/geometry/physical_size.cc): an aspect ratio of two float sizes
// as two LayoutUnits. Sizes that are multiples of 1/64 are the ratio. For
// other sizes the ratio is a continued fraction of width / height, in
// float32, with at most 16 steps and an error below 0.000001.
func layoutRatioFromSizeF(ratioWidth, ratioHeight float32) (lu, lu) {
	// Check if we can convert without any error.
	width, height := luFromFloat(ratioWidth), luFromFloat(ratioHeight)
	// gfx::SizeF holds zero for a size that is not above 8 * FLT_EPSILON.
	const trivial = 8 * 1.1920929e-07
	isEmpty := !(ratioWidth > trivial) || !(ratioHeight > trivial)
	if (width.f32() == ratioWidth && height.f32() == ratioHeight) || isEmpty {
		return width, height
	}
	if ratioWidth == ratioHeight {
		return luPerPx, luPerPx
	}

	// If we can't get a precise ratio we use the continued fraction
	// algorithm to get an approximation.
	initial := ratioWidth / ratioHeight
	x := initial

	// clamp is the saturation of base::ClampedNumeric<int>.
	clamp := func(v int64) int64 {
		return min(max(v, math.MinInt32), math.MaxInt32)
	}
	var h0, h1, k0, k1 int64 = 0, 1, 1, 0

	// The worst case for this algorithm is the golden ratio, which requires
	// 16 iterations to reach our desired error.
	for i := 0; i < 16; i++ {
		// Break if we've gone Inf, or NaN.
		if math.IsInf(float64(x), 0) || math.IsNaN(float64(x)) {
			break
		}
		// Break if we've hit a good approximation.
		estimate := float32(h1) / float32(k1)
		if math.Abs(float64(initial-estimate)) < float64(float32(0.000001)) {
			break
		}

		// base::ClampFloor<int>(x)
		a := int64(math.MaxInt32)
		if floor := math.Floor(float64(x)); floor < math.MaxInt32 {
			a = clamp(int64(floor))
		}
		h2 := clamp(clamp(h1*a) + h0)
		k2 := clamp(clamp(k1*a) + k0)

		// Break if we've saturated (the ratio becomes meaningless).
		if h2 == math.MaxInt32 || k2 == math.MaxInt32 {
			break
		}

		// Update our convergents.
		h0, k0, h1, k1 = h1, k1, h2, k2
		x = 1 / (x - float32(a))
	}

	// Don't return an invalid ratio - instead just return the truncated ratio.
	if h1 == 0 || k1 == 0 {
		return width, height
	}

	// LayoutUnit::FromRawValue: the convergents are raw values.
	return lu(h1), lu(k1)
}

// rootLength is the computed value of the width, the height or the max-width
// of an outermost svg element.
type rootLength struct {
	auto    bool
	percent bool
	value   float64 // px, or the percentage
}

// rootLengthOf reads a size property of an outermost svg element: the CSS
// declaration, or the attribute with the same name. The width and the
// height attributes of an outermost svg are presentation attributes.
func (e *Engine) rootLengthOf(svg *dom.Node, prop string, emSize float64) rootLength {
	v, ok := e.declared(svg, prop)
	fromAttr := false
	if !ok && (prop == "width" || prop == "height") {
		v, ok = svg.GetAttr(prop)
		fromAttr = true
	}
	v = strings.ToLower(strings.TrimSpace(v))
	if !ok || v == "" || v == "auto" || v == "none" {
		return rootLength{auto: true}
	}
	if strings.HasSuffix(v, "%") {
		f, ok := parseCSSNumber(v[:len(v)-1])
		if !ok {
			return rootLength{auto: true}
		}
		return rootLength{percent: true, value: f}
	}
	px, ok := resolveLength(v, emSize, fromAttr)
	if !ok || px < 0 {
		return rootLength{auto: true}
	}
	return rootLength{value: px}
}

// svgRootBoxOf lays out an outermost svg element as Blink lays out a
// replaced element with the natural sizes of the svg: the width and the
// height attributes when they are not percentages, and the ratio of the
// viewBox. The containing block is the block that holds the diagram.
func (e *Engine) svgRootBoxOf(svg *dom.Node) svgRootBox {
	ts := e.textStyleOf(svg)
	available := luFromFloat(float32(e.ContainerWidth()))

	vb := parseNumberList(svg.AttrOr("viewBox", ""))
	hasViewBox := len(vb) == 4 && vb[2] > 0 && vb[3] > 0

	// The natural sizes.
	var naturalW, naturalH lu
	hasNaturalW, hasNaturalH := false, false
	if l := e.rootAttrLength(svg, "width", ts.size); !l.auto && !l.percent {
		naturalW, hasNaturalW = luFromFloat(float32(l.value)), true
	}
	if l := e.rootAttrLength(svg, "height", ts.size); !l.auto && !l.percent {
		naturalH, hasNaturalH = luFromFloat(float32(l.value)), true
	}
	var ratioW, ratioH lu
	switch {
	case hasViewBox:
		ratioW, ratioH = layoutRatioFromSizeF(vb[2], vb[3])
	case hasNaturalW && hasNaturalH:
		ratioW, ratioH = naturalW, naturalH
	}
	hasRatio := ratioW > 0 && ratioH > 0

	width := e.rootLengthOf(svg, "width", ts.size)
	height := e.rootLengthOf(svg, "height", ts.size)
	// A percentage of the height of a block with an automatic height
	// behaves as auto.
	heightAuto := height.auto || height.percent
	var h lu
	if !heightAuto {
		h = luFromFloat(float32(height.value))
	}

	var w lu
	switch {
	case width.percent:
		w = luFromFloat(float32(available.px()) * float32(width.value) / 100)
	case !width.auto:
		w = luFromFloat(float32(width.value))
	case hasNaturalW:
		w = naturalW
	case !heightAuto && hasRatio:
		w = h.mulDiv(ratioW, ratioH)
	case hasRatio:
		w = available
	default:
		w = 300 * luPerPx
	}
	if maxW := e.rootLengthOf(svg, "max-width", ts.size); !maxW.auto {
		limit := luFromFloat(float32(maxW.value))
		if maxW.percent {
			limit = luFromFloat(float32(available.px()) * float32(maxW.value) / 100)
		}
		w = min(w, limit)
	}
	if heightAuto {
		switch {
		case hasRatio:
			h = w.mulDiv(ratioH, ratioW)
		case hasNaturalH:
			h = naturalH
		default:
			h = 150 * luPerPx
		}
	}

	box := svgRootBox{w: w, h: h, toBorderBox: identity}
	// The svg is an inline box on the first line of its block. Its bottom
	// is on the baseline, so that a box that is lower than the ascent of
	// the line starts below the top of the line.
	margin := luFromFloat(float32(e.BodyMargin))
	box.x, box.y = margin, margin
	if parent := svg.Parent; parent != nil && parent.Type == dom.ElementNode {
		pts := e.textStyleOf(parent)
		if primary := e.chainOf(pts).Primary(); primary != nil {
			ascent, _ := lineExtent(pts, primary.Metrics(pts.fontSize()))
			if ascent > h {
				box.y += ascent - h
			}
		}
	}
	if hasViewBox && w > 0 && h > 0 {
		box.toBorderBox = viewBoxToViewTransform(vb, svg.AttrOr("preserveAspectRatio", ""), w.f32(), h.f32())
	}
	return box
}

// rootAttrLength reads the width or the height attribute of an svg element.
func (e *Engine) rootAttrLength(svg *dom.Node, name string, emSize float64) rootLength {
	v, ok := svg.GetAttr(name)
	v = strings.ToLower(strings.TrimSpace(v))
	if !ok || v == "" {
		return rootLength{auto: true}
	}
	if strings.HasSuffix(v, "%") {
		return rootLength{percent: true}
	}
	px, ok := resolveLength(v, emSize, true)
	if !ok || px < 0 {
		return rootLength{auto: true}
	}
	return rootLength{value: px}
}

// viewBoxToViewTransform is SVGPreserveAspectRatio::ComputeTransform: the
// transform that maps the viewBox into a viewport of the given size. par is
// the preserveAspectRatio attribute.
func viewBoxToViewTransform(vb []float32, par string, viewW, viewH float32) affine {
	logicalX, logicalY := float64(vb[0]), float64(vb[1])
	logicalW, logicalH := float64(vb[2]), float64(vb[3])
	physicalW, physicalH := float64(viewW), float64(viewH)

	align, slice := "xMidYMid", false
	if fields := strings.Fields(par); len(fields) > 0 {
		align = fields[0]
		slice = len(fields) > 1 && fields[1] == "slice"
	}
	transform := identity
	if align == "none" {
		transform = transform.scale(physicalW/logicalW, physicalH/logicalH)
		return transform.translate(-logicalX, -logicalY)
	}
	xMin, xMid := strings.HasPrefix(align, "xMin"), strings.HasPrefix(align, "xMid")
	yMin, yMid := strings.HasSuffix(align, "YMin"), strings.HasSuffix(align, "YMid")

	logicalRatio := logicalW / logicalH
	physicalRatio := physicalW / physicalH
	if (logicalRatio < physicalRatio && !slice) || (logicalRatio >= physicalRatio && slice) {
		transform = transform.scale(physicalH/logicalH, physicalH/logicalH)
		switch {
		case xMin:
			return transform.translate(-logicalX, -logicalY)
		case xMid:
			return transform.translate(-logicalX-float64((logicalW-float64(physicalW*logicalH)/physicalH)/2), -logicalY)
		}
		return transform.translate(-logicalX-(logicalW-float64(physicalW*logicalH)/physicalH), -logicalY)
	}
	transform = transform.scale(physicalW/logicalW, physicalW/logicalW)
	switch {
	case yMin:
		return transform.translate(-logicalX, -logicalY)
	case yMid:
		return transform.translate(-logicalX, -logicalY-float64((logicalH-float64(physicalH*logicalW)/physicalW)/2))
	}
	return transform.translate(-logicalX, -logicalY-(logicalH-float64(physicalH*logicalW)/physicalW))
}

// isIntegerTranslation is Transform::IsIdentityOrInteger2dTranslation.
func (m affine) isIntegerTranslation() bool {
	return m.isIdentityOrTranslation() && m.e == math.Trunc(m.e) && m.f == math.Trunc(m.f) &&
		math.Abs(m.e) < 1<<24 && math.Abs(m.f) < 1<<24
}

// localToAbsoluteRect maps a rectangle in the user space of an SVG element
// to the page, as LayoutObject::LocalToAbsoluteQuad and QuadF::BoundingBox
// do for getBoundingClientRect. The four corners go through the transform of
// each ancestor in float32. A translation by integers is not applied at
// once: TransformState adds it to an offset, and applies the offset before
// the next transform that is not such a translation, and at the end.
func (e *Engine) localToAbsoluteRect(n *dom.Node, r rectF) rectF {
	xs := [4]float32{r.x, r.right(), r.right(), r.x}
	ys := [4]float32{r.y, r.y, r.bottom(), r.bottom()}
	var offsetX, offsetY lu
	flush := func() {
		if offsetX == 0 && offsetY == 0 {
			return
		}
		dx, dy := offsetX.f32(), offsetY.f32()
		for i := range xs {
			xs[i] += dx
			ys[i] += dy
		}
		offsetX, offsetY = 0, 0
	}
	apply := func(m affine) {
		if m.isIntegerTranslation() {
			offsetX += lu(m.e) * luPerPx
			offsetY += lu(m.f) * luPerPx
			return
		}
		flush()
		for i := range xs {
			xs[i], ys[i] = m.mapPoint(xs[i], ys[i])
		}
	}
	root := outermostSVG(n)
	for cur := n; cur != nil && cur != root && cur.Type == dom.ElementNode && cur.NS == dom.NSSVG; cur = cur.Parent {
		apply(e.localTransform(cur))
	}
	if root != nil {
		box := e.svgRootBoxOf(root)
		if n != root {
			apply(box.toBorderBox)
		}
		offsetX += box.x
		offsetY += box.y
	} else {
		margin := luFromFloat(float32(e.BodyMargin))
		offsetX += margin
		offsetY += margin
	}
	flush()
	rl, rr := min(xs[0], xs[1], xs[2], xs[3]), max(xs[0], xs[1], xs[2], xs[3])
	rt, rb := min(ys[0], ys[1], ys[2], ys[3]), max(ys[0], ys[1], ys[2], ys[3])
	return rectF{rl, rt, rr - rl, rb - rt}
}

// ScreenCTM is SVGGraphicsElement.getScreenCTM(): the matrix a, b, c, d, e,
// f that maps the user space of an SVG element to the page. The second
// result is false for a node that is not an SVG element.
func (e *Engine) ScreenCTM(n *dom.Node) ([6]float64, bool) {
	if n == nil || n.Type != dom.ElementNode || n.NS != dom.NSSVG {
		return [6]float64{}, false
	}
	root := outermostSVG(n)
	ctm := identity
	for cur := n; cur != nil && cur.Type == dom.ElementNode && cur.NS == dom.NSSVG; cur = cur.Parent {
		local := e.localTransform(cur)
		if cur == root {
			// SVGSVGElement::LocalCoordinateSpaceTransform for the screen:
			// the position of the box, then LocalToBorderBoxTransform.
			box := e.svgRootBoxOf(root)
			local = identity.translate(float64(box.x.px()), float64(box.y.px())).multiply(box.toBorderBox)
		}
		ctm = local.multiply(ctm)
	}
	return [6]float64{ctm.a, ctm.b, ctm.c, ctm.d, ctm.e, ctm.f}, true
}

// screenFontScale is CalculateScreenFontSizeScalingFactor for the text of an
// SVG text element: the mean scale of the transform from the text to the
// page. It is 1 when no ancestor scales.
func (e *Engine) screenFontScale(text *dom.Node) float32 {
	root := outermostSVG(text)
	transform := identity
	scaled := false
	for cur := text; cur != nil && cur.Type == dom.ElementNode && cur.NS == dom.NSSVG; cur = cur.Parent {
		local := identity
		if cur == root {
			if !cur.HasAttr("viewBox") {
				break
			}
			local = e.svgRootBoxOf(root).toBorderBox
		} else {
			local = e.localTransform(cur)
		}
		if local.isIdentityOrTranslation() {
			continue
		}
		scaled = true
		transform = local.multiply(transform)
	}
	if !scaled {
		return 1
	}
	xScaleSquared := float64(transform.a*transform.a) + float64(transform.b*transform.b)
	yScaleSquared := float64(transform.c*transform.c) + float64(transform.d*transform.d)
	factor := float32(math.Sqrt((xScaleSquared + yScaleSquared) / 2))
	if factor == 0 || factor != factor {
		return 1
	}
	return factor
}
