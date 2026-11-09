// Ported from Chromium 156.0.8078.4 (Blink): the path traversal that Blink
// uses for getTotalLength and getPointAtLength of a path element.
//
//   - SVGPathQuery: core/svg/svg_path_query.cc (LGPL-2.0-or-later).
//   - SVGPathNormalizer: core/svg/svg_path_parser.cc and svg_path_parser.h
//     (LGPL-2.0-or-later).
//   - SVGPathStringSource: core/svg/svg_path_string_source.cc
//     (LGPL-2.0-or-later).
//   - PathSegmentData: core/svg/svg_path_data.h (LGPL-2.0-or-later).
//   - PathTraversalState: platform/graphics/path_traversal_state.cc
//     (LGPL-2.0-or-later).
//   - gfx::Vector2dF::Length: ui/gfx/geometry/vector2d_f.cc of Chromium
//     (BSD-3-Clause).
//
// The Blink paths are in third_party/blink/renderer. The license of each
// file is the license in its header at that version. See NOTICE in this
// directory.

package browser

import (
	"math"
)

// This file ports the path traversal that Blink uses for getTotalLength and
// getPointAtLength of a <path>: SVGPathQuery, SVGPathNormalizer and
// PathTraversalState. It is separate from the Skia geometry in path.go:
// Blink measures a path element from its normalized segments (arcs and
// quadratic curves become cubic curves), not from the SkPath.

// rawSeg is one segment of path data as written: PathSegmentData in Blink.
type rawSeg struct {
	cmd          byte // the command letter
	p1, p2, tgt  pt
	rx, ry       float32
	angle        float32
	large, sweep bool
}

// parseRawSegments parses path data into segments, stopping at the first
// error. It is SVGPathStringSource::ParseSegment in a loop.
func parseRawSegments(d string) []rawSeg {
	var out []rawSeg
	s := skipSVGSpace(d)
	var cmd byte
	first := true
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
	point := func() (pt, bool) {
		x, ok1 := num()
		if !ok1 {
			return pt{}, false
		}
		y, ok2 := num()
		return pt{x, y}, ok2
	}
	for s != "" {
		c := s[0]
		isCmd := false
		switch c {
		case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
			isCmd = true
		}
		if isCmd {
			cmd = c
			s = skipSVGSpace(s[1:])
		} else {
			switch {
			case first:
				return out
			case cmd == 'M':
				cmd = 'L'
			case cmd == 'm':
				cmd = 'l'
			case cmd == 'Z' || cmd == 'z':
				return out
			}
		}
		if first && cmd != 'M' && cmd != 'm' {
			return out
		}
		first = false
		seg := rawSeg{cmd: cmd}
		ok := true
		switch cmd {
		case 'Z', 'z':
		case 'M', 'm', 'L', 'l', 'T', 't':
			seg.tgt, ok = point()
		case 'H', 'h':
			seg.tgt.x, ok = num()
		case 'V', 'v':
			seg.tgt.y, ok = num()
		case 'C', 'c':
			if seg.p1, ok = point(); ok {
				if seg.p2, ok = point(); ok {
					seg.tgt, ok = point()
				}
			}
		case 'S', 's':
			if seg.p2, ok = point(); ok {
				seg.tgt, ok = point()
			}
		case 'Q', 'q':
			if seg.p1, ok = point(); ok {
				seg.tgt, ok = point()
			}
		case 'A', 'a':
			var ok1, ok2, ok3, ok4, ok5, ok6 bool
			seg.rx, ok1 = num()
			seg.ry, ok2 = num()
			seg.angle, ok3 = num()
			seg.large, ok4 = flag()
			seg.sweep, ok5 = flag()
			seg.tgt, ok6 = point()
			ok = ok1 && ok2 && ok3 && ok4 && ok5 && ok6
		}
		if !ok {
			return out
		}
		out = append(out, seg)
	}
	return out
}

// normSeg is a normalized segment: M, L, C or Z with absolute points.
type normSeg struct {
	cmd         byte
	p1, p2, tgt pt
}

func isCubicCmd(c byte) bool     { return c == 'C' || c == 'c' || c == 'S' || c == 's' }
func isQuadraticCmd(c byte) bool { return c == 'Q' || c == 'q' || c == 'T' || c == 't' }

func reflectedPoint(in, p pt) pt { return pt{float32(2*in.x) - p.x, float32(2*in.y) - p.y} }

// blendPoints blends two points with the ratio 1/3 : 2/3.
func blendPoints(p1, p2 pt) pt {
	const oneOverThree = float32(1) / 3
	return pt{float32((p1.x + float32(2*p2.x)) * oneOverThree), float32((p1.y + float32(2*p2.y)) * oneOverThree)}
}

// normalizeSegments is SVGPathNormalizer.
func normalizeSegments(raw []rawSeg, m cmath) []normSeg {
	var out []normSeg
	var cur, sub, control pt
	var last byte
	for _, seg := range raw {
		n := seg
		rel := seg.cmd >= 'a' && seg.cmd <= 'z'
		switch seg.cmd {
		case 'q':
			n.p1 = n.p1.add(cur)
			n.tgt = n.tgt.add(cur)
		case 'c':
			n.p1 = n.p1.add(cur)
			n.p2 = n.p2.add(cur)
			n.tgt = n.tgt.add(cur)
		case 's':
			n.p2 = n.p2.add(cur)
			n.tgt = n.tgt.add(cur)
		case 'm', 'l', 't', 'a':
			n.tgt = n.tgt.add(cur)
		case 'h':
			n.tgt = pt{cur.x + n.tgt.x, cur.y}
		case 'v':
			n.tgt = pt{cur.x, cur.y + n.tgt.y}
		case 'H':
			n.tgt.y = cur.y
		case 'V':
			n.tgt.x = cur.x
		case 'Z', 'z':
			n.tgt = sub
		}
		_ = rel
		emit := true
		var ns normSeg
		switch seg.cmd {
		case 'M', 'm':
			sub = n.tgt
			ns = normSeg{cmd: 'M', tgt: n.tgt}
		case 'L', 'l', 'H', 'h', 'V', 'v':
			ns = normSeg{cmd: 'L', tgt: n.tgt}
		case 'Z', 'z':
			ns = normSeg{cmd: 'Z', tgt: n.tgt}
		case 'S', 's', 'C', 'c':
			if seg.cmd == 'S' || seg.cmd == 's' {
				if !isCubicCmd(last) {
					n.p1 = cur
				} else {
					n.p1 = reflectedPoint(cur, control)
				}
			}
			control = n.p2
			ns = normSeg{cmd: 'C', p1: n.p1, p2: n.p2, tgt: n.tgt}
		case 'T', 't', 'Q', 'q':
			if seg.cmd == 'T' || seg.cmd == 't' {
				if !isQuadraticCmd(last) {
					n.p1 = cur
				} else {
					n.p1 = reflectedPoint(cur, control)
				}
			}
			control = n.p1
			ns = normSeg{cmd: 'C', p1: blendPoints(cur, control), p2: blendPoints(n.tgt, control), tgt: n.tgt}
		case 'A', 'a':
			cubics, ok := decomposeArcToCubic(cur, n, m)
			if !ok {
				ns = normSeg{cmd: 'L', tgt: n.tgt}
			} else {
				out = append(out, cubics...)
				emit = false
			}
		}
		if emit {
			out = append(out, ns)
		}
		cur = n.tgt
		if !isCubicCmd(seg.cmd) && !isQuadraticCmd(seg.cmd) {
			control = cur
		}
		last = seg.cmd
	}
	return out
}

func sqrtf(x float32) float32 { return float32(math.Sqrt(float64(x))) }
func fabsf(x float32) float32 { return float32(math.Abs(float64(x))) }

// decomposeArcToCubic is SVGPathNormalizer::DecomposeArcToCubic.
func decomposeArcToCubic(current pt, arc rawSeg, m cmath) ([]normSeg, bool) {
	rx := fabsf(arc.rx)
	ry := fabsf(arc.ry)
	if rx == 0 || ry == 0 {
		return nil, false
	}
	if arc.tgt == current {
		return nil, false
	}
	angle := float64(arc.angle)
	mid := current.sub(arc.tgt).scale(0.5)

	pointTransform := identity.rotate(-angle)
	tmx, tmy := pointTransform.mapPoint(mid.x, mid.y)
	squareRx := float32(rx * rx)
	squareRy := float32(ry * ry)
	squareX := float32(tmx * tmx)
	squareY := float32(tmy * tmy)

	radiiScale := squareX/squareRx + squareY/squareRy
	if radiiScale > 1 {
		rx *= sqrtf(radiiScale)
		ry *= sqrtf(radiiScale)
	}

	pointTransform = identity.scale(float64(1/rx), float64(1/ry)).rotate(-angle)
	p1x, p1y := pointTransform.mapPoint(current.x, current.y)
	p2x, p2y := pointTransform.mapPoint(arc.tgt.x, arc.tgt.y)
	point1, point2 := pt{p1x, p1y}, pt{p2x, p2y}
	delta := point2.sub(point1)

	// delta.LengthSquared() is a double, and the scale factor is computed
	// in double precision from it. The center of an arc whose chord is a
	// little shorter than its diameter (a line hop of mermaid, with its
	// numbers rounded to three decimals) depends on that: the probe cases
	// of pathlength_probe_test.go have other lengths with float32 here.
	d := float64(float64(delta.x)*float64(delta.x)) + float64(float64(delta.y)*float64(delta.y))
	scaleFactorSquared := math.Max(1/d-0.25, 0)
	scaleFactor := float32(math.Sqrt(scaleFactorSquared))
	if arc.sweep == arc.large {
		scaleFactor = -scaleFactor
	}
	delta = delta.scale(scaleFactor)
	center := point1.add(point2).scale(0.5)
	center = pt{center.x - delta.y, center.y + delta.x}

	theta1 := m.atan2(point1.y-center.y, point1.x-center.x)
	theta2 := m.atan2(point2.y-center.y, point2.x-center.x)
	thetaArc := theta2 - theta1
	const twoPi = float32(2 * math.Pi)
	const piOverTwo = float32(math.Pi / 2)
	if thetaArc < 0 && arc.sweep {
		thetaArc += twoPi
	} else if thetaArc > 0 && !arc.sweep {
		thetaArc -= twoPi
	}

	pointTransform = identity.rotate(angle).scale(float64(rx), float64(ry))

	segments := int(math.Ceil(float64(fabsf(thetaArc / (piOverTwo + 0.001)))))
	var out []normSeg
	for i := 0; i < segments; i++ {
		startTheta := theta1 + float32(i)*thetaArc/float32(segments)
		endTheta := theta1 + float32(i+1)*thetaArc/float32(segments)

		t := float32((8 / float32(6)) * m.tan(float32(0.25*(endTheta-startTheta))))
		if math.IsInf(float64(t), 0) || math.IsNaN(float64(t)) {
			return nil, false
		}
		sinStart, cosStart := m.sin(startTheta), m.cos(startTheta)
		sinEnd, cosEnd := m.sin(endTheta), m.cos(endTheta)

		c1 := pt{cosStart - float32(t*sinStart), sinStart + float32(t*cosStart)}
		c1 = pt{c1.x + center.x, c1.y + center.y}
		target := pt{cosEnd + center.x, sinEnd + center.y}
		c2 := pt{target.x + float32(t*sinEnd), target.y - float32(t*cosEnd)}

		m1x, m1y := pointTransform.mapPoint(c1.x, c1.y)
		m2x, m2y := pointTransform.mapPoint(c2.x, c2.y)
		mtx, mty := pointTransform.mapPoint(target.x, target.y)
		out = append(out, normSeg{cmd: 'C', p1: pt{m1x, m1y}, p2: pt{m2x, m2y}, tgt: pt{mtx, mty}})
	}
	return out, true
}

// ---- PathTraversalState ----

func midPoint(a, b pt) pt { return pt{float32((a.x + b.x) / 2), float32((a.y + b.y) / 2)} }

// distanceLine is DistanceLine: (end - start).Length(). The difference is
// a float32 vector. gfx::Vector2dF::Length computes the squares and their
// sum in double precision and rounds the root to float32. The probe cases
// of pathlength_probe_test.go show that: float32 squares give another last
// bit of getTotalLength for one path in seven.
func distanceLine(a, b pt) float32 {
	dx, dy := float64(b.x-a.x), float64(b.y-a.y)
	return float32(math.Sqrt(float64(dx*dx) + float64(dy*dy)))
}

type cubicBezier struct {
	start, c1, c2, end pt
	depth              uint16
}

func (c cubicBezier) magnitudeSquared() float64 {
	sq := func(v float32) float64 { return float64(v * v) }
	return (sq(c.start.x) + sq(c.start.y) + sq(c.c1.x) + sq(c.c1.y) + sq(c.c2.x) + sq(c.c2.y) + sq(c.end.x) + sq(c.end.y)) / 16
}

func (c cubicBezier) approximateDistance() float32 {
	return distanceLine(c.start, c.c1) + distanceLine(c.c1, c.c2) + distanceLine(c.c2, c.end)
}

func (c cubicBezier) split() (left, right cubicBezier) {
	startToControl1 := midPoint(c.c1, c.c2)
	left.start = c.start
	left.c1 = midPoint(c.start, c.c1)
	left.c2 = midPoint(left.c1, startToControl1)

	right.c2 = midPoint(c.c2, c.end)
	right.c1 = midPoint(right.c2, startToControl1)
	right.end = c.end

	mid := midPoint(left.c2, right.c1)
	left.end = mid
	right.start = mid
	left.depth = c.depth + 1
	right.depth = c.depth + 1
	return left, right
}

type traversal struct {
	// libm has atan2f, sinf and cosf of the C library.
	libm      cmath
	wantPoint bool
	desired   float32
	total     float32
	start     pt
	current   pt
	previous  pt
	success   bool
}

const (
	curveSplitDepthLimit              = 20
	pathSegmentLengthToleranceSquared = 1.e-16
)

// curveLength is CurveLength<CubicBezier> of PathTraversalState.
func (t *traversal) curveLength(curve cubicBezier) float32 {
	scale := curve.magnitudeSquared()
	if scale < pathSegmentLengthToleranceSquared {
		return 0
	}
	stack := []cubicBezier{curve}
	var total float32
	for len(stack) > 0 {
		length := curve.approximateDistance()
		discrepancy := float64(length - distanceLine(curve.start, curve.end))
		if (discrepancy*discrepancy)/scale > pathSegmentLengthToleranceSquared && curve.depth < curveSplitDepthLimit {
			left, right := curve.split()
			curve = left
			stack = append(stack, right)
			continue
		}
		total += length
		if t.wantPoint {
			t.previous = curve.start
			t.current = curve.end
			if t.total+total > t.desired {
				return total
			}
		}
		curve = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
	}
	return total
}

func (t *traversal) processSegment() {
	if t.wantPoint && t.total >= t.desired {
		slope := t.libm.atan2(t.current.y-t.previous.y, t.current.x-t.previous.x)
		offset := t.desired - t.total
		t.current = pt{t.current.x + float32(offset*t.libm.cos(slope)), t.current.y + float32(offset*t.libm.sin(slope))}
		t.success = true
	}
	t.previous = t.current
}

func (t *traversal) run(segs []normSeg) {
	for _, s := range segs {
		if t.success {
			return
		}
		switch s.cmd {
		case 'M':
			t.current, t.start = s.tgt, s.tgt
		case 'L':
			t.total += distanceLine(t.current, s.tgt)
			t.current = s.tgt
		case 'Z':
			t.total += distanceLine(t.current, t.start)
			t.current = t.start
		case 'C':
			t.total += t.curveLength(cubicBezier{start: t.current, c1: s.p1, c2: s.p2, end: s.tgt})
			if !t.wantPoint {
				t.current = s.tgt
			}
		}
		t.processSegment()
	}
}

// pathDataLength is SVGPathQuery::GetTotalLength.
func pathDataLength(d string, m cmath) float32 {
	t := &traversal{libm: m}
	t.run(normalizeSegments(parseRawSegments(d), m))
	return t.total
}

// pathDataPointAt is SVGPathQuery::GetPointAtLength.
func pathDataPointAt(d string, length float32, m cmath) pt {
	t := &traversal{libm: m, wantPoint: true, desired: length}
	t.run(normalizeSegments(parseRawSegments(d), m))
	return t.current
}
