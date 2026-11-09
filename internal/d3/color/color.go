// Ported from d3-color 3.1.0, d3-color/src/color.js and d3-color/src/math.js
// (ISC).

// Package color is a port of d3-color 3.1.0.
package color

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Color is a color in one of the color spaces of d3-color: RGB, HSL, Lab,
// HCL or Cubehelix. A nil Color is the null that d3.color returns for a
// string that is not a color.
type Color interface {
	// RGB is color.rgb().
	RGB() RGB
	// Displayable is color.displayable().
	Displayable() bool
	// FormatHex is color.formatHex().
	FormatHex() string
	// FormatHex8 is color.formatHex8().
	FormatHex8() string
	// FormatHsl is color.formatHsl().
	FormatHsl() string
	// FormatRgb is color.formatRgb().
	FormatRgb() string
	// String is color.toString(), which is formatRgb.
	String() string
}

// These are variables, not constants, so that Go computes with float64
// values as JavaScript does.
var (
	darker   float64 = 0.7
	brighter         = 1 / darker

	pi      float64 = math.Pi
	radians         = pi / 180
	degrees         = 180 / pi
)

// factor is the k of brighter(k) and darker(k). No argument is the null of d3.
func factor(base float64, k []float64) float64 {
	if len(k) == 0 {
		return base
	}
	return jsm.Pow(base, k[0])
}

// The white space of a JavaScript regular expression (\s).
const ws = `[\t\n\x0b\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]*`

const (
	reI = ws + `([+-]?\d+)` + ws
	reN = ws + `([+-]?(?:\d*\.)?\d+(?:[eE][+-]?\d+)?)` + ws
	reP = ws + `([+-]?(?:\d*\.)?\d+(?:[eE][+-]?\d+)?)%` + ws
)

var (
	reHex         = regexp.MustCompile(`^#([0-9a-f]{3,8})$`)
	reRgbInteger  = regexp.MustCompile(`^rgb\(` + reI + `,` + reI + `,` + reI + `\)$`)
	reRgbPercent  = regexp.MustCompile(`^rgb\(` + reP + `,` + reP + `,` + reP + `\)$`)
	reRgbaInteger = regexp.MustCompile(`^rgba\(` + reI + `,` + reI + `,` + reI + `,` + reN + `\)$`)
	reRgbaPercent = regexp.MustCompile(`^rgba\(` + reP + `,` + reP + `,` + reP + `,` + reN + `\)$`)
	reHslPercent  = regexp.MustCompile(`^hsl\(` + reN + `,` + reP + `,` + reP + `\)$`)
	reHslaPercent = regexp.MustCompile(`^hsla\(` + reN + `,` + reP + `,` + reP + `,` + reN + `\)$`)
)

// toLowerCase is String.prototype.toLowerCase for the purpose of color
// names. JavaScript maps U+0130 to "i" and a combining dot; Go maps it to "i".
func toLowerCase(s string) string {
	if strings.ContainsRune(s, 0x130) {
		s = strings.ReplaceAll(s, "İ", "i̇")
	}
	return strings.ToLower(s)
}

// Parse is d3.color(specifier). It returns nil for a string that is not a
// color.
func Parse(format string) Color {
	format = toLowerCase(js.TrimSpace(format))
	if m := reHex.FindStringSubmatch(format); m != nil {
		l := len(m[1])
		v, _ := strconv.ParseUint(m[1], 16, 64)
		n := js.ToInt32(float64(v))
		switch l {
		case 6: // #ff0000
			return rgbn(n)
		case 3: // #f00
			return RGB{float64((n >> 8 & 0xf) | (n >> 4 & 0xf0)), float64((n >> 4 & 0xf) | (n & 0xf0)), float64(((n & 0xf) << 4) | (n & 0xf)), 1}
		case 8: // #ff000000
			return rgba(float64(n>>24&0xff), float64(n>>16&0xff), float64(n>>8&0xff), float64(n&0xff)/0xff)
		case 4: // #f000
			return rgba(float64((n>>12&0xf)|(n>>8&0xf0)), float64((n>>8&0xf)|(n>>4&0xf0)), float64((n>>4&0xf)|(n&0xf0)), float64(((n&0xf)<<4)|(n&0xf))/0xff)
		}
		return nil // invalid hex
	}
	if m := reRgbInteger.FindStringSubmatch(format); m != nil { // rgb(255, 0, 0)
		return RGB{js.Number(m[1]), js.Number(m[2]), js.Number(m[3]), 1}
	}
	if m := reRgbPercent.FindStringSubmatch(format); m != nil { // rgb(100%, 0%, 0%)
		return RGB{js.Number(m[1]) * 255 / 100, js.Number(m[2]) * 255 / 100, js.Number(m[3]) * 255 / 100, 1}
	}
	if m := reRgbaInteger.FindStringSubmatch(format); m != nil { // rgba(255, 0, 0, 1)
		return rgba(js.Number(m[1]), js.Number(m[2]), js.Number(m[3]), js.Number(m[4]))
	}
	if m := reRgbaPercent.FindStringSubmatch(format); m != nil { // rgb(100%, 0%, 0%, 1)
		return rgba(js.Number(m[1])*255/100, js.Number(m[2])*255/100, js.Number(m[3])*255/100, js.Number(m[4]))
	}
	if m := reHslPercent.FindStringSubmatch(format); m != nil { // hsl(120, 50%, 50%)
		return hsla(js.Number(m[1]), js.Number(m[2])/100, js.Number(m[3])/100, 1)
	}
	if m := reHslaPercent.FindStringSubmatch(format); m != nil { // hsla(120, 50%, 50%, 1)
		return hsla(js.Number(m[1]), js.Number(m[2])/100, js.Number(m[3])/100, js.Number(m[4]))
	}
	if n, ok := named[format]; ok {
		return rgbn(n)
	}
	if format == "transparent" {
		return RGB{math.NaN(), math.NaN(), math.NaN(), 0}
	}
	return nil
}

func rgbn(n int32) RGB {
	return RGB{float64(n >> 16 & 0xff), float64(n >> 8 & 0xff), float64(n & 0xff), 1}
}

func rgba(r, g, b, a float64) RGB {
	if a <= 0 {
		r, g, b = math.NaN(), math.NaN(), math.NaN()
	}
	return RGB{r, g, b, a}
}

// RGB is a color in the RGB color space (d3.rgb). The channels are 0 to 255.
type RGB struct {
	R, G, B, Opacity float64
}

var nanRGB = RGB{math.NaN(), math.NaN(), math.NaN(), math.NaN()}

// RGBFrom is d3.rgb(color) for a color object (rgbConvert). A nil color
// gives NaN channels.
func RGBFrom(c Color) RGB {
	if c == nil {
		return nanRGB
	}
	return c.RGB()
}

// ParseRGB is d3.rgb(specifier) for a string.
func ParseRGB(specifier string) RGB {
	return RGBFrom(Parse(specifier))
}

// Brighter is rgb.brighter(k). Without an argument, k is 1.
func (c RGB) Brighter(k ...float64) RGB {
	f := factor(brighter, k)
	return RGB{float64(c.R * f), float64(c.G * f), float64(c.B * f), c.Opacity}
}

// Darker is rgb.darker(k). Without an argument, k is 1.
func (c RGB) Darker(k ...float64) RGB {
	f := factor(darker, k)
	return RGB{float64(c.R * f), float64(c.G * f), float64(c.B * f), c.Opacity}
}

// RGB is rgb.rgb().
func (c RGB) RGB() RGB { return c }

// Clamp is rgb.clamp().
func (c RGB) Clamp() RGB {
	return RGB{clampi(c.R), clampi(c.G), clampi(c.B), clampa(c.Opacity)}
}

// Displayable is rgb.displayable().
func (c RGB) Displayable() bool {
	return (-0.5 <= c.R && c.R < 255.5) &&
		(-0.5 <= c.G && c.G < 255.5) &&
		(-0.5 <= c.B && c.B < 255.5) &&
		(0 <= c.Opacity && c.Opacity <= 1)
}

// FormatHex is rgb.formatHex().
func (c RGB) FormatHex() string {
	return "#" + hex(c.R) + hex(c.G) + hex(c.B)
}

// FormatHex8 is rgb.formatHex8().
func (c RGB) FormatHex8() string {
	o := c.Opacity
	if math.IsNaN(o) {
		o = 1
	}
	return "#" + hex(c.R) + hex(c.G) + hex(c.B) + hex(float64(o*255))
}

// FormatRgb is rgb.formatRgb().
func (c RGB) FormatRgb() string {
	a := clampa(c.Opacity)
	var sb strings.Builder
	if a == 1 {
		sb.WriteString("rgb(")
	} else {
		sb.WriteString("rgba(")
	}
	sb.WriteString(js.Str(clampi(c.R)))
	sb.WriteString(", ")
	sb.WriteString(js.Str(clampi(c.G)))
	sb.WriteString(", ")
	sb.WriteString(js.Str(clampi(c.B)))
	if a == 1 {
		sb.WriteString(")")
	} else {
		sb.WriteString(", ")
		sb.WriteString(js.Str(a))
		sb.WriteString(")")
	}
	return sb.String()
}

// FormatHsl is color.formatHsl().
func (c RGB) FormatHsl() string { return HSLFrom(c).FormatHsl() }

// String is rgb.toString().
func (c RGB) String() string { return c.FormatRgb() }

func clampa(opacity float64) float64 {
	if math.IsNaN(opacity) {
		return 1
	}
	return js.Max(0, js.Min(1, opacity))
}

// orZero is the JavaScript expression value || 0.
func orZero(value float64) float64 {
	if value == 0 || math.IsNaN(value) {
		return 0
	}
	return value
}

func clampi(value float64) float64 {
	return js.Max(0, js.Min(255, orZero(js.Round(value))))
}

func hex(value float64) string {
	value = clampi(value)
	s := strconv.FormatInt(int64(value), 16)
	if value < 16 {
		return "0" + s
	}
	return s
}

func hsla(h, s, l, a float64) HSL {
	if a <= 0 {
		h, s, l = math.NaN(), math.NaN(), math.NaN()
	} else if l <= 0 || l >= 1 {
		h, s = math.NaN(), math.NaN()
	} else if s <= 0 {
		h = math.NaN()
	}
	return HSL{h, s, l, a}
}

// HSL is a color in the HSL color space (d3.hsl).
type HSL struct {
	H, S, L, Opacity float64
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// HSLFrom is d3.hsl(color) for a color object (hslConvert). A nil color gives
// NaN channels.
func HSLFrom(c Color) HSL {
	if c == nil {
		return HSL{math.NaN(), math.NaN(), math.NaN(), math.NaN()}
	}
	if o, ok := c.(HSL); ok {
		return o
	}
	o := c.RGB()
	r := o.R / 255
	g := o.G / 255
	b := o.B / 255
	min := js.Min(r, g, b)
	max := js.Max(r, g, b)
	h := math.NaN()
	s := max - min
	l := (max + min) / 2
	if s != 0 && !math.IsNaN(s) {
		if r == max {
			h = (g-b)/s + float64(b2f(g < b)*6)
		} else if g == max {
			h = (b-r)/s + 2
		} else {
			h = (r-g)/s + 4
		}
		if l < 0.5 {
			s /= max + min
		} else {
			s /= 2 - max - min
		}
		h = float64(h * 60)
	} else {
		if l > 0 && l < 1 {
			s = 0
		} else {
			s = h
		}
	}
	return HSL{h, s, l, o.Opacity}
}

// ParseHSL is d3.hsl(specifier) for a string.
func ParseHSL(specifier string) HSL {
	return HSLFrom(Parse(specifier))
}

// Brighter is hsl.brighter(k). Without an argument, k is 1.
func (c HSL) Brighter(k ...float64) HSL {
	f := factor(brighter, k)
	return HSL{c.H, c.S, float64(c.L * f), c.Opacity}
}

// Darker is hsl.darker(k). Without an argument, k is 1.
func (c HSL) Darker(k ...float64) HSL {
	f := factor(darker, k)
	return HSL{c.H, c.S, float64(c.L * f), c.Opacity}
}

// RGB is hsl.rgb().
func (c HSL) RGB() RGB {
	h := math.Mod(c.H, 360) + float64(b2f(c.H < 0)*360)
	s := c.S
	if math.IsNaN(h) || math.IsNaN(c.S) {
		s = 0
	}
	l := c.L
	var m2 float64
	if l < 0.5 {
		m2 = l + float64(l*s)
	} else {
		m2 = l + float64((1-l)*s)
	}
	m1 := float64(2*l) - m2
	var hr, hb float64
	if h >= 240 {
		hr = h - 240
	} else {
		hr = h + 120
	}
	if h < 120 {
		hb = h + 240
	} else {
		hb = h - 120
	}
	return RGB{hsl2rgb(hr, m1, m2), hsl2rgb(h, m1, m2), hsl2rgb(hb, m1, m2), c.Opacity}
}

// Clamp is hsl.clamp().
func (c HSL) Clamp() HSL {
	return HSL{clamph(c.H), clampt(c.S), clampt(c.L), clampa(c.Opacity)}
}

// Displayable is hsl.displayable().
func (c HSL) Displayable() bool {
	return (0 <= c.S && c.S <= 1 || math.IsNaN(c.S)) &&
		(0 <= c.L && c.L <= 1) &&
		(0 <= c.Opacity && c.Opacity <= 1)
}

// FormatHsl is hsl.formatHsl().
func (c HSL) FormatHsl() string {
	a := clampa(c.Opacity)
	var sb strings.Builder
	if a == 1 {
		sb.WriteString("hsl(")
	} else {
		sb.WriteString("hsla(")
	}
	sb.WriteString(js.Str(clamph(c.H)))
	sb.WriteString(", ")
	sb.WriteString(js.Str(float64(clampt(c.S) * 100)))
	sb.WriteString("%, ")
	sb.WriteString(js.Str(float64(clampt(c.L) * 100)))
	sb.WriteString("%")
	if a == 1 {
		sb.WriteString(")")
	} else {
		sb.WriteString(", ")
		sb.WriteString(js.Str(a))
		sb.WriteString(")")
	}
	return sb.String()
}

// FormatHex is color.formatHex().
func (c HSL) FormatHex() string { return c.RGB().FormatHex() }

// FormatHex8 is color.formatHex8().
func (c HSL) FormatHex8() string { return c.RGB().FormatHex8() }

// FormatRgb is color.formatRgb().
func (c HSL) FormatRgb() string { return c.RGB().FormatRgb() }

// String is color.toString().
func (c HSL) String() string { return c.RGB().FormatRgb() }

func clamph(value float64) float64 {
	value = math.Mod(orZero(value), 360)
	if value < 0 {
		return value + 360
	}
	return value
}

func clampt(value float64) float64 {
	return js.Max(0, js.Min(1, orZero(value)))
}

// hsl2rgb is from FvD 13.37, CSS Color Module Level 3.
func hsl2rgb(h, m1, m2 float64) float64 {
	var v float64
	switch {
	case h < 60:
		v = m1 + (m2-m1)*h/60
	case h < 180:
		v = m2
	case h < 240:
		v = m1 + (m2-m1)*(240-h)/60
	default:
		v = m1
	}
	return float64(v * 255)
}
