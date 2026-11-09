// Ported from the number helpers of the GWT runtime in elk-worker.js of
// elkjs 0.9.3 (round_int, Double.compare, Math.signum, String.hashCode,
// Double.hashCode, Integer.reverse) (Apache-2.0).
//
// GWT has the Apache License 2.0. elk-worker.js has no notice for this
// code. The Java sources of GWT for it are Double.java, Math.java,
// String.java and Integer.java in user/super/com/google/gwt/emul/java/lang,
// and Cast.java (round_int) in
// dev/core/super/com/google/gwt/dev/jjs/intrinsic/com/google/gwt/lang.
// Their copyright lines, read from the main branch of
// https://github.com/gwtproject/gwt on 2026-11-09:
//
//	Copyright 2007 Google Inc. (Double.java)
//	Copyright 2008 Google Inc. (Math.java, String.java, Integer.java, Cast.java)
//
// This file is a Go translation of that code. It is a changed version of
// it. LICENSE-GWT.txt in this directory has the license, and NOTICE in this
// directory has the license notice of the GWT files.

package gwt

import "math"

// RoundInt is the cast (int) x of a double: it truncates, clamps to the range
// of int, and gives 0 for NaN.
func RoundInt(x float64) int {
	if x != x {
		return 0
	}
	if x >= 2147483647 {
		return 2147483647
	}
	if x <= -2147483648 {
		return -2147483648
	}
	return int(x)
}

// ToInt32 is x | 0 of JavaScript for an integer that is exact in a double.
func ToInt32(x int) int {
	return int(int32(uint32(x)))
}

// DoubleCompare is Double.compare. -0 is smaller than 0, and NaN is larger
// than each other value.
func DoubleCompare(x, y float64) int {
	if x < y {
		return -1
	}
	if x > y {
		return 1
	}
	if x == y {
		if x == 0 {
			sx, sy := math.Signbit(x), math.Signbit(y)
			switch {
			case sx && !sy:
				return -1
			case !sx && sy:
				return 1
			}
		}
		return 0
	}
	if x != x {
		if y != y {
			return 0
		}
		return 1
	}
	return -1
}

// IntCompare is Integer.compare.
func IntCompare(x, y int) int {
	if x < y {
		return -1
	}
	if x > y {
		return 1
	}
	return 0
}

// BoolCompare is Boolean.compare.
func BoolCompare(x, y bool) int {
	if x == y {
		return 0
	}
	if x {
		return 1
	}
	return -1
}

// Signum is Math.signum.
func Signum(d float64) float64 {
	if d == 0 || d != d {
		return d
	}
	if d < 0 {
		return -1
	}
	return 1
}

// StringHash is String.hashCode over the UTF-16 code units of s.
func StringHash(s string) int32 {
	var h int32
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			h = h*31 + int32(0xd800+(r>>10))
			h = h*31 + int32(0xdc00+(r&0x3ff))
			continue
		}
		h = h*31 + int32(r)
	}
	return h
}

// DoubleHash is Double.hashCode of GWT: the value cast to int.
func DoubleHash(d float64) int32 {
	return int32(RoundInt(d))
}

// BoolHash is Boolean.hashCode.
func BoolHash(b bool) int32 {
	if b {
		return 1231
	}
	return 1237
}

// IntegerReverse is Integer.reverse.
func IntegerReverse(i int32) int32 {
	u := uint32(i)
	u = (u&0x55555555)<<1 | (u>>1)&0x55555555
	u = (u&0x33333333)<<2 | (u>>2)&0x33333333
	u = (u&0x0f0f0f0f)<<4 | (u>>4)&0x0f0f0f0f
	u = u<<24 | (u&0xff00)<<8 | (u>>8)&0xff00 | u>>24
	return int32(u)
}
