// This file ports no upstream file. It adds the correctly rounded Math.acos
// and Math.atan2 that the d3 port needs. It builds on the helpers of
// math_rough.go (crAtan, crCompute, crPiOver2), which has the correctly
// rounded sine, cosine, tangent, arc sine and arc tangent.
//
// The reference renders show that Chromium rounds Math.sin and Math.cos
// correctly (testdata/d3_chromium_trig.json, from the pie and radar charts).
// The renders of the d3 diagrams have no value of Math.acos or Math.atan2
// without rounding, so that these two functions are not checked against the
// browser. They are checked against mpmath (math_d3_test.go).

package js

import (
	"math"
	"math/big"
)

// crPiMultiple returns pi * num / den, correctly rounded.
func crPiMultiple(num, den int64) float64 {
	return crCompute(func(prec uint) *big.Float {
		w := prec + crGuard
		v := crPiOver2(w)
		v.Mul(v, crNew(w).SetInt64(2*num))
		return v.Quo(v, crNew(w).SetInt64(den))
	})
}

// AcosCR is the correctly rounded arc cosine.
func AcosCR(x float64) float64 { return Acos(x) }

// Atan2CR is the correctly rounded Math.atan2(y, x).
func Atan2CR(y, x float64) float64 {
	switch {
	case math.IsNaN(x) || math.IsNaN(y):
		return math.NaN()
	case y == 0:
		if x > 0 || (x == 0 && !math.Signbit(x)) {
			return y // +0 or -0
		}
		return math.Copysign(crPiMultiple(1, 1), y)
	case x == 0:
		return math.Copysign(crPiMultiple(1, 2), y)
	case math.IsInf(y, 0):
		switch {
		case math.IsInf(x, 1):
			return math.Copysign(crPiMultiple(1, 4), y)
		case math.IsInf(x, -1):
			return math.Copysign(crPiMultiple(3, 4), y)
		}
		return math.Copysign(crPiMultiple(1, 2), y)
	case math.IsInf(x, 1):
		return math.Copysign(0, y)
	case math.IsInf(x, -1):
		return math.Copysign(crPiMultiple(1, 1), y)
	}
	r := crCompute(func(prec uint) *big.Float {
		w := prec + crGuard + 64
		q := crNew(w).SetFloat64(math.Abs(y))
		q.Quo(q, crNew(w).SetFloat64(math.Abs(x)))
		a := crAtan(q, prec+4)
		if x < 0 {
			pi := crPiOver2(a.Prec())
			pi.SetMantExp(pi, 1)
			a.Sub(pi, a)
		}
		return a
	})
	return math.Copysign(r, y)
}
