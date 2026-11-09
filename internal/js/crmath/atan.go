// Ported from LLVM libc at revision b8b28272d32b, the revision of Chromium
// 156.0.8078.4: libc/src/__support/math/atan.h, atan2.h and atan_utils.h
// (Apache-2.0 WITH LLVM-exception). These two functions are not correctly
// rounded in LLVM libc: they compute with two float64 numbers and return the
// sum. The port is complete and has the fused operations of the arm64 build.
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

var (
	atanPi           = dd{0x1.1a62633145c07p-53, 0x1.921fb54442d18p+1}
	atanMPi          = dd{-0x1.1a62633145c07p-53, -0x1.921fb54442d18p+1}
	atanPiOver2      = dd{0x1.1a62633145c07p-54, 0x1.921fb54442d18p0}
	atanMPiOver2     = dd{-0x1.1a62633145c07p-54, -0x1.921fb54442d18p0}
	atanPiOver4      = dd{0x1.1a62633145c07p-55, 0x1.921fb54442d18p-1}
	atanThreePiOver4 = dd{0x1.a79394c9e8a0ap-54, 0x1.2d97c7f3321d2p+1}
	atanZero         = dd{0, 0}
	atanMZero        = dd{math.Copysign(0, -1), math.Copysign(0, -1)}

	// atanIsNeg is IS_NEG.
	atanIsNeg = [2]float64{1.0, -1.0}

	// atan2ConstAdj is CONST_ADJ[x_sign][y_sign][recip]: the constant term.
	atan2ConstAdj = [2][2][2]dd{
		{{atanZero, atanMPiOver2}, {atanMZero, atanMPiOver2}},
		{{atanMPi, atanPiOver2}, {atanMPi, atanPiOver2}},
	}

	// atan2Excepts is EXCEPTS[y_except][x_except][x_is_neg], with 0 for a
	// zero, 1 for a finite number that is not zero and 2 for an infinity.
	atan2Excepts = [3][3][2]dd{
		{{atanZero, atanPi}, {atanZero, atanPi}, {atanZero, atanPi}},
		{{atanPiOver2, atanPiOver2}, {atanZero, atanZero}, {atanZero, atanPi}},
		{{atanPiOver2, atanPiOver2}, {atanPiOver2, atanPiOver2}, {atanPiOver4, atanThreePiOver4}},
	}
)

// atanEval is atan_eval: atan(x) for |x| <= 2^-7 by the Taylor polynomial of
// degree 9.
func atanEval(x dd) dd {
	var p dd
	p.hi = x.hi
	xHiSq := float64(x.hi * x.hi)
	// c0 ~ x_hi^2 * 1/5 - 1/3
	c0 := math.FMA(xHiSq, 0x1.999999999999ap-3, -0x1.5555555555555p-2)
	// c1 ~ x_hi^2 * 1/9 - 1/7
	c1 := math.FMA(xHiSq, 0x1.c71c71c71c71cp-4, -0x1.2492492492492p-3)
	xHi3 := float64(xHiSq * x.hi)
	xHi4 := float64(xHiSq * xHiSq)
	// d0 ~ 1/3 - x_hi^2 / 5 + x_hi^4 / 7 - x_hi^6 / 9
	d0 := math.FMA(xHi4, c1, c0)
	// x_lo - x_lo * x_hi^2 + x_lo * x_hi^4
	d1 := math.FMA(xHi4-xHiSq, x.lo, x.lo)
	p.lo = math.FMA(xHi3, d0, d1)
	return p
}

func atanSignIndex(neg bool) int {
	if neg {
		return 1
	}
	return 0
}

// Atan is Math.atan.
func Atan(x float64) float64 {
	bits := math.Float64bits(x)
	xSign := atanSignIndex(bits>>63 != 0)
	xAbs := bits &^ (1 << 63)
	xExp := int(xAbs>>52) - 1023

	// |x| < 1.
	if xExp < 0 {
		if xExp < -26 {
			if x == 0.0 {
				return x
			}
			// |x| < 2^-26
			return math.FMA(-0x1.0p-54, x, x)
		}

		xD := math.Float64frombits(xAbs)
		// k = 2^-6 * round(2^6 * |x|)
		k := math.RoundToEven(0x1.0p6 * xD)
		idx := uint(k)
		k = float64(k * 0x1.0p-6)

		// numerator = |x| - k
		num := dd{0.0, xD - k}
		// denominator = 1 + k * |x|
		var den dd
		den.hi = math.FMA(xD, k, 1.0)
		prod := ddExactMult(xD, k)
		den.lo = ((1.0 - den.hi) + prod.hi) + prod.lo

		// x_r = (|x| - k) / (1 + k * |x|)
		xR := ddDiv(num, den)
		p := atanEval(xR)

		// atan(x) = sign(x) * (atan(k) + atan(x_r))
		c0 := ddExactAdd(crAtanI[idx].hi, p.hi)
		c1 := c0.lo + (crAtanI[idx].lo + p.lo)
		return atanIsNeg[xSign] * (c0.hi + c1)
	}

	// |x| >= 2^53 or x is NaN.
	if xExp >= 53 {
		if x != x {
			return x
		}
		// atan(x) ~ sign(x) * pi/2.
		return math.FMA(atanIsNeg[xSign], atanPiOver2.hi, atanIsNeg[xSign]*atanPiOver2.lo)
	}

	xD := math.Float64frombits(xAbs)
	y := 1.0 / xD

	// k = 2^-6 * round(2^6 / |x|)
	k := math.RoundToEven(0x1.0p6 * y)
	idx := uint(k)
	k = float64(k * 0x1.0p-6)

	// denominator = |x| + k
	den := ddExactAdd(xD, k)
	// numerator = 1 - k * |x|
	var num dd
	num.hi = math.FMA(-xD, k, 1.0)
	prod := ddExactMult(xD, k)
	num.lo = ((1.0 - num.hi) - prod.hi) - prod.lo

	// x_r = (1 - k * |x|) / (|x| + k)
	xR := ddDiv(num, den)
	p := atanEval(xR)

	// atan(x) = (-sign(x)) * (-pi/2 + atan(k) + atan(x_r))
	c0 := ddExactAdd(atanMPiOver2.hi, crAtanI[idx].hi)
	c1 := ddExactAdd(c0.hi, p.hi)
	c2 := c1.lo + (c0.lo + p.lo) + (crAtanI[idx].lo + atanMPiOver2.lo)
	return atanIsNeg[1-xSign] * (c1.hi + c2)
}

// Atan2 is Math.atan2.
func Atan2(y, x float64) float64 {
	xBits, yBits := math.Float64bits(x), math.Float64bits(y)
	xSign := atanSignIndex(xBits>>63 != 0)
	ySign := atanSignIndex(yBits>>63 != 0)
	xAbs := xBits &^ (1 << 63)
	yAbs := yBits &^ (1 << 63)
	recip := xAbs < yAbs
	minAbs, maxAbs := yAbs, xAbs
	if recip {
		minAbs, maxAbs = xAbs, yAbs
	}
	minExp := uint(minAbs >> 52)
	maxExp := uint(maxAbs >> 52)

	num := math.Float64frombits(minAbs)
	den := math.Float64frombits(maxAbs)

	// The exceptional cases: an argument is 0, infinite or NaN, or it is
	// near the overflow or the underflow limit.
	if maxExp > 0x7ff-128 || minExp < 128 {
		if x != x || y != y {
			return math.NaN()
		}
		except := func(v float64, abs uint64) int {
			switch {
			case v == 0.0:
				return 0
			case abs == 0x7ff0000000000000:
				return 2
			}
			return 1
		}
		xExcept := except(x, xAbs)
		yExcept := except(y, yAbs)
		if xExcept != 1 || yExcept != 1 {
			r := atan2Excepts[yExcept][xExcept][xSign]
			return math.FMA(atanIsNeg[ySign], r.hi, atanIsNeg[ySign]*r.lo)
		}
		scaleUp := minExp < 128
		scaleDown := maxExp > 0x7ff-128
		// Multiply both numbers by a power of 2 to make them normal.
		if scaleUp {
			num *= 0x1.0p64
			if !scaleDown {
				den *= 0x1.0p64
			}
		} else if scaleDown {
			den *= 0x1.0p-64
			if !scaleUp {
				num *= 0x1.0p-64
			}
		}

		minAbs = math.Float64bits(num)
		maxAbs = math.Float64bits(den)
		minExp = uint(minAbs >> 52)
		maxExp = uint(maxAbs >> 52)
	}

	finalSign := atanIsNeg[atanSignIndex((xSign != ySign) != recip)]
	constTerm := atan2ConstAdj[xSign][ySign][atanSignIndex(recip)]
	expDiff := maxExp - minExp
	// 2^(-exp_diff - 1) < n/d < 2^(-exp_diff + 1).
	if expDiff > 54 {
		return math.FMA(finalSign, constTerm.hi, finalSign*(constTerm.lo+num/den))
	}

	k := math.RoundToEven(64.0 * num / den)
	idx := uint(k)
	// k = idx / 64
	k = float64(k * 0x1.0p-6)

	// atan(n/d) - atan(k/64) = atan((n - d * k/64)) / (d + n * k/64))
	numK := ddExactMult(num, k)
	denK := ddExactMult(den, k)

	// num_dd = n - d * k
	numDD := ddExactAdd(num-denK.hi, -denK.lo)
	// den_dd = d + n * k
	denDD := ddExactAdd(den, numK.hi)
	denDD.lo += numK.lo

	// q = (n - d * k) / (d + n * k)
	q := ddDiv(numDD, denDD)
	// p ~ atan(q)
	p := atanEval(q)

	r := ddAdd(constTerm, ddAdd(crAtanI[idx], p))
	r.hi = float64(r.hi * finalSign)
	r.lo = float64(r.lo * finalSign)

	return r.hi + r.lo
}
