// Ported from d3-interpolate 3.0.1, d3-interpolate/src: number.js, round.js,
// color.js, rgb.js, hsl.js, hcl.js, lab.js, cubehelix.js, hue.js, date.js,
// discrete.js, quantize.js, piecewise.js, basis.js and basisClosed.js (ISC).

// Package interpolate is a port of the parts of d3-interpolate 3.0.1 that the
// d3 scales and mermaid use.
package interpolate

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/color"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Number is d3.interpolateNumber(a, b).
func Number(a, b float64) func(t float64) float64 {
	return func(t float64) float64 {
		return float64(a*(1-t)) + float64(b*t)
	}
}

// Round is d3.interpolateRound(a, b).
func Round(a, b float64) func(t float64) float64 {
	return func(t float64) float64 {
		return js.Round(float64(a*(1-t)) + float64(b*t))
	}
}

// Date is d3.interpolateDate(a, b). The arguments and the result are time
// values in milliseconds, clipped as the Date object clips them.
func Date(a, b float64) func(t float64) float64 {
	return func(t float64) float64 {
		return js.TimeClip(float64(a*(1-t)) + float64(b*t))
	}
}

func constant(x float64) func(t float64) float64 {
	return func(float64) float64 { return x }
}

func linear(a, d float64) func(t float64) float64 {
	return func(t float64) float64 {
		return a + float64(t*d)
	}
}

func exponential(a, b, y float64) func(t float64) float64 {
	a = jsm.Pow(a, y)
	b = jsm.Pow(b, y) - a
	y = 1 / y
	return func(t float64) float64 {
		return jsm.Pow(a+float64(t*b), y)
	}
}

func constantOf(a, b float64) func(t float64) float64 {
	if math.IsNaN(a) {
		return constant(b)
	}
	return constant(a)
}

// hue is the hue function of color.js: it takes the shorter way around the
// color circle.
func hue(a, b float64) func(t float64) float64 {
	d := b - a
	if d != 0 && !math.IsNaN(d) {
		if d > 180 || d < -180 {
			d = d - float64(360*js.Round(d/360))
		}
		return linear(a, d)
	}
	return constantOf(a, b)
}

// nogamma is the default export of color.js.
func nogamma(a, b float64) func(t float64) float64 {
	d := b - a
	if d != 0 && !math.IsNaN(d) {
		return linear(a, d)
	}
	return constantOf(a, b)
}

// gamma is the gamma function of color.js.
func gamma(y float64) func(a, b float64) func(t float64) float64 {
	if y == 1 {
		return nogamma
	}
	return func(a, b float64) func(t float64) float64 {
		if d := b - a; d != 0 && !math.IsNaN(d) {
			return exponential(a, b, y)
		}
		return constantOf(a, b)
	}
}

// Hue is d3.interpolateHue(a, b).
func Hue(a, b float64) func(t float64) float64 {
	i := hue(a, b)
	return func(t float64) float64 {
		x := i(t)
		return x - float64(360*math.Floor(x/360))
	}
}

// RGB is d3.interpolateRgb(a, b). The result of the interpolator is an RGB
// string, as color.toString gives it.
func RGB(a, b color.Color) func(t float64) string {
	return RGBGamma(1)(a, b)
}

// RGBGamma is d3.interpolateRgb.gamma(y).
func RGBGamma(y float64) func(a, b color.Color) func(t float64) string {
	channel := gamma(y)
	return func(a, b color.Color) func(t float64) string {
		start, end := color.RGBFrom(a), color.RGBFrom(b)
		r := channel(start.R, end.R)
		g := channel(start.G, end.G)
		bl := channel(start.B, end.B)
		opacity := nogamma(start.Opacity, end.Opacity)
		return func(t float64) string {
			return color.RGB{R: r(t), G: g(t), B: bl(t), Opacity: opacity(t)}.String()
		}
	}
}

func hslWith(hue func(a, b float64) func(float64) float64) func(a, b color.Color) func(t float64) string {
	return func(a, b color.Color) func(t float64) string {
		start, end := color.HSLFrom(a), color.HSLFrom(b)
		h := hue(start.H, end.H)
		s := nogamma(start.S, end.S)
		l := nogamma(start.L, end.L)
		opacity := nogamma(start.Opacity, end.Opacity)
		return func(t float64) string {
			return color.HSL{H: h(t), S: s(t), L: l(t), Opacity: opacity(t)}.String()
		}
	}
}

// HSL is d3.interpolateHsl(a, b).
func HSL(a, b color.Color) func(t float64) string { return hslWith(hue)(a, b) }

// HSLLong is d3.interpolateHslLong(a, b).
func HSLLong(a, b color.Color) func(t float64) string { return hslWith(nogamma)(a, b) }

// Lab is d3.interpolateLab(a, b).
func Lab(a, b color.Color) func(t float64) string {
	start, end := color.LabFrom(a), color.LabFrom(b)
	l := nogamma(start.L, end.L)
	aa := nogamma(start.A, end.A)
	bb := nogamma(start.B, end.B)
	opacity := nogamma(start.Opacity, end.Opacity)
	return func(t float64) string {
		return color.Lab{L: l(t), A: aa(t), B: bb(t), Opacity: opacity(t)}.String()
	}
}

func hclWith(hue func(a, b float64) func(float64) float64) func(a, b color.Color) func(t float64) string {
	return func(a, b color.Color) func(t float64) string {
		start, end := color.HCLFrom(a), color.HCLFrom(b)
		h := hue(start.H, end.H)
		c := nogamma(start.C, end.C)
		l := nogamma(start.L, end.L)
		opacity := nogamma(start.Opacity, end.Opacity)
		return func(t float64) string {
			return color.HCL{H: h(t), C: c(t), L: l(t), Opacity: opacity(t)}.String()
		}
	}
}

// HCL is d3.interpolateHcl(a, b).
func HCL(a, b color.Color) func(t float64) string { return hclWith(hue)(a, b) }

// HCLLong is d3.interpolateHclLong(a, b).
func HCLLong(a, b color.Color) func(t float64) string { return hclWith(nogamma)(a, b) }

func cubehelixWith(hue func(a, b float64) func(float64) float64, y float64) func(a, b color.Color) func(t float64) string {
	return func(a, b color.Color) func(t float64) string {
		start, end := color.CubehelixFrom(a), color.CubehelixFrom(b)
		h := hue(start.H, end.H)
		s := nogamma(start.S, end.S)
		l := nogamma(start.L, end.L)
		opacity := nogamma(start.Opacity, end.Opacity)
		return func(t float64) string {
			return color.Cubehelix{H: h(t), S: s(t), L: l(jsm.Pow(t, y)), Opacity: opacity(t)}.String()
		}
	}
}

// Cubehelix is d3.interpolateCubehelix(a, b).
func Cubehelix(a, b color.Color) func(t float64) string { return cubehelixWith(hue, 1)(a, b) }

// CubehelixLong is d3.interpolateCubehelixLong(a, b).
func CubehelixLong(a, b color.Color) func(t float64) string {
	return cubehelixWith(nogamma, 1)(a, b)
}

// CubehelixGamma is d3.interpolateCubehelix.gamma(y).
func CubehelixGamma(y float64) func(a, b color.Color) func(t float64) string {
	return cubehelixWith(hue, y)
}

// CubehelixLongGamma is d3.interpolateCubehelixLong.gamma(y).
func CubehelixLongGamma(y float64) func(a, b color.Color) func(t float64) string {
	return cubehelixWith(nogamma, y)
}

// Discrete is d3.interpolateDiscrete(values). The result is the zero value
// where d3 returns undefined: for an empty list and for a NaN argument.
func Discrete[T any](values []T) func(t float64) T {
	n := float64(len(values))
	return func(t float64) T {
		i := js.Max(0, js.Min(n-1, math.Floor(t*n)))
		if math.IsNaN(i) || int(i) >= len(values) {
			var zero T
			return zero
		}
		return values[int(i)]
	}
}

// Quantize is d3.quantize(interpolator, n).
func Quantize[T any](interpolator func(t float64) T, n int) []T {
	samples := make([]T, n)
	for i := 0; i < n; i++ {
		samples[i] = interpolator(float64(i) / float64(n-1))
	}
	return samples
}

// Piecewise is d3.piecewise(interpolate, values). The result is the zero
// value where d3 throws: for fewer than two values and for a NaN argument.
func Piecewise[T, R any](interpolate func(a, b T) func(t float64) R, values []T) func(t float64) R {
	n := len(values) - 1
	parts := make([]func(t float64) R, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, interpolate(values[i], values[i+1]))
	}
	return func(t float64) R {
		t = float64(t * float64(n))
		i := js.Max(0, js.Min(float64(n-1), math.Floor(t)))
		if math.IsNaN(i) || int(i) >= len(parts) {
			var zero R
			return zero
		}
		return parts[int(i)](t - i)
	}
}

// basis is the basis function of basis.js.
func basis(t1, v0, v1, v2, v3 float64) float64 {
	t2 := float64(t1 * t1)
	t3 := float64(t2 * t1)
	return (float64((1-float64(3*t1)+float64(3*t2)-t3)*v0) +
		float64((4-float64(6*t2)+float64(3*t3))*v1) +
		float64((1+float64(3*t1)+float64(3*t2)-float64(3*t3))*v2) +
		float64(t3*v3)) / 6
}

// at is values[i] in JavaScript: NaN for an index that does not exist.
func at(values []float64, i float64) float64 {
	if i >= 0 && i < float64(len(values)) && i == math.Trunc(i) {
		return values[int(i)]
	}
	return math.NaN()
}

// Basis is d3.interpolateBasis(values).
func Basis(values []float64) func(t float64) float64 {
	n := float64(len(values) - 1)
	return func(t float64) float64 {
		var i float64
		switch {
		case t <= 0:
			t = 0
			i = 0
		case t >= 1:
			t = 1
			i = n - 1
		default:
			i = math.Floor(t * n)
		}
		v1 := at(values, i)
		v2 := at(values, i+1)
		var v0, v3 float64
		if i > 0 {
			v0 = at(values, i-1)
		} else {
			v0 = float64(2*v1) - v2
		}
		if i < n-1 {
			v3 = at(values, i+2)
		} else {
			v3 = float64(2*v2) - v1
		}
		return basis(float64((t-i/n)*n), v0, v1, v2, v3)
	}
}

// BasisClosed is d3.interpolateBasisClosed(values).
func BasisClosed(values []float64) func(t float64) float64 {
	n := float64(len(values))
	return func(t float64) float64 {
		t = math.Mod(t, 1)
		if t < 0 {
			t++
		}
		i := math.Floor(t * n)
		v0 := at(values, math.Mod(i+n-1, n))
		v1 := at(values, math.Mod(i, n))
		v2 := at(values, math.Mod(i+1, n))
		v3 := at(values, math.Mod(i+2, n))
		return basis(float64((t-i/n)*n), v0, v1, v2, v3)
	}
}

func rgbSpline(spline func([]float64) func(float64) float64) func(colors []color.Color) func(t float64) string {
	return func(colors []color.Color) func(t float64) string {
		n := len(colors)
		r := make([]float64, n)
		g := make([]float64, n)
		b := make([]float64, n)
		for i, c := range colors {
			rgb := color.RGBFrom(c)
			r[i] = orZero(rgb.R)
			g[i] = orZero(rgb.G)
			b[i] = orZero(rgb.B)
		}
		rs, gs, bs := spline(r), spline(g), spline(b)
		return func(t float64) string {
			return color.RGB{R: rs(t), G: gs(t), B: bs(t), Opacity: 1}.String()
		}
	}
}

// orZero is the JavaScript expression value || 0.
func orZero(value float64) float64 {
	if value == 0 || math.IsNaN(value) {
		return 0
	}
	return value
}

// RGBBasis is d3.interpolateRgbBasis(colors). The colors must not be empty.
func RGBBasis(colors []color.Color) func(t float64) string {
	return rgbSpline(Basis)(colors)
}

// RGBBasisClosed is d3.interpolateRgbBasisClosed(colors). The colors must not
// be empty.
func RGBBasisClosed(colors []color.Color) func(t float64) string {
	return rgbSpline(BasisClosed)(colors)
}
