// Ported from Chromium 156.0.8078.4, in part: rectangles, the affine
// transform and the number parser of SVG. The types and the functions of
// this file follow these parts of Chromium:
//
//   - GenericParseNumber: core/svg/svg_parser_utilities.cc of Blink
//     (LGPL-2.0-or-later).
//   - AffineTransform: platform/transforms/affine_transform.cc of Blink
//     (BSD-2-Clause, with the copyright of Apple and Google in the file).
//   - gfx::RectF, gfx::BoundingRect, gfx::QuadF, gfx::Transform::MapQuad and
//     gfx::SkRectToRectF: ui/gfx/geometry (BSD-3-Clause).
//
// The Blink paths are in third_party/blink/renderer. The license of each
// file is the license in its header at that version. See NOTICE in this
// directory. The package comment that follows ports no upstream file.

// Package browser emulates the parts of Chromium that mermaid depends on
// while it renders: the CSS cascade, text and HTML layout, SVG geometry, and
// the measurement APIs getBBox, getBoundingClientRect, getComputedTextLength
// and getTotalLength. It implements dom.Engine.
//
// The values that these APIs return end up in the output SVG. Chromium
// computes most of them in float32. This package keeps the same precision
// and the same order of operations, so that the numbers are the same.
package browser

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// rectF is gfx::RectF: a rectangle in float32.
type rectF struct {
	x, y, w, h float32
}

func (r rectF) right() float32  { return r.x + r.w }
func (r rectF) bottom() float32 { return r.y + r.h }
func (r rectF) isEmpty() bool   { return r.w <= 0 || r.h <= 0 }

func (r rectF) toDOM() dom.Rect {
	return dom.Rect{X: float64(r.x), Y: float64(r.y), Width: float64(r.w), Height: float64(r.h)}
}

// rectFromBounds is gfx::BoundingRect(p1, p2), which gfx::SkRectToRectF
// uses to convert the bounds of a Skia path: the rectangle with the edges
// left, top, right and bottom. The width and the height are float32
// differences. If the precision is lost during the calculation, the width
// or the height always grows to the next float32, also when the rounded
// difference is already too large. For example the edges 117.8 and 423.4
// give the height 305.6000061035156 and not 305.5999755859375.
func rectFromBounds(left, top, right, bottom float32) rectF {
	width := right - left
	height := bottom - top
	if left+width != right {
		width = math.Nextafter32(width, float32(math.Inf(1)))
		if math.IsInf(float64(width), 0) {
			width = math.MaxFloat32
		}
	}
	if top+height != bottom {
		height = math.Nextafter32(height, float32(math.Inf(1)))
		if math.IsInf(float64(height), 0) {
			height = math.MaxFloat32
		}
	}
	return rectF{left, top, width, height}
}

// unionEvenIfEmpty is gfx::RectF::UnionEvenIfEmpty. The width and the
// height are float32 differences, so the new rectangle can end before the
// right or the bottom edge of the two rectangles. gfx::RectF then grows the
// size to the next float32. TestContainerUnionAgainstChromium has the
// values of Chromium for this step.
func (r rectF) unionEvenIfEmpty(o rectF) rectF {
	rx := min(r.x, o.x)
	ry := min(r.y, o.y)
	rr := max(r.right(), o.right())
	rb := max(r.bottom(), o.bottom())
	u := rectF{rx, ry, rr - rx, rb - ry}
	if u.right() < rr && u.w < math.MaxFloat32 {
		u.w = math.Nextafter32(u.w, math.MaxFloat32)
	}
	if u.bottom() < rb && u.h < math.MaxFloat32 {
		u.h = math.Nextafter32(u.h, math.MaxFloat32)
	}
	return u
}

// union is gfx::RectF::Union: an empty rectangle does not contribute.
func (r rectF) union(o rectF) rectF {
	if r.isEmpty() {
		return o
	}
	if o.isEmpty() {
		return r
	}
	return r.unionEvenIfEmpty(o)
}

// affine is blink::AffineTransform: [a c e; b d f] in float64.
type affine struct {
	a, b, c, d, e, f float64
}

var identity = affine{a: 1, d: 1}

func (m affine) isIdentity() bool {
	return m.a == 1 && m.b == 0 && m.c == 0 && m.d == 1 && m.e == 0 && m.f == 0
}

func (m affine) isIdentityOrTranslation() bool {
	return m.a == 1 && m.b == 0 && m.c == 0 && m.d == 1
}

// multiply returns m * o: o is applied first (AffineTransform::PreConcat).
func (m affine) multiply(o affine) affine {
	return affine{
		a: float64(o.a*m.a) + float64(o.b*m.c),
		b: float64(o.a*m.b) + float64(o.b*m.d),
		c: float64(o.c*m.a) + float64(o.d*m.c),
		d: float64(o.c*m.b) + float64(o.d*m.d),
		e: float64(o.e*m.a) + float64(o.f*m.c) + m.e,
		f: float64(o.e*m.b) + float64(o.f*m.d) + m.f,
	}
}

func (m affine) translate(tx, ty float64) affine {
	if m.isIdentityOrTranslation() {
		m.e += tx
		m.f += ty
		return m
	}
	m.e += float64(tx*m.a) + float64(ty*m.c)
	m.f += float64(tx*m.b) + float64(ty*m.d)
	return m
}

func (m affine) scale(sx, sy float64) affine {
	m.a *= sx
	m.b *= sx
	m.c *= sy
	m.d *= sy
	return m
}

func (m affine) rotate(deg float64) affine {
	rad := float64(deg * (math.Pi / 180))
	cosA, sinA := math.Cos(rad), math.Sin(rad)
	return m.multiply(affine{a: cosA, b: sinA, c: -sinA, d: cosA})
}

func (m affine) skewX(deg float64) affine {
	return m.multiply(affine{a: 1, d: 1, c: math.Tan(deg * (math.Pi / 180))})
}

func (m affine) skewY(deg float64) affine {
	return m.multiply(affine{a: 1, d: 1, b: math.Tan(deg * (math.Pi / 180))})
}

func clampToFloat(v float64) float32 {
	if v > math.MaxFloat32 {
		return math.MaxFloat32
	}
	if v < -math.MaxFloat32 {
		return -math.MaxFloat32
	}
	return float32(v)
}

func (m affine) mapPoint(x, y float32) (float32, float32) {
	x2 := float64(m.a*float64(x)) + float64(m.c*float64(y)) + m.e
	y2 := float64(m.b*float64(x)) + float64(m.d*float64(y)) + m.f
	return clampToFloat(x2), clampToFloat(y2)
}

// mapQuadBounds is gfx::Transform::MapQuad for a rectangle, and
// gfx::QuadF::BoundingBox of the result: each corner goes through the
// matrix in double and becomes a float, and the width and the height are
// the float differences of the edges.
func (m affine) mapQuadBounds(r rectF) rectF {
	x1, y1 := m.mapPoint(r.x, r.y)
	x2, y2 := m.mapPoint(r.right(), r.y)
	x3, y3 := m.mapPoint(r.right(), r.bottom())
	x4, y4 := m.mapPoint(r.x, r.bottom())
	rl := min(x1, x2, x3, x4)
	rr := max(x1, x2, x3, x4)
	rt := min(y1, y2, y3, y4)
	rb := max(y1, y2, y3, y4)
	return rectF{rl, rt, rr - rl, rb - rt}
}

// mapRect is AffineTransform::MapRect.
func (m affine) mapRect(r rectF) rectF {
	if m.isIdentityOrTranslation() {
		if m.e == 0 && m.f == 0 {
			return r
		}
		// The translation is added in double, then the sum is rounded.
		r.x = clampToFloat(float64(r.x) + m.e)
		r.y = clampToFloat(float64(r.y) + m.f)
		return r
	}
	x1, y1 := m.mapPoint(r.x, r.y)
	x2, y2 := m.mapPoint(r.right(), r.y)
	x3, y3 := m.mapPoint(r.right(), r.bottom())
	x4, y4 := m.mapPoint(r.x, r.bottom())
	rl := min(x1, x2, x3, x4)
	rr := max(x1, x2, x3, x4)
	rt := min(y1, y2, y3, y4)
	rb := max(y1, y2, y3, y4)
	return rectF{rl, rt, rr - rl, rb - rt}
}

// parseSVGNumber is Blink's GenericParseNumber<float>: it parses a number at
// the start of s with float32 arithmetic and returns the rest of the string.
// The float32 arithmetic is intended: the result can differ from the nearest
// float32 of the decimal text, and Chromium uses this result.
func parseSVGNumber(s string) (value float32, rest string, ok bool) {
	i := 0
	n := len(s)
	var integer, decimal, exponent float32
	frac := float32(1)
	sign := float32(1)
	expSign := 1
	if i < n && s[i] == '+' {
		i++
	} else if i < n && s[i] == '-' {
		i++
		sign = -1
	}
	if i == n || ((s[i] < '0' || s[i] > '9') && s[i] != '.') {
		return 0, s, false
	}
	startInt := i
	for i < n && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i != startInt {
		multiplier := float32(1)
		for j := i - 1; j >= startInt; j-- {
			integer += float32(multiplier * float32(s[j]-'0'))
			multiplier *= 10
		}
		if math.IsInf(float64(integer), 0) {
			return 0, s, false
		}
	}
	if i < n && s[i] == '.' {
		i++
		if i >= n || s[i] < '0' || s[i] > '9' {
			return 0, s, false
		}
		for i < n && s[i] >= '0' && s[i] <= '9' {
			frac *= float32(0.1)
			decimal += float32(float32(s[i]-'0') * frac)
			i++
		}
	}
	if i+1 < n && (s[i] == 'e' || s[i] == 'E') && s[i+1] != 'x' && s[i+1] != 'm' {
		j := i + 1
		if s[j] == '+' {
			j++
		} else if s[j] == '-' {
			j++
			expSign = -1
		}
		if j < n && s[j] >= '0' && s[j] <= '9' {
			for j < n && s[j] >= '0' && s[j] <= '9' {
				exponent = float32(exponent * 10)
				exponent += float32(s[j] - '0')
				j++
			}
			i = j
		}
	}
	number := integer + decimal
	number *= sign
	if exponent != 0 {
		number *= float32(math.Pow(10, float64(expSign*int(exponent))))
	}
	if math.IsInf(float64(number), 0) || math.IsNaN(float64(number)) {
		return 0, s, false
	}
	return number, s[i:], true
}

func skipSVGSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r' || s[0] == '\f') {
		s = s[1:]
	}
	return s
}

func skipSVGSpaceComma(s string) string {
	s = skipSVGSpace(s)
	if len(s) > 0 && s[0] == ',' {
		s = skipSVGSpace(s[1:])
	}
	return s
}

// parseNumberList parses numbers separated by white space or commas.
func parseNumberList(s string) []float32 {
	var out []float32
	s = skipSVGSpace(s)
	for s != "" {
		v, rest, ok := parseSVGNumber(s)
		if !ok {
			break
		}
		out = append(out, v)
		s = skipSVGSpaceComma(rest)
	}
	return out
}

// parseTransform parses an SVG transform attribute and returns the matrix
// that layout uses. See parseTransformList in transform.go.
func parseTransform(s string) affine {
	return parseTransformList(s)
}
