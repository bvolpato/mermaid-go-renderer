// Ported from d3-color 3.1.0, d3-color/src/cubehelix.js (ISC).

package color

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
)

// The constants of the cubehelix conversion. They are variables so that Go
// computes the products with float64 arithmetic, as JavaScript does.
var (
	chA    float64 = -0.14861
	chB    float64 = +1.78277
	chC    float64 = -0.29227
	chD    float64 = -0.90649
	chE    float64 = +1.97294
	chED           = float64(chE * chD)
	chEB           = float64(chE * chB)
	chBCDA         = float64(chB*chC) - float64(chD*chA)
)

// Cubehelix is a color in the cubehelix color space (d3.cubehelix).
type Cubehelix struct {
	H, S, L, Opacity float64
}

// CubehelixFrom is d3.cubehelix(color) for a color object (cubehelixConvert).
func CubehelixFrom(c Color) Cubehelix {
	if o, ok := c.(Cubehelix); ok {
		return o
	}
	o := RGBFrom(c)
	r := o.R / 255
	g := o.G / 255
	b := o.B / 255
	l := (float64(chBCDA*b) + float64(chED*r) - float64(chEB*g)) / (chBCDA + chED - chEB)
	bl := b - l
	k := (float64(chE*(g-l)) - float64(chC*bl)) / chD
	s := math.Sqrt(float64(k*k)+float64(bl*bl)) / float64(chE*l*(1-l)) // NaN if l=0 or l=1
	h := math.NaN()
	if s != 0 && !math.IsNaN(s) {
		h = float64(jsm.Atan2(k, bl)*degrees) - 120
	}
	if h < 0 {
		h += 360
	}
	return Cubehelix{h, s, l, o.Opacity}
}

// ParseCubehelix is d3.cubehelix(specifier) for a string.
func ParseCubehelix(specifier string) Cubehelix {
	return CubehelixFrom(Parse(specifier))
}

// Brighter is cubehelix.brighter(k). Without an argument, k is 1.
func (c Cubehelix) Brighter(k ...float64) Cubehelix {
	f := factor(brighter, k)
	return Cubehelix{c.H, c.S, float64(c.L * f), c.Opacity}
}

// Darker is cubehelix.darker(k). Without an argument, k is 1.
func (c Cubehelix) Darker(k ...float64) Cubehelix {
	f := factor(darker, k)
	return Cubehelix{c.H, c.S, float64(c.L * f), c.Opacity}
}

// RGB is cubehelix.rgb().
func (c Cubehelix) RGB() RGB {
	h := 0.0
	if !math.IsNaN(c.H) {
		h = float64((c.H + 120) * radians)
	}
	l := c.L
	a := 0.0
	if !math.IsNaN(c.S) {
		a = float64(c.S * l * (1 - l))
	}
	cosh := jsm.Cos(h)
	sinh := jsm.Sin(h)
	return RGB{
		float64(255 * (l + float64(a*(float64(chA*cosh)+float64(chB*sinh))))),
		float64(255 * (l + float64(a*(float64(chC*cosh)+float64(chD*sinh))))),
		float64(255 * (l + float64(a*float64(chE*cosh)))),
		c.Opacity,
	}
}

// Displayable is color.displayable().
func (c Cubehelix) Displayable() bool { return c.RGB().Displayable() }

// FormatHex is color.formatHex().
func (c Cubehelix) FormatHex() string { return c.RGB().FormatHex() }

// FormatHex8 is color.formatHex8().
func (c Cubehelix) FormatHex8() string { return c.RGB().FormatHex8() }

// FormatHsl is color.formatHsl().
func (c Cubehelix) FormatHsl() string { return HSLFrom(c).FormatHsl() }

// FormatRgb is color.formatRgb().
func (c Cubehelix) FormatRgb() string { return c.RGB().FormatRgb() }

// String is color.toString().
func (c Cubehelix) String() string { return c.RGB().FormatRgb() }
