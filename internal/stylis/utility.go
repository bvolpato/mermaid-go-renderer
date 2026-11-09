// Ported from stylis/src/Utility.js (stylis 4.3.6).

package stylis

import (
	"slices"
	"unicode/utf16"
)

// unit is one character of the text that the parser reads. Stylis works on
// UTF-16 code units, and some of its index arithmetic can cut a character
// that is not ASCII. So the port reads ASCII text as bytes and all other text
// as UTF-16 code units. Both forms have the indices of the JavaScript string.
type unit interface {
	~byte | ~uint16
}

// toUnits converts a literal to units. The literals in the port are ASCII.
func toUnits[T unit](s string) []T {
	out := make([]T, len(s))
	for i := 0; i < len(s); i++ {
		out[i] = T(s[i])
	}
	return out
}

// str converts units to a Go string. A surrogate that has no pair becomes
// U+FFFD, as it does when JavaScript text is written as UTF-8.
func str[T unit](value []T) string {
	switch v := any(value).(type) {
	case []byte:
		return string(v)
	case []uint16:
		return string(utf16.Decode(v))
	}
	buf := make([]uint16, len(value))
	for i, c := range value {
		buf[i] = uint16(c)
	}
	return string(utf16.Decode(buf))
}

// strs converts a list of unit strings.
func strs[T unit](values [][]T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = str(v)
	}
	return out
}

// abs is Math.abs for the integers that stylis stores.
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// isSpace reports whether String.prototype.trim removes the character.
func isSpace[T unit](c T) bool {
	switch uint16(c) {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return uint16(c) >= 0x2000 && uint16(c) <= 0x200a
}

// trim is String.prototype.trim.
func trim[T unit](value []T) []T {
	begin, end := 0, len(value)
	for begin < end && isSpace(value[begin]) {
		begin++
	}
	for end > begin && isSpace(value[end-1]) {
		end--
	}
	return value[begin:end]
}

// charat is value.charCodeAt(index) | 0. An index outside the string gives 0.
func charat[T unit](value []T, index int) int {
	if index < 0 || index >= len(value) {
		return 0
	}
	return int(value[index])
}

// substr is value.slice(begin, end). A negative index counts from the end.
func substr[T unit](value []T, begin, end int) []T {
	n := len(value)
	if begin < 0 {
		begin = max(n+begin, 0)
	} else if begin > n {
		begin = n
	}
	if end < 0 {
		end = max(n+end, 0)
	} else if end > n {
		end = n
	}
	if begin >= end {
		return nil
	}
	return value[begin:end]
}

// remove is replace(value, pattern, ”) for a pattern of one character. It
// removes each match when all is true, and the first match when it is false.
func remove[T unit](value []T, pattern T, all bool) []T {
	i := slices.Index(value, pattern)
	if i < 0 {
		return value
	}
	out := make([]T, 0, len(value))
	out = append(out, value[:i]...)
	for _, c := range value[i+1:] {
		if all && c == pattern {
			continue
		}
		out = append(out, c)
	}
	return out
}

// replaceAmpersand is replace(value, /&\f/g, replacement). The replacement is
// a string, so JavaScript expands the "$" patterns in it.
func replaceAmpersand[T unit](value, replacement []T) []T {
	first := indexAmpersand(value, 0)
	if first < 0 {
		return value
	}
	out := make([]T, 0, len(value)+len(replacement))
	rest := 0
	for i := first; i >= 0; i = indexAmpersand(value, rest) {
		out = append(out, value[rest:i]...)
		out = getSubstitution(out, replacement, value, i, 2)
		rest = i + 2
	}
	return append(out, value[rest:]...)
}

// appendAmpersand is characters + replace(value, '&', '&\f'). The pattern is
// a string, so JavaScript replaces only the first "&".
func appendAmpersand[T unit](characters, value []T) []T {
	i := slices.Index(value, '&')
	if i < 0 {
		return append(characters, value...)
	}
	characters = append(characters, value[:i+1]...)
	characters = append(characters, '\f')
	return append(characters, value[i+1:]...)
}

// indexof is indexof(value, '&\f', position): value.indexOf('&\f', position).
func indexof[T unit](value []T, position int) int {
	return indexAmpersand(value, position)
}

// indexAmpersand finds "&\f" in value at or after from.
func indexAmpersand[T unit](value []T, from int) int {
	for i := from; i+1 < len(value); i++ {
		if value[i] == '&' && value[i+1] == '\f' {
			return i
		}
	}
	return -1
}

// getSubstitution is the GetSubstitution abstract operation for a match that
// has no capture groups. It appends the expanded replacement to out. The
// match is the length characters of str at position.
func getSubstitution[T unit](out, replacement, str []T, position, length int) []T {
	for i := 0; i < len(replacement); i++ {
		c := replacement[i]
		if c != '$' || i+1 >= len(replacement) {
			out = append(out, c)
			continue
		}
		switch replacement[i+1] {
		case '$':
			out = append(out, '$')
			i++
		case '&':
			out = append(out, str[position:position+length]...)
			i++
		case '`':
			out = append(out, str[:position]...)
			i++
		case '\'':
			out = append(out, str[position+length:]...)
			i++
		default:
			// "$1" and "$<name>" stay as written, because the pattern has no groups.
			out = append(out, c)
		}
	}
	return out
}
