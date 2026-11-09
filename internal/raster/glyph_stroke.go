// Ported from skia/src/core/SkScalerContext.cpp
// (SkScalerContext::internalGetPath with the stroke of a synthetic bold),
// SkStrokeRec.cpp (SkStrokeRec::applyToPath) and the end of
// SkStroke::strokePath for the style stroke and fill (Skia branch
// chrome/m156).

package raster

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	ot "github.com/go-text/typesetting/font/opentype"
)

// Skia makes a synthetic bold with a stroke: the path of a glyph becomes
// the path of the glyph together with the outline of a stroke around it
// (SkScalerContextRec::useStrokeForFakeBold). The font package computes the
// bounds of such a glyph for the measurement of text, and it cannot import
// this package, so the stroker is registered there.
func init() {
	font.RegisterGlyphStroker(strokeGlyphOutline)
}

// strokeGlyphOutline is the path that SkScalerContext::internalGetPath
// makes for a glyph with a stroke of the given width in the style stroke
// and fill: SkStroke::strokePath with a miter join, the miter limit 4 and
// the resolution scale 1. segments is the path of the glyph in px with y
// up, and so is the result. Each move starts a contour, and each contour is
// closed.
func strokeGlyphOutline(segments []ot.Segment, width float32) []ot.Segment {
	if len(segments) == 0 || !(width > 0) {
		return segments
	}
	// The path of Skia has y down.
	src := NewPath()
	for _, seg := range segments {
		a := seg.Args
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			if !src.isEmpty() {
				src.Close()
			}
			src.MoveTo(a[0].X, -a[0].Y)
		case ot.SegmentOpLineTo:
			src.LineTo(a[0].X, -a[0].Y)
		case ot.SegmentOpQuadTo:
			src.QuadTo(a[0].X, -a[0].Y, a[1].X, -a[1].Y)
		case ot.SegmentOpCubeTo:
			src.CubicTo(a[0].X, -a[0].Y, a[1].X, -a[1].Y, a[2].X, -a[2].Y)
		}
	}
	src.Close()

	var dst Path
	var scratch strokeScratch
	sp := strokeParams{width: width, miterLimit: 4, cap: capButt, join: joinMiter, resScale: 1}
	strokePath(src, &dst, &sp, &scratch)
	// The style stroke and fill: the path of the glyph is part of the
	// result. Skia adds it in the direction of the stroke
	// (SkPathPriv::ComputeFirstDirection). Here the direction is the sign of
	// the area of the first contour, which is the same for a simple
	// contour.
	if firstContourIsCCW(src) {
		reversed := NewPath()
		reverseContours(reversed, src)
		dst.addPath(reversed)
	} else {
		dst.addPath(src)
	}

	out := make([]ot.Segment, 0, len(dst.verbs))
	pi := 0
	for _, v := range dst.verbs {
		switch v {
		case verbMove:
			p := dst.pts[pi]
			pi++
			out = append(out, ot.Segment{Op: ot.SegmentOpMoveTo, Args: [3]ot.SegmentPoint{{X: p.X, Y: -p.Y}}})
		case verbLine:
			p := dst.pts[pi]
			pi++
			out = append(out, ot.Segment{Op: ot.SegmentOpLineTo, Args: [3]ot.SegmentPoint{{X: p.X, Y: -p.Y}}})
		case verbQuad, verbConic:
			// The stroker makes no conic for a miter join. A conic would
			// be drawn as the quadratic curve with its points.
			c, p := dst.pts[pi], dst.pts[pi+1]
			pi += 2
			out = append(out, ot.Segment{Op: ot.SegmentOpQuadTo, Args: [3]ot.SegmentPoint{{X: c.X, Y: -c.Y}, {X: p.X, Y: -p.Y}}})
		case verbCubic:
			c1, c2, p := dst.pts[pi], dst.pts[pi+1], dst.pts[pi+2]
			pi += 3
			out = append(out, ot.Segment{Op: ot.SegmentOpCubeTo, Args: [3]ot.SegmentPoint{{X: c1.X, Y: -c1.Y}, {X: c2.X, Y: -c2.Y}, {X: p.X, Y: -p.Y}}})
		}
	}
	return out
}

// firstContourIsCCW reports whether the points of the first contour of a
// path go counter-clockwise on a surface with y down.
func firstContourIsCCW(p *Path) bool {
	var area float64
	var first, prev Point
	pi := 0
	started := false
	add := func(q Point) {
		area += float64(float64(prev.X)*float64(q.Y)) - float64(float64(q.X)*float64(prev.Y))
		prev = q
	}
loop:
	for _, v := range p.verbs {
		switch v {
		case verbMove:
			if started {
				break loop
			}
			started = true
			first, prev = p.pts[pi], p.pts[pi]
			pi++
		case verbLine:
			add(p.pts[pi])
			pi++
		case verbQuad, verbConic:
			add(p.pts[pi])
			add(p.pts[pi+1])
			pi += 2
		case verbCubic:
			add(p.pts[pi])
			add(p.pts[pi+1])
			add(p.pts[pi+2])
			pi += 3
		case verbClose:
			break loop
		}
	}
	add(first)
	// With y down a positive area is clockwise.
	return area < 0
}

// reverseContours appends every contour of src to dst with its points in
// the opposite order (SkPathBuilder::privateReverseAddPath).
func reverseContours(dst, src *Path) {
	type contour struct{ verbStart, verbEnd, ptStart, ptEnd int }
	var contours []contour
	pi := 0
	cur := contour{verbStart: -1}
	for vi, v := range src.verbs {
		switch v {
		case verbMove:
			if cur.verbStart >= 0 {
				cur.verbEnd, cur.ptEnd = vi, pi
				contours = append(contours, cur)
			}
			cur = contour{verbStart: vi, ptStart: pi}
			pi++
		case verbLine:
			pi++
		case verbQuad, verbConic:
			pi += 2
		case verbCubic:
			pi += 3
		}
	}
	if cur.verbStart >= 0 {
		cur.verbEnd, cur.ptEnd = len(src.verbs), pi
		contours = append(contours, cur)
	}
	for i := len(contours) - 1; i >= 0; i-- {
		c := contours[i]
		pts := src.pts[c.ptStart:c.ptEnd]
		last := pts[len(pts)-1]
		dst.MoveTo(last.X, last.Y)
		pj := len(pts) - 1
		closed := false
		for vi := c.verbEnd - 1; vi > c.verbStart; vi-- {
			switch src.verbs[vi] {
			case verbLine:
				pj--
				dst.LineTo(pts[pj].X, pts[pj].Y)
			case verbQuad, verbConic:
				pj -= 2
				dst.QuadTo(pts[pj+1].X, pts[pj+1].Y, pts[pj].X, pts[pj].Y)
			case verbCubic:
				pj -= 3
				dst.CubicTo(pts[pj+2].X, pts[pj+2].Y, pts[pj+1].X, pts[pj+1].Y, pts[pj].X, pts[pj].Y)
			case verbClose:
				closed = true
			}
		}
		if closed {
			dst.Close()
		}
	}
}
