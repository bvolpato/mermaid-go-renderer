// Ported from LLVM libc at revision b8b28272d32b, the revision of Chromium
// 156.0.8078.4: libc/src/__support/FPUtil/double_double.h (Apache-2.0 WITH
// LLVM-exception). The file of LLVM has no copyright line. It says:
//
//	Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
//	See https://llvm.org/LICENSE.txt for license information.
//	SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
// This file is a Go translation of that file. It is a changed version of
// it. LICENSE-LLVM.txt in this directory has the license. The package
// comment that follows ports no upstream file.

// Package crmath has the Math functions of the reference browser, Chromium
// 156 with V8 15.6 on arm64. Each function returns the same bits as the
// browser. The package does not depend on other packages of the port.
//
// The reference browser has two platforms, Linux and macOS (platform.go).
// They differ for pow and tanh only.
//
// V8 takes the functions from three places (src/base/ieee754.cc,
// src/numbers/ieee754.cc and src/builtins/math.tq of V8 15.6.75):
//
//   - LLVM libc: sin, cos, tan, asin, acos, atan, atan2, exp, expm1, log,
//     log1p, log2, log10 and cbrt. All of them but atan and atan2 are
//     correctly rounded: the result is the float64 that is nearest to the
//     exact value. For these functions the package ports the first steps of
//     the LLVM function, which compute with two float64 numbers and test if
//     the error bound allows the rounding. When the test fails, a computation
//     with math/big gives the result (big.go). atan and atan2 are not
//     correctly rounded in LLVM libc, so their ports are complete and exact.
//   - Formulas of fdlibm on top of those functions: sinh, cosh, asinh, acosh
//     and atanh (hyp.go), and the builtins of math.tq (misc.go).
//   - The C library of the platform: pow and tanh (pow.go, tanh.go). On
//     Linux that is glibc, with the pow of Arm Optimized Routines and the
//     tanh of fdlibm (linux_libm_*.go). On macOS that is libsystem_m of
//     Apple, which is closed source. The module has no port of it, and the
//     platform macOS returns the results of Linux (platform.go).
//
// LLVM libc has fused multiply-add operations on arm64 (multiply_add is
// fma), and the ports call math.FMA at the same places. No other product is
// fused: a product that an addition or a subtraction uses has a conversion,
// which the Go specification defines as a rounding step. So the results do
// not depend on the CPU.
//
// The golden tests against the browser are in golden_test.go (macOS) and
// linux_libm_test.go (Linux).
//
// This file has the arithmetic with numbers in two float64 parts. It is
// ported from libc/src/__support/FPUtil/double_double.h of LLVM libc.
package crmath

import "math"

// dd is a number in two parts: the sum hi + lo, with a small lo. The fields
// have the order of NumberPair of LLVM libc, so that the tables keep their
// form.
type dd struct{ lo, hi float64 }

// ddExactAdd is exact_add (Fast2Sum). The sum is exact when the exponent of
// a is not below that of b.
func ddExactAdd(a, b float64) dd {
	hi := a + b
	t := hi - a
	return dd{b - t, hi}
}

// ddExactAdd2 is exact_add<false> (2Sum). The sum is exact for all a and b.
func ddExactAdd2(a, b float64) dd {
	hi := a + b
	t1 := hi - a
	t2 := hi - t1
	t3 := b - t1
	t4 := a - t2
	return dd{t3 + t4, hi}
}

// ddExactMult is exact_mult with a fused operation.
func ddExactMult(a, b float64) dd {
	hi := float64(a * b)
	return dd{math.FMA(a, b, -hi), hi}
}

// ddQuickMultD is quick_mult(double, DoubleDouble).
func ddQuickMultD(a float64, b dd) dd {
	r := ddExactMult(a, b.hi)
	r.lo = math.FMA(a, b.lo, r.lo)
	return r
}

// ddQuickMult is quick_mult(DoubleDouble, DoubleDouble).
func ddQuickMult(a, b dd) dd {
	r := ddExactMult(a.hi, b.hi)
	t1 := math.FMA(a.hi, b.lo, r.lo)
	r.lo = math.FMA(a.lo, b.hi, t1)
	return r
}

// ddAdd is add(DoubleDouble, DoubleDouble).
func ddAdd(a, b dd) dd {
	r := ddExactAdd(a.hi, b.hi)
	lo := a.lo + b.lo
	return ddExactAdd(r.hi, r.lo+lo)
}

// ddMulAdd is multiply_add<DoubleDouble>: a b + c, for |c| >= |a b|.
func ddMulAdd(a, b, c dd) dd {
	return ddAdd(c, ddQuickMult(a, b))
}

// ddDiv is div: the quotient of two numbers in two parts.
func ddDiv(a, b dd) dd {
	q := 1 / b.hi
	hi := float64(a.hi * q)
	eHi := math.FMA(b.hi, -hi, a.hi)
	eLo := math.FMA(b.lo, -hi, a.lo)
	return dd{float64(q * (eHi + eLo)), hi}
}
