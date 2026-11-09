// Ported from LLVM libc at revision b8b28272d32b, the revision of Chromium
// 156.0.8078.4: libc/src/__support/math/sin.h, cos.h, tan.h, sincos_eval.h,
// range_reduction_double_common.h and range_reduction_double_fma.h
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

// rangeReductionSmall is range_reduction_small: for |x| < 2^16 it returns
// k = round(x 128/pi) and u = x - k pi/128.
func rangeReductionSmall(x float64) (k uint32, u dd) {
	const (
		mPiOver128Hi  = -0x1.921fb544p-6
		mPiOver128Mid = -0x1.0b4611a6p-40
		mPiOver128Lo  = -0x1.3198a2e037073p-75
		invPi128      = 0x1.45f306dc9c883p5
	)
	prodHi := float64(x * invPi128)
	kd := math.RoundToEven(prodHi)

	yHi := math.FMA(kd, mPiOver128Hi, x) // Exact
	u.hi = math.FMA(kd, mPiOver128Mid, yHi)
	u0 := yHi - u.hi                      // Exact
	u1 := math.FMA(kd, mPiOver128Mid, u0) // Exact
	u.lo = math.FMA(kd, mPiOver128Lo, u1)
	return uint32(int64(kd)), u
}

// rangeReductionLarge is LargeRangeReduction::fast: the same reduction for
// |x| >= 2^16, with 128/pi from a table.
func rangeReductionLarge(x float64) (k uint32, u dd) {
	bits := math.Float64bits(x)
	xEM62 := int(bits>>52&0x7ff) - (1023 + 62)
	idx := (xEM62 >> 4) + 3
	bits = bits&^(0x7ff<<52) | uint64((xEM62&15)+1023+62)<<52
	xReduced := math.Float64frombits(bits)
	t := &crInvPi128[idx]
	ph := ddExactMult(xReduced, t[0])
	pm := ddExactMult(xReduced, t[1])
	pl := ddExactMult(xReduced, t[2])
	sumHi := ph.lo + pm.hi
	kd := math.RoundToEven(sumHi)

	yHi := (ph.lo - kd) + pm.hi // Exact
	yMid := ddExactAdd(pm.lo, pl.hi)
	yLo := pl.lo

	yL := math.FMA(xReduced, t[3], yLo)
	y := ddExactAdd(yHi, yMid.hi)
	y.lo += yMid.lo + yL

	u = ddQuickMult(y, dd{0x1.1a62633145c07p-60, 0x1.921fb54442d18p-6})
	return uint32(int64(kd)), u
}

// sincosEval is sincos_eval: sin(u) and cos(u) for |u| <= pi/256, and the
// bound of the error.
func sincosEval(u dd) (sinU, cosU dd, err float64) {
	uHiSq := float64(u.hi * u.hi)
	p1 := math.FMA(uHiSq, -0x1.a01a01a01a01ap-13, 0x1.1111111111111p-7)
	q1 := math.FMA(uHiSq, 0x1.5555555555555p-5, -0x1.0p-1)
	uHi3 := float64(uHiSq * u.hi)
	p2 := math.FMA(uHiSq, p1, -0x1.5555555555555p-3)
	q2 := math.FMA(uHiSq, q1, 1.0)
	sinLo := math.FMA(uHi3, p2, float64(u.lo*q2))

	uHiNegHalf := float64(-0.5 * u.hi)
	var v dd
	v.hi = math.FMA(u.hi, uHiNegHalf, 1.0)
	v.lo = 1.0 - v.hi // Exact
	v.lo = math.FMA(u.hi, uHiNegHalf, v.lo)

	r1 := math.FMA(uHiSq, 0x1.a01a01a01a01ap-16, -0x1.6c16c16c16c17p-10)
	s1 := math.FMA(uHiSq, 0x1.5555555555555p-3, -1.0)
	uHi4 := float64(uHiSq * uHiSq)
	uHiULo := float64(u.hi * u.lo)
	r2 := math.FMA(uHiSq, r1, 0x1.5555555555555p-5)
	s2 := math.FMA(uHiULo, s1, v.lo)
	cosLo := math.FMA(uHi4, r2, s2)

	sinU = ddExactAdd(u.hi, sinLo)
	cosU = ddExactAdd(v.hi, cosLo)
	return sinU, cosU, math.FMA(math.Abs(uHi3), 0x1.0p-51, 0x1.0p-105)
}

// Sin is Math.sin, correctly rounded.
func Sin(x float64) float64 {
	if r, ok := sinFast(x); ok {
		return r
	}
	return sinSlow(x)
}

// sinFast is the fast pass of sin. ok is false when the rounding test fails.
func sinFast(x float64) (r float64, ok bool) {
	xE := int(math.Float64bits(x) >> 52 & 0x7ff)
	var y dd
	var k uint32
	if xE < 1023+16 {
		// |x| < 2^16
		if xE < 1023-4 {
			// |x| < 2^-4
			if xE < 1023-26 {
				// |x| < 2^-26, |sin(x) - x| < ulp(x)/2.
				if x == 0 {
					return x, true
				}
				return math.FMA(x, -0x1.0p-54, x), true
			}
			// A polynomial of degree 9, without range reduction.
			xSq := float64(x * x)
			c0 := math.FMA(xSq, 0x1.111111110f491p-7, -0x1.5555555555555p-3)
			c1 := math.FMA(xSq, 0x1.71c24233f1bafp-19, -0x1.a01a00e16af3ep-13)
			x4 := float64(xSq * xSq)
			x3 := float64(x * xSq)
			rLo := float64(math.FMA(x4, c1, c0) * x3)
			// Overall errors <= ulp(x^3) + |x| * 2^-68.
			err := math.FMA(xSq, 0x1.0p-52, 0x1.0p-68)
			rLoU := math.FMA(x, err, rLo)
			rLoL := math.FMA(-x, err, rLo)
			rUpper := x + rLoU
			rLower := x + rLoL
			return rUpper, rUpper == rLower
		}
		k, y = rangeReductionSmall(x)
	} else {
		if xE > 2*1023 {
			return math.NaN(), true // Inf or NaN
		}
		k, y = rangeReductionLarge(x)
	}

	sinY, cosY, err := sincosEval(y)
	// cos(k pi/128) = sin((k + 64) pi/128)
	sinK := crSinKPi128[k&255]
	cosK := crSinKPi128[(k+64)&255]
	// sin(x) = sin(y) cos(k pi/128) + cos(y) sin(k pi/128)
	sinKCosY := ddQuickMult(cosY, sinK)
	cosKSinY := ddQuickMult(sinY, cosK)
	rr := ddExactAdd(sinKCosY.hi, cosKSinY.hi)
	rr.lo += sinKCosY.lo + cosKSinY.lo

	// The rounding test of Ziv.
	rlp := rr.lo + err
	rlm := rr.lo - err
	rUpper := rr.hi + rlp
	rLower := rr.hi + rlm
	return rUpper, rUpper == rLower
}

// Cos is Math.cos, correctly rounded.
func Cos(x float64) float64 {
	if r, ok := cosFast(x); ok {
		return r
	}
	return cosSlow(x)
}

// cosFast is the fast pass of cos. ok is false when the rounding test fails.
func cosFast(x float64) (r float64, ok bool) {
	xE := int(math.Float64bits(x) >> 52 & 0x7ff)
	var y dd
	var k uint32
	if xE < 1023+16 {
		// |x| < 2^16
		if xE < 1023-4 {
			// |x| < 2^-4
			if xE < 1023-27 {
				// |x| < 2^-27, |cos(x) - 1| < ulp(1 - 2^-53)/2.
				return 1, true
			}
			// A polynomial of degree 8, without range reduction.
			xSq := float64(x * x)
			c0 := math.FMA(xSq, 0x1.5555555555262p-5, -0x1p-1)
			c1 := math.FMA(xSq, 0x1.a00ffd769159ap-16, -0x1.6c16c1508bff1p-10)
			x4 := float64(xSq * xSq)
			rLo := float64(math.FMA(x4, c1, c0) * xSq)
			// Overall errors <= 1.0 * ulp(x^2) + 2^-69.
			err := math.FMA(xSq, 0x1.0p-51, 0x1.0p-69)
			rLoU := rLo + err
			rLoL := rLo - err
			rUpper := 1.0 + rLoU
			rLower := 1.0 + rLoL
			return rUpper, rUpper == rLower
		}
		k, y = rangeReductionSmall(x)
	} else {
		if xE > 2*1023 {
			return math.NaN(), true // Inf or NaN
		}
		k, y = rangeReductionLarge(x)
	}

	sinY, cosY, err := sincosEval(y)
	// -sin(k pi/128) = sin((k + 128) pi/128)
	msinK := crSinKPi128[(k+128)&255]
	cosK := crSinKPi128[(k+64)&255]
	// cos(x) = cos(y) cos(k pi/128) - sin(y) sin(k pi/128)
	cosKCosY := ddQuickMult(cosY, cosK)
	msinKSinY := ddQuickMult(sinY, msinK)
	rr := ddExactAdd(cosKCosY.hi, msinKSinY.hi)
	rr.lo += msinKSinY.lo + cosKCosY.lo

	rlp := rr.lo + err
	rlm := rr.lo - err
	rUpper := rr.hi + rlp
	rLower := rr.hi + rlm
	return rUpper, rUpper == rLower
}

// tanEval is tan_eval: tan(u) for |u| <= pi/256, and the bound of the error.
func tanEval(u dd) (result dd, err float64) {
	uHiSq := float64(u.hi * u.hi)
	// p1 ~ 17/315 + u_hi^2 62 / 2835.
	p1 := math.FMA(uHiSq, 0x1.664f4882c10fap-6, 0x1.ba1ba1ba1ba1cp-5)
	// p2 ~ 1/3 + u_hi^2 2 / 15.
	p2 := math.FMA(uHiSq, 0x1.1111111111111p-3, 0x1.5555555555555p-2)
	// q1 ~ 1 + u_hi^2 * 2/3.
	q1 := math.FMA(uHiSq, 0x1.5555555555555p-1, 1.0)
	uHi3 := float64(uHiSq * u.hi)
	uHi4 := float64(uHiSq * uHiSq)
	p3 := math.FMA(uHi4, p1, p2)
	q2 := math.FMA(uHiSq, q1, 1.0)
	tanLo := math.FMA(uHi3, p3, float64(u.lo*q2))
	result = ddExactAdd(u.hi, tanLo)
	return result, math.FMA(math.Abs(uHi3), 0x1.0p-51, 0x1.0p-102)
}

// Tan is Math.tan, correctly rounded.
func Tan(x float64) float64 {
	if r, ok := tanFast(x); ok {
		return r
	}
	return tanSlow(x)
}

// tanFast is the fast pass of tan. ok is false when the rounding test fails.
func tanFast(x float64) (r float64, ok bool) {
	xE := int(math.Float64bits(x) >> 52 & 0x7ff)
	var y dd
	var k uint32
	if xE < 1023+16 {
		// |x| < 2^16
		if xE < 1023-7 {
			// |x| < 2^-7
			if xE < 1023-27 {
				// |x| < 2^-27, |tan(x) - x| < ulp(x)/2.
				if x == 0 {
					return x, true
				}
				return math.FMA(x, 0x1.0p-54, x), true
			}
			// No range reduction needed.
			k = 0
			y = dd{0, x}
		} else {
			k, y = rangeReductionSmall(x)
		}
	} else {
		if xE > 2*1023 {
			return math.NaN(), true // Inf or NaN
		}
		k, y = rangeReductionLarge(x)
	}

	tanY, err := tanEval(y)
	msinK := crSinKPi128[(k+128)&255]
	cosK := crSinKPi128[(k+64)&255]
	// tan(x) = (sin(k pi/128) + tan(y) cos(k pi/128)) /
	//          (cos(k pi/128) - tan(y) sin(k pi/128))
	cosKTanY := ddQuickMult(tanY, cosK)
	msinKTanY := ddQuickMult(tanY, msinK)
	numDD := ddExactAdd2(cosKTanY.hi, -msinK.hi)
	denDD := ddExactAdd2(msinKTanY.hi, cosK.hi)
	numDD.lo += cosKTanY.lo - msinK.lo
	denDD.lo += msinKTanY.lo + cosK.lo

	tanX := ddDiv(numDD, denDD)
	// A bound of |1 / den_dd|: 2^(1 + floor(-log2(den_dd))).
	denInv := uint64(1024)<<53 - math.Float64bits(denDD.hi)&0x7ff0000000000000
	// | tan_x - num_dd / den_dd | <= err * ( 1 + | tan_x * den_dd | ).
	tanErr := float64(err * math.FMA(math.Float64frombits(denInv), math.Abs(tanX.hi), 1.0))

	errHigher := tanX.lo + tanErr
	errLower := tanX.lo - tanErr
	tanUpper := tanX.hi + errHigher
	tanLower := tanX.hi + errLower
	return tanUpper, tanUpper == tanLower
}
