// Ported from LLVM libc at revision b8b28272d32b, the revision of Chromium
// 156.0.8078.4: libc/src/__support/math/asin.h, acos.h and asin_utils.h
// (Apache-2.0 WITH LLVM-exception). The port has the fast pass of each
// function. The accurate pass is in big.go.
//
// The files of LLVM have no copyright line. Each file says:
//
//	Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
//	See https://llvm.org/LICENSE.txt for license information.
//	SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
// This file is a Go translation of those files. It is a changed version of
// them. LICENSE-LLVM.txt in this directory has the license.

package crmath

import "math"

// asinEval is asin_eval for two float64 numbers: asin(sqrt(u)) / sqrt(u)
// for 0 <= u <= 0.5, by the Taylor polynomial at the nearest multiple of
// 1/32. err is the error bound of the caller, and the result has the new
// bound.
func asinEval(u dd, err float64) (r dd, errOut float64) {
	// k = round(u * 32)
	k := math.RoundToEven(u.hi * 0x1.0p5)
	idx := uint(k)
	// y = u - k/32
	yHi := math.FMA(k, -0x1.0p-5, u.hi) // Exact
	y := ddExactAdd(yHi, u.lo)
	y2 := float64(y.hi * y.hi)
	// Add the error of the polynomial.
	errOut = math.FMA(err, y2, 0x1.0p-102)

	c := &crAsinCoeffs[idx]
	c0 := ddQuickMult(y, dd{c[3], c[2]})
	c1 := math.FMA(y.hi, c[5], c[4])
	c2 := math.FMA(y.hi, c[7], c[6])
	c3 := math.FMA(y.hi, c[9], c[8])
	c4 := math.FMA(y.hi, c[11], c[10])

	y4 := float64(y2 * y2)
	d0 := math.FMA(y2, c2, c1)
	d1 := math.FMA(y2, c4, c3)

	r = ddExactAdd(c[0], c0.hi)

	e1 := math.FMA(y4, d1, d0)

	r.lo = math.FMA(y2, e1, c[1]+c0.lo+r.lo)
	return r, errOut
}

// Asin is Math.asin, correctly rounded.
func Asin(x float64) float64 {
	if r, ok := asinFast(x); ok {
		return r
	}
	return asinSlow(x)
}

// asinFast is the fast pass of asin. ok is false when the rounding test
// fails.
func asinFast(x float64) (r float64, ok bool) {
	xExp := int(math.Float64bits(x) >> 52 & 0x7ff)

	// |x| < 0.5.
	if xExp < 1023-1 {
		// |x| < 2^-26.
		if xExp < 1023-26 {
			// asin(x) = x + x^3/6 + ..., and the rest is below ulp(x)/2.
			return math.FMA(x, 0x1.0p-54, x), true
		}

		// asin(x) = x P(x^2)
		xSq := ddExactMult(x, x)
		err := float64(math.Abs(x) * 0x1.0p-51)
		p, err := asinEval(xSq, err)
		r0 := ddExactMult(x, p.hi)
		rLo := math.FMA(x, p.lo, r0.lo)

		rUpper := r0.hi + (rLo + err)
		rLower := r0.hi + (rLo - err)
		return rUpper, rUpper == rLower
	}
	// |x| >= 0.5

	xAbs := math.Abs(x)
	xSign := 1.0
	if math.Signbit(x) {
		xSign = -1.0
	}

	// |x| >= 1
	if xExp >= 1023 {
		// x = +-1, asin(x) = +- pi/2
		if xAbs == 1.0 {
			return math.FMA(xSign, atanPiOver2.hi, xSign*atanPiOver2.lo), true
		}
		// |x| > 1, or x is NaN
		return math.NaN(), true
	}

	// When |x| >= 0.5, with u = (1 - |x|)/2:
	//   asin(x) = sign(x) (pi/2 - 2 sqrt(u) P(u))
	// u = (1 - |x|)/2
	u := math.FMA(xAbs, -0.5, 0.5)
	// v_hi + v_lo ~ sqrt(u), with the iteration of Newton:
	// h = u - v_hi^2 = (sqrt(u) - v_hi) (sqrt(u) + v_hi)
	vHi := math.Sqrt(u)
	h := math.FMA(vHi, -vHi, u)

	// Scale v_lo and v_hi by 2 from the formula:
	//   vh = v_hi * 2
	//   vl = 2*v_lo = h / v_hi.
	vh := vHi * 2.0
	vl := h / vHi

	err := float64(vh * 0x1.0p-51)
	p, err := asinEval(dd{0.0, u}, err)

	// Perform computations in double-double arithmetic:
	//   asin(x) = pi/2 - (v_hi + v_lo) * (ASIN_COEFFS[idx][0] + p)
	r0 := ddQuickMult(dd{vl, vh}, p)
	rr := ddExactAdd(atanPiOver2.hi, -r0.hi)

	rLo := atanPiOver2.lo - r0.lo + rr.lo

	rUpper := math.FMA(rr.hi, xSign, math.FMA(rLo, xSign, err))
	rLower := math.FMA(rr.hi, xSign, math.FMA(rLo, xSign, -err))
	return rUpper, rUpper == rLower
}

// Acos is Math.acos, correctly rounded.
func Acos(x float64) float64 {
	if r, ok := acosFast(x); ok {
		return r
	}
	return acosSlow(x)
}

// acosFast is the fast pass of acos. ok is false when the rounding test
// fails.
func acosFast(x float64) (r float64, ok bool) {
	xExp := int(math.Float64bits(x) >> 52 & 0x7ff)

	// |x| < 0.5.
	if xExp < 1023-1 {
		// |x| < 2^-55.
		if xExp < 1023-55 {
			// acos(x) = pi/2 - asin(x), and |x| is below ulp(pi/2)/4.
			return (math.Abs(x) + 0x1.0p-160) + atanPiOver2.hi, true
		}

		// acos(x) = pi/2 - x P(x^2)
		xSq := ddExactMult(x, x)
		err := float64(math.Abs(x) * 0x1.0p-51)
		p, err := asinEval(xSq, err)
		r0 := ddExactMult(x, p.hi)
		// Direct Fast2Sum, because |x P(x^2)| < pi/2.
		rHi := math.FMA(-x, p.hi, atanPiOver2.hi)
		rLo := ((atanPiOver2.hi - rHi) - r0.hi) - r0.lo
		rLo = math.FMA(-x, p.lo, rLo+atanPiOver2.lo)

		rUpper := rHi + (rLo + err)
		rLower := rHi + (rLo - err)
		return rUpper, rUpper == rLower
	}
	// |x| >= 0.5

	xAbs := math.Abs(x)
	neg := math.Signbit(x)

	// |x| >= 1
	if xExp >= 1023 {
		// x = +-1, acos(x) = 0 or pi
		if xAbs == 1.0 {
			if x == 1.0 {
				return 0, true
			}
			return math.FMA(1.0, atanPi.hi, atanPi.lo), true
		}
		// |x| > 1, or x is NaN
		return math.NaN(), true
	}

	// When |x| >= 0.5, with u = (1 - |x|)/2:
	//   acos(x) = 2 sqrt(u) P(u) for x > 0,
	//   acos(x) = pi - 2 sqrt(u) P(u) for x < 0.
	u := math.FMA(xAbs, -0.5, 0.5)
	vHi := math.Sqrt(u)
	h := math.FMA(vHi, -vHi, u)

	vh := vHi * 2.0
	vl := h / vHi

	err := float64(vh * 0x1.0p-51)
	p, err := asinEval(dd{0.0, u}, err)

	r0 := ddQuickMult(dd{vl, vh}, p)

	rHi, rLo := r0.hi, r0.lo
	if neg {
		rr := ddExactAdd(atanPi.hi, -r0.hi)
		rHi = rr.hi
		rLo = (atanPi.lo - r0.lo) + rr.lo
	}

	rUpper := rHi + (rLo + err)
	rLower := rHi + (rLo - err)
	return rUpper, rUpper == rLower
}
