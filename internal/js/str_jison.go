// This file ports no upstream file. It has the string functions of
// ECMAScript for strings that can hold one half of a surrogate pair.

package js

import "unicode/utf8"

// JavaScript strings are sequences of UTF-16 code units, and a jison lexer
// rule can match one half of a surrogate pair. Go strings in this port hold
// such a half as the three bytes that UTF-8 would use for the code point
// (generalized UTF-8, also named WTF-8). The functions in this file read and
// write that form. A string without a lone surrogate is plain UTF-8.

// UTF16Units returns the UTF-16 code units of s, one rune for each unit. A
// character above U+FFFF becomes two surrogate units. A lone surrogate in the
// three-byte form becomes one unit. Each other invalid byte becomes U+FFFD.
func UTF16Units(s string) []rune {
	return AppendUTF16Units(make([]rune, 0, len(s)), s)
}

// AppendUTF16Units appends the UTF-16 code units of s to dst.
func AppendUTF16Units(dst []rune, s string) []rune {
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			dst = append(dst, rune(c))
			i++
			continue
		}
		if c == 0xed && i+2 < len(s) && s[i+1] >= 0xa0 && s[i+1] <= 0xbf && s[i+2]&0xc0 == 0x80 {
			dst = append(dst, 0xd000|rune(s[i+1]&0x3f)<<6|rune(s[i+2]&0x3f))
			i += 3
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r >= 0x10000 {
			r -= 0x10000
			dst = append(dst, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
			continue
		}
		dst = append(dst, r)
	}
	return dst
}

// FromUTF16Units is the inverse of UTF16Units. A high surrogate that a low
// surrogate follows becomes one character. Any other surrogate keeps its
// three-byte form.
func FromUTF16Units(units []rune) string {
	ascii := true
	for _, u := range units {
		if u >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		b := make([]byte, len(units))
		for i, u := range units {
			b[i] = byte(u)
		}
		return string(b)
	}
	return string(AppendFromUTF16Units(make([]byte, 0, len(units)+8), units))
}

// AppendFromUTF16Units appends the string form of the code units to dst.
func AppendFromUTF16Units(dst []byte, units []rune) []byte {
	for i := 0; i < len(units); i++ {
		u := units[i]
		switch {
		case u < utf8.RuneSelf:
			dst = append(dst, byte(u))
		case u >= 0xd800 && u < 0xdc00 && i+1 < len(units) && units[i+1] >= 0xdc00 && units[i+1] < 0xe000:
			dst = utf8.AppendRune(dst, 0x10000+(u-0xd800)<<10+(units[i+1]-0xdc00))
			i++
		case u >= 0xd800 && u < 0xe000:
			dst = append(dst, 0xed, 0x80|byte(u>>6&0x3f), 0x80|byte(u&0x3f))
		default:
			dst = utf8.AppendRune(dst, u)
		}
	}
	return dst
}

// hasSurrogateBytes reports whether s can hold a lone surrogate.
func hasSurrogateBytes(s string) bool {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == 0xed && s[i+1] >= 0xa0 {
			return true
		}
	}
	return false
}

// Concat is the JavaScript + operator on strings. It differs from the Go
// operator when one string ends with a high surrogate and the next string
// starts with a low surrogate: the two halves become one character.
func Concat(parts ...string) string {
	n := 0
	lone := false
	for _, p := range parts {
		n += len(p)
		lone = lone || hasSurrogateBytes(p)
	}
	b := make([]byte, 0, n)
	for _, p := range parts {
		b = append(b, p...)
	}
	if !lone {
		return string(b)
	}
	return FromUTF16Units(UTF16Units(string(b)))
}

// ToWellFormed is String.prototype.toWellFormed: each lone surrogate becomes U+FFFD.
func ToWellFormed(s string) string {
	if !hasSurrogateBytes(s) {
		return s
	}
	units := UTF16Units(s)
	for i := 0; i < len(units); i++ {
		u := units[i]
		switch {
		case u >= 0xd800 && u < 0xdc00 && i+1 < len(units) && units[i+1] >= 0xdc00 && units[i+1] < 0xe000:
			i++
		case u >= 0xd800 && u < 0xe000:
			units[i] = utf8.RuneError
		}
	}
	return FromUTF16Units(units)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// clampIndex resolves a relative index as String.prototype.slice does.
func clampIndex(i, n int) int {
	if i < 0 {
		i += n
		if i < 0 {
			return 0
		}
		return i
	}
	if i > n {
		return n
	}
	return i
}

// StringSlice is String.prototype.slice(start, end). The indexes count UTF-16
// code units, and a negative index counts from the end.
func StringSlice(s string, start, end int) string {
	if isASCII(s) {
		start, end = clampIndex(start, len(s)), clampIndex(end, len(s))
		if start >= end {
			return ""
		}
		return s[start:end]
	}
	units := UTF16Units(s)
	start, end = clampIndex(start, len(units)), clampIndex(end, len(units))
	if start >= end {
		return ""
	}
	return FromUTF16Units(units[start:end])
}

// StringSliceFrom is String.prototype.slice(start) with no end.
func StringSliceFrom(s string, start int) string {
	if isASCII(s) {
		return s[clampIndex(start, len(s)):]
	}
	units := UTF16Units(s)
	return FromUTF16Units(units[clampIndex(start, len(units)):])
}

// Substr is String.prototype.substr(start, length). A negative start counts
// from the end, and a negative length gives the empty string.
func Substr(s string, start, length int) string {
	var units []rune
	n := len(s)
	ascii := isASCII(s)
	if !ascii {
		units = UTF16Units(s)
		n = len(units)
	}
	start = clampIndex(start, n)
	if length <= 0 {
		return ""
	}
	end := n
	if length < n-start {
		end = start + length
	}
	if ascii {
		return s[start:end]
	}
	return FromUTF16Units(units[start:end])
}

// CharAt is String.prototype.charAt: the code unit at index i as a string, or
// the empty string when i is out of range.
func CharAt(s string, i int) string {
	if i < 0 {
		return ""
	}
	if isASCII(s) {
		if i >= len(s) {
			return ""
		}
		return s[i : i+1]
	}
	units := UTF16Units(s)
	if i >= len(units) {
		return ""
	}
	return FromUTF16Units(units[i : i+1])
}

// QuoteJSON prints s as JSON.stringify does for a string: a lone surrogate
// becomes a \udXXX escape and all other characters above U+001F stay as they are.
func QuoteJSON(s string) string {
	const hex = "0123456789abcdef"
	b := make([]byte, 0, len(s)+2)
	b = append(b, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			b = append(b, '\\', '"')
		case c == '\\':
			b = append(b, '\\', '\\')
		case c == '\b':
			b = append(b, '\\', 'b')
		case c == '\f':
			b = append(b, '\\', 'f')
		case c == '\n':
			b = append(b, '\\', 'n')
		case c == '\r':
			b = append(b, '\\', 'r')
		case c == '\t':
			b = append(b, '\\', 't')
		case c < 0x20:
			b = append(b, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xf])
		case c == 0xed && i+2 < len(s) && s[i+1] >= 0xa0 && s[i+1] <= 0xbf && s[i+2]&0xc0 == 0x80:
			u := 0xd000 | rune(s[i+1]&0x3f)<<6 | rune(s[i+2]&0x3f)
			b = append(b, '\\', 'u', hex[u>>12], hex[u>>8&0xf], hex[u>>4&0xf], hex[u&0xf])
			i += 2
		default:
			b = append(b, c)
		}
	}
	return string(append(b, '"'))
}
