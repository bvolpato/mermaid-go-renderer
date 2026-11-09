// Ported from plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/math/KVector.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package emath has the geometry classes of org.eclipse.elk.core.math.
package emath

import (
	"errors"
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// KVector is a vector with two components. ELK shares and changes vectors in
// place, so ported code passes *KVector.
type KVector struct {
	X, Y float64
}

// NewKVector is new KVector(x, y).
func NewKVector(x, y float64) *KVector { return &KVector{x, y} }

// Clone is clone() and the copy constructor.
func (v *KVector) Clone() *KVector { return &KVector{v.X, v.Y} }

// String is toString().
func (v *KVector) String() string { return "(" + js.Str(v.X) + "," + js.Str(v.Y) + ")" }

// Equals is equals(Object).
func (v *KVector) Equals(o *KVector) bool { return o != nil && v.X == o.X && v.Y == o.Y }

// EqualsFuzzily is equalsFuzzily(other) with the default fuzzyness.
func (v *KVector) EqualsFuzzily(o *KVector) bool { return v.EqualsFuzzilyBy(o, 0.05) }

// EqualsFuzzilyBy is equalsFuzzily(other, fuzzyness).
func (v *KVector) EqualsFuzzilyBy(o *KVector, fuzzyness float64) bool {
	return math.Abs(v.X-o.X) <= fuzzyness && math.Abs(v.Y-o.Y) <= fuzzyness
}

// HashCode is hashCode().
func (v *KVector) HashCode() int32 {
	return gwt.DoubleHash(v.X) + gwt.IntegerReverse(gwt.DoubleHash(v.Y))
}

// Length is length().
func (v *KVector) Length() float64 {
	return math.Sqrt(float64(v.X*v.X) + float64(v.Y*v.Y))
}

// SquareLength is squareLength().
func (v *KVector) SquareLength() float64 {
	return float64(v.X*v.X) + float64(v.Y*v.Y)
}

// Reset is reset().
func (v *KVector) Reset() *KVector {
	v.X, v.Y = 0, 0
	return v
}

// Set is set(KVector).
func (v *KVector) Set(o *KVector) *KVector {
	v.X, v.Y = o.X, o.Y
	return v
}

// SetXY is set(double, double).
func (v *KVector) SetXY(x, y float64) *KVector {
	v.X, v.Y = x, y
	return v
}

// Add is add(KVector).
func (v *KVector) Add(o *KVector) *KVector {
	v.X += o.X
	v.Y += o.Y
	return v
}

// AddXY is add(double, double).
func (v *KVector) AddXY(dx, dy float64) *KVector {
	v.X += dx
	v.Y += dy
	return v
}

// Sum is KVector.sum(vs...).
func Sum(vs ...*KVector) *KVector {
	sum := &KVector{}
	for _, v := range vs {
		sum.X += v.X
		sum.Y += v.Y
	}
	return sum
}

// Sub is sub(KVector).
func (v *KVector) Sub(o *KVector) *KVector {
	v.X -= o.X
	v.Y -= o.Y
	return v
}

// SubXY is sub(double, double).
func (v *KVector) SubXY(dx, dy float64) *KVector {
	v.X -= dx
	v.Y -= dy
	return v
}

// Diff is KVector.diff(v1, v2).
func Diff(v1, v2 *KVector) *KVector { return &KVector{v1.X - v2.X, v1.Y - v2.Y} }

// Scale is scale(double).
func (v *KVector) Scale(scale float64) *KVector {
	v.X *= scale
	v.Y *= scale
	return v
}

// ScaleXY is scale(double, double).
func (v *KVector) ScaleXY(sx, sy float64) *KVector {
	v.X *= sx
	v.Y *= sy
	return v
}

// Normalize is normalize().
func (v *KVector) Normalize() *KVector {
	length := v.Length()
	if length > 0 {
		v.X /= length
		v.Y /= length
	}
	return v
}

// ScaleToLength is scaleToLength(length).
func (v *KVector) ScaleToLength(length float64) *KVector {
	v.Normalize()
	v.Scale(length)
	return v
}

// Negate is negate().
func (v *KVector) Negate() *KVector {
	v.X = -v.X
	v.Y = -v.Y
	return v
}

// Distance is distance(v2).
func (v *KVector) Distance(v2 *KVector) float64 {
	dx := v.X - v2.X
	dy := v.Y - v2.Y
	return math.Sqrt(float64(dx*dx) + float64(dy*dy))
}

// DotProduct is dotProduct(v2).
func (v *KVector) DotProduct(v2 *KVector) float64 {
	return float64(v.X*v2.X) + float64(v.Y*v2.Y)
}

// CrossProduct is KVector.crossProduct(v, w).
func CrossProduct(v, w *KVector) float64 {
	return float64(v.X*w.Y) - float64(v.Y*w.X)
}

// Bound is bound(lowx, lowy, highx, highy). The bounds must be ordered.
func (v *KVector) Bound(lowx, lowy, highx, highy float64) *KVector {
	if v.X < lowx {
		v.X = lowx
	} else if v.X > highx {
		v.X = highx
	}
	if v.Y < lowy {
		v.Y = lowy
	} else if v.Y > highy {
		v.Y = highy
	}
	return v
}

// IsNaN is isNaN().
func (v *KVector) IsNaN() bool { return v.X != v.X || v.Y != v.Y }

// IsInfinite is isInfinite().
func (v *KVector) IsInfinite() bool { return math.IsInf(v.X, 0) || math.IsInf(v.Y, 0) }

func isDelim(c byte, delims string) bool { return strings.IndexByte(delims, c) >= 0 }

// stripDelims removes the leading and the trailing delimiters of the text of
// a data object.
func stripDelims(s string) string {
	start := 0
	for start < len(s) && isDelim(s[start], "([{\"' \t\r\n") {
		start++
	}
	end := len(s)
	for end > 0 && isDelim(s[end-1], ")]}\"' \t\r\n") {
		end--
	}
	if start >= end {
		return ""
	}
	return s[start:end]
}

// splitAny is String.split of GWT for a regular expression that is an
// alternation of single characters. Trailing empty strings are dropped.
func splitAny(s, chars string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(chars, s[i]) >= 0 {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	if len(s) > 0 {
		for len(out) > 0 && out[len(out)-1] == "" {
			out = out[:len(out)-1]
		}
	}
	return out
}

// Parse is parse(String): the text of an option value such as "(10, 20)".
// It returns an error where Java throws an IllegalArgumentException.
func (v *KVector) Parse(s string) error {
	body := stripDelims(s)
	if body == "" {
		return errors.New("the given string does not contain any numbers")
	}
	tokens := splitAny(body, ",;\r\n")
	if len(tokens) != 2 {
		return errors.New("exactly two numbers are expected")
	}
	x, ok1 := gwt.ParseDouble(gwt.Trim(tokens[0]))
	y, ok2 := gwt.ParseDouble(gwt.Trim(tokens[1]))
	if !ok1 || !ok2 {
		return errors.New("the given string contains parts that cannot be parsed as numbers")
	}
	v.X, v.Y = x, y
	return nil
}
