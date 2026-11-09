// This file ports no mermaid file. It adds the global functions escape and
// unescape (ECMAScript Annex B), which entityDecode of utils.ts calls.

package js

import "strings"

const upperHex = "0123456789ABCDEF"

// isEscapeUnreserved reports whether escape keeps the code unit as it is.
func isEscapeUnreserved(u rune) bool {
	switch {
	case u >= 'A' && u <= 'Z', u >= 'a' && u <= 'z', u >= '0' && u <= '9':
		return true
	}
	switch u {
	case '@', '*', '_', '+', '-', '.', '/':
		return true
	}
	return false
}

// Escape is the global function escape. It works on UTF-16 code units: a
// unit below 256 becomes %XX and another unit becomes %uXXXX.
func Escape(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, u := range UTF16Units(s) {
		switch {
		case isEscapeUnreserved(u):
			sb.WriteByte(byte(u))
		case u < 256:
			sb.WriteByte('%')
			sb.WriteByte(upperHex[u>>4])
			sb.WriteByte(upperHex[u&15])
		default:
			sb.WriteString("%u")
			sb.WriteByte(upperHex[u>>12&15])
			sb.WriteByte(upperHex[u>>8&15])
			sb.WriteByte(upperHex[u>>4&15])
			sb.WriteByte(upperHex[u&15])
		}
	}
	return sb.String()
}

func hexValue(u rune) (rune, bool) {
	switch {
	case u >= '0' && u <= '9':
		return u - '0', true
	case u >= 'a' && u <= 'f':
		return u - 'a' + 10, true
	case u >= 'A' && u <= 'F':
		return u - 'A' + 10, true
	}
	return 0, false
}

// hexUnits reads n hexadecimal digits from units.
func hexUnits(units []rune, n int) (rune, bool) {
	if len(units) < n {
		return 0, false
	}
	var v rune
	for _, u := range units[:n] {
		d, ok := hexValue(u)
		if !ok {
			return 0, false
		}
		v = v<<4 | d
	}
	return v, true
}

// Unescape is the global function unescape: it replaces %XX and %uXXXX by
// the code unit with that value.
func Unescape(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	units := UTF16Units(s)
	out := make([]rune, 0, len(units))
	for k := 0; k < len(units); k++ {
		c := units[k]
		if c == '%' {
			if k+6 <= len(units) && units[k+1] == 'u' {
				if v, ok := hexUnits(units[k+2:], 4); ok {
					c = v
					k += 5
				}
			} else if k+3 <= len(units) {
				if v, ok := hexUnits(units[k+1:], 2); ok {
					c = v
					k += 2
				}
			}
		}
		out = append(out, c)
	}
	return FromUTF16Units(out)
}
