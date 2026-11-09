// Ported from V8 15.6.75, src/numbers/ieee754.cc: math::pow (BSD-3-Clause,
// Copyright 2011 the V8 project authors, ../LICENSE-V8.txt).
//
// V8 calls std::pow of the C library for Math.pow (the flag
// use_std_math_pow), after some special cases. So the last bit of a result
// depends on the platform of the browser:
//
//   - Linux: glibc, which has the pow of Arm Optimized Routines
//     (linux_libm_pow.go).
//   - macOS: libsystem_m of Apple, which is closed source. The module has no
//     port of it. PowDarwin returns the results of Linux (platform.go).
//
// The golden files of the two platforms differ for 283 of 220000 arguments
// of pow, each time by one unit in the last place.

package crmath

import "math"

// darwinLibmPow is the place of a pow of the C library of macOS. No file of
// the module sets it. It is nil, and the platform Darwin has the pow of
// Linux.
var darwinLibmPow func(x, y float64) float64

// Pow is Math.pow, and the operator **, of the current platform.
func Pow(x, y float64) float64 {
	if CurrentPlatform() == PlatformLinux {
		return PowLinux(x, y)
	}
	return PowDarwin(x, y)
}

// PowLinux is Math.pow of Chromium on Linux. It does not read the current
// platform.
func PowLinux(x, y float64) float64 {
	if r, ok := powSpecial(x, y); ok {
		return r
	}
	if x != x {
		// The pow of glibc returns NaN for a signaling NaN and a zero
		// exponent. Chromium on Linux returns 1 for these arguments, as for
		// a quiet NaN (measured with a NaN from a Float64Array).
		x = math.NaN()
	}
	return linuxPow(x, y)
}

// PowDarwin is Math.pow of Chromium on macOS. It does not read the current
// platform. It is PowLinux when the package is built without the pow of
// macOS (see HasDarwinLibm).
func PowDarwin(x, y float64) float64 {
	libm := darwinLibmPow
	if libm == nil {
		return PowLinux(x, y)
	}
	if r, ok := powSpecial(x, y); ok {
		return r
	}
	return libm(x, y)
}

// powSpecial has the special cases of math::pow of V8, which do not call
// the C library. ok is false for the other arguments.
func powSpecial(x, y float64) (r float64, ok bool) {
	if math.IsNaN(y) {
		return math.NaN(), true
	}
	if math.IsInf(y, 0) && (x == 1 || x == -1) {
		return math.NaN(), true
	}
	if y == 2 {
		return float64(x * x), true
	}
	if y == 0.5 {
		switch {
		case math.IsInf(x, 0):
			return math.Inf(1), true
		case x == 0:
			return 0, true // sqrt(-0 + 0)
		}
		return math.Sqrt(x), true
	}
	return 0, false
}

// powLibm is the pow of the C library of the current platform, without the
// special cases of V8.
func powLibm(x, y float64) float64 {
	if libm := darwinLibmPow; libm != nil && CurrentPlatform() != PlatformLinux {
		return libm(x, y)
	}
	return linuxPow(x, y)
}
