// This file ports no upstream file. It exports read-only views of the engine
// for the rasterizer.

package browser

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// This file exports read-only views of the engine for the rasterizer
// (internal/raster): computed style, geometry and the results of text and
// HTML layout. The views copy what they return. Nothing here changes the
// engine.

// RasterDeclared returns the cascaded value of a property on the element
// itself: the winner of the style sheets, the style attribute and the
// presentation attributes. The second result is false when no declaration
// applies, or when the winning value is initial, unset or revert.
func (e *Engine) RasterDeclared(n *dom.Node, prop string) (string, bool) {
	return e.declared(n, prop)
}

// RasterLookup returns the value of a property with inheritance: the value
// of the nearest ancestor (or the element) that declares it, when the
// property inherits. It returns "" when no element declares it.
func (e *Engine) RasterLookup(n *dom.Node, prop string) string {
	return e.lookup(n, prop)
}

// RasterDisplay returns the computed display of an element.
func (e *Engine) RasterDisplay(n *dom.Node) string {
	if n == nil || n.Type != dom.ElementNode {
		return "inline"
	}
	return e.display(n)
}

// RasterTextStyle is the resolved font of an element.
type RasterTextStyle struct {
	// Size is the computed font size in px.
	Size float64
	// FontSize is the size of the platform font (the computed size cut to
	// 1/100 px, as Blink does).
	FontSize   float64
	Weight     int
	Italic     bool
	Families   []string
	Visibility string
	// Ex is the length of the CSS unit ex.
	Ex float64
}

// RasterTextStyleOf returns the resolved font of an element.
func (e *Engine) RasterTextStyleOf(n *dom.Node) RasterTextStyle {
	ts := e.textStyleOf(n)
	return RasterTextStyle{
		Size:       ts.size,
		FontSize:   ts.fontSize(),
		Weight:     ts.weight,
		Italic:     ts.italic,
		Families:   append([]string(nil), ts.families...),
		Visibility: ts.visibility,
		Ex:         e.exSize(ts),
	}
}

// RasterPrimaryMetrics returns the vertical metrics of the primary font of
// an element at its font size. ok is false when no font resolves.
func (e *Engine) RasterPrimaryMetrics(n *dom.Node) (m font.Metrics, face *font.Face, ok bool) {
	ts := e.textStyleOf(n)
	p := e.chainOf(ts).Primary()
	if p == nil {
		return font.Metrics{}, nil, false
	}
	return p.Metrics(ts.fontSize()), p, true
}

// RasterAttrLength resolves a geometry attribute (x, y, width, height, cx,
// cy, r, rx, ry, x1, ...) of an SVG element to user units, as SVGLength
// does. A percentage resolves against zero: the caller handles percentages.
func (e *Engine) RasterAttrLength(n *dom.Node, name string) float32 {
	return e.attrLength(n, name)
}

// RasterLength resolves a CSS length value for an element to px. em and ex
// use the font of the element. unitlessOK allows a plain number, as SVG
// presentation attributes do.
func (e *Engine) RasterLength(n *dom.Node, value string, unitlessOK bool) (float64, bool) {
	return e.cssLength(value, e.textStyleOf(n), unitlessOK)
}

// RasterLocalTransform returns the transform from the user space of an
// element to the user space of its parent, as the six values a, b, c, d, e,
// f of an affine matrix. It includes the viewport transform of a nested
// svg element. It applies a transform-origin of px lengths or numbers. A
// keyword or a percentage in transform-origin counts as 0.
func (e *Engine) RasterLocalTransform(n *dom.Node) [6]float64 {
	m := e.localTransform(n)
	return [6]float64{m.a, m.b, m.c, m.d, m.e, m.f}
}

// RasterParseTransform parses an SVG transform list or a CSS transform
// value with px and deg units.
func RasterParseTransform(value string, css bool) [6]float64 {
	var m affine
	if css {
		m = parseCSSTransform(value)
	} else {
		m = parseTransform(value)
	}
	return [6]float64{m.a, m.b, m.c, m.d, m.e, m.f}
}

// RasterParseNumbers parses a list of numbers separated by white space or
// commas, with the number parser of Blink.
func RasterParseNumbers(s string) []float32 { return parseNumberList(s) }

// Path verbs of RasterPath.
const (
	RasterVerbMove  = uint8(verbMove)
	RasterVerbLine  = uint8(verbLine)
	RasterVerbQuad  = uint8(verbQuad)
	RasterVerbConic = uint8(verbConic)
	RasterVerbCubic = uint8(verbCubic)
	RasterVerbClose = uint8(verbClose)
)

// RasterPath is the Skia path of a shape element: verbs, points as x, y
// pairs, and one weight per conic.
type RasterPath struct {
	Verbs   []uint8
	Points  []float32
	Weights []float32
}

// RasterPathOf returns the path that Blink builds for a path, line,
// polyline or polygon element. Elliptical arcs are conics, as in Skia.
func (e *Engine) RasterPathOf(n *dom.Node) RasterPath {
	return exportPath(e.pathOf(n))
}

// RasterParsePath builds the Skia path of SVG path data.
func RasterParsePath(d string) RasterPath { return exportPath(parsePathData(d)) }

func exportPath(p *skPath) RasterPath {
	out := RasterPath{
		Verbs:   make([]uint8, len(p.verbs)),
		Points:  make([]float32, 0, len(p.pts)*2),
		Weights: append([]float32(nil), p.weights...),
	}
	for i, v := range p.verbs {
		out.Verbs[i] = uint8(v)
	}
	for _, q := range p.pts {
		out.Points = append(out.Points, q.x, q.y)
	}
	return out
}

// RasterGlyph is one shaped glyph with the face that draws it.
type RasterGlyph struct {
	Face *font.Face
	ID   uint32
	// Advance, XOffset and YOffset are in px. A positive YOffset is up.
	Advance, XOffset, YOffset float64
}

// RasterTextFragment is a run of glyphs of an SVG text element with one
// style.
type RasterTextFragment struct {
	// Owner is the element whose style applies: the text element or a
	// tspan.
	Owner *dom.Node
	// Scale is the factor between the user space of the text element and
	// the space of the fragment. Blink lays out SVG text with the font
	// that has the size of the text on the screen, and paints it under a
	// scale of 1/Scale. X, Baseline, Width, FontSize and the glyph
	// advances are in that scaled space. Scale is 1 when no ancestor
	// scales.
	Scale float32
	// X is the start of the fragment and Baseline the y of the alphabetic
	// baseline of the glyphs.
	X, Baseline float32
	// Width is the advance of the fragment.
	Width float32
	// FontSize is the size of the platform font in px.
	FontSize float64
	// Ascent and Descent are the rounded metrics of the primary font.
	Ascent, Descent float64
	Glyphs          []RasterGlyph
}

// RasterSVGText returns the fragments of a text element after layout:
// text-anchor, dx, dy and the dominant baseline are applied.
func (e *Engine) RasterSVGText(text *dom.Node) []RasterTextFragment {
	layout := e.layoutSVGText(text)
	out := make([]RasterTextFragment, 0, len(layout.frags))
	for i := range layout.frags {
		f := &layout.frags[i]
		it := f.item
		rf := RasterTextFragment{
			Owner:    it.owner,
			Scale:    layout.scale,
			X:        f.x,
			Baseline: f.glyphY,
			Width:    f.width,
			FontSize: it.style.fontSize(),
			Ascent:   it.metrics.Ascent,
			Descent:  it.metrics.Descent,
			Glyphs:   exportGlyphs(it.glyphs, 0, len(it.runes)),
		}
		out = append(out, rf)
	}
	return out
}

func exportGlyphs(glyphs []itemGlyph, lo, hi int) []RasterGlyph {
	out := make([]RasterGlyph, 0, len(glyphs))
	for i := range glyphs {
		g := &glyphs[i]
		if g.runeIdx < lo || g.runeIdx >= hi {
			continue
		}
		out = append(out, RasterGlyph{Face: g.face, ID: g.glyph.ID, Advance: g.glyph.Advance, XOffset: g.glyph.XOffset, YOffset: g.glyph.YOffset})
	}
	return out
}

// RasterHTMLFragment is the part of a text node that is on one line.
type RasterHTMLFragment struct {
	// Owner is the element whose style applies to the text.
	Owner *dom.Node
	// X is the left edge of the fragment in the coordinate system of the
	// foreignObject content. Width is its advance.
	X, Width float64
	// FontSize is the size of the platform font in px.
	FontSize float64
	// Ascent and Descent are the rounded metrics of the primary font.
	Ascent, Descent float64
	Glyphs          []RasterGlyph
}

// RasterHTMLLine is one line box of a block.
type RasterHTMLLine struct {
	// X, Y, Width and Height are the line box in the coordinate system of
	// the foreignObject content. Baseline is the distance from the top of
	// the line to the alphabetic baseline.
	X, Y, Width, Height, Baseline float64
	Fragments                     []RasterHTMLFragment
}

// RasterHTMLBox is the border box of a block-level element inside a
// foreignObject, with its lines and its child boxes in document order.
type RasterHTMLBox struct {
	Node *dom.Node
	// X, Y, W and H are the border box in the coordinate system of the
	// foreignObject content (the origin is the x and y of the
	// foreignObject).
	X, Y, W, H float64
	Lines      []RasterHTMLLine
	Children   []*RasterHTMLBox
}

// RasterForeignObject returns the box tree of the HTML content of a
// foreignObject after layout.
func (e *Engine) RasterForeignObject(fo *dom.Node) []*RasterHTMLBox {
	root := e.layoutForeignObject(fo)
	var conv func(b *htmlBox) *RasterHTMLBox
	conv = func(b *htmlBox) *RasterHTMLBox {
		o := root.origin[b]
		// The conversions keep the compiler from fusing the product inside
		// px into the sums below.
		ox, oy := float64(o[0].px()), float64(o[1].px())
		out := &RasterHTMLBox{Node: b.node, X: ox, Y: oy, W: b.w.px(), H: b.h.px()}
		for _, line := range b.lines {
			rl := RasterHTMLLine{
				X: ox + float64(line.x.px()), Y: oy + float64(line.y.px()),
				Width: line.width.px(), Height: line.height.px(), Baseline: line.baseline.px(),
			}
			for i := range line.frags {
				f := &line.frags[i]
				it := f.item
				rl.Fragments = append(rl.Fragments, RasterHTMLFragment{
					Owner:    it.owner,
					X:        rl.X + float64(f.x.px()),
					Width:    f.width.px(),
					FontSize: it.style.fontSize(),
					Ascent:   it.metrics.Ascent,
					Descent:  it.metrics.Descent,
					Glyphs:   exportGlyphs(it.glyphs, f.lo, f.hi),
				})
			}
			out.Lines = append(out.Lines, rl)
		}
		for _, c := range b.children {
			out.Children = append(out.Children, conv(c))
		}
		return out
	}
	out := make([]*RasterHTMLBox, 0, len(root.boxes))
	for _, b := range root.boxes {
		out = append(out, conv(b))
	}
	return out
}

// RasterHTMLBoxRect returns the border box of an HTML element inside a
// foreignObject, in the coordinate system of the foreignObject content. An
// inline element gets the union of its line fragments.
func (e *Engine) RasterHTMLBoxRect(n *dom.Node) (x, y, w, h float64, ok bool) {
	bx, by, bw, bh, ok := e.htmlBoxRect(n)
	return bx.px(), by.px(), bw.px(), bh.px(), ok
}

// RasterBBox returns the object bounding box of an SVG element in its own
// user space. ok is false when the element has no valid box.
func (e *Engine) RasterBBox(n *dom.Node) (x, y, w, h float32, ok bool) {
	r, ok := e.objectBBox(n)
	return r.x, r.y, r.w, r.h, ok
}
