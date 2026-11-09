// Ported from d3-shape 3.2.0, d3-shape/src/curve: cardinal.js,
// cardinalClosed.js, cardinalOpen.js, catmullRom.js, catmullRomClosed.js and
// catmullRomOpen.js (ISC).

package shape

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
)

// cardinalState holds the fields that the three cardinal curves share.
type cardinalState struct {
	ctx                    Context
	k                      float64
	x0, x1, x2, y0, y1, y2 float64
	point                  int
}

func newCardinalState(ctx Context, tension float64) cardinalState {
	return cardinalState{ctx: ctx, k: (1 - tension) / 6}
}

// curveTo is the point function of cardinal.js.
func (c *cardinalState) curveTo(x, y float64) {
	c.ctx.BezierCurveTo(
		c.x1+float64(c.k*(c.x2-c.x0)),
		c.y1+float64(c.k*(c.y2-c.y0)),
		c.x2+float64(c.k*(c.x1-x)),
		c.y2+float64(c.k*(c.y1-y)),
		c.x2,
		c.y2,
	)
}

func (c *cardinalState) reset() {
	nan := math.NaN()
	c.x0, c.x1, c.x2 = nan, nan, nan
	c.y0, c.y1, c.y2 = nan, nan, nan
	c.point = 0
}

func (c *cardinalState) shift(x, y float64) {
	c.x0, c.x1, c.x2 = c.x1, c.x2, x
	c.y0, c.y1, c.y2 = c.y1, c.y2, y
}

type cardinal struct {
	lineState
	cardinalState
}

func newCardinal(ctx Context, tension float64) *cardinal {
	return &cardinal{lineState: newLineState(), cardinalState: newCardinalState(ctx, tension)}
}

// CurveCardinal is d3.curveCardinal, with a tension of 0.
func CurveCardinal(ctx Context) Curve { return newCardinal(ctx, 0) }

// CurveCardinalTension is d3.curveCardinal.tension(tension).
func CurveCardinalTension(tension float64) CurveFactory {
	return func(ctx Context) Curve { return newCardinal(ctx, tension) }
}

func (c *cardinal) LineStart() { c.reset() }

func (c *cardinal) LineEnd() {
	switch c.point {
	case 2:
		c.ctx.LineTo(c.x2, c.y2)
	case 3:
		c.curveTo(c.x1, c.y1)
	}
	c.end(c.ctx, c.point, 1)
}

func (c *cardinal) Point(x, y float64) {
	switch c.point {
	case 0:
		c.point = 1
		c.begin(c.ctx, x, y)
	case 1:
		c.point = 2
		c.x1, c.y1 = x, y
	case 2:
		c.point = 3
		fallthrough
	default:
		c.curveTo(x, y)
	}
	c.shift(x, y)
}

type cardinalClosed struct {
	cardinalState
	x3, x4, x5, y3, y4, y5 float64
}

func newCardinalClosed(ctx Context, tension float64) *cardinalClosed {
	return &cardinalClosed{cardinalState: newCardinalState(ctx, tension)}
}

// CurveCardinalClosed is d3.curveCardinalClosed, with a tension of 0.
func CurveCardinalClosed(ctx Context) Curve { return newCardinalClosed(ctx, 0) }

// CurveCardinalClosedTension is d3.curveCardinalClosed.tension(tension).
func CurveCardinalClosedTension(tension float64) CurveFactory {
	return func(ctx Context) Curve { return newCardinalClosed(ctx, tension) }
}

func (c *cardinalClosed) AreaStart() {}
func (c *cardinalClosed) AreaEnd()   {}

func (c *cardinalClosed) LineStart() {
	c.reset()
	nan := math.NaN()
	c.x3, c.x4, c.x5 = nan, nan, nan
	c.y3, c.y4, c.y5 = nan, nan, nan
}

func (c *cardinalClosed) LineEnd() {
	switch c.point {
	case 1:
		c.ctx.MoveTo(c.x3, c.y3)
		c.ctx.ClosePath()
	case 2:
		c.ctx.LineTo(c.x3, c.y3)
		c.ctx.ClosePath()
	case 3:
		c.Point(c.x3, c.y3)
		c.Point(c.x4, c.y4)
		c.Point(c.x5, c.y5)
	}
}

func (c *cardinalClosed) Point(x, y float64) {
	switch c.point {
	case 0:
		c.point = 1
		c.x3, c.y3 = x, y
	case 1:
		c.point = 2
		c.x4, c.y4 = x, y
		c.ctx.MoveTo(x, y)
	case 2:
		c.point = 3
		c.x5, c.y5 = x, y
	default:
		c.curveTo(x, y)
	}
	c.shift(x, y)
}

type cardinalOpen struct {
	lineState
	cardinalState
}

func newCardinalOpen(ctx Context, tension float64) *cardinalOpen {
	return &cardinalOpen{lineState: newLineState(), cardinalState: newCardinalState(ctx, tension)}
}

// CurveCardinalOpen is d3.curveCardinalOpen, with a tension of 0.
func CurveCardinalOpen(ctx Context) Curve { return newCardinalOpen(ctx, 0) }

// CurveCardinalOpenTension is d3.curveCardinalOpen.tension(tension).
func CurveCardinalOpenTension(tension float64) CurveFactory {
	return func(ctx Context) Curve { return newCardinalOpen(ctx, tension) }
}

func (c *cardinalOpen) LineStart() { c.reset() }
func (c *cardinalOpen) LineEnd()   { c.end(c.ctx, c.point, 3) }

func (c *cardinalOpen) Point(x, y float64) {
	switch c.point {
	case 0:
		c.point = 1
	case 1:
		c.point = 2
	case 2:
		c.point = 3
		c.begin(c.ctx, c.x2, c.y2)
	case 3:
		c.point = 4
		fallthrough
	default:
		c.curveTo(x, y)
	}
	c.shift(x, y)
}

// catmullState holds the fields that the three Catmull-Rom curves share.
type catmullState struct {
	ctx                    Context
	alpha                  float64
	x0, x1, x2, y0, y1, y2 float64
	l01a, l12a, l23a       float64
	l012a, l122a, l232a    float64
	point                  int
}

// curveTo is the point function of catmullRom.js.
func (c *catmullState) curveTo(x, y float64) {
	x1, y1, x2, y2 := c.x1, c.y1, c.x2, c.y2

	if c.l01a > epsilon {
		a := float64(2*c.l012a) + float64(3*c.l01a*c.l12a) + c.l122a
		n := float64(3 * c.l01a * (c.l01a + c.l12a))
		x1 = (float64(x1*a) - float64(c.x0*c.l122a) + float64(c.x2*c.l012a)) / n
		y1 = (float64(y1*a) - float64(c.y0*c.l122a) + float64(c.y2*c.l012a)) / n
	}

	if c.l23a > epsilon {
		b := float64(2*c.l232a) + float64(3*c.l23a*c.l12a) + c.l122a
		m := float64(3 * c.l23a * (c.l23a + c.l12a))
		x2 = (float64(x2*b) + float64(c.x1*c.l232a) - float64(x*c.l122a)) / m
		y2 = (float64(y2*b) + float64(c.y1*c.l232a) - float64(y*c.l122a)) / m
	}

	c.ctx.BezierCurveTo(x1, y1, x2, y2, c.x2, c.y2)
}

func (c *catmullState) reset() {
	nan := math.NaN()
	c.x0, c.x1, c.x2 = nan, nan, nan
	c.y0, c.y1, c.y2 = nan, nan, nan
	c.l01a, c.l12a, c.l23a = 0, 0, 0
	c.l012a, c.l122a, c.l232a = 0, 0, 0
	c.point = 0
}

// distance updates the length of the last segment before a point.
func (c *catmullState) distance(x, y float64) {
	if c.point != 0 {
		x23 := c.x2 - x
		y23 := c.y2 - y
		c.l232a = jsm.Pow(float64(x23*x23)+float64(y23*y23), c.alpha)
		c.l23a = math.Sqrt(c.l232a)
	}
}

func (c *catmullState) shift(x, y float64) {
	c.l01a, c.l12a = c.l12a, c.l23a
	c.l012a, c.l122a = c.l122a, c.l232a
	c.x0, c.x1, c.x2 = c.x1, c.x2, x
	c.y0, c.y1, c.y2 = c.y1, c.y2, y
}

type catmullRom struct {
	lineState
	catmullState
}

// CurveCatmullRom is d3.curveCatmullRom, with an alpha of 0.5.
func CurveCatmullRom(ctx Context) Curve { return CurveCatmullRomAlpha(0.5)(ctx) }

// CurveCatmullRomAlpha is d3.curveCatmullRom.alpha(alpha).
func CurveCatmullRomAlpha(alpha float64) CurveFactory {
	return func(ctx Context) Curve {
		if !truthy(alpha) {
			return newCardinal(ctx, 0)
		}
		return &catmullRom{lineState: newLineState(), catmullState: catmullState{ctx: ctx, alpha: alpha}}
	}
}

func (c *catmullRom) LineStart() { c.reset() }

func (c *catmullRom) LineEnd() {
	switch c.point {
	case 2:
		c.ctx.LineTo(c.x2, c.y2)
	case 3:
		c.Point(c.x2, c.y2)
	}
	c.end(c.ctx, c.point, 1)
}

func (c *catmullRom) Point(x, y float64) {
	c.distance(x, y)
	switch c.point {
	case 0:
		c.point = 1
		c.begin(c.ctx, x, y)
	case 1:
		c.point = 2
	case 2:
		c.point = 3
		fallthrough
	default:
		c.curveTo(x, y)
	}
	c.shift(x, y)
}

type catmullRomClosed struct {
	catmullState
	x3, x4, x5, y3, y4, y5 float64
}

// CurveCatmullRomClosed is d3.curveCatmullRomClosed, with an alpha of 0.5.
func CurveCatmullRomClosed(ctx Context) Curve { return CurveCatmullRomClosedAlpha(0.5)(ctx) }

// CurveCatmullRomClosedAlpha is d3.curveCatmullRomClosed.alpha(alpha).
func CurveCatmullRomClosedAlpha(alpha float64) CurveFactory {
	return func(ctx Context) Curve {
		if !truthy(alpha) {
			return newCardinalClosed(ctx, 0)
		}
		return &catmullRomClosed{catmullState: catmullState{ctx: ctx, alpha: alpha}}
	}
}

func (c *catmullRomClosed) AreaStart() {}
func (c *catmullRomClosed) AreaEnd()   {}

func (c *catmullRomClosed) LineStart() {
	c.reset()
	nan := math.NaN()
	c.x3, c.x4, c.x5 = nan, nan, nan
	c.y3, c.y4, c.y5 = nan, nan, nan
}

func (c *catmullRomClosed) LineEnd() {
	switch c.point {
	case 1:
		c.ctx.MoveTo(c.x3, c.y3)
		c.ctx.ClosePath()
	case 2:
		c.ctx.LineTo(c.x3, c.y3)
		c.ctx.ClosePath()
	case 3:
		c.Point(c.x3, c.y3)
		c.Point(c.x4, c.y4)
		c.Point(c.x5, c.y5)
	}
}

func (c *catmullRomClosed) Point(x, y float64) {
	c.distance(x, y)
	switch c.point {
	case 0:
		c.point = 1
		c.x3, c.y3 = x, y
	case 1:
		c.point = 2
		c.x4, c.y4 = x, y
		c.ctx.MoveTo(x, y)
	case 2:
		c.point = 3
		c.x5, c.y5 = x, y
	default:
		c.curveTo(x, y)
	}
	c.shift(x, y)
}

type catmullRomOpen struct {
	lineState
	catmullState
}

// CurveCatmullRomOpen is d3.curveCatmullRomOpen, with an alpha of 0.5.
func CurveCatmullRomOpen(ctx Context) Curve { return CurveCatmullRomOpenAlpha(0.5)(ctx) }

// CurveCatmullRomOpenAlpha is d3.curveCatmullRomOpen.alpha(alpha).
func CurveCatmullRomOpenAlpha(alpha float64) CurveFactory {
	return func(ctx Context) Curve {
		if !truthy(alpha) {
			return newCardinalOpen(ctx, 0)
		}
		return &catmullRomOpen{lineState: newLineState(), catmullState: catmullState{ctx: ctx, alpha: alpha}}
	}
}

func (c *catmullRomOpen) LineStart() { c.reset() }
func (c *catmullRomOpen) LineEnd()   { c.end(c.ctx, c.point, 3) }

func (c *catmullRomOpen) Point(x, y float64) {
	c.distance(x, y)
	switch c.point {
	case 0:
		c.point = 1
	case 1:
		c.point = 2
	case 2:
		c.point = 3
		c.begin(c.ctx, c.x2, c.y2)
	case 3:
		c.point = 4
		fallthrough
	default:
		c.curveTo(x, y)
	}
	c.shift(x, y)
}
