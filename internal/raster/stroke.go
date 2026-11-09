// Ported from skia/src/core/SkStroke.cpp, SkStrokerPriv.cpp and the helpers
// they use in SkGeometry.cpp (Skia branch chrome/m156).
//
// The stroker turns a path and a stroke width into the path of the stroke
// outline. Skia fills that outline. Curves become quadratic approximations
// whose precision depends on the resolution scale of the transform.

package raster

import "math"

type strokeJoin uint8

const (
	joinMiter strokeJoin = iota
	joinRound
	joinBevel
)

type strokeType int8

const (
	outerStroke strokeType = 1
	innerStroke strokeType = -1
)

type resultType uint8

const (
	resultSplit resultType = iota
	resultDegenerate
	resultQuad
)

type reductionType uint8

const (
	reductionPoint reductionType = iota
	reductionLine
	reductionQuad
	reductionDegenerate
	reductionDegenerate2
	reductionDegenerate3
)

type intersectRayType uint8

const (
	ctrlPtRay intersectRayType = iota
	resultTypeRay
)

const (
	tangentRecursiveLimit = 0
	cubicRecursiveLimit   = 1
	conicRecursiveLimit   = 2
	quadRecursiveLimit    = 3
)

var recursiveLimits = [4]int{5 * 3, 24, 16, 16}

func degenerateVector(v Point) bool {
	return !(isFinite32(v.X) && isFinite32(v.Y) && (v.X != 0 || v.Y != 0))
}

func rotateCCW(v Point) Point { return Point{v.Y, -v.X} }
func rotateCW(v Point) Point  { return Point{-v.Y, v.X} }

func scalePt(v Point, s float32) Point { return Point{float32(v.X * s), float32(v.Y * s)} }

// setNormalUnitNormal is set_normal_unitnormal(before, after, scale, radius).
func setNormalUnitNormal(before, after Point, scale, radius float32) (normal, unitNormal Point, ok bool) {
	u, ok := setLength(Point{float32((after.X - before.X) * scale), float32((after.Y - before.Y) * scale)}, 1)
	if !ok {
		return Point{}, Point{}, false
	}
	u = rotateCCW(u)
	return scalePt(u, radius), u, true
}

// setNormalUnitNormalVec is set_normal_unitnormal(vec, radius).
func setNormalUnitNormalVec(vec Point, radius float32) (normal, unitNormal Point, ok bool) {
	u, ok := setLength(vec, 1)
	if !ok {
		return Point{}, Point{}, false
	}
	u = rotateCCW(u)
	return scalePt(u, radius), u, true
}

// quadConstruct is SkQuadConstruct: the state of the quad stroke under
// construction.
type quadConstruct struct {
	quad             [3]Point
	tangentStart     Point
	tangentEnd       Point
	startT           float32
	midT             float32
	endT             float32
	startSet         bool
	endSet           bool
	oppositeTangents bool
}

func (q *quadConstruct) init(start, end float32) bool {
	q.startT = start
	q.midT = float32((start + end) * 0.5)
	q.endT = end
	q.startSet, q.endSet = false, false
	return q.startT < q.midT && q.midT < q.endT
}

func (q *quadConstruct) initWithStart(parent *quadConstruct) bool {
	if !q.init(parent.startT, parent.midT) {
		return false
	}
	q.quad[0] = parent.quad[0]
	q.tangentStart = parent.tangentStart
	q.startSet = true
	return true
}

func (q *quadConstruct) initWithEnd(parent *quadConstruct) bool {
	if !q.init(parent.midT, parent.endT) {
		return false
	}
	q.quad[2] = parent.quad[2]
	q.tangentEnd = parent.tangentEnd
	q.endSet = true
	return true
}

func isZeroLengthSincePoint(pts []Point, start int) bool {
	count := len(pts) - start
	if count < 2 {
		return true
	}
	first := pts[start]
	for i := 1; i < count; i++ {
		if first != pts[start+i] {
			return false
		}
	}
	return true
}

// pathStroker is SkPathStroker.
type pathStroker struct {
	radius             float32
	invMiterLimit      float32
	resScale           float32
	invResScale        float32
	invResScaleSquared float32
	firstNormal        Point
	prevNormal         Point
	firstUnitNormal    Point
	prevUnitNormal     Point
	firstPt, prevPt    Point
	firstOuterPt       Point
	firstOuterPtIndex  int
	segmentCount       int
	prevIsLine         bool
	cap                strokeCap
	join               strokeJoin
	inner, outer       *Path
	cusper             *Path
	stype              strokeType
	recursionDepth     int
	foundTangents      bool
	joinCompleted      bool
}

func (s *pathStroker) init(radius, miterLimit float32, cap strokeCap, join strokeJoin, resScale float32, outer, inner, cusper *Path) {
	*s = pathStroker{radius: radius, resScale: resScale, outer: outer, inner: inner, cusper: cusper}
	if join == joinMiter {
		if miterLimit <= 1 {
			join = joinBevel
		} else {
			s.invMiterLimit = 1 / miterLimit
		}
	}
	s.cap = cap
	s.join = join
	s.segmentCount = -1
	s.invResScale = 1 / float32(resScale*4)
	s.invResScaleSquared = float32(s.invResScale * s.invResScale)
}

// ---- caps: SkStrokerPriv.cpp ----

func (s *pathStroker) capper(path *Path, pivot, normal, stop Point, extendLastPt bool) {
	switch s.cap {
	case capButt:
		path.LineTo(stop.X, stop.Y)
	case capRound:
		parallel := rotateCW(normal)
		projectedCenter := pivot.add(parallel)
		c1 := projectedCenter.add(normal)
		path.ConicTo(c1.X, c1.Y, projectedCenter.X, projectedCenter.Y, scalarRoot2Over2)
		c2 := projectedCenter.sub(normal)
		path.ConicTo(c2.X, c2.Y, stop.X, stop.Y, scalarRoot2Over2)
	case capSquare:
		parallel := rotateCW(normal)
		if extendLastPt {
			a := pivot.add(normal).add(parallel)
			path.setLastPt(a.X, a.Y)
			b := pivot.sub(normal).add(parallel)
			path.LineTo(b.X, b.Y)
		} else {
			a := pivot.add(normal).add(parallel)
			path.LineTo(a.X, a.Y)
			b := pivot.sub(normal).add(parallel)
			path.LineTo(b.X, b.Y)
			path.LineTo(stop.X, stop.Y)
		}
	}
}

// ---- joins: SkStrokerPriv.cpp ----

func isClockwise(before, after Point) bool {
	return float32(before.X*after.Y) > float32(before.Y*after.X)
}

type angleType uint8

const (
	angleNearly180 angleType = iota
	angleSharp
	angleShallow
	angleNearlyLine
)

func dot2AngleType(d float32) angleType {
	if d >= 0 {
		if nearlyZero(1 - d) {
			return angleNearlyLine
		}
		return angleShallow
	}
	if nearlyZero(1 + d) {
		return angleNearly180
	}
	return angleSharp
}

func handleInnerJoin(inner *Path, pivot, after Point) {
	inner.LineTo(pivot.X, pivot.Y)
	inner.LineTo(pivot.X-after.X, pivot.Y-after.Y)
}

// buildUnitArc is SkConic::BuildUnitArc with a matrix that scales by radius
// and translates to pivot.
func buildUnitArc(uStart, uStop Point, cw bool, radius float32, pivot Point, dst *[5]conic) int {
	x := dot(uStart, uStop)
	y := cross(uStart, uStop)
	absY := abs32f(y)
	if absY <= scalarNearlyZero && x > 0 && ((y >= 0 && cw) || (y <= 0 && !cw)) {
		return 0
	}
	if !cw {
		y = -y
	}
	quadrant := 0
	if y == 0 {
		quadrant = 2
	} else if x == 0 {
		if y > 0 {
			quadrant = 1
		} else {
			quadrant = 3
		}
	} else {
		if y < 0 {
			quadrant += 2
		}
		if (x < 0) != (y < 0) {
			quadrant++
		}
	}
	quadrantPts := [8]Point{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}}
	conicCount := quadrant
	for i := 0; i < conicCount; i++ {
		dst[i].pts = [3]Point{quadrantPts[(i*2)&7], quadrantPts[(i*2+1)&7], quadrantPts[(i*2+2)&7]}
		dst[i].w = scalarRoot2Over2
	}
	finalP := Point{x, y}
	lastQ := quadrantPts[(quadrant*2)&7]
	d := dot(lastQ, finalP)
	if d != d {
		return 0
	}
	if d < 1 {
		offCurve := Point{lastQ.X + x, lastQ.Y + y}
		cosThetaOver2 := sqrt32((1 + d) / 2)
		offCurve, _ = setLength(offCurve, 1/cosThetaOver2)
		if !ptsEqualWithinTolerance(lastQ, offCurve) {
			dst[conicCount].pts = [3]Point{lastQ, offCurve, finalP}
			dst[conicCount].w = cosThetaOver2
			conicCount++
		}
	}
	// matrix = userMatrix * setSinCos(uStart.y, uStart.x) * (ccw: scale(1, -1)).
	sinV, cosV := uStart.Y, uStart.X
	rot := Matrix{sx: cosV, kx: -sinV, ky: sinV, sy: cosV}
	if !cw {
		// preScale(1, -1): the y column is negated.
		rot.kx = -rot.kx
		rot.sy = -rot.sy
	}
	// postConcat(userMatrix): SkMatrix::setConcat(user, rot) with the
	// double precision products of muladdmul.
	m := concatMatrix(&Matrix{sx: radius, sy: radius, tx: pivot.X, ty: pivot.Y}, &rot)
	for i := 0; i < conicCount; i++ {
		m.mapPoints(dst[i].pts[:], dst[i].pts[:])
	}
	return conicCount
}

func muladdmul(a, b, c, d float32) float32 {
	return float32(float64(float64(a)*float64(b)) + float64(float64(c)*float64(d)))
}

// concatMatrix is SkMatrix::setConcat(a, b): b applies first.
func concatMatrix(a, b *Matrix) Matrix {
	if a.isIdentity() {
		return *b
	}
	if b.isIdentity() {
		return *a
	}
	if a.kx == 0 && a.ky == 0 && b.kx == 0 && b.ky == 0 {
		return Matrix{
			sx: float32(a.sx * b.sx), sy: float32(a.sy * b.sy),
			tx: float32(a.sx*b.tx) + a.tx, ty: float32(a.sy*b.ty) + a.ty,
		}
	}
	return Matrix{
		sx: muladdmul(a.sx, b.sx, a.kx, b.ky),
		kx: muladdmul(a.sx, b.kx, a.kx, b.sy),
		tx: muladdmul(a.sx, b.tx, a.kx, b.ty) + a.tx,
		ky: muladdmul(a.ky, b.sx, a.sy, b.ky),
		sy: muladdmul(a.ky, b.kx, a.sy, b.sy),
		ty: muladdmul(a.ky, b.tx, a.sy, b.ty) + a.ty,
	}
}

func (s *pathStroker) joiner(join strokeJoin, beforeUnitNormal, pivot, afterUnitNormal Point, prevIsLine, currIsLine bool) {
	outer, inner := s.outer, s.inner
	radius := s.radius
	switch join {
	case joinBevel:
		after := scalePt(afterUnitNormal, radius)
		if !isClockwise(beforeUnitNormal, afterUnitNormal) {
			outer, inner = inner, outer
			after = Point{-after.X, -after.Y}
		}
		outer.LineTo(pivot.X+after.X, pivot.Y+after.Y)
		handleInnerJoin(inner, pivot, after)
	case joinRound:
		dotProd := dot(beforeUnitNormal, afterUnitNormal)
		if dot2AngleType(dotProd) == angleNearlyLine {
			return
		}
		before, after := beforeUnitNormal, afterUnitNormal
		cw := true
		if !isClockwise(before, after) {
			outer, inner = inner, outer
			before = Point{-before.X, -before.Y}
			after = Point{-after.X, -after.Y}
			cw = false
		}
		var conics [5]conic
		count := buildUnitArc(before, after, cw, radius, pivot, &conics)
		if count > 0 {
			for i := 0; i < count; i++ {
				c := &conics[i]
				outer.ConicTo(c.pts[1].X, c.pts[1].Y, c.pts[2].X, c.pts[2].Y, c.w)
			}
			after = scalePt(after, radius)
			handleInnerJoin(inner, pivot, after)
		}
	case joinMiter:
		const oneOverSqrt2 = float32(0.707106781)
		dotProd := dot(beforeUnitNormal, afterUnitNormal)
		at := dot2AngleType(dotProd)
		before, after := beforeUnitNormal, afterUnitNormal
		var mid Point
		if at == angleNearlyLine {
			return
		}
		doMiter := false
		doBlunt := func() {
			after = scalePt(after, radius)
			if !currIsLine {
				outer.LineTo(pivot.X+after.X, pivot.Y+after.Y)
			}
			handleInnerJoin(inner, pivot, after)
		}
		if at == angleNearly180 {
			currIsLine = false
			doBlunt()
			return
		}
		ccw := !isClockwise(before, after)
		if ccw {
			outer, inner = inner, outer
			before = Point{-before.X, -before.Y}
			after = Point{-after.X, -after.Y}
		}
		if dotProd == 0 && s.invMiterLimit <= oneOverSqrt2 {
			mid = scalePt(before.add(after), radius)
			doMiter = true
		}
		if !doMiter {
			sinHalfAngle := sqrt32((1 + dotProd) / 2)
			if sinHalfAngle < s.invMiterLimit {
				currIsLine = false
				doBlunt()
				return
			}
			if at == angleSharp {
				mid = Point{after.Y - before.Y, before.X - after.X}
				if ccw {
					mid = Point{-mid.X, -mid.Y}
				}
			} else {
				mid = before.add(after)
			}
			mid, _ = setLength(mid, radius/sinHalfAngle)
		}
		if prevIsLine {
			outer.setLastPt(pivot.X+mid.X, pivot.Y+mid.Y)
		} else {
			outer.LineTo(pivot.X+mid.X, pivot.Y+mid.Y)
		}
		doBlunt()
	}
}

// ---- SkPathStroker ----

func (s *pathStroker) preJoinTo(currPt Point, currIsLine bool) (normal, unitNormal Point, ok bool) {
	normal, unitNormal, ok = setNormalUnitNormal(s.prevPt, currPt, s.resScale, s.radius)
	if !ok {
		if s.cap == capButt {
			return Point{}, Point{}, false
		}
		normal = Point{s.radius, 0}
		unitNormal = Point{1, 0}
	}
	if s.segmentCount == 0 {
		s.firstNormal = normal
		s.firstUnitNormal = unitNormal
		s.firstOuterPt = s.prevPt.add(normal)
		s.outer.MoveTo(s.firstOuterPt.X, s.firstOuterPt.Y)
		s.inner.MoveTo(s.prevPt.X-normal.X, s.prevPt.Y-normal.Y)
	} else {
		s.joiner(s.join, s.prevUnitNormal, s.prevPt, unitNormal, s.prevIsLine, currIsLine)
	}
	s.prevIsLine = currIsLine
	return normal, unitNormal, true
}

func (s *pathStroker) postJoinTo(currPt, normal, unitNormal Point) {
	s.joinCompleted = true
	s.prevPt = currPt
	s.prevUnitNormal = unitNormal
	s.prevNormal = normal
	s.segmentCount++
}

// reversePathTo is SkPathBuilder::privateReversePathTo: it appends the last
// contour of src in reverse, without its first point.
func (p *Path) reversePathTo(src *Path) {
	if len(src.verbs) == 0 {
		return
	}
	pi := len(src.pts) - 1
	wi := len(src.conics)
	for vi := len(src.verbs) - 1; vi >= 0; vi-- {
		v := src.verbs[vi]
		switch v {
		case verbMove:
			return
		case verbLine:
			pi--
			p.LineTo(src.pts[pi].X, src.pts[pi].Y)
		case verbQuad:
			pi -= 2
			p.QuadTo(src.pts[pi+1].X, src.pts[pi+1].Y, src.pts[pi].X, src.pts[pi].Y)
		case verbConic:
			pi -= 2
			wi--
			p.ConicTo(src.pts[pi+1].X, src.pts[pi+1].Y, src.pts[pi].X, src.pts[pi].Y, src.conics[wi])
		case verbCubic:
			pi -= 3
			p.CubicTo(src.pts[pi+2].X, src.pts[pi+2].Y, src.pts[pi+1].X, src.pts[pi+1].Y, src.pts[pi].X, src.pts[pi].Y)
		case verbClose:
		}
	}
}

// addPath appends src (SkPathBuilder::addPath in append mode).
func (p *Path) addPath(src *Path) {
	if len(src.verbs) == 0 {
		return
	}
	if n := len(p.verbs); n > 0 && p.verbs[n-1] == verbMove {
		p.verbs = p.verbs[:n-1]
		p.pts = p.pts[:len(p.pts)-1]
	}
	base := len(p.pts)
	last := 0
	pi := 0
	for _, v := range src.verbs {
		switch v {
		case verbMove:
			last = pi
			pi++
		case verbLine:
			pi++
		case verbQuad, verbConic:
			pi += 2
		case verbCubic:
			pi += 3
		}
	}
	p.lastMoveIdx = base + last
	p.verbs = append(p.verbs, src.verbs...)
	p.pts = append(p.pts, src.pts...)
	p.conics = append(p.conics, src.conics...)
	p.segMask |= src.segMask
	p.convex = convexityUnknown
}

func (s *pathStroker) finishContour(closeIt, currIsLine bool) {
	if s.segmentCount > 0 {
		if closeIt {
			s.joiner(s.join, s.prevUnitNormal, s.prevPt, s.firstUnitNormal, s.prevIsLine, currIsLine)
			s.outer.Close()
			if pt, ok := s.inner.lastPt(); ok {
				s.outer.MoveTo(pt.X, pt.Y)
				s.outer.reversePathTo(s.inner)
				s.outer.Close()
			}
		} else {
			if pt, ok := s.inner.lastPt(); ok {
				s.capper(s.outer, s.prevPt, s.prevNormal, pt, currIsLine)
				s.outer.reversePathTo(s.inner)
				s.capper(s.outer, s.firstPt, Point{-s.firstNormal.X, -s.firstNormal.Y}, s.firstOuterPt, s.prevIsLine)
				s.outer.Close()
			}
		}
		if !s.cusper.isEmpty() {
			s.outer.addPath(s.cusper)
			s.cusper.Reset()
		}
	}
	s.inner.Reset()
	s.segmentCount = -1
	s.firstOuterPtIndex = len(s.outer.pts)
}

func (s *pathStroker) moveTo(pt Point) {
	if s.segmentCount > 0 {
		s.finishContour(false, false)
	}
	s.segmentCount = 0
	s.firstPt, s.prevPt = pt, pt
	s.joinCompleted = false
}

func (s *pathStroker) lineToPts(currPt, normal Point) {
	s.outer.LineTo(currPt.X+normal.X, currPt.Y+normal.Y)
	s.inner.LineTo(currPt.X-normal.X, currPt.Y-normal.Y)
}

// strokeIter is SkPath::Iter without forced close: a close yields the line
// back to the start when the contour is open at that point.
type strokeIter struct {
	p          *Path
	vi, pi, wi int
	moveTo     Point
	lastPt     Point
	closeLine  bool
	weight     float32
}

// next returns the verb and its points. ok is false at the end.
func (it *strokeIter) next(pts *[4]Point) (pathVerb, bool) {
	if it.vi == len(it.p.verbs) {
		return 0, false
	}
	verb := it.p.verbs[it.vi]
	it.vi++
	switch verb {
	case verbMove:
		if it.vi == len(it.p.verbs) {
			return 0, false
		}
		it.moveTo = it.p.pts[it.pi]
		pts[0] = it.moveTo
		it.pi++
		it.lastPt = it.moveTo
	case verbLine:
		pts[0] = it.lastPt
		pts[1] = it.p.pts[it.pi]
		it.lastPt = pts[1]
		it.closeLine = false
		it.pi++
	case verbConic, verbQuad:
		if verb == verbConic {
			it.weight = it.p.conics[it.wi]
			it.wi++
		}
		pts[0] = it.lastPt
		pts[1], pts[2] = it.p.pts[it.pi], it.p.pts[it.pi+1]
		it.lastPt = pts[2]
		it.pi += 2
	case verbCubic:
		pts[0] = it.lastPt
		pts[1], pts[2], pts[3] = it.p.pts[it.pi], it.p.pts[it.pi+1], it.p.pts[it.pi+2]
		it.lastPt = pts[3]
		it.pi += 3
	case verbClose:
		if it.lastPt != it.moveTo {
			nan := func(v float32) bool { return v != v }
			if !(nan(it.lastPt.X) || nan(it.lastPt.Y) || nan(it.moveTo.X) || nan(it.moveTo.Y)) {
				pts[0] = it.lastPt
				pts[1] = it.moveTo
				it.lastPt = it.moveTo
				it.closeLine = true
				it.vi--
				return verbLine, true
			}
		} else {
			pts[0] = it.moveTo
		}
		it.lastPt = it.moveTo
	}
	return verb, true
}

// hasValidTangent is has_valid_tangent: a copy of the iterator looks for a
// segment with a length before the contour ends.
func hasValidTangent(it strokeIter) bool {
	var pts [4]Point
	for {
		verb, ok := it.next(&pts)
		if !ok {
			return false
		}
		switch verb {
		case verbMove:
			return false
		case verbLine:
			if pts[0] == pts[1] {
				continue
			}
			return true
		case verbQuad, verbConic:
			if pts[0] == pts[1] && pts[0] == pts[2] {
				continue
			}
			return true
		case verbCubic:
			if pts[0] == pts[1] && pts[0] == pts[2] && pts[0] == pts[3] {
				continue
			}
			return true
		case verbClose:
			return false
		}
	}
}

func (s *pathStroker) lineTo(currPt Point, it *strokeIter) {
	tol := float32(scalarNearlyZero * s.invResScale)
	teenyLine := abs32f(s.prevPt.X-currPt.X) <= tol && abs32f(s.prevPt.Y-currPt.Y) <= tol
	if s.cap == capButt && teenyLine {
		return
	}
	if teenyLine && (s.joinCompleted || (it != nil && hasValidTangent(*it))) {
		return
	}
	normal, unitNormal, ok := s.preJoinTo(currPt, true)
	if !ok {
		return
	}
	s.lineToPts(currPt, normal)
	s.postJoinTo(currPt, normal, unitNormal)
}

func (s *pathStroker) setQuadEndNormal(quad []Point, normalAB, unitNormalAB Point) (normalBC, unitNormalBC Point) {
	n, u, ok := setNormalUnitNormal(quad[1], quad[2], s.resScale, s.radius)
	if !ok {
		return normalAB, unitNormalAB
	}
	return n, u
}

func (s *pathStroker) setCubicEndNormal(cubic []Point, normalAB, unitNormalAB Point) (normalCD, unitNormalCD Point) {
	ab := cubic[1].sub(cubic[0])
	cd := cubic[3].sub(cubic[2])
	degenerateAB := degenerateVector(ab)
	degenerateCD := degenerateVector(cd)
	if degenerateAB && degenerateCD {
		return normalAB, unitNormalAB
	}
	if degenerateAB {
		ab = cubic[2].sub(cubic[0])
		degenerateAB = degenerateVector(ab)
	}
	if degenerateCD {
		cd = cubic[3].sub(cubic[1])
		degenerateCD = degenerateVector(cd)
	}
	if degenerateAB || degenerateCD {
		return normalAB, unitNormalAB
	}
	n, u, _ := setNormalUnitNormalVec(cd, s.radius)
	return n, u
}

func (s *pathStroker) initQuad(st strokeType, q *quadConstruct, tStart, tEnd float32) {
	s.stype = st
	s.foundTangents = false
	s.recursionDepth = 0
	q.init(tStart, tEnd)
}

func distanceToSqd(a, b Point) float32 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return float32(dx*dx) + float32(dy*dy)
}

func lengthSqd(v Point) float32 { return float32(v.X*v.X) + float32(v.Y*v.Y) }

// ptToLine is pt_to_line: the squared distance from a point to a segment.
func ptToLine(pt, lineStart, lineEnd Point) float32 {
	dxy := lineEnd.sub(lineStart)
	ab0 := pt.sub(lineStart)
	numer := dot(dxy, ab0)
	denom := dot(dxy, dxy)
	t := numer / denom
	if t >= 0 && t <= 1 {
		hit := Point{
			float32(lineStart.X*(1-t)) + float32(lineEnd.X*t),
			float32(lineStart.Y*(1-t)) + float32(lineEnd.Y*t),
		}
		return distanceToSqd(hit, pt)
	}
	return distanceToSqd(pt, lineStart)
}

func ptToTangentLine(pt, lineStart, tangent Point) float32 {
	ab0 := pt.sub(lineStart)
	numer := dot(tangent, ab0)
	denom := dot(tangent, tangent)
	t := numer / denom
	if t >= 0 && t <= 1 {
		hit := Point{lineStart.X + float32(tangent.X*t), lineStart.Y + float32(tangent.Y*t)}
		return distanceToSqd(hit, pt)
	}
	return distanceToSqd(pt, lineStart)
}

func cubicInLine(cubic []Point) bool {
	ptMax := float32(-1)
	outer1, outer2 := 0, 0
	for index := 0; index < 3; index++ {
		for inner := index + 1; inner < 4; inner++ {
			d := cubic[inner].sub(cubic[index])
			testMax := max(abs32f(d.X), abs32f(d.Y))
			if ptMax < testMax {
				outer1, outer2 = index, inner
				ptMax = testMax
			}
		}
	}
	mid1 := (1 + (2 >> uint(outer2))) >> uint(outer1)
	mid2 := outer1 ^ outer2 ^ mid1
	lineSlop := float32(float32(ptMax*ptMax) * 0.00001)
	return ptToLine(cubic[mid1], cubic[outer1], cubic[outer2]) <= lineSlop &&
		ptToLine(cubic[mid2], cubic[outer1], cubic[outer2]) <= lineSlop
}

func quadInLine(quad []Point) bool {
	ptMax := float32(-1)
	outer1, outer2 := 0, 0
	for index := 0; index < 2; index++ {
		for inner := index + 1; inner < 3; inner++ {
			d := quad[inner].sub(quad[index])
			testMax := max(abs32f(d.X), abs32f(d.Y))
			if ptMax < testMax {
				outer1, outer2 = index, inner
				ptMax = testMax
			}
		}
	}
	mid := outer1 ^ outer2 ^ 3
	const curvatureSlop = float32(0.000005)
	lineSlop := float32(float32(ptMax*ptMax) * curvatureSlop)
	return ptToLine(quad[mid], quad[outer1], quad[outer2]) <= lineSlop
}

// findQuadMaxCurvature is SkFindQuadMaxCurvature.
func findQuadMaxCurvature(src []Point) float32 {
	ax := src[1].X - src[0].X
	ay := src[1].Y - src[0].Y
	bx := src[0].X - src[1].X - src[1].X + src[2].X
	by := src[0].Y - src[1].Y - src[1].Y + src[2].Y
	numer := -(float32(ax*bx) + float32(ay*by))
	denom := float32(bx*bx) + float32(by*by)
	if denom < 0 {
		numer = -numer
		denom = -denom
	}
	if numer <= 0 {
		return 0
	}
	if numer >= denom {
		return 1
	}
	return numer / denom
}

// findCubicMaxCurvature is SkFindCubicMaxCurvature.
func findCubicMaxCurvature(src []Point, ts *[3]float32) int {
	cx := formulateF1DotF2(src[0].X, src[1].X, src[2].X, src[3].X)
	cy := formulateF1DotF2(src[0].Y, src[1].Y, src[2].Y, src[3].Y)
	for i := range cx {
		cx[i] += cy[i]
	}
	return solveCubicPoly(cx, ts)
}

// evalCubicDerivative is eval_cubic_derivative.
func evalCubicDerivative(src []Point, t float32) Point {
	p0, p1, p2, p3 := src[0], src[1], src[2], src[3]
	ax := p3.X + float32(3*(p1.X-p2.X)) - p0.X
	ay := p3.Y + float32(3*(p1.Y-p2.Y)) - p0.Y
	bx := float32(2 * (p2.X - float32(2*p1.X) + p0.X))
	by := float32(2 * (p2.Y - float32(2*p1.Y) + p0.Y))
	cx := p1.X - p0.X
	cy := p1.Y - p0.Y
	return Point{float32((float32(ax*t)+bx)*t) + cx, float32((float32(ay*t)+by)*t) + cy}
}

// evalCubicAt is SkEvalCubicAt with the position and the tangent.
func evalCubicAt(src []Point, t float32) (loc, tangent Point) {
	loc = evalCubicPosAt(src, t)
	if (t == 0 && src[0] == src[1]) || (t == 1 && src[2] == src[3]) {
		if t == 0 {
			tangent = src[2].sub(src[0])
		} else {
			tangent = src[3].sub(src[1])
		}
		if tangent.X == 0 && tangent.Y == 0 {
			tangent = src[3].sub(src[0])
		}
	} else {
		tangent = evalCubicDerivative(src, t)
	}
	return loc, tangent
}

// evalConicAt is SkConic::evalAt with the position and the tangent.
func (c *conic) evalAt(t float32) (pt, tangent Point) {
	p0, p1, p2 := c.pts[0], c.pts[1], c.pts[2]
	w := c.w
	{
		p1w := Point{float32(p1.X * w), float32(p1.Y * w)}
		nax := p2.X - float32(2*p1w.X) + p0.X
		nay := p2.Y - float32(2*p1w.Y) + p0.Y
		nbx := float32(2 * (p1w.X - p0.X))
		nby := float32(2 * (p1w.Y - p0.Y))
		db := float32(2 * (w - 1))
		da := 0 - db
		denom := float32((float32(da*t)+db)*t) + 1
		pt = Point{
			(float32((float32(nax*t)+nbx)*t) + p0.X) / denom,
			(float32((float32(nay*t)+nby)*t) + p0.Y) / denom,
		}
	}
	if (t == 0 && p0 == p1) || (t == 1 && p1 == p2) {
		return pt, p2.sub(p0)
	}
	p20 := p2.sub(p0)
	p10 := p1.sub(p0)
	cc := Point{float32(w * p10.X), float32(w * p10.Y)}
	a := Point{float32(w*p20.X) - p20.X, float32(w*p20.Y) - p20.Y}
	b := Point{p20.X - cc.X - cc.X, p20.Y - cc.Y - cc.Y}
	tangent = Point{float32((float32(a.X*t)+b.X)*t) + cc.X, float32((float32(a.Y*t)+b.Y)*t) + cc.Y}
	return pt, tangent
}

// findCubicInflections is SkFindCubicInflections.
func findCubicInflections(src []Point, ts *[2]float32) int {
	ax := src[1].X - src[0].X
	ay := src[1].Y - src[0].Y
	bx := src[2].X - float32(2*src[1].X) + src[0].X
	by := src[2].Y - float32(2*src[1].Y) + src[0].Y
	cx := src[3].X + float32(3*(src[1].X-src[2].X)) - src[0].X
	cy := src[3].Y + float32(3*(src[1].Y-src[2].Y)) - src[0].Y
	return findUnitQuadRoots(float32(bx*cy)-float32(by*cx), float32(ax*cy)-float32(ay*cx), float32(ax*by)-float32(ay*bx), ts)
}

func onSameSide(src []Point, testIndex, lineIndex int) bool {
	origin := src[lineIndex]
	line := src[lineIndex+1].sub(origin)
	var crosses [2]float32
	for i := 0; i < 2; i++ {
		testLine := src[testIndex+i].sub(origin)
		crosses[i] = cross(line, testLine)
	}
	return float32(crosses[0]*crosses[1]) >= 0
}

// findCubicCusp is SkFindCubicCusp.
func findCubicCusp(src []Point) float32 {
	if src[0] == src[1] || src[2] == src[3] {
		return -1
	}
	if onSameSide(src, 0, 2) || onSameSide(src, 2, 0) {
		return -1
	}
	var maxCurvature [3]float32
	roots := findCubicMaxCurvature(src, &maxCurvature)
	for i := 0; i < roots; i++ {
		testT := maxCurvature[i]
		if 0 >= testT || testT >= 1 {
			continue
		}
		dPt := evalCubicDerivative(src, testT)
		dPtMagnitude := lengthSqd(dPt)
		precision := float32((distanceToSqd(src[1], src[0]) + distanceToSqd(src[2], src[1]) + distanceToSqd(src[3], src[2])) * 1e-8)
		if dPtMagnitude < precision {
			return testT
		}
	}
	return -1
}

func checkCubicLinear(cubic []Point, reduction *[3]Point) (reductionType, Point) {
	degenerateAB := degenerateVector(cubic[1].sub(cubic[0]))
	degenerateBC := degenerateVector(cubic[2].sub(cubic[1]))
	degenerateCD := degenerateVector(cubic[3].sub(cubic[2]))
	if degenerateAB && degenerateBC && degenerateCD {
		return reductionPoint, Point{}
	}
	n := 0
	for _, d := range []bool{degenerateAB, degenerateBC, degenerateCD} {
		if d {
			n++
		}
	}
	if n == 2 {
		return reductionLine, Point{}
	}
	if !cubicInLine(cubic) {
		if degenerateAB {
			return reductionQuad, cubic[2]
		}
		return reductionQuad, cubic[1]
	}
	var ts [3]float32
	count := findCubicMaxCurvature(cubic, &ts)
	rCount := 0
	for i := 0; i < count; i++ {
		t := ts[i]
		if 0 >= t || t >= 1 {
			continue
		}
		reduction[rCount] = evalCubicPosAt(cubic, t)
		if reduction[rCount] != cubic[0] && reduction[rCount] != cubic[3] {
			rCount++
		}
	}
	if rCount == 0 {
		return reductionLine, Point{}
	}
	return reductionQuad + reductionType(rCount), Point{}
}

func checkConicLinear(c *conic, reduction *Point) reductionType {
	degenerateAB := degenerateVector(c.pts[1].sub(c.pts[0]))
	degenerateBC := degenerateVector(c.pts[2].sub(c.pts[1]))
	if degenerateAB && degenerateBC {
		return reductionPoint
	}
	if degenerateAB || degenerateBC {
		return reductionLine
	}
	if !quadInLine(c.pts[:]) {
		return reductionQuad
	}
	t := findQuadMaxCurvature(c.pts[:])
	if t == 0 || t != t {
		return reductionLine
	}
	*reduction, _ = c.evalAt(t)
	return reductionDegenerate
}

func checkQuadLinear(quad []Point, reduction *Point) reductionType {
	degenerateAB := degenerateVector(quad[1].sub(quad[0]))
	degenerateBC := degenerateVector(quad[2].sub(quad[1]))
	if degenerateAB && degenerateBC {
		return reductionPoint
	}
	if degenerateAB || degenerateBC {
		return reductionLine
	}
	if !quadInLine(quad) {
		return reductionQuad
	}
	t := findQuadMaxCurvature(quad)
	if t == 0 || t == 1 {
		return reductionLine
	}
	*reduction = evalQuadAt(quad, t)
	return reductionDegenerate
}

func (s *pathStroker) setRayPts(tPt Point, dxy *Point) (onPt, tangent Point) {
	d, ok := setLength(*dxy, s.radius)
	if !ok {
		d = Point{s.radius, 0}
	}
	*dxy = d
	axisFlip := float32(s.stype)
	onPt.X = tPt.X + float32(axisFlip*d.Y)
	onPt.Y = tPt.Y - float32(axisFlip*d.X)
	return onPt, d
}

func (s *pathStroker) conicPerpRay(c *conic, t float32) (tPt, onPt, tangent Point) {
	tPt, dxy := c.evalAt(t)
	if dxy.isZero() {
		dxy = c.pts[2].sub(c.pts[0])
	}
	onPt, tangent = s.setRayPts(tPt, &dxy)
	return tPt, onPt, tangent
}

func (s *pathStroker) conicQuadEnds(c *conic, q *quadConstruct) {
	if !q.startSet {
		_, q.quad[0], q.tangentStart = s.conicPerpRay(c, q.startT)
		q.startSet = true
	}
	if !q.endSet {
		_, q.quad[2], q.tangentEnd = s.conicPerpRay(c, q.endT)
		q.endSet = true
	}
}

func (s *pathStroker) cubicPerpRay(cubic []Point, t float32) (tPt, onPt, tangent Point) {
	tPt, dxy := evalCubicAt(cubic, t)
	if dxy.isZero() {
		cPts := cubic
		var chopped [7]Point
		if nearlyZero(t) {
			dxy = cubic[2].sub(cubic[0])
		} else if nearlyZero(1 - t) {
			dxy = cubic[3].sub(cubic[1])
		} else {
			chopCubicAt(cubic, chopped[:], t)
			dxy = chopped[3].sub(chopped[2])
			if dxy.isZero() {
				dxy = chopped[3].sub(chopped[1])
				cPts = chopped[:]
			}
		}
		if dxy.isZero() {
			dxy = cPts[3].sub(cPts[0])
		}
	}
	onPt, tangent = s.setRayPts(tPt, &dxy)
	return tPt, onPt, tangent
}

func (s *pathStroker) cubicQuadEnds(cubic []Point, q *quadConstruct) {
	if !q.startSet {
		_, q.quad[0], q.tangentStart = s.cubicPerpRay(cubic, q.startT)
		q.startSet = true
	}
	if !q.endSet {
		_, q.quad[2], q.tangentEnd = s.cubicPerpRay(cubic, q.endT)
		q.endSet = true
	}
}

func (s *pathStroker) quadPerpRay(quad []Point, t float32) (tPt, onPt, tangent Point) {
	tPt = evalQuadAt(quad, t)
	dxy := evalQuadTangentAt(quad, t)
	if dxy.isZero() {
		dxy = quad[2].sub(quad[0])
	}
	onPt, tangent = s.setRayPts(tPt, &dxy)
	return tPt, onPt, tangent
}

func (s *pathStroker) intersectRay(q *quadConstruct, rayType intersectRayType) resultType {
	start := q.quad[0]
	end := q.quad[2]
	aLen := q.tangentStart
	bLen := q.tangentEnd
	denom := cross(aLen, bLen)
	if denom == 0 || !isFinite32(denom) {
		q.oppositeTangents = dot(aLen, bLen) < 0
		return resultDegenerate
	}
	q.oppositeTangents = false
	ab0 := start.sub(end)
	numerA := cross(bLen, ab0)
	numerB := cross(aLen, ab0)
	if (numerA >= 0) == (numerB >= 0) {
		dist1 := ptToTangentLine(start, end, q.tangentEnd)
		dist2 := ptToTangentLine(end, start, q.tangentStart)
		if max(dist1, dist2) <= s.invResScaleSquared {
			return resultDegenerate
		}
		return resultSplit
	}
	numerA /= denom
	validDivide := numerA > numerA-1
	if validDivide {
		if rayType == ctrlPtRay {
			q.quad[1] = Point{start.X + float32(q.tangentStart.X*numerA), start.Y + float32(q.tangentStart.Y*numerA)}
		}
		return resultQuad
	}
	q.oppositeTangents = dot(aLen, bLen) < 0
	return resultDegenerate
}

func (s *pathStroker) tangentsMeet(cubic []Point, q *quadConstruct) resultType {
	s.cubicQuadEnds(cubic, q)
	return s.intersectRay(q, resultTypeRay)
}

func intersectQuadRay(line *[2]Point, quad []Point, roots *[2]float32) int {
	vec := line[1].sub(line[0])
	var r [3]float32
	for n := 0; n < 3; n++ {
		r[n] = cross(vec, quad[n].sub(line[0]))
	}
	a, b, c := r[2], r[1], r[0]
	a += c - float32(2*b)
	b -= c
	return findUnitQuadRoots(a, float32(2*b), c, roots)
}

func (s *pathStroker) ptInQuadBounds(quad []Point, pt Point) bool {
	xMin := min(quad[0].X, quad[1].X, quad[2].X)
	if pt.X+s.invResScale < xMin {
		return false
	}
	xMax := max(quad[0].X, quad[1].X, quad[2].X)
	if pt.X-s.invResScale > xMax {
		return false
	}
	yMin := min(quad[0].Y, quad[1].Y, quad[2].Y)
	if pt.Y+s.invResScale < yMin {
		return false
	}
	yMax := max(quad[0].Y, quad[1].Y, quad[2].Y)
	return !(pt.Y-s.invResScale > yMax)
}

func pointsWithinDist(nearPt, farPt Point, limit float32) bool {
	return distanceToSqd(nearPt, farPt) <= float32(limit*limit)
}

func sharpAngle(quad []Point) bool {
	smaller := quad[1].sub(quad[0])
	larger := quad[1].sub(quad[2])
	smallerLen := lengthSqd(smaller)
	largerLen := lengthSqd(larger)
	if smallerLen > largerLen {
		smaller, larger = larger, smaller
		largerLen = smallerLen
	}
	sm, ok := setLength(smaller, largerLen)
	if !ok {
		return false
	}
	return dot(sm, larger) > 0
}

func (s *pathStroker) strokeCloseEnough(stroke []Point, ray *[2]Point, q *quadConstruct) resultType {
	strokeMid := evalQuadAt(stroke, 0.5)
	if pointsWithinDist(ray[0], strokeMid, s.invResScale) {
		if sharpAngle(q.quad[:]) {
			return resultSplit
		}
		return resultQuad
	}
	if !s.ptInQuadBounds(stroke, ray[0]) {
		return resultSplit
	}
	var roots [2]float32
	rootCount := intersectQuadRay(ray, stroke, &roots)
	if rootCount != 1 {
		return resultSplit
	}
	quadPt := evalQuadAt(stroke, roots[0])
	errv := float32(s.invResScale * (1 - float32(abs32f(roots[0]-0.5)*2)))
	if pointsWithinDist(ray[0], quadPt, errv) {
		if sharpAngle(q.quad[:]) {
			return resultSplit
		}
		return resultQuad
	}
	return resultSplit
}

func (s *pathStroker) compareQuadCubic(cubic []Point, q *quadConstruct) resultType {
	s.cubicQuadEnds(cubic, q)
	rt := s.intersectRay(q, ctrlPtRay)
	if rt != resultQuad {
		return rt
	}
	var ray [2]Point
	ray[1], ray[0], _ = s.cubicPerpRay(cubic, q.midT)
	return s.strokeCloseEnough(q.quad[:], &ray, q)
}

func (s *pathStroker) compareQuadConic(c *conic, q *quadConstruct) resultType {
	s.conicQuadEnds(c, q)
	rt := s.intersectRay(q, ctrlPtRay)
	if rt != resultQuad {
		return rt
	}
	var ray [2]Point
	ray[1], ray[0], _ = s.conicPerpRay(c, q.midT)
	return s.strokeCloseEnough(q.quad[:], &ray, q)
}

func (s *pathStroker) compareQuadQuad(quad []Point, q *quadConstruct) resultType {
	if !q.startSet {
		_, q.quad[0], q.tangentStart = s.quadPerpRay(quad, q.startT)
		q.startSet = true
	}
	if !q.endSet {
		_, q.quad[2], q.tangentEnd = s.quadPerpRay(quad, q.endT)
		q.endSet = true
	}
	rt := s.intersectRay(q, ctrlPtRay)
	if rt != resultQuad {
		return rt
	}
	var ray [2]Point
	ray[1], ray[0], _ = s.quadPerpRay(quad, q.midT)
	return s.strokeCloseEnough(q.quad[:], &ray, q)
}

func (s *pathStroker) sink() *Path {
	if s.stype == outerStroke {
		return s.outer
	}
	return s.inner
}

func (s *pathStroker) addDegenerateLine(q *quadConstruct) {
	s.sink().LineTo(q.quad[2].X, q.quad[2].Y)
}

func (s *pathStroker) cubicMidOnLine(cubic []Point, q *quadConstruct) bool {
	_, strokeMid, _ := s.cubicPerpRay(cubic, q.midT)
	dist := ptToLine(strokeMid, q.quad[0], q.quad[2])
	return dist < s.invResScaleSquared
}

func (s *pathStroker) cubicStroke(cubic []Point, q *quadConstruct) bool {
	if !s.foundTangents {
		rt := s.tangentsMeet(cubic, q)
		if rt != resultQuad {
			if (rt == resultDegenerate || pointsWithinDist(q.quad[0], q.quad[2], s.invResScale)) && s.cubicMidOnLine(cubic, q) {
				s.addDegenerateLine(q)
				return true
			}
		} else {
			s.foundTangents = true
		}
	}
	if s.foundTangents {
		rt := s.compareQuadCubic(cubic, q)
		if rt == resultQuad {
			s.sink().QuadTo(q.quad[1].X, q.quad[1].Y, q.quad[2].X, q.quad[2].Y)
			return true
		}
		if rt == resultDegenerate {
			if !q.oppositeTangents {
				s.addDegenerateLine(q)
				return true
			}
		}
	}
	if !q.quad[2].isFinite() {
		return false
	}
	s.recursionDepth++
	limit := recursiveLimits[tangentRecursiveLimit]
	if s.foundTangents {
		limit = recursiveLimits[cubicRecursiveLimit]
	}
	if s.recursionDepth > limit {
		s.addDegenerateLine(q)
		s.recursionDepth--
		return true
	}
	var half quadConstruct
	if !half.initWithStart(q) {
		s.addDegenerateLine(q)
		s.recursionDepth--
		return true
	}
	if !s.cubicStroke(cubic, &half) {
		return false
	}
	if !half.initWithEnd(q) {
		s.addDegenerateLine(q)
		s.recursionDepth--
		return true
	}
	if !s.cubicStroke(cubic, &half) {
		return false
	}
	s.recursionDepth--
	return true
}

func (s *pathStroker) conicStroke(c *conic, q *quadConstruct) bool {
	rt := s.compareQuadConic(c, q)
	if rt == resultQuad {
		s.sink().QuadTo(q.quad[1].X, q.quad[1].Y, q.quad[2].X, q.quad[2].Y)
		return true
	}
	if rt == resultDegenerate {
		s.addDegenerateLine(q)
		return true
	}
	if !q.quad[2].isFinite() {
		return false
	}
	s.recursionDepth++
	if s.recursionDepth > recursiveLimits[conicRecursiveLimit] {
		s.addDegenerateLine(q)
		s.recursionDepth--
		return true
	}
	var half quadConstruct
	if !half.initWithStart(q) {
		s.addDegenerateLine(q)
		s.recursionDepth--
		return true
	}
	if !s.conicStroke(c, &half) {
		return false
	}
	if !half.initWithEnd(q) {
		s.addDegenerateLine(q)
		s.recursionDepth--
		return true
	}
	if !s.conicStroke(c, &half) {
		return false
	}
	s.recursionDepth--
	return true
}

func (s *pathStroker) quadStroke(quad []Point, q *quadConstruct) bool {
	rt := s.compareQuadQuad(quad, q)
	if rt == resultQuad {
		s.sink().QuadTo(q.quad[1].X, q.quad[1].Y, q.quad[2].X, q.quad[2].Y)
		return true
	}
	if rt == resultDegenerate {
		s.addDegenerateLine(q)
		return true
	}
	if !q.quad[2].isFinite() {
		return false
	}
	s.recursionDepth++
	if s.recursionDepth > recursiveLimits[quadRecursiveLimit] {
		s.addDegenerateLine(q)
		s.recursionDepth--
		return true
	}
	var half quadConstruct
	if !half.initWithStart(q) {
		s.addDegenerateLine(q)
		s.recursionDepth--
		return true
	}
	if !s.quadStroke(quad, &half) {
		return false
	}
	if !half.initWithEnd(q) {
		s.addDegenerateLine(q)
		s.recursionDepth--
		return true
	}
	if !s.quadStroke(quad, &half) {
		return false
	}
	s.recursionDepth--
	return true
}

func (s *pathStroker) conicTo(pt1, pt2 Point, weight float32) {
	c := conic{pts: [3]Point{s.prevPt, pt1, pt2}, w: weight}
	var reduction Point
	rt := checkConicLinear(&c, &reduction)
	if rt == reductionPoint || rt == reductionLine {
		s.lineTo(pt2, nil)
		return
	}
	if rt == reductionDegenerate {
		s.lineTo(reduction, nil)
		saveJoin := s.join
		s.join = joinRound
		s.lineTo(pt2, nil)
		s.join = saveJoin
		return
	}
	normalAB, unitAB, ok := s.preJoinTo(pt1, false)
	if !ok {
		s.lineTo(pt2, nil)
		return
	}
	var q quadConstruct
	s.initQuad(outerStroke, &q, 0, 1)
	s.conicStroke(&c, &q)
	s.initQuad(innerStroke, &q, 0, 1)
	s.conicStroke(&c, &q)
	normalBC, unitBC := s.setQuadEndNormal(c.pts[:], normalAB, unitAB)
	s.postJoinTo(pt2, normalBC, unitBC)
}

func (s *pathStroker) quadTo(pt1, pt2 Point) {
	quad := [3]Point{s.prevPt, pt1, pt2}
	var reduction Point
	rt := checkQuadLinear(quad[:], &reduction)
	if rt == reductionPoint || rt == reductionLine {
		s.lineTo(pt2, nil)
		return
	}
	if rt == reductionDegenerate {
		s.lineTo(reduction, nil)
		saveJoin := s.join
		s.join = joinRound
		s.lineTo(pt2, nil)
		s.join = saveJoin
		return
	}
	normalAB, unitAB, ok := s.preJoinTo(pt1, false)
	if !ok {
		s.lineTo(pt2, nil)
		return
	}
	var q quadConstruct
	s.initQuad(outerStroke, &q, 0, 1)
	s.quadStroke(quad[:], &q)
	s.initQuad(innerStroke, &q, 0, 1)
	s.quadStroke(quad[:], &q)
	normalBC, unitBC := s.setQuadEndNormal(quad[:], normalAB, unitAB)
	s.postJoinTo(pt2, normalBC, unitBC)
}

func (s *pathStroker) cubicTo(pt1, pt2, pt3 Point) {
	cubic := [4]Point{s.prevPt, pt1, pt2, pt3}
	var reduction [3]Point
	rt, tangentPt := checkCubicLinear(cubic[:], &reduction)
	if rt == reductionPoint || rt == reductionLine {
		s.lineTo(pt3, nil)
		return
	}
	if rt >= reductionDegenerate && rt <= reductionDegenerate3 {
		s.lineTo(reduction[0], nil)
		saveJoin := s.join
		s.join = joinRound
		if rt >= reductionDegenerate2 {
			s.lineTo(reduction[1], nil)
		}
		if rt == reductionDegenerate3 {
			s.lineTo(reduction[2], nil)
		}
		s.lineTo(pt3, nil)
		s.join = saveJoin
		return
	}
	normalAB, unitAB, ok := s.preJoinTo(tangentPt, false)
	if !ok {
		s.lineTo(pt3, nil)
		return
	}
	var ts [2]float32
	count := findCubicInflections(cubic[:], &ts)
	lastT := float32(0)
	for index := 0; index <= count; index++ {
		nextT := float32(1)
		if index < count {
			nextT = ts[index]
		}
		var q quadConstruct
		s.initQuad(outerStroke, &q, lastT, nextT)
		s.cubicStroke(cubic[:], &q)
		s.initQuad(innerStroke, &q, lastT, nextT)
		s.cubicStroke(cubic[:], &q)
		lastT = nextT
	}
	cusp := findCubicCusp(cubic[:])
	if cusp > 0 {
		cuspLoc := evalCubicPosAt(cubic[:], cusp)
		// addCircle: an oval, clockwise, start index 1.
		s.cusper.AddOval(Rect{cuspLoc.X - s.radius, cuspLoc.Y - s.radius, cuspLoc.X + s.radius, cuspLoc.Y + s.radius}, 1)
	}
	normalCD, unitCD := s.setCubicEndNormal(cubic[:], normalAB, unitAB)
	s.postJoinTo(pt3, normalCD, unitCD)
}

// strokeParams are the stroke properties of SkStroke.
type strokeParams struct {
	width      float32
	miterLimit float32
	cap        strokeCap
	join       strokeJoin
	resScale   float32
}

// strokeScratch holds the temporary paths of the stroker.
type strokeScratch struct {
	inner, cusper Path
	stroker       pathStroker
}

// isClosedRect is SkPath::isRect with the closed flag and the direction.
func (p *Path) isClosedRect() (Rect, bool, bool) {
	r, ok := p.isRect()
	if !ok {
		return Rect{}, false, false
	}
	closed := false
	for _, v := range p.verbs {
		if v == verbClose {
			closed = true
		}
	}
	if !closed {
		return Rect{}, false, false
	}
	// The direction: the turn at the first corner.
	var pts []Point
	for i, q := range p.pts {
		if i == 0 || q != p.pts[i-1] {
			pts = append(pts, q)
		}
	}
	cw := true
	for i := 0; i+2 < len(pts); i++ {
		c := cross(pts[i+1].sub(pts[i]), pts[i+2].sub(pts[i+1]))
		if c != 0 {
			cw = c > 0
			break
		}
	}
	return r, true, cw
}

// strokeRect is SkStroke::strokeRect.
func strokeRect(orig Rect, dst *Path, cw bool, sp *strokeParams) {
	dst.Reset()
	radius := sp.width / 2
	if radius <= 0 {
		return
	}
	rw, rh := orig.width(), orig.height()
	if (rw < 0) != (rh < 0) {
		cw = !cw
	}
	rect := orig.sorted()
	rw, rh = rect.width(), rect.height()
	r := rect.outset(radius, radius)
	join := sp.join
	if join == joinMiter && sp.miterLimit < float32(math.Sqrt2) {
		join = joinBevel
	}
	addRectDir := func(rc Rect, cw bool) {
		if cw {
			dst.AddRect(rc, 0)
			return
		}
		// Counter-clockwise from the top left corner.
		dst.MoveTo(rc.L, rc.T)
		dst.LineTo(rc.L, rc.B)
		dst.LineTo(rc.R, rc.B)
		dst.LineTo(rc.R, rc.T)
		dst.Close()
	}
	switch join {
	case joinMiter:
		addRectDir(r, cw)
	case joinBevel:
		pts := [8]Point{
			{rect.L, r.T}, {rect.R, r.T}, {r.R, rect.T}, {r.R, rect.B},
			{rect.R, r.B}, {rect.L, r.B}, {r.L, rect.B}, {r.L, rect.T},
		}
		if !cw {
			for i, j := 0, 7; i < j; i, j = i+1, j-1 {
				pts[i], pts[j] = pts[j], pts[i]
			}
		}
		dst.MoveTo(pts[0].X, pts[0].Y)
		for _, q := range pts[1:] {
			dst.LineTo(q.X, q.Y)
		}
		dst.Close()
	case joinRound:
		if cw {
			// SkPath::RRect default start index for clockwise is 6.
			addRRectIndex(dst, r, radius, radius, true, 6)
		} else {
			addRRectIndex(dst, r, radius, radius, false, 7)
		}
	}
	if sp.width < min(rw, rh) {
		r = rect.outset(-radius, -radius)
		addRectDir(r, !cw)
	}
	dst.convex = convexityUnknown
}

// addRRectIndex is SkPathBuilder::addRRect(rrect, dir, index) for a rounded
// rectangle with equal corners.
func addRRectIndex(p *Path, r Rect, rx, ry float32, cw bool, index int) {
	if r.width() < rx+rx || r.height() < ry+ry {
		scale := min(r.width()/(rx+rx), r.height()/(ry+ry))
		rx = float32(rx * scale)
		ry = float32(ry * scale)
	}
	l, t, rr, b := r.L, r.T, r.R, r.B
	rrectPt := func(i int) Point {
		switch ((i % 8) + 8) % 8 {
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
	adv, radv := 1, 1
	if !cw {
		adv, radv = 7, 3
	}
	startsWithConic := (index&1 == 1) == cw
	rrectI := index
	nextRRect := func() Point {
		rrectI += adv
		return rrectPt(rrectI)
	}
	rectI := index / 2
	if !cw {
		rectI++
	}
	nextRect := func() Point {
		rectI += radv
		return rectCorner(r, rectI)
	}
	c := rrectPt(rrectI)
	p.MoveTo(c.X, c.Y)
	if startsWithConic {
		for i := 0; i < 3; i++ {
			ctrl := nextRect()
			end := nextRRect()
			p.ConicTo(ctrl.X, ctrl.Y, end.X, end.Y, scalarRoot2Over2)
			line := nextRRect()
			p.LineTo(line.X, line.Y)
		}
		ctrl := nextRect()
		end := nextRRect()
		p.ConicTo(ctrl.X, ctrl.Y, end.X, end.Y, scalarRoot2Over2)
	} else {
		for i := 0; i < 4; i++ {
			line := nextRRect()
			p.LineTo(line.X, line.Y)
			ctrl := nextRect()
			end := nextRRect()
			p.ConicTo(ctrl.X, ctrl.Y, end.X, end.Y, scalarRoot2Over2)
		}
	}
	p.Close()
}

// strokePath is SkStroke::strokePath: dst receives the outline of the
// stroke of src.
func strokePath(src, dst *Path, sp *strokeParams, scratch *strokeScratch) {
	dst.Reset()
	radius := sp.width / 2
	if radius <= 0 {
		return
	}
	if rect, closed, cw := src.isClosedRect(); closed {
		strokeRect(rect, dst, cw, sp)
		return
	}
	scratch.inner.Reset()
	scratch.cusper.Reset()
	s := &scratch.stroker
	s.init(radius, sp.miterLimit, sp.cap, sp.join, sp.resScale, dst, &scratch.inner, &scratch.cusper)

	it := strokeIter{p: src}
	lastSegment := verbMove
	var pts [4]Point
	for {
		verb, ok := it.next(&pts)
		if !ok {
			break
		}
		switch verb {
		case verbMove:
			s.moveTo(pts[0])
		case verbLine:
			s.lineTo(pts[1], &it)
			lastSegment = verbLine
		case verbQuad:
			s.quadTo(pts[1], pts[2])
			lastSegment = verbQuad
		case verbConic:
			s.conicTo(pts[1], pts[2], it.weight)
			lastSegment = verbConic
		case verbCubic:
			s.cubicTo(pts[1], pts[2], pts[3])
			lastSegment = verbCubic
		case verbClose:
			if sp.cap != capButt {
				if s.segmentCount == 0 {
					s.lineTo(s.firstPt, nil)
					lastSegment = verbLine
					continue
				}
				if isZeroLengthSincePoint(s.inner.pts, 0) && isZeroLengthSincePoint(s.outer.pts, s.firstOuterPtIndex) {
					lastSegment = verbLine
					continue
				}
			}
			s.finishContour(true, lastSegment == verbLine)
		}
	}
	s.finishContour(false, lastSegment == verbLine)
	dst.convex = convexityUnknown
}
