// Ported from skia/src/core/SkLineClipper.cpp and SkEdgeClipper.cpp (Skia
// branch chrome/m156).

package raster

import "math"

func pinUnsorted64(value, limit0, limit1 float64) float64 {
	if limit1 < limit0 {
		limit0, limit1 = limit1, limit0
	}
	if value < limit0 {
		value = limit0
	} else if value > limit1 {
		value = limit1
	}
	return value
}

func pinUnsorted32(value, limit0, limit1 float32) float32 {
	if limit1 < limit0 {
		limit0, limit1 = limit1, limit0
	}
	if value < limit0 {
		value = limit0
	} else if value > limit1 {
		value = limit1
	}
	return value
}

const scalarNearlyZero = float32(1.0 / (1 << 12))

func nearlyZero(v float32) bool { return abs32f(v) <= scalarNearlyZero }

func midpoint32(a, b float32) float32 { return float32(0.5 * (float64(a) + float64(b))) }

func sectWithHorizontal(src *[2]Point, y float32) float32 {
	dy := src[1].Y - src[0].Y
	if nearlyZero(dy) {
		return midpoint32(src[0].X, src[1].X)
	}
	x0, y0 := float64(src[0].X), float64(src[0].Y)
	x1, y1 := float64(src[1].X), float64(src[1].Y)
	result := x0 + float64((float64(y)-y0)*(x1-x0))/(y1-y0)
	return float32(pinUnsorted64(result, x0, x1))
}

func sectWithVertical(src *[2]Point, x float32) float32 {
	dx := src[1].X - src[0].X
	if nearlyZero(dx) {
		return midpoint32(src[0].Y, src[1].Y)
	}
	x0, y0 := float64(src[0].X), float64(src[0].Y)
	x1, y1 := float64(src[1].X), float64(src[1].Y)
	return float32(y0 + float64((float64(x)-x0)*(y1-y0))/(x1-x0))
}

func sectClampWithVertical(src *[2]Point, x float32) float32 {
	return pinUnsorted32(sectWithVertical(src, x), src[0].Y, src[1].Y)
}

// clipLine is SkLineClipper::ClipLine. It returns the number of lines; the
// lines are lines[i], lines[i+1].
func clipLine(p0, p1 Point, clip Rect, lines *[4]Point, canCullToTheRight bool) int {
	pts := [2]Point{p0, p1}
	index0, index1 := 0, 1
	if !(pts[0].Y < pts[1].Y) {
		index0, index1 = 1, 0
	}
	if pts[index1].Y <= clip.T || pts[index0].Y >= clip.B {
		return 0
	}
	tmp := pts
	if pts[index0].Y < clip.T {
		tmp[index0] = Point{sectWithHorizontal(&pts, clip.T), clip.T}
	}
	if tmp[index1].Y > clip.B {
		tmp[index1] = Point{sectWithHorizontal(&pts, clip.B), clip.B}
	}
	var storage [4]Point
	var result []Point
	lineCount := 1
	var reverse bool
	if pts[0].X < pts[1].X {
		index0, index1 = 0, 1
		reverse = false
	} else {
		index0, index1 = 1, 0
		reverse = true
	}
	if tmp[index1].X <= clip.L {
		tmp[0].X, tmp[1].X = clip.L, clip.L
		result = tmp[:]
		reverse = false
	} else if tmp[index0].X >= clip.R {
		if canCullToTheRight {
			return 0
		}
		tmp[0].X, tmp[1].X = clip.R, clip.R
		result = tmp[:]
		reverse = false
	} else {
		r := 0
		if tmp[index0].X < clip.L {
			storage[r] = Point{clip.L, tmp[index0].Y}
			r++
			storage[r] = Point{clip.L, sectClampWithVertical(&tmp, clip.L)}
		} else {
			storage[r] = tmp[index0]
		}
		r++
		if tmp[index1].X > clip.R {
			storage[r] = Point{clip.R, sectClampWithVertical(&tmp, clip.R)}
			r++
			storage[r] = Point{clip.R, tmp[index1].Y}
		} else {
			storage[r] = tmp[index1]
		}
		lineCount = r
		result = storage[:]
	}
	if reverse {
		for i := 0; i <= lineCount; i++ {
			lines[lineCount-i] = result[i]
		}
	} else {
		copy(lines[:lineCount+1], result[:lineCount+1])
	}
	return lineCount
}

// edgeClipper is SkEdgeClipper: it clips one segment to a rectangle and
// keeps the pieces. Parts to the left or right of the clip become vertical
// lines on the clip edge, so that the winding stays correct.
type edgeClipper struct {
	canCullToTheRight bool
	verbs             []pathVerb
	offs              []int
	pts               []Point
}

func (c *edgeClipper) begin() {
	c.verbs, c.offs, c.pts = c.verbs[:0], c.offs[:0], c.pts[:0]
}

func (c *edgeClipper) appendLine(p0, p1 Point) {
	c.verbs = append(c.verbs, verbLine)
	c.offs = append(c.offs, len(c.pts))
	c.pts = append(c.pts, p0, p1)
}

func (c *edgeClipper) appendVLine(x, y0, y1 float32, reverse bool) {
	if reverse {
		y0, y1 = y1, y0
	}
	c.appendLine(Point{x, y0}, Point{x, y1})
}

func (c *edgeClipper) appendQuad(pts []Point, reverse bool) {
	c.verbs = append(c.verbs, verbQuad)
	c.offs = append(c.offs, len(c.pts))
	if reverse {
		c.pts = append(c.pts, pts[2], pts[1], pts[0])
	} else {
		c.pts = append(c.pts, pts[0], pts[1], pts[2])
	}
}

func (c *edgeClipper) appendCubic(pts []Point, reverse bool) {
	c.verbs = append(c.verbs, verbCubic)
	c.offs = append(c.offs, len(c.pts))
	if reverse {
		c.pts = append(c.pts, pts[3], pts[2], pts[1], pts[0])
	} else {
		c.pts = append(c.pts, pts[0], pts[1], pts[2], pts[3])
	}
}

func (c *edgeClipper) clipLine(p0, p1 Point, clip Rect) bool {
	c.begin()
	var lines [4]Point
	n := clipLine(p0, p1, clip, &lines, c.canCullToTheRight)
	for i := 0; i < n; i++ {
		c.appendLine(lines[i], lines[i+1])
	}
	return len(c.verbs) > 0
}

func chopMonoQuadAt(c0, c1, c2, target float32) (float32, bool) {
	a := c0 - c1 - c1 + c2
	b := float32(2 * (c1 - c0))
	cc := c0 - target
	var roots [2]float32
	if findUnitQuadRoots(a, b, cc, &roots) > 0 {
		return roots[0], true
	}
	return 0, false
}

func chopQuadInY(pts *[3]Point, clip Rect) {
	var tmp [5]Point
	if pts[0].Y < clip.T {
		if t, ok := chopMonoQuadAt(pts[0].Y, pts[1].Y, pts[2].Y, clip.T); ok {
			chopQuadAt(pts[:], &tmp, t)
			tmp[2].Y = clip.T
			if tmp[3].Y < clip.T {
				tmp[3].Y = clip.T
			}
			pts[0] = tmp[2]
			pts[1] = tmp[3]
		} else {
			for i := range pts {
				if pts[i].Y < clip.T {
					pts[i].Y = clip.T
				}
			}
		}
	}
	if pts[2].Y > clip.B {
		if t, ok := chopMonoQuadAt(pts[0].Y, pts[1].Y, pts[2].Y, clip.B); ok {
			chopQuadAt(pts[:], &tmp, t)
			if tmp[1].Y > clip.B {
				tmp[1].Y = clip.B
			}
			tmp[2].Y = clip.B
			pts[1] = tmp[1]
			pts[2] = tmp[2]
		} else {
			for i := range pts {
				if pts[i].Y > clip.B {
					pts[i].Y = clip.B
				}
			}
		}
	}
}

func (c *edgeClipper) clipMonoQuad(src []Point, clip Rect) {
	var pts [3]Point
	reverse := false
	if src[0].Y > src[2].Y {
		pts = [3]Point{src[2], src[1], src[0]}
		reverse = true
	} else {
		pts = [3]Point{src[0], src[1], src[2]}
	}
	if pts[2].Y <= clip.T || pts[0].Y >= clip.B {
		return
	}
	chopQuadInY(&pts, clip)
	if pts[0].X > pts[2].X {
		pts[0], pts[2] = pts[2], pts[0]
		reverse = !reverse
	}
	if pts[2].X <= clip.L {
		c.appendVLine(clip.L, pts[0].Y, pts[2].Y, reverse)
		return
	}
	if pts[0].X >= clip.R {
		if !c.canCullToTheRight {
			c.appendVLine(clip.R, pts[0].Y, pts[2].Y, reverse)
		}
		return
	}
	var tmp [5]Point
	if pts[0].X < clip.L {
		if t, ok := chopMonoQuadAt(pts[0].X, pts[1].X, pts[2].X, clip.L); ok {
			chopQuadAt(pts[:], &tmp, t)
			c.appendVLine(clip.L, tmp[0].Y, tmp[2].Y, reverse)
			tmp[2].X = clip.L
			if tmp[3].X < clip.L {
				tmp[3].X = clip.L
			}
			pts[0] = tmp[2]
			pts[1] = tmp[3]
		} else {
			c.appendVLine(clip.L, pts[0].Y, pts[2].Y, reverse)
			return
		}
	}
	if pts[2].X > clip.R {
		if t, ok := chopMonoQuadAt(pts[0].X, pts[1].X, pts[2].X, clip.R); ok {
			chopQuadAt(pts[:], &tmp, t)
			if tmp[1].X > clip.R {
				tmp[1].X = clip.R
			}
			tmp[2].X = clip.R
			c.appendQuad(tmp[:3], reverse)
			c.appendVLine(clip.R, tmp[2].Y, tmp[4].Y, reverse)
		} else {
			pts[1].X = min(pts[1].X, clip.R)
			pts[2].X = min(pts[2].X, clip.R)
			c.appendQuad(pts[:], reverse)
		}
	} else {
		c.appendQuad(pts[:], reverse)
	}
}

func boundsOf(pts []Point) Rect {
	r := Rect{pts[0].X, pts[0].Y, pts[0].X, pts[0].Y}
	for _, p := range pts[1:] {
		r.L, r.R = min(r.L, p.X), max(r.R, p.X)
		r.T, r.B = min(r.T, p.Y), max(r.B, p.Y)
	}
	return r
}

func (c *edgeClipper) clipQuad(src []Point, clip Rect) bool {
	c.begin()
	bounds := boundsOf(src[:3])
	if !(bounds.T >= clip.B || bounds.B <= clip.T) {
		var monoY [5]Point
		countY := chopQuadAtYExtrema(src, &monoY)
		for y := 0; y <= countY; y++ {
			var monoX [5]Point
			countX := chopQuadAtXExtrema(monoY[y*2:y*2+3], &monoX)
			for x := 0; x <= countX; x++ {
				c.clipMonoQuad(monoX[x*2:x*2+3], clip)
			}
		}
	}
	return len(c.verbs) > 0
}

func monoCubicClosestT(c0, c1, c2, c3, x float32) float32 {
	t := float32(0.5)
	var lastT, bestT float32
	step := float32(0.25)
	d := c0
	a := c3 + float32(3*(c1-c2)) - d
	b := float32(3 * (c2 - c1 - c1 + d))
	cc := float32(3 * (c1 - d))
	x -= d
	closest := float32(math.MaxFloat32)
	for {
		loc := float32((float32((float32(a*t)+b)*t) + cc) * t)
		dist := abs32f(loc - x)
		if closest > dist {
			closest = dist
			bestT = t
		}
		lastT = t
		if loc < x {
			t += step
		} else {
			t -= step
		}
		step *= 0.5
		if !(closest > 0.25 && lastT != t) {
			break
		}
	}
	return bestT
}

// chopMonoCubicAt is SkChopMonoCubicAtY and SkChopMonoCubicAtX: it finds
// the parameter where a monotonic cubic crosses a coordinate and cuts the
// cubic there, in float64. Skia solves the cubic equation in closed form;
// this port finds the same root by bisection.
func chopMonoCubicAt(src []Point, target float32, useY bool, dst *[7]Point) bool {
	coord := func(p Point) float64 {
		if useY {
			return float64(p.Y)
		}
		return float64(p.X)
	}
	c0, c1, c2, c3 := coord(src[0]), coord(src[1]), coord(src[2]), coord(src[3])
	a := -c0 + float64(3*c1) - float64(3*c2) + c3
	b := float64(3*c0) - float64(6*c1) + float64(3*c2)
	cc := float64(-3*c0) + float64(3*c1)
	d := c0 - float64(target)
	eval := func(t float64) float64 {
		v := float64(a*t) + b
		v = float64(v*t) + cc
		return float64(v*t) + d
	}
	f0, f1 := eval(0), eval(1)
	if f0 == 0 {
		return false
	}
	if (f0 < 0) == (f1 < 0) && f1 != 0 {
		return false
	}
	lo, hi := 0.0, 1.0
	for i := 0; i < 64; i++ {
		mid := (lo + hi) / 2
		fm := eval(mid)
		if fm == 0 {
			lo, hi = mid, mid
			break
		}
		if (fm < 0) == (f0 < 0) {
			lo = mid
		} else {
			hi = mid
		}
	}
	t := (lo + hi) / 2
	if !(t > 0 && t < 1) || math.Abs(eval(t)) >= 0.00001 {
		return false
	}
	type p64 struct{ x, y float64 }
	q := [4]p64{}
	for i := range q {
		q[i] = p64{float64(src[i].X), float64(src[i].Y)}
	}
	mix := func(u, v p64) p64 { return p64{u.x + float64((v.x-u.x)*t), u.y + float64((v.y-u.y)*t)} }
	ab, bc, cd := mix(q[0], q[1]), mix(q[1], q[2]), mix(q[2], q[3])
	abc, bcd := mix(ab, bc), mix(bc, cd)
	abcd := mix(abc, bcd)
	out := [7]p64{q[0], ab, abc, abcd, bcd, cd, q[3]}
	for i := range out {
		dst[i] = Point{float32(out[i].x), float32(out[i].y)}
	}
	return true
}

func chopMonoCubicAtY(src []Point, y float32, dst *[7]Point) {
	if chopMonoCubicAt(src, y, true, dst) {
		return
	}
	chopCubicAt(src, dst[:], monoCubicClosestT(src[0].Y, src[1].Y, src[2].Y, src[3].Y, y))
}

func chopMonoCubicAtX(src []Point, x float32, dst *[7]Point) {
	if chopMonoCubicAt(src, x, false, dst) {
		return
	}
	chopCubicAt(src, dst[:], monoCubicClosestT(src[0].X, src[1].X, src[2].X, src[3].X, x))
}

func chopCubicInY(pts *[4]Point, clip Rect) {
	if pts[0].Y < clip.T {
		var tmp [7]Point
		chopMonoCubicAtY(pts[:], clip.T, &tmp)
		if tmp[3].Y < clip.T && tmp[4].Y < clip.T && tmp[5].Y < clip.T {
			tmp2 := [4]Point{tmp[3], tmp[4], tmp[5], tmp[6]}
			chopMonoCubicAtY(tmp2[:], clip.T, &tmp)
		}
		tmp[3].Y = clip.T
		if tmp[4].Y < clip.T {
			tmp[4].Y = clip.T
		}
		pts[0], pts[1], pts[2] = tmp[3], tmp[4], tmp[5]
	}
	if pts[3].Y > clip.B {
		var tmp [7]Point
		chopMonoCubicAtY(pts[:], clip.B, &tmp)
		tmp[3].Y = clip.B
		if tmp[2].Y > clip.B {
			tmp[2].Y = clip.B
		}
		pts[1], pts[2], pts[3] = tmp[1], tmp[2], tmp[3]
	}
}

func (c *edgeClipper) clipMonoCubic(src []Point, clip Rect) {
	var pts [4]Point
	reverse := false
	if src[0].Y > src[3].Y {
		pts = [4]Point{src[3], src[2], src[1], src[0]}
		reverse = true
	} else {
		pts = [4]Point{src[0], src[1], src[2], src[3]}
	}
	if pts[3].Y <= clip.T || pts[0].Y >= clip.B {
		return
	}
	chopCubicInY(&pts, clip)
	if pts[0].X > pts[3].X {
		pts[0], pts[3] = pts[3], pts[0]
		pts[1], pts[2] = pts[2], pts[1]
		reverse = !reverse
	}
	if pts[3].X <= clip.L {
		c.appendVLine(clip.L, pts[0].Y, pts[3].Y, reverse)
		return
	}
	if pts[0].X >= clip.R {
		if !c.canCullToTheRight {
			c.appendVLine(clip.R, pts[0].Y, pts[3].Y, reverse)
		}
		return
	}
	if pts[0].X < clip.L {
		var tmp [7]Point
		chopMonoCubicAtX(pts[:], clip.L, &tmp)
		c.appendVLine(clip.L, tmp[0].Y, tmp[3].Y, reverse)
		tmp[3].X = clip.L
		if tmp[4].X < clip.L {
			tmp[4].X = clip.L
		}
		pts[0], pts[1], pts[2] = tmp[3], tmp[4], tmp[5]
	}
	if pts[3].X > clip.R {
		var tmp [7]Point
		chopMonoCubicAtX(pts[:], clip.R, &tmp)
		tmp[3].X = clip.R
		if tmp[2].X > clip.R {
			tmp[2].X = clip.R
		}
		c.appendCubic(tmp[:4], reverse)
		c.appendVLine(clip.R, tmp[3].Y, tmp[6].Y, reverse)
	} else {
		c.appendCubic(pts[:], reverse)
	}
}

func (c *edgeClipper) clipCubic(src []Point, clip Rect) bool {
	c.begin()
	bounds := boundsOf(src[:4])
	if bounds.B > clip.T && bounds.T < clip.B {
		const limit = float32(1 << 22)
		if bounds.L < -limit || bounds.T < -limit || bounds.R > limit || bounds.B > limit {
			return c.clipLine(src[0], src[3], clip)
		}
		var monoY [10]Point
		countY := chopCubicAtYExtrema(src, &monoY)
		for y := 0; y <= countY; y++ {
			var monoX [10]Point
			countX := chopCubicAtXExtrema(monoY[y*3:y*3+4], &monoX)
			for x := 0; x <= countX; x++ {
				c.clipMonoCubic(monoX[x*3:x*3+4], clip)
			}
		}
	}
	return len(c.verbs) > 0
}
