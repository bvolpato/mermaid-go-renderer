// Ported from Chromium 156.0.8078.4 (Blink), in part: the measurement APIs
// getBBox and getBoundingClientRect. The functions of this file follow the
// functions of Blink that their comments name:
//
//   - LayoutObject::ObjectBoundingBox: core/layout/layout_object.cc
//     (LGPL-2.0-or-later).
//   - LayoutInline::QuadsForSelf: core/layout/layout_inline.cc
//     (LGPL-2.0-or-later).
//   - Element::GetBoundingClientRect: core/dom/element.cc
//     (LGPL-2.0-or-later).
//   - ComputedStyle::ApplyTransform: core/style/computed_style.cc
//     (LGPL-2.0-or-later).
//   - HasValidBoundingBoxForContainer:
//     core/layout/svg/svg_content_container.cc (BSD-3-Clause).
//   - LayoutSVGText::IsObjectBoundingBoxValid:
//     core/layout/svg/layout_svg_text.cc (BSD-3-Clause).
//   - gfx::RectF::Union and gfx::SkRectToRectF: ui/gfx/geometry/rect_f.cc
//     and skia_conversions.cc of Chromium (BSD-3-Clause).
//
// The Blink paths are in third_party/blink/renderer. The license of each
// file is the license in its header at that version. See NOTICE in this
// directory. The rest of the file ports no upstream file.

package browser

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Engine implements dom.Engine for one document. It is not safe for use by
// several goroutines. Create one Engine per render.
type Engine struct {
	// Fonts resolves font families. The default is the system library.
	Fonts *font.Library
	// ViewportWidth and ViewportHeight are the size of the browser window
	// that the reference renderer uses: 800 x 600, as in mermaid-cli.
	ViewportWidth, ViewportHeight float64
	// BodyMargin is the default margin of the body element.
	BodyMargin float64
	// ImageSize gives the natural size of the image of an img element by
	// its src, when it is not nil. The second result is false for an image
	// that fails to load. The engine reads a data URL itself and does not
	// ask for it. It never reads from the network: without ImageSize every
	// other image fails, as in a browser that cannot fetch it. See
	// image.go.
	ImageSize func(src string) (width, height float64, ok bool)

	// loadedImages has the sources of the images whose load ended, and
	// imageEpoch counts the calls of CompleteImages. See image.go.
	loadedImages map[string]bool
	imageEpoch   uint64

	// rules is the rule set of rulesDoc, rulesKey is the text of its style
	// elements, and rulesGen is the style generation of the document when
	// they were read. See collectRules.
	rules    *ruleSet
	rulesKey string
	rulesGen uint64
	rulesDoc *dom.Document
	// classBuf is storage for the classes of the ancestors of the element
	// of a cascade. See cascade.
	classBuf []string
}

// New returns an engine that uses the fonts of the system.
func New() *Engine {
	return NewWithFonts(nil)
}

// NewWithFonts returns an engine that resolves fonts with the given
// library. The platform of the library selects the rules of Chromium that
// the engine applies. A nil library is the system library.
func NewWithFonts(fonts *font.Library) *Engine {
	if fonts == nil {
		fonts = font.System()
	}
	return &Engine{
		Fonts:          fonts,
		ViewportWidth:  800,
		ViewportHeight: 600,
		BodyMargin:     8,
	}
}

// NewDocument returns an empty document that measures with a new engine.
func NewDocument() *dom.Document {
	return NewDocumentWithFonts(nil)
}

// NewDocumentWithFonts returns an empty document that measures with a new
// engine that uses the given font library. A nil library is the system
// library.
func NewDocumentWithFonts(fonts *font.Library) *dom.Document {
	doc := dom.NewDocument()
	doc.Engine = NewWithFonts(fonts)
	return doc
}

// ContainerWidth is the width of a block child of the body: what
// parentElement.offsetWidth returns for the element that holds the SVG.
func (e *Engine) ContainerWidth() float64 {
	return e.ViewportWidth - float64(2*e.BodyMargin)
}

// notRendered lists the SVG elements that never take part in a bounding box.
var notRendered = map[string]bool{
	"defs": true, "symbol": true, "marker": true, "clipPath": true, "mask": true, "pattern": true,
	"linearGradient": true, "radialGradient": true, "filter": true, "style": true, "title": true,
	"desc": true, "metadata": true, "script": true, "stop": true, "animate": true, "animateMotion": true,
	"animateTransform": true, "set": true, "mpath": true, "view": true, "cursor": true,
}

// localTransform is the transform that maps the user space of an element to
// the user space of its parent.
func (e *Engine) localTransform(n *dom.Node) affine {
	m := identity
	// transform-origin: the transform is about that point. It is a
	// presentation attribute and a CSS property.
	ox, oy := e.transformOrigin(n)
	// The CSS properties translate, rotate and scale come before the
	// transform property (ComputedStyle::ApplyTransform).
	list := e.individualTransforms(n)
	if v, ok := e.declared(n, "transform"); ok && !strings.EqualFold(v, "none") {
		list += cssTransformText(v)
	} else if v, ok := n.GetAttr("transform"); ok {
		list += v
	}
	if strings.TrimSpace(list) != "" {
		m = parseTransformListOrigin(list, ox, oy)
	}
	if n.NS == dom.NSSVG && n.Tag == "svg" && n.Parent != nil && n.Parent.NS == dom.NSSVG {
		m = m.multiply(e.nestedSVGTransform(n))
	}
	return m
}

// parseCSSTransform parses the translate, scale, rotate and matrix
// functions of the CSS transform property.
func parseCSSTransform(v string) affine {
	return parseTransform(cssTransformText(v))
}

// cssTransformText turns the value of the CSS transform property into the
// form of the transform attribute. CSS functions take units: px and deg
// are user units, so they go away.
func cssTransformText(v string) string {
	return strings.NewReplacer("px", "", "deg", "").Replace(v)
}

// individualTransforms returns the CSS properties translate, rotate and
// scale of an element as functions of a transform list, in the order that
// CSS applies them. A highlighted slice of a pie chart has scale: 1.05. A
// percentage of translate and a rotation about another axis are not
// handled: they count as none.
func (e *Engine) individualTransforms(n *dom.Node) string {
	var list strings.Builder
	value := func(prop string) []string {
		v, ok := e.declared(n, prop)
		if !ok || strings.EqualFold(strings.TrimSpace(v), "none") {
			return nil
		}
		return strings.Fields(strings.ToLower(v))
	}
	if f := value("translate"); len(f) == 1 || len(f) == 2 {
		ok := true
		for i := range f {
			if !strings.HasSuffix(f[i], "px") && f[i] != "0" {
				ok = false
			}
			f[i] = strings.TrimSuffix(f[i], "px")
		}
		if ok {
			list.WriteString("translate(" + strings.Join(f, ", ") + ") ")
		}
	}
	if f := value("rotate"); len(f) == 1 && strings.HasSuffix(f[0], "deg") {
		list.WriteString("rotate(" + strings.TrimSuffix(f[0], "deg") + ") ")
	}
	if f := value("scale"); len(f) == 1 || len(f) == 2 {
		ok := true
		for i := range f {
			if strings.HasSuffix(f[i], "%") {
				// A percentage is a number: 150% is 1.5.
				p, valid := parseCSSNumber(strings.TrimSuffix(f[i], "%"))
				ok = ok && valid
				f[i] = css.FormatNumber(p / 100)
			}
		}
		if ok {
			list.WriteString("scale(" + strings.Join(f, ", ") + ") ")
		}
	}
	return list.String()
}

// transformOrigin returns the transform-origin of an element in user
// units, for the lengths in px and the numbers. A keyword or a percentage
// refers to the reference box, which the engine does not resolve: it
// counts as 0.
func (e *Engine) transformOrigin(n *dom.Node) (x, y float32) {
	v, ok := e.declared(n, "transform-origin")
	if !ok {
		return 0, 0
	}
	fields := strings.Fields(strings.ReplaceAll(v, ",", " "))
	parse := func(i int) float32 {
		if i >= len(fields) {
			return 0
		}
		f, rest, ok := parseSVGNumber(fields[i])
		if !ok || (rest != "" && !strings.EqualFold(rest, "px")) {
			return 0
		}
		return clampCSSLength(f)
	}
	return parse(0), parse(1)
}

// nestedSVGTransform is the viewport transform of an inner <svg>: its x and
// y, and the mapping of its viewBox into its width and height.
func (e *Engine) nestedSVGTransform(n *dom.Node) affine {
	ts := e.textStyleOf(n)
	x, _ := e.firstLength(n, "x", ts.size)
	y, _ := e.firstLength(n, "y", ts.size)
	m := identity.translate(float64(x), float64(y))
	vb := parseNumberList(n.AttrOr("viewBox", ""))
	w, hasW := e.firstLength(n, "width", ts.size)
	h, hasH := e.firstLength(n, "height", ts.size)
	if len(vb) == 4 && vb[2] > 0 && vb[3] > 0 && hasW && hasH {
		sx := float64(w) / float64(vb[2])
		sy := float64(h) / float64(vb[3])
		par := strings.TrimSpace(n.AttrOr("preserveAspectRatio", "xMidYMid meet"))
		if par != "none" {
			s := min(sx, sy)
			if strings.HasSuffix(par, "slice") {
				s = max(sx, sy)
			}
			// The free space is split by the alignment: none, half or all.
			freeX := float64(w) - float64(float64(vb[2])*s)
			freeY := float64(h) - float64(float64(vb[3])*s)
			tx, ty := freeX/2, freeY/2
			if strings.HasPrefix(par, "xMin") {
				tx = 0
			} else if strings.HasPrefix(par, "xMax") {
				tx = freeX
			}
			if strings.Contains(par, "YMin") {
				ty = 0
			} else if strings.Contains(par, "YMax") {
				ty = freeY
			}
			m = m.translate(tx, ty).scale(s, s)
		} else {
			m = m.scale(sx, sy)
		}
		m = m.translate(-float64(vb[0]), -float64(vb[1]))
	}
	return m
}

func (e *Engine) attrLength(n *dom.Node, name string) float32 {
	v, _ := e.geometryLength(n, name)
	return v
}

// geometryLength returns a geometry property of an SVG element: x, y,
// width, height, cx, cy, r, rx or ry. These are CSS properties, and the
// attribute with the same name is a presentation attribute. So a
// declaration of the style wins over the attribute: a foreignObject with
// width="6.125" and style="width: 9px" is 9 wide. The second result is
// false when the element has neither.
func (e *Engine) geometryLength(n *dom.Node, name string) (float32, bool) {
	ts := e.textStyleOf(n)
	if v, ok := e.declared(n, name); ok {
		if px, ok := e.cssLength(v, ts, true); ok {
			return float32(px), true
		}
	}
	return e.firstLength(n, name, ts.size)
}

// foreignObjectFrame is the box of a foreignObject in the user space of
// its parent. LayoutSVGForeignObject is a block: its x, y, width and height
// become LayoutUnits, and the LayoutUnit(float) constructor truncates.
// getBoundingClientRect of the foreignObject and of the HTML in it start
// from this box. getBBox returns the lengths that are not truncated.
func (e *Engine) foreignObjectFrame(fo *dom.Node) luRect {
	return luRect{
		luFromFloat(e.attrLength(fo, "x")), luFromFloat(e.attrLength(fo, "y")),
		luFromFloat(max(e.attrLength(fo, "width"), 0)), luFromFloat(max(e.attrLength(fo, "height"), 0)),
	}
}

// objectBBox is LayoutObject::ObjectBoundingBox for an SVG element. The
// second result is false when the element has no valid bounding box for its
// container (HasValidBoundingBoxForContainer in Blink).
func (e *Engine) objectBBox(n *dom.Node) (rectF, bool) {
	if n == nil || n.Type != dom.ElementNode || n.NS != dom.NSSVG {
		return rectF{}, false
	}
	if notRendered[n.Tag] || e.svgBoxless(n) {
		return rectF{}, false
	}
	switch n.Tag {
	case "svg", "g", "a", "switch":
		bbox, valid := e.containerBBox(n)
		// A g with display: none is a hidden container: it has the box of
		// its content, and its own container does not count it.
		return bbox, valid && !e.isHiddenContainer(n)
	case "rect":
		// An empty rect is not part of the box of its container, but
		// getBBox on the rect itself returns its geometry.
		w, h := max(e.attrLength(n, "width"), 0), max(e.attrLength(n, "height"), 0)
		return rectF{e.attrLength(n, "x"), e.attrLength(n, "y"), w, h}, w > 0 && h > 0
	case "circle":
		r := e.attrLength(n, "r")
		if r <= 0 {
			return rectF{}, false
		}
		cx, cy := e.attrLength(n, "cx"), e.attrLength(n, "cy")
		return rectF{cx - r, cy - r, float32(2 * r), float32(2 * r)}, true
	case "ellipse":
		rx, ry := e.attrLength(n, "rx"), e.attrLength(n, "ry")
		_, hasRx := n.GetAttr("rx")
		_, hasRy := n.GetAttr("ry")
		if hasRx && !hasRy {
			ry = rx
		} else if hasRy && !hasRx {
			rx = ry
		}
		if rx <= 0 || ry <= 0 {
			return rectF{}, false
		}
		cx, cy := e.attrLength(n, "cx"), e.attrLength(n, "cy")
		return rectF{cx - rx, cy - ry, float32(2 * rx), float32(2 * ry)}, true
	case "line":
		x1, y1 := e.attrLength(n, "x1"), e.attrLength(n, "y1")
		x2, y2 := e.attrLength(n, "x2"), e.attrLength(n, "y2")
		// A line, a polyline and a polygon are paths in Blink: the box is
		// the bounds of the Skia path, converted with gfx::SkRectToRectF.
		return rectFromBounds(min(x1, x2), min(y1, y2), max(x1, x2), max(y1, y2)), true
	case "polyline", "polygon":
		pts := parseNumberList(n.AttrOr("points", ""))
		if len(pts) < 2 {
			return rectF{}, false
		}
		pts = pts[:len(pts)&^1]
		minX, minY, maxX, maxY := pts[0], pts[1], pts[0], pts[1]
		for i := 2; i < len(pts); i += 2 {
			minX, maxX = min(minX, pts[i]), max(maxX, pts[i])
			minY, maxY = min(minY, pts[i+1]), max(maxY, pts[i+1])
		}
		return rectFromBounds(minX, minY, maxX, maxY), true
	case "path":
		p := e.pathOf(n)
		if p.isEmpty() {
			return rectF{}, false
		}
		return p.tightBounds(), true
	case "image":
		w, h := e.attrLength(n, "width"), e.attrLength(n, "height")
		if w <= 0 || h <= 0 {
			return rectF{}, false
		}
		return rectF{e.attrLength(n, "x"), e.attrLength(n, "y"), w, h}, true
	case "foreignObject":
		w, h := e.attrLength(n, "width"), e.attrLength(n, "height")
		if w <= 0 || h <= 0 {
			return rectF{}, false
		}
		return rectF{e.attrLength(n, "x"), e.attrLength(n, "y"), w, h}, true
	case "text":
		r, ok := e.svgTextBBox(n)
		if !ok && e.svgTextHasInlineBox(n) {
			// A text with no characters and with an inline box is valid
			// for its container: an empty box at the origin of the text.
			return rectF{}, true
		}
		return r, ok
	case "tspan", "textPath":
		return e.svgTextBBox(n)
	}
	return rectF{}, false
}

// svgTextHasInlineBox reports whether a text element has a child that makes
// an inline box: a tspan, an a or a textPath that is displayed. Blink then
// lays out a line for the text, also when the text has no characters
// (LayoutSVGText::IsObjectBoundingBoxValid).
func (e *Engine) svgTextHasInlineBox(text *dom.Node) bool {
	for c := text.FirstChild; c != nil; c = c.Next {
		if c.Type != dom.ElementNode || c.NS != dom.NSSVG {
			continue
		}
		switch c.Tag {
		case "tspan", "a", "textPath":
			if e.display(c) != "none" {
				return true
			}
		}
	}
	return false
}

// containerBBox is SVGContentContainer's bounding box computation: the
// union of the valid children, each mapped by its own transform.
func (e *Engine) containerBBox(n *dom.Node) (rectF, bool) {
	// Blink computes the box only when the layout of a child reports that
	// bounds changed. See boundschange.go.
	res := e.containerLayout(n)
	return res.bbox, res.valid
}

func (e *Engine) pathOf(n *dom.Node) *skPath {
	st := e.styleOf(n)
	if st.layoutOK {
		if p, ok := st.layout.(*skPath); ok {
			return p
		}
	}
	var p *skPath
	switch n.Tag {
	case "path":
		d := n.AttrOr("d", "")
		if v, ok := e.declared(n, "d"); ok && strings.HasPrefix(strings.TrimSpace(v), "path(") {
			v = strings.TrimSpace(v)
			d = strings.Trim(v[5:len(v)-1], `"' `)
		}
		p = parsePathDataFor(d, e.Fonts.Platform())
	case "line":
		p = newPath()
		p.moveTo(e.attrLength(n, "x1"), e.attrLength(n, "y1"))
		p.lineTo(e.attrLength(n, "x2"), e.attrLength(n, "y2"))
	case "polyline", "polygon":
		p = newPath()
		pts := parseNumberList(n.AttrOr("points", ""))
		pts = pts[:len(pts)&^1]
		for i := 0; i < len(pts); i += 2 {
			if i == 0 {
				p.moveTo(pts[0], pts[1])
			} else {
				p.lineTo(pts[i], pts[i+1])
			}
		}
		if n.Tag == "polygon" && len(pts) > 0 {
			p.close()
		}
	case "rect":
		p = newPath()
		x, y := e.attrLength(n, "x"), e.attrLength(n, "y")
		w, h := e.attrLength(n, "width"), e.attrLength(n, "height")
		if w > 0 && h > 0 {
			p.moveTo(x, y)
			p.lineTo(x+w, y)
			p.lineTo(x+w, y+h)
			p.lineTo(x, y+h)
			p.close()
		}
	default:
		p = newPath()
	}
	st.layout, st.layoutOK = p, true
	return p
}

// ---- dom.Engine ----

// BBox is getBBox().
func (e *Engine) BBox(n *dom.Node) dom.Rect {
	if has, _ := e.svgLayoutState(n); !has {
		// No layout object: see norender.go.
		return dom.Rect{}
	}
	r, _ := e.objectBBox(n)
	return r.toDOM()
}

// ComputedTextLength is getComputedTextLength().
func (e *Engine) ComputedTextLength(n *dom.Node) float64 {
	if has, _ := e.svgLayoutState(n); !has {
		return 0
	}
	return e.svgTextLength(n)
}

// TotalLength is getTotalLength().
func (e *Engine) TotalLength(n *dom.Node) float64 {
	if n.IsElement("path") {
		// A path element is measured from its normalized segment data.
		return float64(pathDataLength(n.AttrOr("d", ""), cmathOf(e.Fonts.Platform())))
	}
	return float64(e.pathOf(n).length())
}

// PointAtLength is getPointAtLength().
func (e *Engine) PointAtLength(n *dom.Node, distance float64) dom.Point {
	if n.IsElement("path") {
		p := pathDataPointAt(n.AttrOr("d", ""), float32(distance), cmathOf(e.Fonts.Platform()))
		return dom.Point{X: float64(p.x), Y: float64(p.y)}
	}
	p := e.pathOf(n).pointAt(float32(distance))
	return dom.Point{X: float64(p.x), Y: float64(p.y)}
}

// screenTransform maps the user space of an SVG element to the viewport of
// the page. Without a viewBox on the root <svg>, one user unit is one CSS
// pixel, and the SVG starts at the body margin. With a viewBox, the root
// maps its user space into its box (svgRootBoxOf).
func (e *Engine) screenTransform(n *dom.Node) affine {
	var chain []*dom.Node
	for p := n; p != nil && p.Type == dom.ElementNode && p.NS == dom.NSSVG; p = p.Parent {
		chain = append(chain, p)
	}
	m := identity.translate(e.BodyMargin, e.BodyMargin)
	for i := len(chain) - 1; i >= 0; i-- {
		el := chain[i]
		if i == len(chain)-1 && el.Tag == "svg" {
			// The outermost svg: its own transform is the page offset and
			// the mapping of its viewBox.
			if el.HasAttr("viewBox") {
				box := e.svgRootBoxOf(el)
				m = identity.translate(float64(box.x.px()), float64(box.y.px())).multiply(box.toBorderBox)
			}
			continue
		}
		m = m.multiply(e.localTransform(el))
	}
	return m
}

// ClientRect is getBoundingClientRect().
func (e *Engine) ClientRect(n *dom.Node) dom.Rect {
	if n == nil || n.Type != dom.ElementNode {
		return dom.Rect{}
	}
	if n.NS == dom.NSSVG {
		if has, hidden := e.svgLayoutState(n); n != outermostSVG(n) && (!has || hidden) {
			// No layout object, or no place on the page: see norender.go.
			return dom.Rect{}
		}
		if n == outermostSVG(n) {
			// The outermost svg is a box of the page: its rectangle is the
			// CSS box, not the bounds of its content.
			box := e.svgRootBoxOf(n)
			return rectF{box.x.f32(), box.y.f32(), box.w.f32(), box.h.f32()}.toDOM()
		}
		if n.Tag == "foreignObject" {
			// Element::GetBoundingClientRect takes the box of a
			// foreignObject, not its object bounding box.
			if e.display(n) == "none" {
				return dom.Rect{}
			}
			f := e.foreignObjectFrame(n)
			return e.localToAbsoluteRect(n, rectF{f.x.f32(), f.y.f32(), f.w.f32(), f.h.f32()}).toDOM()
		}
		r, _ := e.objectBBox(n)
		start := n
		if n.Tag == "tspan" || n.Tag == "textPath" {
			// The box of a text content element is in the user space of
			// its <text> element.
			if text := textElementOf(n); text != nil {
				start = text
			}
		}
		return e.localToAbsoluteRect(start, r).toDOM()
	}
	fo := foreignObjectOf(n)
	if fo == nil {
		return dom.Rect{}
	}
	quads, ok := e.htmlQuads(n)
	if !ok {
		return dom.Rect{}
	}
	// Each rectangle of the element is mapped to the page, and
	// gfx::RectF::Union unites the results. The rectangles are relative to
	// the box of the foreignObject.
	frame := e.foreignObjectFrame(fo)
	// LayoutInline::QuadsForSelf maps the rectangles of an inline element
	// with one transform, the product of the transforms of the ancestors
	// (LocalToAncestorTransform). A box goes through the ancestors one by
	// one (LocalToAbsoluteQuad). The two can differ in the last bit.
	inline := !isBlockLevelDisplay(e.display(n)) && !isImage(n) && !(n.NS == dom.NSHTML && n.Tag == "br")
	var toPage affine
	if inline {
		toPage = e.screenTransform(fo)
	}
	var result rectF
	for _, q := range quads {
		r := rectF{(frame.x + q.x).f32(), (frame.y + q.y).f32(), q.w.f32(), q.h.f32()}
		if inline {
			result = result.union(toPage.mapQuadBounds(r))
			continue
		}
		result = result.union(e.localToAbsoluteRect(fo, r))
	}
	return result.toDOM()
}

// ComputedStyle is getComputedStyle(n).getPropertyValue(property), for the
// properties that mermaid reads.
func (e *Engine) ComputedStyle(n *dom.Node, property string) string {
	if n == nil || n.Type != dom.ElementNode {
		return ""
	}
	ts := e.textStyleOf(n)
	switch property {
	case "font-size":
		// The CSSOM prints a computed length with 6 significant digits.
		return css.FormatNumber(ts.size) + "px"
	case "font-family":
		// The nearest declaration, or the monospace of the style sheet of
		// the user agent for code, kbd, samp, tt and pre.
		for cur := n; cur != nil && cur.Type == dom.ElementNode; cur = cur.Parent {
			if v, ok := e.declared(cur, "font-family"); ok && !strings.EqualFold(v, "inherit") {
				if norm, ok := css.NormalizeValue("font-family", v); ok {
					return norm
				}
				return v
			}
			if cur.NS == dom.NSHTML && uaMono[cur.Tag] {
				return "monospace"
			}
		}
		return "Times"
	case "font-weight":
		return js.Str(float64(ts.weight))
	case "font-style":
		if ts.italic {
			return "italic"
		}
		return "normal"
	case "display":
		return e.display(n)
	}
	return e.lookup(n, property)
}
