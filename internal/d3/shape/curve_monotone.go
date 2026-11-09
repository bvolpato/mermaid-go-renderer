// Ported from d3-shape 3.2.0, d3-shape/src/curve/monotone.js (ISC).

package shape

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func sign(x float64) float64 {
	if x < 0 {
		return -1
	}
	return 1
}

type monotone struct {
	lineState
	ctx            Context
	x0, x1, y0, y1 float64
	t0             float64
	point          int
	// swap is true for the Y variant, which swaps the coordinates of each point.
	swap bool
}

// slope3 calculates the slopes of the tangents (Hermite-type interpolation)
// based on the following paper: Steffen, M. 1990. A Simple Method for
// Monotonic Interpolation in One Dimension. Astronomy and Astrophysics, Vol.
// 239, NO. NOV(II), P. 443, 1990.
func (c *monotone) slope3(x2, y2 float64) float64 {
	h0 := c.x1 - c.x0
	h1 := x2 - c.x1
	// h0 || h1 < 0 && -0
	d0 := h0
	if !truthy(h0) {
		d0 = 0
		if h1 < 0 {
			d0 = math.Copysign(0, -1)
		}
	}
	// h1 || h0 < 0 && -0
	d1 := h1
	if !truthy(h1) {
		d1 = 0
		if h0 < 0 {
			d1 = math.Copysign(0, -1)
		}
	}
	s0 := (c.y1 - c.y0) / d0
	s1 := (y2 - c.y1) / d1
	p := (float64(s0*h1) + float64(s1*h0)) / (h0 + h1)
	r := float64((sign(s0) + sign(s1)) * js.Min(math.Abs(s0), math.Abs(s1), float64(0.5*math.Abs(p))))
	if !truthy(r) {
		return 0
	}
	return r
}

// slope2 calculates a one-sided slope.
func (c *monotone) slope2(t float64) float64 {
	h := c.x1 - c.x0
	if truthy(h) {
		return (3*(c.y1-c.y0)/h - t) / 2
	}
	return t
}

// curveTo is the point function of monotone.js. According to
// https://en.wikipedia.org/wiki/Cubic_Hermite_spline#Representations
// "you can express cubic Hermite interpolation in terms of cubic Bézier
// curves with respect to the four values p0, p0 + m0 / 3, p1 - m1 / 3, p1".
func (c *monotone) curveTo(t0, t1 float64) {
	x0, y0, x1, y1 := c.x0, c.y0, c.x1, c.y1
	dx := (x1 - x0) / 3
	c.ctx.BezierCurveTo(x0+dx, y0+float64(dx*t0), x1-dx, y1-float64(dx*t1), x1, y1)
}

// CurveMonotoneX is d3.curveMonotoneX.
func CurveMonotoneX(ctx Context) Curve {
	return &monotone{lineState: newLineState(), ctx: ctx}
}

// CurveMonotoneY is d3.curveMonotoneY.
func CurveMonotoneY(ctx Context) Curve {
	return &monotone{lineState: newLineState(), ctx: reflectContext{ctx}, swap: true}
}

func (c *monotone) LineStart() {
	nan := math.NaN()
	c.x0, c.x1 = nan, nan
	c.y0, c.y1 = nan, nan
	c.t0 = nan
	c.point = 0
}

func (c *monotone) LineEnd() {
	switch c.point {
	case 2:
		c.ctx.LineTo(c.x1, c.y1)
	case 3:
		c.curveTo(c.t0, c.slope2(c.t0))
	}
	c.end(c.ctx, c.point, 1)
}

func (c *monotone) Point(x, y float64) {
	if c.swap {
		x, y = y, x
	}
	t1 := math.NaN()

	if x == c.x1 && y == c.y1 {
		return // Ignore coincident points.
	}
	switch c.point {
	case 0:
		c.point = 1
		c.begin(c.ctx, x, y)
	case 1:
		c.point = 2
	case 2:
		c.point = 3
		t1 = c.slope3(x, y)
		c.curveTo(c.slope2(t1), t1)
	default:
		t1 = c.slope3(x, y)
		c.curveTo(c.t0, t1)
	}

	c.x0, c.x1 = c.x1, x
	c.y0, c.y1 = c.y1, y
	c.t0 = t1
}

// reflectContext swaps the coordinates of every command.
type reflectContext struct {
	ctx Context
}

func (r reflectContext) MoveTo(x, y float64) { r.ctx.MoveTo(y, x) }
func (r reflectContext) ClosePath()          { r.ctx.ClosePath() }
func (r reflectContext) LineTo(x, y float64) { r.ctx.LineTo(y, x) }

func (r reflectContext) BezierCurveTo(x1, y1, x2, y2, x, y float64) {
	r.ctx.BezierCurveTo(y1, x1, y2, x2, y, x)
}
