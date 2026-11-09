// Ported from skrifa 0.46.2 (src/outline/path.rs: contour_to_path with the
// path style of FreeType, src/outline/autohint/outline.rs: Outline::to_path,
// src/outline/glyf/outline.rs: ScaledOutline::to_path) and from Skia at
// revision aa789d9b (src/ports/fontations/src/verbs_points_pen.rs:
// VerbsPointsPen; BSD 3-clause license, LICENSE-Skia.txt).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported the conversion of an outline to a path
// from FreeType 2.13.2 (src/base/ftoutln.c: FT_Outline_Decompose), which has
// the FreeType License (LICENSE-FreeType.txt).

package font

import (
	ot "github.com/go-text/typesetting/font/opentype"
)

// Skia gets the outline of a glyph as a path: skrifa walks the points of
// each contour and calls a pen of Skia for each segment. The rasterizer of
// Skia fills that path. ahPathBounds in autohint_glyf.go has the bounds of
// the points of the same path without making the path.

// ahPathPoint is ContourPoint: a point of a contour in 26.6 px with y up,
// and the flags of the point.
type ahPathPoint struct {
	x, y  int32
	flags uint8
}

func (p ahPathPoint) isOnCurve() bool { return p.flags&ahPointOnCurve != 0 }

func (p ahPathPoint) isOffCurveQuad() bool { return p.flags&ahPointCurveMask == 0 }

func (p ahPathPoint) isOffCurveCubic() bool { return p.flags&ahPointOffCurveCubic != 0 }

// midpoint is ContourPoint::midpoint: the point in the middle of two
// points, with the flags of the other point. FreeType divides the 26.6
// numbers as integers, and the sum wraps.
func (p ahPathPoint) midpoint(other ahPathPoint) ahPathPoint {
	return ahPathPoint{x: (p.x + other.x) / 2, y: (p.y + other.y) / 2, flags: other.flags}
}

// f32 is ContourPoint::point_f32: the point in px (F26Dot6::to_f32).
func (p ahPathPoint) f32() (x, y float32) {
	const scaleFactor = 1.0 / 64.0
	return float32(p.x) * scaleFactor, float32(p.y) * scaleFactor
}

// ahPathPen is VerbsPointsPen of Skia: it collects the segments of the path
// of a glyph. Skia changes the sign of y for its path. Here y stays up, as
// in the outlines of go-text.
//
// The pen starts a contour at the first segment that moves, and it drops a
// segment whose points are all at the current point. So a contour whose
// points are all at one position adds nothing to the path.
//
// Skia closes each contour. The segments here have no close: a move ends
// the contour before it, and the last contour ends with the outline.
type ahPathPen struct {
	segments []ot.Segment
	started  bool
	// x and y are the current point.
	x, y float32
}

// moveTo is move_to: the next contour starts at the point. The move goes
// into the path with the first segment of the contour.
func (p *ahPathPen) moveTo(x, y float32) {
	p.started = false
	p.x, p.y = x, y
}

// goingTo is going_to.
func (p *ahPathPen) goingTo(x, y float32) {
	if !p.started {
		p.started = true
		p.segments = append(p.segments, ot.Segment{Op: ot.SegmentOpMoveTo, Args: [3]ot.SegmentPoint{{X: p.x, Y: p.y}}})
	}
	p.x, p.y = x, y
}

// currentIsNot is current_is_not.
func (p *ahPathPen) currentIsNot(x, y float32) bool { return p.x != x || p.y != y }

// lineTo is line_to.
func (p *ahPathPen) lineTo(x, y float32) {
	if p.currentIsNot(x, y) {
		p.goingTo(x, y)
		p.segments = append(p.segments, ot.Segment{Op: ot.SegmentOpLineTo, Args: [3]ot.SegmentPoint{{X: x, Y: y}}})
	}
}

// quadTo is quad_to.
func (p *ahPathPen) quadTo(cx0, cy0, x, y float32) {
	if p.currentIsNot(cx0, cy0) || p.currentIsNot(x, y) {
		p.goingTo(x, y)
		p.segments = append(p.segments, ot.Segment{Op: ot.SegmentOpQuadTo, Args: [3]ot.SegmentPoint{{X: cx0, Y: cy0}, {X: x, Y: y}}})
	}
}

// curveTo is curve_to.
func (p *ahPathPen) curveTo(cx0, cy0, cx1, cy1, x, y float32) {
	if p.currentIsNot(cx0, cy0) || p.currentIsNot(cx1, cy1) || p.currentIsNot(x, y) {
		p.goingTo(x, y)
		p.segments = append(p.segments, ot.Segment{Op: ot.SegmentOpCubeTo, Args: [3]ot.SegmentPoint{{X: cx0, Y: cy0}, {X: cx1, Y: cy1}, {X: x, Y: y}}})
	}
}

// The kinds of ahPendingState.
const (
	// ahPendingEmpty: no pending points.
	ahPendingEmpty uint8 = iota
	// ahPendingQuad: one pending off-curve point of a quadratic curve.
	ahPendingQuad
	// ahPendingCubic: one pending off-curve point of a cubic curve.
	ahPendingCubic
	// ahTwoPendingCubics: two pending off-curve points of a cubic curve.
	ahTwoPendingCubics
)

// ahPendingState is PendingState: the off-curve points that wait for the end
// of their curve.
type ahPendingState struct {
	kind   uint8
	p0, p1 ahPathPoint
}

// emit is PendingState::emit: the next point of a contour. The result is
// false for a point that cannot follow the pending points.
func (s *ahPendingState) emit(point ahPathPoint, pen *ahPathPen) bool {
	switch s.kind {
	case ahPendingEmpty:
		switch {
		case point.isOffCurveQuad():
			*s = ahPendingState{kind: ahPendingQuad, p0: point}
		case point.isOffCurveCubic():
			*s = ahPendingState{kind: ahPendingCubic, p0: point}
		default:
			x, y := point.f32()
			pen.lineTo(x, y)
		}
	case ahPendingQuad:
		switch {
		case point.isOffCurveQuad():
			// Two off-curve points in a row: the curve ends in the middle
			// of them.
			cx, cy := s.p0.f32()
			x, y := s.p0.midpoint(point).f32()
			pen.quadTo(cx, cy, x, y)
			*s = ahPendingState{kind: ahPendingQuad, p0: point}
		case point.isOffCurveCubic():
			return false
		default:
			cx, cy := s.p0.f32()
			x, y := point.f32()
			pen.quadTo(cx, cy, x, y)
			*s = ahPendingState{}
		}
	case ahPendingCubic:
		if !point.isOffCurveCubic() {
			return false
		}
		*s = ahPendingState{kind: ahTwoPendingCubics, p0: s.p0, p1: point}
	case ahTwoPendingCubics:
		switch {
		case point.isOffCurveQuad():
			return false
		case point.isOffCurveCubic():
			cx0, cy0 := s.p0.f32()
			cx1, cy1 := s.p1.f32()
			x, y := s.p1.midpoint(point).f32()
			pen.curveTo(cx0, cy0, cx1, cy1, x, y)
			*s = ahPendingState{kind: ahPendingCubic, p0: point}
		default:
			cx0, cy0 := s.p0.f32()
			cx1, cy1 := s.p1.f32()
			x, y := point.f32()
			pen.curveTo(cx0, cy0, cx1, cy1, x, y)
			*s = ahPendingState{}
		}
	}
	return true
}

// ahContourToPath is contour_to_path with the path style of FreeType
// (contour_to_path_freetype): it sends the segments of a contour of n points
// to the pen. at returns a point of the contour. The result is false for a
// contour that is not valid.
//
// A contour that starts with an off-curve point starts at its last point,
// when that point is on the curve, and else in the middle of its last and
// its first point.
func ahContourToPath(n int, at func(i int) ahPathPoint, pen *ahPathPen) bool {
	if n == 0 {
		return true
	}
	first, last := at(0), at(n-1)
	// An off-curve point of a cubic curve cannot be the first point.
	if first.isOffCurveCubic() {
		return false
	}
	var start ahPathPoint
	i, end := 0, n
	if first.isOffCurveQuad() {
		if last.isOnCurve() {
			// The contour starts at its last point, so the loop does not
			// have that point.
			start, end = last, n-1
		} else {
			start = last.midpoint(first)
		}
	} else {
		start, i = first, 1
	}
	pen.moveTo(start.f32())
	var state ahPendingState
	for ; i < end; i++ {
		if !state.emit(at(i), pen) {
			return false
		}
	}
	// PendingState::finish: a curve that is not complete ends at the start
	// point. The pen of Skia closes the contour.
	if state.kind != ahPendingEmpty {
		start.flags = ahPointOnCurve
		if !state.emit(start, pen) {
			return false
		}
	}
	return true
}

// path is Outline::to_path with the pen of Skia: the path of the hinted
// outline. The result is false for an outline that has no valid path.
func (o *ahOutline) path(pen *ahPathPen) bool {
	for _, contour := range o.contours {
		first, last := contour.first(), contour.last()
		if last >= len(o.points) || first > last {
			continue
		}
		points := o.points[first : last+1]
		ok := ahContourToPath(len(points), func(i int) ahPathPoint {
			p := &points[i]
			return ahPathPoint{x: p.x, y: p.y, flags: p.flags}
		}, pen)
		if !ok {
			return false
		}
	}
	return true
}

// unhintedOutline is FreeTypeScaler::unhinted and FreeTypeScaler::scale for
// a font size in px: the outline of a glyph in 26.6 px without hinting.
func (f *ahFont) unhintedOutline(o ahGlyfOutline, size float32) (*ahGlyfScaler, bool) {
	// Scale26Dot6::new.
	scale, isScaled := int32(0x10000), false
	if f.unitsPerEm > 0 {
		scale, isScaled = ahFixedDiv(ahF32ToI32(size*64.0), int32(f.unitsPerEm)), true
	}
	return f.scaleOutline(o, scale, isScaled)
}

// unhintedPath is OutlineGlyph::draw_unhinted with the pen of Skia: the path
// of the outline of a glyph at a font size without hinting. The result is
// false for a glyph that has no valid path.
func (f *ahFont) unhintedPath(o ahGlyfOutline, size float32, pen *ahPathPen) bool {
	s, ok := f.unhintedOutline(o, size)
	if !ok {
		return false
	}
	points, flags := s.scaled[:s.pointCount], s.flags[:s.pointCount]
	// to_path: a contour that ends before it starts, or after the last
	// point, is an error.
	startIx := 0
	for _, c := range s.contours[:s.contourCount] {
		endIx := int(c)
		if endIx < startIx || endIx >= len(points) {
			return false
		}
		contour, contourFlags := points[startIx:endIx+1], flags[startIx:endIx+1]
		ok := ahContourToPath(len(contour), func(i int) ahPathPoint {
			return ahPathPoint{x: contour[i].x, y: contour[i].y, flags: contourFlags[i]}
		}, pen)
		if !ok {
			return false
		}
		startIx = endIx + 1
	}
	return true
}
