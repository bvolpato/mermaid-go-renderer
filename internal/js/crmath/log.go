// Ported from LLVM libc at revision b8b28272d32b, the revision of Chromium
// 156.0.8078.4: libc/src/__support/math/log.h, log2.h, log10.h and log1p.h
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

const (
	// log(2) in two parts. The first has 43 bits, so that its product with
	// the exponent is exact.
	logLog2Hi = 0x1.62e42fefa38p-1
	logLog2Lo = 0x1.ef35793c7673p-45
	// A bound of the error of e_x log(2) - log(r).
	logHiErr = 0x1.0p-85
)

var (
	logLog2E  = dd{0x1.777d0ffda0d24p-56, 0x1.71547652b82fep0}  // 1/log(2)
	logLog10E = dd{0x1.95355baaafad3p-57, 0x1.bcb7b1526e50ep-2} // 1/log(10)
)

// logReduce is the first step of log, log2 and log10 for a positive finite
// x: the exponent e_x, the index of the table and u = r m - 1, where m is
// the mantissa of x and r is the reciprocal from the table.
func logReduce(x float64) (eX float64, index int, u float64) {
	xU := math.Float64bits(x)
	xE := -1023
	if xU < 0x0010000000000000 {
		// Normalize a subnormal input.
		xU = math.Float64bits(x * 0x1.0p52)
		xE -= 52
	}
	// log(x) = e_x log(2) - log(r) + log(1 + u)
	index = int(xU>>45) & 0x7F
	r := crLogRD[index]
	// Add the unbiased exponent. Add one more if the 8 leading bits of the
	// fraction are all 1.
	xE += int((xU + (1 << 45)) >> 52)
	xM := xU&0x000FFFFFFFFFFFFF | 0x3FF0000000000000
	m := math.Float64frombits(xM)
	return float64(xE), index, math.FMA(r, m, -1.0) // exact
}

// logSpecial handles the arguments of log, log2 and log10 that the fast
// pass does not: 1, 0, the negative numbers, the infinities and NaN.
func logSpecial(x float64) (r float64, ok bool) {
	xU := math.Float64bits(x)
	if xU == 0x3ff0000000000000 {
		return 0, true
	}
	if xU < 0x0010000000000000 || xU > 0x7fefffffffffffff {
		if x == 0.0 {
			return math.Inf(-1), true
		}
		if xU>>63 != 0 && x == x {
			return math.NaN(), true
		}
		if xU >= 0x7ff0000000000000 {
			return x, true // +Inf or NaN
		}
	}
	return 0, false
}

// logPoly is the polynomial of the log functions: a0 plus the terms of
// log(1 + u) - u.
func logPoly(u, uSq, a0 float64, c *[6]float64) float64 {
	p0 := math.FMA(u, c[1], c[0])
	p1 := math.FMA(u, c[3], c[2])
	p2 := math.FMA(u, c[5], c[4])
	return math.FMA(uSq, math.FMA(uSq, math.FMA(uSq, p2, p1), p0), a0)
}

// Log is Math.log, correctly rounded.
func Log(x float64) float64 {
	if r, ok := logFast(x); ok {
		return r
	}
	return logSlow(x)
}

// logFast is the fast pass of log. ok is false when the rounding test fails.
func logFast(x float64) (r float64, ok bool) {
	if r, ok := logSpecial(x); ok {
		return r, true
	}
	eX, index, u := logReduce(x)
	hi := math.FMA(eX, logLog2Hi, crLogRDD[index].hi)
	lo := math.FMA(eX, logLog2Lo, crLogRDD[index].lo)

	r1 := ddExactAdd(hi, u)
	uSq := float64(u * u)
	p := logPoly(u, uSq, lo+r1.lo, &crLogCoeffs)

	// The error of the polynomial, 2^-50 u^2, and of the high part.
	err := math.FMA(uSq, 0x1.0p-50, logHiErr)
	if hi == 0 && lo == 0 {
		// x is near 1: the exponent is 0 and the table entry is 0, so the
		// high part has no error. LLVM libc keeps the term and takes its
		// accurate pass. Without the term, the rounding test passes here.
		err = math.FMA(uSq, 0x1.0p-50, math.Abs(u)*0x1.0p-103)
	}
	left := r1.hi + (p - err)
	right := r1.hi + (p + err)
	return left, left == right
}

// Log2 is Math.log2, correctly rounded.
func Log2(x float64) float64 {
	if r, ok := log2Fast(x); ok {
		return r
	}
	return log2Slow(x)
}

// log2Fast is the fast pass of log2. ok is false when the rounding test
// fails.
func log2Fast(x float64) (r float64, ok bool) {
	if r, ok := logSpecial(x); ok {
		return r, true
	}
	eX, index, u := logReduce(x)

	r1 := ddExactAdd(crLog2R1[index].hi, u)
	uSq := float64(u * u)
	p := logPoly(u, uSq, crLog2R1[index].lo, &crLogCoeffs)
	r1.lo += p

	// log2(x) = e_x + log(x m) / log(2)
	r2 := ddQuickMult(r1, logLog2E)
	r3 := ddExactAdd(eX, r2.hi)
	r3.lo += r2.lo

	err := float64(uSq * 0x1.0p-49)
	left := r3.hi + (r3.lo - err)
	right := r3.hi + (r3.lo + err)
	return left, left == right
}

// Log10 is Math.log10, correctly rounded.
func Log10(x float64) float64 {
	if r, ok := log10Fast(x); ok {
		return r
	}
	return log10Slow(x)
}

// log10Fast is the fast pass of log10. ok is false when the rounding test
// fails.
func log10Fast(x float64) (r float64, ok bool) {
	if r, ok := logSpecial(x); ok {
		return r, true
	}
	eX, index, u := logReduce(x)
	hi := math.FMA(eX, logLog2Hi, crLogRDD[index].hi)
	lo := math.FMA(eX, logLog2Lo, crLogRDD[index].lo)

	uSq := float64(u * u)
	p := logPoly(u, uSq, lo, &crLogCoeffs)
	r1 := ddExactAdd(hi, u)
	r1.lo += p

	// log10(x) = log(x) / log(10)
	r2 := ddQuickMult(r1, logLog10E)

	err := math.FMA(uSq, 0x1.0p-51, logHiErr)
	if hi == 0 && lo == 0 {
		// x is near 1: the high part has no error, as in logFast. The second
		// term is a bound of the error of the product above.
		err = math.FMA(uSq, 0x1.0p-51, math.Abs(u)*0x1.0p-103)
	}
	left := r2.hi + (r2.lo - err)
	right := r2.hi + (r2.lo + err)
	return left, left == right
}

// Log1p is Math.log1p, correctly rounded.
func Log1p(x float64) float64 {
	if r, ok := log1pFast(x); ok {
		return r
	}
	return log1pSlow(x)
}

// log1pFast is the fast pass of log1p. ok is false when the rounding test
// fails.
func log1pFast(x float64) (r float64, ok bool) {
	xU := math.Float64bits(x)
	xExp := int(xU >> 52 & 0x7ff)
	var xDD dd

	if xExp >= 1023 {
		// |x| >= 1
		if xU >= 0x4650000000000000 {
			// x >= 2^102, or x is negative, infinite or NaN
			if xU > 0x7fefffffffffffff {
				switch {
				case xU == 0xbff0000000000000:
					return math.Inf(-1), true // x = -1
				case xU == 0x7ff0000000000000:
					return x, true // +Inf
				}
				return math.NaN(), true // x < -1 or NaN
			}
			xDD.hi = x
		} else {
			xDD = ddExactAdd(x, 1.0)
		}
	} else {
		// |x| < 1
		if xExp < 1023-52-1 {
			// |x| < 2^-53: log1p(x) rounds to x.
			return x, true
		}
		xDD = ddExactAdd(1.0, x)
	}

	// 1 + x = 2^(x_e) (m_dd.hi + m_dd.lo). The index is the rounded first 7
	// bits of the fraction.
	xhiBits := math.Float64bits(xDD.hi)
	xhiFrac := xhiBits & 0x000FFFFFFFFFFFFF
	xU = xhiBits
	idx := int((xhiFrac + (1 << (52 - 8))) >> (52 - 7))
	xE := int(xhiBits>>52&0x7ff) - 1023 + (idx >> 7)
	eX := float64(xE)

	hi := math.FMA(eX, logLog2Hi, crLog1pLogR1[idx].hi)
	lo := math.FMA(eX, logLog2Lo, crLog1pLogR1[idx].lo)
	errHi := 0x1.0p-85
	if hi == 0.0 {
		errHi = 0
	}

	// Scale x_dd by 2^(-xh_bits.get_exponent()).
	sU := int64(xU&0x7ff0000000000000) - (int64(1023) << 52)
	mHi := 0x3ff0000000000000 | xhiFrac
	mLo := uint64(0)
	if math.Abs(xDD.lo) > xDD.hi*0x1.0p-127 {
		mLo = uint64(int64(math.Float64bits(xDD.lo)) - sU)
	}
	mDD := dd{math.Float64frombits(mLo), math.Float64frombits(mHi)}

	// v = r m - 1 in two parts.
	rr := crLog1pR1[idx]
	vLo := ddExactMult(mDD.lo, rr)
	vHi := math.FMA(rr, mDD.hi, -1.0) // Exact
	vDD := ddExactAdd(vHi, vLo.hi)
	vDD.lo += vLo.lo

	r1 := ddExactAdd(hi, vDD.hi)
	vSq := float64(vDD.hi * vDD.hi)
	p := logPoly(vDD.hi, vSq, (vDD.lo+r1.lo)+lo, &crLog1pCoeffs)

	err := math.FMA(vSq, 0x1.0p-49, errHi)
	left := r1.hi + (p - err)
	right := r1.hi + (p + err)
	return left, left == right
}
