// Ported from d3-shape 3.2.0, d3-shape/src/arc.js (ISC).

package shape

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// AngleDatum is a datum that has the properties startAngle, endAngle and
// padAngle, like the arcs that Pie returns. The default angle accessors of
// Arc read them. A property that does not exist is NaN.
type AngleDatum interface {
	ArcAngles() (startAngle, endAngle, padAngle float64)
}

// RadiusDatum is a datum that has the properties innerRadius and
// outerRadius. The default radius accessors of Arc read them.
type RadiusDatum interface {
	ArcRadii() (innerRadius, outerRadius float64)
}

// ArcDatum is a datum with all the properties that the default accessors of
// d3.arc read. Use NaN for a property that the JavaScript object does not
// have.
type ArcDatum struct {
	InnerRadius, OuterRadius, StartAngle, EndAngle, PadAngle float64
}

// ArcAngles implements AngleDatum.
func (d ArcDatum) ArcAngles() (startAngle, endAngle, padAngle float64) {
	return d.StartAngle, d.EndAngle, d.PadAngle
}

// ArcRadii implements RadiusDatum.
func (d ArcDatum) ArcRadii() (innerRadius, outerRadius float64) {
	return d.InnerRadius, d.OuterRadius
}

func arcInnerRadius[T any](d T) float64 {
	if r, ok := any(d).(RadiusDatum); ok {
		inner, _ := r.ArcRadii()
		return inner
	}
	return math.NaN()
}

func arcOuterRadius[T any](d T) float64 {
	if r, ok := any(d).(RadiusDatum); ok {
		_, outer := r.ArcRadii()
		return outer
	}
	return math.NaN()
}

func arcStartAngle[T any](d T) float64 {
	if a, ok := any(d).(AngleDatum); ok {
		start, _, _ := a.ArcAngles()
		return start
	}
	return math.NaN()
}

func arcEndAngle[T any](d T) float64 {
	if a, ok := any(d).(AngleDatum); ok {
		_, end, _ := a.ArcAngles()
		return end
	}
	return math.NaN()
}

func arcPadAngle[T any](d T) float64 {
	if a, ok := any(d).(AngleDatum); ok {
		_, _, pad := a.ArcAngles()
		return pad
	}
	return math.NaN()
}

func intersect(x0, y0, x1, y1, x2, y2, x3, y3 float64) (float64, float64, bool) {
	x10, y10 := x1-x0, y1-y0
	x32, y32 := x3-x2, y3-y2
	t := float64(y32*x10) - float64(x32*y10)
	if t*t < epsilon {
		return 0, 0, false
	}
	t = (float64(x32*(y0-y2)) - float64(y32*(x0-x2))) / t
	return x0 + float64(t*x10), y0 + float64(t*y10), true
}

type tangents struct {
	cx, cy, x01, y01, x11, y11 float64
}

// cornerTangents computes the perpendicular offset line of length rc.
// http://mathworld.wolfram.com/Circle-LineIntersection.html
func cornerTangents(x0, y0, x1, y1, r1, rc float64, cw bool) tangents {
	x01 := x0 - x1
	y01 := y0 - y1
	lo := rc
	if !cw {
		lo = -rc
	}
	lo = lo / math.Sqrt(float64(x01*x01)+float64(y01*y01))
	ox := float64(lo * y01)
	oy := float64(-lo * x01)
	x11 := x0 + ox
	y11 := y0 + oy
	x10 := x1 + ox
	y10 := y1 + oy
	// The conversions keep the compiler from fusing the halving, which it
	// turns into a multiplication by 0.5, with the subtractions below.
	x00 := float64((x11 + x10) / 2)
	y00 := float64((y11 + y10) / 2)
	dx := x10 - x11
	dy := y10 - y11
	d2 := float64(dx*dx) + float64(dy*dy)
	r := r1 - rc
	D := float64(x11*y10) - float64(x10*y11)
	s := 1.0
	if dy < 0 {
		s = -1
	}
	d := float64(s * math.Sqrt(js.Max(0, float64(r*r*d2)-float64(D*D))))
	cx0 := (float64(D*dy) - float64(dx*d)) / d2
	cy0 := (float64(-D*dx) - float64(dy*d)) / d2
	cx1 := (float64(D*dy) + float64(dx*d)) / d2
	cy1 := (float64(-D*dx) + float64(dy*d)) / d2
	dx0 := cx0 - x00
	dy0 := cy0 - y00
	dx1 := cx1 - x00
	dy1 := cy1 - y00

	// Pick the closer of the two intersection points.
	// TODO Is there a faster way to determine which intersection to use?
	if float64(dx0*dx0)+float64(dy0*dy0) > float64(dx1*dx1)+float64(dy1*dy1) {
		cx0, cy0 = cx1, cy1
	}

	return tangents{
		cx:  cx0,
		cy:  cy0,
		x01: -ox,
		y01: -oy,
		x11: float64(cx0 * (r1/r - 1)),
		y11: float64(cy0 * (r1/r - 1)),
	}
}

// Arc is the arc generator of d3 (d3.arc). T is the type of a datum.
type Arc[T any] struct {
	innerRadius  func(d T) float64
	outerRadius  func(d T) float64
	cornerRadius func(d T) float64
	padRadius    func(d T) float64 // nil is the null of d3
	startAngle   func(d T) float64
	endAngle     func(d T) float64
	padAngle     func(d T) float64
	digits       digits
}

func constantOf[T any](x float64) func(T) float64 {
	return func(T) float64 { return x }
}

// NewArc is d3.arc(). The default accessors read the properties of a datum
// that implements RadiusDatum and AngleDatum, and give NaN for another datum,
// as d3 does for an object without the properties.
func NewArc[T any]() *Arc[T] {
	return &Arc[T]{
		innerRadius:  arcInnerRadius[T],
		outerRadius:  arcOuterRadius[T],
		cornerRadius: constantOf[T](0),
		startAngle:   arcStartAngle[T],
		endAngle:     arcEndAngle[T],
		padAngle:     arcPadAngle[T],
		digits:       defaultDigits(),
	}
}

// InnerRadius is arc.innerRadius(number).
func (a *Arc[T]) InnerRadius(r float64) *Arc[T] { a.innerRadius = constantOf[T](r); return a }

// InnerRadiusFunc is arc.innerRadius(accessor).
func (a *Arc[T]) InnerRadiusFunc(f func(d T) float64) *Arc[T] { a.innerRadius = f; return a }

// OuterRadius is arc.outerRadius(number).
func (a *Arc[T]) OuterRadius(r float64) *Arc[T] { a.outerRadius = constantOf[T](r); return a }

// OuterRadiusFunc is arc.outerRadius(accessor).
func (a *Arc[T]) OuterRadiusFunc(f func(d T) float64) *Arc[T] { a.outerRadius = f; return a }

// CornerRadius is arc.cornerRadius(number).
func (a *Arc[T]) CornerRadius(r float64) *Arc[T] { a.cornerRadius = constantOf[T](r); return a }

// CornerRadiusFunc is arc.cornerRadius(accessor).
func (a *Arc[T]) CornerRadiusFunc(f func(d T) float64) *Arc[T] { a.cornerRadius = f; return a }

// PadRadius is arc.padRadius(number).
func (a *Arc[T]) PadRadius(r float64) *Arc[T] { a.padRadius = constantOf[T](r); return a }

// PadRadiusFunc is arc.padRadius(accessor). A nil accessor is
// arc.padRadius(null).
func (a *Arc[T]) PadRadiusFunc(f func(d T) float64) *Arc[T] { a.padRadius = f; return a }

// StartAngle is arc.startAngle(number).
func (a *Arc[T]) StartAngle(angle float64) *Arc[T] { a.startAngle = constantOf[T](angle); return a }

// StartAngleFunc is arc.startAngle(accessor).
func (a *Arc[T]) StartAngleFunc(f func(d T) float64) *Arc[T] { a.startAngle = f; return a }

// EndAngle is arc.endAngle(number).
func (a *Arc[T]) EndAngle(angle float64) *Arc[T] { a.endAngle = constantOf[T](angle); return a }

// EndAngleFunc is arc.endAngle(accessor).
func (a *Arc[T]) EndAngleFunc(f func(d T) float64) *Arc[T] { a.endAngle = f; return a }

// PadAngle is arc.padAngle(number).
func (a *Arc[T]) PadAngle(angle float64) *Arc[T] { a.padAngle = constantOf[T](angle); return a }

// PadAngleFunc is arc.padAngle(accessor).
func (a *Arc[T]) PadAngleFunc(f func(d T) float64) *Arc[T] { a.padAngle = f; return a }

// Digits is arc.digits(digits). The default is 3. A negative value is
// arc.digits(null): the numbers are not rounded.
func (a *Arc[T]) Digits(digits int) *Arc[T] {
	a.digits.n = digits
	return a
}

// Path is arc(datum) with a null context: it returns the path data.
func (a *Arc[T]) Path(datum T) string {
	r0 := a.innerRadius(datum)
	r1 := a.outerRadius(datum)
	a0 := a.startAngle(datum) - halfPi
	a1 := a.endAngle(datum) - halfPi
	da := math.Abs(a1 - a0)
	cw := a1 > a0

	context := a.digits.newPath()

	// Ensure that the outer radius is always larger than the inner radius.
	if r1 < r0 {
		r0, r1 = r1, r0
	}

	if !(r1 > epsilon) {
		// Is it a point?
		context.MoveTo(0, 0)
	} else if da > tau-epsilon {
		// Or is it a circle or annulus?
		context.MoveTo(float64(r1*jsm.Cos(a0)), float64(r1*jsm.Sin(a0)))
		_ = context.Arc(0, 0, r1, a0, a1, !cw)
		if r0 > epsilon {
			context.MoveTo(float64(r0*jsm.Cos(a1)), float64(r0*jsm.Sin(a1)))
			_ = context.Arc(0, 0, r0, a1, a0, cw)
		}
	} else {
		// Or is it a circular or annular sector?
		a01 := a0
		a11 := a1
		a00 := a0
		a10 := a1
		da0 := da
		da1 := da
		ap := a.padAngle(datum) / 2
		// rp is false in d3 when the pad angle is not above epsilon. False
		// is not above epsilon either, so zero stands for it here.
		rp := 0.0
		if ap > epsilon {
			if a.padRadius != nil {
				rp = a.padRadius(datum)
			} else {
				rp = math.Sqrt(float64(r0*r0) + float64(r1*r1))
			}
		}
		rc := js.Min(math.Abs(r1-r0)/2, a.cornerRadius(datum))
		rc0 := rc
		rc1 := rc
		var t0, t1 tangents

		// Apply padding? Note that since r1 ≥ r0, da1 ≥ da0.
		if rp > epsilon {
			p0 := asin(float64(rp / r0 * jsm.Sin(ap)))
			p1 := asin(float64(rp / r1 * jsm.Sin(ap)))
			da0 -= float64(p0 * 2)
			if da0 > epsilon {
				if !cw {
					p0 = float64(p0 * -1)
				}
				a00 += p0
				a10 -= p0
			} else {
				da0 = 0
				a00 = (a0 + a1) / 2
				a10 = a00
			}
			da1 -= float64(p1 * 2)
			if da1 > epsilon {
				if !cw {
					p1 = float64(p1 * -1)
				}
				a01 += p1
				a11 -= p1
			} else {
				da1 = 0
				a01 = (a0 + a1) / 2
				a11 = a01
			}
		}

		x01 := float64(r1 * jsm.Cos(a01))
		y01 := float64(r1 * jsm.Sin(a01))
		x10 := float64(r0 * jsm.Cos(a10))
		y10 := float64(r0 * jsm.Sin(a10))
		var x11, y11, x00, y00 float64

		// Apply rounded corners?
		if rc > epsilon {
			x11 = float64(r1 * jsm.Cos(a11))
			y11 = float64(r1 * jsm.Sin(a11))
			x00 = float64(r0 * jsm.Cos(a00))
			y00 = float64(r0 * jsm.Sin(a00))

			// Restrict the corner radius according to the sector angle. If this
			// intersection fails, it's probably because the arc is too small, so
			// disable the corner radius entirely.
			if da < pi {
				if ocx, ocy, ok := intersect(x01, y01, x00, y00, x11, y11, x10, y10); ok {
					ax := x01 - ocx
					ay := y01 - ocy
					bx := x11 - ocx
					by := y11 - ocy
					kc := 1 / jsm.Sin(acos((float64(ax*bx)+float64(ay*by))/(math.Sqrt(float64(ax*ax)+float64(ay*ay))*math.Sqrt(float64(bx*bx)+float64(by*by))))/2)
					lc := math.Sqrt(float64(ocx*ocx) + float64(ocy*ocy))
					rc0 = js.Min(rc, (r0-lc)/(kc-1))
					rc1 = js.Min(rc, (r1-lc)/(kc+1))
				} else {
					rc0, rc1 = 0, 0
				}
			}
		}

		if !(da1 > epsilon) {
			// Is the sector collapsed to a line?
			context.MoveTo(x01, y01)
		} else if rc1 > epsilon {
			// Does the sector's outer ring have rounded corners?
			t0 = cornerTangents(x00, y00, x01, y01, r1, rc1, cw)
			t1 = cornerTangents(x11, y11, x10, y10, r1, rc1, cw)

			context.MoveTo(t0.cx+t0.x01, t0.cy+t0.y01)

			if rc1 < rc {
				// Have the corners merged?
				_ = context.Arc(t0.cx, t0.cy, rc1, jsm.Atan2(t0.y01, t0.x01), jsm.Atan2(t1.y01, t1.x01), !cw)
			} else {
				// Otherwise, draw the two corners and the ring.
				_ = context.Arc(t0.cx, t0.cy, rc1, jsm.Atan2(t0.y01, t0.x01), jsm.Atan2(t0.y11, t0.x11), !cw)
				_ = context.Arc(0, 0, r1, jsm.Atan2(t0.cy+t0.y11, t0.cx+t0.x11), jsm.Atan2(t1.cy+t1.y11, t1.cx+t1.x11), !cw)
				_ = context.Arc(t1.cx, t1.cy, rc1, jsm.Atan2(t1.y11, t1.x11), jsm.Atan2(t1.y01, t1.x01), !cw)
			}
		} else {
			// Or is the outer ring just a circular arc?
			context.MoveTo(x01, y01)
			_ = context.Arc(0, 0, r1, a01, a11, !cw)
		}

		if !(r0 > epsilon) || !(da0 > epsilon) {
			// Is there no inner ring, and it's a circular sector?
			// Or perhaps it's an annular sector collapsed due to padding?
			context.LineTo(x10, y10)
		} else if rc0 > epsilon {
			// Does the sector's inner ring (or point) have rounded corners?
			t0 = cornerTangents(x10, y10, x11, y11, r0, -rc0, cw)
			t1 = cornerTangents(x01, y01, x00, y00, r0, -rc0, cw)

			context.LineTo(t0.cx+t0.x01, t0.cy+t0.y01)

			if rc0 < rc {
				// Have the corners merged?
				_ = context.Arc(t0.cx, t0.cy, rc0, jsm.Atan2(t0.y01, t0.x01), jsm.Atan2(t1.y01, t1.x01), !cw)
			} else {
				// Otherwise, draw the two corners and the ring.
				_ = context.Arc(t0.cx, t0.cy, rc0, jsm.Atan2(t0.y01, t0.x01), jsm.Atan2(t0.y11, t0.x11), !cw)
				_ = context.Arc(0, 0, r0, jsm.Atan2(t0.cy+t0.y11, t0.cx+t0.x11), jsm.Atan2(t1.cy+t1.y11, t1.cx+t1.x11), cw)
				_ = context.Arc(t1.cx, t1.cy, rc0, jsm.Atan2(t1.y11, t1.x11), jsm.Atan2(t1.y01, t1.x01), !cw)
			}
		} else {
			// Or is the inner ring just a circular arc?
			_ = context.Arc(0, 0, r0, a10, a00, cw)
		}
	}

	context.ClosePath()

	return context.String()
}

// Centroid is arc.centroid(datum).
func (a *Arc[T]) Centroid(datum T) (x, y float64) {
	r := (a.innerRadius(datum) + a.outerRadius(datum)) / 2
	// The conversion keeps the compiler from fusing the halving, which it
	// turns into a multiplication by 0.5, with the subtraction.
	angle := float64((a.startAngle(datum)+a.endAngle(datum))/2) - halfPi
	return float64(jsm.Cos(angle) * r), float64(jsm.Sin(angle) * r)
}
