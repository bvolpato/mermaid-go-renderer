package crmath

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
)

// mathSlowFuncs are the slow paths of the correctly rounded functions.
var mathSlowFuncs = map[string]func(float64) float64{
	"sin": sinSlow, "cos": cosSlow, "tan": tanSlow, "asin": asinSlow, "acos": acosSlow,
	"exp": expSlow, "expm1": expm1Slow, "log": logSlow, "log1p": log1pSlow,
	"log2": log2Slow, "log10": log10Slow, "cbrt": cbrtSlow,
}

// TestMathEval evaluates one function for the arguments of a file and writes
// the results to another file, so that another program can compare them with
// a reference such as MPFR. Both files have little-endian float64 numbers.
// The environment variable JSMATH_EVAL names the function, for example
// "atan2", "sin.slow" for the slow path of a correctly rounded function, or
// "pow.libm" for the pow of the C library.
// JSMATH_EVAL_IN and JSMATH_EVAL_OUT name the files.
func TestMathEval(t *testing.T) {
	name := os.Getenv("JSMATH_EVAL")
	if name == "" {
		t.Skip("JSMATH_EVAL is not set")
	}
	var fn func(a []float64) float64
	arity := 1
	if name == "pow.libm" {
		// The pow of the C library, without the special cases of V8.
		fn = func(a []float64) float64 { return powLibm(a[0], a[1]) }
		arity = 2
	} else if n := len(name) - len(".slow"); n > 0 && name[n:] == ".slow" {
		slow := mathSlowFuncs[name[:n]]
		if slow == nil {
			t.Fatalf("no slow path for %s", name[:n])
		}
		fn = func(a []float64) float64 { return slow(a[0]) }
	} else {
		for _, f := range mathFuncs {
			if f.name == name {
				fn = f.fn
				arity = mathArity(name)
			}
		}
		if fn == nil {
			t.Fatalf("no function %s", name)
		}
	}
	data, err := os.ReadFile(os.Getenv("JSMATH_EVAL_IN"))
	if err != nil {
		t.Fatal(err)
	}
	in := make([]float64, len(data)/8)
	for i := range in {
		in[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[8*i:]))
	}
	out := make([]byte, 0, len(in)/arity*8)
	for i := 0; i+arity <= len(in); i += arity {
		r := fn(in[i : i+arity])
		b := math.Float64bits(r)
		if math.IsNaN(r) {
			b = 0x7ff8000000000000
		}
		out = binary.LittleEndian.AppendUint64(out, b)
	}
	if err := os.WriteFile(os.Getenv("JSMATH_EVAL_OUT"), out, 0o644); err != nil {
		t.Fatal(err)
	}
}
