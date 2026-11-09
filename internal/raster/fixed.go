// Ported from skia/include/private/SkFixed.h, src/core/SkFDot6.h and
// include/private/SkSafe32.h (Skia branch chrome/m156).

package raster

import "math"

// fixed is SkFixed: a 16.16 fixed-point number.
type fixed = int32

// fdot6 is SkFDot6: a 26.6 fixed-point number.
type fdot6 = int32

const (
	fixed1    fixed = 1 << 16
	fixedHalf fixed = 1 << 15
	maxS32          = math.MaxInt32
	minS32          = math.MinInt32
)

func intToFixed(n int) fixed          { return fixed(uint32(n) << 16) }
func fixedFloorToInt(x fixed) int     { return int(x >> 16) }
func fixedCeilToInt(x fixed) int      { return int((x + fixed1 - 1) >> 16) }
func fixedRoundToInt(x fixed) int     { return int((x + fixedHalf) >> 16) }
func fixedFloorToFixed(x fixed) fixed { return fixed(uint32(x) & 0xFFFF0000) }
func fixedCeilToFixed(x fixed) fixed {
	return fixed(uint32(x+fixed1-1) & 0xFFFF0000)
}
func fixedRoundToFixed(x fixed) fixed {
	return fixed(uint32(x+fixedHalf) & 0xFFFF0000)
}

// fixedMul is SkFixedMul.
func fixedMul(a, b fixed) fixed { return fixed(int64(a) * int64(b) >> 16) }

// fixedDiv is SkFixedDiv: the quotient is pinned to 32 bits.
func fixedDiv(numer, denom int32) fixed {
	return pinToS32((int64(numer) << 16) / int64(denom))
}

func pinToS32(x int64) int32 {
	if x < minS32 {
		return minS32
	}
	if x > maxS32 {
		return maxS32
	}
	return int32(x)
}

func satAdd32(a, b int32) int32 { return pinToS32(int64(a) + int64(b)) }
func satSub32(a, b int32) int32 { return pinToS32(int64(a) - int64(b)) }

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

func fixedToFDot6(x fixed) fdot6 { return x >> 10 }
func fdot6ToFixed(x fdot6) fixed { return fixed(uint32(x) << 10) }
func fdot6Round(x fdot6) int     { return int((x + 32) >> 6) }
func fdot6Floor(x fdot6) int     { return int(x >> 6) }
func fdot6Ceil(x fdot6) int      { return int((x + 63) >> 6) }

// fdot6Div is SkFDot6Div.
func fdot6Div(a, b fdot6) fixed {
	if a == int32(int16(a)) {
		return fixed((uint32(a) << 16)) / b
	}
	return fixedDiv(a, b)
}

// scalarToFDot6 is SkScalarToFDot6: a truncating conversion of x * 64.
func scalarToFDot6(x float32) fdot6 { return floatToInt(float32(x * 64)) }

// floatToInt is the C conversion (int)x on arm64 and with Skia's saturating
// helpers: it truncates toward zero and saturates.
func floatToInt(x float32) int32 {
	if x != x {
		return 0
	}
	if x >= 2147483648.0 {
		return maxS32
	}
	if x <= -2147483648.0 {
		return minS32
	}
	return int32(x)
}

// floatToFixed is SkFloatToFixed: sk_float_saturate2int(x * 65536).
func floatToFixed(x float32) fixed { return floatToInt(float32(x * 65536)) }

// clz32 is SkCLZ.
func clz32(x uint32) int {
	n := 0
	if x == 0 {
		return 32
	}
	for x&0x80000000 == 0 {
		x <<= 1
		n++
	}
	return n
}
