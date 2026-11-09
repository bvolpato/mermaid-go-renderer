// This file ports no upstream file. It has String.prototype.toLowerCase of
// ECMAScript, with the case conversion that the Unicode Standard defines.

package js

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ToLowerCase is String.prototype.toLowerCase: the locale-independent default
// case conversion of Unicode. It differs from strings.ToLower in two places.
// U+0130 becomes "i" and U+0307, and a capital sigma at the end of a word
// becomes the final form U+03C2.
func ToLowerCase(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return strings.ToLower(s)
	}
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			// An invalid byte, or one byte of a lone surrogate, is kept.
			sb.WriteByte(s[i])
			i++
			continue
		}
		switch r {
		case 0x130:
			sb.WriteString("i")
			sb.WriteRune(0x307)
		case 0x3A3:
			if finalSigma(s, i, i+size) {
				sb.WriteRune(0x3C2)
			} else {
				sb.WriteRune(0x3C3)
			}
		default:
			sb.WriteRune(unicode.ToLower(r))
		}
		i += size
	}
	return sb.String()
}

// finalSigma is the Final_Sigma condition of SpecialCasing.txt for the sigma
// at s[start:end]: a cased letter comes before it, with any number of
// case-ignorable characters between, and no such sequence comes after it.
func finalSigma(s string, start, end int) bool {
	before := false
	for i := start; i > 0; {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		i -= size
		if caseIgnorable(r) {
			continue
		}
		before = cased(r)
		break
	}
	if !before {
		return false
	}
	for i := end; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if caseIgnorable(r) {
			continue
		}
		return !cased(r)
	}
	return true
}

// cased is the Unicode property Cased.
func cased(r rune) bool {
	return unicode.In(r, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Other_Lowercase, unicode.Other_Uppercase)
}

// caseIgnorable is the Unicode property Case_Ignorable: the general
// categories Mn, Me, Cf, Lm and Sk, and the characters whose Word_Break
// property is MidLetter, MidNumLet or Single_Quote.
func caseIgnorable(r rune) bool {
	switch r {
	case 0x27, 0x2E, 0x3A, 0xB7, 0x387, 0x55F, 0x5F4, 0x2018, 0x2019, 0x2024, 0x2027,
		0xFE13, 0xFE52, 0xFE55, 0xFF07, 0xFF0E, 0xFF1A:
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk)
}
