// This file ports no upstream file. It has the number conversions of
// ECMAScript: Number to string, Math.round, toFixed, parseFloat and parseInt.

// Package js holds the JavaScript semantics that the port of mermaid needs.
//
// The Go renderer must produce the same strings and the same iteration order
// as mermaid in a browser. This package is the single place for the parts of
// ECMAScript that differ from Go: number formatting, Math.round, parseFloat,
// property order of objects, and insertion-ordered maps.
package js

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Str formats a number as JavaScript String(x) does (Number::toString, radix 10).
func Str(x float64) string {
	switch {
	case math.IsNaN(x):
		return "NaN"
	case x == 0:
		return "0"
	case math.IsInf(x, 1):
		return "Infinity"
	case math.IsInf(x, -1):
		return "-Infinity"
	}
	// An integer below 2^53 prints as its digits: the shortest digits that
	// give the number back are the digits of the integer.
	if x == math.Trunc(x) && math.Abs(x) < 1<<53 {
		return strconv.FormatInt(int64(x), 10)
	}
	var buf [32]byte
	b := strconv.AppendFloat(buf[:0], math.Abs(x), 'e', -1, 64)
	// b is "d.ddddde±xx" or "de±xx". Split it into digits and exponent.
	ePos := -1
	for i, c := range b {
		if c == 'e' {
			ePos = i
			break
		}
	}
	exp := 0
	for _, c := range b[ePos+2:] {
		exp = exp*10 + int(c-'0')
	}
	if b[ePos+1] == '-' {
		exp = -exp
	}
	var digitBuf [20]byte
	digits := digitBuf[:0]
	for _, c := range b[:ePos] {
		if c != '.' {
			digits = append(digits, c)
		}
	}
	k := len(digits)
	n := exp + 1 // the decimal point is after the first n digits

	// The longest result has a sign, 21 digits and a point, or 17 digits
	// and an exponent.
	var outBuf [48]byte
	out := outBuf[:0]
	if x < 0 {
		out = append(out, '-')
	}
	switch {
	case k <= n && n <= 21:
		out = append(out, digits...)
		for i := k; i < n; i++ {
			out = append(out, '0')
		}
	case 0 < n && n <= 21:
		out = append(out, digits[:n]...)
		out = append(out, '.')
		out = append(out, digits[n:]...)
	case -6 < n && n <= 0:
		out = append(out, '0', '.')
		for i := n; i < 0; i++ {
			out = append(out, '0')
		}
		out = append(out, digits...)
	default:
		out = append(out, digits[0])
		if k > 1 {
			out = append(out, '.')
			out = append(out, digits[1:]...)
		}
		out = append(out, 'e')
		e := n - 1
		if e < 0 {
			out = append(out, '-')
			e = -e
		} else {
			out = append(out, '+')
		}
		out = strconv.AppendInt(out, int64(e), 10)
	}
	return string(out)
}

// Round is Math.round: the nearest integer, and a tie goes toward +Infinity.
func Round(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	f := math.Floor(x)
	if x-f >= 0.5 {
		f++
	}
	if f == 0 && (x < 0 || math.Signbit(x)) {
		return math.Copysign(0, -1) // Math.round(-0.2) is -0
	}
	return f
}

// ToFixed is Number.prototype.toFixed. A tie rounds away from zero, on the
// exact decimal value of the number.
func ToFixed(x float64, digits int) string {
	if math.IsNaN(x) {
		return "NaN"
	}
	if math.Abs(x) >= 1e21 || math.IsInf(x, 0) {
		return Str(x)
	}
	neg := x < 0
	ax := math.Abs(x)
	// The exact decimal expansion of a float64 has at most 1074 fraction digits.
	exact := new(big.Float).SetPrec(2048).SetFloat64(ax).Text('f', 1080)
	dot := strings.IndexByte(exact, '.')
	intPart, frac := exact[:dot], exact[dot+1:]
	keep := frac[:digits]
	rest := frac[digits:]
	num := []byte(intPart + keep)
	if rest[0] >= '5' {
		i := len(num) - 1
		for ; i >= 0; i-- {
			if num[i] == '9' {
				num[i] = '0'
				continue
			}
			num[i]++
			break
		}
		if i < 0 {
			num = append([]byte{'1'}, num...)
		}
	}
	s := string(num)
	if digits > 0 {
		s = s[:len(s)-digits] + "." + s[len(s)-digits:]
	}
	// JavaScript prints "-0.00" for a negative x that rounds to zero, except for -0 itself.
	if neg && x != 0 {
		s = "-" + s
	}
	return s
}

func isJSSpace(c rune) bool {
	switch c {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return c >= 0x2000 && c <= 0x200a
}

// TrimSpace removes the white space that String.prototype.trim removes.
func TrimSpace(s string) string {
	return strings.TrimFunc(s, isJSSpace)
}

// ParseFloat is the global parseFloat: it reads the longest numeric prefix.
func ParseFloat(s string) float64 {
	s = strings.TrimLeftFunc(s, isJSSpace)
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	if strings.HasPrefix(s[i:], "Infinity") {
		if s[0] == '-' {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	start := i
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return math.NaN()
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	_ = start
	text := s[:i]
	if strings.HasSuffix(text, ".") {
		text = text[:len(text)-1]
	}
	if strings.HasPrefix(text, ".") || strings.HasPrefix(text, "-.") || strings.HasPrefix(text, "+.") {
		text = strings.Replace(text, ".", "0.", 1)
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		// strconv reports a range error and still returns ±Inf or 0, as JavaScript does.
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return f
		}
		return math.NaN()
	}
	return f
}

// ParseInt is the global parseInt. A radix of 0 means "detect" (10, or 16 after 0x).
func ParseInt(s string, radix int) float64 {
	s = strings.TrimLeftFunc(s, isJSSpace)
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if radix == 0 {
		radix = 10
		if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
			radix = 16
			s = s[2:]
		}
	} else if radix == 16 && len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	}
	if radix < 2 || radix > 36 {
		return math.NaN()
	}
	val := 0.0
	n := 0
	for ; n < len(s); n++ {
		c := s[n]
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'z':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'Z':
			d = int(c-'A') + 10
		default:
			d = 99
		}
		if d >= radix {
			break
		}
		val = float64(val*float64(radix)) + float64(d)
	}
	if n == 0 {
		return math.NaN()
	}
	if neg {
		return -val
	}
	return val
}

// Number is the conversion Number(string): the whole string must be numeric.
// An empty or blank string is 0.
func Number(s string) float64 {
	s = TrimSpace(s)
	if s == "" {
		return 0
	}
	if len(s) > 2 && s[0] == '0' {
		radix := 0
		switch s[1] {
		case 'x', 'X':
			radix = 16
		case 'o', 'O':
			radix = 8
		case 'b', 'B':
			radix = 2
		}
		if radix != 0 {
			v, err := strconv.ParseUint(s[2:], radix, 64)
			if err != nil {
				return math.NaN()
			}
			return float64(v)
		}
	}
	switch s {
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9') && c != '.' && c != 'e' && c != 'E' && c != '+' && c != '-' {
			return math.NaN()
		}
	}
	f := ParseFloat(s)
	if math.IsNaN(f) {
		return f
	}
	// ParseFloat accepts a prefix. Number does not: check that nothing is left.
	if _, err := strconv.ParseFloat(normalizeFloatLiteral(s), 64); err != nil {
		if ne, ok := err.(*strconv.NumError); !ok || ne.Err != strconv.ErrRange {
			return math.NaN()
		}
	}
	return f
}

func normalizeFloatLiteral(s string) string {
	if strings.HasSuffix(s, ".") {
		s = s[:len(s)-1]
	}
	switch {
	case strings.HasPrefix(s, "."):
		s = "0" + s
	case strings.HasPrefix(s, "-."), strings.HasPrefix(s, "+."):
		s = s[:1] + "0" + s[1:]
	}
	return strings.Replace(s, ".e", "e", 1)
}

// IsArrayIndex reports whether key is a canonical array index ("0", "17", not
// "017" or "-1"). Objects enumerate these keys first, in numeric order.
func IsArrayIndex(key string) (uint32, bool) {
	if key == "" || len(key) > 10 {
		return 0, false
	}
	if key[0] == '0' {
		return 0, len(key) == 1
	}
	var v uint64
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + uint64(c-'0')
	}
	if v > 4294967294 {
		return 0, false
	}
	return uint32(v), true
}
