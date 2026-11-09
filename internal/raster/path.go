// Ported from skia/src/core/SkPath.cpp, SkPathPriv.cpp, SkPathRawShapes.cpp,
// SkPathMakers.h and SkRRect.cpp (Skia branch chrome/m156).

package raster

import "math"

// Point is SkPoint: a point in float32.
type Point struct{ X, Y float32 }

func (p Point) add(o Point) Point { return Point{p.X + o.X, p.Y + o.Y} }
func (p Point) sub(o Point) Point { return Point{p.X - o.X, p.Y - o.Y} }
func (p Point) isZero() bool      { return p.X == 0 && p.Y == 0 }
func (p Point) isFinite() bool {
	return !math.IsInf(float64(p.X), 0) && !math.IsNaN(float64(p.X)) &&
		!math.IsInf(float64(p.Y), 0) && !math.IsNaN(float64(p.Y))
}

// cross is SkPoint::CrossProduct.
func cross(a, b Point) float32 { return float32(a.X*b.Y) - float32(a.Y*b.X) }

// dot is SkPoint::DotProduct.
func dot(a, b Point) float32 { return float32(a.X*b.X) + float32(a.Y*b.Y) }

// Rect is SkRect.
type Rect struct{ L, T, R, B float32 }

func (r Rect) width() float32  { return r.R - r.L }
func (r Rect) height() float32 { return r.B - r.T }
func (r Rect) isEmpty() bool   { return !(r.L < r.R && r.T < r.B) }

func (r Rect) sorted() Rect {
	if r.L > r.R {
		r.L, r.R = r.R, r.L
	}
	if r.T > r.B {
		r.T, r.B = r.B, r.T
	}
	return r
}

func (r Rect) outset(dx, dy float32) Rect {
	// The conversions keep a caller from fusing a product into the sums.
	return Rect{r.L - float32(dx), r.T - float32(dy), r.R + float32(dx), r.B + float32(dy)}
}

// roundOut is SkRect::roundOut.
func (r Rect) roundOut() IRect {
	return IRect{floorToInt(r.L), floorToInt(r.T), ceilToInt(r.R), ceilToInt(r.B)}
}

// round is SkRect::round.
func (r Rect) round() IRect {
	return IRect{roundToInt(r.L), roundToInt(r.T), roundToInt(r.R), roundToInt(r.B)}
}

func (r *Rect) intersect(o Rect) bool {
	l, t := max(r.L, o.L), max(r.T, o.T)
	rr, b := min(r.R, o.R), min(r.B, o.B)
	if !(l < rr && t < b) {
		return false
	}
	r.L, r.T, r.R, r.B = l, t, rr, b
	return true
}

// IRect is SkIRect.
type IRect struct{ L, T, R, B int }

func (r IRect) width() int    { return r.R - r.L }
func (r IRect) height() int   { return r.B - r.T }
func (r IRect) isEmpty() bool { return r.L >= r.R || r.T >= r.B }
func (r IRect) contains(o IRect) bool {
	return !o.isEmpty() && !r.isEmpty() && r.L <= o.L && r.T <= o.T && r.R >= o.R && r.B >= o.B
}
func (r *IRect) intersect(o IRect) bool {
	l, t := max(r.L, o.L), max(r.T, o.T)
	rr, b := min(r.R, o.R), min(r.B, o.B)
	if l >= rr || t >= b {
		return false
	}
	r.L, r.T, r.R, r.B = l, t, rr, b
	return true
}
func (r IRect) outset(dx, dy int) IRect { return IRect{r.L - dx, r.T - dy, r.R + dx, r.B + dy} }
func (r IRect) toRect() Rect {
	return Rect{float32(r.L), float32(r.T), float32(r.R), float32(r.B)}
}
func (r IRect) join(o IRect) IRect {
	if o.isEmpty() {
		return r
	}
	if r.isEmpty() {
		return o
	}
	return IRect{min(r.L, o.L), min(r.T, o.T), max(r.R, o.R), max(r.B, o.B)}
}

// floorToInt, ceilToInt and roundToInt are sk_float_floor2int and friends:
// the result saturates.
func floorToInt(x float32) int { return int(floatToInt(float32(math.Floor(float64(x))))) }
func ceilToInt(x float32) int  { return int(floatToInt(float32(math.Ceil(float64(x))))) }
func roundToInt(x float32) int {
	return int(floatToInt(float32(math.Floor(float64(x) + 0.5))))
}

type pathVerb uint8

const (
	verbMove pathVerb = iota
	verbLine
	verbQuad
	verbConic
	verbCubic
	verbClose
)

const (
	segLine  = 1
	segQuad  = 2
	segConic = 4
	segCubic = 8
)

type fillType uint8

const (
	fillWinding fillType = iota
	fillEvenOdd
)

type convexity uint8

const (
	convexityUnknown convexity = iota
	convexityConvex
	convexityConcave
)

// Path is the subset of SkPath that the painter needs.
type Path struct {
	verbs  []pathVerb
	pts    []Point
	conics []float32
	fill   fillType

	convex  convexity
	segMask uint8
	// lastMoveIdx is the index in pts of the last moveTo, or -1.
	lastMoveIdx int
}

// NewPath returns an empty path.
func NewPath() *Path { return &Path{lastMoveIdx: -1} }

// Reset empties the path and keeps its storage.
func (p *Path) Reset() {
	p.verbs, p.pts, p.conics = p.verbs[:0], p.pts[:0], p.conics[:0]
	p.fill, p.convex, p.segMask = fillWinding, convexityUnknown, 0
	p.lastMoveIdx = -1
}

func (p *Path) isEmpty() bool { return len(p.verbs) == 0 }

func (p *Path) lastPt() (Point, bool) {
	if len(p.pts) == 0 {
		return Point{}, false
	}
	return p.pts[len(p.pts)-1], true
}

// MoveTo is SkPathBuilder::moveTo. A move that follows a move replaces it.
func (p *Path) MoveTo(x, y float32) {
	if n := len(p.verbs); n > 0 && p.verbs[n-1] == verbMove {
		p.pts[len(p.pts)-1] = Point{x, y}
		return
	}
	p.lastMoveIdx = len(p.pts)
	p.verbs = append(p.verbs, verbMove)
	p.pts = append(p.pts, Point{x, y})
	p.convex = convexityUnknown
}

// ensureMove is SkPathBuilder::ensureMove: a segment after a close starts a
// new contour at the start point of the closed contour.
func (p *Path) ensureMove() {
	n := len(p.verbs)
	if n == 0 {
		p.MoveTo(0, 0)
	} else if p.verbs[n-1] == verbClose {
		s := p.pts[p.lastMoveIdx]
		p.MoveTo(s.X, s.Y)
	}
}

// LineTo is SkPathBuilder::lineTo.
func (p *Path) LineTo(x, y float32) {
	p.ensureMove()
	p.verbs = append(p.verbs, verbLine)
	p.pts = append(p.pts, Point{x, y})
	p.segMask |= segLine
	p.convex = convexityUnknown
}

// QuadTo is SkPathBuilder::quadTo.
func (p *Path) QuadTo(x1, y1, x2, y2 float32) {
	p.ensureMove()
	p.verbs = append(p.verbs, verbQuad)
	p.pts = append(p.pts, Point{x1, y1}, Point{x2, y2})
	p.segMask |= segQuad
	p.convex = convexityUnknown
}

// ConicTo is SkPathBuilder::conicTo. Special weights give simpler verbs.
func (p *Path) ConicTo(x1, y1, x2, y2, w float32) {
	p.ensureMove()
	if w <= 0 {
		p.LineTo(x2, y2)
		return
	}
	p.pts = append(p.pts, Point{x1, y1}, Point{x2, y2})
	switch {
	case w == 1:
		p.verbs = append(p.verbs, verbQuad)
		p.segMask |= segQuad
	case isFinite32(w):
		p.verbs = append(p.verbs, verbConic)
		p.conics = append(p.conics, w)
		p.segMask |= segConic
	default:
		p.verbs = append(p.verbs, verbLine, verbLine)
		p.segMask |= segLine
	}
	p.convex = convexityUnknown
}

// CubicTo is SkPathBuilder::cubicTo.
func (p *Path) CubicTo(x1, y1, x2, y2, x3, y3 float32) {
	p.ensureMove()
	p.verbs = append(p.verbs, verbCubic)
	p.pts = append(p.pts, Point{x1, y1}, Point{x2, y2}, Point{x3, y3})
	p.segMask |= segCubic
	p.convex = convexityUnknown
}

// Close is SkPathBuilder::close. A second close is ignored.
func (p *Path) Close() {
	if n := len(p.verbs); n > 0 && p.verbs[n-1] != verbClose {
		p.ensureMove()
		p.verbs = append(p.verbs, verbClose)
	}
}

func (p *Path) setLastPt(x, y float32) {
	if len(p.pts) == 0 {
		p.MoveTo(x, y)
		return
	}
	p.pts[len(p.pts)-1] = Point{x, y}
}

var rectCorner = func(r Rect, i int) Point {
	switch i & 3 {
	case 0:
		return Point{r.L, r.T}
	case 1:
		return Point{r.R, r.T}
	case 2:
		return Point{r.R, r.B}
	}
	return Point{r.L, r.B}
}

// AddRect is SkPathBuilder::addRect(rect, kCW, startIndex).
func (p *Path) AddRect(r Rect, startIndex int) {
	wasEmpty := p.isEmpty()
	c := rectCorner(r, startIndex)
	p.MoveTo(c.X, c.Y)
	for i := 1; i < 4; i++ {
		c = rectCorner(r, startIndex+i)
		p.LineTo(c.X, c.Y)
	}
	p.Close()
	if wasEmpty {
		p.convex = convexityConvex
	}
}

const scalarRoot2Over2 = float32(0.707106781)

// AddOval is SkPathBuilder::addOval(oval, kCW, startIndex).
func (p *Path) AddOval(r Rect, startIndex int) {
	wasEmpty := p.isEmpty()
	cx := float32(0.5 * (float64(r.L) + float64(r.R)))
	cy := float32(0.5 * (float64(r.T) + float64(r.B)))
	oval := func(i int) Point {
		switch i & 3 {
		case 0:
			return Point{cx, r.T}
		case 1:
			return Point{r.R, cy}
		case 2:
			return Point{cx, r.B}
		}
		return Point{r.L, cy}
	}
	c := oval(startIndex)
	p.MoveTo(c.X, c.Y)
	for i := 1; i <= 4; i++ {
		ctrl := rectCorner(r, startIndex+i)
		end := oval(startIndex + i)
		p.ConicTo(ctrl.X, ctrl.Y, end.X, end.Y, scalarRoot2Over2)
	}
	p.Close()
	if wasEmpty {
		p.convex = convexityConvex
	}
}

// AddRRect is SkPathBuilder::addRRect for a rounded rectangle whose four
// corners have the radii (rx, ry), clockwise, with start index 0. The radii
// are scaled as SkRRect::setRectXY does.
func (p *Path) AddRRect(r Rect, rx, ry float32) {
	if r.isEmpty() {
		return
	}
	if math.IsInf(float64(rx), 0) || math.IsNaN(float64(rx)) || math.IsInf(float64(ry), 0) || math.IsNaN(float64(ry)) {
		rx, ry = 0, 0
	}
	if r.width() < rx+rx || r.height() < ry+ry {
		scale := min(r.width()/(rx+rx), r.height()/(ry+ry))
		rx = float32(rx * scale)
		ry = float32(ry * scale)
	}
	if rx <= 0 || ry <= 0 {
		// (index + 1) / 2 with index 0.
		p.AddRect(r, 0)
		return
	}
	if rx >= r.width()/2 && ry >= r.height()/2 {
		// index / 2 with index 0.
		p.AddOval(r, 0)
		return
	}
	wasEmpty := p.isEmpty()
	l, t, rr, b := r.L, r.T, r.R, r.B
	rrectPt := func(i int) Point {
		switch i & 7 {
		case 0:
			return Point{l + rx, t}
		case 1:
			return Point{rr - rx, t}
		case 2:
			return Point{rr, t + ry}
		case 3:
			return Point{rr, b - ry}
		case 4:
			return Point{rr - rx, b}
		case 5:
			return Point{l + rx, b}
		case 6:
			return Point{l, b - ry}
		}
		return Point{l, t + ry}
	}
	// Index 0, clockwise: the contour starts with a line.
	c := rrectPt(0)
	p.MoveTo(c.X, c.Y)
	for i := 0; i < 4; i++ {
		line := rrectPt(2*i + 1)
		p.LineTo(line.X, line.Y)
		ctrl := rectCorner(r, i+1)
		end := rrectPt(2*i + 2)
		p.ConicTo(ctrl.X, ctrl.Y, end.X, end.Y, scalarRoot2Over2)
	}
	p.Close()
	if wasEmpty {
		p.convex = convexityConvex
	}
}

// bounds is SkPath::getBounds: the bounds of all points, control points
// included.
func (p *Path) bounds() Rect {
	if len(p.pts) == 0 {
		return Rect{}
	}
	l, t := p.pts[0].X, p.pts[0].Y
	r, b := l, t
	for _, q := range p.pts[1:] {
		l, r = min(l, q.X), max(r, q.X)
		t, b = min(t, q.Y), max(b, q.Y)
	}
	return Rect{l, t, r, b}
}

func (p *Path) isFinite() bool {
	for _, q := range p.pts {
		if !q.isFinite() {
			return false
		}
	}
	return true
}

// transformInto writes the path mapped by m into dst (SkPath::transform).
func (p *Path) transformInto(dst *Path, m *Matrix) {
	dst.verbs = append(dst.verbs[:0], p.verbs...)
	dst.conics = append(dst.conics[:0], p.conics...)
	if cap(dst.pts) < len(p.pts) {
		dst.pts = make([]Point, len(p.pts))
	}
	dst.pts = dst.pts[:len(p.pts)]
	m.mapPoints(dst.pts, p.pts)
	dst.fill = p.fill
	dst.segMask = p.segMask
	dst.lastMoveIdx = p.lastMoveIdx
	// Skia keeps the convexity only for an axis-aligned path under a
	// scale and translate matrix, and computes it again otherwise. Both
	// give the result of the computation on the mapped points.
	dst.convex = convexityUnknown
}

func rectMakeDir(dx, dy float32) int {
	d := 0
	if dx != 0 {
		d |= 1
	}
	if dx > 0 || dy > 0 {
		d |= 2
	}
	return d
}

// isRect is SkPathRaw::isRect (SkPathPriv::IsRectContour without partial
// contours).
func (p *Path) isRect() (Rect, bool) {
	if p.segMask != segLine || len(p.pts) < 4 || len(p.verbs) < 4 {
		return Rect{}, false
	}
	pts := p.pts
	if len(pts) == 4 && len(p.verbs) == 5 && p.verbs[0] == verbMove && p.verbs[1] == verbLine &&
		p.verbs[2] == verbLine && p.verbs[3] == verbLine && p.verbs[4] == verbClose {
		v0, v1, v2, v3 := pts[1].sub(pts[0]), pts[2].sub(pts[1]), pts[3].sub(pts[2]), pts[0].sub(pts[3])
		ortho := func(a, b Point) bool {
			return ((a.X == 0) != (b.X == 0)) && ((a.Y == 0) != (b.Y == 0))
		}
		if ((v0.X == 0) != (v0.Y == 0)) && ortho(v0, v1) && ortho(v1, v2) && ortho(v2, v3) {
			return Rect{pts[0].X, pts[0].Y, pts[2].X, pts[2].Y}.sorted(), true
		}
	}
	corners := 0
	var lineStart, firstCorner, thirdCorner Point
	firstPt, lastPt := -1, -1
	directions := [5]int{-1, -1, -1, -1, -1}
	closedOrMoved := false
	autoClose := false
	pi := 0
	for _, verb := range p.verbs {
		switch verb {
		case verbClose, verbLine:
			if verb == verbClose {
				autoClose = true
			} else {
				lastPt = pi
			}
			var lineEnd Point
			if verb == verbClose {
				if firstPt < 0 {
					return Rect{}, false
				}
				lineEnd = pts[firstPt]
			} else {
				lineEnd = pts[pi]
				pi++
			}
			delta := lineEnd.sub(lineStart)
			if delta.X != 0 && delta.Y != 0 {
				return Rect{}, false
			}
			if !delta.isFinite() {
				return Rect{}, false
			}
			if lineStart == lineEnd {
				continue
			}
			next := rectMakeDir(delta.X, delta.Y)
			if corners == 0 {
				directions[0] = next
				corners = 1
				closedOrMoved = false
				lineStart = lineEnd
				continue
			}
			if closedOrMoved {
				return Rect{}, false
			}
			if autoClose && next == directions[0] {
				continue
			}
			closedOrMoved = autoClose
			if directions[corners-1] == next {
				if corners == 3 && verb == verbLine {
					thirdCorner = lineEnd
				}
				lineStart = lineEnd
				continue
			}
			if corners >= 4 {
				return Rect{}, false
			}
			directions[corners] = next
			corners++
			switch corners {
			case 2:
				firstCorner = lineStart
			case 3:
				if directions[0]^directions[2] != 2 {
					return Rect{}, false
				}
				thirdCorner = lineEnd
			case 4:
				if directions[1]^directions[3] != 2 {
					return Rect{}, false
				}
			}
			lineStart = lineEnd
		case verbMove:
			if corners == 0 {
				firstPt = pi
			} else {
				c := pts[firstPt].sub(pts[lastPt])
				if c.X != 0 && c.Y != 0 {
					return Rect{}, false
				}
			}
			lineStart = pts[pi]
			pi++
			closedOrMoved = true
		default:
			return Rect{}, false
		}
	}
	if corners < 3 || corners > 4 || firstPt < 0 || lastPt < 0 {
		return Rect{}, false
	}
	c := pts[firstPt].sub(pts[lastPt])
	if c.X != 0 && c.Y != 0 {
		return Rect{}, false
	}
	return Rect{firstCorner.X, firstCorner.Y, thirdCorner.X, thirdCorner.Y}.sorted(), true
}

// ---- convexity: SkPathPriv::ComputeConvexity ----

type dirChange uint8

const (
	dirUnknown dirChange = iota
	dirLeft
	dirRight
	dirStraight
	dirBackwards
	dirInvalid
)

type convexicator struct {
	firstPt, firstVec, lastPt, lastVec Point
	expectedDir                        dirChange
	firstDirKnown                      bool
	reversals                          int
	isFinite                           bool
}

func (c *convexicator) setMovePt(pt Point) {
	c.firstPt, c.lastPt = pt, pt
	c.expectedDir = dirInvalid
}

func (c *convexicator) addPt(pt Point) bool {
	if c.lastPt == pt {
		return true
	}
	if c.firstPt == c.lastPt && c.expectedDir == dirInvalid && c.lastVec.isZero() {
		c.lastVec = pt.sub(c.lastPt)
		c.firstVec = c.lastVec
	} else if !c.addVec(pt.sub(c.lastPt)) {
		return false
	}
	c.lastPt = pt
	return true
}

func (c *convexicator) close() bool {
	return c.addPt(c.firstPt) && c.addVec(c.firstVec)
}

func (c *convexicator) directionChange(cur Point) dirChange {
	cr := cross(c.lastVec, cur)
	if math.IsInf(float64(cr), 0) || math.IsNaN(float64(cr)) {
		return dirUnknown
	}
	if cr == 0 {
		if dot(c.lastVec, cur) < 0 {
			return dirBackwards
		}
		return dirStraight
	}
	if cr > 0 {
		return dirRight
	}
	return dirLeft
}

func (c *convexicator) addVec(cur Point) bool {
	dir := c.directionChange(cur)
	switch dir {
	case dirLeft, dirRight:
		if c.expectedDir == dirInvalid {
			c.expectedDir = dir
			c.firstDirKnown = true
		} else if dir != c.expectedDir {
			c.firstDirKnown = false
			return false
		}
		c.lastVec = cur
	case dirStraight:
	case dirBackwards:
		c.lastVec = cur
		c.reversals++
		return c.reversals < 3
	case dirUnknown:
		c.isFinite = false
		return false
	}
	return true
}

func isConcaveBySign(pts []Point) bool {
	if len(pts) <= 3 {
		return false
	}
	sign := func(x float32) int {
		if x < 0 {
			return 1
		}
		return 0
	}
	cur := pts[0]
	first := cur
	dxes, dyes := 0, 0
	lastSx, lastSy := 2, 2
	step := func(next Point) bool {
		vec := next.sub(cur)
		if !vec.isZero() {
			if !vec.isFinite() {
				return true
			}
			sx, sy := sign(vec.X), sign(vec.Y)
			if sx != lastSx {
				dxes++
			}
			if sy != lastSy {
				dyes++
			}
			if dxes > 3 || dyes > 3 {
				return true
			}
			lastSx, lastSy = sx, sy
		}
		cur = next
		return false
	}
	for _, q := range pts[1:] {
		if step(q) {
			return true
		}
	}
	return step(first)
}

// isConvex reports the convexity of the path, computing it when unknown.
func (p *Path) isConvex() bool {
	if p.convex == convexityUnknown {
		p.convex = p.computeConvexity()
	}
	return p.convex == convexityConvex
}

func (p *Path) computeConvexity() convexity {
	if !p.isFinite() {
		return convexityConcave
	}
	verbs, pts := p.verbs, p.pts
	for len(verbs) > 0 && verbs[len(verbs)-1] == verbMove {
		verbs = verbs[:len(verbs)-1]
		pts = pts[:len(pts)-1]
	}
	if len(verbs) == 0 {
		return convexityConvex
	}
	if isConcaveBySign(pts) {
		return convexityConcave
	}
	contourCount := 0
	needsClose := false
	state := convexicator{expectedDir: dirInvalid, isFinite: true}
	pi := 0
	for _, verb := range verbs {
		n := 0
		switch verb {
		case verbMove, verbLine:
			n = 1
		case verbQuad, verbConic:
			n = 2
		case verbCubic:
			n = 3
		}
		if contourCount == 0 {
			if verb == verbMove {
				state.setMovePt(pts[pi])
			} else {
				contourCount++
				needsClose = true
			}
		}
		if contourCount == 1 {
			if verb == verbClose || verb == verbMove {
				if !state.close() {
					return convexityConcave
				}
				needsClose = false
				contourCount++
			} else {
				for i := 0; i < n; i++ {
					if !state.addPt(pts[pi+i]) {
						return convexityConcave
					}
				}
			}
		} else if contourCount > 1 && verb != verbMove {
			return convexityConcave
		}
		pi += n
	}
	if needsClose && !state.close() {
		return convexityConcave
	}
	if !state.firstDirKnown && state.reversals >= 3 {
		return convexityConcave
	}
	// Skia maps an unknown first direction to a degenerate convex path.
	return convexityConvex
}
