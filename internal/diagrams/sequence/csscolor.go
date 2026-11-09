// Ported from mermaid 12.2.0, diagrams/sequence/sequenceDb.ts (parseBoxData):
// the color test, window.CSS.supports('color', value) of Chromium (MIT).

package sequence

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
)

// cssColorKeywords holds the keywords that are a valid value of the color
// property and that are not a named color: the CSS-wide keywords, and the
// system colors of Chromium.
var cssColorKeywords = map[string]bool{
	"transparent": true, "currentcolor": true,
	"inherit": true, "initial": true, "unset": true, "revert": true,
	"accentcolor": true, "accentcolortext": true, "activeborder": true, "activecaption": true,
	"activetext": true, "appworkspace": true, "background": true, "buttonborder": true,
	"buttonface": true, "buttonhighlight": true, "buttonshadow": true, "buttontext": true,
	"canvas": true, "canvastext": true, "captiontext": true, "field": true, "fieldtext": true,
	"graytext": true, "highlight": true, "highlighttext": true, "inactiveborder": true,
	"inactivecaption": true, "inactivecaptiontext": true, "infobackground": true, "infotext": true,
	"linktext": true, "mark": true, "marktext": true, "menu": true, "menutext": true,
	"scrollbar": true, "selecteditem": true, "selecteditemtext": true, "threeddarkshadow": true,
	"threedface": true, "threedhighlight": true, "threedlightshadow": true, "threedshadow": true,
	"visitedtext": true, "window": true, "windowframe": true, "windowtext": true,
}

// cssSupportsColor is window.CSS.supports('color', value) for the values
// that parseBoxData tests: one word of the class \w, or a text that starts
// with rgb, rgba, hsl or hsla.
//
// The function knows the legacy syntax (commas) and the modern syntax (white
// space and a slash) of rgb() and hsl() with numbers, percentages, angles and
// none. It does not know math functions (calc), the relative color syntax
// (from) and substitution functions other than a plain var(--name). A
// function text must end with ")", as the texts of parseBoxData do: the
// function that the CSS parser closes at the end of the input is not
// accepted.
func cssSupportsColor(value string) bool {
	v := strings.ToLower(stripCSSComments(value))
	v = strings.Trim(v, " \t\n\r\f")
	if v == "" {
		return false
	}
	if !strings.ContainsAny(v, "(),/ \t\n\r\f") {
		return css.IsNamedColor(v) || cssColorKeywords[v]
	}
	open := strings.IndexByte(v, '(')
	if open < 0 || v[len(v)-1] != ')' {
		return false
	}
	name, body := v[:open], v[open+1:len(v)-1]
	if name != "rgb" && name != "rgba" && name != "hsl" && name != "hsla" {
		return false
	}
	if strings.Contains(body, "var(") {
		return cssValidVarReferences(body)
	}
	tokens, ok := cssColorTokens(body)
	if !ok {
		return false
	}
	isHSL := name == "hsl" || name == "hsla"
	legacy := false
	for _, t := range tokens {
		if t.kind == cssComma {
			legacy = true
		}
	}
	if legacy {
		return cssLegacyColor(tokens, isHSL)
	}
	return cssModernColor(tokens, isHSL)
}

// stripCSSComments replaces every comment by a space: a comment separates
// two tokens. A comment that is not closed runs to the end.
func stripCSSComments(s string) string {
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i+2:], "*/")
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + " " + s[i+2+j+2:]
	}
}

// cssValidVarReferences reports whether a text with var() is a valid
// variable reference: every var( has a custom property name as its first
// argument, and the parentheses are balanced.
func cssValidVarReferences(body string) bool {
	depth := 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '(':
			depth++
			if i >= 3 && body[i-3:i] == "var" {
				rest := strings.TrimLeft(body[i+1:], " \t\n\r\f")
				if !strings.HasPrefix(rest, "--") || len(rest) < 3 || !cssIdentChar(rest[2]) {
					return false
				}
			}
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		case ';', '!', '{', '}', '[', ']':
			return false
		}
	}
	return depth == 0
}

func cssIdentChar(c byte) bool {
	return c == '_' || c == '-' || c >= 0x80 || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// Kinds of a token of a color function.
const (
	cssNumber = iota
	cssPercentage
	cssAngle
	cssNone
	cssComma
	cssSlash
)

type cssToken struct{ kind int }

// cssColorTokens splits the arguments of a color function into tokens. White
// space is dropped: the grammar of the color functions does not need it
// between two tokens. The result is false when the text has a token that no
// color function accepts.
func cssColorTokens(s string) ([]cssToken, bool) {
	var out []cssToken
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f':
			i++
		case c == ',':
			out = append(out, cssToken{cssComma})
			i++
		case c == '/':
			out = append(out, cssToken{cssSlash})
			i++
		case strings.HasPrefix(s[i:], "none") && (i+4 == len(s) || !cssIdentChar(s[i+4])):
			out = append(out, cssToken{cssNone})
			i += 4
		default:
			n := cssNumberLen(s[i:])
			if n == 0 {
				return nil, false
			}
			i += n
			// The unit: an identifier that follows the number directly.
			j := i
			if cssStartsIdent(s[i:]) {
				for j < len(s) && cssIdentChar(s[j]) {
					j++
				}
			}
			unit := s[i:j]
			switch {
			case unit == "" && i < len(s) && s[i] == '%':
				out = append(out, cssToken{cssPercentage})
				i++
			case unit == "":
				out = append(out, cssToken{cssNumber})
			case unit == "deg" || unit == "grad" || unit == "rad" || unit == "turn":
				out = append(out, cssToken{cssAngle})
				i = j
			default:
				return nil, false
			}
		}
	}
	return out, true
}

// cssStartsIdent reports whether s starts an identifier: a letter, an
// underscore or a character above U+007F, or a hyphen before one of those
// or before a second hyphen. A hyphen before a digit starts a number.
func cssStartsIdent(s string) bool {
	if s == "" {
		return false
	}
	start := func(c byte) bool {
		return c == '_' || c >= 0x80 || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
	}
	if s[0] == '-' {
		return len(s) > 1 && (s[1] == '-' || start(s[1]))
	}
	return start(s[0])
}

// cssNumberLen is the length of the number token at the start of s, or 0.
func cssNumberLen(s string) int {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i+1 < len(s) && s[i] == '.' && s[i+1] >= '0' && s[i+1] <= '9' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0
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
	return i
}

// cssLegacyColor checks the legacy syntax: three components and an optional
// alpha, with commas.
func cssLegacyColor(tokens []cssToken, isHSL bool) bool {
	var parts []int
	for i, t := range tokens {
		if i%2 == 1 {
			if t.kind != cssComma {
				return false
			}
			continue
		}
		parts = append(parts, t.kind)
	}
	if len(tokens)%2 == 0 || (len(parts) != 3 && len(parts) != 4) {
		return false
	}
	if isHSL {
		if parts[0] != cssNumber && parts[0] != cssAngle {
			return false
		}
		if parts[1] != cssPercentage || parts[2] != cssPercentage {
			return false
		}
	} else {
		if parts[0] != cssNumber && parts[0] != cssPercentage {
			return false
		}
		if parts[1] != parts[0] || parts[2] != parts[0] {
			return false
		}
	}
	return len(parts) == 3 || parts[3] == cssNumber || parts[3] == cssPercentage
}

// cssModernColor checks the modern syntax: three components, and an optional
// alpha after a slash.
func cssModernColor(tokens []cssToken, isHSL bool) bool {
	if len(tokens) != 3 && len(tokens) != 5 {
		return false
	}
	for i := 0; i < 3; i++ {
		k := tokens[i].kind
		switch {
		case k == cssNumber || k == cssNone:
		case k == cssPercentage && (!isHSL || i > 0):
		case k == cssAngle && isHSL && i == 0:
		default:
			return false
		}
	}
	if len(tokens) == 5 {
		if tokens[3].kind != cssSlash {
			return false
		}
		k := tokens[4].kind
		return k == cssNumber || k == cssPercentage || k == cssNone
	}
	return true
}
