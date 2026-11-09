// This file ports no upstream file. It adds the Number.prototype methods
// that the d3 port needs: toExponential and toPrecision. They give the
// results of V8 (src/numbers/conversions.cc).

package js

import (
	"math"
	"strconv"
	"strings"
)

// exactDigits returns the first n significant decimal digits of ax (a finite
// number greater than zero) and the decimal exponent of the first digit. The
// last digit is rounded on the exact value of ax, and a tie rounds up, as
// V8 does for toExponential and toPrecision.
func exactDigits(ax float64, n int) ([]byte, int) {
	// A float64 has at most 767 significant decimal digits, so 800 fraction
	// digits give the exact expansion.
	b := strconv.AppendFloat(make([]byte, 0, 820), ax, 'e', 800, 64)
	ePos := len(b) - 1
	for b[ePos] != 'e' {
		ePos--
	}
	exp, _ := strconv.Atoi(string(b[ePos+1:]))
	all := make([]byte, 0, ePos)
	all = append(all, b[0])
	all = append(all, b[2:ePos]...)
	digits := make([]byte, n)
	copy(digits, all[:n])
	if all[n] >= '5' {
		i := n - 1
		for ; i >= 0; i-- {
			if digits[i] == '9' {
				digits[i] = '0'
				continue
			}
			digits[i]++
			break
		}
		if i < 0 {
			digits[0] = '1'
			exp++
		}
	}
	return digits, exp
}

// shortestDigits returns the shortest digits that identify ax (a finite
// number greater than zero) and the decimal exponent of the first digit.
func shortestDigits(ax float64) ([]byte, int) {
	b := strconv.AppendFloat(make([]byte, 0, 32), ax, 'e', -1, 64)
	ePos := len(b) - 1
	for b[ePos] != 'e' {
		ePos--
	}
	exp, _ := strconv.Atoi(string(b[ePos+1:]))
	digits := make([]byte, 0, ePos)
	digits = append(digits, b[0])
	if ePos > 2 {
		digits = append(digits, b[2:ePos]...)
	}
	return digits, exp
}

func nonFiniteString(x float64) (string, bool) {
	switch {
	case math.IsNaN(x):
		return "NaN", true
	case math.IsInf(x, 1):
		return "Infinity", true
	case math.IsInf(x, -1):
		return "-Infinity", true
	}
	return "", false
}

func writeExponent(sb *strings.Builder, exp int) {
	sb.WriteByte('e')
	if exp < 0 {
		sb.WriteByte('-')
		exp = -exp
	} else {
		sb.WriteByte('+')
	}
	sb.WriteString(strconv.Itoa(exp))
}

// ToExponential is Number.prototype.toExponential(fractionDigits). A negative
// fractionDigits means that the argument is undefined: the result then has as
// many digits as necessary. The caller keeps fractionDigits at 100 or less,
// because JavaScript throws a RangeError above 100.
func ToExponential(x float64, fractionDigits int) string {
	if s, ok := nonFiniteString(x); ok {
		return s
	}
	var sb strings.Builder
	if x < 0 {
		sb.WriteByte('-')
		x = -x
	}
	var digits []byte
	exp := 0
	switch {
	case x == 0:
		n := 1
		if fractionDigits > 0 {
			n = fractionDigits + 1
		}
		digits = []byte(strings.Repeat("0", n))
	case fractionDigits < 0:
		digits, exp = shortestDigits(x)
	default:
		digits, exp = exactDigits(x, fractionDigits+1)
	}
	sb.WriteByte(digits[0])
	if len(digits) > 1 {
		sb.WriteByte('.')
		sb.Write(digits[1:])
	}
	writeExponent(&sb, exp)
	return sb.String()
}

// ToPrecision is Number.prototype.toPrecision(precision). The caller keeps
// precision in the range 1 to 100, because JavaScript throws a RangeError
// outside that range.
func ToPrecision(x float64, precision int) string {
	if s, ok := nonFiniteString(x); ok {
		return s
	}
	var sb strings.Builder
	if x < 0 {
		sb.WriteByte('-')
		x = -x
	}
	var digits []byte
	exp := 0
	if x == 0 {
		digits = []byte(strings.Repeat("0", precision))
	} else {
		digits, exp = exactDigits(x, precision)
	}
	if exp < -6 || exp >= precision {
		sb.WriteByte(digits[0])
		if len(digits) > 1 {
			sb.WriteByte('.')
			sb.Write(digits[1:])
		}
		writeExponent(&sb, exp)
		return sb.String()
	}
	if exp < 0 {
		sb.WriteString("0.")
		for i := exp + 1; i < 0; i++ {
			sb.WriteByte('0')
		}
		sb.Write(digits)
		return sb.String()
	}
	sb.Write(digits[:exp+1])
	if exp+1 < len(digits) {
		sb.WriteByte('.')
		sb.Write(digits[exp+1:])
	}
	return sb.String()
}
