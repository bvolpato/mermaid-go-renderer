// Ported from font-types 0.12.5 (src/fixed.rs) and from skrifa 0.46.2
// (src/outline/autohint/metrics/mod.rs: the fixed point helpers, and
// src/outline/autohint/mod.rs: derived_constant).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/base/ftcalc.c), which has the FreeType License
// (LICENSE-FreeType.txt).

package font

// The automatic hinter of skrifa works with 32 bit integers, and the
// arithmetic wraps on overflow in a release build. The Go code uses int32 for
// the same values, so an overflow gives the same result.

// ahUnsignedAbs is unsigned_abs of a 32 bit integer, as a 64 bit number.
func ahUnsignedAbs(a int32) uint64 {
	if a < 0 {
		return uint64(uint32(-a))
	}
	return uint64(a)
}

// ahAbs is abs of a 32 bit integer. The smallest integer stays as it is.
func ahAbs(a int32) int32 {
	if a < 0 {
		return -a
	}
	return a
}

// ahFixedMul is fixed_mul: the product of two 16.16 numbers. A half rounds
// away from zero.
func ahFixedMul(a, b int32) int32 {
	ab := int64(a) * int64(b)
	if ab < 0 {
		return int32((ab + 0x8000 - 1) >> 16)
	}
	return int32((ab + 0x8000) >> 16)
}

// ahFixedDiv is fixed_div: the quotient of two 16.16 numbers, rounded to the
// nearest integer. A division by zero gives the largest number.
func ahFixedDiv(a, b int32) int32 {
	sign := (a < 0) != (b < 0)
	au, bu := ahUnsignedAbs(a), ahUnsignedAbs(b)
	q := uint32(0x7FFFFFFF)
	if bu != 0 {
		q = uint32(((au << 16) + (bu >> 1)) / bu)
	}
	if sign {
		return -int32(q)
	}
	return int32(q)
}

// ahFixedMulDiv is fixed_mul_div: a*b/c, rounded to the nearest integer.
func ahFixedMulDiv(a, b, c int32) int32 {
	sign := 1
	su, au, bu := uint64(int64(a)), uint64(int64(b)), uint64(int64(c))
	if a < 0 {
		su = -su
		sign = -1
	}
	if b < 0 {
		au = -au
		sign = -sign
	}
	if c < 0 {
		bu = -bu
		sign = -sign
	}
	result := uint64(0x7FFFFFFF)
	if bu > 0 {
		result = (su*au + (bu >> 1)) / bu
	}
	if sign < 0 {
		return -int32(result)
	}
	return int32(result)
}

// ahPixRound is pix_round: a 26.6 number rounded to a whole pixel.
func ahPixRound(a int32) int32 { return (a + 32) &^ 63 }

// ahPixFloor is pix_floor: a 26.6 number rounded down to a whole pixel.
func ahPixFloor(a int32) int32 { return a &^ 63 }

// ahDerivedConstant is derived_constant: a constant that is defined for
// 2048 units per em, for the em size of the font.
func ahDerivedConstant(unitsPerEm, value int32) int32 {
	return value * unitsPerEm / 2048
}
