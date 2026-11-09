// Ported from skia/src/core/SkContourMeasure.cpp, SkPathMeasure.cpp,
// src/utils/SkDashPath.cpp and src/effects/SkDashPathEffect.cpp (Skia branch
// chrome/m156): the dash path effect.

package raster

import "math"

const (
	measLine = iota
	measQuad
	measCubic
	measConic

	maxTValue      = 0x3FFFFFFF
	cheapDistLimit = float32(0.5)

	measMaxRecursionDepth = 8
	maxDashCount          = float32(1000000)
)

type measSegment struct {
	distance float32
	ptIndex  int
	tValue   uint32
	segType  uint8
}

func (s *measSegment) scalarT() float32 {
	const maxTReciprocal = float32(1.0) / float32(maxTValue)
	return float32(float32(s.tValue) * maxTReciprocal)
}

// contourMeasure is SkContourMeasure.
type contourMeasure struct {
	segs     []measSegment
	pts      []Point
	length   float32
	isClosed bool
}

// measureIter is SkContourMeasureIter::Impl.
type measureIter struct {
	p          *Path
	vi, pi, wi int
	tolerance  float32
	cm         contourMeasure
}

func tspanBigEnough(tspan int) bool { return tspan>>10 != 0 }

func ptDistance(a, b Point) float32 {
	dx, dy := a.X-b.X, a.Y-b.Y
	mag2 := float32(dx*dx) + float32(dy*dy)
	if isFinite32(mag2) {
		return sqrt32(mag2)
	}
	xx, yy := float64(dx), float64(dy)
	return float32(math.Sqrt(float64(xx*xx) + float64(yy*yy)))
}

func scalarInterp(a, b, t float32) float32 { return a + float32((b-a)*t) }

func (m *measureIter) quadTooCurvy(pts []Point) bool {
	dx := float32(pts[1].X/2) - float32(float32((pts[0].X+pts[2].X)/2)/2)
	dy := float32(pts[1].Y/2) - float32(float32((pts[0].Y+pts[2].Y)/2)/2)
	return max(abs32f(dx), abs32f(dy)) > m.tolerance
}

func (m *measureIter) conicTooCurvy(first, mid, last Point) bool {
	midEnds := Point{float32((first.X + last.X) * 0.5), float32((first.Y + last.Y) * 0.5)}
	return max(abs32f(mid.X-midEnds.X), abs32f(mid.Y-midEnds.Y)) > m.tolerance
}

func (m *measureIter) cubicTooCurvy(pts []Point) bool {
	exceeds := func(pt Point, x, y float32) bool {
		return max(abs32f(x-pt.X), abs32f(y-pt.Y)) > m.tolerance
	}
	const third = float32(1.0) / 3
	const twoThirds = float32(1.0) * 2 / 3
	return exceeds(pts[1], scalarInterp(pts[0].X, pts[3].X, third), scalarInterp(pts[0].Y, pts[3].Y, third)) ||
		exceeds(pts[2], scalarInterp(pts[0].X, pts[3].X, twoThirds), scalarInterp(pts[0].Y, pts[3].Y, twoThirds))
}

func (m *measureIter) push(distance, d float32, ptIndex int, segType uint8, t int) float32 {
	prevD := distance
	distance += d
	if distance > prevD {
		m.cm.segs = append(m.cm.segs, measSegment{distance: distance, ptIndex: ptIndex, tValue: uint32(t), segType: segType})
	}
	return distance
}

func (m *measureIter) quadSegs(pts []Point, distance float32, mint, maxt, ptIndex, depth int) float32 {
	if depth < measMaxRecursionDepth && tspanBigEnough(maxt-mint) && m.quadTooCurvy(pts) {
		var tmp [5]Point
		halft := (mint + maxt) >> 1
		chopQuadAt(pts, &tmp, 0.5)
		depth++
		distance = m.quadSegs(tmp[:3], distance, mint, halft, ptIndex, depth)
		distance = m.quadSegs(tmp[2:], distance, halft, maxt, ptIndex, depth)
		return distance
	}
	return m.push(distance, ptDistance(pts[0], pts[2]), ptIndex, measQuad, maxt)
}

func (m *measureIter) conicSegs(c *conic, distance float32, mint int, minPt Point, maxt int, maxPt Point, ptIndex, depth int) float32 {
	halft := (mint + maxt) >> 1
	const maxTReciprocal = float32(1.0) / float32(maxTValue)
	halfPt, _ := c.evalAt(float32(float32(halft) * maxTReciprocal))
	if !halfPt.isFinite() {
		return distance
	}
	if depth < measMaxRecursionDepth && tspanBigEnough(maxt-mint) && m.conicTooCurvy(minPt, halfPt, maxPt) {
		depth++
		distance = m.conicSegs(c, distance, mint, minPt, halft, halfPt, ptIndex, depth)
		distance = m.conicSegs(c, distance, halft, halfPt, maxt, maxPt, ptIndex, depth)
		return distance
	}
	return m.push(distance, ptDistance(minPt, maxPt), ptIndex, measConic, maxt)
}

func (m *measureIter) cubicSegs(pts []Point, distance float32, mint, maxt, ptIndex, depth int) float32 {
	if depth < measMaxRecursionDepth && tspanBigEnough(maxt-mint) && m.cubicTooCurvy(pts) {
		var tmp [7]Point
		halft := (mint + maxt) >> 1
		chopCubicAt(pts, tmp[:], 0.5)
		depth++
		distance = m.cubicSegs(tmp[:4], distance, mint, halft, ptIndex, depth)
		distance = m.cubicSegs(tmp[3:], distance, halft, maxt, ptIndex, depth)
		return distance
	}
	return m.push(distance, ptDistance(pts[0], pts[3]), ptIndex, measCubic, maxt)
}

func (m *measureIter) hasNext() bool { return m.vi < len(m.p.verbs) }

// buildSegments is SkContourMeasureIter::Impl::buildSegments. It reports
// false when the contour has no length.
func (m *measureIter) buildSegments() bool {
	ptIndex := -1
	var distance float32
	haveSeenClose := false
	haveSeenMoveTo := false
	m.cm.segs = m.cm.segs[:0]
	m.cm.pts = m.cm.pts[:0]
	src := m.p.pts

	for ; m.vi < len(m.p.verbs); m.vi++ {
		verb := m.p.verbs[m.vi]
		if haveSeenMoveTo && verb == verbMove {
			break
		}
		switch verb {
		case verbMove:
			ptIndex++
			m.cm.pts = append(m.cm.pts, src[m.pi])
			m.pi++
			haveSeenMoveTo = true
		case verbLine:
			prevD := distance
			distance = m.push(distance, ptDistance(src[m.pi-1], src[m.pi]), ptIndex, measLine, maxTValue)
			if distance > prevD {
				m.cm.pts = append(m.cm.pts, src[m.pi])
				ptIndex++
			}
			m.pi++
		case verbQuad:
			prevD := distance
			distance = m.quadSegs(src[m.pi-1:m.pi+2], distance, 0, maxTValue, ptIndex, 0)
			if distance > prevD {
				m.cm.pts = append(m.cm.pts, src[m.pi], src[m.pi+1])
				ptIndex += 2
			}
			m.pi += 2
		case verbConic:
			c := conic{pts: [3]Point{src[m.pi-1], src[m.pi], src[m.pi+1]}, w: m.p.conics[m.wi]}
			m.wi++
			prevD := distance
			distance = m.conicSegs(&c, distance, 0, c.pts[0], maxTValue, c.pts[2], ptIndex, 0)
			if distance > prevD {
				m.cm.pts = append(m.cm.pts, Point{c.w, 0}, src[m.pi], src[m.pi+1])
				ptIndex += 3
			}
			m.pi += 2
		case verbCubic:
			prevD := distance
			distance = m.cubicSegs(src[m.pi-1:m.pi+3], distance, 0, maxTValue, ptIndex, 0)
			if distance > prevD {
				m.cm.pts = append(m.cm.pts, src[m.pi], src[m.pi+1], src[m.pi+2])
				ptIndex += 3
			}
			m.pi += 3
		case verbClose:
			haveSeenClose = true
		}
	}
	if !isFinite32(distance) || len(m.cm.segs) == 0 {
		return false
	}
	if haveSeenClose {
		prevD := distance
		firstPt := m.cm.pts[0]
		distance = m.push(distance, ptDistance(m.cm.pts[ptIndex], firstPt), ptIndex, measLine, maxTValue)
		if distance > prevD {
			m.cm.pts = append(m.cm.pts, firstPt)
		}
	}
	m.cm.length = distance
	m.cm.isClosed = haveSeenClose
	return true
}

// next is SkContourMeasureIter::next.
func (m *measureIter) next() bool {
	for m.hasNext() {
		if m.buildSegments() {
			return true
		}
	}
	return false
}

// distanceToSegment is SkContourMeasure::distanceToSegment.
func (cm *contourMeasure) distanceToSegment(distance float32) (int, float32) {
	segs := cm.segs
	count := len(segs)
	lo, hi := 0, count-1
	for lo < hi {
		mid := (hi + lo) >> 1
		if segs[mid].distance < distance {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	index := hi
	if segs[hi].distance < distance {
		index = hi + 1
		// The search returns ~(hi+1), and the caller folds it back.
		if index >= count {
			index = count - 1
		}
	}
	seg := &segs[index]
	var startT, startD float32
	if index > 0 {
		startD = segs[index-1].distance
		if segs[index-1].ptIndex == seg.ptIndex {
			startT = segs[index-1].scalarT()
		}
	}
	t := startT + float32(float32(seg.scalarT()-startT)*(distance-startD))/(seg.distance-startD)
	return index, t
}

func (cm *contourMeasure) computePos(seg *measSegment, t float32) Point {
	pts := cm.pts[seg.ptIndex:]
	switch seg.segType {
	case measLine:
		return Point{scalarInterp(pts[0].X, pts[1].X, t), scalarInterp(pts[0].Y, pts[1].Y, t)}
	case measQuad:
		return evalQuadAt(pts, t)
	case measConic:
		c := conic{pts: [3]Point{pts[0], pts[2], pts[3]}, w: pts[1].X}
		p, _ := c.evalAt(t)
		return p
	default:
		return evalCubicPosAt(pts, t)
	}
}

// chopAtT is SkConic::chopAt(t, dst).
func (c *conic) chopAtT(t float32, dst *[2]conic) bool {
	type p3 struct{ x, y, z float32 }
	tmp := [3]p3{
		{c.pts[0].X, c.pts[0].Y, 1},
		{float32(c.pts[1].X * c.w), float32(c.pts[1].Y * c.w), c.w},
		{c.pts[2].X, c.pts[2].Y, 1},
	}
	interp := func(a, b, cc float32) (float32, float32, float32) {
		ab := scalarInterp(a, b, t)
		bc := scalarInterp(b, cc, t)
		return ab, scalarInterp(ab, bc, t), bc
	}
	var tmp2 [3]p3
	tmp2[0].x, tmp2[1].x, tmp2[2].x = interp(tmp[0].x, tmp[1].x, tmp[2].x)
	tmp2[0].y, tmp2[1].y, tmp2[2].y = interp(tmp[0].y, tmp[1].y, tmp[2].y)
	tmp2[0].z, tmp2[1].z, tmp2[2].z = interp(tmp[0].z, tmp[1].z, tmp[2].z)
	project := func(p p3) Point { return Point{p.x / p.z, p.y / p.z} }
	dst[0].pts[0] = c.pts[0]
	dst[0].pts[1] = project(tmp2[0])
	dst[0].pts[2] = project(tmp2[1])
	dst[1].pts[0] = dst[0].pts[2]
	dst[1].pts[1] = project(tmp2[2])
	dst[1].pts[2] = c.pts[2]
	root := sqrt32(tmp2[1].z)
	dst[0].w = tmp2[0].z / root
	dst[1].w = tmp2[2].z / root
	for i := range dst {
		for _, p := range dst[i].pts {
			if !p.isFinite() {
				return false
			}
		}
		if !isFinite32(dst[i].w) {
			return false
		}
	}
	return true
}

// chopAtRange is SkConic::chopAt(t1, t2, dst).
func (c *conic) chopAtRange(t1, t2 float32) conic {
	if t1 == 0 || t2 == 1 {
		if t1 == 0 && t2 == 1 {
			return *c
		}
		var pair [2]conic
		t := t2
		idx := 0
		if t1 != 0 {
			t = t1
			idx = 1
		}
		if c.chopAtT(t, &pair) {
			return pair[idx]
		}
	}
	p0, p1, p2 := c.pts[0], c.pts[1], c.pts[2]
	w := c.w
	p1w := Point{float32(p1.X * w), float32(p1.Y * w)}
	nA := Point{p2.X - float32(2*p1w.X) + p0.X, p2.Y - float32(2*p1w.Y) + p0.Y}
	nB := Point{float32(2 * (p1w.X - p0.X)), float32(2 * (p1w.Y - p0.Y))}
	dB := float32(2 * (w - 1))
	dA := 0 - dB
	numer := func(t float32) Point {
		return Point{float32((float32(nA.X*t)+nB.X)*t) + p0.X, float32((float32(nA.Y*t)+nB.Y)*t) + p0.Y}
	}
	denom := func(t float32) float32 { return float32((float32(dA*t)+dB)*t) + 1 }
	aXY, aZZ := numer(t1), denom(t1)
	mid := (t1 + t2) / 2
	dXY, dZZ := numer(mid), denom(mid)
	cXY, cZZ := numer(t2), denom(t2)
	bXY := Point{float32(2*dXY.X) - float32((aXY.X+cXY.X)*0.5), float32(2*dXY.Y) - float32((aXY.Y+cXY.Y)*0.5)}
	bZZ := float32(2*dZZ) - float32((aZZ+cZZ)*0.5)
	var out conic
	out.pts[0] = Point{aXY.X / aZZ, aXY.Y / aZZ}
	out.pts[1] = Point{bXY.X / bZZ, bXY.Y / bZZ}
	out.pts[2] = Point{cXY.X / cZZ, cXY.Y / cZZ}
	out.w = bZZ / sqrt32(float32(aZZ*cZZ))
	return out
}

// segTo is SkContourMeasure_segTo.
func segTo(pts []Point, segType uint8, startT, stopT float32, dst *Path) {
	if startT == stopT {
		if !dst.isEmpty() {
			if last, ok := dst.lastPt(); ok {
				dst.LineTo(last.X, last.Y)
			}
		}
		return
	}
	var tmp0, tmp1 [7]Point
	switch segType {
	case measLine:
		if stopT == 1 {
			dst.LineTo(pts[1].X, pts[1].Y)
		} else {
			dst.LineTo(scalarInterp(pts[0].X, pts[1].X, stopT), scalarInterp(pts[0].Y, pts[1].Y, stopT))
		}
	case measQuad:
		var q0, q1 [5]Point
		if startT == 0 {
			if stopT == 1 {
				dst.QuadTo(pts[1].X, pts[1].Y, pts[2].X, pts[2].Y)
			} else {
				chopQuadAt(pts, &q0, stopT)
				dst.QuadTo(q0[1].X, q0[1].Y, q0[2].X, q0[2].Y)
			}
		} else {
			chopQuadAt(pts, &q0, startT)
			if stopT == 1 {
				dst.QuadTo(q0[3].X, q0[3].Y, q0[4].X, q0[4].Y)
			} else {
				chopQuadAt(q0[2:], &q1, (stopT-startT)/(1-startT))
				dst.QuadTo(q1[1].X, q1[1].Y, q1[2].X, q1[2].Y)
			}
		}
	case measConic:
		c := conic{pts: [3]Point{pts[0], pts[2], pts[3]}, w: pts[1].X}
		if startT == 0 {
			if stopT == 1 {
				dst.ConicTo(c.pts[1].X, c.pts[1].Y, c.pts[2].X, c.pts[2].Y, c.w)
			} else {
				var tmp [2]conic
				if c.chopAtT(stopT, &tmp) {
					dst.ConicTo(tmp[0].pts[1].X, tmp[0].pts[1].Y, tmp[0].pts[2].X, tmp[0].pts[2].Y, tmp[0].w)
				}
			}
		} else {
			if stopT == 1 {
				var tmp [2]conic
				if c.chopAtT(startT, &tmp) {
					dst.ConicTo(tmp[1].pts[1].X, tmp[1].pts[1].Y, tmp[1].pts[2].X, tmp[1].pts[2].Y, tmp[1].w)
				}
			} else {
				tmp := c.chopAtRange(startT, stopT)
				dst.ConicTo(tmp.pts[1].X, tmp.pts[1].Y, tmp.pts[2].X, tmp.pts[2].Y, tmp.w)
			}
		}
	case measCubic:
		if startT == 0 {
			if stopT == 1 {
				dst.CubicTo(pts[1].X, pts[1].Y, pts[2].X, pts[2].Y, pts[3].X, pts[3].Y)
			} else {
				chopCubicAt(pts, tmp0[:], stopT)
				dst.CubicTo(tmp0[1].X, tmp0[1].Y, tmp0[2].X, tmp0[2].Y, tmp0[3].X, tmp0[3].Y)
			}
		} else {
			chopCubicAt(pts, tmp0[:], startT)
			if stopT == 1 {
				dst.CubicTo(tmp0[4].X, tmp0[4].Y, tmp0[5].X, tmp0[5].Y, tmp0[6].X, tmp0[6].Y)
			} else {
				chopCubicAt(tmp0[3:], tmp1[:], (stopT-startT)/(1-startT))
				dst.CubicTo(tmp1[1].X, tmp1[1].Y, tmp1[2].X, tmp1[2].Y, tmp1[3].X, tmp1[3].Y)
			}
		}
	}
}

// getSegment is SkContourMeasure::getSegment.
func (cm *contourMeasure) getSegment(startD, stopD float32, dst *Path, startWithMoveTo bool) bool {
	if startD < 0 {
		startD = 0
	}
	if stopD > cm.length {
		stopD = cm.length
	}
	if !(startD <= stopD) || len(cm.segs) == 0 {
		return false
	}
	si, startT := cm.distanceToSegment(startD)
	if !isFinite32(startT) {
		return false
	}
	ei, stopT := cm.distanceToSegment(stopD)
	if !isFinite32(stopT) {
		return false
	}
	seg := &cm.segs[si]
	stopSeg := &cm.segs[ei]
	if startWithMoveTo {
		p := cm.computePos(seg, startT)
		dst.MoveTo(p.X, p.Y)
	}
	if seg.ptIndex == stopSeg.ptIndex {
		segTo(cm.pts[seg.ptIndex:], seg.segType, startT, stopT, dst)
	} else {
		for {
			segTo(cm.pts[seg.ptIndex:], seg.segType, startT, 1, dst)
			// Segment::Next: the next segment with another point index.
			ptIndex := seg.ptIndex
			for {
				si++
				seg = &cm.segs[si]
				if seg.ptIndex != ptIndex {
					break
				}
			}
			startT = 0
			if !(seg.ptIndex < stopSeg.ptIndex) {
				break
			}
		}
		segTo(cm.pts[seg.ptIndex:], seg.segType, 0, stopT, dst)
	}
	return true
}

// dashInfo holds the parameters of SkDashImpl.
type dashInfo struct {
	intervals         []float32
	phase             float32
	initialDashLength float32
	initialDashIndex  int
	intervalLength    float32
}

func scalarMod(x, y float32) float32 { return float32(math.Mod(float64(x), float64(y))) }

// validDashPath is SkDashPath::ValidDashPath.
func validDashPath(phase float32, intervals []float32) bool {
	if len(intervals) < 2 || len(intervals)%2 != 0 {
		return false
	}
	var length float32
	for _, v := range intervals {
		if v < 0 {
			return false
		}
		length += v
	}
	return length > 0 && isFinite32(phase) && isFinite32(length)
}

// newDash is SkDashPathEffect::Make. It reports false when the intervals
// are not a valid dash.
func newDash(intervals []float32, phase float32) (dashInfo, bool) {
	if !validDashPath(phase, intervals) {
		return dashInfo{}, false
	}
	d := dashInfo{intervals: intervals}
	var length float32
	for _, v := range intervals {
		length += v
	}
	d.intervalLength = length
	if phase < 0 {
		phase = -phase
		if phase > length {
			phase = scalarMod(phase, length)
		}
		phase = length - phase
		if phase == length {
			phase = 0
		}
	} else if phase >= length {
		phase = scalarMod(phase, length)
	}
	d.phase = phase
	// find_first_interval.
	d.initialDashIndex = 0
	d.initialDashLength = intervals[0]
	found := false
	for i, gap := range intervals {
		if phase > gap || (phase == gap && gap != 0) {
			phase -= gap
		} else {
			d.initialDashIndex = i
			d.initialDashLength = gap - phase
			found = true
			break
		}
	}
	if !found {
		d.initialDashIndex = 0
		d.initialDashLength = intervals[0]
	}
	return d, true
}

func outsetForStroke(r Rect, sp *strokeParams) Rect {
	radius := sp.width / 2
	if radius == 0 {
		radius = 1
	}
	if sp.join == joinMiter {
		radius *= sp.miterLimit
	}
	return r.outset(radius, radius)
}

func adjustZeroLengthLine(pts *[2]Point) {
	pts[1].X += float32(max(float32(1.001), pts[1].X) * scalarNearlyZero)
}

// dashClipLine is clip_line of SkDashPath.cpp.
func dashClipLine(pts *[2]Point, bounds Rect, intervalLength, priorPhase float32) bool {
	dxy := pts[1].sub(pts[0])
	if dxy.X != 0 && dxy.Y != 0 {
		return false
	}
	vertical := dxy.Y != 0
	get := func(p Point) float32 {
		if vertical {
			return p.Y
		}
		return p.X
	}
	minXY, maxXY := get(pts[0]), get(pts[1])
	swapped := maxXY < minXY
	if swapped {
		minXY, maxXY = maxXY, minXY
	}
	leftTop, rightBottom := bounds.L, bounds.R
	if vertical {
		leftTop, rightBottom = bounds.T, bounds.B
	}
	if maxXY < leftTop || minXY > rightBottom {
		return false
	}
	if minXY < leftTop {
		minXY = leftTop - scalarMod(leftTop-minXY, intervalLength)
		if !swapped {
			minXY -= priorPhase
		}
	}
	if maxXY > rightBottom {
		maxXY = rightBottom + scalarMod(maxXY-rightBottom, intervalLength)
		if swapped {
			maxXY += priorPhase
		}
	}
	if swapped {
		minXY, maxXY = maxXY, minXY
	}
	if vertical {
		pts[0].Y, pts[1].Y = minXY, maxXY
	} else {
		pts[0].X, pts[1].X = minXY, maxXY
	}
	if minXY == maxXY {
		adjustZeroLengthLine(pts)
	}
	return true
}

// isLine is SkPath::isLine.
func (p *Path) isLine() ([2]Point, bool) {
	if len(p.verbs) == 2 && p.verbs[0] == verbMove && p.verbs[1] == verbLine {
		return [2]Point{p.pts[0], p.pts[1]}, true
	}
	return [2]Point{}, false
}

func (p *Path) isLastContourClosed() bool {
	n := len(p.verbs)
	return n > 0 && p.verbs[n-1] == verbClose
}

// cullPath is cull_path of SkDashPath.cpp: it shortens a line or a
// rectangle to the cull rectangle.
func cullPath(src *Path, sp *strokeParams, cullRect *Rect, intervalLength float32, dst *Path) bool {
	if cullRect == nil {
		if pts, ok := src.isLine(); ok && pts[0] == pts[1] {
			adjustZeroLengthLine(&pts)
			dst.MoveTo(pts[0].X, pts[0].Y)
			dst.LineTo(pts[1].X, pts[1].Y)
			return true
		}
		return false
	}
	bounds := outsetForStroke(*cullRect, sp)
	if pts, ok := src.isLine(); ok {
		if dashClipLine(&pts, bounds, intervalLength, 0) {
			dst.MoveTo(pts[0].X, pts[0].Y)
			dst.LineTo(pts[1].X, pts[1].Y)
			return true
		}
		return false
	}
	if _, ok := src.isRect(); ok {
		it := strokeIter{p: src}
		var q [4]Point
		it.next(&q)
		accum := 0.0
		for {
			verb, ok := it.next(&q)
			if !ok || verb != verbLine {
				break
			}
			v := q[1].sub(q[0])
			pts := [2]Point{q[0], q[1]}
			if dashClipLine(&pts, bounds, intervalLength, float32(math.Mod(accum, float64(intervalLength)))) {
				if last, has := dst.lastPt(); !has || last != pts[0] {
					dst.MoveTo(pts[0].X, pts[0].Y)
				}
				dst.LineTo(pts[1].X, pts[1].Y)
			}
			accum += float64(abs32f(v.X + v.Y))
		}
		return !dst.isEmpty()
	}
	return false
}

// dashScratch holds the temporary state of the dasher.
type dashScratch struct {
	cull Path
	meas measureIter
}

// dashPath is SkDashPath::InternalFilter: dst receives the dashes of src.
// It reports whether the dashes were produced. When fillStyle is set after
// the call, the dashes are already the outline of the stroke (a dashed
// line with butt caps).
func dashPath(dst, src *Path, sp *strokeParams, hairline bool, cullRect *Rect, d *dashInfo, scratch *dashScratch) (ok bool, fillStyle bool) {
	if len(src.pts) == 0 {
		return true, false
	}
	count := len(d.intervals)
	intervals := d.intervals
	var dashCount float32

	srcPtr := src
	scratch.cull.Reset()
	if cullPath(src, sp, cullRect, d.intervalLength, &scratch.cull) {
		if _, isRect := src.isRect(); isRect && src.isLastContourClosed() && d.initialDashIndex%2 == 0 {
			m := measureIter{p: src, tolerance: float32(cheapDistLimit * (1 / sp.resScale))}
			var pathLength float32
			if m.next() {
				pathLength = m.cm.length
			}
			endPhase := scalarMod(pathLength+d.phase, d.intervalLength)
			index := 0
			for endPhase > intervals[index] {
				endPhase -= intervals[index]
				index++
				if index == count {
					endPhase = 0
					break
				}
			}
			if (index%2 == 0) == (endPhase > 0) {
				midPoint := src.pts[0]
				last := len(src.pts) - 1
				for last > 0 && midPoint == src.pts[last] {
					last--
				}
				next := 1
				for next < last && midPoint == src.pts[next] {
					next++
				}
				v := midPoint.sub(src.pts[last])
				v = scalePt(v, scalarNearlyZero)
				scratch.cull.MoveTo(midPoint.X-v.X, midPoint.Y-v.Y)
				scratch.cull.LineTo(midPoint.X, midPoint.Y)
				v = midPoint.sub(src.pts[next])
				v = scalePt(v, scalarNearlyZero)
				scratch.cull.LineTo(midPoint.X-v.X, midPoint.Y-v.Y)
			}
		}
		srcPtr = &scratch.cull
	}

	// SpecialLineRec: a dashed line with butt caps becomes rectangles.
	specialLine := false
	var linePts [2]Point
	var tangent, normal Point
	var pathLength float32
	if !hairline && sp.cap == capButt {
		if pts, isLine := srcPtr.isLine(); isLine {
			pl := ptDistance(pts[0], pts[1])
			tg := pts[1].sub(pts[0])
			if !tg.isZero() {
				tg = scalePt(tg, float32(1.0)/pl)
				if isFinite32(tg.X) && isFinite32(tg.Y) {
					ptCount := float32(pl*float32(count>>1)) / d.intervalLength
					ptCount = min(ptCount, maxDashCount)
					if ptCount == ptCount {
						specialLine = true
						linePts = pts
						tangent = tg
						pathLength = pl
						normal = scalePt(rotateCCW(tg), sp.width/2)
					}
				}
			}
		}
	}

	m := &scratch.meas
	*m = measureIter{p: srcPtr, tolerance: float32(cheapDistLimit * (1 / sp.resScale)), cm: contourMeasure{segs: m.cm.segs[:0], pts: m.cm.pts[:0]}}
	if !m.next() {
		return true, specialLine
	}
	for {
		skipFirstSegment := m.cm.isClosed
		addedSegment := false
		length := m.cm.length
		index := d.initialDashIndex

		dashCount += float32(length*float32(count>>1)) / d.intervalLength
		if dashCount > maxDashCount {
			dst.Reset()
			return false, false
		}

		distance := 0.0
		dlen := float64(d.initialDashLength)
		for distance < float64(length) {
			addedSegment = false
			if index%2 == 0 && !skipFirstSegment {
				addedSegment = true
				if specialLine {
					d0, d1 := float32(distance), float32(distance+dlen)
					if d1 > pathLength {
						d1 = pathLength
					}
					x0 := linePts[0].X + float32(tangent.X*d0)
					x1 := linePts[0].X + float32(tangent.X*d1)
					y0 := linePts[0].Y + float32(tangent.Y*d0)
					y1 := linePts[0].Y + float32(tangent.Y*d1)
					dst.MoveTo(x0+normal.X, y0+normal.Y)
					dst.LineTo(x1+normal.X, y1+normal.Y)
					dst.LineTo(x1-normal.X, y1-normal.Y)
					dst.LineTo(x0-normal.X, y0-normal.Y)
				} else {
					m.cm.getSegment(float32(distance), float32(distance+dlen), dst, true)
				}
			}
			distance += dlen
			skipFirstSegment = false
			index++
			if index == count {
				index = 0
			}
			dlen = float64(intervals[index])
		}
		if m.cm.isClosed && d.initialDashIndex%2 == 0 && d.initialDashLength >= 0 {
			m.cm.getSegment(0, d.initialDashLength, dst, !addedSegment)
		}
		if !m.next() {
			break
		}
	}
	return true, specialLine
}
