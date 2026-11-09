// Ported from skia/src/core/SkAnalyticEdge.h, SkAnalyticEdge.cpp and
// SkEdgeBuilder.cpp (Skia branch chrome/m156).

package raster

const (
	edgeLine  = 0
	edgeQuad  = 1
	edgeCubic = 2

	// kDefaultAccuracy: y is snapped to 1/4 pixel.
	edgeAccuracy = 2

	maxCoeffShift = 6
)

// aEdge is SkAnalyticEdge with the fields of its quadratic and cubic
// subclasses.
type aEdge struct {
	next, prev *aEdge

	x, dx, upperX  fixed
	y              fixed
	upperY, lowerY fixed
	dy             fixed // abs(1/dx); maxS32 when dx is close to 0

	edgeType   int8
	curveCount int8  // quad: positive, cubic: negative
	curveShift uint8 // quad: applied to all, cubic: applied to cdx and cdy
	winding    int8

	// Quadratic state.
	qx, qy, qdx, qdy, qddx, qddy fixed
	qLastX, qLastY               fixed
	snappedX, snappedY           fixed

	// Cubic state. The cubic shares snappedY.
	cx, cy, cdx, cdy fixed
	cddx, cddy       fixed
	cdddx, cdddy     fixed
	cLastX, cLastY   fixed
	toFixedShift     uint8
}

// snapY is SkAnalyticEdge::SnapY.
func snapY(y fixed) fixed {
	return fixed((uint32(y) + uint32(fixed1>>(edgeAccuracy+1))) >> (16 - edgeAccuracy) << (16 - edgeAccuracy))
}

// goY is SkAnalyticEdge::goY(y).
func (e *aEdge) goY(y fixed) {
	if y == e.y+fixed1 {
		e.x = satAdd32(e.x, e.dx)
		e.y = y
	} else if y != e.y {
		e.x = pinToS32(int64(e.upperX) + (int64(e.dx) * int64(y-e.upperY) >> 16))
		e.y = y
	}
}

// goYShift is SkAnalyticEdge::goY(y, yShift).
func (e *aEdge) goYShift(y fixed, yShift uint) {
	e.y = y
	e.x = satAdd32(e.x, e.dx>>yShift)
}

const inverseTableSize = 1024

// quickInverseTable[i] is -(2^22 / (1024 - i)), truncated: the table of
// quick_inverse in SkAnalyticEdge.cpp.
var quickInverseTable = func() (t [inverseTableSize + 1]int32) {
	for i := 0; i < inverseTableSize; i++ {
		t[i] = -int32((1 << 22) / (inverseTableSize - i))
	}
	return
}()

func quickInverse(x fdot6) fixed {
	if x > 0 {
		return -quickInverseTable[inverseTableSize-x]
	}
	return quickInverseTable[inverseTableSize+x]
}

func quickDiv(a, b fdot6) fixed {
	const minBits = 3
	const maxAbsA = 1 << (31 - (22 - minBits))
	absA, absB := abs32(a), abs32(b)
	if absB >= (1<<minBits) && absB < inverseTableSize && absA < maxAbsA {
		return (a * quickInverse(b)) >> 6
	}
	return fdot6Div(a, b)
}

// setLine is SkAnalyticEdge::setLine.
func (e *aEdge) setLine(p0, p1 Point) bool {
	const multiplier = 1 << edgeAccuracy
	x0 := fdot6ToFixed(scalarToFDot6(float32(p0.X*multiplier))) >> edgeAccuracy
	y0 := snapY(fdot6ToFixed(scalarToFDot6(float32(p0.Y*multiplier))) >> edgeAccuracy)
	x1 := fdot6ToFixed(scalarToFDot6(float32(p1.X*multiplier))) >> edgeAccuracy
	y1 := snapY(fdot6ToFixed(scalarToFDot6(float32(p1.Y*multiplier))) >> edgeAccuracy)

	winding := int8(1)
	if y0 > y1 {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
		winding = -1
	}
	dy := fixedToFDot6(y1 - y0)
	if dy == 0 {
		return false
	}
	dx := fixedToFDot6(x1 - x0)
	slope := quickDiv(dx, dy)
	absSlope := abs32(slope)

	e.x = x0
	e.dx = slope
	e.upperX = x0
	e.y = y0
	e.upperY = y0
	e.lowerY = y1
	switch {
	case dx == 0 || slope == 0:
		e.dy = maxS32
	case absSlope < inverseTableSize:
		e.dy = quickInverse(absSlope)
	default:
		e.dy = abs32(quickDiv(dy, dx))
	}
	e.edgeType = edgeLine
	e.curveCount = 0
	e.winding = winding
	e.curveShift = 0
	return true
}

// updateLine is SkAnalyticEdge::updateLine.
func (e *aEdge) updateLine(x0, y0, x1, y1, slope fixed) bool {
	if y0 > y1 {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
		e.winding = -e.winding
	}
	dx := fixedToFDot6(x1 - x0)
	dy := fixedToFDot6(y1 - y0)
	if dy == 0 {
		return false
	}
	absSlope := abs32(fixedToFDot6(slope))
	e.x = x0
	e.dx = slope
	e.upperX = x0
	e.y = y0
	e.upperY = y0
	e.lowerY = y1
	switch {
	case dx == 0 || slope == 0:
		e.dy = maxS32
	case absSlope < inverseTableSize:
		e.dy = quickInverse(absSlope)
	default:
		e.dy = abs32(quickDiv(dy, dx))
	}
	return true
}

// update is SkAnalyticEdge::update: it moves a curve to its next segment
// and reports whether the edge has one.
func (e *aEdge) update() bool {
	if e.curveCount < 0 {
		return e.updateCubic()
	} else if e.curveCount > 0 {
		return e.updateQuadratic()
	}
	return false
}

func cheapDistance(dx, dy fdot6) fdot6 {
	dx, dy = abs32(dx), abs32(dy)
	if dx > dy {
		dx += dy >> 1
	} else {
		dx = dy + (dx >> 1)
	}
	return dx
}

func diffToShift(dx, dy fdot6, shiftAA int) int {
	dist := cheapDistance(dx, dy)
	dist = (dist + (1 << (2 + shiftAA))) >> (3 + shiftAA)
	return (32 - clz32(uint32(dist))) >> 1
}

func fdot6ToFixedDiv2(v fdot6) fixed { return fixed(uint32(v) << (16 - 6 - 1)) }

// setQuadraticWithoutUpdate is SkAnalyticQuadraticEdge::setQuadraticWithoutUpdate.
func (e *aEdge) setQuadraticWithoutUpdate(pts []Point, shift int) bool {
	scale := float32(int(1) << (shift + 6))
	x0 := floatToInt(float32(pts[0].X * scale))
	y0 := floatToInt(float32(pts[0].Y * scale))
	x1 := floatToInt(float32(pts[1].X * scale))
	y1 := floatToInt(float32(pts[1].Y * scale))
	x2 := floatToInt(float32(pts[2].X * scale))
	y2 := floatToInt(float32(pts[2].Y * scale))

	winding := int8(1)
	if y0 > y2 {
		x0, x2 = x2, x0
		y0, y2 = y2, y0
		winding = -1
	}
	top := fdot6Round(y0)
	bot := fdot6Round(y2)
	if top == bot {
		return false
	}
	{
		dx := (int32(uint32(x1)<<1) - x0 - x2) >> 2
		dy := (int32(uint32(y1)<<1) - y0 - y2) >> 2
		shift = diffToShift(dx, dy, shift)
	}
	if shift == 0 {
		shift = 1
	} else if shift > maxCoeffShift {
		shift = maxCoeffShift
	}
	e.winding = winding
	e.edgeType = edgeQuad
	e.curveCount = int8(1 << shift)
	e.curveShift = uint8(shift - 1)

	a := fdot6ToFixedDiv2(x0 - x1 - x1 + x2)
	b := fdot6ToFixed(x1 - x0)
	e.qx = fdot6ToFixed(x0)
	e.qdx = b + (a >> uint(shift))
	e.qddx = a >> uint(shift-1)

	a = fdot6ToFixedDiv2(y0 - y1 - y1 + y2)
	b = fdot6ToFixed(y1 - y0)
	e.qy = fdot6ToFixed(y0)
	e.qdy = b + (a >> uint(shift))
	e.qddy = a >> uint(shift-1)

	e.qLastX = fdot6ToFixed(x2)
	e.qLastY = fdot6ToFixed(y2)
	return true
}

// setQuadratic is SkAnalyticQuadraticEdge::setQuadratic.
func (e *aEdge) setQuadratic(pts []Point) bool {
	if !e.setQuadraticWithoutUpdate(pts, edgeAccuracy) {
		return false
	}
	e.qx >>= edgeAccuracy
	e.qy >>= edgeAccuracy
	e.qdx >>= edgeAccuracy
	e.qdy >>= edgeAccuracy
	e.qddx >>= edgeAccuracy
	e.qddy >>= edgeAccuracy
	e.qLastX >>= edgeAccuracy
	e.qLastY >>= edgeAccuracy
	e.qy = snapY(e.qy)
	e.qLastY = snapY(e.qLastY)
	e.edgeType = edgeQuad
	e.snappedX = e.qx
	e.snappedY = e.qy
	return e.updateQuadratic()
}

// updateQuadratic is SkAnalyticQuadraticEdge::updateQuadratic.
func (e *aEdge) updateQuadratic() bool {
	success := false
	count := int(e.curveCount)
	oldx, oldy := e.qx, e.qy
	dx, dy := e.qdx, e.qdy
	var newx, newy, newSnappedX, newSnappedY fixed
	shift := uint(e.curveShift)

	for {
		var slope fixed
		count--
		if count > 0 {
			newx = oldx + (dx >> shift)
			newy = oldy + (dy >> shift)
			if abs32(dy>>shift) >= fixed1*2 && (int64(abs32(dy))<<6) > int64(abs32(dx)) {
				diffY := fixedToFDot6(newy - e.snappedY)
				if diffY != 0 {
					slope = quickDiv(fixedToFDot6(newx-e.snappedX), diffY)
				} else {
					slope = maxS32
				}
				newSnappedY = min(e.qLastY, fixedRoundToFixed(newy))
				newSnappedX = newx - fixedMul(slope, newy-newSnappedY)
			} else {
				newSnappedY = min(e.qLastY, snapY(newy))
				newSnappedX = newx
				diffY := fixedToFDot6(newSnappedY - e.snappedY)
				if diffY != 0 {
					slope = quickDiv(fixedToFDot6(newx-e.snappedX), diffY)
				} else {
					slope = maxS32
				}
			}
			dx += e.qddx
			dy += e.qddy
		} else {
			newx = e.qLastX
			newy = e.qLastY
			newSnappedY = newy
			newSnappedX = newx
			diffY := fixedToFDot6(newy - e.snappedY)
			if diffY != 0 {
				slope = quickDiv(fixedToFDot6(newx-e.snappedX), diffY)
			} else {
				slope = maxS32
			}
		}
		if slope < maxS32 {
			success = e.updateLine(e.snappedX, e.snappedY, newSnappedX, newSnappedY, slope)
		}
		oldx, oldy = newx, newy
		if !(count > 0 && !success) {
			break
		}
	}
	e.qx, e.qy = newx, newy
	e.qdx, e.qdy = dx, dy
	e.snappedX, e.snappedY = newSnappedX, newSnappedY
	e.curveCount = int8(count)
	return success
}

// keepContinuousQuad is SkAnalyticQuadraticEdge::keepContinuous.
func (e *aEdge) keepContinuousQuad() {
	e.snappedX = e.x
	e.snappedY = e.y
}

// keepContinuousCubic is SkAnalyticCubicEdge::keepContinuous.
func (e *aEdge) keepContinuousCubic() {
	e.cx = e.x
	e.snappedY = e.y
}

func cubicDeltaFromLine(a, b, c, d fdot6) fdot6 {
	oneThird := (a*8 - b*15 + 6*c + d) * 19 >> 9
	twoThird := (a + 6*b - c*15 + d*8) * 19 >> 9
	return max(abs32(oneThird), abs32(twoThird))
}

// setCubicWithoutUpdate is SkAnalyticCubicEdge::setCubicWithoutUpdate.
func (e *aEdge) setCubicWithoutUpdate(pts []Point, shift int) bool {
	scale := float32(int(1) << (shift + 6))
	x0 := floatToInt(float32(pts[0].X * scale))
	y0 := floatToInt(float32(pts[0].Y * scale))
	x1 := floatToInt(float32(pts[1].X * scale))
	y1 := floatToInt(float32(pts[1].Y * scale))
	x2 := floatToInt(float32(pts[2].X * scale))
	y2 := floatToInt(float32(pts[2].Y * scale))
	x3 := floatToInt(float32(pts[3].X * scale))
	y3 := floatToInt(float32(pts[3].Y * scale))

	winding := int8(1)
	if y0 > y3 {
		x0, x3 = x3, x0
		x1, x2 = x2, x1
		y0, y3 = y3, y0
		y1, y2 = y2, y1
		winding = -1
	}
	top := fdot6Round(y0)
	bot := fdot6Round(y3)
	if top == bot {
		return false
	}
	{
		dx := cubicDeltaFromLine(x0, x1, x2, x3)
		dy := cubicDeltaFromLine(y0, y1, y2, y3)
		shift = diffToShift(dx, dy, 2) + 1
	}
	if shift > maxCoeffShift {
		shift = maxCoeffShift
	}
	upShift := 6
	downShift := shift + upShift - 10
	if downShift < 0 {
		downShift = 0
		upShift = 10 - shift
	}
	e.winding = winding
	e.edgeType = edgeCubic
	e.curveCount = int8(-(1 << uint(shift)))
	e.curveShift = uint8(shift)
	e.toFixedShift = uint8(downShift)

	up := func(x fdot6) fixed { return fixed(uint32(x) << uint(upShift)) }
	b := up(3 * (x1 - x0))
	c := up(3 * (x0 - x1 - x1 + x2))
	d := up(x3 + 3*(x1-x2) - x0)
	e.cx = fdot6ToFixed(x0)
	e.cdx = b + (c >> uint(shift)) + (d >> uint(2*shift))
	e.cddx = int32(2*int64(c) + ((3 * int64(d)) >> uint(shift-1)))
	e.cdddx = int32((3 * int64(d)) >> uint(shift-1))

	b = up(3 * (y1 - y0))
	c = up(3 * (y0 - y1 - y1 + y2))
	d = up(y3 + 3*(y1-y2) - y0)
	e.cy = fdot6ToFixed(y0)
	e.cdy = b + (c >> uint(shift)) + (d >> uint(2*shift))
	e.cddy = int32(2*int64(c) + ((3 * int64(d)) >> uint(shift-1)))
	e.cdddy = int32((3 * int64(d)) >> uint(shift-1))

	e.cLastX = fdot6ToFixed(x3)
	e.cLastY = fdot6ToFixed(y3)
	return true
}

// setCubic is SkAnalyticCubicEdge::setCubic.
func (e *aEdge) setCubic(pts []Point) bool {
	if !e.setCubicWithoutUpdate(pts, edgeAccuracy) {
		return false
	}
	e.cx >>= edgeAccuracy
	e.cy >>= edgeAccuracy
	e.cdx >>= edgeAccuracy
	e.cdy >>= edgeAccuracy
	e.cddx >>= edgeAccuracy
	e.cddy >>= edgeAccuracy
	e.cdddx >>= edgeAccuracy
	e.cdddy >>= edgeAccuracy
	e.cLastX >>= edgeAccuracy
	e.cLastY >>= edgeAccuracy
	e.cy = snapY(e.cy)
	e.snappedY = e.cy
	e.cLastY = snapY(e.cLastY)
	e.edgeType = edgeCubic
	return e.updateCubic()
}

// updateCubic is SkAnalyticCubicEdge::updateCubic.
func (e *aEdge) updateCubic() bool {
	var success bool
	count := int(e.curveCount)
	oldx, oldy := e.cx, e.cy
	var newx, newy fixed
	ddshift := uint(e.curveShift)
	dshift := uint(e.toFixedShift)

	for {
		count++
		if count < 0 {
			newx = oldx + (e.cdx >> dshift)
			e.cdx += e.cddx >> ddshift
			e.cddx += e.cdddx

			newy = oldy + (e.cdy >> dshift)
			e.cdy += e.cddy >> ddshift
			e.cddy += e.cdddy
		} else {
			newx = e.cLastX
			newy = e.cLastY
		}
		if newy < oldy {
			newy = oldy
		}
		newSnappedY := snapY(newy)
		if e.cLastY < newSnappedY {
			newSnappedY = e.cLastY
			count = 0
		}
		var slope fixed
		if fixedToFDot6(newSnappedY-e.snappedY) == 0 {
			slope = maxS32
		} else {
			slope = fdot6Div(fixedToFDot6(newx-oldx), fixedToFDot6(newSnappedY-e.snappedY))
		}
		success = e.updateLine(oldx, e.snappedY, newx, newSnappedY, slope)

		oldx, oldy = newx, newy
		e.snappedY = newSnappedY
		if !(count < 0 && !success) {
			break
		}
	}
	e.cx, e.cy = newx, newy
	e.curveCount = int8(count)
	return success
}

// ---- edge builder: SkAnalyticEdgeBuilder ----

type edgeBuilder struct {
	pool  []aEdge
	used  int
	list  []*aEdge
	quads []Point // scratch for conic to quads

	canCullToTheRight bool
}

func (b *edgeBuilder) reset() {
	b.used = 0
	b.list = b.list[:0]
}

func (b *edgeBuilder) alloc() *aEdge {
	if b.used == len(b.pool) {
		// Edges are referenced by pointer, so a full pool is not grown in
		// place: a new chunk is started.
		n := max(64, len(b.pool))
		b.pool = make([]aEdge, n*2)
		b.used = 0
	}
	e := &b.pool[b.used]
	b.used++
	*e = aEdge{}
	return e
}

type combineResult uint8

const (
	combinePartial combineResult = iota
	combineTotal
	combineNo
)

func approxEqualFixed(a, b fixed) bool { return abs32(a-b) < 0x100 }

// combineVertical is SkAnalyticEdgeBuilder::combineVertical.
func (b *edgeBuilder) combineVertical(edge, last *aEdge) combineResult {
	if last.edgeType != edgeLine || last.dx != 0 || edge.x != last.x {
		return combineNo
	}
	if edge.winding == last.winding {
		if edge.lowerY == last.upperY {
			last.upperY = edge.upperY
			last.y = last.upperY
			return combinePartial
		}
		if approxEqualFixed(edge.upperY, last.lowerY) {
			last.lowerY = edge.lowerY
			return combinePartial
		}
		return combineNo
	}
	if !b.canCullToTheRight {
		return combineNo
	}
	if approxEqualFixed(edge.upperY, last.upperY) {
		if approxEqualFixed(edge.lowerY, last.lowerY) {
			return combineTotal
		}
		if edge.lowerY < last.lowerY {
			last.upperY = edge.lowerY
			last.y = last.upperY
			return combinePartial
		}
		last.upperY = last.lowerY
		last.y = last.upperY
		last.lowerY = edge.lowerY
		last.winding = edge.winding
		return combinePartial
	}
	if approxEqualFixed(edge.lowerY, last.lowerY) {
		if edge.upperY > last.upperY {
			last.lowerY = edge.upperY
			return combinePartial
		}
		last.lowerY = last.upperY
		last.upperY = edge.upperY
		last.y = last.upperY
		last.winding = edge.winding
		return combinePartial
	}
	return combineNo
}

func (b *edgeBuilder) addLine(p0, p1 Point) {
	e := b.alloc()
	if !e.setLine(p0, p1) {
		b.used--
		return
	}
	combine := combineNo
	if e.dx == 0 && len(b.list) > 0 {
		combine = b.combineVertical(e, b.list[len(b.list)-1])
	}
	switch combine {
	case combineTotal:
		b.list = b.list[:len(b.list)-1]
	case combinePartial:
	case combineNo:
		b.list = append(b.list, e)
	}
}

func (b *edgeBuilder) addQuad(pts []Point) {
	e := b.alloc()
	if e.setQuadratic(pts) {
		b.list = append(b.list, e)
	} else {
		b.used--
	}
}

func (b *edgeBuilder) addCubic(pts []Point) {
	e := b.alloc()
	if e.setCubic(pts) {
		b.list = append(b.list, e)
	} else {
		b.used--
	}
}

// pathEdgeIter is SkPathEdgeIter: it yields the segments of a path and adds
// the line that closes each contour.
type pathEdgeIter struct {
	p           *Path
	vi, pi, wi  int
	moveIdx     int
	needsClose  bool
	scratch     [2]Point
	conicWeight float32
}

// next returns the verb and the points of the next segment. The verb is
// verbClose at the end of the path.
func (it *pathEdgeIter) next() (pathVerb, []Point) {
	closeLine := func() (pathVerb, []Point) {
		it.scratch[0] = it.p.pts[it.pi-1]
		it.scratch[1] = it.p.pts[it.moveIdx]
		it.needsClose = false
		return verbLine, it.scratch[:]
	}
	for {
		if it.vi == len(it.p.verbs) {
			if it.needsClose {
				return closeLine()
			}
			return verbClose, nil
		}
		verb := it.p.verbs[it.vi]
		it.vi++
		switch verb {
		case verbMove:
			if it.needsClose {
				v, pts := closeLine()
				it.moveIdx = it.pi
				it.pi++
				return v, pts
			}
			it.moveIdx = it.pi
			it.pi++
		case verbClose:
			if it.needsClose {
				return closeLine()
			}
		default:
			n := 1
			switch verb {
			case verbQuad:
				n = 2
			case verbConic:
				n = 2
				it.conicWeight = it.p.conics[it.wi]
				it.wi++
			case verbCubic:
				n = 3
			}
			it.needsClose = true
			it.pi += n
			return verb, it.p.pts[it.pi-n-1 : it.pi]
		}
	}
}

// buildEdges is SkEdgeBuilder::buildEdges. clip is nil when the path is
// inside the clip.
func (b *edgeBuilder) buildEdges(p *Path, clip *IRect) int {
	b.reset()
	b.canCullToTheRight = !p.isConvex()
	it := pathEdgeIter{p: p}
	if clip != nil {
		c := edgeClipper{canCullToTheRight: b.canCullToTheRight}
		clipR := clip.toRect()
		consume := func() {
			for i, v := range c.verbs {
				pts := c.pts[c.offs[i]:]
				switch v {
				case verbLine:
					b.addLine(pts[0], pts[1])
				case verbQuad:
					b.addQuad(pts[:3])
				case verbCubic:
					b.addCubic(pts[:4])
				}
			}
		}
		if p.segMask == segLine {
			// buildPoly with a clip.
			for {
				v, pts := it.next()
				if v == verbClose {
					break
				}
				var lines [4]Point
				n := clipLine(pts[0], pts[1], clipR, &lines, b.canCullToTheRight)
				for i := 0; i < n; i++ {
					b.addLine(lines[i], lines[i+1])
				}
			}
			return len(b.list)
		}
		for {
			v, pts := it.next()
			if v == verbClose {
				break
			}
			switch v {
			case verbLine:
				if c.clipLine(pts[0], pts[1], clipR) {
					consume()
				}
			case verbQuad:
				if c.clipQuad(pts, clipR) {
					consume()
				}
			case verbConic:
				b.quads = conicToQuads(pts[0], pts[1], pts[2], it.conicWeight, 0.25, b.quads)
				for i := 0; i+2 < len(b.quads); i += 2 {
					if c.clipQuad(b.quads[i:i+3], clipR) {
						consume()
					}
				}
			case verbCubic:
				if c.clipCubic(pts, clipR) {
					consume()
				}
			}
		}
		return len(b.list)
	}
	handleQuad := func(pts []Point) {
		var mono [5]Point
		n := chopQuadAtYExtrema(pts, &mono)
		for i := 0; i <= n; i++ {
			b.addQuad(mono[i*2 : i*2+3])
		}
	}
	for {
		v, pts := it.next()
		if v == verbClose {
			break
		}
		switch v {
		case verbLine:
			b.addLine(pts[0], pts[1])
		case verbQuad:
			handleQuad(pts)
		case verbConic:
			b.quads = conicToQuads(pts[0], pts[1], pts[2], it.conicWeight, 0.25, b.quads)
			for i := 0; i+2 < len(b.quads); i += 2 {
				handleQuad(b.quads[i : i+3])
			}
		case verbCubic:
			var mono [10]Point
			n := chopCubicAtYExtrema(pts, &mono)
			for i := 0; i <= n; i++ {
				b.addCubic(mono[i*3 : i*3+4])
			}
		}
	}
	return len(b.list)
}
