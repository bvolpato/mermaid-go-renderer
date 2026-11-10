// Effects of an element: opacity, blend mode, clip path and filter. Blink
// turns them into paint property nodes, and the rasterizer of Chromium
// turns each node into a layer (SkCanvas::saveLayer) or a clip. The order
// is, from the outside: opacity and blend mode, clip path, mask, filter.
//
// The comments of this file name parts of Chromium. The license of each
// file is the license in its header at Chromium 156.0.8078.4. The Blink
// paths are in third_party/blink/renderer:
//
//   - LayoutSVGResourceClipper::AsPath and CalculateLocalClipBounds:
//     core/layout/svg/layout_svg_resource_clipper.cc (LGPL-2.0-or-later).
//   - SVGLayoutSupport::IsIsolationRequired:
//     core/layout/svg/svg_layout_support.cc (LGPL-2.0-or-later).
//   - ComputedStyle::HasGroupingProperty: core/style/computed_style.h
//     (LGPL-2.0-or-later).
//   - FragmentPaintPropertyTreeBuilder::UpdateClipPathClip:
//     core/paint/paint_property_tree_builder.cc (BSD-3-Clause).
//   - ConversionContext::StartEffect and StartClip:
//     platform/graphics/compositing/paint_chunks_to_cc_layer.cc
//     (BSD-3-Clause).
//
// See NOTICE in this directory and finding F1 of docs/LICENSE-AUDIT.md.

package raster

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/pdf"
)

// paintWithEffects applies the effects of an element around paint. st is
// the resolved style of the element and ctm its transform to the device.
func (p *painter) paintWithEffects(n *dom.Node, st *svgStyle, ctm Affine, paint func(st *svgStyle, ctm Affine)) {
	opacity := float32(1)
	if v, ok := p.eng.RasterDeclared(n, "opacity"); ok {
		if f, ok := parseOpacity(v); ok {
			opacity = f
		}
	}
	blend := blendSrcOver
	if v, ok := p.eng.RasterDeclared(n, "mix-blend-mode"); ok {
		if strings.EqualFold(strings.TrimSpace(v), "multiply") {
			blend = blendMultiply
		}
	}
	clipID := ""
	if v, ok := p.eng.RasterDeclared(n, "clip-path"); ok {
		clipID = parseURLRef(v)
	}
	filter := p.resolveFilter(n)
	if p.vector != nil {
		if len(filter) > 0 && p.vector.shadow == nil {
			ops, source := filter, paint
			paint = func(st *svgStyle, ctm Affine) {
				p.paintVectorFiltered(n, st, ctm, ops, source)
			}
		}
		filter = nil
	}

	// isolate asks for the layer of an isolation group (scanEffects).
	isolate := p.isolated[n]
	if opacity == 1 && blend == blendSrcOver && clipID == "" && filter == nil && !isolate {
		paint(st, ctm)
		return
	}
	if opacity <= 0 && blend == blendSrcOver {
		return
	}
	needLayer := opacity < 1 || blend != blendSrcOver || isolate
	if needLayer {
		if blend == blendSrcOver {
			p.opacityLayer()
		} else {
			p.cv.pushLayer()
		}
	}
	dev := p.cv.dev()
	savedClip, savedAA := dev.clip, dev.aa
	visible := true
	if clipID != "" {
		visible = p.applyClipPath(n, clipID, ctm)
	}
	if visible {
		if filter != nil {
			p.paintFiltered(n, st, ctm, filter, paint)
		} else {
			paint(st, ctm)
		}
	}
	dev.clip, dev.aa = savedClip, savedAA
	if needLayer {
		p.cv.popLayer(opacity, blend)
	}
}

// opacityLayer starts the layer of an element with an opacity and without a
// blend mode: the SaveLayerAlphaOp of cc/paint.
//
// The layer keeps LCD text when no paint operation of the page prevents
// it. PaintOpBuffer::Playback of cc/paint/paint_op_buffer.cc decides that
// for the whole display list: the list has text, and it has no SaveLayerOp
// (has_effects_preventing_lcd_text_for_save_layer_alpha). Blink records a
// SaveLayerOp for an effect with a filter and for an effect with a blend
// mode (ConversionContext::StartEffect of paint_chunks_to_cc_layer.cc). So
// one drop shadow anywhere in a diagram turns the text of every opacity
// group of that diagram to grayscale masks.
func (p *painter) opacityLayer() {
	// PaintOpBuffer::Playback also asks that the canvas has a pixel
	// geometry: without one no layer keeps LCD text.
	if p.layerEffects || !p.cv.stack[0].lcd {
		p.cv.pushLayer()
		return
	}
	p.cv.pushLayerWithPrevious()
}

// scanEffects finds the elements of the subtree of n that Blink paints
// through a SaveLayerOp: an element with a filter and an element with a
// blend mode. It follows what paintElement paints: a use element paints its
// target.
//
// An element with a blend mode blends with what is below it inside its
// isolation group only. The group is the nearest container with a property
// that makes a group (SVGLayoutSupport::IsIsolationRequired and
// ComputedStyle::HasGroupingProperty of Blink). Without such a container
// the group is the page: Blink then paints the page into a layer, and no
// text of the page has LCD masks. group is the container of the group that
// n is in, or nil.
func (p *painter) scanEffects(n, group *dom.Node, depth int) {
	if depth > maxDepth || n.Type != dom.ElementNode {
		return
	}
	if n.NS != dom.NSSVG || notRendered[n.Tag] || p.eng.RasterDisplay(n) == "none" {
		return
	}
	if p.resolveFilter(n) != nil {
		p.layerEffects = true
	}
	blends := false
	if v, ok := p.eng.RasterDeclared(n, "mix-blend-mode"); ok && strings.EqualFold(strings.TrimSpace(v), "multiply") {
		blends = true
		p.layerEffects = true
		if group == nil {
			p.pageIsolated = true
		} else {
			if p.isolated == nil {
				p.isolated = map[*dom.Node]bool{}
			}
			p.isolated[group] = true
		}
	}
	switch n.Tag {
	case "g", "a", "svg", "switch", "use":
		if blends || p.hasGroupingProperty(n) {
			group = n
		}
	default:
		return
	}
	if n.Tag == "use" {
		href, ok := n.GetAttr("href")
		if !ok {
			href, ok = n.GetAttr("xlink:href")
		}
		if !ok || !strings.HasPrefix(href, "#") {
			return
		}
		t := p.ids[href[1:]]
		if t == nil || t == n {
			return
		}
		if t.Tag != "symbol" {
			p.scanEffects(t, group, depth+1)
			return
		}
		n = t
	}
	for c := n.FirstChild; c != nil; c = c.Next {
		p.scanEffects(c, group, depth+1)
	}
}

// hasGroupingProperty is ComputedStyle::HasGroupingProperty for the
// properties that the painter knows: an opacity, a filter, a clip path, a
// mask and isolation.
func (p *painter) hasGroupingProperty(n *dom.Node) bool {
	if v, ok := p.eng.RasterDeclared(n, "opacity"); ok {
		if f, ok := parseOpacity(v); ok && f != 1 {
			return true
		}
	}
	for _, prop := range []string{"filter", "clip-path", "mask"} {
		if v, ok := p.eng.RasterDeclared(n, prop); ok {
			if v = strings.TrimSpace(v); v != "" && !strings.EqualFold(v, "none") {
				return true
			}
		}
	}
	if v, ok := p.eng.RasterDeclared(n, "isolation"); ok && strings.EqualFold(strings.TrimSpace(v), "isolate") {
		return true
	}
	return false
}

// clipRectNonAA is SkCanvas::clipRect without anti-aliasing: the rectangle
// in user space intersects the clip of the device. A rectangle that stays
// a rectangle rounds to whole pixels. Another one becomes a region of the
// pixels whose centers are inside. It reports false when nothing is left.
func (p *painter) clipRectNonAA(dev *device, r Rect, ctm Affine) bool {
	if p.vector != nil {
		return p.clipRectAA(dev, r, ctm)
	}
	m := deviceMatrix(ctm)
	if m.rectStaysRect() {
		ir := m.mapRect(r).round()
		if !dev.clip.intersect(ir) {
			dev.clip = IRect{}
			return false
		}
		return true
	}
	var path, devPath Path
	path.lastMoveIdx = -1
	path.AddRect(r, 0)
	path.transformInto(&devPath, &m)
	bounds := devPath.bounds().roundOut()
	if !bounds.intersect(dev.clip) {
		dev.clip = IRect{}
		return false
	}
	clip := newAAClip(bounds)
	b := maskBuilderBlitter{clip: clip}
	p.cv.dr.nonAA.fillPath(&devPath, bounds, &b)
	p.intersectClip(dev, clip)
	return !dev.clip.isEmpty()
}

// intersectClip intersects the clip of a device with a coverage mask.
func (p *painter) intersectClip(dev *device, clip *aaClip) {
	if dev.aa != nil {
		clip = dev.aa.intersect(clip)
	}
	b := clip.bounds
	if !b.intersect(dev.clip) {
		dev.clip = IRect{}
		dev.aa = nil
		return
	}
	dev.clip = b
	dev.aa = clip
}

// applyClipPath sets the clip of the current device to the clipPath
// element with the given id. It reports false when the clip is empty or
// invalid: an invalid reference clips everything in Blink only when the
// element exists but is not a clipPath, so a missing element clips
// nothing.
//
// Blink turns a clipPath whose children are shapes into one path
// (LayoutSVGResourceClipper::AsPath) and then into clip operations
// (FragmentPaintPropertyTreeBuilder::UpdateClipPathClip and
// ConversionContext::StartClip):
//
//   - a path that is a rectangle is a ClipRectOp;
//   - a path that is a rounded rectangle or an oval is a ClipRRectOp;
//   - another path is a ClipRectOp with the bounding box of the clip
//     path, grown to whole user units, and then a ClipPathOp.
//
// All of them have anti-aliasing. A rectangle on whole pixels leaves a clip
// without coverage (clipRectAA), and text below it keeps LCD masks.
func (p *painter) applyClipPath(n *dom.Node, id string, ctm Affine) bool {
	cp := p.ids[id]
	if cp == nil || cp.NS != dom.NSSVG || (cp.Tag != "clipPath" && cp.Tag != "clippath") {
		return true
	}
	dev := p.cv.dev()
	units := strings.TrimSpace(cp.AttrOr("clipPathUnits", "userSpaceOnUse"))
	// local maps the content of the clipPath to the user space of the
	// clipped element.
	local := p.localTransform(cp)
	if units == "objectBoundingBox" {
		x, y, w, h, ok := p.eng.RasterBBox(n)
		if !ok {
			dev.clip = IRect{}
			return false
		}
		local = local.Translate(float64(x), float64(y)).Scale(float64(w), float64(h))
	}

	// The shapes of the clip, each with its transform to the user space
	// of the clipped element.
	type clipShape struct {
		node *dom.Node
		m    Affine
		rule fillType
	}
	var shapes []clipShape
	st := initialStyle()
	p.inheritedStyle(cp, &st)
	var visit func(c *dom.Node, parent *svgStyle, m Affine, depth int)
	visit = func(c *dom.Node, parent *svgStyle, m Affine, depth int) {
		if depth > maxDepth || c.Type != dom.ElementNode || c.NS != dom.NSSVG || p.eng.RasterDisplay(c) == "none" {
			return
		}
		cst := *parent
		p.resolveStyle(c, &cst)
		if !cst.visible {
			return
		}
		cm := m.Mul(p.localTransform(c))
		switch c.Tag {
		case "use":
			href, ok := c.GetAttr("href")
			if !ok {
				href, ok = c.GetAttr("xlink:href")
			}
			if ok && strings.HasPrefix(href, "#") {
				if t := p.ids[href[1:]]; t != nil && t != c {
					x := p.attrLength(c, "x", lengthWidth)
					y := p.attrLength(c, "y", lengthHeight)
					visit(t, &cst, cm.Translate(float64(x), float64(y)), depth+1)
				}
			}
		case "rect", "circle", "ellipse", "line", "polyline", "polygon", "path":
			shapes = append(shapes, clipShape{c, cm, cst.clipRule})
		case "text":
			// Text clips are rare in diagrams: the glyph boxes are not
			// rasterized here.
		}
	}
	for c := cp.FirstChild; c != nil; c = c.Next {
		visit(c, &st, local, 0)
	}

	// One shape that is a rectangle in the user space of the clipped
	// element: PathToRRect finds the rectangle.
	if len(shapes) == 1 {
		s := shapes[0]
		if geom := p.buildShape(s.node); geom.kind == geomRect && s.m.B == 0 && s.m.C == 0 {
			return p.clipRectAA(dev, geom.rect, ctm.Mul(s.m))
		}
	}

	// The paths of the shapes in device space, and the bounding box of the
	// clip path in the user space of the clipped element.
	var paths []Path
	var box Rect
	hasBox := false
	simple := len(shapes) == 1
	for _, s := range shapes {
		geom := p.buildShape(s.node)
		if geom.kind == geomEmpty {
			continue
		}
		src := &p.path
		switch geom.kind {
		case geomRect:
			src.Reset()
			src.AddRect(geom.rect, 0)
			simple = false
		case geomOval:
			src.Reset()
			src.AddOval(geom.rect, 1)
		default:
			// A rounded rectangle stays one under a scale and a
			// translation.
			if s.node.Tag != "rect" {
				simple = false
			}
		}
		if s.m.B != 0 || s.m.C != 0 {
			simple = false
		}
		// LayoutSVGResourceClipper::CalculateLocalClipBounds: the object
		// bounding box of each shape, mapped to the user space of the
		// clipped element.
		if x, y, w, h, ok := p.eng.RasterBBox(s.node); ok {
			lm := deviceMatrix(s.m)
			b := lm.mapRect(Rect{x, y, x + w, y + h})
			if !hasBox {
				box, hasBox = b, true
			} else {
				box = Rect{min(box.L, b.L), min(box.T, b.T), max(box.R, b.R), max(box.B, b.B)}
			}
		}
		dm := deviceMatrix(ctm.Mul(s.m))
		paths = append(paths, Path{})
		devPath := &paths[len(paths)-1]
		devPath.lastMoveIdx = -1
		src.transformInto(devPath, &dm)
		devPath.fill = s.rule
	}
	if len(paths) == 0 {
		dev.clip = IRect{}
		return false
	}
	if !simple && hasBox {
		// The ClipRectOp of the bounding box.
		enclosing := Rect{float32(math.Floor(float64(box.L))), float32(math.Floor(float64(box.T))), float32(math.Ceil(float64(box.R))), float32(math.Ceil(float64(box.B)))}
		if !p.clipRectAA(dev, enclosing, ctm) {
			return false
		}
	}
	if len(paths) == 1 {
		return p.clipPathAA(dev, &paths[0])
	}
	if p.vector != nil {
		shapes := make([]pdf.ClipShape, 0, len(paths))
		for i := range paths {
			shapes = append(shapes, pdf.ClipShape{Path: pdfPath(&paths[i]), EvenOdd: paths[i].fill == fillEvenOdd})
		}
		p.vector.doc.ClipUnion(shapes)
		return true
	}
	// Blink unites the shapes with path operations. Here the coverage of
	// the shapes accumulates in one mask.
	bounds := dev.clip
	clip := newAAClip(bounds)
	b := maskBuilderBlitter{clip: clip}
	for i := range paths {
		p.cv.dr.aaa.antiFillPathAA(&paths[i], bounds, &b)
	}
	p.intersectClip(dev, clip)
	return !dev.clip.isEmpty()
}
