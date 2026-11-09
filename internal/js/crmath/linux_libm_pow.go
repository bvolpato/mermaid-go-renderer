// Ported from math/pow.c and math/math_err.c of Arm Optimized Routines
// v23.01 (https://github.com/ARM-software/optimized-routines). The tables
// are in linux_libm_pow_tables.go.
//
// Copyright (c) 2018-2020, Arm Limited.
// SPDX-License-Identifier: MIT OR Apache-2.0 WITH LLVM-exception
//
// The MIT license of Arm Optimized Routines:
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// This file has the pow of the C library of Linux. V8 calls std::pow for
// Math.pow, and Chromium on Linux has the pow of glibc. glibc 2.28 and later
// has the pow of Arm Optimized Routines, so this port follows the Arm source
// and reads nothing of glibc. The function is not correctly rounded: the
// error is at most 0.54 units in the last place.
//
// The reference is glibc 2.39 on arm64, in the image
// mcr.microsoft.com/playwright:v1.64.1-noble. That build has fused
// operations at two kinds of places: where the source calls fma (the macro
// HAVE_FAST_FMA), and where GCC joins a product and a sum (the default
// -ffp-contract=fast). The port calls math.FMA at the places where GCC 14
// fuses the Arm source at -O2. The result of that build is the pow of the
// image for 20 million arguments. No other product is fused. The build also
// has the rounding of the macro TOINT_INTRINSICS: a tie goes away from zero.
//
// tools/reference/check-jsmath-glibc.mjs compares linuxPow with the pow of
// the image, and the pow of the image with Chromium on Linux.

package crmath

import "math"

const (
	// linuxPowOff is OFF: the mantissa range of log_inline starts at
	// 0x1.69555p-1.
	linuxPowOff = 0x3fe6955500000000
	// linuxPowSignBias is SIGN_BIAS: the sign of the result, in the place of
	// the exponent of the scale of exp_inline.
	linuxPowSignBias = 0x800 << 7
	linuxPowOne      = 0x3ff0000000000000
	linuxPowInf      = 0x7ff0000000000000
)

// linuxPowTop12 is top12: the top 12 bits of a float64 (the sign and the
// exponent bits).
func linuxPowTop12(x float64) uint32 { return uint32(math.Float64bits(x) >> 52) }

// linuxPowLogInline is log_inline. It computes y+tail = log(x) where the
// rounded result is y and tail has about additional 15 bits precision. ix is
// the bit representation of x, but normalized in the subnormal range using
// the sign bit for the exponent.
func linuxPowLogInline(ix uint64) (y, tail float64) {
	// x = 2^k z; where z is in range [OFF,2*OFF) and exact.
	// The range is split into N subintervals.
	// The ith subinterval contains z and c is near its center.
	tmp := ix - linuxPowOff
	i := (tmp >> (52 - 7)) % 128
	k := int64(tmp) >> 52 // arithmetic shift
	iz := ix - (tmp & (0xfff << 52))
	z := math.Float64frombits(iz)
	kd := float64(k)

	// log(x) = k*Ln2 + log(c) + log1p(z/c-1).
	invc := linuxPowLogTab[i].invc
	logc := linuxPowLogTab[i].logc
	logctail := linuxPowLogTab[i].logctail

	// Note: 1/c is j/N or j/N/2 where j is an integer in [N,2N) and
	// |z/c - 1| < 1/N, so r = z/c - 1 is exactly representible.
	r := math.FMA(z, invc, -1.0)

	// k*Ln2 + log(c) + r.
	t1 := math.FMA(kd, linuxPowLn2hi, logc)
	t2 := t1 + r
	lo1 := math.FMA(kd, linuxPowLn2lo, logctail)
	lo2 := t1 - t2 + r

	// Evaluation is optimized assuming superscalar pipelined execution.
	a := &linuxPowLogPoly
	ar := float64(a[0] * r) // A[0] = -0.5.
	ar2 := float64(r * ar)
	ar3 := float64(r * ar2)
	// k*Ln2 + log(c) + r + A[0]*r*r.
	hi := t2 + ar2
	lo3 := math.FMA(ar, r, -ar2)
	lo4 := t2 - hi + ar2
	// p = log1p(r) - r - A[0]*r*r. The product of ar3 and the polynomial is
	// fused with the sum of the low parts.
	p := math.FMA(ar2, math.FMA(ar2, math.FMA(r, a[6], a[5]), math.FMA(r, a[4], a[3])), math.FMA(r, a[2], a[1]))
	lo := math.FMA(ar3, p, lo1+lo2+lo3+lo4)
	y = hi + lo
	tail = hi - y + lo
	return y, tail
}

// linuxPowSpecialcase is specialcase. It handles cases that may overflow or
// underflow when computing the result that is scale*(1+tmp) without
// intermediate rounding. The bit representation of scale is in sbits,
// however it has a computed exponent that may have overflown into the sign
// bit so that needs to be adjusted before using it as a double. int32(ki) is
// the k used in the argument reduction and exponent adjustment of scale,
// positive k here means the result may overflow and negative k means the
// result may underflow.
func linuxPowSpecialcase(tmp float64, sbits, ki uint64) float64 {
	if ki&0x80000000 == 0 {
		// k > 0, the exponent of scale might have overflowed by <= 460.
		sbits -= 1009 << 52
		scale := math.Float64frombits(sbits)
		return 0x1p1009 * math.FMA(scale, tmp, scale)
	}
	// k < 0, need special care in the subnormal range.
	sbits += 1022 << 52
	// Note: sbits is signed scale.
	scale := math.Float64frombits(sbits)
	// GCC does not fuse this product: the branch below uses it again.
	st := float64(scale * tmp)
	y := scale + st
	if math.Abs(y) < 1.0 {
		// Round y to the right precision before scaling it into the subnormal
		// range to avoid double rounding that can cause 0.5+E/2 ulp error where
		// E is the worst-case ulp error outside the subnormal range. So this
		// is only useful if the goal is better than 1 ulp worst-case error.
		one := 1.0
		if y < 0.0 {
			one = -1.0
		}
		lo := scale - y + st
		hi := one + y
		lo = one - hi + y + lo
		y = (hi + lo) - one
		// Fix the sign of 0.
		if y == 0.0 {
			y = math.Float64frombits(sbits & 0x8000000000000000)
		}
	}
	return 0x1p-1022 * y
}

// linuxPowExpInline is exp_inline. It computes sign*exp(x+xtail) where
// |xtail| < 2^-8/N and |xtail| <= |x|. The signBias argument is
// linuxPowSignBias or 0 and sets the sign to -1 or 1.
func linuxPowExpInline(x, xtail float64, signBias uint32) float64 {
	const (
		top12Tiny = 0x3c9 // top12(0x1p-54)
		top12Big  = 0x408 // top12(512.0)
		top12Huge = 0x409 // top12(1024.0)
	)
	abstop := linuxPowTop12(x) & 0x7ff
	if abstop-top12Tiny >= top12Big-top12Tiny {
		if abstop-top12Tiny >= 0x80000000 {
			// Avoid spurious underflow for tiny x.
			// Note: 0 is common input.
			one := 1.0 + x
			if signBias != 0 {
				return -one
			}
			return one
		}
		if abstop >= top12Huge {
			// Note: inf and nan are already handled.
			if math.Float64bits(x)>>63 != 0 {
				return linuxPowUflow(signBias)
			}
			return linuxPowOflow(signBias)
		}
		// Large x is special cased below.
		abstop = 0
	}

	// exp(x) = 2^(k/N) * exp(r), with exp(r) in [2^(-1/2N),2^(1/2N)].
	// x = ln2/N*k + r, with int k and r in [-ln2/2N, ln2/2N].
	z := linuxExpInvLn2N * x
	// roundtoint and converttoint of TOINT_INTRINSICS: round and lround.
	kd := math.Round(z)
	ki := uint64(int64(kd))
	r := math.FMA(kd, linuxExpNegLn2loN, math.FMA(kd, linuxExpNegLn2hiN, x))
	// The code assumes 2^-200 < |xtail| < 2^-8/N.
	r += xtail
	// 2^(k/N) ~= scale * (1 + tail).
	idx := 2 * (ki % 128)
	top := (ki + uint64(signBias)) << (52 - 7)
	tail := math.Float64frombits(linuxExpTab[idx])
	// This is only a valid scale when -1023*N < k < 1024*N.
	sbits := linuxExpTab[idx+1] + top
	// exp(x) = 2^(k/N) * exp(r) ~= scale + scale * (tail + exp(r) - 1).
	// Evaluation is optimized assuming superscalar pipelined execution.
	r2 := float64(r * r)
	// Without fma the worst case error is 0.25/N ulp larger.
	// Worst case error is less than 0.5+1.11/N+(abs poly error * 2^53) ulp.
	// tmp = tail + r + r2 * (C2 + r * C3) + r2 * r2 * (C4 + r * C5)
	c := &linuxExpPoly
	tmp := math.FMA(r2, math.FMA(r, c[1], c[0]), tail+r)
	tmp = math.FMA(float64(r2*r2), math.FMA(r, c[3], c[2]), tmp)
	if abstop == 0 {
		return linuxPowSpecialcase(tmp, sbits, ki)
	}
	scale := math.Float64frombits(sbits)
	// Note: tmp == 0 or |tmp| > 2^-200 and scale > 2^-739, so there
	// is no spurious underflow here even without fma.
	return math.FMA(scale, tmp, scale)
}

// linuxPowCheckint is checkint. It returns 0 if not int, 1 if odd int, 2 if
// even int. The argument is the bit representation of a non-zero finite
// floating-point value.
func linuxPowCheckint(iy uint64) int {
	e := int(iy >> 52 & 0x7ff)
	if e < 0x3ff {
		return 0
	}
	if e > 0x3ff+52 {
		return 2
	}
	if iy&(1<<uint(0x3ff+52-e)-1) != 0 {
		return 0
	}
	if iy&(1<<uint(0x3ff+52-e)) != 0 {
		return 1
	}
	return 2
}

// linuxPowZeroInfNaN is zeroinfnan. It returns true if the input is the bit
// representation of 0, infinity or nan.
func linuxPowZeroInfNaN(i uint64) bool {
	return 2*i-1 >= 2*linuxPowInf-1
}

// linuxPowIsSignaling is issignaling_inline: a NaN without the quiet bit.
func linuxPowIsSignaling(ix uint64) bool {
	return 2*(ix^0x0008000000000000) > 2*0x7ff8000000000000
}

// linuxPowOflow is __math_oflow: the overflow result, an infinity.
func linuxPowOflow(sign uint32) float64 {
	if sign != 0 {
		return math.Inf(-1)
	}
	return math.Inf(1)
}

// linuxPowUflow is __math_uflow: the underflow result, a zero.
func linuxPowUflow(sign uint32) float64 {
	if sign != 0 {
		return math.Copysign(0, -1)
	}
	return 0
}

// linuxPow is pow.
func linuxPow(x, y float64) float64 {
	signBias := uint32(0)
	ix := math.Float64bits(x)
	iy := math.Float64bits(y)
	topx := linuxPowTop12(x)
	topy := linuxPowTop12(y)
	if topx-0x001 >= 0x7ff-0x001 || (topy&0x7ff)-0x3be >= 0x43e-0x3be {
		// Note: if |y| > 1075 * ln2 * 2^53 ~= 0x1.749p62 then pow(x,y) = inf/0
		// and if |y| < 2^-54 / 1075 ~= 0x1.e7b6p-65 then pow(x,y) = +-1.
		// Special cases: (x < 0x1p-126 or inf or nan) or
		// (|y| < 0x1p-65 or |y| >= 0x1p63 or nan).
		if linuxPowZeroInfNaN(iy) {
			if 2*iy == 0 {
				if linuxPowIsSignaling(ix) {
					return x + y
				}
				return 1.0
			}
			if ix == linuxPowOne {
				if linuxPowIsSignaling(iy) {
					return x + y
				}
				return 1.0
			}
			if 2*ix > 2*linuxPowInf || 2*iy > 2*linuxPowInf {
				return x + y
			}
			if 2*ix == 2*linuxPowOne {
				return 1.0
			}
			if (2*ix < 2*linuxPowOne) == (iy>>63 == 0) {
				return 0.0 // |x|<1 && y==inf or |x|>1 && y==-inf.
			}
			return y * y
		}
		if linuxPowZeroInfNaN(ix) {
			x2 := x * x
			if ix>>63 != 0 && linuxPowCheckint(iy) == 1 {
				x2 = -x2
			}
			if iy>>63 != 0 {
				return 1 / x2
			}
			return x2
		}
		// Here x and y are non-zero finite.
		if ix>>63 != 0 {
			// Finite x < 0.
			yint := linuxPowCheckint(iy)
			if yint == 0 {
				return math.NaN() // __math_invalid
			}
			if yint == 1 {
				signBias = linuxPowSignBias
			}
			ix &= 0x7fffffffffffffff
			topx &= 0x7ff
		}
		if (topy&0x7ff)-0x3be >= 0x43e-0x3be {
			// Note: sign_bias == 0 here because y is not odd.
			if ix == linuxPowOne {
				return 1.0
			}
			if topy&0x7ff < 0x3be {
				// |y| < 2^-65, x^y ~= 1 + y*log(x).
				if ix > linuxPowOne {
					return 1.0 + y
				}
				return 1.0 - y
			}
			if (ix > linuxPowOne) == (topy < 0x800) {
				return linuxPowOflow(0)
			}
			return linuxPowUflow(0)
		}
		if topx == 0 {
			// Normalize subnormal x so exponent becomes negative.
			ix = math.Float64bits(x * 0x1p52)
			ix &= 0x7fffffffffffffff
			ix -= 52 << 52
		}
	}

	hi, lo := linuxPowLogInline(ix)
	ehi := float64(y * hi)
	elo := math.FMA(y, lo, math.FMA(y, hi, -ehi))
	return linuxPowExpInline(ehi, elo, signBias)
}
