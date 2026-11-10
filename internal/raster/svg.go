// The SVG painter: it walks the document as the SVG painters of Blink do
// (SVGRootPainter, SVGContainerPainter, SVGShapePainter) and issues the
// draw calls that they issue.
//
// The license of each file of Chromium that this file names is the license
// in its header at Chromium 156.0.8078.4. The Blink paths are in
// third_party/blink/renderer:
//
//   - SVGRootPainter, SVGContainerPainter, SVGShapePainter and
//     SVGObjectPainter: core/paint/svg_root_painter.cc,
//     svg_container_painter.cc, svg_shape_painter.cc and
//     svg_object_painter.cc (BSD-3-Clause).
//   - LayoutSVGRoot::BuildLocalToBorderBoxTransform:
//     core/layout/svg/layout_svg_root.cc (LGPL-2.0-or-later).
//   - LayoutSVGShape: core/layout/svg/layout_svg_shape.cc
//     (LGPL-2.0-or-later).
//   - SVGTests::IsValid: core/svg/svg_tests.cc (LGPL-2.0-or-later).
//   - StrokeData::SetupPaint: platform/geometry/stroke_data.cc
//     (BSD-3-Clause, with the copyright of Google in the file).
//   - PictureLayerImpl and SaveLayerAlphaOp: cc/layers and cc/paint
//     (BSD-3-Clause).
//
// See NOTICE in this directory and finding F1 of docs/LICENSE-AUDIT.md.

package raster

import (
	"image/color"
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

type paintKind uint8

const (
	paintNone paintKind = iota
	paintColor
	paintURL
	paintCurrentColor
	paintContextFill
	paintContextStroke
)

// svgPaint is the value of fill or stroke.
type svgPaint struct {
	kind  paintKind
	color css.Color
	url   string // the fragment id of a paint server
	// fallback is the paint to use when the server does not resolve.
	fallbackKind  paintKind
	fallbackColor css.Color
}

// svgStyle holds the inherited properties that painting reads.
type svgStyle struct {
	fill, stroke               svgPaint
	fillOpacity, strokeOpacity float32
	fillRule, clipRule         fillType
	strokeWidth                float32
	cap                        strokeCap
	join                       strokeJoin
	miter                      float32
	dashArray                  []float32
	dashOffset                 float32
	color                      css.Color
	visible                    bool
	markerStart                string
	markerMid                  string
	markerEnd                  string
	paintOrder                 [3]uint8
	crisp                      bool
}

const (
	orderFill = iota
	orderStroke
	orderMarkers
)

func initialStyle() svgStyle {
	return svgStyle{
		fill:        svgPaint{kind: paintColor, color: css.Color{A: 1}},
		fillOpacity: 1, strokeOpacity: 1,
		strokeWidth: 1, miter: 4,
		color:      css.Color{A: 1},
		visible:    true,
		paintOrder: [3]uint8{orderFill, orderStroke, orderMarkers},
	}
}

// contextPaints are the fill and stroke of the element that references a
// marker: the values of context-fill and context-stroke.
type contextPaints struct {
	fill, stroke svgPaint
	fillOpacity  float32
	strokeOp     float32
}

type painter struct {
	vector *vectorPainter
	eng    *browser.Engine
	doc    *dom.Document
	cv     *canvas
	scale  float64
	ids    map[string]*dom.Node

	// viewport is the size of the nearest svg viewport in user units, for
	// percentages.
	viewportW, viewportH float32

	path    Path
	glyphs  *glyphCache
	context *contextPaints
	depth   int

	// layerEffects reports that an element of the document is painted
	// through a layer that is not a plain opacity layer: a filter or a
	// blend mode. See opacityLayer.
	layerEffects bool
	// isolated holds the containers that get a layer of their own because
	// a descendant blends with a blend mode, and pageIsolated reports that
	// the whole page is such a group. See scanEffects.
	isolated     map[*dom.Node]bool
	pageIsolated bool

	// foY is the y of the foreignObject whose content is painted. Blink
	// rounds the top of a block of text in the space of the SVG.
	foY float64
}

func newPainter(eng *browser.Engine, doc *dom.Document, w, h int, scale float64) *painter {
	return &painter{eng: eng, doc: doc, cv: newCanvas(w, h), scale: scale, glyphs: sharedGlyphCache()}
}

func (p *painter) fillBackground(bg color.Color) {
	r, g, b, a := uint8(255), uint8(255), uint8(255), uint8(255)
	if bg != nil {
		r, g, b, a = colorToNRGBA(bg)
	}
	if a == 0 {
		return
	}
	dev := p.cv.dev()
	pm := premultiply(r, g, b, a)
	fill32(dev.pm.pix, dev.pm.w*dev.pm.h, pm)
	// Chromium on Linux draws LCD text on a layer whose content is opaque
	// (PictureLayerImpl::ComputeLCDTextDisallowedReason). On macOS it
	// never does: the pixel geometry of its surfaces is unknown, so an
	// opacity layer starts transparent there (opacityLayer).
	dev.lcd = a == 255 && p.eng.Fonts != nil && p.eng.Fonts.Platform() != font.PlatformDarwin
}

func (p *painter) indexIDs(n *dom.Node) {
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type != dom.ElementNode {
			continue
		}
		if id, ok := c.GetAttr("id"); ok && id != "" {
			if _, dup := p.ids[id]; !dup {
				p.ids[id] = c
			}
		}
		p.indexIDs(c)
	}
}

// paintRoot paints the root svg element (SVGRootPainter).
func (p *painter) paintRoot(root *dom.Node, sz svgRootSize) {
	if p.vector != nil {
		p.vector.doc.Save()
		defer p.vector.doc.Restore()
	}
	p.ids = map[string]*dom.Node{}
	p.indexIDs(p.doc.Body)
	if p.eng.RasterDisplay(root) == "none" {
		return
	}
	// The border box, snapped to whole CSS pixels, clips the content.
	snappedW := int(math.Round(float64((sz.boxW+layoutUnit/2)/layoutUnit) * p.scale))
	snappedH := int(math.Round(float64((sz.boxH+layoutUnit/2)/layoutUnit) * p.scale))
	dev := p.cv.dev()
	clip := IRect{0, 0, snappedW, snappedH}
	if !clip.intersect(dev.clip) {
		return
	}
	saved := dev.clip
	dev.clip = clip
	defer func() { dev.clip = saved }()
	if p.vector != nil {
		p.vector.doc.Clip(pdfRect(clip.toRect()), false)
	}

	// LayoutSVGRoot::BuildLocalToBorderBoxTransform: the content size is
	// the box in CSS pixels, and the device scale comes on top.
	contentW := float32(sz.boxW) / layoutUnit
	contentH := float32(sz.boxH) / layoutUnit
	ctm := Affine{A: p.scale, D: p.scale}
	p.viewportW, p.viewportH = contentW, contentH
	if sz.hasViewBox {
		if contentW <= 0 || contentH <= 0 {
			return
		}
		ctm = ctm.Mul(viewBoxTransform(sz.viewBox, sz.par, contentW, contentH))
		p.viewportW, p.viewportH = sz.viewBox[2], sz.viewBox[3]
	} else if _, has := root.GetAttr("viewBox"); has {
		if vb := browser.RasterParseNumbers(root.AttrOr("viewBox", "")); len(vb) == 4 {
			// An empty viewBox disables rendering.
			return
		}
	}
	st := initialStyle()
	p.resolveStyle(root, &st)
	p.scanEffects(root, nil, 0)
	if p.pageIsolated {
		// The isolation group of the page: a layer without an opacity
		// (SaveLayerAlphaOp with alpha 1).
		p.cv.pushLayer()
		layer := p.cv.dev()
		savedLayerClip := layer.clip
		layer.clip = clip
		defer func() {
			layer.clip = savedLayerClip
			p.cv.popLayer(1, blendSrcOver)
		}()
	}
	p.paintWithEffects(root, &st, ctm, func(st *svgStyle, ctm Affine) {
		p.paintChildren(root, st, ctm)
	})
}

// notRendered lists the elements that are never painted directly.
var notRendered = map[string]bool{
	"defs": true, "symbol": true, "marker": true, "clipPath": true, "clippath": true, "mask": true, "pattern": true,
	"linearGradient": true, "lineargradient": true, "radialGradient": true, "radialgradient": true,
	"filter": true, "style": true, "title": true, "desc": true, "metadata": true, "script": true, "stop": true,
}

func (p *painter) paintChildren(n *dom.Node, st *svgStyle, ctm Affine) {
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.ElementNode {
			p.paintElement(c, st, ctm)
		}
	}
}

const maxDepth = 256

// paintElement paints one SVG element with the style of its parent.
func (p *painter) paintElement(n *dom.Node, parent *svgStyle, ctm Affine) {
	if p.vector != nil {
		p.vector.doc.Save()
		defer p.vector.doc.Restore()
	}
	if n.NS != dom.NSSVG || notRendered[n.Tag] {
		return
	}
	if p.depth > maxDepth {
		return
	}
	if p.eng.RasterDisplay(n) == "none" {
		return
	}
	p.depth++
	defer func() { p.depth-- }()

	st := *parent
	p.resolveStyle(n, &st)
	ctm = ctm.Mul(p.localTransform(n))

	switch n.Tag {
	case "g", "a":
		p.paintWithEffects(n, &st, ctm, func(st *svgStyle, ctm Affine) {
			p.paintChildren(n, st, ctm)
		})
	case "svg":
		p.paintNestedSVG(n, &st, ctm)
	case "switch":
		p.paintWithEffects(n, &st, ctm, func(st *svgStyle, ctm Affine) {
			for c := n.FirstChild; c != nil; c = c.Next {
				if c.Type == dom.ElementNode && c.NS == dom.NSSVG && switchChildValid(c) {
					p.paintElement(c, st, ctm)
					break
				}
			}
		})
	case "use":
		p.paintUse(n, &st, ctm)
	case "rect", "circle", "ellipse", "line", "polyline", "polygon", "path":
		p.paintWithEffects(n, &st, ctm, func(st *svgStyle, ctm Affine) {
			p.paintShape(n, st, ctm)
		})
	case "text":
		p.paintWithEffects(n, &st, ctm, func(st *svgStyle, ctm Affine) {
			p.paintText(n, st, ctm)
		})
	case "foreignObject", "foreignobject":
		p.paintWithEffects(n, &st, ctm, func(st *svgStyle, ctm Affine) {
			p.paintForeignObject(n, st, ctm)
		})
	case "image":
		p.paintWithEffects(n, &st, ctm, func(st *svgStyle, ctm Affine) {
			p.paintImage(n, st, ctm)
		})
	}
}

// switchChildValid is SVGTests::IsValid for a child of a switch element.
// Blink ignores requiredFeatures. requiredExtensions must name supported
// namespaces, and systemLanguage must match the language of the browser.
func switchChildValid(n *dom.Node) bool {
	if v, ok := n.GetAttr("systemLanguage"); ok {
		match := false
		for _, lang := range strings.Split(v, ",") {
			lang = strings.TrimSpace(lang)
			if lang == "en" || strings.HasPrefix(lang, "en-") {
				match = true
			}
		}
		if !match {
			return false
		}
	}
	if v, ok := n.GetAttr("requiredExtensions"); ok {
		fields := strings.Fields(v)
		if len(fields) == 0 {
			return false
		}
		for _, ext := range fields {
			if ext != "http://www.w3.org/1999/xhtml" && ext != "http://www.w3.org/1998/Math/MathML" {
				return false
			}
		}
	}
	return true
}

// localTransform is the transform of an element to its parent, with
// the scale property and transform-origin applied.
func (p *painter) localTransform(n *dom.Node) Affine {
	v := p.eng.RasterLocalTransform(n)
	// The transform of the engine has the properties translate, rotate
	// and scale too (Engine.individualTransforms): mermaid uses scale for
	// the highlighted slice of a pie.
	m := Affine{A: v[0], B: v[1], C: v[2], D: v[3], E: v[4], F: v[5]}
	if m.IsIdentity() {
		return m
	}
	origin, ok := p.eng.RasterDeclared(n, "transform-origin")
	if !ok {
		origin, ok = n.GetAttr("transform-origin")
	}
	if !ok || n.Tag == "svg" {
		return m
	}
	ox, oy, ok := p.parseOrigin(n, origin)
	if !ok {
		return m
	}
	// The engine applies the part of the origin that it resolves: lengths
	// in px and numbers. A keyword or a percentage counts as 0 there. The
	// rest of the origin applies here.
	ex, ey := engineOrigin(origin)
	ox, oy = ox-ex, oy-ey
	if ox == 0 && oy == 0 {
		return m
	}
	return IdentityAffine.Translate(ox, oy).Mul(m).Translate(-ox, -oy)
}

// engineOrigin returns the transform-origin that Engine.localTransform
// applies: the px lengths and the numbers of the first two fields.
func engineOrigin(v string) (x, y float64) {
	fields := strings.Fields(strings.ReplaceAll(v, ",", " "))
	parse := func(i int) float64 {
		if i >= len(fields) {
			return 0
		}
		f := strings.TrimSuffix(strings.ToLower(fields[i]), "px")
		nums := browser.RasterParseNumbers(f)
		if len(nums) != 1 || strings.ContainsAny(f, "%abcdefghijklmnopqrstuvwxyz") {
			return 0
		}
		return float64(nums[0])
	}
	return parse(0), parse(1)
}

// parseOrigin parses a transform-origin value with lengths. Percentages
// and keywords refer to the view box (transform-box: view-box).
func (p *painter) parseOrigin(n *dom.Node, v string) (x, y float64, ok bool) {
	fields := strings.Fields(strings.ToLower(v))
	if len(fields) == 0 {
		return 0, 0, false
	}
	one := func(s string, size float32, horizontal bool) (float64, bool) {
		switch s {
		case "left", "top":
			return 0, true
		case "center":
			return float64(size) / 2, true
		case "right", "bottom":
			return float64(size), true
		}
		if strings.HasSuffix(s, "%") {
			f, ok := parseFloat(s[:len(s)-1])
			return float64(f / 100 * float64(size)), ok
		}
		return p.eng.RasterLength(n, s, true)
	}
	x, ok = one(fields[0], p.viewportW, true)
	if !ok {
		return 0, 0, false
	}
	y = float64(p.viewportH) / 2
	if len(fields) > 1 {
		if y, ok = one(fields[1], p.viewportH, false); !ok {
			return 0, 0, false
		}
	}
	return x, y, true
}

// ---- style ----

func (p *painter) parsePaint(v string, cur css.Color) (svgPaint, bool) {
	v = strings.TrimSpace(v)
	lower := strings.ToLower(v)
	switch lower {
	case "none":
		return svgPaint{kind: paintNone}, true
	case "currentcolor":
		return svgPaint{kind: paintCurrentColor, color: cur}, true
	case "context-fill":
		return svgPaint{kind: paintContextFill}, true
	case "context-stroke":
		return svgPaint{kind: paintContextStroke}, true
	}
	if strings.HasPrefix(lower, "url(") {
		end := strings.IndexByte(v, ')')
		if end < 0 {
			return svgPaint{}, false
		}
		ref := strings.Trim(strings.TrimSpace(v[4:end]), `"'`)
		ref = strings.TrimSpace(ref)
		if i := strings.IndexByte(ref, '#'); i >= 0 {
			ref = ref[i+1:]
		}
		out := svgPaint{kind: paintURL, url: ref}
		rest := strings.TrimSpace(v[end+1:])
		if rest != "" {
			if fb, ok := p.parsePaint(rest, cur); ok && fb.kind != paintURL {
				out.fallbackKind, out.fallbackColor = fb.kind, fb.color
			}
		}
		return out, true
	}
	if c, ok := parseColor(v); ok {
		return svgPaint{kind: paintColor, color: c}, true
	}
	return svgPaint{}, false
}

// parseColor parses a CSS color. A color function with the number NaN or
// infinity outside of calc() is not a color: css.ParseColor reads such a
// number, and Blink drops the declaration. mermaid writes
// fill="hsl(240, 100%, NaN%)" for the points of a quadrant chart, which
// then inherit the fill.
func parseColor(v string) (css.Color, bool) {
	if strings.IndexByte(v, '(') >= 0 {
		lower := strings.ToLower(v)
		if !strings.Contains(lower, "calc(") && (strings.Contains(lower, "nan") || strings.Contains(lower, "inf")) {
			return css.Color{}, false
		}
	}
	return css.ParseColor(v)
}

func parseOpacity(v string) (float32, bool) {
	v = strings.TrimSpace(v)
	pct := strings.HasSuffix(v, "%")
	if pct {
		v = v[:len(v)-1]
	}
	f, ok := parseFloat(v)
	if !ok {
		return 0, false
	}
	if pct {
		f /= 100
	}
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return float32(f), true
}

func isInherit(v string) bool { return strings.EqualFold(strings.TrimSpace(v), "inherit") }

// resolveStyle updates the inherited properties with the declarations of
// an element. st holds the values of the parent on entry.
func (p *painter) resolveStyle(n *dom.Node, st *svgStyle) {
	get := func(prop string) (string, bool) {
		v, ok := p.eng.RasterDeclared(n, prop)
		if !ok || isInherit(v) {
			return "", false
		}
		return v, true
	}
	if v, ok := get("color"); ok {
		if c, ok := parseColor(v); ok {
			st.color = c
		}
	}
	if v, ok := get("fill"); ok {
		if paint, ok := p.parsePaint(v, st.color); ok {
			st.fill = paint
		}
	} else if st.fill.kind == paintCurrentColor {
		st.fill.color = st.color
	}
	if v, ok := get("stroke"); ok {
		if paint, ok := p.parsePaint(v, st.color); ok {
			st.stroke = paint
		}
	} else if st.stroke.kind == paintCurrentColor {
		st.stroke.color = st.color
	}
	if v, ok := get("fill-opacity"); ok {
		if f, ok := parseOpacity(v); ok {
			st.fillOpacity = f
		}
	}
	if v, ok := get("stroke-opacity"); ok {
		if f, ok := parseOpacity(v); ok {
			st.strokeOpacity = f
		}
	}
	if v, ok := get("fill-rule"); ok {
		st.fillRule = parseFillRule(v, st.fillRule)
	}
	if v, ok := get("clip-rule"); ok {
		st.clipRule = parseFillRule(v, st.clipRule)
	}
	if v, ok := get("stroke-width"); ok {
		if f, ok := p.length(n, v, lengthOther); ok && f >= 0 {
			st.strokeWidth = f
		}
	}
	if v, ok := get("stroke-linecap"); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "butt":
			st.cap = capButt
		case "round":
			st.cap = capRound
		case "square":
			st.cap = capSquare
		}
	}
	if v, ok := get("stroke-linejoin"); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "miter", "miter-clip", "arcs":
			st.join = joinMiter
		case "round":
			st.join = joinRound
		case "bevel":
			st.join = joinBevel
		}
	}
	if v, ok := get("stroke-miterlimit"); ok {
		if f, ok := parseFloat(v); ok && f >= 1 {
			st.miter = float32(f)
		}
	}
	if v, ok := get("stroke-dasharray"); ok {
		st.dashArray = p.parseDashArray(n, v)
	}
	if v, ok := get("stroke-dashoffset"); ok {
		if f, ok := p.length(n, v, lengthOther); ok {
			st.dashOffset = f
		}
	}
	if v, ok := get("visibility"); ok {
		st.visible = strings.EqualFold(strings.TrimSpace(v), "visible")
	}
	if v, ok := get("marker-start"); ok {
		st.markerStart = parseURLRef(v)
	}
	if v, ok := get("marker-mid"); ok {
		st.markerMid = parseURLRef(v)
	}
	if v, ok := get("marker-end"); ok {
		st.markerEnd = parseURLRef(v)
	}
	if v, ok := get("paint-order"); ok {
		st.paintOrder = parsePaintOrder(v)
	}
	if v, ok := get("shape-rendering"); ok {
		lower := strings.ToLower(strings.TrimSpace(v))
		st.crisp = lower == "crispedges" || lower == "optimizespeed"
	}
}

func parseFillRule(v string, def fillType) fillType {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "evenodd":
		return fillEvenOdd
	case "nonzero":
		return fillWinding
	}
	return def
}

// parseURLRef returns the fragment id of url(#id), or "".
func parseURLRef(v string) string {
	v = strings.TrimSpace(v)
	if len(v) < 5 || !strings.EqualFold(v[:4], "url(") {
		return ""
	}
	end := strings.IndexByte(v, ')')
	if end < 0 {
		return ""
	}
	ref := strings.TrimSpace(strings.Trim(strings.TrimSpace(v[4:end]), `"'`))
	if i := strings.IndexByte(ref, '#'); i >= 0 {
		return ref[i+1:]
	}
	return ""
}

func parsePaintOrder(v string) [3]uint8 {
	order := [3]uint8{orderFill, orderStroke, orderMarkers}
	fields := strings.Fields(strings.ToLower(v))
	if len(fields) == 0 || fields[0] == "normal" {
		return order
	}
	var out []uint8
	seen := [3]bool{}
	for _, f := range fields {
		var k uint8
		switch f {
		case "fill":
			k = orderFill
		case "stroke":
			k = orderStroke
		case "markers":
			k = orderMarkers
		default:
			return order
		}
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for k := uint8(0); k < 3; k++ {
		if !seen[k] {
			out = append(out, k)
		}
	}
	copy(order[:], out)
	return order
}

type lengthMode uint8

const (
	lengthWidth lengthMode = iota
	lengthHeight
	lengthOther
)

// length resolves an SVG length value to user units. A percentage refers
// to the viewport: its width, its height, or its normalized diagonal.
func (p *painter) length(n *dom.Node, v string, mode lengthMode) (float32, bool) {
	v = strings.TrimSpace(v)
	if strings.HasSuffix(v, "%") {
		f, ok := parseFloat(v[:len(v)-1])
		if !ok {
			return 0, false
		}
		var base float32
		switch mode {
		case lengthWidth:
			base = p.viewportW
		case lengthHeight:
			base = p.viewportH
		default:
			base = sqrt32((float32(p.viewportW*p.viewportW) + float32(p.viewportH*p.viewportH)) / 2)
		}
		return float32(float32(f) * base / 100), true
	}
	f, ok := p.eng.RasterLength(n, v, true)
	return float32(f), ok
}

// parseDashArray is ResolveSVGDashArray: a list of lengths. A negative
// value makes the list invalid.
func (p *painter) parseDashArray(n *dom.Node, v string) []float32 {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "none") {
		return nil
	}
	fields := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	out := make([]float32, 0, len(fields))
	for _, f := range fields {
		l, ok := p.length(n, f, lengthOther)
		if !ok || l < 0 {
			return nil
		}
		out = append(out, l)
	}
	return out
}

// attrLength resolves a geometry property of an element: the CSS
// declaration when there is one, the attribute otherwise.
func (p *painter) attrLength(n *dom.Node, name string, mode lengthMode) float32 {
	if v, ok := p.eng.RasterDeclared(n, name); ok && !strings.EqualFold(v, "auto") {
		if f, ok := p.length(n, v, mode); ok {
			return f
		}
	}
	if v, ok := n.GetAttr(name); ok {
		v = strings.TrimSpace(v)
		if strings.HasSuffix(v, "%") {
			if f, ok := p.length(n, v, mode); ok {
				return f
			}
			return 0
		}
	}
	return p.eng.RasterAttrLength(n, name)
}

// hasLength reports whether a geometry property has a value other than
// auto.
func (p *painter) hasLength(n *dom.Node, name string) bool {
	if v, ok := p.eng.RasterDeclared(n, name); ok {
		return !strings.EqualFold(strings.TrimSpace(v), "auto")
	}
	v, ok := n.GetAttr(name)
	return ok && strings.TrimSpace(v) != "" && !strings.EqualFold(strings.TrimSpace(v), "auto")
}

// ---- shapes ----

// shapeGeometry is the geometry of a shape element, as LayoutSVGShape
// keeps it.
type shapeGeometry struct {
	kind uint8
	rect Rect // the rectangle of a rect, or the bounds of an ellipse
}

const (
	geomEmpty = iota
	geomRect
	geomOval
	geomPath
)

// buildShape computes the geometry of a shape element. A path goes into
// p.path.
func (p *painter) buildShape(n *dom.Node) shapeGeometry {
	path := &p.path
	path.Reset()
	switch n.Tag {
	case "rect":
		w := p.attrLength(n, "width", lengthWidth)
		h := p.attrLength(n, "height", lengthHeight)
		if !(w > 0 && h > 0) {
			return shapeGeometry{}
		}
		x := p.attrLength(n, "x", lengthWidth)
		y := p.attrLength(n, "y", lengthHeight)
		rect := Rect{x, y, x + w, y + h}
		rx, ry := float32(0), float32(0)
		hasRx, hasRy := p.hasLength(n, "rx"), p.hasLength(n, "ry")
		if hasRx {
			rx = p.attrLength(n, "rx", lengthWidth)
		}
		if hasRy {
			ry = p.attrLength(n, "ry", lengthHeight)
		}
		if !hasRx || rx < 0 {
			rx = max(0, ry)
		}
		if !hasRy || ry < 0 {
			ry = rx
		}
		if rx > 0 || ry > 0 {
			rx = min(rx, float32(w*0.5))
			ry = min(ry, float32(h*0.5))
			path.AddRRect(rect, rx, ry)
			return shapeGeometry{kind: geomPath, rect: rect}
		}
		return shapeGeometry{kind: geomRect, rect: rect}
	case "circle", "ellipse":
		var rx, ry float32
		if n.Tag == "circle" {
			rx = p.attrLength(n, "r", lengthOther)
			ry = rx
		} else {
			hasRx, hasRy := p.hasLength(n, "rx"), p.hasLength(n, "ry")
			if hasRx {
				rx = p.attrLength(n, "rx", lengthWidth)
			}
			if hasRy {
				ry = p.attrLength(n, "ry", lengthHeight)
			}
			if !hasRx {
				rx = ry
			} else if !hasRy {
				ry = rx
			}
		}
		if !(rx > 0 && ry > 0) {
			return shapeGeometry{}
		}
		cx := p.attrLength(n, "cx", lengthWidth)
		cy := p.attrLength(n, "cy", lengthHeight)
		// gfx::RectF(center - radii, radii * 2) as an SkRect.
		l, t := cx-rx, cy-ry
		return shapeGeometry{kind: geomOval, rect: Rect{l, t, l + float32(rx*2), t + float32(ry*2)}}
	case "line", "polyline", "polygon", "path":
		if !appendEnginePath(path, p.eng.RasterPathOf(n)) {
			return shapeGeometry{}
		}
		return shapeGeometry{kind: geomPath}
	}
	return shapeGeometry{}
}

// appendEnginePath copies a path of the layout engine. It reports false
// for an empty path.
func appendEnginePath(dst *Path, src browser.RasterPath) bool {
	pi, wi := 0, 0
	pts := src.Points
	for _, v := range src.Verbs {
		switch v {
		case browser.RasterVerbMove:
			dst.MoveTo(pts[pi], pts[pi+1])
			pi += 2
		case browser.RasterVerbLine:
			dst.LineTo(pts[pi], pts[pi+1])
			pi += 2
		case browser.RasterVerbQuad:
			dst.QuadTo(pts[pi], pts[pi+1], pts[pi+2], pts[pi+3])
			pi += 4
		case browser.RasterVerbConic:
			dst.ConicTo(pts[pi], pts[pi+1], pts[pi+2], pts[pi+3], src.Weights[wi])
			pi += 4
			wi++
		case browser.RasterVerbCubic:
			dst.CubicTo(pts[pi], pts[pi+1], pts[pi+2], pts[pi+3], pts[pi+4], pts[pi+5])
			pi += 6
		case browser.RasterVerbClose:
			dst.Close()
		}
	}
	return !dst.isEmpty()
}

// deviceMatrix converts the accumulated transform to the matrix of the
// canvas.
func deviceMatrix(ctm Affine) Matrix { return ctm.toMatrix() }

// paintShape is SVGShapePainter::PaintShape.
func (p *painter) paintShape(n *dom.Node, st *svgStyle, ctm Affine) {
	geom := p.buildShape(n)
	if geom.kind == geomEmpty {
		return
	}
	if !st.visible {
		return
	}
	m := deviceMatrix(ctm)
	dev := p.cv.dev()
	for _, order := range st.paintOrder {
		switch order {
		case orderFill:
			var paint Paint
			if !p.preparePaint(n, st, false, &paint, ctm) {
				continue
			}
			paint.antiAlias = !st.crisp
			switch geom.kind {
			case geomRect:
				p.cv.dr.drawRect(dev, geom.rect, &paint, &m)
			case geomOval:
				p.cv.dr.drawOval(dev, geom.rect, &paint, &m)
			default:
				p.path.fill = st.fillRule
				p.cv.dr.drawPath(dev, &p.path, &paint, &m)
			}
		case orderStroke:
			if !(st.strokeWidth > 0) {
				continue
			}
			var paint Paint
			if !p.preparePaint(n, st, true, &paint, ctm) {
				continue
			}
			paint.antiAlias = !st.crisp
			p.setStroke(&paint, st)
			switch geom.kind {
			case geomRect:
				p.cv.dr.drawRect(dev, geom.rect, &paint, &m)
			case geomOval:
				p.cv.dr.drawOval(dev, geom.rect, &paint, &m)
			default:
				p.path.fill = fillWinding
				p.cv.dr.drawPath(dev, &p.path, &paint, &m)
			}
		case orderMarkers:
			if st.markerStart != "" || st.markerMid != "" || st.markerEnd != "" {
				p.paintMarkers(n, st, ctm, geom)
				// The markers reuse p.path: build the shape again for
				// the remaining paint-order steps.
				geom = p.buildShape(n)
			}
		}
	}
}

// setStroke is StrokeData::SetupPaint.
func (p *painter) setStroke(paint *Paint, st *svgStyle) {
	paint.stroke = true
	paint.width = st.strokeWidth
	paint.cap = st.cap
	paint.join = st.join
	paint.miterLimit = st.miter
	paint.dash = nil
	if n := len(st.dashArray); n > 0 {
		count := n
		if n%2 != 0 {
			count = n * 2
		}
		intervals := make([]float32, count)
		for i := range intervals {
			intervals[i] = st.dashArray[i%n]
		}
		if d, ok := newDash(intervals, st.dashOffset); ok {
			paint.dash = &d
		}
	}
}

// preparePaint is SVGObjectPainter::PreparePaint: it sets the color or the
// shader of a fill or a stroke. It reports false when there is nothing to
// paint.
func (p *painter) preparePaint(n *dom.Node, st *svgStyle, stroke bool, paint *Paint, ctm Affine) bool {
	sp := st.fill
	alpha := st.fillOpacity
	if stroke {
		sp = st.stroke
		alpha = st.strokeOpacity
	}
	switch sp.kind {
	case paintContextFill:
		if p.context == nil {
			return false
		}
		sp = p.context.fill
	case paintContextStroke:
		if p.context == nil {
			return false
		}
		sp = p.context.stroke
	}
	if sp.kind == paintURL {
		if sh := p.paintServer(n, sp.url, ctm); sh != nil {
			paint.shader = sh
			paint.R, paint.G, paint.B = 0, 0, 0
			// The alpha is quantized to 8 bits.
			a := alpha
			if !(a > 0) {
				a = 0
			}
			paint.A = uint8(float32(a*255) + 0.5)
			return true
		}
		sp = svgPaint{kind: sp.fallbackKind, color: sp.fallbackColor}
	}
	switch sp.kind {
	case paintColor, paintCurrentColor:
		c := sp.color
		paint.shader = nil
		paint.R, paint.G, paint.B = c.R, c.G, c.B
		paint.A = cssAlphaByte(c, alpha)
		return true
	}
	return false
}

// paintNestedSVG paints an inner svg element: its viewport clips unless
// overflow is visible. The viewport transform is part of the local
// transform.
func (p *painter) paintNestedSVG(n *dom.Node, st *svgStyle, ctm Affine) {
	savedW, savedH := p.viewportW, p.viewportH
	defer func() { p.viewportW, p.viewportH = savedW, savedH }()
	w, hasW := float32(0), p.hasLength(n, "width")
	h, hasH := float32(0), p.hasLength(n, "height")
	if hasW {
		w = p.attrLength(n, "width", lengthWidth)
	} else {
		w = savedW
	}
	if hasH {
		h = p.attrLength(n, "height", lengthHeight)
	} else {
		h = savedH
	}
	if !(w > 0 && h > 0) {
		return
	}
	if vb := browser.RasterParseNumbers(n.AttrOr("viewBox", "")); len(vb) == 4 {
		if !(vb[2] > 0 && vb[3] > 0) {
			return
		}
		p.viewportW, p.viewportH = vb[2], vb[3]
	} else {
		p.viewportW, p.viewportH = w, h
	}
	p.paintWithEffects(n, st, ctm, func(st *svgStyle, ctm Affine) {
		if p.overflowHidden(n) {
			// The viewport clips the content. The clip is the rectangle x,
			// y, width, height in the space of the parent: the local
			// transform maps the content to that space.
			x := p.attrLength(n, "x", lengthWidth)
			y := p.attrLength(n, "y", lengthHeight)
			local := p.localTransform(n)
			inv, ok := local.Invert()
			if !ok {
				return
			}
			dev := p.cv.dev()
			savedClip, savedAA := dev.clip, dev.aa
			defer func() { dev.clip, dev.aa = savedClip, savedAA }()
			if !p.clipRectAA(dev, Rect{x, y, x + w, y + h}, ctm.Mul(inv)) {
				return
			}
		}
		p.paintChildren(n, st, ctm)
	})
}

// nearlyIntegral is nearly_integral of skia/src/core/SkRasterClip.cpp: the
// anti-aliasing has a granularity of a quarter of a pixel, so a coordinate
// that is less than an eighth of a pixel away from a whole number counts as
// that number.
func nearlyIntegral(x float32) bool {
	const domain = float32(1.0 / 4)
	const halfDomain = domain / 2
	x += halfDomain
	return x-float32(math.Floor(float64(x))) < domain
}

// clipRectAA is SkCanvas::clipRect with anti-aliasing, as SkRasterClip::op
// applies it for a rectangle: the rectangle in user space intersects the
// clip of the device. A clip without coverage stays without coverage when
// the edges of the rectangle are nearly on whole pixels. It reports false
// when nothing is left.
func (p *painter) clipRectAA(dev *device, r Rect, ctm Affine) bool {
	if p.vector != nil {
		var path, transformed Path
		path.AddRect(r, 0)
		m := deviceMatrix(ctm)
		path.transformInto(&transformed, &m)
		return p.vector.clipPath(&transformed)
	}
	m := deviceMatrix(ctm)
	var devPath Path
	devPath.lastMoveIdx = -1
	if m.isScaleTranslate() {
		dr := m.mapRect(r)
		if dev.aa == nil && nearlyIntegral(dr.L) && nearlyIntegral(dr.T) && nearlyIntegral(dr.R) && nearlyIntegral(dr.B) {
			if !dev.clip.intersect(dr.round()) {
				dev.clip = IRect{}
				return false
			}
			return true
		}
		// SkAAClip::op for a rectangle with coverage.
		pixelBounds := dev.clip
		if !pixelBounds.intersect(dr.roundOut()) {
			dev.clip = IRect{}
			return false
		}
		if c := dev.clip.toRect(); dr.L <= c.L && dr.T <= c.T && dr.R >= c.R && dr.B >= c.B {
			return true
		}
		devPath.AddRect(dr, 0)
	} else {
		var path Path
		path.lastMoveIdx = -1
		path.AddRect(r, 0)
		path.transformInto(&devPath, &m)
	}
	return p.clipPathAA(dev, &devPath)
}

// clipPathAA is SkRasterClip::op for a path in device space with
// anti-aliasing: the coverage of the path, inside the bounds of the clip,
// multiplies the clip. It reports false when nothing is left.
func (p *painter) clipPathAA(dev *device, devPath *Path) bool {
	if p.vector != nil {
		return p.vector.clipPath(devPath)
	}
	bounds := devPath.bounds().roundOut()
	if !bounds.intersect(dev.clip) {
		dev.clip = IRect{}
		return false
	}
	clip := newAAClip(bounds)
	b := maskBuilderBlitter{clip: clip}
	p.cv.dr.aaa.antiFillPathAA(devPath, bounds, &b)
	p.intersectClip(dev, clip)
	return !dev.clip.isEmpty()
}

// paintUse paints the element that a use element references.
func (p *painter) paintUse(n *dom.Node, st *svgStyle, ctm Affine) {
	href, ok := n.GetAttr("href")
	if !ok {
		href, ok = n.GetAttr("xlink:href")
	}
	if !ok || !strings.HasPrefix(href, "#") {
		return
	}
	target := p.ids[href[1:]]
	if target == nil || target == n {
		return
	}
	for a := n; a != nil; a = a.Parent {
		if a == target {
			return
		}
	}
	x := p.attrLength(n, "x", lengthWidth)
	y := p.attrLength(n, "y", lengthHeight)
	ctm = ctm.Translate(float64(x), float64(y))
	p.paintWithEffects(n, st, ctm, func(st *svgStyle, ctm Affine) {
		if target.Tag == "symbol" {
			inner := *st
			p.resolveStyle(target, &inner)
			p.paintChildren(target, &inner, ctm)
			return
		}
		p.paintElement(target, st, ctm)
	})
}

// paintImage paints an image element with a data URL. Remote images are
// not loaded.
func (p *painter) paintImage(n *dom.Node, st *svgStyle, ctm Affine) {}
