// Ported from skia/src/core/SkScan_Antihair.cpp and SkScan_Hairline.cpp
// (Skia branch chrome/m156): anti-aliased hairlines. Skia draws a stroke
// that is at most one device pixel wide as a hairline whose alpha is
// modulated by the stroke width (see drawPath in draw.go).

package raster

import "math"

func scaleAlphaByCoverage(value uint8, coverage fdot6) uint8 {
	return uint8((int32(value) * coverage) >> 6)
}

func fixedFracAlpha(f fixed) uint8 { return uint8((f >> 8) & 0xFF) }

type hairKind uint8

const (
	hairHLine hairKind = iota
	hairHorish
	hairVLine
	hairVertish
)

func hairDrawCap(kind hairKind, b blitter, x int, f, slope fixed, coverage fdot6) fixed {
	switch kind {
	case hairHLine:
		f += fixedHalf
		y := fixedFloorToInt(f)
		a := fixedFracAlpha(f)
		if ma := scaleAlphaByCoverage(a, coverage); ma != 0 {
			callHLineBlitter(b, x, y, 1, ma)
		}
		if ma := scaleAlphaByCoverage(255-a, coverage); ma != 0 {
			callHLineBlitter(b, x, y-1, 1, ma)
		}
		return f - fixedHalf
	case hairHorish:
		f += fixedHalf
		lowerY := fixedFloorToInt(f)
		a := fixedFracAlpha(f)
		a0 := scaleAlphaByCoverage(255-a, coverage)
		a1 := scaleAlphaByCoverage(a, coverage)
		b.blitAntiV2(x, lowerY-1, a0, a1)
		return f + slope - fixedHalf
	case hairVLine:
		f += fixedHalf
		px := fixedFloorToInt(f)
		a := fixedFracAlpha(f)
		if ma := scaleAlphaByCoverage(a, coverage); ma != 0 {
			b.blitV(px, x, 1, ma)
		}
		if ma := scaleAlphaByCoverage(255-a, coverage); ma != 0 {
			b.blitV(px-1, x, 1, ma)
		}
		return f - fixedHalf
	default:
		f += fixedHalf
		px := fixedFloorToInt(f)
		a := fixedFracAlpha(f)
		b.blitAntiH2(px-1, x, scaleAlphaByCoverage(255-a, coverage), scaleAlphaByCoverage(a, coverage))
		return f + slope - fixedHalf
	}
}

func hairDrawLine(kind hairKind, b blitter, x, stopx int, f, slope fixed) fixed {
	switch kind {
	case hairHLine:
		count := stopx - x
		f += fixedHalf
		y := fixedFloorToInt(f)
		a := fixedFracAlpha(f)
		if a != 0 {
			callHLineBlitter(b, x, y, count, a)
		}
		a = 255 - a
		if a != 0 {
			callHLineBlitter(b, x, y-1, count, a)
		}
		return f - fixedHalf
	case hairHorish:
		f += fixedHalf
		for {
			lowerY := fixedFloorToInt(f)
			a := fixedFracAlpha(f)
			b.blitAntiV2(x, lowerY-1, 255-a, a)
			f += slope
			x++
			if x >= stopx {
				break
			}
		}
		return f - fixedHalf
	case hairVLine:
		f += fixedHalf
		px := fixedFloorToInt(f)
		a := fixedFracAlpha(f)
		if a != 0 {
			b.blitV(px, x, stopx-x, a)
		}
		a = 255 - a
		if a != 0 {
			b.blitV(px-1, x, stopx-x, a)
		}
		return f - fixedHalf
	default:
		f += fixedHalf
		for {
			px := fixedFloorToInt(f)
			a := fixedFracAlpha(f)
			b.blitAntiH2(px-1, x, 255-a, a)
			f += slope
			x++
			if x >= stopx {
				break
			}
		}
		return f - fixedHalf
	}
}

func fd6Frac(x fdot6) fdot6 { return x & 63 }

func partialPixelCoverage(pos fdot6) fdot6 { return fd6Frac(pos-1) + 1 }

func fastFixDiv(a, b fdot6) fixed { return fixed(uint32(a)<<16) / b }

// doAntiHairline is do_anti_hairline.
func doAntiHairline(x0, y0, x1, y1 fdot6, clip *IRect, b blitter) {
	if x0 == minS32 || y0 == minS32 || x1 == minS32 || y1 == minS32 {
		return
	}
	if abs32(x1-x0) > 511<<6 || abs32(y1-y0) > 511<<6 {
		hx := (x0 >> 1) + (x1 >> 1)
		hy := (y0 >> 1) + (y1 >> 1)
		doAntiHairline(x0, y0, hx, hy, clip, b)
		doAntiHairline(hx, hy, x1, y1, clip, b)
		return
	}
	var startCoverage, stopCoverage fdot6
	var istart, istop int
	var fstart, slope fixed
	var kind hairKind

	if abs32(x1-x0) > abs32(y1-y0) {
		if x0 > x1 {
			x0, x1 = x1, x0
			y0, y1 = y1, y0
		}
		istart = fdot6Floor(x0)
		istop = fdot6Ceil(x1)
		if y0 == y1 {
			slope = 0
			kind = hairHLine
			fstart = fdot6ToFixed(y0)
		} else {
			slope = fastFixDiv(y1-y0, x1-x0)
			dxToCenter := 32 - fd6Frac(x0)
			fstart = fdot6ToFixed(y0) + ((slope*dxToCenter + 32) >> 6)
			kind = hairHorish
		}
		if istop-istart == 1 {
			startCoverage = x1 - x0
			stopCoverage = 0
		} else {
			startCoverage = 64 - fd6Frac(x0)
			stopCoverage = fd6Frac(x1)
		}
		if clip != nil {
			if istart >= clip.R || istop <= clip.L {
				return
			}
			if istart < clip.L {
				fstart += slope * fixed(clip.L-istart)
				istart = clip.L
				startCoverage = 64
				if istop-istart == 1 {
					startCoverage = partialPixelCoverage(x1)
					stopCoverage = 0
				}
			}
			if istop > clip.R {
				istop = clip.R
				stopCoverage = 0
			}
			if istart == istop {
				return
			}
			var top, bottom int
			if slope >= 0 {
				top = fixedFloorToInt(fstart - fixedHalf)
				bottom = fixedCeilToInt(fstart + fixed(istop-istart-1)*slope + fixedHalf)
			} else {
				bottom = fixedCeilToInt(fstart + fixedHalf)
				top = fixedFloorToInt(fstart + fixed(istop-istart-1)*slope - fixedHalf)
			}
			top--
			bottom++
			if top >= clip.B || bottom <= clip.T {
				return
			}
			if clip.T <= top && clip.B >= bottom {
				clip = nil
			}
		}
	} else {
		if y0 > y1 {
			x0, x1 = x1, x0
			y0, y1 = y1, y0
		}
		istart = fdot6Floor(y0)
		istop = fdot6Ceil(y1)
		if x0 == x1 {
			if y0 == y1 {
				return
			}
			slope = 0
			kind = hairVLine
			fstart = fdot6ToFixed(x0)
		} else {
			slope = fastFixDiv(x1-x0, y1-y0)
			dyToCenter := 32 - fd6Frac(y0)
			fstart = fdot6ToFixed(x0) + ((slope*dyToCenter + 32) >> 6)
			kind = hairVertish
		}
		if istop-istart == 1 {
			startCoverage = y1 - y0
			stopCoverage = 0
		} else {
			startCoverage = 64 - fd6Frac(y0)
			stopCoverage = fd6Frac(y1)
		}
		if clip != nil {
			if istart >= clip.B || istop <= clip.T {
				return
			}
			if istart < clip.T {
				fstart += slope * fixed(clip.T-istart)
				istart = clip.T
				startCoverage = 64
				if istop-istart == 1 {
					startCoverage = partialPixelCoverage(y1)
					stopCoverage = 0
				}
			}
			if istop > clip.B {
				istop = clip.B
				stopCoverage = 0
			}
			if istart == istop {
				return
			}
			var left, right int
			if slope >= 0 {
				left = fixedFloorToInt(fstart - fixedHalf)
				right = fixedCeilToInt(fstart + fixed(istop-istart-1)*slope + fixedHalf)
			} else {
				right = fixedCeilToInt(fstart + fixedHalf)
				left = fixedFloorToInt(fstart + fixed(istop-istart-1)*slope - fixedHalf)
			}
			left--
			right++
			if left >= clip.R || right <= clip.L {
				return
			}
			if clip.L <= left && clip.R >= right {
				clip = nil
			}
		}
	}

	var rectClipper rectClipBlitter
	if clip != nil {
		rectClipper = rectClipBlitter{b: b, clip: *clip}
		b = &rectClipper
	}

	fstart = hairDrawCap(kind, b, istart, fstart, slope, startCoverage)
	istart++
	fullSpans := istop - istart
	if stopCoverage > 0 {
		fullSpans--
	}
	if fullSpans > 0 {
		fstart = hairDrawLine(kind, b, istart, istart+fullSpans, fstart, slope)
	}
	if stopCoverage > 0 {
		hairDrawCap(kind, b, istop-1, fstart, slope, stopCoverage)
	}
}

// intersectLine is SkLineClipper::IntersectLine.
func intersectLine(src [2]Point, clip Rect) ([2]Point, bool) {
	bounds := Rect{min(src[0].X, src[1].X), min(src[0].Y, src[1].Y), max(src[0].X, src[1].X), max(src[0].Y, src[1].Y)}
	if clip.L <= bounds.L && clip.T <= bounds.T && clip.R >= bounds.R && clip.B >= bounds.B {
		return src, true
	}
	nestedLT := func(a, b, dim float32) bool { return a <= b && (a < b || dim > 0) }
	if nestedLT(bounds.R, clip.L, bounds.width()) || nestedLT(clip.R, bounds.L, bounds.width()) ||
		nestedLT(bounds.B, clip.T, bounds.height()) || nestedLT(clip.B, bounds.T, bounds.height()) {
		return src, false
	}
	index0, index1 := 0, 1
	if !(src[0].Y < src[1].Y) {
		index0, index1 = 1, 0
	}
	tmp := src
	if tmp[index0].Y < clip.T {
		tmp[index0] = Point{sectWithHorizontal(&src, clip.T), clip.T}
	}
	if tmp[index1].Y > clip.B {
		tmp[index1] = Point{sectWithHorizontal(&src, clip.B), clip.B}
	}
	if tmp[0].X < tmp[1].X {
		index0, index1 = 0, 1
	} else {
		index0, index1 = 1, 0
	}
	if tmp[index1].X <= clip.L || tmp[index0].X >= clip.R {
		if tmp[0].X != tmp[1].X || tmp[0].X < clip.L || tmp[0].X > clip.R {
			return src, false
		}
	}
	if tmp[index0].X < clip.L {
		tmp[index0] = Point{clip.L, sectWithVertical(&tmp, clip.L)}
	}
	if tmp[index1].X > clip.R {
		tmp[index1] = Point{clip.R, sectWithVertical(&tmp, clip.R)}
	}
	return tmp, true
}

// antiHairLineRgn is SkScan::AntiHairLineRgn for a rectangular clip. clip
// is nil when the lines are inside the clip.
func antiHairLineRgn(pts []Point, clip *IRect, b blitter) {
	if len(pts) == 0 || (clip != nil && clip.isEmpty()) {
		return
	}
	const maxCoord = float32(32767)
	fixedBounds := Rect{-maxCoord, -maxCoord, maxCoord, maxCoord}
	var clipBounds Rect
	if clip != nil {
		clipBounds = clip.toRect().outset(1, 1)
	}
	for i := 0; i+1 < len(pts); i++ {
		seg, ok := intersectLine([2]Point{pts[i], pts[i+1]}, fixedBounds)
		if !ok {
			continue
		}
		if clip != nil {
			if seg, ok = intersectLine(seg, clipBounds); !ok {
				continue
			}
		}
		x0 := scalarToFDot6(seg[0].X)
		y0 := scalarToFDot6(seg[0].Y)
		x1 := scalarToFDot6(seg[1].X)
		y1 := scalarToFDot6(seg[1].Y)
		if clip != nil {
			left, top := min(x0, x1), min(y0, y1)
			right, bottom := max(x0, x1), max(y0, y1)
			ir := IRect{fdot6Floor(left) - 1, fdot6Floor(top) - 1, fdot6Ceil(right) + 1, fdot6Ceil(bottom) + 1}
			if ir.isEmpty() || ir.L >= clip.R || clip.L >= ir.R || ir.T >= clip.B || clip.T >= ir.B {
				continue
			}
			if !clip.contains(ir) {
				doAntiHairline(x0, y0, x1, y1, clip, b)
				continue
			}
		}
		doAntiHairline(x0, y0, x1, y1, nil, b)
	}
}

const (
	maxCubicSubdivideLevel = 9
	maxQuadSubdivideLevel  = 5
)

type hairParams struct {
	clip       *IRect
	insetClip  *Rect
	outsetClip *Rect
	b          blitter
	tmp        []Point
}

func geometricOverlap(a, b Rect) bool {
	return a.L < b.R && b.L < a.R && a.T < b.B && b.T < a.B
}

func geometricContains(outer, inner Rect) bool {
	return inner.R <= outer.R && inner.L >= outer.L && inner.B <= outer.B && inner.T >= outer.T
}

func computeIntQuadDist(pts []Point) uint32 {
	dx := abs32f(float32((pts[0].X+pts[2].X)/2) - pts[1].X)
	dy := abs32f(float32((pts[0].Y+pts[2].Y)/2) - pts[1].Y)
	idx := uint32(ceilToInt(dx))
	idy := uint32(ceilToInt(dy))
	if idx > idy {
		return idx + (idy >> 1)
	}
	return idy + (idx >> 1)
}

func computeQuadLevel(pts []Point) int {
	d := computeIntQuadDist(pts)
	level := (33 - clz32(d)) >> 1
	if level > maxQuadSubdivideLevel {
		level = maxQuadSubdivideLevel
	}
	return level
}

func (d *hairParams) scratch(n int) []Point {
	if cap(d.tmp) < n {
		d.tmp = make([]Point, n*2)
	}
	return d.tmp[:n]
}

// hairQuadLines is hair_quad.
func (d *hairParams) hairQuadLines(pts []Point, clip *IRect, level int) {
	lines := 1 << level
	ax := pts[2].X - float32(2*pts[1].X) + pts[0].X
	ay := pts[2].Y - float32(2*pts[1].Y) + pts[0].Y
	bx := float32(2 * (pts[1].X - pts[0].X))
	by := float32(2 * (pts[1].Y - pts[0].Y))
	cx, cy := pts[0].X, pts[0].Y
	var t float32
	dt := float32(1.0) / float32(lines)
	tmp := d.scratch(lines + 1)
	tmp[0] = pts[0]
	finite := true
	for i := 1; i < lines; i++ {
		t = t + dt
		p := Point{float32((float32(ax*t)+bx)*t) + cx, float32((float32(ay*t)+by)*t) + cy}
		finite = finite && p.isFinite()
		tmp[i] = p
	}
	if finite {
		tmp[lines] = pts[2]
		antiHairLineRgn(tmp, clip, d.b)
	}
}

func (d *hairParams) hairQuad(pts []Point, level int) {
	clip := d.clip
	if d.insetClip != nil {
		bounds := boundsOf(pts[:3])
		if !geometricOverlap(*d.outsetClip, bounds) {
			return
		} else if geometricContains(*d.insetClip, bounds) {
			clip = nil
		}
	}
	d.hairQuadLines(pts, clip, level)
}

func computeCubicSegs(pts []Point) int {
	const oneThird = float32(1.0 / 3.0)
	const twoThird = float32(2.0 / 3.0)
	p0, p1, p2, p3 := pts[0], pts[1], pts[2], pts[3]
	p13 := Point{float32(oneThird*p3.X) + float32(twoThird*p0.X), float32(oneThird*p3.Y) + float32(twoThird*p0.Y)}
	p23 := Point{float32(oneThird*p0.X) + float32(twoThird*p3.X), float32(oneThird*p0.Y) + float32(twoThird*p3.Y)}
	diff := max(max(abs32f(p1.X-p13.X), abs32f(p2.X-p23.X)), max(abs32f(p1.Y-p13.Y), abs32f(p2.Y-p23.Y)))
	tol := float32(1.0 / 8)
	for i := 0; i < maxCubicSubdivideLevel; i++ {
		if diff < tol {
			return 1 << i
		}
		tol *= 4
	}
	return 1 << maxCubicSubdivideLevel
}

func lt90(p0, pivot, p2 Point) bool {
	return dot(p0.sub(pivot), p2.sub(pivot)) >= 0
}

func quickCubicNicenessCheck(pts []Point) bool {
	return lt90(pts[1], pts[0], pts[3]) && lt90(pts[2], pts[0], pts[3]) &&
		lt90(pts[1], pts[3], pts[0]) && lt90(pts[2], pts[3], pts[0])
}

// hairCubicLines is hair_cubic.
func (d *hairParams) hairCubicLines(pts []Point, clip *IRect) {
	lines := computeCubicSegs(pts)
	if lines == 1 {
		antiHairLineRgn([]Point{pts[0], pts[3]}, clip, d.b)
		return
	}
	p0, p1, p2, p3 := pts[0], pts[1], pts[2], pts[3]
	ax := p3.X + float32(3*(p1.X-p2.X)) - p0.X
	ay := p3.Y + float32(3*(p1.Y-p2.Y)) - p0.Y
	bx := float32(3 * (p2.X - float32(2*p1.X) + p0.X))
	by := float32(3 * (p2.Y - float32(2*p1.Y) + p0.Y))
	cx := float32(3 * (p1.X - p0.X))
	cy := float32(3 * (p1.Y - p0.Y))
	var t float32
	dt := float32(1.0) / float32(lines)
	tmp := d.scratch(lines + 1)
	tmp[0] = pts[0]
	finite := true
	for i := 1; i < lines; i++ {
		t = t + dt
		p := Point{
			float32((float32((float32(ax*t)+bx)*t)+cx)*t) + p0.X,
			float32((float32((float32(ay*t)+by)*t)+cy)*t) + p0.Y,
		}
		finite = finite && p.isFinite()
		tmp[i] = p
	}
	if finite {
		tmp[lines] = pts[3]
		antiHairLineRgn(tmp, clip, d.b)
	}
}

func (d *hairParams) hairCubic(pts []Point) {
	clip := d.clip
	if d.insetClip != nil {
		bounds := boundsOf(pts[:4])
		if !geometricOverlap(*d.outsetClip, bounds) {
			return
		} else if geometricContains(*d.insetClip, bounds) {
			clip = nil
		}
	}
	if quickCubicNicenessCheck(pts) {
		d.hairCubicLines(pts, clip)
	} else {
		var tmp [13]Point
		count := chopCubicAtMaxCurvature(pts, &tmp)
		for i := 0; i < count; i++ {
			d.hairCubicLines(tmp[i*3:i*3+4], clip)
		}
	}
}

func bubbleSort3(a *[3]float32) {
	for i := 2; i > 0; i-- {
		for j := i; j > 0; j-- {
			if a[j] < a[j-1] {
				a[j], a[j-1] = a[j-1], a[j]
			}
		}
	}
}

func collapsDuplicates(a []float32) int {
	count := len(a)
	i := 0
	for n := count; n > 1; n-- {
		if a[i] == a[i+1] {
			for j := 1; j < n; j++ {
				a[i+j-1] = a[i+j]
			}
			count--
		} else {
			i++
		}
	}
	return count
}

func pinF(v, lo, hi float32) float32 {
	// SkTPin: std::max(lo, std::min(v, hi)).
	if !(v < hi) {
		v = hi
	}
	if !(lo < v) {
		v = lo
	}
	return v
}

// solveCubicPoly is solve_cubic_poly of SkGeometry.cpp. The float
// functions acosf, cosf and powf are computed in float64 and rounded.
func solveCubicPoly(coeff [4]float32, ts *[3]float32) int {
	if nearlyZero(coeff[0]) {
		var r [2]float32
		n := findUnitQuadRoots(coeff[1], coeff[2], coeff[3], &r)
		ts[0], ts[1] = r[0], r[1]
		return n
	}
	inva := 1 / coeff[0]
	a := float32(coeff[1] * inva)
	b := float32(coeff[2] * inva)
	c := float32(coeff[3] * inva)
	q := (float32(a*a) - float32(b*3)) / 9
	r := (float32(2*a*a*a) - float32(9*a*b) + float32(27*c)) / 54

	q3 := float32(q * q * q)
	r2MinusQ3 := float32(r*r) - q3
	adiv3 := a / 3

	if r2MinusQ3 < 0 {
		theta := float32(math.Acos(float64(pinF(r/sqrt32(q3), -1, 1))))
		neg2RootQ := float32(-2 * sqrt32(q))
		cosf := func(x float32) float32 { return float32(math.Cos(float64(x))) }
		const pi = float32(math.Pi)
		ts[0] = pinF(float32(neg2RootQ*cosf(theta/3))-adiv3, 0, 1)
		ts[1] = pinF(float32(neg2RootQ*cosf((theta+float32(2*pi))/3))-adiv3, 0, 1)
		ts[2] = pinF(float32(neg2RootQ*cosf((theta-float32(2*pi))/3))-adiv3, 0, 1)
		bubbleSort3(ts)
		return collapsDuplicates(ts[:])
	}
	A := abs32f(r) + sqrt32(r2MinusQ3)
	A = float32(math.Pow(float64(A), float64(float32(0.3333333))))
	if r > 0 {
		A = -A
	}
	if A != 0 {
		A += q / A
	}
	ts[0] = pinF(A-adiv3, 0, 1)
	return 1
}

func formulateF1DotF2(s0, s1, s2, s3 float32) [4]float32 {
	a := s1 - s0
	b := s2 - float32(2*s1) + s0
	c := s3 + float32(3*(s1-s2)) - s0
	return [4]float32{float32(c * c), float32(3 * b * c), float32(2*b*b) + float32(c*a), float32(a * b)}
}

// chopCubicAtMaxCurvature is SkChopCubicAtMaxCurvature. It returns the
// number of cubics in dst.
func chopCubicAtMaxCurvature(src []Point, dst *[13]Point) int {
	cx := formulateF1DotF2(src[0].X, src[1].X, src[2].X, src[3].X)
	cy := formulateF1DotF2(src[0].Y, src[1].Y, src[2].Y, src[3].Y)
	for i := range cx {
		cx[i] += cy[i]
	}
	var roots [3]float32
	rootCount := solveCubicPoly(cx, &roots)
	var ts [3]float32
	count := 0
	for i := 0; i < rootCount; i++ {
		if 0 < roots[i] && roots[i] < 1 {
			ts[count] = roots[i]
			count++
		}
	}
	if count == 0 {
		copy(dst[:4], src[:4])
	} else {
		chopCubicAtTs(src, dst[:], ts[:count])
	}
	return count + 1
}

// setLength is SkPoint::setLength.
func setLength(v Point, length float32) (Point, bool) {
	xx, yy := float64(v.X), float64(v.Y)
	dmag := math.Sqrt(float64(xx*xx) + float64(yy*yy))
	dscale := float64(length) / dmag
	x := float32(float64(v.X) * dscale)
	y := float32(float64(v.Y) * dscale)
	if !isFinite32(x) || !isFinite32(y) || (x == 0 && y == 0) {
		return Point{}, false
	}
	return Point{x, y}, true
}

type strokeCap uint8

const (
	capButt strokeCap = iota
	capRound
	capSquare
)

// extendPts is extend_pts: it moves the end points of a contour by the cap
// outset along the tangent.
func extendPts(cap strokeCap, prevVerb pathVerb, hasPrev bool, nextVerb pathVerb, hasNext bool, pts []Point) {
	capOutset := float32(0.5)
	if cap == capRound {
		capOutset = float32(math.Pi) / 8
	}
	n := len(pts)
	if hasPrev && prevVerb == verbMove {
		first := 0
		ctrl := 0
		controls := n - 1
		var tangent Point
		for {
			ctrl++
			tangent = pts[first].sub(pts[ctrl])
			controls--
			if !(tangent.isZero() && controls > 0) {
				break
			}
		}
		// The loop above decrements controls once more than Skia does when
		// it stops on a non-zero tangent; restore it.
		if !tangent.isZero() {
			controls++
		}
		if tangent.isZero() {
			tangent = Point{1, 0}
			controls = n - 1
		} else {
			tangent, _ = setLength(tangent, 1)
		}
		for {
			pts[first].X += float32(tangent.X * capOutset)
			pts[first].Y += float32(tangent.Y * capOutset)
			first++
			controls++
			if !(controls < n) {
				break
			}
		}
	}
	if !hasNext || nextVerb == verbMove || nextVerb == verbClose {
		last := n - 1
		ctrl := last
		controls := n - 1
		var tangent Point
		for {
			ctrl--
			tangent = pts[last].sub(pts[ctrl])
			controls--
			if !(tangent.isZero() && controls > 0) {
				break
			}
		}
		if !tangent.isZero() {
			controls++
		}
		if tangent.isZero() {
			tangent = Point{-1, 0}
			controls = n - 1
		} else {
			tangent, _ = setLength(tangent, 1)
		}
		for {
			pts[last].X += float32(tangent.X * capOutset)
			pts[last].Y += float32(tangent.Y * capOutset)
			last--
			controls++
			if !(controls < n) {
				break
			}
		}
	}
}

// isNextContourClosed is is_next_contour_closed: vi is the index after the
// move verb.
func isNextContourClosed(verbs []pathVerb, vi int) bool {
	if vi >= len(verbs) {
		return false
	}
	if verbs[vi] == verbClose {
		return true
	}
	if verbs[vi] == verbMove {
		return false
	}
	for i := vi + 1; i < len(verbs); i++ {
		if verbs[i] == verbMove {
			return false
		}
		if verbs[i] == verbClose {
			return true
		}
	}
	return false
}

// antiHairPath is hair_path with SkScan::AntiHairLineRgn for a rectangular
// clip: the anti-aliased hairline stroke of a path in device space.
func antiHairPath(p *Path, clipRect IRect, b blitter, cap strokeCap, scratch *[]Point) {
	if p.isEmpty() {
		return
	}
	var clip *IRect
	var insetStorage, outsetStorage Rect
	var insetClip, outsetClip *Rect
	{
		capOut := 1
		if cap != capButt {
			capOut = 2
		}
		ibounds := p.bounds().roundOut().outset(capOut, capOut)
		if clipRect.isEmpty() || ibounds.isEmpty() || ibounds.L >= clipRect.R || clipRect.L >= ibounds.R ||
			ibounds.T >= clipRect.B || clipRect.T >= ibounds.B {
			return
		}
		if !clipRect.contains(ibounds) {
			clip = &clipRect
			insetStorage = clipRect.toRect()
			outsetStorage = insetStorage.outset(1, 1)
			insetStorage = insetStorage.outset(-1, -1)
			if insetStorage.L > insetStorage.R || insetStorage.T > insetStorage.B {
				insetStorage = Rect{}
			}
			insetClip = &insetStorage
			outsetClip = &outsetStorage
		}
	}
	d := hairParams{clip: clip, insetClip: insetClip, outsetClip: outsetClip, b: b, tmp: *scratch}
	defer func() { *scratch = d.tmp }()

	var pts [4]Point
	var firstPt, lastPt Point
	isClosed := false
	isButt := cap == capButt
	var prevVerb pathVerb
	hasPrev := false
	var quads []Point

	pi, wi := 0, 0
	for vi, verb := range p.verbs {
		hasNext := vi+1 < len(p.verbs)
		var nextVerb pathVerb
		if hasNext {
			nextVerb = p.verbs[vi+1]
		}
		switch verb {
		case verbMove:
			firstPt = p.pts[pi]
			lastPt = firstPt
			pi++
			isClosed = !isButt && isNextContourClosed(p.verbs, vi+1)
		case verbLine:
			pts[0], pts[1] = p.pts[pi-1], p.pts[pi]
			pi++
			if !isButt && (!isClosed || pts[0] == pts[1]) {
				extendPts(cap, prevVerb, hasPrev, nextVerb, hasNext, pts[:2])
			}
			antiHairLineRgn(pts[:2], clip, b)
			lastPt = pts[1]
		case verbQuad:
			pts[0], pts[1], pts[2] = p.pts[pi-1], p.pts[pi], p.pts[pi+1]
			pi += 2
			if !isButt && (!isClosed || (pts[0] == pts[1] && pts[1] == pts[2])) {
				extendPts(cap, prevVerb, hasPrev, nextVerb, hasNext, pts[:3])
			}
			d.hairQuad(pts[:3], computeQuadLevel(pts[:3]))
			lastPt = pts[2]
		case verbConic:
			pts[0], pts[1], pts[2] = p.pts[pi-1], p.pts[pi], p.pts[pi+1]
			pi += 2
			w := p.conics[wi]
			wi++
			if !isButt && (!isClosed || (pts[0] == pts[1] && pts[1] == pts[2])) {
				extendPts(cap, prevVerb, hasPrev, nextVerb, hasNext, pts[:3])
			}
			quads = conicToQuads(pts[0], pts[1], pts[2], w, 0.25, quads)
			for i := 0; i+2 < len(quads); i += 2 {
				d.hairQuad(quads[i:i+3], computeQuadLevel(quads[i:i+3]))
			}
			lastPt = pts[2]
		case verbCubic:
			pts[0], pts[1], pts[2], pts[3] = p.pts[pi-1], p.pts[pi], p.pts[pi+1], p.pts[pi+2]
			pi += 3
			if !isButt && (!isClosed || (pts[0] == pts[1] && pts[1] == pts[2] && pts[2] == pts[3])) {
				extendPts(cap, prevVerb, hasPrev, nextVerb, hasNext, pts[:4])
			}
			d.hairCubic(pts[:4])
			lastPt = pts[3]
		case verbClose:
			pts[0] = lastPt
			pts[1] = firstPt
			if !isButt && hasPrev && prevVerb == verbMove {
				extendPts(cap, prevVerb, hasPrev, nextVerb, hasNext, pts[:2])
			}
			antiHairLineRgn(pts[:2], clip, b)
		}
		if !isButt {
			if hasPrev && prevVerb == verbMove && verb >= verbLine && verb <= verbCubic {
				firstPt = pts[0]
			}
			prevVerb = verb
			hasPrev = true
		}
	}
}
