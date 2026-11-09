// Ported from LLVM libc at revision b8b28272d32b, the revision of Chromium
// 156.0.8078.4: libc/src/__support/math/cbrt.h (Apache-2.0 WITH
// LLVM-exception). The port has the two fast passes. The accurate pass is in
// big.go.
//
// The file of LLVM has no copyright line. It says:
//
//	Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
//	See https://llvm.org/LICENSE.txt for license information.
//	SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
// This file is a Go translation of that file. It is a changed version of
// it. LICENSE-LLVM.txt in this directory has the license.

package crmath

import "math"

// cbrtRelError is REL_ERROR: the bound of the relative error of the first
// pass.
const cbrtRelError = 0x1.0p-51

// cbrtInitialApproximation is initial_approximation: a polynomial of degree
// 7 for x^(-2/3) on [1, 2], with a relative error below 1.28 * 2^-21.
func cbrtInitialApproximation(x float64) float64 {
	c := &crCbrtCoeffs
	xSq := float64(x * x)

	c0 := math.FMA(x, c[1], c[0])
	c1 := math.FMA(x, c[3], c[2])
	c2 := math.FMA(x, c[5], c[4])
	c3 := math.FMA(x, c[7], c[6])

	x4 := float64(xSq * xSq)
	d0 := math.FMA(xSq, c1, c0)
	d1 := math.FMA(xSq, c3, c2)

	return math.FMA(x4, d1, d0)
}

// cbrtGetError is get_error: h(x) = x^3 a^2 - 1.
func cbrtGetError(x3, aSq dd) float64 {
	return math.FMA(x3.hi, aSq.hi, -1.0) + math.FMA(x3.lo, aSq.hi, float64(x3.hi*aSq.lo))
}

// Cbrt is Math.cbrt, correctly rounded.
func Cbrt(x float64) float64 {
	if r, ok := cbrtFast(x); ok {
		return r
	}
	return cbrtSlow(x)
}

// cbrtFast has the two fast passes of cbrt. ok is false when both rounding
// tests fail.
func cbrtFast(x float64) (r float64, ok bool) {
	xAbs := math.Float64bits(x) &^ (1 << 63)

	expBiasCorrection := uint(682) // 1023 * 2/3

	if xAbs < 0x0010000000000000 || xAbs >= 0x7ff0000000000000 {
		if x == 0.0 || xAbs >= 0x7ff0000000000000 {
			// x is 0, Inf, or NaN.
			return x + x, true
		}

		// x is subnormal: normalize x.
		x *= 0x1.0p60
		expBiasCorrection -= 20
	}

	xBits := math.Float64bits(x)

	// When using biased exponent of x in double precision,
	//   x_e = real_exponent_of_x + 1023
	// Then:
	//   x_e / 3 = real_exponent_of_x / 3 + 1023/3
	//           = real_exponent_of_x / 3 + 341
	// So to make it the correct biased exponent of x^(1/3), we add
	//   1023 - 341 = 682.
	xE := uint(xBits >> 52 & 0x7ff)
	outE := xE/3 + expBiasCorrection
	shiftE := xE % 3

	// Set x_r = 1.mantissa
	xR := math.Float64frombits(xBits&0x000FFFFFFFFFFFFF | 1023<<52)

	// Set a = 2^shift_e * 1.mantissa, with the sign of x.
	aBits := xBits&0x800FFFFFFFFFFFFF | uint64(shiftE+1023)<<52
	a := math.Float64frombits(aBits)

	// Initial approximation of x_r^(-2/3).
	p := cbrtInitialApproximation(xR)

	// Look up for 2^(-2*n/3) used for first approximation step.
	exp2M2Over3 := [3]float64{1.0, 0x1.428a2f98d728bp-1, 0x1.965fea53d6e3dp-2}

	// x0 is an initial approximation of a^(-2/3) with a relative error of
	// about 2^-21.
	x0 := float64(exp2M2Over3[shiftE] * p)

	// The first iteration in two float64 numbers.
	//   h = a^2 x0^3 - 1
	aSq := ddExactMult(a, a)
	x0Sq := ddExactMult(x0, x0)
	x03 := ddQuickMultD(x0, x0Sq)

	h0 := cbrtGetError(x03, aSq)

	// Taylor polynomial of (1 + h)^(-1/3):
	//   (1 + h)^(-1/3) = 1 - h/3 + 2 h^2 / 9 - 14 h^3 / 81 + ...
	// e0 = -14 * h^2 / 81 + 2 * h / 9 - 1/3 - relative_error.
	e0 := math.FMA(h0, math.FMA(h0, -0x1.61f9add3c0ca4p-3, 0x1.c71c71c71c71cp-3), -0x1.5555555555555p-2-cbrtRelError)
	x0H0 := float64(x0 * h0)

	// x1 = x0 (1 - h/3 + 2 h^2 / 9 - 14 h^3 / 81)
	// x1 approximates a^(-2/3) with a relative error of about 2^-85.
	x1DD := dd{float64(x0H0 * e0), x0}

	// r1 = x1 * a ~ a^(-2/3) * a = a^(1/3).
	r1 := ddQuickMultD(a, x1DD)

	updateExponent := func(r float64) float64 {
		rM := math.Float64bits(r) - 0x3FF0000000000000
		return math.Float64frombits(rM + uint64(outE)<<52)
	}

	// Accurate checks and passes.
	r1Lower := r1.hi + r1.lo
	r1Upper := r1.hi + math.FMA(x0H0, 2.0*cbrtRelError*a, r1.lo)

	// Ziv's accuracy test.
	if r1Upper == r1Lower {
		return updateExponent(r1Lower), true
	}

	// Accuracy test failed, perform another Newton iteration.
	x1 := x1DD.hi + float64((e0+cbrtRelError)*x0H0)

	// Second iteration in double-double precision.
	// h1 = x1^3 * a^2 - 1.
	x1Sq := ddExactMult(x1, x1)
	x13 := ddQuickMultD(x1, x1Sq)
	h1 := cbrtGetError(x13, aSq)

	// e1 = -x1*h1/3.
	e1 := float64(h1 * float64(x1*-0x1.5555555555555p-2))
	// x2 = x1*(1 - h1/3) = x1 + e1 ~ a^(-2/3) with relative errors < 2^-101.
	x2 := ddExactAdd(x1, e1)
	// r2 = a * x2 ~ a^(1/3) with relative errors < 2^-100.
	r2 := ddQuickMultD(a, x2)

	// Check for rounding to nearest with ties to even.
	r2Upper := r2.hi + math.FMA(a, 0x1.0p-102, r2.lo)
	r2Lower := r2.hi + math.FMA(a, -0x1.0p-102, r2.lo)

	// Ziv's accuracy test.
	if r2Upper == r2Lower {
		return updateExponent(r2Upper), true
	}
	return 0, false
}
