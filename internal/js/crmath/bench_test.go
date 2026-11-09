package crmath

import (
	"math"
	"testing"
)

// mathTypicalArgs returns the arguments of n cases of a typical size: angles
// for the trigonometric functions, and numbers of a moderate size for the
// others. The benchmarks and the self-check use them.
func mathTypicalArgs(name string, n int) []float64 {
	g := &mathGen{s: 0x62656e6368}
	arity := mathArity(name)
	in := make([]float64, 0, n*arity)
	for len(in) < n*arity {
		switch name {
		case "sin", "cos", "tan":
			in = append(in, g.uniform(-10, 10))
		case "asin", "acos", "atanh":
			in = append(in, g.uniform(-1, 1))
		case "atan", "asinh", "sinh", "cosh", "tanh", "expm1":
			in = append(in, g.uniform(-5, 5))
		case "exp":
			in = append(in, g.uniform(-20, 20))
		case "log", "log2", "log10", "log1p", "cbrt", "sqrt":
			in = append(in, g.uniform(0, 1000))
		case "acosh":
			in = append(in, g.uniform(1, 100))
		case "atan2", "hypot":
			in = append(in, g.uniform(-500, 500), g.uniform(-500, 500))
		case "pow":
			in = append(in, g.uniform(0, 100), g.uniform(-4, 4))
		default:
			in = append(in, g.any())
		}
	}
	return in
}

// BenchmarkMath measures every function of the golden file, and the
// function of Go's math package for comparison.
func BenchmarkMath(b *testing.B) {
	var sink float64
	for _, f := range mathFuncs {
		arity := mathArity(f.name)
		in := mathTypicalArgs(f.name, 1024)
		run := func(fn func([]float64) float64) func(b *testing.B) {
			return func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					k := (i & 1023) * arity
					sink += fn(in[k : k+arity])
				}
			}
		}
		b.Run(f.name, run(f.fn))
		if f.std != nil {
			b.Run(f.name+"/go-math", run(f.std))
		}
	}
	if math.IsInf(sink, 0) {
		b.Log("the sum is infinite")
	}
}

// BenchmarkMathSlow measures the slow path of the correctly rounded
// functions, which the fast passes call when their rounding test fails.
func BenchmarkMathSlow(b *testing.B) {
	var sink float64
	for _, f := range mathCRFuncs {
		var in []float64
		for _, x := range mathTypicalArgs(f.name, 1024) {
			if f.valid(x) {
				in = append(in, x)
			}
		}
		b.Run(f.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sink += f.slow(in[i%len(in)])
			}
		})
	}
	if math.IsInf(sink, 0) {
		b.Log("the sum is infinite")
	}
}
