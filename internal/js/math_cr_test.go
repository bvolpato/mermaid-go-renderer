package js

import (
	"math"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js/crmath"
)

// TestMathWrappers checks that every Math function of math.go returns the
// result of the package crmath, which has the golden test against the
// browser. It also compares Round of num.go with Math.round of crmath.
func TestMathWrappers(t *testing.T) {
	unary := []struct {
		name string
		js   func(float64) float64
		cr   func(float64) float64
	}{
		{"Sin", Sin, crmath.Sin}, {"Cos", Cos, crmath.Cos}, {"Tan", Tan, crmath.Tan},
		{"Asin", Asin, crmath.Asin}, {"Acos", Acos, crmath.Acos}, {"Atan", Atan, crmath.Atan},
		{"Exp", Exp, crmath.Exp}, {"Expm1", Expm1, crmath.Expm1}, {"Log", Log, crmath.Log},
		{"Log1p", Log1p, crmath.Log1p}, {"Log2", Log2, crmath.Log2}, {"Log10", Log10, crmath.Log10},
		{"Sqrt", Sqrt, crmath.Sqrt}, {"Cbrt", Cbrt, crmath.Cbrt}, {"Sinh", Sinh, crmath.Sinh},
		{"Cosh", Cosh, crmath.Cosh}, {"Tanh", Tanh, crmath.Tanh}, {"Asinh", Asinh, crmath.Asinh},
		{"Acosh", Acosh, crmath.Acosh}, {"Atanh", Atanh, crmath.Atanh}, {"Sign", Sign, crmath.Sign},
		{"Trunc", Trunc, crmath.Trunc}, {"Fround", Fround, crmath.Fround}, {"Clz32", Clz32, crmath.Clz32},
		{"Round", Round, crmath.Round},
		{"SinCR", SinCR, crmath.Sin}, {"CosCR", CosCR, crmath.Cos}, {"TanCR", TanCR, crmath.Tan},
		{"AsinCR", AsinCR, crmath.Asin}, {"AcosCR", AcosCR, crmath.Acos},
	}
	binary := []struct {
		name string
		js   func(a, b float64) float64
		cr   func(a, b float64) float64
	}{
		{"Atan2", Atan2, crmath.Atan2}, {"Pow", Pow, crmath.Pow}, {"Imul", Imul, crmath.Imul},
		{"Hypot", func(a, b float64) float64 { return Hypot(a, b) }, func(a, b float64) float64 { return crmath.Hypot(a, b) }},
	}
	// The arguments: special values, and random numbers of all sizes from a
	// xorshift generator.
	args := []float64{0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 2.5, -2.5, 1e-320, 1e300, -1e300,
		math.Pi, math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN(), 0.49999999999999994, 4503599627370497.5}
	s := uint64(0x6d6d6467)
	next := func() uint64 {
		s ^= s << 13
		s ^= s >> 7
		s ^= s << 17
		return s
	}
	for i := 0; i < 20000; i++ {
		switch i % 4 {
		case 0:
			args = append(args, math.Float64frombits(next()))
		case 1:
			args = append(args, float64(int64(next())>>40)/1024)
		default:
			args = append(args, float64(int64(next())>>11)/float64(uint64(1)<<(40+next()%20)))
		}
	}
	same := func(a, b float64) bool {
		return math.Float64bits(a) == math.Float64bits(b) || (math.IsNaN(a) && math.IsNaN(b))
	}
	for _, f := range unary {
		for _, x := range args {
			if got, want := f.js(x), f.cr(x); !same(got, want) {
				t.Errorf("%s(%v) = %v, crmath has %v", f.name, x, got, want)
				break
			}
		}
	}
	for _, f := range binary {
		for i, x := range args {
			y := args[(i*7+3)%len(args)]
			if got, want := f.js(x, y), f.cr(x, y); !same(got, want) {
				t.Errorf("%s(%v, %v) = %v, crmath has %v", f.name, x, y, got, want)
				break
			}
		}
	}
}
