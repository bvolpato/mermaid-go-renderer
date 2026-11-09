// Ported from LLVM libc at revision b8b28272d32b, the revision of Chromium
// 156.0.8078.4: libc/src/__support/math/exp.h, expm1.h and exp_utils.h
// (Apache-2.0 WITH LLVM-exception). The port has the pass with one float64
// and the pass with two float64 numbers of each function. The accurate pass
// is in big.go.
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
	expLog2E = 0x1.71547652b82fep+0
	// expErrD and expErrDD are the error bounds of the two passes of exp.
	expErrD  = 0x1.8p-63
	expErrDD = 0x1.0p-99
	// -2^-12 log(2) in parts: the first has 28 bits and the middle 30 bits.
	expMLog2HI    = -0x1.62e42ffp-13
	expMLog2Mid   = 0x1.718432a1b0e26p-47
	expMLog2Mid30 = 0x1.718432ap-47
	expMLog2Lo    = 0x1.b0e2633fe0685p-79
	// expm1ErrD and expm1ErrDD are the bits of the error bounds of expm1.
	expm1ErrD  = 0x3c08000000000000
	expm1ErrDD = 0x39c0000000000000
)

// expPolyD is poly_approx_d: (exp(dx) - 1) / dx for |dx| < 2^-13.
func expPolyD(dx float64) float64 {
	dx2 := float64(dx * dx)
	c0 := math.FMA(dx, 0.5, 1.0)
	c1 := math.FMA(dx, 0x1.5555555555555p-5, 0x1.5555555555555p-3)
	return math.FMA(dx2, c1, c0)
}

// expPolyDD is poly_approx_dd of exp.h: exp(dx) with the Taylor polynomial
// of degree 6.
func expPolyDD(dx dd) dd {
	p := dd{-0x1.f49f49f49f49fp-65, 0x1.6c16c16c16c17p-10}               // 1/720
	p = ddMulAdd(dx, p, dd{0x1.1111111111111p-63, 0x1.1111111111111p-7}) // 1/120
	p = ddMulAdd(dx, p, dd{0x1.5555555555555p-59, 0x1.5555555555555p-5}) // 1/24
	p = ddMulAdd(dx, p, dd{0x1.5555555555555p-57, 0x1.5555555555555p-3}) // 1/6
	p = ddMulAdd(dx, p, dd{0, 0x1p-1})
	p = ddMulAdd(dx, p, dd{0, 0x1p0})
	return ddMulAdd(dx, p, dd{0, 0x1p0})
}

// expm1PolyDD is poly_approx_dd of expm1.h: (exp(dx) - 1) / dx with the
// Taylor polynomial of degree 6.
func expm1PolyDD(dx dd) dd {
	p := dd{0x1.a01a01a01a01ap-73, 0x1.a01a01a01a01ap-13}                  // 1/5040
	p = ddMulAdd(dx, p, dd{-0x1.f49f49f49f49fp-65, 0x1.6c16c16c16c17p-10}) // 1/720
	p = ddMulAdd(dx, p, dd{0x1.1111111111111p-63, 0x1.1111111111111p-7})   // 1/120
	p = ddMulAdd(dx, p, dd{0x1.5555555555555p-59, 0x1.5555555555555p-5})   // 1/24
	p = ddMulAdd(dx, p, dd{0x1.5555555555555p-57, 0x1.5555555555555p-3})   // 1/6
	p = ddMulAdd(dx, p, dd{0, 0x1p-1})
	return ddMulAdd(dx, p, dd{0, 0x1p0})
}

// expReducedDD is the reduced argument x - k 2^-12 log(2) in two parts.
func expReducedDD(x, kd float64) dd {
	t1 := math.FMA(kd, expMLog2HI, x) // exact
	t2 := float64(kd * expMLog2Mid30) // exact
	t3 := float64(kd * expMLog2Lo)    // Error < 2^-130
	dx := ddExactAdd(t1, t2)
	dx.lo += t3
	return dx
}

// expZivTestDenorm is ziv_test_denorm: the rounding test for a result that
// is subnormal. The result is mid + lo, scaled by 2^hi.
func expZivTestDenorm(hi int, mid, lo, err float64) (r float64, ok bool) {
	// Scale the number so that the result is in [1, 2) or in [0.5, 1): the
	// rounding to the bits of a subnormal number is then an addition.
	expHi := int64(hi+1022) * (1 << 52)
	midHi := math.Float64frombits(uint64(expHi + int64(math.Float64bits(mid))))
	loScaled := 0.0
	if lo != 0.0 {
		loScaled = math.Float64frombits(uint64(expHi + int64(math.Float64bits(lo))))
	}
	extraFactor := 0.0
	scaleDown := uint64(0x3FE0000000000000) // 1022 in the exponent field.
	// The result is subnormal if (mid_hi + lo_scaled) < 1.
	if (1.0 - midHi) > loScaled {
		err += 0x1.0p-52
		extraFactor = 1.0
		scaleDown = 0x3FF0000000000000 // 1023 in the exponent field.
	}
	errScaled := math.Float64frombits(uint64(expHi + int64(math.Float64bits(err))))
	loU := loScaled + errScaled
	loL := loScaled - errScaled
	upper := extraFactor + (midHi + loU)
	lower := extraFactor + (midHi + loL)
	if upper == lower {
		return math.Float64frombits(math.Float64bits(upper) - scaleDown), true
	}
	return 0, false
}

// expScale multiplies r by 2^hi: it adds hi to the exponent field.
func expScale(hi int, r float64) float64 {
	return math.Float64frombits(uint64(int64(hi)*(1<<52) + int64(math.Float64bits(r))))
}

// Exp is Math.exp, correctly rounded.
func Exp(x float64) float64 {
	if r, ok := expFast(x); ok {
		return r
	}
	return expSlow(x)
}

// expFast has the two fast passes of exp. ok is false when both rounding
// tests fail.
func expFast(x float64) (r float64, ok bool) {
	xU := math.Float64bits(x)
	if xU >= 0xc0874910d52d3052 || (xU < 0xbca0000000000000 && xU >= 0x40862e42fefa39f0) || xU < 0x3ca0000000000000 {
		// set_exceptional
		switch {
		case xU&^(1<<63) <= 0x3ca0000000000000:
			// |x| < 2^-53
			return 1 + x, true
		case xU >= 0xc0874910d52d3052:
			// x < log(2^-1075), or x is NaN
			if x != x {
				return x, true
			}
			return 0, true
		}
		// x >= log(2^1024), or x is NaN
		return x + math.Inf(1), true
	}

	// x = log(2) (hi + mid1 + mid2) + lo, with an integer hi, a mid1 that is
	// a multiple of 2^-6 and a mid2 that is a multiple of 2^-12. Then
	// exp(x) = 2^hi 2^mid1 2^mid2 exp(lo). k is round(x 2^12 log2(e)).
	tmp := math.FMA(x, expLog2E, 0x1.800000004p21)
	k := int32(math.Float64bits(tmp) >> 19)
	kd := float64(k)

	idx1 := (k >> 6) & 0x3f
	idx2 := k & 0x3f
	hi := int(k >> 12)
	denorm := hi <= -1022

	expMid1 := dd{crExp2Mid1[idx1][1], crExp2Mid1[idx1][2]}
	expMid2 := dd{crExp2Mid2[idx2][1], crExp2Mid2[idx2][2]}
	expMid := ddQuickMult(expMid1, expMid2)

	loH := math.FMA(kd, expMLog2HI, x) // exact
	dx := math.FMA(kd, expMLog2Mid, loH)

	// (exp_mid.hi + exp_mid.lo) (1 + dx P(dx)) ~
	//   exp_mid.hi + (exp_mid.hi dx P(dx) + exp_mid.lo)
	midLo := float64(dx * expMid.hi)
	p := expPolyD(dx)
	lo := math.FMA(p, midLo, expMid.lo)

	if denorm {
		if r, ok := expZivTestDenorm(hi, expMid.hi, lo, expErrD); ok {
			return r, true
		}
	} else {
		upper := expMid.hi + (lo + expErrD)
		lower := expMid.hi + (lo - expErrD)
		if upper == lower {
			return expScale(hi, upper), true
		}
	}

	// The pass with two float64 numbers.
	rDD := ddQuickMult(expMid, expPolyDD(expReducedDD(x, kd)))
	if denorm {
		return expZivTestDenorm(hi, rDD.hi, rDD.lo, expErrDD)
	}
	upperDD := rDD.hi + (rDD.lo + expErrDD)
	lowerDD := rDD.hi + (rDD.lo - expErrDD)
	if upperDD == lowerDD {
		return expScale(hi, upperDD), true
	}
	return 0, false
}

// Expm1 is Math.expm1, correctly rounded.
func Expm1(x float64) float64 {
	if r, ok := expm1Fast(x); ok {
		return r
	}
	return expm1Slow(x)
}

// expm1Fast has the two fast passes of expm1. ok is false when both
// rounding tests fail.
func expm1Fast(x float64) (r float64, ok bool) {
	xU := math.Float64bits(x)
	xIsNeg := xU>>63 != 0
	// x < log(2^-54) or x >= log(2^1024) or |x| <= 2^-53.
	if xU >= 0xc042b708872320e2 || (xU <= 0xbca0000000000000 && xU >= 0x40862e42fefa39f0) || xU <= 0x3ca0000000000000 {
		// set_exceptional
		xAbs := xU &^ (1 << 63)
		switch {
		case xAbs <= 0x3ca0000000000000:
			// |x| <= 2^-53
			if xAbs <= 0x0370000000000000 {
				if xAbs == 0 {
					return x, true
				}
				// |x| <= 2^-968
				return 0x1.0p-200 * math.FMA(x, 0x1.0p+200, 0x1.0p-1022), true
			}
			return x, true
		case xU >= 0xc042b708872320e2:
			// x < log(2^-54), or x is NaN
			if x != x {
				return x, true
			}
			return -1, true
		}
		return x + math.Inf(1), true
	}

	if xU&^(1<<63) < 0x3e60000000000000 {
		// 2^-53 < |x| < 2^-25. This step is not in LLVM libc, whose two
		// passes have absolute error bounds and fail for such a small
		// result. expm1(x) = x + x^2/2 + x^3/6 + ..., and the terms from
		// x^4/24 are below 2^-79 |x|. The rounding errors of the low part
		// are below 2^-77 |x|.
		lo := float64(float64(x*x) * math.FMA(x, 0x1.5555555555555p-3, 0.5))
		err := float64(math.Abs(x) * 0x1.0p-75)
		upper := x + (lo + err)
		lower := x + (lo - err)
		return upper, upper == lower
	}

	tmp := math.FMA(x, expLog2E, 0x1.800000004p21)
	k := int32(math.Float64bits(tmp) >> 19)
	kd := float64(k)

	idx1 := (k >> 6) & 0x3f
	idx2 := k & 0x3f
	hi := int(k >> 12)

	expMid1 := dd{crExp2Mid1[idx1][1], crExp2Mid1[idx1][2]}
	expMid2 := dd{crExp2Mid2[idx2][1], crExp2Mid2[idx2][2]}
	expMid := ddQuickMult(expMid1, expMid2)

	// -2^(-hi)
	oneScaled := math.Float64frombits(1<<63 | uint64(1023-hi)<<52)

	// 2^(mid1 + mid2) - 2^(-hi)
	var hiPart dd
	if xIsNeg {
		hiPart = ddExactAdd(oneScaled, expMid.hi)
	} else {
		hiPart = ddExactAdd(expMid.hi, oneScaled)
	}
	hiPart.lo += expMid.lo

	loH := math.FMA(kd, expMLog2HI, x) // exact
	dx := math.FMA(kd, expMLog2Mid, loH)

	// expm1(x) ~ 2^hi ((exp_mid.hi - 2^-hi) + (exp_mid.hi dx P(dx) + exp_mid.lo))
	midLo := float64(dx * expMid.hi)
	p := expPolyD(dx)
	lo := math.FMA(p, midLo, hiPart.lo)

	errBits := uint64(0)
	if xIsNeg {
		errBits = uint64(-hi) << 52
	}
	errD := math.Float64frombits(expm1ErrD + errBits)

	upper := hiPart.hi + (lo + errD)
	lower := hiPart.hi + (lo - errD)
	if upper == lower {
		return expScale(hi, upper), true
	}

	// The pass with two float64 numbers.
	dxDD := expReducedDD(x, kd)
	rDD := ddMulAdd(ddQuickMult(expMid, dxDD), expm1PolyDD(dxDD), hiPart)

	errDD := math.Float64frombits(expm1ErrDD + errBits)
	upperDD := rDD.hi + (rDD.lo + errDD)
	lowerDD := rDD.hi + (rDD.lo - errDD)
	if upperDD == lowerDD {
		return expScale(hi, upperDD), true
	}
	return 0, false
}
