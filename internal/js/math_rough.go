// This file ports no upstream file. It has correctly rounded versions of the
// transcendental functions that roughjs calls: Math.sin, Math.cos, Math.tan,
// Math.atan and Math.asin.
//
// The reference browser is Chromium 156. Its V8 (version 15) takes these
// functions from LLVM libc, whose double-precision functions are correctly
// rounded: the result is the float64 that is nearest to the exact value. A
// correctly rounded result does not depend on the algorithm, so these
// functions give the results of that browser. Older versions of V8 and
// Node.js 26 have the fdlibm functions, which are not correctly rounded, and
// the math package of Go has other algorithms again.
//
// The check against the browser is in math_rough_test.go: SinCR and CosCR
// give every coordinate that mermaid computes with Math.cos and Math.sin for
// the stadium nodes of the reference SVGs. The wrappers Sin and Cos of
// math.go do not.
//
// SinCR, CosCR, TanCR and AsinCR are the functions Sin, Cos, Tan and Asin of
// math.go, which are correctly rounded and fast (package crmath). AtanCR
// computes with math/big and is slower: a call takes a few microseconds.
// Math.atan of the browser is not correctly rounded, so that AtanCR and Atan
// differ for about 1 percent of the arguments.

package js

import (
	"math"
	"math/big"
	"sync"
)

// crGuard is the number of bits that a computation has above the precision
// that the caller asks for.
const crGuard = 96

var (
	crOnce   sync.Once
	crHalfPi *big.Float // pi/2 with crPiPrec bits
)

// crPiPrec is the precision of pi/2. The argument reduction of the largest
// float64 needs 1024 bits above the working precision.
const crPiPrec = 4096

func crNew(prec uint) *big.Float { return new(big.Float).SetPrec(prec) }

// crAtanInv is atan(1/n) with prec bits, by the Taylor series.
func crAtanInv(n int64, prec uint) *big.Float {
	sum := crNew(prec)
	term := crNew(prec).Quo(crNew(prec).SetInt64(1), crNew(prec).SetInt64(n))
	n2 := crNew(prec).SetInt64(n * n)
	t := crNew(prec)
	for k := int64(0); ; k++ {
		t.Quo(term, crNew(prec).SetInt64(2*k+1))
		if t.MantExp(nil) < -int(prec) {
			break
		}
		if k%2 == 0 {
			sum.Add(sum, t)
		} else {
			sum.Sub(sum, t)
		}
		term.Quo(term, n2)
	}
	return sum
}

// crPiOver2 returns pi/2 rounded to prec bits.
func crPiOver2(prec uint) *big.Float {
	crOnce.Do(func() {
		// The Machin formula: pi/4 = 4 atan(1/5) - atan(1/239).
		p := uint(crPiPrec + 64)
		a := crAtanInv(5, p)
		a.Mul(a, crNew(p).SetInt64(8))
		b := crAtanInv(239, p)
		b.Mul(b, crNew(p).SetInt64(2))
		a.Sub(a, b)
		crHalfPi = a.SetPrec(crPiPrec)
	})
	return crNew(prec).Set(crHalfPi)
}

// crAtan returns atan(y) for a positive y, with a relative error below
// 2^-prec.
func crAtan(y *big.Float, prec uint) *big.Float {
	w := prec + crGuard
	one := crNew(w).SetInt64(1)
	a := crNew(w).Set(y)
	invert := a.Cmp(one) > 0
	if invert {
		// atan(y) = pi/2 - atan(1/y)
		a.Quo(one, a)
	}
	// atan(a) = 2 atan(a / (1 + sqrt(1 + a^2)))
	const halvings = 6
	t := crNew(w)
	for i := 0; i < halvings; i++ {
		t.Mul(a, a)
		t.Add(t, one)
		t.Sqrt(t)
		t.Add(t, one)
		a.Quo(a, t)
	}
	// The Taylor series a - a^3/3 + a^5/5 - ...
	a2 := crNew(w).Mul(a, a)
	sum := crNew(w).Set(a)
	term := crNew(w).Set(a)
	d := crNew(w)
	for i := int64(1); i < 2000; i++ {
		term.Mul(term, a2)
		t.Quo(term, d.SetInt64(2*i+1))
		if i%2 == 1 {
			sum.Sub(sum, t)
		} else {
			sum.Add(sum, t)
		}
		if t.Sign() == 0 || t.MantExp(nil)-sum.MantExp(nil) < -int(w)-8 {
			break
		}
	}
	sum.SetMantExp(sum, halvings)
	if invert {
		sum.Sub(crPiOver2(w), sum)
	}
	return sum
}

// crRound rounds v to a float64. v has a relative error below 2^-prec. ok is
// false when the values within the error bound do not all round to the same
// float64.
func crRound(v *big.Float, prec uint) (r float64, ok bool) {
	if v.Sign() == 0 {
		return 0, true
	}
	p := v.Prec()
	// 2^-prec times a number above |v|.
	e := crNew(p).SetMantExp(crNew(p).SetInt64(1), v.MantExp(nil)-int(prec))
	lo, _ := crNew(p).Sub(v, e).Float64()
	hi, _ := crNew(p).Add(v, e).Float64()
	return lo, lo == hi
}

// crCompute calls f with a growing precision until the result rounds to one
// float64. f returns a value with a relative error below 2^-prec. The exact
// value is never in the middle of two float64 numbers for the functions of
// this file, so the loop ends.
func crCompute(f func(prec uint) *big.Float) float64 {
	for prec := uint(96); ; prec *= 2 {
		if r, ok := crRound(f(prec), prec); ok {
			return r
		}
	}
}

// SinCR is the correctly rounded sine: Math.sin of a browser with V8 15.
func SinCR(x float64) float64 { return Sin(x) }

// CosCR is the correctly rounded cosine: Math.cos of a browser with V8 15.
func CosCR(x float64) float64 { return Cos(x) }

// TanCR is the correctly rounded tangent.
func TanCR(x float64) float64 { return Tan(x) }

// AtanCR is the correctly rounded arc tangent.
func AtanCR(x float64) float64 {
	switch {
	case x == 0 || math.IsNaN(x):
		return x
	case math.IsInf(x, 0):
		r, _ := crPiOver2(128).Float64()
		return math.Copysign(r, x)
	}
	r := crCompute(func(prec uint) *big.Float {
		return crAtan(crNew(prec+crGuard).SetFloat64(math.Abs(x)), prec)
	})
	return math.Copysign(r, x)
}

// AsinCR is the correctly rounded arc sine.
func AsinCR(x float64) float64 { return Asin(x) }
