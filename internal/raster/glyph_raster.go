// Filling of glyph outlines with exact area coverage, and the growth of
// outlines.
//
// This file is not a port: CoreGraphics is closed source. The fill is the
// signed area accumulation of font-rs and golang.org/x/image/vector: each
// edge adds the area that it covers on one side of itself, and a running
// sum gives the coverage of each pixel. The scan converter of Skia (aaa.go)
// snaps edges to a quarter of a pixel, which CoreGraphics does not do. The
// growth of outlines (growOutline) and the coarse flattening of curves
// (flattenTolerance) model what the screenshots of Chromium show.

package raster

import "math"

// areaRaster accumulates the signed area of the edges of an outline.
type areaRaster struct {
	w, h int
	acc  []float32
}

// reset prepares a buffer of w by h pixels. The outline must stay one
// pixel away from the left and right borders.
func (r *areaRaster) reset(w, h int) {
	r.w, r.h = w, h
	n := w*h + 2
	if cap(r.acc) < n {
		r.acc = make([]float32, n)
		return
	}
	r.acc = r.acc[:n]
	clear(r.acc)
}

func floor32(x float32) float32 { return float32(math.Floor(float64(x))) }
func ceil32(x float32) float32  { return float32(math.Ceil(float64(x))) }

// line adds one edge.
func (r *areaRaster) line(x0, y0, x1, y1 float32) {
	if y0 == y1 {
		return
	}
	dir := float32(1)
	if y0 > y1 {
		x0, y0, x1, y1 = x1, y1, x0, y0
		dir = -1
	}
	dxdy := (x1 - x0) / (y1 - y0)
	x := x0
	if y0 < 0 {
		x -= float32(y0 * dxdy)
	}
	yStart := 0
	if y0 > 0 {
		yStart = int(y0)
	}
	yEnd := min(r.h, int(ceil32(y1)))
	wMax := float32(r.w)
	for y := yStart; y < yEnd; y++ {
		row := r.acc[y*r.w:]
		dy := min(float32(y+1), y1) - max(float32(y), y0)
		xnext := x + float32(dxdy*dy)
		d := float32(dy * dir)
		xa, xb := x, xnext
		if xa > xb {
			xa, xb = xb, xa
		}
		// Points outside the buffer would write outside the row.
		xa = min(max(xa, 0), wMax)
		xb = min(max(xb, 0), wMax)
		xaFloor := floor32(xa)
		xai := int(xaFloor)
		xbi := int(ceil32(xb))
		if xbi <= xai+1 {
			xmf := float32(0.5*(xa+xb)) - xaFloor
			row[xai] += d - float32(d*xmf)
			row[xai+1] += float32(d * xmf)
		} else {
			s := 1 / (xb - xa)
			xaf := xa - xaFloor
			a0 := float32(float32(0.5*s) * float32((1-xaf)*(1-xaf)))
			xbf := xb - float32(xbi) + 1
			am := float32(float32(0.5*s) * float32(xbf*xbf))
			row[xai] += float32(d * a0)
			if xbi == xai+2 {
				row[xai+1] += float32(d * (1 - a0 - am))
			} else {
				a1 := float32(s * (1.5 - xaf))
				row[xai+1] += float32(d * (a1 - a0))
				ds := float32(d * s)
				for xi := xai + 2; xi < xbi-1; xi++ {
					row[xi] += ds
				}
				a2 := a1 + float32(float32(xbi-xai-3)*s)
				row[xbi-1] += float32(d * (1 - a2 - am))
			}
			row[xbi] += float32(d * am)
		}
		x = xnext
	}
}

// flattenTolerance is the largest distance in pixels between a curve and
// the lines that replace it. CoreGraphics flattens glyph curves coarsely:
// each curve is cut in halves until the lines are this close. The value is
// a fit (glyph_calibrate_test.go).
var flattenTolerance = float32(0.24)

// curveSegments returns the number of lines that replace a curve whose
// largest distance to its chord is dev: a power of two.
func curveSegments(dev float32) int {
	n := 1
	for dev > flattenTolerance && n < 256 {
		dev /= 4
		n *= 2
	}
	return n
}

func (r *areaRaster) quad(p0, p1, p2 Point) {
	ddx := p0.X - float32(2*p1.X) + p2.X
	ddy := p0.Y - float32(2*p1.Y) + p2.Y
	dd := float32(math.Sqrt(float64(float32(ddx*ddx) + float32(ddy*ddy))))
	n := curveSegments(dd / 4)
	prev := p0
	pts := [3]Point{p0, p1, p2}
	for i := 1; i <= n; i++ {
		var q Point
		if i == n {
			q = p2
		} else {
			t := float32(i) / float32(n)
			q = evalQuadAt(pts[:], t)
		}
		r.line(prev.X, prev.Y, q.X, q.Y)
		prev = q
	}
}

func (r *areaRaster) cubic(p0, p1, p2, p3 Point) {
	ax := p0.X - float32(2*p1.X) + p2.X
	ay := p0.Y - float32(2*p1.Y) + p2.Y
	bx := p1.X - float32(2*p2.X) + p3.X
	by := p1.Y - float32(2*p2.Y) + p3.Y
	dd := float32(math.Sqrt(float64(max(float32(ax*ax)+float32(ay*ay), float32(bx*bx)+float32(by*by)))))
	n := curveSegments(float32(0.75 * dd))
	prev := p0
	pts := [4]Point{p0, p1, p2, p3}
	for i := 1; i <= n; i++ {
		var q Point
		if i == n {
			q = p3
		} else {
			t := float32(i) / float32(n)
			q = evalCubicPosAt(pts[:], t)
		}
		r.line(prev.X, prev.Y, q.X, q.Y)
		prev = q
	}
}

// addPath adds the contours of a path, moved by (-ox, -oy). Each contour
// is closed.
func (r *areaRaster) addPath(p *Path, ox, oy float32) {
	var first, cur Point
	open := false
	pi, ci := 0, 0
	at := func(i int) Point { return Point{p.pts[i].X - ox, p.pts[i].Y - oy} }
	closeContour := func() {
		if open && cur != first {
			r.line(cur.X, cur.Y, first.X, first.Y)
		}
		open = false
	}
	for _, v := range p.verbs {
		switch v {
		case verbMove:
			closeContour()
			first = at(pi)
			cur = first
			open = true
			pi++
		case verbLine:
			q := at(pi)
			r.line(cur.X, cur.Y, q.X, q.Y)
			cur = q
			pi++
		case verbQuad:
			q1, q2 := at(pi), at(pi+1)
			r.quad(cur, q1, q2)
			cur = q2
			pi += 2
		case verbConic:
			// Glyph outlines have no conics. A conic becomes quads.
			q1, q2 := at(pi), at(pi+1)
			var buf [1 + 2*(1<<5)]Point
			quads := conicToQuads(cur, q1, q2, p.conics[ci], 0.01, buf[:])
			for k := 0; k+2 < len(quads); k += 2 {
				r.quad(quads[k], quads[k+1], quads[k+2])
			}
			cur = q2
			pi += 2
			ci++
		case verbCubic:
			q1, q2, q3 := at(pi), at(pi+1), at(pi+2)
			r.cubic(cur, q1, q2, q3)
			cur = q3
			pi += 3
		case verbClose:
			closeContour()
			cur = first
		}
	}
	closeContour()
}

// coverage writes the coverage of each pixel into dst, as values from 0
// to 1. The fill rule is non-zero for contours that do not overlap.
func (r *areaRaster) coverage(dst []float32) {
	var sum float32
	n := r.w * r.h
	for i := 0; i < n; i++ {
		sum += r.acc[i]
		c := sum
		if c < 0 {
			c = -c
		}
		if c > 1 {
			c = 1
		}
		dst[i] = c
	}
}

// growOutline grows the closed contours of a path by dx horizontally and
// by dy vertically on each side: the outline becomes the sum of the glyph
// and a rectangle of 2*dx by 2*dy.
//
// Each edge of the control polygon moves by (+-dx, +-dy), with the signs of
// the components of its outward normal. An edge that is parallel to an
// axis moves only across itself. A point goes to the intersection of its
// two moved edges. An on-curve point that TrueType implies between two
// off-curve points stays in the middle of them.
func growOutline(p *Path, dx, dy float32) {
	if len(p.pts) < 3 {
		return
	}
	type span struct{ lo, hi int }
	var stack [16]span
	spans := stack[:0]
	// off marks the off-curve points.
	var offBuf [256]bool
	off := offBuf[:0]
	pi := 0
	start := -1
	for _, v := range p.verbs {
		switch v {
		case verbMove:
			if start >= 0 && pi > start {
				spans = append(spans, span{start, pi})
			}
			start = pi
			off = append(off, false)
			pi++
		case verbLine:
			off = append(off, false)
			pi++
		case verbQuad, verbConic:
			off = append(off, true, false)
			pi += 2
		case verbCubic:
			off = append(off, true, true, false)
			pi += 3
		}
	}
	if start >= 0 && pi > start {
		spans = append(spans, span{start, pi})
	}
	if len(off) != len(p.pts) {
		return
	}
	// The sign of the area of the outline tells which side of an edge is
	// outside.
	var area float64
	for _, s := range spans {
		pts := p.pts[s.lo:s.hi]
		for i := range pts {
			a, b := pts[i], pts[(i+1)%len(pts)]
			area += float64(float64(a.X)*float64(b.Y)) - float64(float64(b.X)*float64(a.Y))
		}
	}
	if area == 0 {
		return
	}
	sign := float32(1)
	if area < 0 {
		sign = -1
	}
	// shift returns the move of an edge with the unit direction v.
	shift := func(v Point) Point {
		nx, ny := float32(sign*v.Y), float32(-sign*v.X)
		var t Point
		if nx > growAxisEps {
			t.X = dx
		} else if nx < -growAxisEps {
			t.X = -dx
		}
		if ny > growAxisEps {
			t.Y = dy
		} else if ny < -growAxisEps {
			t.Y = -dy
		}
		return t
	}
	var ringBuf, outBuf [96]Point
	for _, s := range spans {
		pts := p.pts[s.lo:s.hi]
		ctrl := off[s.lo:s.hi]
		n := len(pts)
		// A contour that ends where it starts has its first point twice.
		closing := n > 1 && pts[n-1] == pts[0]
		if closing {
			n--
		}
		if n < 3 {
			continue
		}
		ring := append(ringBuf[:0], pts[:n]...)
		out := append(outBuf[:0], pts[:n]...)
		implied := func(i int) bool {
			if ctrl[i] {
				return false
			}
			a, b := (i-1+n)%n, (i+1)%n
			if !ctrl[a] || !ctrl[b] {
				return false
			}
			mx := float32(0.5 * (ring[a].X + ring[b].X))
			my := float32(0.5 * (ring[a].Y + ring[b].Y))
			return fabs32(mx-ring[i].X) < 1e-3 && fabs32(my-ring[i].Y) < 1e-3
		}
		for i := 0; i < n; i++ {
			if implied(i) {
				continue
			}
			cur := ring[i]
			// The nearest different points before and after.
			var in, outv Point
			var lin, lout float32
			for k := 1; k < n; k++ {
				q := ring[(i-k+n)%n]
				if q != cur {
					in = cur.sub(q)
					lin = float32(math.Hypot(float64(in.X), float64(in.Y)))
					break
				}
			}
			for k := 1; k < n; k++ {
				q := ring[(i+k)%n]
				if q != cur {
					outv = q.sub(cur)
					lout = float32(math.Hypot(float64(outv.X), float64(outv.Y)))
					break
				}
			}
			if lin == 0 || lout == 0 {
				continue
			}
			in.X, in.Y = in.X/lin, in.Y/lin
			outv.X, outv.Y = outv.X/lout, outv.Y/lout
			ta, tb := shift(in), shift(outv)
			c := cross(in, outv)
			if ta == tb || fabs32(c) < growParallel {
				// The edges are parallel, or move together.
				out[i] = Point{cur.X + float32(0.5*(ta.X+tb.X)), cur.Y + float32(0.5*(ta.Y+tb.Y))}
				continue
			}
			// The intersection of the two moved edges.
			k := cross(tb.sub(ta), outv) / c
			out[i] = Point{cur.X + ta.X + float32(k*in.X), cur.Y + ta.Y + float32(k*in.Y)}
		}
		for i := 0; i < n; i++ {
			if implied(i) {
				a, b := (i-1+n)%n, (i+1)%n
				out[i] = Point{float32(0.5 * (out[a].X + out[b].X)), float32(0.5 * (out[a].Y + out[b].Y))}
			}
		}
		copy(pts[:n], out)
		if closing {
			pts[n] = pts[0]
		}
	}
	p.convex = convexityUnknown
}

const (
	// growAxisEps is the largest normal component that counts as zero: an
	// edge with such a normal is parallel to an axis.
	growAxisEps = 1e-3
	// growParallel is the sine of the largest angle between two edges
	// that are treated as parallel.
	growParallel = 0.1
)

func fabs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
