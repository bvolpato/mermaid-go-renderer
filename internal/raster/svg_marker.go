// Markers: ported from blink/renderer/core/layout/svg/svg_marker_data.cc,
// layout_svg_resource_marker.cc and the marker part of
// core/paint/svg_shape_painter.cc (Chromium 156).
//
// The license of each Blink file is the license in its header at Chromium
// 156.0.8078.4. The paths are in third_party/blink/renderer:
//
//   - core/layout/svg/svg_marker_data.cc and svg_marker_data.h:
//     LGPL-2.0-or-later.
//   - core/layout/svg/layout_svg_resource_marker.cc: LGPL-2.0-or-later.
//   - core/paint/svg_shape_painter.cc: BSD-3-Clause.
//   - SVGLayoutSupport::IsOverflowHidden, which a comment names:
//     core/layout/svg/svg_layout_support.cc (LGPL-2.0-or-later).
//   - AffineTransform::Rotate, which a comment names:
//     platform/transforms/affine_transform.cc (BSD-2-Clause, with the
//     copyright of Apple and Google in the file).
//
// See NOTICE in this directory and finding F1 of docs/LICENSE-AUDIT.md.

package raster

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

const (
	markerStart = iota
	markerMid
	markerEnd
)

type markerPosition struct {
	kind   uint8
	origin Point
	angle  float32
}

// markerBuilder is SVGMarkerDataBuilder.
type markerBuilder struct {
	positions          []markerPosition
	lastMovetoIndex    int
	lastElementType    pathVerb
	origin             Point
	subpathStart       Point
	inSlope, outSlope  Point
	lastMovetoOutSlope Point
}

func bisectingAngle(inAngle, outAngle float64) float64 {
	diff := inAngle - outAngle
	if diff > 180 || diff <= -180 {
		inAngle += 360
	}
	return (inAngle + outAngle) / 2
}

func slopeAngleDeg(v Point) float64 {
	// gfx::Vector2dF::SlopeAngleRadians is atan2f.
	return float64(float64(float32(math.Atan2(float64(v.Y), float64(v.X)))) * (180 / math.Pi))
}

const (
	angleBisecting = iota
	angleInbound
	angleOutbound
)

func (b *markerBuilder) currentAngle(kind int) float64 {
	in := slopeAngleDeg(b.inSlope)
	out := slopeAngleDeg(b.outSlope)
	switch kind {
	case angleOutbound:
		return out
	case angleInbound:
		return in
	}
	return bisectingAngle(in, out)
}

func (b *markerBuilder) updateAngle(endsSubpath bool) {
	if b.lastElementType == verbClose {
		b.outSlope = b.lastMovetoOutSlope
	}
	kind := angleBisecting
	switch {
	case b.lastElementType == verbClose:
		kind = angleBisecting
	case endsSubpath:
		kind = angleInbound
	case b.lastElementType == verbMove:
		kind = angleOutbound
	}
	angle := float32(b.currentAngle(kind))
	if b.lastElementType == verbClose {
		b.positions[b.lastMovetoIndex].angle = angle
	}
	b.positions[len(b.positions)-1].angle = angle
}

func quadTangents(start, control, end Point) (startT, endT Point) {
	startT = control.sub(start)
	endT = end.sub(control)
	if startT.isZero() {
		startT = endT
	} else if endT.isZero() {
		endT = startT
	}
	return
}

// element is SVGMarkerDataBuilder::UpdateFromPathElement.
func (b *markerBuilder) element(verb pathVerb, pts []Point) {
	var position, startT, endT Point
	switch verb {
	case verbCubic:
		position = pts[2]
		startT = pts[0].sub(b.origin)
		endT = pts[2].sub(pts[1])
		if startT.isZero() {
			startT, endT = quadTangents(pts[0], pts[1], pts[2])
		} else if endT.isZero() {
			startT, endT = quadTangents(b.origin, pts[0], pts[1])
		}
	case verbQuad:
		position = pts[1]
		startT, endT = quadTangents(b.origin, pts[0], pts[1])
	case verbMove, verbLine:
		position = pts[0]
		startT = position.sub(b.origin)
		endT = startT
	case verbClose:
		tangent := b.subpathStart.sub(b.origin)
		if b.lastElementType != verbMove && tangent.isZero() {
			tangent = b.lastMovetoOutSlope
		}
		position = b.subpathStart
		startT, endT = tangent, tangent
	}
	b.outSlope = startT
	if b.lastElementType == verbMove {
		b.lastMovetoOutSlope = b.outSlope
	}
	startsNewSubpath := verb == verbMove
	if len(b.positions) > 0 {
		b.updateAngle(startsNewSubpath)
	}
	b.inSlope = endT
	b.origin = position
	if startsNewSubpath {
		b.subpathStart = pts[0]
		b.lastMovetoIndex = len(b.positions)
	}
	b.lastElementType = verb
	kind := uint8(markerMid)
	if len(b.positions) == 0 {
		kind = markerStart
	}
	b.positions = append(b.positions, markerPosition{kind: kind, origin: b.origin})
}

func (b *markerBuilder) flush() {
	if len(b.positions) == 0 {
		return
	}
	b.updateAngle(true)
	b.positions[len(b.positions)-1].kind = markerEnd
}

// buildMarkerPositions computes the marker positions of a path.
func buildMarkerPositions(path *Path) []markerPosition {
	b := markerBuilder{lastElementType: verbMove}
	pi := 0
	for _, v := range path.verbs {
		switch v {
		case verbMove, verbLine:
			b.element(v, path.pts[pi:pi+1])
			pi++
		case verbQuad, verbConic:
			b.element(verbQuad, path.pts[pi:pi+2])
			pi += 2
		case verbCubic:
			b.element(v, path.pts[pi:pi+3])
			pi += 3
		case verbClose:
			b.element(v, nil)
		}
	}
	b.flush()
	return b.positions
}

// rotateDeg is AffineTransform::Rotate.
func (m Affine) rotateDeg(deg float64) Affine {
	rad := float64(deg * (math.Pi / 180))
	s, c := math.Sin(rad), math.Cos(rad)
	return m.Mul(Affine{A: c, B: s, C: -s, D: c})
}

// paintMarkers is SVGShapePainter::PaintMarkers. Markers apply to path,
// line, polyline and polygon elements.
func (p *painter) paintMarkers(n *dom.Node, st *svgStyle, ctm Affine, geom shapeGeometry) {
	switch n.Tag {
	case "path", "line", "polyline", "polygon":
	default:
		return
	}
	if geom.kind != geomPath {
		return
	}
	positions := buildMarkerPositions(&p.path)
	if len(positions) == 0 {
		return
	}
	markers := [3]*dom.Node{p.markerElement(st.markerStart), p.markerElement(st.markerMid), p.markerElement(st.markerEnd)}
	if markers[0] == nil && markers[1] == nil && markers[2] == nil {
		return
	}
	saved := p.context
	p.context = &contextPaints{fill: st.fill, stroke: st.stroke, fillOpacity: st.fillOpacity, strokeOp: st.strokeOpacity}
	defer func() { p.context = saved }()
	for _, pos := range positions {
		if marker := markers[pos.kind]; marker != nil {
			p.paintMarker(marker, pos, st.strokeWidth, ctm)
		}
	}
}

func (p *painter) markerElement(id string) *dom.Node {
	if id == "" {
		return nil
	}
	n := p.ids[id]
	if n == nil || n.NS != dom.NSSVG || n.Tag != "marker" {
		return nil
	}
	return n
}

// paintMarker is SVGShapePainter::PaintMarker.
func (p *painter) paintMarker(marker *dom.Node, pos markerPosition, strokeWidth float32, ctm Affine) {
	if p.depth > maxDepth {
		return
	}
	// An empty viewBox disables rendering.
	vb := browser.RasterParseNumbers(marker.AttrOr("viewBox", ""))
	hasViewBox := len(vb) == 4
	if hasViewBox && !(vb[2] > 0 && vb[3] > 0) {
		return
	}
	markerW, markerH := float32(3), float32(3)
	if marker.HasAttr("markerWidth") {
		markerW = p.eng.RasterAttrLength(marker, "markerWidth")
	}
	if marker.HasAttr("markerHeight") {
		markerH = p.eng.RasterAttrLength(marker, "markerHeight")
	}
	if !(markerW > 0 && markerH > 0) {
		return
	}
	// LocalToSVGParentTransform: the viewBox to the marker viewport.
	local := IdentityAffine
	if hasViewBox {
		var box [4]float32
		copy(box[:], vb)
		local = viewBoxTransform(box, strings.TrimSpace(marker.AttrOr("preserveAspectRatio", "")), markerW, markerH)
	}
	// MarkerTransformation.
	markerScale := float32(1)
	if !strings.EqualFold(strings.TrimSpace(marker.AttrOr("markerUnits", "strokeWidth")), "userSpaceOnUse") {
		markerScale = strokeWidth
	}
	angle := float64(pos.angle)
	orient := strings.TrimSpace(marker.AttrOr("orient", "0"))
	switch orient {
	case "auto":
	case "auto-start-reverse":
		if pos.kind == markerStart {
			angle += 180
		}
	default:
		angle = 0
		if v := browser.RasterParseNumbers(strings.TrimSuffix(orient, "deg")); len(v) == 1 {
			angle = float64(v[0])
		}
	}
	refX := p.eng.RasterAttrLength(marker, "refX")
	refY := p.eng.RasterAttrLength(marker, "refY")
	mx, my := local.MapPoint(float64(refX), float64(refY))
	// gfx::PointF holds the mapped reference point.
	transform := IdentityAffine.Translate(float64(pos.origin.X), float64(pos.origin.Y)).rotateDeg(angle).
		Scale(float64(markerScale), float64(markerScale)).Translate(-float64(float32(mx)), -float64(float32(my)))

	mctm := ctm.Mul(transform)
	dev := p.cv.dev()
	savedClip, savedAA := dev.clip, dev.aa
	defer func() { dev.clip, dev.aa = savedClip, savedAA }()

	st := initialStyle()
	p.inheritedStyle(marker, &st)
	if p.overflowHidden(marker) {
		// canvas->clipRect(viewport): a clip without anti-aliasing.
		if !p.clipRectNonAA(dev, Rect{0, 0, markerW, markerH}, mctm) {
			return
		}
	}
	savedW, savedH := p.viewportW, p.viewportH
	if hasViewBox {
		p.viewportW, p.viewportH = vb[2], vb[3]
	} else {
		p.viewportW, p.viewportH = markerW, markerH
	}
	p.depth++
	p.paintWithEffects(marker, &st, mctm.Mul(local), func(st *svgStyle, ctm Affine) {
		p.paintChildren(marker, st, ctm)
	})
	p.depth--
	p.viewportW, p.viewportH = savedW, savedH
}

// overflowHidden is SVGLayoutSupport::IsOverflowHidden. The default for a
// marker, an inner svg, a symbol and a pattern is hidden (the user agent
// style sheet).
func (p *painter) overflowHidden(n *dom.Node) bool {
	v, ok := p.eng.RasterDeclared(n, "overflow")
	if !ok {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(strings.Fields(v + " x")[0])) {
	case "hidden", "clip", "scroll":
		return true
	case "visible", "auto":
		return false
	}
	return true
}

// inheritedStyle computes the inherited properties of an element from the
// root of the document: a marker inherits from its ancestors, not from the
// element that references it.
func (p *painter) inheritedStyle(n *dom.Node, st *svgStyle) {
	var chain []*dom.Node
	for a := n; a != nil && a.Type == dom.ElementNode; a = a.Parent {
		chain = append(chain, a)
	}
	for i := len(chain) - 1; i >= 0; i-- {
		p.resolveStyle(chain[i], st)
	}
}
