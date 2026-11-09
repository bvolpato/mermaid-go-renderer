// Ported from V8 15.6.75, src/builtins/math.tq and
// src/codegen/code-stub-assembler.cc (Float64Round) (BSD-3-Clause, Copyright
// 2019 and 2016 the V8 project authors, ../LICENSE-V8.txt).

package crmath

import (
	"math"
	"math/bits"
)

// Sqrt is Math.sqrt. IEEE 754 defines the result.
func Sqrt(x float64) float64 { return math.Sqrt(x) }

// Hypot is Math.hypot. V8 has one formula for two arguments, one for three
// arguments and a loop with a compensated sum for more arguments.
func Hypot(values ...float64) float64 {
	inf := math.Inf(1)
	switch len(values) {
	case 0:
		return 0
	case 1:
		return math.Abs(values[0])
	case 2:
		// FastMathHypot
		a, b := math.Abs(values[0]), math.Abs(values[1])
		if a == inf || b == inf {
			return inf
		}
		max := float64Max(a, b)
		if math.IsNaN(max) {
			return math.NaN()
		}
		if max == 0 {
			return 0
		}
		return math.Sqrt(float64((a/max)*(a/max))+float64((b/max)*(b/max))) * max
	case 3:
		a, b, c := math.Abs(values[0]), math.Abs(values[1]), math.Abs(values[2])
		if a == inf || b == inf || c == inf {
			return inf
		}
		max := float64Max(float64Max(a, b), c)
		if math.IsNaN(max) {
			return math.NaN()
		}
		if max == 0 {
			return 0
		}
		powerA := float64((a / max) * (a / max))
		powerB := float64((b / max) * (b / max))
		compensation := (powerA + powerB) - powerA - powerB
		powerC := float64((c/max)*(c/max)) - compensation
		return math.Sqrt(powerA+powerB+powerC) * max
	}
	// MathHypot, the slow path.
	oneArgIsNaN := false
	max := 0.0
	for _, value := range values {
		if math.IsNaN(value) {
			oneArgIsNaN = true
		} else if absValue := math.Abs(value); absValue > max {
			max = absValue
		}
	}
	switch {
	case max == inf:
		return inf
	case oneArgIsNaN:
		return math.NaN()
	case max == 0:
		return 0
	}
	// Kahan summation to avoid rounding errors.
	sum, compensation := 0.0, 0.0
	for _, value := range values {
		n := math.Abs(value) / max
		summand := float64(n*n) - compensation
		preliminary := sum + summand
		compensation = (preliminary - sum) - summand
		sum = preliminary
	}
	return math.Sqrt(sum) * max
}

// float64Max is Float64Max: NaN when an argument is NaN.
func float64Max(a, b float64) float64 {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.NaN()
	}
	if a > b {
		return a
	}
	return b
}

// Round is Math.round: a tie goes toward +Infinity (Float64Round).
func Round(x float64) float64 {
	c := math.Ceil(x)
	if c-0.5 <= x {
		return c
	}
	return c - 1
}

// Trunc is Math.trunc.
func Trunc(x float64) float64 { return math.Trunc(x) }

// Sign is Math.sign.
func Sign(x float64) float64 {
	switch {
	case x < 0:
		return -1
	case x > 0:
		return 1
	}
	return x
}

// Fround is Math.fround: the nearest float32, as a float64.
func Fround(x float64) float64 { return float64(float32(x)) }

// ToInt32 is the ToInt32 operation of ECMAScript: the integer part of x
// modulo 2^32, as a signed number.
func ToInt32(x float64) int32 {
	b := math.Float64bits(x)
	// |x| = mant * 2^exp for a normal number.
	exp := int(b>>52&0x7ff) - 1075
	mant := b&(1<<52-1) | 1<<52
	var v uint32
	switch {
	case exp <= -53, exp >= 32:
		// |x| < 1, a multiple of 2^32, an infinity or NaN.
	case exp < 0:
		v = uint32(mant >> uint(-exp))
	default:
		v = uint32(mant) << uint(exp)
	}
	if b>>63 != 0 {
		v = -v
	}
	return int32(v)
}

// ToUint32 is the ToUint32 operation of ECMAScript.
func ToUint32(x float64) uint32 { return uint32(ToInt32(x)) }

// Clz32 is Math.clz32: the number of leading zero bits of ToUint32(x).
func Clz32(x float64) float64 { return float64(bits.LeadingZeros32(ToUint32(x))) }

// Imul is Math.imul: the 32-bit product of ToInt32(a) and ToInt32(b).
func Imul(a, b float64) float64 { return float64(int32(ToUint32(a) * ToUint32(b))) }
