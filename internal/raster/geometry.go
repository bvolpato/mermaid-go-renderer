// Ported from skia/src/core/SkGeometry.cpp (Skia branch chrome/m156).

package raster

import "math"

func isFinite32(v float32) bool {
	return !math.IsInf(float64(v), 0) && !math.IsNaN(float64(v))
}

func abs32f(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func sqrt32(v float32) float32 { return float32(math.Sqrt(float64(v))) }

func isNotMonotonic(a, b, c float32) bool {
	ab := a - b
	bc := b - c
	if ab < 0 {
		bc = -bc
	}
	return ab == 0 || bc < 0
}

func validUnitDivide(numer, denom float32) (float32, bool) {
	if numer < 0 {
		numer = -numer
		denom = -denom
	}
	if denom == 0 || numer == 0 || numer >= denom {
		return 0, false
	}
	r := numer / denom
	if r != r || r == 0 {
		return 0, false
	}
	return r, true
}

// findUnitQuadRoots is SkFindUnitQuadRoots.
func findUnitQuadRoots(a, b, c float32, roots *[2]float32) int {
	if a == 0 {
		if r, ok := validUnitDivide(-c, b); ok {
			roots[0] = r
			return 1
		}
		return 0
	}
	dr := float64(float64(b)*float64(b)) - float64(4*float64(a)*float64(c))
	if dr < 0 {
		return 0
	}
	dr = math.Sqrt(dr)
	r32 := float32(dr)
	if !isFinite32(r32) {
		return 0
	}
	var q float32
	if b < 0 {
		q = -(b - r32) / 2
	} else {
		q = -(b + r32) / 2
	}
	n := 0
	if r, ok := validUnitDivide(q, a); ok {
		roots[n] = r
		n++
	}
	if r, ok := validUnitDivide(c, q); ok {
		roots[n] = r
		n++
	}
	if n == 2 {
		if roots[0] > roots[1] {
			roots[0], roots[1] = roots[1], roots[0]
		} else if roots[0] == roots[1] {
			n = 1
		}
	}
	return n
}

func interpPt(a, b Point, t float32) Point {
	return Point{a.X + float32((b.X-a.X)*t), a.Y + float32((b.Y-a.Y)*t)}
}

// chopQuadAt is SkChopQuadAt.
func chopQuadAt(src []Point, dst *[5]Point, t float32) {
	p01 := interpPt(src[0], src[1], t)
	p12 := interpPt(src[1], src[2], t)
	dst[0] = src[0]
	dst[1] = p01
	dst[2] = interpPt(p01, p12, t)
	dst[3] = p12
	dst[4] = src[2]
}

// chopQuadAtYExtrema is SkChopQuadAtYExtrema. It returns the number of
// extra quads: 0 or 1.
func chopQuadAtYExtrema(src []Point, dst *[5]Point) int {
	a, b, c := src[0].Y, src[1].Y, src[2].Y
	if isNotMonotonic(a, b, c) {
		if t, ok := validUnitDivide(a-b, a-b-b+c); ok {
			chopQuadAt(src, dst, t)
			dst[1].Y = dst[2].Y
			dst[3].Y = dst[2].Y
			return 1
		}
		if abs32f(a-b) < abs32f(b-c) {
			b = a
		} else {
			b = c
		}
	}
	dst[0] = Point{src[0].X, a}
	dst[1] = Point{src[1].X, b}
	dst[2] = Point{src[2].X, c}
	return 0
}

// chopQuadAtXExtrema is SkChopQuadAtXExtrema.
func chopQuadAtXExtrema(src []Point, dst *[5]Point) int {
	a, b, c := src[0].X, src[1].X, src[2].X
	if isNotMonotonic(a, b, c) {
		if t, ok := validUnitDivide(a-b, a-b-b+c); ok {
			chopQuadAt(src, dst, t)
			dst[1].X = dst[2].X
			dst[3].X = dst[2].X
			return 1
		}
		if abs32f(a-b) < abs32f(b-c) {
			b = a
		} else {
			b = c
		}
	}
	dst[0] = Point{a, src[0].Y}
	dst[1] = Point{b, src[1].Y}
	dst[2] = Point{c, src[2].Y}
	return 0
}

// uncheckedMix is (b - a)*t + a.
func uncheckedMix(a, b Point, t float32) Point {
	return Point{float32((b.X-a.X)*t) + a.X, float32((b.Y-a.Y)*t) + a.Y}
}

// chopCubicAt is SkChopCubicAt(src, dst, t): dst has 7 points.
func chopCubicAt(src []Point, dst []Point, t float32) {
	if t == 1 {
		copy(dst[:4], src[:4])
		dst[4], dst[5], dst[6] = src[3], src[3], src[3]
		return
	}
	p0, p1, p2, p3 := src[0], src[1], src[2], src[3]
	ab := uncheckedMix(p0, p1, t)
	bc := uncheckedMix(p1, p2, t)
	cd := uncheckedMix(p2, p3, t)
	abc := uncheckedMix(ab, bc, t)
	bcd := uncheckedMix(bc, cd, t)
	abcd := uncheckedMix(abc, bcd, t)
	dst[0], dst[1], dst[2], dst[3], dst[4], dst[5], dst[6] = p0, ab, abc, abcd, bcd, cd, p3
}

// chopCubicAt2 is SkChopCubicAt(src, dst, t0, t1): dst has 10 points.
func chopCubicAt2(src []Point, dst []Point, t0, t1 float32) {
	if t1 == 1 {
		chopCubicAt(src, dst, t0)
		dst[7], dst[8], dst[9] = src[3], src[3], src[3]
		return
	}
	p0, p1, p2, p3 := src[0], src[1], src[2], src[3]
	// Lane 0 uses t0, lane 1 uses t1.
	ab0, ab1 := uncheckedMix(p0, p1, t0), uncheckedMix(p0, p1, t1)
	bc0, bc1 := uncheckedMix(p1, p2, t0), uncheckedMix(p1, p2, t1)
	cd0, cd1 := uncheckedMix(p2, p3, t0), uncheckedMix(p2, p3, t1)
	abc0, abc1 := uncheckedMix(ab0, bc0, t0), uncheckedMix(ab1, bc1, t1)
	bcd0, bcd1 := uncheckedMix(bc0, cd0, t0), uncheckedMix(bc1, cd1, t1)
	abcd0, abcd1 := uncheckedMix(abc0, bcd0, t0), uncheckedMix(abc1, bcd1, t1)
	// middle = mix(abc, bcd, shuffled T): lane 0 uses t1, lane 1 uses t0.
	mid0, mid1 := uncheckedMix(abc0, bcd0, t1), uncheckedMix(abc1, bcd1, t0)
	dst[0], dst[1], dst[2], dst[3] = p0, ab0, abc0, abcd0
	dst[4], dst[5] = mid0, mid1
	dst[6], dst[7], dst[8], dst[9] = abcd1, bcd1, cd1, p3
}

func pin01(t float32) float32 {
	// skvx::pin and SkTPin: NaN gives the lower bound.
	if !(t > 0) {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

// chopCubicAtTs is SkChopCubicAt(src, dst, tValues, tCount).
func chopCubicAtTs(src []Point, dst []Point, ts []float32) {
	if len(ts) == 0 {
		copy(dst[:4], src[:4])
		return
	}
	i := 0
	for ; i < len(ts)-1; i += 2 {
		t0, t1 := ts[i], ts[i+1]
		if i != 0 {
			last := ts[i-1]
			t0 = pin01((t0 - last) / (1 - last))
			t1 = pin01((t1 - last) / (1 - last))
		}
		var tmp [10]Point
		chopCubicAt2(src, tmp[:], t0, t1)
		copy(dst[:10], tmp[:])
		src = dst[6:]
		dst = dst[6:]
	}
	if i < len(ts) {
		t := ts[i]
		if i != 0 {
			last := ts[i-1]
			t = pin01((t - last) / (1 - last))
		}
		var tmp [7]Point
		chopCubicAt(src, tmp[:], t)
		copy(dst[:7], tmp[:])
	}
}

// findCubicExtrema is SkFindCubicExtrema.
func findCubicExtrema(a, b, c, d float32, ts *[2]float32) int {
	A := d - a + float32(3*(b-c))
	B := float32(2 * (a - b - b + c))
	C := b - a
	return findUnitQuadRoots(A, B, C, ts)
}

// chopCubicAtYExtrema is SkChopCubicAtYExtrema: dst has 10 points. It
// returns the number of extra cubics.
func chopCubicAtYExtrema(src []Point, dst *[10]Point) int {
	var ts [2]float32
	roots := findCubicExtrema(src[0].Y, src[1].Y, src[2].Y, src[3].Y, &ts)
	chopCubicAtTs(src, dst[:], ts[:roots])
	if roots > 0 {
		dst[2].Y = dst[3].Y
		dst[4].Y = dst[3].Y
		if roots == 2 {
			dst[5].Y = dst[6].Y
			dst[7].Y = dst[6].Y
		}
	}
	return roots
}

// chopCubicAtXExtrema is SkChopCubicAtXExtrema.
func chopCubicAtXExtrema(src []Point, dst *[10]Point) int {
	var ts [2]float32
	roots := findCubicExtrema(src[0].X, src[1].X, src[2].X, src[3].X, &ts)
	chopCubicAtTs(src, dst[:], ts[:roots])
	if roots > 0 {
		dst[2].X = dst[3].X
		dst[4].X = dst[3].X
		if roots == 2 {
			dst[5].X = dst[6].X
			dst[7].X = dst[6].X
		}
	}
	return roots
}

// ---- conics ----

type conic struct {
	pts [3]Point
	w   float32
}

const maxConicToQuadPOW2 = 5

// computeQuadPOW2 is SkConic::computeQuadPOW2.
func (c *conic) computeQuadPOW2(tol float32) int {
	if tol < 0 || !isFinite32(tol) || !c.pts[0].isFinite() || !c.pts[1].isFinite() || !c.pts[2].isFinite() ||
		c.w < 0 || !isFinite32(c.w) {
		return 0
	}
	a := c.w - 1
	k := a / float32(4*(2+a))
	x := float32(k * (c.pts[0].X - float32(2*c.pts[1].X) + c.pts[2].X))
	y := float32(k * (c.pts[0].Y - float32(2*c.pts[1].Y) + c.pts[2].Y))
	err := sqrt32(float32(x*x) + float32(y*y))
	pow2 := 0
	for ; pow2 < maxConicToQuadPOW2; pow2++ {
		if err <= tol {
			break
		}
		err *= 0.25
	}
	return pow2
}

// chop is SkConic::chop with SK_SUPPORT_LEGACY_CONIC_CHOP, which Chromium
// defines.
func (c *conic) chop(dst *[2]conic) {
	scale := 1 / (1 + c.w)
	newW := sqrt32(0.5 + float32(c.w*0.5))
	p0, p1, p2 := c.pts[0], c.pts[1], c.pts[2]
	wp1 := Point{float32(c.w * p1.X), float32(c.w * p1.Y)}
	m := Point{
		float32(float32((p0.X+float32(2*wp1.X)+p2.X)*scale) * 0.5),
		float32(float32((p0.Y+float32(2*wp1.Y)+p2.Y)*scale) * 0.5),
	}
	if !m.isFinite() {
		wd := float64(c.w)
		w2 := float64(wd * 2)
		scaleHalf := float64(1 / (1 + wd) * 0.5)
		m.X = float32((float64(p0.X) + float64(w2*float64(p1.X)) + float64(p2.X)) * scaleHalf)
		m.Y = float32((float64(p0.Y) + float64(w2*float64(p1.Y)) + float64(p2.Y)) * scaleHalf)
	}
	dst[0].pts[0] = p0
	dst[0].pts[1] = Point{float32((p0.X + wp1.X) * scale), float32((p0.Y + wp1.Y) * scale)}
	dst[0].pts[2] = m
	dst[1].pts[0] = m
	dst[1].pts[1] = Point{float32((wp1.X + p2.X) * scale), float32((wp1.Y + p2.Y) * scale)}
	dst[1].pts[2] = p2
	dst[0].w, dst[1].w = newW, newW
}

func between(a, b, c float32) bool { return float32((a-b)*(c-b)) <= 0 }

func subdivideConic(src *conic, pts []Point, level int) []Point {
	if level == 0 {
		pts[0], pts[1] = src.pts[1], src.pts[2]
		return pts[2:]
	}
	var dst [2]conic
	src.chop(&dst)
	startY := src.pts[0].Y
	endY := src.pts[2].Y
	if between(startY, src.pts[1].Y, endY) {
		midY := dst[0].pts[2].Y
		if !between(startY, midY, endY) {
			closerY := endY
			if abs32f(midY-startY) < abs32f(midY-endY) {
				closerY = startY
			}
			dst[0].pts[2].Y = closerY
			dst[1].pts[0].Y = closerY
		}
		if !between(startY, dst[0].pts[1].Y, dst[0].pts[2].Y) {
			dst[0].pts[1].Y = startY
		}
		if !between(dst[1].pts[0].Y, dst[1].pts[1].Y, endY) {
			dst[1].pts[1].Y = endY
		}
	}
	level--
	pts = subdivideConic(&dst[0], pts, level)
	return subdivideConic(&dst[1], pts, level)
}

// ptsEqualWithinTolerance is SkPointPriv::EqualsWithinTolerance(p1, p2): the
// difference cannot be normalized.
func ptsEqualWithinTolerance(a, b Point) bool {
	dx, dy := a.X-b.X, a.Y-b.Y
	return !(isFinite32(dx) && isFinite32(dy) && (dx != 0 || dy != 0))
}

// chopIntoQuadsPOW2 is SkConic::chopIntoQuadsPOW2. pts receives
// 2*2^pow2 + 1 points. It returns the number of quads.
func (c *conic) chopIntoQuadsPOW2(pts []Point, pow2 int) int {
	if c.w < 0 || !isFinite32(c.w) {
		pow2 = 0
	}
	pts[0] = c.pts[0]
	done := false
	if pow2 == maxConicToQuadPOW2 {
		var dst [2]conic
		c.chop(&dst)
		if ptsEqualWithinTolerance(dst[0].pts[1], dst[0].pts[2]) && ptsEqualWithinTolerance(dst[1].pts[0], dst[1].pts[1]) {
			pts[1], pts[2], pts[3] = dst[0].pts[1], dst[0].pts[1], dst[0].pts[1]
			pts[4] = dst[1].pts[2]
			pow2 = 1
			done = true
		}
	}
	if !done {
		subdivideConic(c, pts[1:], pow2)
	}
	quadCount := 1 << pow2
	ptCount := 2*quadCount + 1
	finite := true
	for i := 0; i < ptCount; i++ {
		if !pts[i].isFinite() {
			finite = false
			break
		}
	}
	if !finite {
		for i := 1; i < ptCount-1; i++ {
			pts[i] = c.pts[1]
		}
	}
	return quadCount
}

// conicToQuads is SkAutoConicToQuads::computeQuads: the conic as quads
// within the tolerance. The result has 2*n + 1 points.
func conicToQuads(p0, p1, p2 Point, w, tol float32, buf []Point) []Point {
	c := conic{pts: [3]Point{p0, p1, p2}, w: w}
	pow2 := c.computeQuadPOW2(tol)
	n := 1 << pow2
	need := 2*n + 1
	if cap(buf) < need {
		buf = make([]Point, need)
	}
	buf = buf[:need]
	n = c.chopIntoQuadsPOW2(buf, pow2)
	return buf[:2*n+1]
}

// evalQuadAt is SkEvalQuadAt (SkQuadCoeff::eval).
func evalQuadAt(src []Point, t float32) Point {
	ax := src[2].X - float32(2*src[1].X) + src[0].X
	ay := src[2].Y - float32(2*src[1].Y) + src[0].Y
	bx := float32(2 * (src[1].X - src[0].X))
	by := float32(2 * (src[1].Y - src[0].Y))
	return Point{
		float32((float32(ax*t)+bx)*t) + src[0].X,
		float32((float32(ay*t)+by)*t) + src[0].Y,
	}
}

// evalQuadTangentAt is SkEvalQuadTangentAt.
func evalQuadTangentAt(src []Point, t float32) Point {
	if (t == 0 && src[0] == src[1]) || (t == 1 && src[1] == src[2]) {
		return src[2].sub(src[0])
	}
	b := src[1].sub(src[0])
	a := src[2].sub(src[1]).sub(b)
	tt := Point{float32(a.X*t) + b.X, float32(a.Y*t) + b.Y}
	return Point{tt.X + tt.X, tt.Y + tt.Y}
}

// evalCubicAt is SkEvalCubicAt for the position (SkCubicCoeff::eval).
func evalCubicPosAt(src []Point, t float32) Point {
	p0, p1, p2, p3 := src[0], src[1], src[2], src[3]
	ax := p3.X + float32(3*(p1.X-p2.X)) - p0.X
	ay := p3.Y + float32(3*(p1.Y-p2.Y)) - p0.Y
	bx := float32(3 * (p2.X - float32(2*p1.X) + p0.X))
	by := float32(3 * (p2.Y - float32(2*p1.Y) + p0.Y))
	cx := float32(3 * (p1.X - p0.X))
	cy := float32(3 * (p1.Y - p0.Y))
	return Point{
		float32((float32((float32(ax*t)+bx)*t)+cx)*t) + p0.X,
		float32((float32((float32(ay*t)+by)*t)+cy)*t) + p0.Y,
	}
}
