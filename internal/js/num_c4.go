// This file ports no mermaid file. It adds the global parseInt(string) with
// no radix argument, with the rounding of V8 for a long digit string. The C4
// db calls parseInt on values of the diagram text. The algorithm follows
// StringToIntHelper and NumberParseIntHelper of V8
// (src/numbers/conversions.cc): the digits of a decimal number go to strtod,
// and the digits of a hexadecimal number are rounded to the nearest double,
// with a tie to the even one. ParseInt of num.go adds one digit at a time
// in floating point, so that it can differ in the last bits for a number
// above 2^53.

package js

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// ParseIntAuto is the global parseInt(s) with no radix: the radix is 16
// after the prefix 0x or 0X, and 10 in all other cases. The result is the
// double that is nearest to the integer that the digits give.
func ParseIntAuto(s string) float64 {
	s = strings.TrimLeftFunc(s, isJSSpace)
	negative := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		negative = s[0] == '-'
		s = s[1:]
	}
	negate := func(v float64) float64 {
		if negative {
			return -v
		}
		return v
	}
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		digits := s[2:]
		n := 0
		for n < len(digits) && isHexDigit(digits[n]) {
			n++
		}
		if n == 0 {
			return math.NaN()
		}
		v, _ := new(big.Int).SetString(digits[:n], 16)
		f, _ := new(big.Float).SetPrec(53).SetMode(big.ToNearestEven).SetInt(v).Float64()
		return negate(f)
	}
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	if n == 0 {
		return math.NaN()
	}
	// strconv gives the nearest double, and an infinity for a number that
	// is too large, as strtod does.
	f, _ := strconv.ParseFloat(s[:n], 64)
	return negate(f)
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
