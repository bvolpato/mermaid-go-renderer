// This file ports no upstream file. It has the Math functions of the port,
// which call the package crmath.

package js

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js/crmath"
)

// The Math functions below are the single entry point for the Math object in
// the port. Each one returns the same bits as the reference browser, Chromium
// 156. Go's math package does not: its results differ from the browser in
// the last bit for a large part of the arguments, and they differ between
// arm64 and amd64. The implementations and the golden tests against the
// browser are in the package crmath.
//
// Pow and Tanh depend on the platform of the browser, because Chromium takes
// them from the C library of the operating system. They return the result
// of the current platform (math_platform.go). The other functions have the
// same result on Linux and on macOS.

func Sin(x float64) float64      { return crmath.Sin(x) }
func Cos(x float64) float64      { return crmath.Cos(x) }
func Tan(x float64) float64      { return crmath.Tan(x) }
func Asin(x float64) float64     { return crmath.Asin(x) }
func Acos(x float64) float64     { return crmath.Acos(x) }
func Atan(x float64) float64     { return crmath.Atan(x) }
func Atan2(y, x float64) float64 { return crmath.Atan2(y, x) }
func Exp(x float64) float64      { return crmath.Exp(x) }
func Expm1(x float64) float64    { return crmath.Expm1(x) }
func Log(x float64) float64      { return crmath.Log(x) }
func Log1p(x float64) float64    { return crmath.Log1p(x) }
func Log2(x float64) float64     { return crmath.Log2(x) }
func Log10(x float64) float64    { return crmath.Log10(x) }
func Sqrt(x float64) float64     { return crmath.Sqrt(x) }
func Cbrt(x float64) float64     { return crmath.Cbrt(x) }
func Sinh(x float64) float64     { return crmath.Sinh(x) }
func Cosh(x float64) float64     { return crmath.Cosh(x) }
func Tanh(x float64) float64     { return crmath.Tanh(x) }
func Asinh(x float64) float64    { return crmath.Asinh(x) }
func Acosh(x float64) float64    { return crmath.Acosh(x) }
func Atanh(x float64) float64    { return crmath.Atanh(x) }

// Pow is Math.pow and the operator **, of the current platform.
func Pow(x, y float64) float64 { return crmath.Pow(x, y) }

// Hypot is Math.hypot.
func Hypot(values ...float64) float64 { return crmath.Hypot(values...) }

// Max and Min follow Math.max and Math.min: NaN wins, and -0 < +0.
func Max(values ...float64) float64 {
	r := math.Inf(-1)
	for _, v := range values {
		if math.IsNaN(v) {
			return v
		}
		if v > r || (v == 0 && r == 0 && !math.Signbit(v)) {
			r = v
		}
	}
	return r
}

func Min(values ...float64) float64 {
	r := math.Inf(1)
	for _, v := range values {
		if math.IsNaN(v) {
			return v
		}
		if v < r || (v == 0 && r == 0 && math.Signbit(v)) {
			r = v
		}
	}
	return r
}

// Sign is Math.sign.
func Sign(x float64) float64 { return crmath.Sign(x) }

// Trunc is Math.trunc.
func Trunc(x float64) float64 { return crmath.Trunc(x) }

// Fround is Math.fround: the nearest float32, as a float64.
func Fround(x float64) float64 { return crmath.Fround(x) }

// Clz32 is Math.clz32: the number of leading zero bits of ToUint32(x).
func Clz32(x float64) float64 { return crmath.Clz32(x) }

// Imul is Math.imul: the 32-bit product of ToInt32(a) and ToInt32(b).
func Imul(a, b float64) float64 { return crmath.Imul(a, b) }

// ToInt32 is the ToInt32 abstract operation, used by bitwise operators (x | 0, x >> 1).
func ToInt32(x float64) int32 { return crmath.ToInt32(x) }

// ToUint32 is the ToUint32 abstract operation (x >>> 0).
func ToUint32(x float64) uint32 { return crmath.ToUint32(x) }
