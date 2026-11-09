// Ported from Chromium 156.0.8078.4, in part: the parser and the printer of
// a color follow the parts of Chromium that their comments name:
//
//   - The fast parser of a color: core/css/parser/css_parser_fast_paths.cc
//     of Blink (BSD-3-Clause).
//   - Color::SerializeLegacyColorAsCSSColor: platform/graphics/color.cc of
//     Blink (BSD-2-Clause, with the copyright of Apple in the file).
//   - gfx::HSLToSRGB: ui/gfx/color_conversions.cc (BSD-3-Clause).
//
// The Blink paths are in third_party/blink/renderer. The license of each
// file is the license in its header at that version. See NOTICE in this
// directory. The package comment that follows ports no upstream file.

// Package css parses the CSS that mermaid emits and that users put in
// diagrams: declaration lists, style sheets and property values. It also
// reproduces how Chromium serializes an inline style through the CSSOM.
package css

import (
	"math"
	"strconv"
	"strings"
)

// Color is an sRGB color with straight alpha. R, G and B are 0 to 255.
type Color struct {
	R, G, B uint8
	A       float64 // 0 to 1
}

// Opaque reports whether the alpha is 1.
func (c Color) Opaque() bool { return c.A >= 1 }

var namedColors = map[string]uint32{
	"aliceblue": 0xf0f8ff, "antiquewhite": 0xfaebd7, "aqua": 0x00ffff, "aquamarine": 0x7fffd4, "azure": 0xf0ffff,
	"beige": 0xf5f5dc, "bisque": 0xffe4c4, "black": 0x000000, "blanchedalmond": 0xffebcd, "blue": 0x0000ff,
	"blueviolet": 0x8a2be2, "brown": 0xa52a2a, "burlywood": 0xdeb887, "cadetblue": 0x5f9ea0, "chartreuse": 0x7fff00,
	"chocolate": 0xd2691e, "coral": 0xff7f50, "cornflowerblue": 0x6495ed, "cornsilk": 0xfff8dc, "crimson": 0xdc143c,
	"cyan": 0x00ffff, "darkblue": 0x00008b, "darkcyan": 0x008b8b, "darkgoldenrod": 0xb8860b, "darkgray": 0xa9a9a9,
	"darkgreen": 0x006400, "darkgrey": 0xa9a9a9, "darkkhaki": 0xbdb76b, "darkmagenta": 0x8b008b, "darkolivegreen": 0x556b2f,
	"darkorange": 0xff8c00, "darkorchid": 0x9932cc, "darkred": 0x8b0000, "darksalmon": 0xe9967a, "darkseagreen": 0x8fbc8f,
	"darkslateblue": 0x483d8b, "darkslategray": 0x2f4f4f, "darkslategrey": 0x2f4f4f, "darkturquoise": 0x00ced1, "darkviolet": 0x9400d3,
	"deeppink": 0xff1493, "deepskyblue": 0x00bfff, "dimgray": 0x696969, "dimgrey": 0x696969, "dodgerblue": 0x1e90ff,
	"firebrick": 0xb22222, "floralwhite": 0xfffaf0, "forestgreen": 0x228b22, "fuchsia": 0xff00ff, "gainsboro": 0xdcdcdc,
	"ghostwhite": 0xf8f8ff, "gold": 0xffd700, "goldenrod": 0xdaa520, "gray": 0x808080, "green": 0x008000,
	"greenyellow": 0xadff2f, "grey": 0x808080, "honeydew": 0xf0fff0, "hotpink": 0xff69b4, "indianred": 0xcd5c5c,
	"indigo": 0x4b0082, "ivory": 0xfffff0, "khaki": 0xf0e68c, "lavender": 0xe6e6fa, "lavenderblush": 0xfff0f5,
	"lawngreen": 0x7cfc00, "lemonchiffon": 0xfffacd, "lightblue": 0xadd8e6, "lightcoral": 0xf08080, "lightcyan": 0xe0ffff,
	"lightgoldenrodyellow": 0xfafad2, "lightgray": 0xd3d3d3, "lightgreen": 0x90ee90, "lightgrey": 0xd3d3d3, "lightpink": 0xffb6c1,
	"lightsalmon": 0xffa07a, "lightseagreen": 0x20b2aa, "lightskyblue": 0x87cefa, "lightslategray": 0x778899, "lightslategrey": 0x778899,
	"lightsteelblue": 0xb0c4de, "lightyellow": 0xffffe0, "lime": 0x00ff00, "limegreen": 0x32cd32, "linen": 0xfaf0e6,
	"magenta": 0xff00ff, "maroon": 0x800000, "mediumaquamarine": 0x66cdaa, "mediumblue": 0x0000cd, "mediumorchid": 0xba55d3,
	"mediumpurple": 0x9370db, "mediumseagreen": 0x3cb371, "mediumslateblue": 0x7b68ee, "mediumspringgreen": 0x00fa9a, "mediumturquoise": 0x48d1cc,
	"mediumvioletred": 0xc71585, "midnightblue": 0x191970, "mintcream": 0xf5fffa, "mistyrose": 0xffe4e1, "moccasin": 0xffe4b5,
	"navajowhite": 0xffdead, "navy": 0x000080, "oldlace": 0xfdf5e6, "olive": 0x808000, "olivedrab": 0x6b8e23,
	"orange": 0xffa500, "orangered": 0xff4500, "orchid": 0xda70d6, "palegoldenrod": 0xeee8aa, "palegreen": 0x98fb98,
	"paleturquoise": 0xafeeee, "palevioletred": 0xdb7093, "papayawhip": 0xffefd5, "peachpuff": 0xffdab9, "peru": 0xcd853f,
	"pink": 0xffc0cb, "plum": 0xdda0dd, "powderblue": 0xb0e0e6, "purple": 0x800080, "rebeccapurple": 0x663399,
	"red": 0xff0000, "rosybrown": 0xbc8f8f, "royalblue": 0x4169e1, "saddlebrown": 0x8b4513, "salmon": 0xfa8072,
	"sandybrown": 0xf4a460, "seagreen": 0x2e8b57, "seashell": 0xfff5ee, "sienna": 0xa0522d, "silver": 0xc0c0c0,
	"skyblue": 0x87ceeb, "slateblue": 0x6a5acd, "slategray": 0x708090, "slategrey": 0x708090, "snow": 0xfffafa,
	"springgreen": 0x00ff7f, "steelblue": 0x4682b4, "tan": 0xd2b48c, "teal": 0x008080, "thistle": 0xd8bfd8,
	"tomato": 0xff6347, "turquoise": 0x40e0d0, "violet": 0xee82ee, "wheat": 0xf5deb3, "white": 0xffffff,
	"whitesmoke": 0xf5f5f5, "yellow": 0xffff00, "yellowgreen": 0x9acd32,
}

// IsNamedColor reports whether name is a CSS named color.
func IsNamedColor(name string) bool {
	_, ok := namedColors[strings.ToLower(name)]
	return ok
}

// ParseColor parses a CSS <color>: a name, a hex color, rgb(), rgba(), hsl()
// or hsla(). It reports false for currentcolor and for anything else.
func ParseColor(s string) (Color, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Color{}, false
	}
	lower := strings.ToLower(s)
	if lower == "transparent" {
		return Color{}, true
	}
	if v, ok := namedColors[lower]; ok {
		return Color{uint8(v >> 16), uint8(v >> 8), uint8(v), 1}, true
	}
	if s[0] == '#' {
		return parseHexColor(s[1:])
	}
	open := strings.IndexByte(lower, '(')
	if open < 0 || !strings.HasSuffix(lower, ")") {
		return Color{}, false
	}
	fn := lower[:open]
	args := splitColorArgs(lower[open+1 : len(lower)-1])
	// Chromium reads a color in a plain notation with the fast parser of
	// css_parser_fast_paths.cc: a function name in lower case, and numbers
	// that are digits with an optional minus sign and fraction. That parser
	// uses at most 7 digits of a fraction. TestColorAgainstChromium has the
	// cases.
	fast := s[:open] == fn
	for _, arg := range args {
		fast = fast && isPlainNumber(strings.TrimSuffix(arg, "%"))
	}
	switch fn {
	case "rgb", "rgba":
		if len(args) != 3 && len(args) != 4 {
			return Color{}, false
		}
		var ch [3]uint8
		for i := 0; i < 3; i++ {
			v, pct, ok := parseNumPct(args[i], fast)
			if !ok {
				return Color{}, false
			}
			if pct {
				v = v / 100 * 255
			}
			ch[i] = clampByte(v)
		}
		a := 1.0
		if len(args) == 4 {
			var ok bool
			if a, ok = parseAlpha(args[3], fast); !ok {
				return Color{}, false
			}
		}
		return Color{ch[0], ch[1], ch[2], a}, true
	case "hsl", "hsla":
		if len(args) != 3 && len(args) != 4 {
			return Color{}, false
		}
		h, ok := parseHue(args[0], fast)
		if !ok {
			return Color{}, false
		}
		sat, satPct, ok1 := parseNumPct(args[1], fast)
		lig, ligPct, ok2 := parseNumPct(args[2], fast)
		if !ok1 || !ok2 {
			return Color{}, false
		}
		// The notation with commas needs percentages.
		if strings.Contains(lower, ",") && !(satPct && ligPct) {
			return Color{}, false
		}
		a := 1.0
		if len(args) == 4 {
			if a, ok = parseAlpha(args[3], fast); !ok {
				return Color{}, false
			}
		}
		r, g, b := hslToRGB(h, clamp01(sat/100), clamp01(lig/100))
		return Color{r, g, b, a}, true
	}
	return Color{}, false
}

// isPlainNumber reports whether s is a number that the fast color parser of
// Chromium reads: digits with an optional minus sign and an optional
// fraction.
func isPlainNumber(s string) bool {
	s = strings.TrimPrefix(s, "-")
	digits := 0
	dot := false
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] >= '0' && s[i] <= '9':
			digits++
		case s[i] == '.' && !dot:
			dot = true
		default:
			return false
		}
	}
	return digits > 0
}

// parseColorNumber parses a number of a color function. fast selects
// ParseDouble of css_parser_fast_paths.cc for a plain number: the integer
// part, plus at most 7 digits of the fraction divided by their scale.
func parseColorNumber(s string, fast bool) (float64, error) {
	if !fast {
		return strconv.ParseFloat(s, 64)
	}
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	value := 0.0
	i := 0
	for ; i < len(s) && s[i] != '.'; i++ {
		value = float64(value*10) + float64(s[i]-'0')
	}
	if i < len(s) {
		fraction := 0.0
		scale := 1.0
		const maxScale = 10000000
		for i++; i < len(s) && scale < maxScale; i++ {
			fraction = float64(fraction*10) + float64(s[i]-'0')
			scale *= 10
		}
		value += fraction / scale
	}
	if negative {
		value = -value
	}
	return value, nil
}

func parseHexColor(h string) (Color, bool) {
	for i := 0; i < len(h); i++ {
		c := h[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return Color{}, false
		}
	}
	hex := func(s string) uint8 {
		v, _ := strconv.ParseUint(s, 16, 8)
		return uint8(v)
	}
	switch len(h) {
	case 3, 4:
		c := Color{hex(h[0:1]) * 17, hex(h[1:2]) * 17, hex(h[2:3]) * 17, 1}
		if len(h) == 4 {
			c.A = float64(hex(h[3:4])*17) / 255
		}
		return c, true
	case 6, 8:
		c := Color{hex(h[0:2]), hex(h[2:4]), hex(h[4:6]), 1}
		if len(h) == 8 {
			c.A = float64(hex(h[6:8])) / 255
		}
		return c, true
	}
	return Color{}, false
}

// splitColorArgs splits "a, b, c" and the modern "a b c / d" form.
func splitColorArgs(s string) []string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	alpha := ""
	if i := strings.IndexByte(s, '/'); i >= 0 {
		alpha = strings.TrimSpace(s[i+1:])
		s = s[:i]
	}
	parts := strings.Fields(s)
	if alpha != "" {
		parts = append(parts, alpha)
	}
	return parts
}

func parseNumPct(s string, fast bool) (v float64, pct bool, ok bool) {
	if strings.HasSuffix(s, "%") {
		pct = true
		s = s[:len(s)-1]
	}
	if s == "none" {
		return 0, pct, true
	}
	v, err := parseColorNumber(s, fast)
	return v, pct, err == nil
}

// parseAlpha parses the alpha value of rgb() or hsl(). Chromium keeps the
// alpha of these colors as 8 bits.
func parseAlpha(s string, fast bool) (float64, bool) {
	v, pct, ok := parseNumPct(s, fast)
	if !ok {
		return 0, false
	}
	if pct {
		v /= 100
	}
	return math.Round(clamp01(v)*255) / 255, true
}

func parseHue(s string, fast bool) (float64, bool) {
	mul := 1.0
	switch {
	case strings.HasSuffix(s, "deg"):
		s = s[:len(s)-3]
	case strings.HasSuffix(s, "grad"):
		s, mul = s[:len(s)-4], 0.9
	case strings.HasSuffix(s, "rad"):
		s, mul = s[:len(s)-3], 180/math.Pi
	case strings.HasSuffix(s, "turn"):
		s, mul = s[:len(s)-4], 360
	}
	if s == "none" {
		return 0, true
	}
	v, err := parseColorNumber(s, fast && mul == 1)
	return float64(v * mul), err == nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampByte(v float64) uint8 {
	if math.IsNaN(v) || v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(math.Round(v))
}

// hslToRGB follows the CSS Color algorithm with the arithmetic of Blink. The
// color is a triple of 32-bit floats, and gfx::HSLToSRGB computes in 32-bit
// floats. Color::SerializeLegacyColorAsCSSColor then adds 1e-07 to each
// channel, so that a channel in the middle between two 8-bit values rounds
// up, and multiplies by 255. TestColorAgainstChromium has the cases.
func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	h32, s32, l32 := float32(h), float32(s), float32(l)
	f := func(n float32) float32 {
		if s32 == 0 {
			return l32
		}
		k := float32(math.Mod(float64(float32(n+float32(h32/30))), 12))
		a := float32(s32 * min(l32, float32(1-l32)))
		return float32(l32 - float32(a*max(-1, min(float32(k-3), float32(9-k), 1))))
	}
	channel := func(v float32) uint8 {
		v = float32(v + 1e-07)
		return clampByte(float64(float32(v * 255)))
	}
	return channel(f(0)), channel(f(8)), channel(f(4))
}

// CSSText serializes the color as Chromium does for a computed or
// specified sRGB color: rgb(r, g, b) or rgba(r, g, b, a).
func (c Color) CSSText() string {
	if c.A >= 1 {
		return "rgb(" + strconv.Itoa(int(c.R)) + ", " + strconv.Itoa(int(c.G)) + ", " + strconv.Itoa(int(c.B)) + ")"
	}
	return "rgba(" + strconv.Itoa(int(c.R)) + ", " + strconv.Itoa(int(c.G)) + ", " + strconv.Itoa(int(c.B)) + ", " + formatAlpha(c.A) + ")"
}

// formatAlpha prints an alpha value as Blink does: the shortest decimal with
// at most 3 digits that round-trips to the same 8-bit alpha, else 2 digits.
func formatAlpha(a float64) string {
	if a <= 0 {
		return "0"
	}
	alpha8 := math.Round(a * 255)
	rounded := math.Round(a*100) / 100
	if math.Round(rounded*255) == alpha8 {
		return strconv.FormatFloat(rounded, 'f', -1, 64)
	}
	return strconv.FormatFloat(math.Round(a*1000)/1000, 'f', -1, 64)
}
