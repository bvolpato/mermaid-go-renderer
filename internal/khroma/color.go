// Ported from khroma/dist/color/index.js, color/hex.js, color/rgb.js,
// color/hsl.js and color/keyword.js (khroma 2.1.0).

package khroma

import (
	"math"
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Error is an error that khroma throws. TypeError is set when JavaScript
// throws a TypeError, which happens for a color that is not a string.
type Error struct {
	Message   string
	TypeError bool
}

func (e *Error) Error() string { return e.Message }

// jsSpace is the character class that \s matches in a JavaScript regular
// expression. The Go class \s is ASCII only.
const jsSpace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

// The patterns below spell out both letter cases instead of using (?i).
// The Go flag folds 'k' with U+212A and 's' with U+017F. A JavaScript regular
// expression without the u flag does not.
var (
	// Hex.re: /^#((?:[a-f0-9]{2}){2,4}|[a-f0-9]{3})$/i
	hexRe = regexp.MustCompile(`^#((?:[a-fA-F0-9]{2}){2,4}|[a-fA-F0-9]{3})$`)

	// RGB.re
	rgbNum = `-?(?:\d+(?:\.\d+)?|(?:\.\d+))(?:[eE]\d+)?(%?)`
	rgbRe  = regexp.MustCompile(strings.ReplaceAll(
		`^[rR][gG][bB][aA]?\(\s*?(`+rgbNum+`)\s*?(?:,|\s)\s*?(`+rgbNum+`)\s*?(?:,|\s)\s*?(`+rgbNum+`)(?:\s*?(?:,|/)\s*?\+?(`+rgbNum+`))?\s*?\)$`,
		`\s`, jsSpace))

	// HSL.re
	hslNum = `-?(?:\d+(?:\.\d+)?|(?:\.\d+))(?:[eE]-?\d+)?`
	hslRe  = regexp.MustCompile(strings.ReplaceAll(
		`^[hH][sS][lL][aA]?\(\s*?(`+hslNum+`(?:[dD][eE][gG]|[gG][rR][aA][dD]|[rR][aA][dD]|[tT][uU][rR][nN])?)\s*?(?:,|\s)\s*?(`+hslNum+`%)\s*?(?:,|\s)\s*?(`+hslNum+`%)(?:\s*?(?:,|/)\s*?\+?(`+hslNum+`(%)?))?\s*?\)$`,
		`\s`, jsSpace))

	// HSL.hueRe: /^(.+?)(deg|grad|rad|turn)$/i. The dot does not match a
	// line terminator.
	hueRe = regexp.MustCompile(`^([^\n\r\x{2028}\x{2029}]+?)([dD][eE][gG]|[gG][rR][aA][dD]|[rR][aA][dD]|[tT][uU][rR][nN])$`)
)

// parseHex is Hex.parse. The second result is false when the string is not a
// hex color.
func parseHex(color string) (*channels, bool) {
	if color == "" || color[0] != '#' {
		return nil, false
	}
	match := hexRe.FindStringSubmatch(color)
	if match == nil {
		return nil, false
	}
	hex := match[1]
	dec := js.ToInt32(js.ParseInt(hex, 16))
	length := len(hex)
	hasAlpha := length%4 == 0
	isFullLength := length > 4
	multiplier, bits, mask := int32(17), 4, int32(15)
	if isFullLength {
		multiplier, bits, mask = 1, 8, 255
	}
	bitsOffset := -1
	if hasAlpha {
		bitsOffset = 0
	}
	a := 1.0
	if hasAlpha {
		a = float64((dec&mask)*multiplier) / 255
	}
	ch := newRGB(
		float64(((dec>>(bits*(bitsOffset+3)))&mask)*multiplier),
		float64(((dec>>(bits*(bitsOffset+2)))&mask)*multiplier),
		float64(((dec>>(bits*(bitsOffset+1)))&mask)*multiplier),
		a,
	)
	return ch.withColor(color), true
}

// stringifyHex is Hex.stringify.
func stringifyHex(ch *channels) string {
	r, g, b, a := ch.getR(), ch.getG(), ch.getB(), ch.a
	s := "#" + dec2hexTable(js.Round(r)) + dec2hexTable(js.Round(g)) + dec2hexTable(js.Round(b))
	if a < 1 { // #RRGGBBAA
		s += dec2hexTable(js.Round(float64(a * 255)))
	}
	return s
}

// parseRGB is RGB.parse.
func parseRGB(color string) (*channels, bool) {
	if color == "" || (color[0] != 'r' && color[0] != 'R') {
		return nil, false
	}
	m := rgbRe.FindStringSubmatch(color)
	if m == nil {
		return nil, false
	}
	component := func(text, isPercentage string) float64 {
		if isPercentage != "" {
			return float64(js.ParseFloat(text) * 2.55)
		}
		return js.ParseFloat(text)
	}
	a := 1.0
	if m[7] != "" {
		if m[8] != "" {
			a = clampA(js.ParseFloat(m[7]) / 100)
		} else {
			a = clampA(js.ParseFloat(m[7]))
		}
	}
	ch := newRGB(
		clamp255(component(m[1], m[2])),
		clamp255(component(m[3], m[4])),
		clamp255(component(m[5], m[6])),
		a,
	)
	return ch.withColor(color), true
}

// stringifyRGB is RGB.stringify.
func stringifyRGB(ch *channels) string {
	r, g, b, a := ch.getR(), ch.getG(), ch.getB(), ch.a
	if a < 1 { // RGBA
		return "rgba(" + js.Str(langRound(r)) + ", " + js.Str(langRound(g)) + ", " + js.Str(langRound(b)) + ", " + js.Str(langRound(a)) + ")"
	}
	return "rgb(" + js.Str(langRound(r)) + ", " + js.Str(langRound(g)) + ", " + js.Str(langRound(b)) + ")"
}

// hue2deg is HSL._hue2deg.
func hue2deg(hue string) float64 {
	if m := hueRe.FindStringSubmatch(hue); m != nil {
		number, unit := m[1], m[2]
		// The comparison with the unit is case sensitive in khroma. A unit
		// in another case is read as degrees.
		switch unit {
		case "grad":
			return clampH(float64(js.ParseFloat(number) * .9))
		case "rad":
			return clampH(float64(js.ParseFloat(number)*180) / math.Pi)
		case "turn":
			return clampH(float64(js.ParseFloat(number) * 360))
		}
	}
	return clampH(js.ParseFloat(hue))
}

// parseHSL is HSL.parse.
func parseHSL(color string) (*channels, bool) {
	if color == "" || (color[0] != 'h' && color[0] != 'H') {
		return nil, false
	}
	m := hslRe.FindStringSubmatch(color)
	if m == nil {
		return nil, false
	}
	a := 1.0
	if m[4] != "" {
		if m[5] != "" {
			a = clampA(js.ParseFloat(m[4]) / 100)
		} else {
			a = clampA(js.ParseFloat(m[4]))
		}
	}
	ch := newHSL(
		hue2deg(m[1]),
		clamp100(js.ParseFloat(m[2])),
		clamp100(js.ParseFloat(m[3])),
		a,
	)
	return ch.withColor(color), true
}

// stringifyHSL is HSL.stringify.
func stringifyHSL(ch *channels) string {
	h, s, l, a := ch.getH(), ch.getS(), ch.getL(), ch.a
	if a < 1 { // HSLA
		return "hsla(" + js.Str(langRound(h)) + ", " + js.Str(langRound(s)) + "%, " + js.Str(langRound(l)) + "%, " + js.Str(a) + ")"
	}
	return "hsl(" + js.Str(langRound(h)) + ", " + js.Str(langRound(s)) + "%, " + js.Str(langRound(l)) + "%)"
}

// keywordLower is color.toLowerCase() as far as the keyword lookup can see.
// Every keyword is ASCII. The only non-ASCII character that JavaScript lowers
// to an ASCII letter is U+212A (KELVIN SIGN).
func keywordLower(color string) string {
	plain := true
	for i := 0; i < len(color); i++ {
		if c := color[i]; c >= 0x80 || (c >= 'A' && c <= 'Z') {
			plain = false
			break
		}
	}
	if plain {
		return color
	}
	var sb strings.Builder
	for _, r := range color {
		switch {
		case r >= 'A' && r <= 'Z':
			sb.WriteRune(r + 'a' - 'A')
		case r == 0x212a:
			sb.WriteByte('k')
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// parseKeyword is Keyword.parse.
func parseKeyword(color string) (*channels, bool, error) {
	color = keywordLower(color)
	// Keyword.colors is an object literal, so these two names find a member
	// of Object.prototype. Hex.parse then fails on a value that is not a
	// string.
	if color == "constructor" || color == "__proto__" {
		return nil, false, &Error{Message: "color.charCodeAt is not a function", TypeError: true}
	}
	hex, ok := keywordColors[color]
	if !ok {
		return nil, false, nil
	}
	ch, ok := parseHex(hex)
	return ch, ok, nil
}

// stringifyKeyword is Keyword.stringify. The second result is false when no
// keyword has this color.
func stringifyKeyword(ch *channels) (string, bool) {
	hex := stringifyHex(ch)
	for _, name := range keywordNames {
		if keywordColors[name] == hex {
			return name, true
		}
	}
	return "", false
}

// parseColor is Color.parse for a string.
func parseColor(color string) (*channels, error) {
	// Color providers ordered with performance in mind.
	if ch, ok := parseHex(color); ok {
		return ch, nil
	}
	if ch, ok := parseRGB(color); ok {
		return ch, nil
	}
	if ch, ok := parseHSL(color); ok {
		return ch, nil
	}
	ch, ok, err := parseKeyword(color)
	if err != nil {
		return nil, err
	}
	if ok {
		return ch, nil
	}
	return nil, &Error{Message: `Unsupported color format: "` + color + `"`}
}

// stringifyColor is Color.stringify.
func stringifyColor(ch *channels) string {
	// SASS returns a keyword if possible, but khroma avoids doing that.
	if !ch.changed && ch.hasColor {
		return ch.color
	}
	if ch.typ == typeHSL || !ch.hasR {
		return stringifyHSL(ch)
	}
	if ch.a < 1 || !isInteger(ch.getR()) || !isInteger(ch.getG()) || !isInteger(ch.getB()) {
		return stringifyRGB(ch)
	}
	return stringifyHex(ch)
}

// isInteger is Number.isInteger.
func isInteger(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0) && x == math.Trunc(x)
}
