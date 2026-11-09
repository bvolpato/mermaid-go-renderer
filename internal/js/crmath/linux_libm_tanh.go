// Ported from s_tanh.c and s_expm1.c of fdlibm 5.3
// (https://www.netlib.org/fdlibm). The notices of the two files:
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunSoft, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// ====================================================
// Copyright (C) 2004 by Sun Microsystems, Inc. All rights reserved.
//
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// This file has the tanh of the C library of Linux. V8 calls std::tanh for
// Math.tanh, and Chromium on Linux has the tanh of glibc. The tanh and the
// expm1 of glibc 2.39 are those of fdlibm, so this port follows the fdlibm
// source and reads nothing of glibc. The function is not correctly rounded.
//
// The reference is glibc 2.39 on arm64, in the image
// mcr.microsoft.com/playwright:v1.64.1-noble. Two things differ from a
// plain build of fdlibm, and comparisons with the C library of the image
// selected both:
//
//   - The polynomial of expm1 is not in the Horner form of fdlibm. It is a
//     sum of three parts, with the powers 1, 2 and 4 of hxs.
//   - GCC joins a product and a sum into a fused operation (the default
//     -ffp-contract=fast). The port calls math.FMA at the places where GCC
//     14 fuses the fdlibm source with that polynomial at -O2. The results of
//     that build are the expm1 and the tanh of the image for 5 million
//     arguments. No other product is fused.
//
// tools/reference/check-jsmath-glibc.mjs compares linuxTanh with the tanh of
// the image, and the tanh of the image with Chromium on Linux.
//
// linuxExpm1 is not Math.expm1, which V8 takes from LLVM libc (exp.go).

package crmath

import "math"

// linuxTanh is tanh.
//
//  0. tanh(x) is defined to be (e^x - e^-x) / (e^x + e^-x)
//  1. reduce x to non-negative by tanh(-x) = -tanh(x).
//  2. 0      <= x <= 2**-55 : tanh(x) := x*(one+x)
//     2**-55 <  x <=  1     : tanh(x) := -t/(t+2); t = expm1(-2x)
//     1      <= x <=  22.0  : tanh(x) := 1 - 2/(t+2); t = expm1(2x)
//     22.0   <  x <= INF    : tanh(x) := 1.
func linuxTanh(x float64) float64 {
	const (
		one = 1.0
		two = 2.0
	)
	// High word of |x|.
	jx := int32(math.Float64bits(x) >> 32)
	ix := jx & 0x7fffffff

	// x is INF or NaN
	if ix >= 0x7ff00000 {
		if jx >= 0 {
			return one/x + one // tanh(+-inf)=+-1
		}
		return one/x - one // tanh(NaN) = NaN
	}

	var z float64
	if ix < 0x40360000 { // |x|<22
		if ix < 0x3c800000 { // |x|<2**-55
			return x * (one + x) // tanh(small) = small
		}
		if ix >= 0x3ff00000 { // |x|>=1
			t := linuxExpm1(two * math.Abs(x))
			z = one - two/(t+two)
		} else {
			t := linuxExpm1(-two * math.Abs(x))
			z = -t / (t + two)
		}
	} else {
		// |x| > 22, return +-1
		z = one // one - tiny rounds to one
	}
	if jx >= 0 {
		return z
	}
	return -z
}

// linuxExpm1 is expm1: exp(x) - 1.
//
//  1. Argument reduction: given x, find r and integer k such that
//     x = k*ln2 + r, |r| <= 0.5*ln2 ~ 0.34658. A correction term c is
//     computed to compensate the error in r when rounded to a
//     floating-point number.
//  2. Approximating expm1(r) by a special rational function on the interval
//     [0,0.34658]: r*(exp(r)+1)/(exp(r)-1) = 2 + r^2/6 * R1(r*r), and R1 is
//     a polynomial of degree 5 in r*r.
func linuxExpm1(x float64) float64 {
	const (
		one        = 1.0
		oThreshold = 7.09782712893383973096e+02 // 0x40862E42, 0xFEFA39EF
		ln2Hi      = 6.93147180369123816490e-01 // 0x3fe62e42, 0xfee00000
		ln2Lo      = 1.90821492927058770002e-10 // 0x3dea39ef, 0x35793c76
		invln2     = 1.44269504088896338700e+00 // 0x3ff71547, 0x652b82fe
		// scaled coefficients related to expm1
		q1 = -3.33333333333331316428e-02 // BFA11111 111110F4
		q2 = 1.58730158725481460165e-03  // 3F5A01A0 19FE5585
		q3 = -7.93650757867487942473e-05 // BF14CE19 9EAADBB7
		q4 = 4.00821782732936239552e-06  // 3ED0CFCA 86E65239
		q5 = -2.01099218183624371326e-07 // BE8AFDB7 6E09C32D
	)
	bits := math.Float64bits(x)
	hx := uint32(bits >> 32) // high word of x
	xsb := hx & 0x80000000   // sign bit of x
	hx &= 0x7fffffff         // high word of |x|

	// filter out huge and non-finite argument
	if hx >= 0x4043687A { // if |x|>=56*ln2
		if hx >= 0x40862E42 { // if |x|>=709.78...
			if hx >= 0x7ff00000 {
				if (hx&0xfffff)|uint32(bits) != 0 {
					return x + x // NaN
				}
				if xsb == 0 {
					return x // exp(+inf) = inf
				}
				return -1.0 // exp(-inf) - 1 = -1
			}
			if x > oThreshold {
				return math.Inf(1) // overflow
			}
		}
		if xsb != 0 { // x < -56*ln2, return -1.0 with inexact
			return -1.0 // tiny - one rounds to -1
		}
	}

	// argument reduction
	var c float64
	var k int32
	if hx > 0x3fd62e42 { // if  |x| > 0.5 ln2
		var hi, lo float64
		if hx < 0x3FF0A2B2 { // and |x| < 1.5 ln2
			if xsb == 0 {
				hi = x - ln2Hi
				lo = ln2Lo
				k = 1
			} else {
				hi = x + ln2Hi
				lo = -ln2Lo
				k = -1
			}
		} else {
			half := 0.5
			if xsb != 0 {
				half = -0.5
			}
			k = int32(float64(invln2*x) + half)
			t := float64(k)
			hi = math.FMA(-t, ln2Hi, x) // t*ln2_hi is exact here
			lo = float64(t * ln2Lo)
		}
		x = hi - lo
		c = (hi - x) - lo
	} else if hx < 0x3c900000 { // when |x|<2**-54, return x
		return x
	}

	// x is now in primary range
	hfx := 0.5 * x
	hxs := float64(x * hfx)
	// r1 = one+hxs*(Q1+hxs*(Q2+hxs*(Q3+hxs*(Q4+hxs*Q5)))), in three parts.
	p1 := math.FMA(hxs, q1, one)
	h2 := float64(hxs * hxs)
	p2 := math.FMA(hxs, q3, q2)
	h4 := float64(h2 * h2)
	p3 := math.FMA(hxs, q5, q4)
	r1 := math.FMA(h4, p3, math.FMA(h2, p2, p1))
	t := math.FMA(-hfx, r1, 3.0)
	e := float64(hxs * ((r1 - t) / math.FMA(-x, t, 6.0)))
	if k == 0 {
		return x - math.FMA(x, e, -hxs) // c is 0
	}
	e = math.FMA(e-c, x, -c)
	e -= hxs
	if k == -1 {
		return math.FMA(x-e, 0.5, -0.5)
	}
	if k == 1 {
		if x < -0.25 {
			return -2.0 * (e - (x + 0.5))
		}
		return math.FMA(x-e, 2.0, one)
	}
	// addK adds k to the exponent of y.
	addK := func(y float64) float64 {
		return math.Float64frombits(math.Float64bits(y) + uint64(int64(k))<<52)
	}
	if k <= -2 || k > 56 { // suffice to return exp(x)-1
		y := one - (e - x)
		return addK(y) - one
	}
	if k < 20 {
		t = math.Float64frombits(uint64(0x3ff00000-(0x200000>>uint(k))) << 32) // t=1-2^-k
		y := t - (e - x)
		return addK(y)
	}
	t = math.Float64frombits(uint64(0x3ff-k) << 52) // 2^-k
	y := x - (e + t)
	y += one
	return addK(y)
}
