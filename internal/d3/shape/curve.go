// Ported from d3-shape 3.2.0, d3-shape/src/curve: linear.js, linearClosed.js,
// basis.js, basisClosed.js, basisOpen.js, bump.js, bundle.js, step.js and
// natural.js (ISC).

// Package shape is a port of the parts of d3-shape 3.2.0 that mermaid uses:
// the line, area, arc and pie generators and the curves.
package shape

import (
	"math"
)

// Context is the part of the canvas path API that a curve draws to.
// *path.Path implements it.
type Context interface {
	MoveTo(x, y float64)
	LineTo(x, y float64)
	BezierCurveTo(x1, y1, x2, y2, x, y float64)
	ClosePath()
}

// Curve is a d3 curve: it turns a sequence of points into path commands.
type Curve interface {
	AreaStart()
	AreaEnd()
	LineStart()
	LineEnd()
	Point(x, y float64)
}

// CurveFactory makes a curve that draws to a context, like d3.curveLinear.
type CurveFactory func(ctx Context) Curve

// truthy is the JavaScript conversion of a number to a boolean.
func truthy(x float64) bool {
	return x != 0 && !math.IsNaN(x)
}

// lineState is the _line field that most curves have. It is undefined until
// an area starts, 0 for the first line of an area, 1 for the second line and
// NaN after the area. Undefined behaves like NaN in every test of the curves.
type lineState struct {
	line float64
}

func newLineState() lineState { return lineState{line: math.NaN()} }

func (s *lineState) AreaStart() { s.line = 0 }
func (s *lineState) AreaEnd()   { s.line = math.NaN() }

// begin starts a line: it is a line-to inside an area and a move-to
// otherwise (this._line ? lineTo : moveTo).
func (s *lineState) begin(ctx Context, x, y float64) {
	if truthy(s.line) {
		ctx.LineTo(x, y)
	} else {
		ctx.MoveTo(x, y)
	}
}

// end ports the two statements that finish a line in most curves:
//
//	if (this._line || (this._line !== 0 && this._point === closePoint)) this._context.closePath();
//	this._line = 1 - this._line;
func (s *lineState) end(ctx Context, point, closePoint int) {
	if truthy(s.line) || (s.line != 0 && point == closePoint) {
		ctx.ClosePath()
	}
	s.line = 1 - s.line
}

// linear.js

type linear struct {
	lineState
	ctx   Context
	point int
}

// CurveLinear is d3.curveLinear.
func CurveLinear(ctx Context) Curve {
	return &linear{lineState: newLineState(), ctx: ctx}
}

func (c *linear) LineStart() { c.point = 0 }
func (c *linear) LineEnd()   { c.end(c.ctx, c.point, 1) }

func (c *linear) Point(x, y float64) {
	switch c.point {
	case 0:
		c.point = 1
		c.begin(c.ctx, x, y)
	case 1:
		c.point = 2
		fallthrough
	default:
		c.ctx.LineTo(x, y)
	}
}

// linearClosed.js

type linearClosed struct {
	ctx   Context
	point int
}

// CurveLinearClosed is d3.curveLinearClosed.
func CurveLinearClosed(ctx Context) Curve {
	return &linearClosed{ctx: ctx}
}

func (c *linearClosed) AreaStart() {}
func (c *linearClosed) AreaEnd()   {}
func (c *linearClosed) LineStart() { c.point = 0 }

func (c *linearClosed) LineEnd() {
	if c.point != 0 {
		c.ctx.ClosePath()
	}
}

func (c *linearClosed) Point(x, y float64) {
	if c.point != 0 {
		c.ctx.LineTo(x, y)
	} else {
		c.point = 1
		c.ctx.MoveTo(x, y)
	}
}

// basis.js

// basisPoint is the point function of basis.js.
func basisPoint(ctx Context, x0, y0, x1, y1, x, y float64) {
	ctx.BezierCurveTo(
		(float64(2*x0)+x1)/3,
		(float64(2*y0)+y1)/3,
		(x0+float64(2*x1))/3,
		(y0+float64(2*y1))/3,
		(x0+float64(4*x1)+x)/6,
		(y0+float64(4*y1)+y)/6,
	)
}

type basis struct {
	lineState
	ctx            Context
	x0, x1, y0, y1 float64
	point          int
}

func newBasis(ctx Context) *basis {
	return &basis{lineState: newLineState(), ctx: ctx}
}

// CurveBasis is d3.curveBasis.
func CurveBasis(ctx Context) Curve { return newBasis(ctx) }

func (c *basis) LineStart() {
	c.x0, c.x1 = math.NaN(), math.NaN()
	c.y0, c.y1 = math.NaN(), math.NaN()
	c.point = 0
}

func (c *basis) LineEnd() {
	switch c.point {
	case 3:
		basisPoint(c.ctx, c.x0, c.y0, c.x1, c.y1, c.x1, c.y1)
		fallthrough
	case 2:
		c.ctx.LineTo(c.x1, c.y1)
	}
	c.end(c.ctx, c.point, 1)
}

func (c *basis) Point(x, y float64) {
	switch c.point {
	case 0:
		c.point = 1
		c.begin(c.ctx, x, y)
	case 1:
		c.point = 2
	case 2:
		c.point = 3
		c.ctx.LineTo((float64(5*c.x0)+c.x1)/6, (float64(5*c.y0)+c.y1)/6)
		fallthrough
	default:
		basisPoint(c.ctx, c.x0, c.y0, c.x1, c.y1, x, y)
	}
	c.x0, c.x1 = c.x1, x
	c.y0, c.y1 = c.y1, y
}

// basisClosed.js

type basisClosed struct {
	ctx                Context
	x0, x1, x2, x3, x4 float64
	y0, y1, y2, y3, y4 float64
	point              int
}

// CurveBasisClosed is d3.curveBasisClosed.
func CurveBasisClosed(ctx Context) Curve {
	return &basisClosed{ctx: ctx}
}

func (c *basisClosed) AreaStart() {}
func (c *basisClosed) AreaEnd()   {}

func (c *basisClosed) LineStart() {
	nan := math.NaN()
	c.x0, c.x1, c.x2, c.x3, c.x4 = nan, nan, nan, nan, nan
	c.y0, c.y1, c.y2, c.y3, c.y4 = nan, nan, nan, nan, nan
	c.point = 0
}

func (c *basisClosed) LineEnd() {
	switch c.point {
	case 1:
		c.ctx.MoveTo(c.x2, c.y2)
		c.ctx.ClosePath()
	case 2:
		c.ctx.MoveTo((c.x2+float64(2*c.x3))/3, (c.y2+float64(2*c.y3))/3)
		c.ctx.LineTo((c.x3+float64(2*c.x2))/3, (c.y3+float64(2*c.y2))/3)
		c.ctx.ClosePath()
	case 3:
		c.Point(c.x2, c.y2)
		c.Point(c.x3, c.y3)
		c.Point(c.x4, c.y4)
	}
}

func (c *basisClosed) Point(x, y float64) {
	switch c.point {
	case 0:
		c.point = 1
		c.x2, c.y2 = x, y
	case 1:
		c.point = 2
		c.x3, c.y3 = x, y
	case 2:
		c.point = 3
		c.x4, c.y4 = x, y
		c.ctx.MoveTo((c.x0+float64(4*c.x1)+x)/6, (c.y0+float64(4*c.y1)+y)/6)
	default:
		basisPoint(c.ctx, c.x0, c.y0, c.x1, c.y1, x, y)
	}
	c.x0, c.x1 = c.x1, x
	c.y0, c.y1 = c.y1, y
}

// basisOpen.js

type basisOpen struct {
	lineState
	ctx            Context
	x0, x1, y0, y1 float64
	point          int
}

// CurveBasisOpen is d3.curveBasisOpen.
func CurveBasisOpen(ctx Context) Curve {
	return &basisOpen{lineState: newLineState(), ctx: ctx}
}

func (c *basisOpen) LineStart() {
	c.x0, c.x1 = math.NaN(), math.NaN()
	c.y0, c.y1 = math.NaN(), math.NaN()
	c.point = 0
}

func (c *basisOpen) LineEnd() { c.end(c.ctx, c.point, 3) }

func (c *basisOpen) Point(x, y float64) {
	switch c.point {
	case 0:
		c.point = 1
	case 1:
		c.point = 2
	case 2:
		c.point = 3
		x0 := (c.x0 + float64(4*c.x1) + x) / 6
		y0 := (c.y0 + float64(4*c.y1) + y) / 6
		c.begin(c.ctx, x0, y0)
	case 3:
		c.point = 4
		fallthrough
	default:
		basisPoint(c.ctx, c.x0, c.y0, c.x1, c.y1, x, y)
	}
	c.x0, c.x1 = c.x1, x
	c.y0, c.y1 = c.y1, y
}

// bump.js

type bump struct {
	lineState
	ctx    Context
	x      bool
	x0, y0 float64
	point  int
}

// CurveBumpX is d3.curveBumpX.
func CurveBumpX(ctx Context) Curve {
	return &bump{lineState: newLineState(), ctx: ctx, x: true, x0: math.NaN(), y0: math.NaN()}
}

// CurveBumpY is d3.curveBumpY.
func CurveBumpY(ctx Context) Curve {
	return &bump{lineState: newLineState(), ctx: ctx, x: false, x0: math.NaN(), y0: math.NaN()}
}

func (c *bump) LineStart() { c.point = 0 }
func (c *bump) LineEnd()   { c.end(c.ctx, c.point, 1) }

func (c *bump) Point(x, y float64) {
	switch c.point {
	case 0:
		c.point = 1
		c.begin(c.ctx, x, y)
	case 1:
		c.point = 2
		fallthrough
	default:
		if c.x {
			c.x0 = (c.x0 + x) / 2
			c.ctx.BezierCurveTo(c.x0, c.y0, c.x0, y, x, y)
		} else {
			c.y0 = (c.y0 + y) / 2
			c.ctx.BezierCurveTo(c.x0, c.y0, x, c.y0, x, y)
		}
	}
	c.x0, c.y0 = x, y
}

// bundle.js

type bundle struct {
	basis *basis
	beta  float64
	x, y  []float64
}

// CurveBundle is d3.curveBundle, with a beta of 0.85.
func CurveBundle(ctx Context) Curve { return CurveBundleBeta(0.85)(ctx) }

// CurveBundleBeta is d3.curveBundle.beta(beta).
//
// d3 defines the bundle curve only for lines: an area generator with this
// curve throws. In the port, AreaStart and AreaEnd do nothing.
func CurveBundleBeta(beta float64) CurveFactory {
	return func(ctx Context) Curve {
		if beta == 1 {
			return newBasis(ctx)
		}
		return &bundle{basis: newBasis(ctx), beta: beta}
	}
}

func (c *bundle) AreaStart() {}
func (c *bundle) AreaEnd()   {}

func (c *bundle) LineStart() {
	c.x = c.x[:0]
	c.y = c.y[:0]
	c.basis.LineStart()
}

func (c *bundle) LineEnd() {
	x, y := c.x, c.y
	j := len(x) - 1

	if j > 0 {
		x0 := x[0]
		y0 := y[0]
		dx := x[j] - x0
		dy := y[j] - y0
		for i := 0; i <= j; i++ {
			t := float64(i) / float64(j)
			c.basis.Point(
				float64(c.beta*x[i])+float64((1-c.beta)*(x0+float64(t*dx))),
				float64(c.beta*y[i])+float64((1-c.beta)*(y0+float64(t*dy))),
			)
		}
	}

	c.x, c.y = nil, nil
	c.basis.LineEnd()
}

func (c *bundle) Point(x, y float64) {
	c.x = append(c.x, x)
	c.y = append(c.y, y)
}

// step.js

type step struct {
	lineState
	ctx   Context
	t     float64
	x, y  float64
	point int
}

// CurveStep is d3.curveStep.
func CurveStep(ctx Context) Curve {
	return &step{lineState: newLineState(), ctx: ctx, t: 0.5}
}

// CurveStepBefore is d3.curveStepBefore.
func CurveStepBefore(ctx Context) Curve {
	return &step{lineState: newLineState(), ctx: ctx, t: 0}
}

// CurveStepAfter is d3.curveStepAfter.
func CurveStepAfter(ctx Context) Curve {
	return &step{lineState: newLineState(), ctx: ctx, t: 1}
}

func (c *step) LineStart() {
	c.x, c.y = math.NaN(), math.NaN()
	c.point = 0
}

func (c *step) LineEnd() {
	if 0 < c.t && c.t < 1 && c.point == 2 {
		c.ctx.LineTo(c.x, c.y)
	}
	if truthy(c.line) || (c.line != 0 && c.point == 1) {
		c.ctx.ClosePath()
	}
	if c.line >= 0 {
		c.t = 1 - c.t
		c.line = 1 - c.line
	}
}

func (c *step) Point(x, y float64) {
	switch c.point {
	case 0:
		c.point = 1
		c.begin(c.ctx, x, y)
	case 1:
		c.point = 2
		fallthrough
	default:
		if c.t <= 0 {
			c.ctx.LineTo(c.x, y)
			c.ctx.LineTo(x, y)
		} else {
			x1 := float64(c.x*(1-c.t)) + float64(x*c.t)
			c.ctx.LineTo(x1, c.y)
			c.ctx.LineTo(x1, y)
		}
	}
	c.x, c.y = x, y
}

// natural.js

type natural struct {
	lineState
	ctx  Context
	x, y []float64
}

// CurveNatural is d3.curveNatural.
func CurveNatural(ctx Context) Curve {
	return &natural{lineState: newLineState(), ctx: ctx}
}

func (c *natural) LineStart() {
	c.x = c.x[:0]
	c.y = c.y[:0]
}

func (c *natural) LineEnd() {
	x, y := c.x, c.y
	n := len(x)

	if n != 0 {
		c.begin(c.ctx, x[0], y[0])
		if n == 2 {
			c.ctx.LineTo(x[1], y[1])
		} else if n > 2 {
			// For one point, d3 computes control points and draws nothing.
			px0, px1 := controlPoints(x)
			py0, py1 := controlPoints(y)
			for i0, i1 := 0, 1; i1 < n; i0, i1 = i0+1, i1+1 {
				c.ctx.BezierCurveTo(px0[i0], py0[i0], px1[i0], py1[i0], x[i1], y[i1])
			}
		}
	}

	if truthy(c.line) || (c.line != 0 && n == 1) {
		c.ctx.ClosePath()
	}
	c.line = 1 - c.line
	c.x, c.y = nil, nil
}

func (c *natural) Point(x, y float64) {
	c.x = append(c.x, x)
	c.y = append(c.y, y)
}

// controlPoints ports controlPoints of natural.js for three points or more.
// See https://www.particleincell.com/2012/bezier-splines/ for derivation.
func controlPoints(x []float64) ([]float64, []float64) {
	n := len(x) - 1
	a := make([]float64, n)
	b := make([]float64, n)
	r := make([]float64, n)
	a[0], b[0], r[0] = 0, 2, x[0]+float64(2*x[1])
	for i := 1; i < n-1; i++ {
		a[i], b[i], r[i] = 1, 4, float64(4*x[i])+float64(2*x[i+1])
	}
	a[n-1], b[n-1], r[n-1] = 2, 7, float64(8*x[n-1])+x[n]
	for i := 1; i < n; i++ {
		m := a[i] / b[i-1]
		b[i] -= m
		r[i] -= float64(m * r[i-1])
	}
	a[n-1] = r[n-1] / b[n-1]
	for i := n - 2; i >= 0; i-- {
		a[i] = (r[i] - a[i+1]) / b[i]
	}
	b[n-1] = (x[n] + a[n-1]) / 2
	for i := 0; i < n-1; i++ {
		b[i] = float64(2*x[i+1]) - a[i+1]
	}
	return a, b
}
