// Ported from V8 15.6.75, src/base/ieee754.cc: acosh, asinh, atanh, cosh and
// sinh (SunPro for the fdlibm code, and BSD-3-Clause for the changes of V8,
// ../LICENSE-V8.txt). These are the formulas of fdlibm on top of the exp,
// expm1, log and log1p of LLVM libc. They are not correctly rounded, so the
// port keeps each expression as it is. V8 builds without fused operations.
//
// The notice of src/base/ieee754.cc of V8:
//
//	The following is adapted from fdlibm (http://www.netlib.org/fdlibm).
//
//	====================================================
//	Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
//	Developed at SunSoft, a Sun Microsystems, Inc. business.
//	Permission to use, copy, modify, and distribute this
//	software is freely granted, provided that this notice
//	is preserved.
//	====================================================
//
//	The original source code covered by the above license above has been
//	modified significantly by Google Inc.
//	Copyright 2016 the V8 project authors. All rights reserved.
//
// This file is a Go translation of that code.

package crmath

import "math"

const hypLn2 = 6.93147180559945286227e-01 // 0x3FE62E42, 0xFEFA39EF

// hypHighWord is GET_HIGH_WORD: the more significant 32 bits, as an int32.
func hypHighWord(x float64) int32 { return int32(math.Float64bits(x) >> 32) }

// Acosh is Math.acosh.
//
//	acosh(x) = log(x)+ln2, if x is large; else
//	acosh(x) = log(2x-1/(sqrt(x*x-1)+x)) if x>2; else
//	acosh(x) = log1p(t+sqrt(2.0*t+t*t)); where t=x-1.
func Acosh(x float64) float64 {
	const one = 1.0
	hx := hypHighWord(x)
	lx := uint32(math.Float64bits(x))
	if hx < 0x3FF00000 { /* x < 1 */
		return math.NaN()
	} else if hx >= 0x41B00000 { /* x > 2**28 */
		if hx >= 0x7FF00000 { /* x is inf of NaN */
			return x + x
		}
		return Log(x) + hypLn2 /* acosh(huge)=log(2x) */
	} else if uint32(hx-0x3FF00000)|lx == 0 {
		return 0.0 /* acosh(1) = 0 */
	} else if hx > 0x40000000 { /* 2**28 > x > 2 */
		t := float64(x * x)
		return Log(float64(2.0*x) - one/(x+math.Sqrt(t-one)))
	}
	/* 1<x<2 */
	t := x - one
	return Log1p(t + math.Sqrt(float64(2.0*t)+float64(t*t)))
}

// Asinh is Math.asinh.
//
//	asinh(x) = x  if  1+x*x=1,
//	         = sign(x)*(log(x)+ln2)) for large |x|, else
//	         = sign(x)*log(2|x|+1/(|x|+sqrt(x*x+1))) if|x|>2, else
//	         = sign(x)*log1p(|x| + x^2/(1 + sqrt(1+x^2)))
func Asinh(x float64) float64 {
	const one = 1.0
	hx := hypHighWord(x)
	ix := hx & 0x7FFFFFFF
	if ix >= 0x7FF00000 {
		return x + x /* x is inf or NaN */
	}
	if ix < 0x3E300000 { /* |x|<2**-28 */
		return x /* return x inexact except 0 */
	}
	var w float64
	if ix > 0x41B00000 { /* |x| > 2**28 */
		w = Log(math.Abs(x)) + hypLn2
	} else if ix > 0x40000000 { /* 2**28 > |x| > 2.0 */
		t := math.Abs(x)
		w = Log(float64(2.0*t) + one/(math.Sqrt(float64(x*x)+one)+t))
	} else { /* 2.0 > |x| > 2**-28 */
		t := float64(x * x)
		w = Log1p(math.Abs(x) + t/(one+math.Sqrt(one+t)))
	}
	if hx > 0 {
		return w
	}
	return -w
}

// Atanh is Math.atanh.
//
//	For x>=0.5: atanh(x) = 0.5 * log1p(2 * x / (1 - x))
//	For x<0.5:  atanh(x) = 0.5*log1p(2x+2x*x/(1-x))
func Atanh(x float64) float64 {
	const one = 1.0
	bits := math.Float64bits(x)
	hx := int32(bits >> 32)
	lx := uint32(bits)
	ix := hx & 0x7FFFFFFF
	if uint32(ix)|((lx|-lx)>>31) > 0x3FF00000 {
		/* |x|>1 */
		return math.NaN()
	}
	if ix == 0x3FF00000 {
		if x > 0 {
			return math.Inf(1)
		}
		return math.Inf(-1)
	}
	if ix < 0x3E300000 {
		return x /* x<2**-28 */
	}
	x = math.Float64frombits(bits &^ (1 << 63)) /* SET_HIGH_WORD(x, ix) */
	var t float64
	if ix < 0x3FE00000 { /* x < 0.5 */
		t = x + x
		t = 0.5 * Log1p(t+float64(t*x)/(one-x))
	} else {
		t = 0.5 * Log1p((x+x)/(one-x))
	}
	if hx >= 0 {
		return t
	}
	return -t
}

// Cosh is Math.cosh.
//
//	0        <= x <= ln2/2  :  cosh(x) = 1 + [exp(x) - 1]^2 / (2*exp(x))
//	ln2/2    <= x <= 22     :  cosh(x) = (exp(x) + 1/exp(x)) / 2
//	22       <= x <= lnovft :  cosh(x) = exp(x)/2
//	lnovft   <= x <= ln2ovft:  cosh(x) = exp(x/2)/2 * exp(x/2)
//	ln2ovft  <  x           :  cosh(x) = huge*huge (overflow)
func Cosh(x float64) float64 {
	const (
		kCoshOverflow = 710.4758600739439
		one           = 1.0
		half          = 0.5
	)
	/* High word of |x|. */
	ix := hypHighWord(x) & 0x7FFFFFFF

	// |x| in [0,0.5*log2], return 1+expm1(|x|)^2/(2*exp(|x|))
	if ix < 0x3FD62E43 {
		t := Expm1(math.Abs(x))
		w := one + t
		// For |x| < 2^-55, cosh(x) = 1
		if ix < 0x3C800000 {
			return w
		}
		return one + float64(t*t)/(w+w)
	}

	// |x| in [0.5*log2, 22], return (exp(|x|)+1/exp(|x|)/2
	if ix < 0x40360000 {
		t := Exp(math.Abs(x))
		return float64(half*t) + half/t
	}

	// |x| in [22, log(maxdouble)], return half*exp(|x|)
	if ix < 0x40862E42 {
		return half * Exp(math.Abs(x))
	}

	// |x| in [log(maxdouble), overflowthreshold]
	if math.Abs(x) <= kCoshOverflow {
		w := Exp(half * math.Abs(x))
		t := half * w
		return t * w
	}

	/* x is INF or NaN */
	if ix >= 0x7FF00000 {
		return x * x
	}

	// |x| > overflowthreshold.
	return math.Inf(1)
}

// Sinh is Math.sinh.
//
//	0        <= x <= 22     :  sinh(x) = (E + E/(E+1)) / 2, E=expm1(x)
//	22       <= x <= lnovft :  sinh(x) = exp(x)/2
//	lnovft   <= x <= ln2ovft:  sinh(x) = exp(x/2)/2 * exp(x/2)
//	ln2ovft  <  x           :  sinh(x) = x*shuge (overflow)
func Sinh(x float64) float64 {
	const (
		kSinhOverflow = 710.4758600739439
		twoM28        = 3.725290298461914e-9 // 2^-28, empty lower half
		logMaxD       = 709.7822265625       // 0x40862E42 00000000, empty lower half
		shuge         = 1.0e307
	)
	h := 0.5
	if x < 0 {
		h = -0.5
	}
	// |x| in [0, 22]. return sign(x)*0.5*(E+E/(E+1))
	ax := math.Abs(x)
	if ax < 22 {
		// For |x| < 2^-28, sinh(x) = x
		if ax < twoM28 {
			return x
		}
		t := Expm1(ax)
		if ax < 1 {
			return h * (float64(2*t) - float64(t*t)/(t+1))
		}
		return h * (t + t/(t+1))
	}
	// |x| in [22, log(maxdouble)], return 0.5 * exp(|x|)
	if ax < logMaxD {
		return h * Exp(ax)
	}
	// |x| in [log(maxdouble), overflowthreshold]
	if ax <= kSinhOverflow {
		w := Exp(0.5 * ax)
		t := h * w
		return t * w
	}
	// |x| > overflowthreshold or is NaN.
	// Return Infinity of the appropriate sign or NaN.
	return x * shuge
}
