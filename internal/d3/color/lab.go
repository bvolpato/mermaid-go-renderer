// Ported from d3-color 3.1.0, d3-color/src/lab.js (ISC).

package color

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
)

// The reference white and the constants of the CIELAB conversion. They are
// variables so that Go computes them with float64 arithmetic, as JavaScript
// does. See https://observablehq.com/@mbostock/lab-and-rgb.
var (
	labK float64 = 18
	xn   float64 = 0.96422
	yn   float64 = 1
	zn   float64 = 0.82521
	t0   float64 = 4.0 / 29
	t1   float64 = 6.0 / 29
	t2           = float64(3 * t1 * t1)
	t3           = float64(t1 * t1 * t1)

	one     float64 = 1
	gamma24 float64 = 2.4
	third           = one / 3
	inv24           = one / gamma24
)

// Lab is a color in the CIELAB color space (d3.lab).
type Lab struct {
	L, A, B, Opacity float64
}

// LabFrom is d3.lab(color) for a color object (labConvert).
func LabFrom(c Color) Lab {
	switch o := c.(type) {
	case Lab:
		return o
	case HCL:
		return hcl2lab(o)
	}
	o := RGBFrom(c)
	r := rgb2lrgb(o.R)
	g := rgb2lrgb(o.G)
	b := rgb2lrgb(o.B)
	y := xyz2lab((float64(0.2225045*r) + float64(0.7168786*g) + float64(0.0606169*b)) / yn)
	var x, z float64
	if r == g && g == b {
		x, z = y, y
	} else {
		x = xyz2lab((float64(0.4360747*r) + float64(0.3850649*g) + float64(0.1430804*b)) / xn)
		z = xyz2lab((float64(0.0139322*r) + float64(0.0971045*g) + float64(0.7141733*b)) / zn)
	}
	return Lab{float64(116*y) - 16, float64(500 * (x - y)), float64(200 * (y - z)), o.Opacity}
}

// ParseLab is d3.lab(specifier) for a string.
func ParseLab(specifier string) Lab {
	return LabFrom(Parse(specifier))
}

// Gray is d3.gray(l, opacity).
func Gray(l, opacity float64) Lab {
	return Lab{l, 0, 0, opacity}
}

// labFactor is K * (k == null ? 1 : k).
func labFactor(k []float64) float64 {
	if len(k) == 0 {
		return float64(labK * 1)
	}
	return float64(labK * k[0])
}

// Brighter is lab.brighter(k). Without an argument, k is 1.
func (c Lab) Brighter(k ...float64) Lab {
	return Lab{c.L + labFactor(k), c.A, c.B, c.Opacity}
}

// Darker is lab.darker(k). Without an argument, k is 1.
func (c Lab) Darker(k ...float64) Lab {
	return Lab{c.L - labFactor(k), c.A, c.B, c.Opacity}
}

// RGB is lab.rgb().
func (c Lab) RGB() RGB {
	y := (c.L + 16) / 116
	x := y
	if !math.IsNaN(c.A) {
		x = y + c.A/500
	}
	z := y
	if !math.IsNaN(c.B) {
		z = y - c.B/200
	}
	x = float64(xn * lab2xyz(x))
	y = float64(yn * lab2xyz(y))
	z = float64(zn * lab2xyz(z))
	return RGB{
		lrgb2rgb(float64(3.1338561*x) - float64(1.6168667*y) - float64(0.4906146*z)),
		lrgb2rgb(float64(-0.9787684*x) + float64(1.9161415*y) + float64(0.0334540*z)),
		lrgb2rgb(float64(0.0719453*x) - float64(0.2289914*y) + float64(1.4052427*z)),
		c.Opacity,
	}
}

// Displayable is color.displayable().
func (c Lab) Displayable() bool { return c.RGB().Displayable() }

// FormatHex is color.formatHex().
func (c Lab) FormatHex() string { return c.RGB().FormatHex() }

// FormatHex8 is color.formatHex8().
func (c Lab) FormatHex8() string { return c.RGB().FormatHex8() }

// FormatHsl is color.formatHsl().
func (c Lab) FormatHsl() string { return HSLFrom(c).FormatHsl() }

// FormatRgb is color.formatRgb().
func (c Lab) FormatRgb() string { return c.RGB().FormatRgb() }

// String is color.toString().
func (c Lab) String() string { return c.RGB().FormatRgb() }

func xyz2lab(t float64) float64 {
	if t > t3 {
		return jsm.Pow(t, third)
	}
	return t/t2 + t0
}

func lab2xyz(t float64) float64 {
	if t > t1 {
		return float64(t * t * t)
	}
	return float64(t2 * (t - t0))
}

func lrgb2rgb(x float64) float64 {
	if x <= 0.0031308 {
		return float64(255 * float64(12.92*x))
	}
	return float64(255 * (float64(1.055*jsm.Pow(x, inv24)) - 0.055))
}

func rgb2lrgb(x float64) float64 {
	x /= 255
	if x <= 0.04045 {
		return x / 12.92
	}
	return jsm.Pow((x+0.055)/1.055, gamma24)
}

// HCL is a color in the CIELCh color space (d3.hcl and d3.lch).
type HCL struct {
	H, C, L, Opacity float64
}

// HCLFrom is d3.hcl(color) for a color object (hclConvert).
func HCLFrom(c Color) HCL {
	if o, ok := c.(HCL); ok {
		return o
	}
	o := LabFrom(c)
	if o.A == 0 && o.B == 0 {
		chroma := math.NaN()
		if 0 < o.L && o.L < 100 {
			chroma = 0
		}
		return HCL{math.NaN(), chroma, o.L, o.Opacity}
	}
	h := float64(jsm.Atan2(o.B, o.A) * degrees)
	if h < 0 {
		h += 360
	}
	return HCL{h, math.Sqrt(float64(o.A*o.A) + float64(o.B*o.B)), o.L, o.Opacity}
}

// ParseHCL is d3.hcl(specifier) for a string.
func ParseHCL(specifier string) HCL {
	return HCLFrom(Parse(specifier))
}

// LCH is d3.lch(l, c, h, opacity).
func LCH(l, c, h, opacity float64) HCL {
	return HCL{h, c, l, opacity}
}

func hcl2lab(o HCL) Lab {
	if math.IsNaN(o.H) {
		return Lab{o.L, 0, 0, o.Opacity}
	}
	h := float64(o.H * radians)
	return Lab{o.L, float64(jsm.Cos(h) * o.C), float64(jsm.Sin(h) * o.C), o.Opacity}
}

// Brighter is hcl.brighter(k). Without an argument, k is 1.
func (c HCL) Brighter(k ...float64) HCL {
	return HCL{c.H, c.C, c.L + labFactor(k), c.Opacity}
}

// Darker is hcl.darker(k). Without an argument, k is 1.
func (c HCL) Darker(k ...float64) HCL {
	return HCL{c.H, c.C, c.L - labFactor(k), c.Opacity}
}

// RGB is hcl.rgb().
func (c HCL) RGB() RGB { return hcl2lab(c).RGB() }

// Displayable is color.displayable().
func (c HCL) Displayable() bool { return c.RGB().Displayable() }

// FormatHex is color.formatHex().
func (c HCL) FormatHex() string { return c.RGB().FormatHex() }

// FormatHex8 is color.formatHex8().
func (c HCL) FormatHex8() string { return c.RGB().FormatHex8() }

// FormatHsl is color.formatHsl().
func (c HCL) FormatHsl() string { return HSLFrom(c).FormatHsl() }

// FormatRgb is color.formatRgb().
func (c HCL) FormatRgb() string { return c.RGB().FormatRgb() }

// String is color.toString().
func (c HCL) String() string { return c.RGB().FormatRgb() }
