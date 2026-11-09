// Ported from Chromium 156.0.8078.4 (Blink) and from Skia: the path geometry
// that the measurement APIs depend on.
//
//   - SVGPathBuilder: core/svg/svg_path_builder.cc of Blink
//     (LGPL-2.0-or-later).
//   - The parser of path data, SVGPathStringSource:
//     core/svg/svg_path_string_source.cc of Blink (LGPL-2.0-or-later).
//   - Path::TightBoundingRect, Path::length and Path::PointAtLength:
//     platform/geometry/path.cc of Blink (BSD-2-Clause, with the copyright
//     of Apple, Eric Seidel, Google and Intel in the file).
//   - gfx::SkRectToRectF: ui/gfx/geometry/skia_conversions.cc of Chromium
//     (BSD-3-Clause).
//   - SkPath (arcTo, conicTo, computeTightBounds, Iter), SkContourMeasure
//     and the coefficient helpers of SkGeometry: Skia (BSD-3-Clause,
//     ../font/LICENSE-Skia.txt). The file does not record the revision of
//     Skia.
//
// The Blink paths are in third_party/blink/renderer. The license of each
// Blink file is the license in its header at that version. See NOTICE in
// this directory.

package browser

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// This file ports the path geometry of Blink and Skia that the measurement
// APIs depend on: SVGPathBuilder (path data to an SkPath), SkPath::arcTo
// (elliptical arcs become conics), SkPath::computeTightBounds (getBBox of a
// path) and SkContourMeasure (getTotalLength, getPointAtLength). All of it
// is float32, as in Skia.

type pathVerb uint8

const (
	verbMove pathVerb = iota
	verbLine
	verbQuad
	verbConic
	verbCubic
	verbClose
)

type pt struct{ x, y float32 }

func (p pt) add(o pt) pt        { return pt{p.x + o.x, p.y + o.y} }
func (p pt) sub(o pt) pt        { return pt{p.x - o.x, p.y - o.y} }
func (p pt) scale(s float32) pt { return pt{float32(p.x * s), float32(p.y * s)} }

// skPath is the subset of SkPath that SVG path data needs.
type skPath struct {
	verbs   []pathVerb
	pts     []pt
	weights []float32 // one per conic
	// lastMoveIdx is the index in pts of the last moveTo, or -1.
	lastMoveIdx int
	needMove    bool
	// platform selects the C library whose rounding the path follows.
	platform font.Platform
}

func newPath() *skPath { return newPathFor(font.DefaultPlatform()) }

func newPathFor(platform font.Platform) *skPath {
	return &skPath{lastMoveIdx: -1, needMove: true, platform: platform}
}

func (p *skPath) isEmpty() bool { return len(p.verbs) == 0 }

func (p *skPath) lastPt() pt {
	if len(p.pts) == 0 {
		return pt{}
	}
	return p.pts[len(p.pts)-1]
}

func (p *skPath) moveTo(x, y float32) {
	p.lastMoveIdx = len(p.pts)
	p.verbs = append(p.verbs, verbMove)
	p.pts = append(p.pts, pt{x, y})
	p.needMove = false
}

// injectMoveToIfNeeded is SkPath::injectMoveToIfNeeded: a segment after a
// close starts a new contour at the start point of the closed contour.
func (p *skPath) injectMoveToIfNeeded() {
	if !p.needMove {
		return
	}
	if p.lastMoveIdx < 0 || len(p.pts) == 0 {
		p.moveTo(0, 0)
		return
	}
	start := p.pts[p.lastMoveIdx]
	p.moveTo(start.x, start.y)
}

func (p *skPath) lineTo(x, y float32) {
	p.injectMoveToIfNeeded()
	p.verbs = append(p.verbs, verbLine)
	p.pts = append(p.pts, pt{x, y})
}

func (p *skPath) quadTo(x1, y1, x2, y2 float32) {
	p.injectMoveToIfNeeded()
	p.verbs = append(p.verbs, verbQuad)
	p.pts = append(p.pts, pt{x1, y1}, pt{x2, y2})
}

func (p *skPath) conicTo(x1, y1, x2, y2, w float32) {
	// SkPath::conicTo degrades to simpler verbs for special weights.
	if !(w > 0) {
		p.lineTo(x2, y2)
		return
	}
	if math.IsInf(float64(w), 0) {
		p.lineTo(x1, y1)
		p.lineTo(x2, y2)
		return
	}
	if w == 1 {
		p.quadTo(x1, y1, x2, y2)
		return
	}
	p.injectMoveToIfNeeded()
	p.verbs = append(p.verbs, verbConic)
	p.pts = append(p.pts, pt{x1, y1}, pt{x2, y2})
	p.weights = append(p.weights, w)
}

func (p *skPath) cubicTo(x1, y1, x2, y2, x3, y3 float32) {
	p.injectMoveToIfNeeded()
	p.verbs = append(p.verbs, verbCubic)
	p.pts = append(p.pts, pt{x1, y1}, pt{x2, y2}, pt{x3, y3})
}

func (p *skPath) close() {
	if n := len(p.verbs); n > 0 {
		switch p.verbs[n-1] {
		case verbLine, verbQuad, verbConic, verbCubic, verbMove:
			p.verbs = append(p.verbs, verbClose)
		}
	}
	p.needMove = true
}

func (p *skPath) setLastPt(x, y float32) {
	if len(p.pts) == 0 {
		p.moveTo(x, y)
		return
	}
	p.pts[len(p.pts)-1] = pt{x, y}
}

// sinSnapToZero and cosSnapToZero are SkScalarSinSnapToZero and
// SkScalarCosSnapToZero, with sinf and cosf of the C library m.
func sinSnapToZero(rad float32, m cmath) float32 {
	v := m.sin(rad)
	if nearlyZero(v) {
		return 0
	}
	return v
}

func cosSnapToZero(rad float32, m cmath) float32 {
	v := m.cos(rad)
	if nearlyZero(v) {
		return 0
	}
	return v
}

const scalarNearlyZero = float32(1.0 / (1 << 12))

func nearlyZero(v float32) bool {
	return float32(math.Abs(float64(v))) <= scalarNearlyZero
}

// skMatrix is the 2x3 part of SkMatrix.
type skMatrix struct {
	sx, kx, tx float32
	ky, sy, ty float32
}

func (m skMatrix) mapPt(p pt) pt {
	return pt{float32(m.sx*p.x) + float32(m.kx*p.y) + m.tx, float32(m.ky*p.x) + float32(m.sy*p.y) + m.ty}
}

func matrixRotate(deg float32, m cmath) skMatrix {
	rad := float32(deg * float32(math.Pi/180))
	sinV, cosV := sinSnapToZero(rad, m), cosSnapToZero(rad, m)
	return skMatrix{sx: cosV, kx: -sinV, ky: sinV, sy: cosV}
}

func (m skMatrix) concat(o skMatrix) skMatrix {
	// m * o: o applies first.
	return skMatrix{
		sx: float32(m.sx*o.sx) + float32(m.kx*o.ky),
		kx: float32(m.sx*o.kx) + float32(m.kx*o.sy),
		tx: float32(m.sx*o.tx) + float32(m.kx*o.ty) + m.tx,
		ky: float32(m.ky*o.sx) + float32(m.sy*o.ky),
		sy: float32(m.ky*o.kx) + float32(m.sy*o.sy),
		ty: float32(m.ky*o.tx) + float32(m.sy*o.ty) + m.ty,
	}
}

// arcTo is SkPath::arcTo for an SVG elliptical arc.
func (p *skPath) arcTo(rx, ry, angle float32, large, sweep bool, x, y float32) {
	p.injectMoveToIfNeeded()
	src0 := p.lastPt()
	if rx == 0 || ry == 0 {
		p.lineTo(x, y)
		return
	}
	// The sines, cosines and tangents are those of the C library.
	m := cmathOf(p.platform)
	src1 := pt{x, y}
	if src0 == src1 {
		p.lineTo(x, y)
		return
	}
	rx = float32(math.Abs(float64(rx)))
	ry = float32(math.Abs(float64(ry)))
	midPointDistance := src0.sub(src1).scale(0.5)

	pointTransform := matrixRotate(-angle, m)
	transformedMidPoint := pointTransform.mapPt(midPointDistance)
	squareRx := float32(rx * rx)
	squareRy := float32(ry * ry)
	squareX := float32(transformedMidPoint.x * transformedMidPoint.x)
	squareY := float32(transformedMidPoint.y * transformedMidPoint.y)

	radiiScale := squareX/squareRx + squareY/squareRy
	if radiiScale > 1 {
		radiiScale = float32(math.Sqrt(float64(radiiScale)))
		rx *= radiiScale
		ry *= radiiScale
	}

	pointTransform = skMatrix{sx: 1 / rx, sy: 1 / ry}.concat(matrixRotate(-angle, m))
	unit0 := pointTransform.mapPt(src0)
	unit1 := pointTransform.mapPt(src1)
	delta := unit1.sub(unit0)

	d := float32(delta.x*delta.x) + float32(delta.y*delta.y)
	scaleFactorSquared := max(1/d-0.25, 0)
	scaleFactor := float32(math.Sqrt(float64(scaleFactorSquared)))
	// In Skia the test is (arcSweep == kCCW) != arcLarge. The SVG sweep
	// flag 1 is clockwise.
	if (!sweep) != large {
		scaleFactor = -scaleFactor
	}
	delta = delta.scale(scaleFactor)
	centerPoint := unit0.add(unit1).scale(0.5)
	centerPoint = pt{centerPoint.x - delta.y, centerPoint.y + delta.x}
	unit0 = unit0.sub(centerPoint)
	unit1 = unit1.sub(centerPoint)
	theta1 := skScalarATan2(unit0.y, unit0.x, p.platform)
	theta2 := skScalarATan2(unit1.y, unit1.x, p.platform)
	thetaArc := theta2 - theta1
	const pi = float32(math.Pi)
	if thetaArc < 0 && sweep {
		thetaArc += pi * 2
	} else if thetaArc > 0 && !sweep {
		thetaArc -= pi * 2
	}
	if float32(math.Abs(float64(thetaArc))) < pi/(1000*1000) {
		p.lineTo(x, y)
		return
	}

	pointTransform = matrixRotate(angle, m).concat(skMatrix{sx: rx, sy: ry})

	segments := int(math.Ceil(float64(float32(math.Abs(float64(thetaArc / (2 * pi / 3)))))))
	thetaWidth := thetaArc / float32(segments)
	t := m.tan(float32(0.5 * thetaWidth))
	if math.IsInf(float64(t), 0) || math.IsNaN(float64(t)) {
		return
	}
	startTheta := theta1
	w := float32(math.Sqrt(float64(0.5 + float32(m.cos(thetaWidth)*0.5))))
	isInteger := func(v float32) bool { return v == float32(math.Floor(float64(v))) }
	expectIntegers := nearlyZero(pi/2-float32(math.Abs(float64(thetaWidth)))) &&
		isInteger(rx) && isInteger(ry) && isInteger(x) && isInteger(y)

	for i := 0; i < segments; i++ {
		endTheta := startTheta + thetaWidth
		sinEnd := sinSnapToZero(endTheta, m)
		cosEnd := cosSnapToZero(endTheta, m)

		u1 := pt{cosEnd, sinEnd}.add(centerPoint)
		u0 := pt{u1.x + float32(t*sinEnd), u1.y - float32(t*cosEnd)}
		m0 := pointTransform.mapPt(u0)
		m1 := pointTransform.mapPt(u1)
		if expectIntegers {
			round := func(v float32) float32 { return float32(math.Floor(float64(v + 0.5))) }
			m0 = pt{round(m0.x), round(m0.y)}
			m1 = pt{round(m1.x), round(m1.y)}
		}
		p.conicTo(m0.x, m0.y, m1.x, m1.y, w)
		startTheta = endTheta
	}
	p.setLastPt(x, y)
}

// parsePathData builds a path from SVG path data, as Blink's
// SVGPathStringSource and SVGPathBuilder do. Parsing stops at the first
// error and keeps the segments before it.
func parsePathData(d string) *skPath {
	return parsePathDataFor(d, font.DefaultPlatform())
}

// parsePathDataFor is parsePathData with the platform whose C library the
// arcs follow.
func parsePathDataFor(d string, platform font.Platform) *skPath {
	p := newPathFor(platform)
	s := skipSVGSpace(d)
	var cur, subpath, lastControl pt
	var lastCmd byte
	var cmd byte
	first := true
	closedSubpath := false

	num := func() (float32, bool) {
		v, rest, ok := parseSVGNumber(s)
		if !ok {
			return 0, false
		}
		s = skipSVGSpaceComma(rest)
		return v, true
	}
	flag := func() (bool, bool) {
		if s == "" || (s[0] != '0' && s[0] != '1') {
			return false, false
		}
		v := s[0] == '1'
		s = skipSVGSpaceComma(s[1:])
		return v, true
	}
	isCmd := func(c byte) bool {
		switch c {
		case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
			return true
		}
		return false
	}

	for s != "" {
		if isCmd(s[0]) {
			cmd = s[0]
			s = skipSVGSpace(s[1:])
		} else {
			// An implicit command repeats the previous one. After a moveto
			// the implicit command is a lineto.
			switch {
			case first:
				return p
			case cmd == 'M':
				cmd = 'L'
			case cmd == 'm':
				cmd = 'l'
			case cmd == 'Z' || cmd == 'z':
				return p
			}
		}
		if first && cmd != 'M' && cmd != 'm' {
			return p
		}
		first = false
		rel := cmd >= 'a' && cmd <= 'z'
		base := pt{}
		if rel {
			base = cur
		}
		switch cmd {
		case 'Z', 'z':
			p.close()
			closedSubpath = true
			cur = subpath
		case 'M', 'm':
			x, ok1 := num()
			y, ok2 := num()
			if !ok1 || !ok2 {
				return p
			}
			cur = pt{base.x + x, base.y + y}
			p.moveTo(cur.x, cur.y)
			subpath = cur
		case 'L', 'l':
			x, ok1 := num()
			y, ok2 := num()
			if !ok1 || !ok2 {
				return p
			}
			cur = pt{base.x + x, base.y + y}
			p.lineTo(cur.x, cur.y)
		case 'H', 'h':
			x, ok := num()
			if !ok {
				return p
			}
			cur = pt{base.x + x, cur.y}
			p.lineTo(cur.x, cur.y)
		case 'V', 'v':
			y, ok := num()
			if !ok {
				return p
			}
			cur = pt{cur.x, base.y + y}
			p.lineTo(cur.x, cur.y)
		case 'C', 'c':
			var v [6]float32
			for i := range v {
				var ok bool
				if v[i], ok = num(); !ok {
					return p
				}
			}
			c1 := pt{base.x + v[0], base.y + v[1]}
			c2 := pt{base.x + v[2], base.y + v[3]}
			cur = pt{base.x + v[4], base.y + v[5]}
			p.cubicTo(c1.x, c1.y, c2.x, c2.y, cur.x, cur.y)
			lastControl = c2
		case 'S', 's':
			var v [4]float32
			for i := range v {
				var ok bool
				if v[i], ok = num(); !ok {
					return p
				}
			}
			c1 := cur
			switch lastCmd {
			case 'C', 'c', 'S', 's':
				c1 = pt{cur.x + (cur.x - lastControl.x), cur.y + (cur.y - lastControl.y)}
			}
			c2 := pt{base.x + v[0], base.y + v[1]}
			cur = pt{base.x + v[2], base.y + v[3]}
			p.cubicTo(c1.x, c1.y, c2.x, c2.y, cur.x, cur.y)
			lastControl = c2
		case 'Q', 'q':
			var v [4]float32
			for i := range v {
				var ok bool
				if v[i], ok = num(); !ok {
					return p
				}
			}
			c1 := pt{base.x + v[0], base.y + v[1]}
			cur = pt{base.x + v[2], base.y + v[3]}
			p.quadTo(c1.x, c1.y, cur.x, cur.y)
			lastControl = c1
		case 'T', 't':
			x, ok1 := num()
			y, ok2 := num()
			if !ok1 || !ok2 {
				return p
			}
			c1 := cur
			switch lastCmd {
			case 'Q', 'q', 'T', 't':
				c1 = pt{cur.x + (cur.x - lastControl.x), cur.y + (cur.y - lastControl.y)}
			}
			cur = pt{base.x + x, base.y + y}
			p.quadTo(c1.x, c1.y, cur.x, cur.y)
			lastControl = c1
		case 'A', 'a':
			rx, ok1 := num()
			ry, ok2 := num()
			angle, ok3 := num()
			large, ok4 := flag()
			sweep, ok5 := flag()
			x, ok6 := num()
			y, ok7 := num()
			if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7) {
				return p
			}
			cur = pt{base.x + x, base.y + y}
			p.arcTo(rx, ry, angle, large, sweep, cur.x, cur.y)
		}
		lastCmd = cmd
		_ = closedSubpath
	}
	return p
}

// ---- iteration ----

// segment is one drawable piece of a contour.
type segment struct {
	verb pathVerb // verbLine, verbQuad, verbConic, verbCubic
	p    [4]pt
	w    float32
}

// contours returns the segments of each contour. A close adds a line back
// to the start when the last point is not the start, as SkPath::Iter does.
func (p *skPath) contours() [][]segment {
	var out [][]segment
	var cur []segment
	var start, last pt
	pi, wi := 0, 0
	flush := func() {
		if cur != nil {
			out = append(out, cur)
			cur = nil
		}
	}
	for _, v := range p.verbs {
		switch v {
		case verbMove:
			flush()
			start = p.pts[pi]
			last = start
			pi++
			cur = []segment{}
		case verbLine:
			cur = append(cur, segment{verb: verbLine, p: [4]pt{last, p.pts[pi]}})
			last = p.pts[pi]
			pi++
		case verbQuad:
			cur = append(cur, segment{verb: verbQuad, p: [4]pt{last, p.pts[pi], p.pts[pi+1]}})
			last = p.pts[pi+1]
			pi += 2
		case verbConic:
			cur = append(cur, segment{verb: verbConic, p: [4]pt{last, p.pts[pi], p.pts[pi+1]}, w: p.weights[wi]})
			last = p.pts[pi+1]
			pi += 2
			wi++
		case verbCubic:
			cur = append(cur, segment{verb: verbCubic, p: [4]pt{last, p.pts[pi], p.pts[pi+1], p.pts[pi+2]}})
			last = p.pts[pi+2]
			pi += 3
		case verbClose:
			if last != start {
				cur = append(cur, segment{verb: verbLine, p: [4]pt{last, start}})
			}
			last = start
		}
	}
	flush()
	return out
}

// ---- tight bounds: SkPath::computeTightBounds ----

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

func findUnitQuadRoots(a, b, c float32) []float32 {
	if a == 0 {
		if r, ok := validUnitDivide(-c, b); ok {
			return []float32{r}
		}
		return nil
	}
	dr := float64(float64(b)*float64(b)) - float64(4*float64(a)*float64(c))
	if dr < 0 {
		return nil
	}
	dr = math.Sqrt(dr)
	r32 := float32(dr)
	if math.IsInf(float64(r32), 0) || math.IsNaN(float64(r32)) {
		return nil
	}
	var q float32
	if b < 0 {
		q = -(b - r32) / 2
	} else {
		q = -(b + r32) / 2
	}
	var roots []float32
	if r, ok := validUnitDivide(q, a); ok {
		roots = append(roots, r)
	}
	if r, ok := validUnitDivide(c, q); ok {
		roots = append(roots, r)
	}
	if len(roots) == 2 {
		if roots[0] > roots[1] {
			roots[0], roots[1] = roots[1], roots[0]
		} else if roots[0] == roots[1] {
			roots = roots[:1]
		}
	}
	return roots
}

func evalQuad(p [4]pt, t float32) pt {
	// SkQuadCoeff: fA = P2 - 2*P1 + P0, fB = 2*(P1 - P0), fC = P0.
	ax := p[2].x - float32(2*p[1].x) + p[0].x
	ay := p[2].y - float32(2*p[1].y) + p[0].y
	bx := float32(2 * (p[1].x - p[0].x))
	by := float32(2 * (p[1].y - p[0].y))
	return pt{float32(float32((float32(float32(ax)*t))+bx)*t) + p[0].x, float32(float32((float32(float32(ay)*t))+by)*t) + p[0].y}
}

// mulAdd is a*b + c with two roundings. The explicit conversion keeps the
// compiler from fusing the operations.
func mulAdd(a, b, c float32) float32 { return float32(a*b) + c }

func evalCubic(p [4]pt, t float32) pt {
	if t == 0 {
		return p[0]
	}
	if t == 1 {
		return p[3]
	}
	ax := p[3].x + float32(3*(p[1].x-p[2].x)) - p[0].x
	ay := p[3].y + float32(3*(p[1].y-p[2].y)) - p[0].y
	bx := float32(3 * (p[2].x - float32(2*p[1].x) + p[0].x))
	by := float32(3 * (p[2].y - float32(2*p[1].y) + p[0].y))
	cx := float32(3 * (p[1].x - p[0].x))
	cy := float32(3 * (p[1].y - p[0].y))
	return pt{mulAdd(mulAdd(mulAdd(ax, t, bx), t, cx), t, p[0].x), mulAdd(mulAdd(mulAdd(ay, t, by), t, cy), t, p[0].y)}
}

func evalConic(p [4]pt, w, t float32) pt {
	// SkConicCoeff.
	p1wx, p1wy := float32(p[1].x*w), float32(p[1].y*w)
	nax := p[2].x - float32(2*p1wx) + p[0].x
	nay := p[2].y - float32(2*p1wy) + p[0].y
	nbx := float32(2 * (p1wx - p[0].x))
	nby := float32(2 * (p1wy - p[0].y))
	db := float32(2 * (w - 1))
	da := 0 - db
	denom := mulAdd(mulAdd(da, t, db), t, 1)
	return pt{mulAdd(mulAdd(nax, t, nbx), t, p[0].x) / denom, mulAdd(mulAdd(nay, t, nby), t, p[0].y) / denom}
}

func conicExtremum(a, b, c, w float32) (float32, bool) {
	p20 := c - a
	p10 := b - a
	wP10 := float32(w * p10)
	roots := findUnitQuadRoots(float32(w*p20)-p20, p20-float32(2*wP10), wP10)
	if len(roots) == 1 {
		return roots[0], true
	}
	return 0, false
}

// tightBounds is SkPath::computeTightBounds.
func (p *skPath) tightBounds() rectF {
	if len(p.verbs) == 0 || len(p.pts) == 0 {
		return rectF{}
	}
	minX, minY := p.pts[0].x, p.pts[0].y
	maxX, maxY := minX, minY
	add := func(q pt) {
		minX, maxX = min(minX, q.x), max(maxX, q.x)
		minY, maxY = min(minY, q.y), max(maxY, q.y)
	}
	var last pt
	pi, wi := 0, 0
	for _, v := range p.verbs {
		switch v {
		case verbMove:
			last = p.pts[pi]
			add(last)
			pi++
		case verbLine:
			last = p.pts[pi]
			add(last)
			pi++
		case verbQuad:
			q := [4]pt{last, p.pts[pi], p.pts[pi+1]}
			if t, ok := validUnitDivide(q[0].x-q[1].x, q[0].x-q[1].x-q[1].x+q[2].x); ok {
				add(evalQuad(q, t))
			}
			if t, ok := validUnitDivide(q[0].y-q[1].y, q[0].y-q[1].y-q[1].y+q[2].y); ok {
				add(evalQuad(q, t))
			}
			add(q[2])
			last = q[2]
			pi += 2
		case verbConic:
			q := [4]pt{last, p.pts[pi], p.pts[pi+1]}
			w := p.weights[wi]
			if t, ok := conicExtremum(q[0].x, q[1].x, q[2].x, w); ok {
				add(evalConic(q, w, t))
			}
			if t, ok := conicExtremum(q[0].y, q[1].y, q[2].y, w); ok {
				add(evalConic(q, w, t))
			}
			add(q[2])
			last = q[2]
			pi += 2
			wi++
		case verbCubic:
			q := [4]pt{last, p.pts[pi], p.pts[pi+1], p.pts[pi+2]}
			for _, t := range findUnitQuadRoots(q[3].x-q[0].x+float32(3*(q[1].x-q[2].x)), float32(2*(q[0].x-q[1].x-q[1].x+q[2].x)), q[1].x-q[0].x) {
				add(evalCubic(q, t))
			}
			for _, t := range findUnitQuadRoots(q[3].y-q[0].y+float32(3*(q[1].y-q[2].y)), float32(2*(q[0].y-q[1].y-q[1].y+q[2].y)), q[1].y-q[0].y) {
				add(evalCubic(q, t))
			}
			add(q[3])
			last = q[3]
			pi += 3
		}
	}
	// Path::TightBoundingRect converts the SkRect with gfx::SkRectToRectF.
	return rectFromBounds(minX, minY, maxX, maxY)
}

// ---- length: SkContourMeasure ----

const (
	maxTValue        = 0x3FFFFFFF
	maxTValueInverse = float32(9.3132257e-10)
	cheapDistLimit   = float32(0.5)
)

// measureTolerance is CHEAP_DIST_LIMIT divided by the resScale of the measure.
var measureTolerance = cheapDistLimit

func tspanBigEnough(tspan int) bool { return tspan>>10 != 0 }

func ptDistance(a, b pt) float32 {
	dx, dy := b.x-a.x, b.y-a.y
	mag2 := float32(dx*dx) + float32(dy*dy)
	if !math.IsInf(float64(mag2), 0) && !math.IsNaN(float64(mag2)) {
		return float32(math.Sqrt(float64(mag2)))
	}
	return float32(math.Sqrt(float64(float64(dx)*float64(dx)) + float64(float64(dy)*float64(dy))))
}

func interp(a, b pt, t float32) pt {
	return pt{a.x + float32((b.x-a.x)*t), a.y + float32((b.y-a.y)*t)}
}

// measureSeg is one piece of a contour for length lookup.
type measureSeg struct {
	distance float32 // total distance up to the end of this piece
	seg      int     // index of the source segment
	t        int     // t value at the end of the piece (fixed point, 30 bits)
}

type contourMeasure struct {
	segs   []segment
	pieces []measureSeg
	length float32
}

func quadTooCurvy(p [4]pt) bool {
	dx := float32(p[1].x*0.5) - float32((p[0].x+p[2].x)*0.5*0.5)
	dy := float32(p[1].y*0.5) - float32((p[0].y+p[2].y)*0.5*0.5)
	dist := max(float32(math.Abs(float64(dx))), float32(math.Abs(float64(dy))))
	return dist > measureTolerance
}

func cheapDistExceedsLimit(p pt, x, y float32) bool {
	dist := max(float32(math.Abs(float64(x-p.x))), float32(math.Abs(float64(y-p.y))))
	return dist > measureTolerance
}

func scalarInterp(a, b, t float32) float32 { return a + float32((b-a)*t) }

func cubicTooCurvy(p [4]pt) bool {
	const third = float32(1.0 / 3)
	const twoThirds = float32(2.0 / 3)
	return cheapDistExceedsLimit(p[1], scalarInterp(p[0].x, p[3].x, third), scalarInterp(p[0].y, p[3].y, third)) ||
		cheapDistExceedsLimit(p[2], scalarInterp(p[0].x, p[3].x, twoThirds), scalarInterp(p[0].y, p[3].y, twoThirds))
}

func conicTooCurvy(first, mid, last pt) bool {
	midEnds := pt{float32((first.x + last.x) * 0.5), float32((first.y + last.y) * 0.5)}
	dx, dy := mid.x-midEnds.x, mid.y-midEnds.y
	dist := max(float32(math.Abs(float64(dx))), float32(math.Abs(float64(dy))))
	return dist > measureTolerance
}

func (cm *contourMeasure) push(distance, prev float32, seg, t int) float32 {
	if distance > prev {
		cm.pieces = append(cm.pieces, measureSeg{distance, seg, t})
	}
	return distance
}

func (cm *contourMeasure) quadSegs(p [4]pt, distance float32, mint, maxt, seg int) float32 {
	if tspanBigEnough(maxt-mint) && quadTooCurvy(p) {
		halft := (mint + maxt) >> 1
		p01 := interp(p[0], p[1], 0.5)
		p12 := interp(p[1], p[2], 0.5)
		mid := interp(p01, p12, 0.5)
		distance = cm.quadSegs([4]pt{p[0], p01, mid}, distance, mint, halft, seg)
		distance = cm.quadSegs([4]pt{mid, p12, p[2]}, distance, halft, maxt, seg)
		return distance
	}
	return cm.push(distance+ptDistance(p[0], p[2]), distance, seg, maxt)
}

func (cm *contourMeasure) cubicSegs(p [4]pt, distance float32, mint, maxt, seg int) float32 {
	if tspanBigEnough(maxt-mint) && cubicTooCurvy(p) {
		halft := (mint + maxt) >> 1
		ab := interp(p[0], p[1], 0.5)
		bc := interp(p[1], p[2], 0.5)
		cd := interp(p[2], p[3], 0.5)
		abc := interp(ab, bc, 0.5)
		bcd := interp(bc, cd, 0.5)
		abcd := interp(abc, bcd, 0.5)
		distance = cm.cubicSegs([4]pt{p[0], ab, abc, abcd}, distance, mint, halft, seg)
		distance = cm.cubicSegs([4]pt{abcd, bcd, cd, p[3]}, distance, halft, maxt, seg)
		return distance
	}
	return cm.push(distance+ptDistance(p[0], p[3]), distance, seg, maxt)
}

func (cm *contourMeasure) conicSegs(p [4]pt, w float32, distance float32, mint int, minPt pt, maxt int, maxPt pt, seg int) float32 {
	halft := (mint + maxt) >> 1
	halfPt := evalConic(p, w, float32(float32(halft)*maxTValueInverse))
	if math.IsInf(float64(halfPt.x), 0) || math.IsNaN(float64(halfPt.x)) || math.IsInf(float64(halfPt.y), 0) || math.IsNaN(float64(halfPt.y)) {
		return distance
	}
	if tspanBigEnough(maxt-mint) && conicTooCurvy(minPt, halfPt, maxPt) {
		distance = cm.conicSegs(p, w, distance, mint, minPt, halft, halfPt, seg)
		distance = cm.conicSegs(p, w, distance, halft, halfPt, maxt, maxPt, seg)
		return distance
	}
	return cm.push(distance+ptDistance(minPt, maxPt), distance, seg, maxt)
}

func measureContour(segs []segment) *contourMeasure {
	cm := &contourMeasure{segs: segs}
	var distance float32
	for i, s := range segs {
		switch s.verb {
		case verbLine:
			distance = cm.push(distance+ptDistance(s.p[0], s.p[1]), distance, i, maxTValue)
		case verbQuad:
			distance = cm.quadSegs(s.p, distance, 0, maxTValue, i)
		case verbConic:
			distance = cm.conicSegs(s.p, s.w, distance, 0, s.p[0], maxTValue, s.p[2], i)
		case verbCubic:
			distance = cm.cubicSegs(s.p, distance, 0, maxTValue, i)
		}
	}
	cm.length = distance
	return cm
}

// length is Blink's Path::length(): the sum of the contour lengths.
func (p *skPath) length() float32 {
	var total float32
	for _, c := range p.contours() {
		total += measureContour(c).length
	}
	return total
}

// pointAt is Blink's Path::PointAtLength: the point at a distance along the
// path, across contours.
func (p *skPath) pointAt(distance float32) pt {
	contours := p.contours()
	if distance < 0 {
		distance = 0
	}
	var last pt
	for _, c := range contours {
		cm := measureContour(c)
		if cm.length == 0 {
			if len(c) > 0 {
				last = c[len(c)-1].p[0]
			}
			continue
		}
		if distance <= cm.length {
			return cm.posAt(distance)
		}
		distance -= cm.length
		last = cm.posAt(cm.length)
	}
	if len(contours) == 0 && len(p.pts) > 0 {
		return p.pts[0]
	}
	return last
}

func (cm *contourMeasure) posAt(distance float32) pt {
	if distance < 0 {
		distance = 0
	} else if distance > cm.length {
		distance = cm.length
	}
	// Find the piece that contains the distance.
	idx := 0
	for idx < len(cm.pieces)-1 && cm.pieces[idx].distance < distance {
		idx++
	}
	piece := cm.pieces[idx]
	var startT int
	var startD float32
	if idx > 0 {
		startD = cm.pieces[idx-1].distance
		if cm.pieces[idx-1].seg == piece.seg {
			startT = cm.pieces[idx-1].t
		}
	}
	t0 := float32(float32(startT) * maxTValueInverse)
	t1 := float32(float32(piece.t) * maxTValueInverse)
	t := t0 + (t1-t0)*(distance-startD)/(piece.distance-startD)
	s := cm.segs[piece.seg]
	switch s.verb {
	case verbLine:
		return interp(s.p[0], s.p[1], t)
	case verbQuad:
		return evalQuad(s.p, t)
	case verbConic:
		return evalConic(s.p, s.w, t)
	default:
		return evalCubic(s.p, t)
	}
}
