// Scan conversion without anti-aliasing: shape-rendering: crispEdges and
// clips without anti-aliasing. The line edges are ported from
// skia/src/core/SkEdge.cpp (SkEdge::setLine) and the scan line walk from
// skia/src/core/SkScan_Path.cpp (walk_edges) of the Skia branch chrome/m156.
// Curves become lines before they become edges: Skia steps along a curve in
// fixed point, which gives the same pixels except for centers that lie
// within about 1/64 of a pixel of the curve.

package raster

import (
	"math"
	"sort"
)

// bwEdge is SkEdge for a line: fX is the x at the center of the current
// scan line in 16.16 fixed point.
type bwEdge struct {
	x, dx         fixed
	firstY, lastY int
	winding       int8
}

// nonAAState holds the scratch memory of the scan converter.
type nonAAState struct {
	edges  []bwEdge
	active []int
}

// addLine is SkEdge::setLine.
func (n *nonAAState) addLine(p0, p1 Point) {
	x0, y0 := scalarToFDot6(p0.X), scalarToFDot6(p0.Y)
	x1, y1 := scalarToFDot6(p1.X), scalarToFDot6(p1.Y)
	winding := int8(1)
	if y0 > y1 {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
		winding = -1
	}
	top, bot := fdot6Round(y0), fdot6Round(y1)
	if top == bot {
		return
	}
	slope := fdot6Div(x1-x0, y1-y0)
	// SkEdge_Compute_DY: the distance from y0 to the center of the first
	// scan line.
	dy := fdot6((int32(top)<<6)+32) - y0
	n.edges = append(n.edges, bwEdge{
		x:       fdot6ToFixed(x0 + fdot6(fixedMul(slope, fixed(dy)))),
		dx:      slope,
		firstY:  top,
		lastY:   bot - 1,
		winding: winding,
	})
}

// flatten appends the points of a curve as lines.
func (n *nonAAState) quad(p0, p1, p2 Point) {
	ddx := p0.X - float32(2*p1.X) + p2.X
	ddy := p0.Y - float32(2*p1.Y) + p2.Y
	dd := float32(math.Sqrt(float64(float32(ddx*ddx) + float32(ddy*ddy))))
	// A distance of 1/64 of a pixel between the curve and its lines.
	k := int(math.Ceil(math.Sqrt(float64(dd) * 16)))
	k = min(max(k, 1), 512)
	pts := [3]Point{p0, p1, p2}
	prev := p0
	for i := 1; i <= k; i++ {
		q := p2
		if i < k {
			q = evalQuadAt(pts[:], float32(i)/float32(k))
		}
		n.addLine(prev, q)
		prev = q
	}
}

func (n *nonAAState) cubic(p0, p1, p2, p3 Point) {
	ax := p0.X - float32(2*p1.X) + p2.X
	ay := p0.Y - float32(2*p1.Y) + p2.Y
	bx := p1.X - float32(2*p2.X) + p3.X
	by := p1.Y - float32(2*p2.Y) + p3.Y
	dd := float32(math.Sqrt(float64(max(float32(ax*ax)+float32(ay*ay), float32(bx*bx)+float32(by*by)))))
	k := int(math.Ceil(math.Sqrt(float64(dd) * 48)))
	k = min(max(k, 1), 512)
	pts := [4]Point{p0, p1, p2, p3}
	prev := p0
	for i := 1; i <= k; i++ {
		q := p3
		if i < k {
			q = evalCubicPosAt(pts[:], float32(i)/float32(k))
		}
		n.addLine(prev, q)
		prev = q
	}
}

// fillPath is SkScan::FillPath: it fills the pixels whose centers are
// inside the path.
func (n *nonAAState) fillPath(p *Path, clip IRect, b blitter) {
	n.edges = n.edges[:0]
	var first, cur Point
	open := false
	pi, ci := 0, 0
	closeContour := func() {
		if open && cur != first {
			n.addLine(cur, first)
		}
		open = false
	}
	for _, v := range p.verbs {
		switch v {
		case verbMove:
			closeContour()
			first = p.pts[pi]
			cur = first
			open = true
			pi++
		case verbLine:
			n.addLine(cur, p.pts[pi])
			cur = p.pts[pi]
			pi++
		case verbQuad:
			n.quad(cur, p.pts[pi], p.pts[pi+1])
			cur = p.pts[pi+1]
			pi += 2
		case verbConic:
			var buf [1 + 2*(1<<5)]Point
			quads := conicToQuads(cur, p.pts[pi], p.pts[pi+1], p.conics[ci], 0.25, buf[:])
			for k := 0; k+2 < len(quads); k += 2 {
				n.quad(quads[k], quads[k+1], quads[k+2])
			}
			cur = p.pts[pi+1]
			pi += 2
			ci++
		case verbCubic:
			n.cubic(cur, p.pts[pi], p.pts[pi+1], p.pts[pi+2])
			cur = p.pts[pi+2]
			pi += 3
		case verbClose:
			closeContour()
			cur = first
		}
	}
	closeContour()
	if len(n.edges) == 0 {
		return
	}
	edges := n.edges
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].firstY != edges[j].firstY {
			return edges[i].firstY < edges[j].firstY
		}
		return edges[i].x < edges[j].x
	})
	mask := int8(-1)
	if p.fill == fillEvenOdd {
		mask = 1
	}
	active := n.active[:0]
	next := 0
	y := edges[0].firstY
	for next < len(edges) || len(active) > 0 {
		if len(active) == 0 && edges[next].firstY > y {
			y = edges[next].firstY
		}
		if y >= clip.B {
			break
		}
		for next < len(edges) && edges[next].firstY <= y {
			active = append(active, next)
			next++
		}
		// Keep the active edges in the order of x.
		for i := 1; i < len(active); i++ {
			for j := i; j > 0 && edges[active[j]].x < edges[active[j-1]].x; j-- {
				active[j], active[j-1] = active[j-1], active[j]
			}
		}
		if y >= clip.T {
			var w int8
			left := 0
			for _, ei := range active {
				x := fixedRoundToInt(edges[ei].x)
				if w&mask == 0 {
					left = x
				}
				w += edges[ei].winding
				if w&mask == 0 {
					l, r := max(left, clip.L), min(x, clip.R)
					if r > l {
						b.blitH(l, y, r-l)
					}
				}
			}
		}
		// Step to the next scan line.
		out := active[:0]
		for _, ei := range active {
			e := &edges[ei]
			if e.lastY == y {
				continue
			}
			e.x += e.dx
			out = append(out, ei)
		}
		active = out
		y++
	}
	n.active = active[:0]
}

// frameRect is SkScan::FrameRect: the stroke of a rectangle with miter
// joins, as four rectangles without anti-aliasing.
func frameRect(r Rect, strokeSize Point, clip IRect, b blitter) {
	if strokeSize.X < 0 || strokeSize.Y < 0 {
		return
	}
	dx, dy := strokeSize.X, strokeSize.Y
	rx, ry := float32(dx*0.5), float32(dy*0.5)
	outer := Rect{r.L - rx, r.T - ry, r.R + rx, r.B + ry}
	if r.width() <= dx || r.height() <= dy {
		fillRect(outer, clip, b)
		return
	}
	fillRect(Rect{outer.L, outer.T, outer.R, outer.T + dy}, clip, b)
	fillRect(Rect{outer.L, outer.B - dy, outer.R, outer.B}, clip, b)
	fillRect(Rect{outer.L, outer.T + dy, outer.L + dx, outer.B - dy}, clip, b)
	fillRect(Rect{outer.R - dx, outer.T + dy, outer.R, outer.B - dy}, clip, b)
}

// hairPath and hairRect draw hairlines without anti-aliasing. SVG has no
// stroke of width zero, and Skia treats a thin stroke as a hairline only
// with anti-aliasing, so these are not reached. They fall back to the
// anti-aliased hairlines.
func hairPath(p *Path, clip IRect, b blitter, cap strokeCap, scratch *[]Point) {
	antiHairPath(p, clip, b, cap, scratch)
}

func hairRect(r Rect, clip IRect, b blitter) {
	pts := [5]Point{{r.L, r.T}, {r.R, r.T}, {r.R, r.B}, {r.L, r.B}, {r.L, r.T}}
	antiHairLineRgn(pts[:], &clip, b)
}
