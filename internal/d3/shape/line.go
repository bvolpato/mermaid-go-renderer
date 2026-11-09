// Ported from d3-shape 3.2.0, d3-shape/src: line.js, area.js, point.js and
// array.js (ISC).

package shape

import "math"

// pointX and pointY are the default accessors of d3 (point.js): the elements
// 0 and 1 of the datum. They know the types [2]float64 and []float64. For
// another type the result is NaN, as for a JavaScript object without the
// properties 0 and 1.
func pointX[T any](d T, _ int, _ []T) float64 {
	switch p := any(d).(type) {
	case [2]float64:
		return p[0]
	case []float64:
		if len(p) > 0 {
			return p[0]
		}
	}
	return math.NaN()
}

func pointY[T any](d T, _ int, _ []T) float64 {
	switch p := any(d).(type) {
	case [2]float64:
		return p[1]
	case []float64:
		if len(p) > 1 {
			return p[1]
		}
	}
	return math.NaN()
}

func constant[T any](x float64) func(T, int, []T) float64 {
	return func(T, int, []T) float64 { return x }
}

// Line is the line generator of d3 (d3.line). T is the type of a datum.
type Line[T any] struct {
	x, y    func(d T, i int, data []T) float64
	defined func(d T, i int, data []T) bool
	curve   CurveFactory
	digits  digits
}

// NewLine is d3.line(). The default accessors read a [2]float64 or a
// []float64 datum; set X and Y for another type.
func NewLine[T any]() *Line[T] {
	return &Line[T]{
		x:       pointX[T],
		y:       pointY[T],
		defined: func(T, int, []T) bool { return true },
		curve:   CurveLinear,
		digits:  defaultDigits(),
	}
}

// X is line.x(accessor).
func (l *Line[T]) X(x func(d T, i int, data []T) float64) *Line[T] {
	l.x = x
	return l
}

// XConst is line.x(number).
func (l *Line[T]) XConst(x float64) *Line[T] {
	l.x = constant[T](x)
	return l
}

// Y is line.y(accessor).
func (l *Line[T]) Y(y func(d T, i int, data []T) float64) *Line[T] {
	l.y = y
	return l
}

// YConst is line.y(number).
func (l *Line[T]) YConst(y float64) *Line[T] {
	l.y = constant[T](y)
	return l
}

// Defined is line.defined(accessor). A datum for which the accessor returns
// false makes a gap in the line.
func (l *Line[T]) Defined(defined func(d T, i int, data []T) bool) *Line[T] {
	l.defined = defined
	return l
}

// Curve is line.curve(curve).
func (l *Line[T]) Curve(curve CurveFactory) *Line[T] {
	l.curve = curve
	return l
}

// Digits is line.digits(digits): the number of digits after the decimal
// point in the path data. The default is 3. A negative value is
// line.digits(null): the numbers are not rounded.
func (l *Line[T]) Digits(digits int) *Line[T] {
	l.digits.n = digits
	return l
}

// Path is line(data) with a null context: it returns the path data. The
// empty string is the null that d3 returns when the path has no commands.
func (l *Line[T]) Path(data []T) string {
	n := len(data)
	defined0 := false
	buffer := l.digits.newPath()
	output := l.curve(buffer)

	for i := 0; i <= n; i++ {
		if !(i < n && l.defined(data[i], i, data)) == defined0 {
			defined0 = !defined0
			if defined0 {
				output.LineStart()
			} else {
				output.LineEnd()
			}
		}
		if defined0 {
			output.Point(l.x(data[i], i, data), l.y(data[i], i, data))
		}
	}

	return buffer.String()
}

// Area is the area generator of d3 (d3.area). T is the type of a datum.
type Area[T any] struct {
	x0, x1, y0, y1 func(d T, i int, data []T) float64
	defined        func(d T, i int, data []T) bool
	curve          CurveFactory
	digits         digits
}

// NewArea is d3.area(). By default x0 is the element 0 of the datum, y0 is
// zero, y1 is the element 1 of the datum, and x1 is null.
func NewArea[T any]() *Area[T] {
	return &Area[T]{
		x0:      pointX[T],
		y0:      constant[T](0),
		y1:      pointY[T],
		defined: func(T, int, []T) bool { return true },
		curve:   CurveLinear,
		digits:  defaultDigits(),
	}
}

// X is area.x(accessor): it sets x0 and sets x1 to null.
func (a *Area[T]) X(x func(d T, i int, data []T) float64) *Area[T] {
	a.x0, a.x1 = x, nil
	return a
}

// X0 is area.x0(accessor).
func (a *Area[T]) X0(x func(d T, i int, data []T) float64) *Area[T] {
	a.x0 = x
	return a
}

// X1 is area.x1(accessor). A nil accessor is null: x1 is then x0.
func (a *Area[T]) X1(x func(d T, i int, data []T) float64) *Area[T] {
	a.x1 = x
	return a
}

// Y is area.y(accessor): it sets y0 and sets y1 to null.
func (a *Area[T]) Y(y func(d T, i int, data []T) float64) *Area[T] {
	a.y0, a.y1 = y, nil
	return a
}

// Y0 is area.y0(accessor).
func (a *Area[T]) Y0(y func(d T, i int, data []T) float64) *Area[T] {
	a.y0 = y
	return a
}

// Y0Const is area.y0(number).
func (a *Area[T]) Y0Const(y float64) *Area[T] {
	a.y0 = constant[T](y)
	return a
}

// Y1 is area.y1(accessor). A nil accessor is null: y1 is then y0.
func (a *Area[T]) Y1(y func(d T, i int, data []T) float64) *Area[T] {
	a.y1 = y
	return a
}

// Defined is area.defined(accessor).
func (a *Area[T]) Defined(defined func(d T, i int, data []T) bool) *Area[T] {
	a.defined = defined
	return a
}

// Curve is area.curve(curve).
func (a *Area[T]) Curve(curve CurveFactory) *Area[T] {
	a.curve = curve
	return a
}

// Digits is area.digits(digits). A negative value is area.digits(null).
func (a *Area[T]) Digits(digits int) *Area[T] {
	a.digits.n = digits
	return a
}

// Path is area(data) with a null context: it returns the path data. The
// empty string is the null that d3 returns when the path has no commands.
func (a *Area[T]) Path(data []T) string {
	n := len(data)
	defined0 := false
	x0z := make([]float64, n)
	y0z := make([]float64, n)
	buffer := a.digits.newPath()
	output := a.curve(buffer)
	j := 0

	for i := 0; i <= n; i++ {
		if !(i < n && a.defined(data[i], i, data)) == defined0 {
			defined0 = !defined0
			if defined0 {
				j = i
				output.AreaStart()
				output.LineStart()
			} else {
				output.LineEnd()
				output.LineStart()
				for k := i - 1; k >= j; k-- {
					output.Point(x0z[k], y0z[k])
				}
				output.LineEnd()
				output.AreaEnd()
			}
		}
		if defined0 {
			d := data[i]
			x0z[i], y0z[i] = a.x0(d, i, data), a.y0(d, i, data)
			x, y := x0z[i], y0z[i]
			if a.x1 != nil {
				x = a.x1(d, i, data)
			}
			if a.y1 != nil {
				y = a.y1(d, i, data)
			}
			output.Point(x, y)
		}
	}

	return buffer.String()
}

// arealine returns a new line generator with the defined accessor and the
// curve of the area. The line has its own digits setting.
func (a *Area[T]) arealine() *Line[T] {
	return NewLine[T]().Defined(a.defined).Curve(a.curve)
}

// LineX0 is area.lineX0(), which is also area.lineY0().
func (a *Area[T]) LineX0() *Line[T] { return a.arealine().X(a.x0).Y(a.y0) }

// LineY0 is area.lineY0().
func (a *Area[T]) LineY0() *Line[T] { return a.LineX0() }

// LineY1 is area.lineY1(). A null y1 gives a constant y of zero, as
// line.y(null) does.
func (a *Area[T]) LineY1() *Line[T] {
	l := a.arealine().X(a.x0)
	if a.y1 == nil {
		return l.YConst(0)
	}
	return l.Y(a.y1)
}

// LineX1 is area.lineX1(). A null x1 gives a constant x of zero, as
// line.x(null) does.
func (a *Area[T]) LineX1() *Line[T] {
	l := a.arealine().Y(a.y0)
	if a.x1 == nil {
		return l.XConst(0)
	}
	return l.X(a.x1)
}
