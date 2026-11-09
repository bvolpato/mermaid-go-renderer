// Ported from d3-path 3.1.0, d3-path/src/path.js (ISC).

// Package path is a port of d3-path 3.1.0: it serializes the path methods of
// the canvas API to SVG path data.
package path

import (
	"fmt"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// These are variables, not constants, so that Go computes tauEpsilon with
// float64 arithmetic, as JavaScript does.
var (
	pi         float64 = math.Pi
	tau                = float64(2 * pi)
	epsilon    float64 = 1e-6
	tauEpsilon         = tau - epsilon
)

// Path is a d3 path: it builds SVG path data.
type Path struct {
	// The start (x0, y0) and the end (x1, y1) of the current subpath. d3
	// keeps null in these fields until a method sets them.
	x0, y0, x1, y1 float64
	null0, null1   bool
	buf            []byte
	// round is true when the numbers are rounded to a number of digits: each
	// number x is written as Math.round(x * k) / k.
	round bool
	k     float64
}

// New is d3.path(). It writes each number as JavaScript converts a number
// to a string.
func New() *Path {
	return &Path{null0: true, null1: true}
}

// NewRound is d3.pathRound(digits) and new Path(digits): it rounds each
// number to the given number of digits after the decimal point. More than 15
// digits mean no rounding. The error is the "invalid digits" error of d3.
func NewRound(digits float64) (*Path, error) {
	d := math.Floor(digits)
	if !(d >= 0) {
		return nil, fmt.Errorf("invalid digits: %s", js.Str(digits))
	}
	p := New()
	if d > 15 {
		return p, nil
	}
	p.round = true
	p.k = jsm.Pow10(int(d))
	return p, nil
}

// num appends a number, as the template tags append and appendRound do.
func (p *Path) num(x float64) {
	if p.round {
		x = js.Round(float64(x*p.k)) / p.k
	}
	p.buf = append(p.buf, js.Str(x)...)
}

func (p *Path) str(s string) {
	p.buf = append(p.buf, s...)
}

// MoveTo is path.moveTo(x, y).
func (p *Path) MoveTo(x, y float64) {
	p.x0, p.x1 = x, x
	p.y0, p.y1 = y, y
	p.null0, p.null1 = false, false
	p.str("M")
	p.num(x)
	p.str(",")
	p.num(y)
}

// ClosePath is path.closePath().
func (p *Path) ClosePath() {
	if !p.null1 {
		p.x1, p.y1 = p.x0, p.y0
		p.null1 = p.null0
		p.str("Z")
	}
}

// LineTo is path.lineTo(x, y).
func (p *Path) LineTo(x, y float64) {
	p.x1, p.y1 = x, y
	p.null1 = false
	p.str("L")
	p.num(x)
	p.str(",")
	p.num(y)
}

// QuadraticCurveTo is path.quadraticCurveTo(x1, y1, x, y).
func (p *Path) QuadraticCurveTo(x1, y1, x, y float64) {
	p.x1, p.y1 = x, y
	p.null1 = false
	p.str("Q")
	p.num(x1)
	p.str(",")
	p.num(y1)
	p.str(",")
	p.num(x)
	p.str(",")
	p.num(y)
}

// BezierCurveTo is path.bezierCurveTo(x1, y1, x2, y2, x, y).
func (p *Path) BezierCurveTo(x1, y1, x2, y2, x, y float64) {
	p.x1, p.y1 = x, y
	p.null1 = false
	p.str("C")
	p.num(x1)
	p.str(",")
	p.num(y1)
	p.str(",")
	p.num(x2)
	p.str(",")
	p.num(y2)
	p.str(",")
	p.num(x)
	p.str(",")
	p.num(y)
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// ArcTo is path.arcTo(x1, y1, x2, y2, r). The error is the "negative radius"
// error of d3.
func (p *Path) ArcTo(x1, y1, x2, y2, r float64) error {
	// Is the radius negative? Error.
	if r < 0 {
		return fmt.Errorf("negative radius: %s", js.Str(r))
	}

	// A null coordinate is zero in the arithmetic of JavaScript.
	x0, y0 := p.x1, p.y1
	if p.null1 {
		x0, y0 = 0, 0
	}
	x21 := x2 - x1
	y21 := y2 - y1
	x01 := x0 - x1
	y01 := y0 - y1
	l01_2 := float64(x01*x01) + float64(y01*y01)

	if p.null1 {
		// Is this path empty? Move to (x1,y1).
		p.x1, p.y1 = x1, y1
		p.null1 = false
		p.str("M")
		p.num(x1)
		p.str(",")
		p.num(y1)
	} else if !(l01_2 > epsilon) {
		// Or, is (x1,y1) coincident with (x0,y0)? Do nothing.
	} else if !(math.Abs(float64(y01*x21)-float64(y21*x01)) > epsilon) || r == 0 || math.IsNaN(r) {
		// Or, are (x0,y0), (x1,y1) and (x2,y2) collinear?
		// Equivalently, is (x1,y1) coincident with (x2,y2)?
		// Or, is the radius zero? Line to (x1,y1).
		p.x1, p.y1 = x1, y1
		p.str("L")
		p.num(x1)
		p.str(",")
		p.num(y1)
	} else {
		// Otherwise, draw an arc!
		x20 := x2 - x0
		y20 := y2 - y0
		l21_2 := float64(x21*x21) + float64(y21*y21)
		l20_2 := float64(x20*x20) + float64(y20*y20)
		l21 := math.Sqrt(l21_2)
		l01 := math.Sqrt(l01_2)
		l := float64(r * jsm.Tan((pi-jsm.Acos((l21_2+l01_2-l20_2)/(2*l21*l01)))/2))
		t01 := l / l01
		t21 := l / l21

		// If the start tangent is not coincident with (x0,y0), line to.
		if math.Abs(t01-1) > epsilon {
			p.str("L")
			p.num(x1 + float64(t01*x01))
			p.str(",")
			p.num(y1 + float64(t01*y01))
		}

		p.x1 = x1 + float64(t21*x21)
		p.y1 = y1 + float64(t21*y21)
		p.str("A")
		p.num(r)
		p.str(",")
		p.num(r)
		p.str(",0,0,")
		p.num(b2f(float64(y01*x20) > float64(x01*y20)))
		p.str(",")
		p.num(p.x1)
		p.str(",")
		p.num(p.y1)
	}
	return nil
}

// Arc is path.arc(x, y, r, a0, a1, ccw). The error is the "negative radius"
// error of d3.
func (p *Path) Arc(x, y, r, a0, a1 float64, ccw bool) error {
	// Is the radius negative? Error.
	if r < 0 {
		return fmt.Errorf("negative radius: %s", js.Str(r))
	}

	dx := float64(r * jsm.Cos(a0))
	dy := float64(r * jsm.Sin(a0))
	x0 := x + dx
	y0 := y + dy
	cw := b2f(!ccw)
	var da float64
	if ccw {
		da = a0 - a1
	} else {
		da = a1 - a0
	}

	if p.null1 {
		// Is this path empty? Move to (x0,y0).
		p.str("M")
		p.num(x0)
		p.str(",")
		p.num(y0)
	} else if math.Abs(p.x1-x0) > epsilon || math.Abs(p.y1-y0) > epsilon {
		// Or, is (x0,y0) not coincident with the previous point? Line to (x0,y0).
		p.str("L")
		p.num(x0)
		p.str(",")
		p.num(y0)
	}

	// Is this arc empty? We're done.
	if r == 0 || math.IsNaN(r) {
		return nil
	}

	// Does the angle go the wrong way? Flip the direction.
	if da < 0 {
		da = math.Mod(da, tau) + tau
	}

	if da > tauEpsilon {
		// Is this a complete circle? Draw two arcs to complete the circle.
		p.str("A")
		p.num(r)
		p.str(",")
		p.num(r)
		p.str(",0,1,")
		p.num(cw)
		p.str(",")
		p.num(x - dx)
		p.str(",")
		p.num(y - dy)
		p.str("A")
		p.num(r)
		p.str(",")
		p.num(r)
		p.str(",0,1,")
		p.num(cw)
		p.str(",")
		p.x1, p.y1 = x0, y0
		p.null1 = false
		p.num(x0)
		p.str(",")
		p.num(y0)
	} else if da > epsilon {
		// Is this arc non-empty? Draw an arc!
		p.x1 = x + float64(r*jsm.Cos(a1))
		p.y1 = y + float64(r*jsm.Sin(a1))
		p.null1 = false
		p.str("A")
		p.num(r)
		p.str(",")
		p.num(r)
		p.str(",0,")
		p.num(b2f(da >= pi))
		p.str(",")
		p.num(cw)
		p.str(",")
		p.num(p.x1)
		p.str(",")
		p.num(p.y1)
	}
	return nil
}

// Rect is path.rect(x, y, w, h).
func (p *Path) Rect(x, y, w, h float64) {
	p.x0, p.x1 = x, x
	p.y0, p.y1 = y, y
	p.null0, p.null1 = false, false
	p.str("M")
	p.num(x)
	p.str(",")
	p.num(y)
	p.str("h")
	p.num(w)
	p.str("v")
	p.num(h)
	p.str("h")
	p.num(-w)
	p.str("Z")
}

// String is path.toString().
func (p *Path) String() string {
	return string(p.buf)
}
