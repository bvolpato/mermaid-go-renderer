package crmath

import (
	"math"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

// mathCRFuncs are the correctly rounded functions: the fast passes, the
// slow path, and the arguments that the slow path accepts.
var mathCRFuncs = []struct {
	name  string
	fast  func(float64) (float64, bool)
	slow  func(float64) float64
	valid func(x float64) bool
}{
	{"sin", sinFast, sinSlow, mathFiniteNotZero},
	{"cos", cosFast, cosSlow, mathFiniteNotZero},
	{"tan", tanFast, tanSlow, mathFiniteNotZero},
	{"asin", asinFast, asinSlow, func(x float64) bool { return x != 0 && math.Abs(x) < 1 }},
	{"acos", acosFast, acosSlow, func(x float64) bool { return x != 0 && math.Abs(x) < 1 }},
	{"exp", expFast, expSlow, func(x float64) bool { return x != 0 && math.Abs(x) < 1000 }},
	{"expm1", expm1Fast, expm1Slow, func(x float64) bool { return x != 0 && math.Abs(x) < 1000 }},
	{"log", logFast, logSlow, mathPositiveNotOne},
	{"log1p", log1pFast, log1pSlow, func(x float64) bool { return x != 0 && x > -1 && !math.IsInf(x, 0) }},
	{"log2", log2Fast, log2Slow, mathPositiveNotOne},
	{"log10", log10Fast, log10Slow, mathPositiveNotOne},
	{"cbrt", cbrtFast, cbrtSlow, mathFiniteNotZero},
}

func mathFiniteNotZero(x float64) bool { return x != 0 && !math.IsNaN(x) && !math.IsInf(x, 0) }

func mathPositiveNotOne(x float64) bool { return x > 0 && x != 1 && !math.IsInf(x, 0) }

// TestMathFastAgainstSlow compares the fast passes of the correctly rounded
// functions with the computation with math/big. A result of a fast pass
// that passes its rounding test must be the correctly rounded value. The
// test also reports how often the rounding test fails, so that the slow path
// gives the result.
//
// Each function gets two sets of random arguments: arguments of a typical
// size (mathTypicalArgs), and the arguments of the generator of the golden
// file with another seed, which has many hard cases. The number of arguments
// of each set is 20000, or 2000 with -short. The environment variable
// JSMATH_SELFCHECK_N sets another number, for example 1000000.
func TestMathFastAgainstSlow(t *testing.T) {
	n := 20000
	if testing.Short() {
		n = 2000
	}
	if s := os.Getenv("JSMATH_SELFCHECK_N"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			t.Fatal(err)
		}
		n = v
	}
	for _, f := range mathCRFuncs {
		for _, set := range []struct {
			name string
			in   []float64
		}{
			{"typical arguments", mathTypicalArgs(f.name, n)},
			{"hard arguments", mathInputsSalt(f.name, n, 1)},
		} {
			in := set.in
			workers := runtime.GOMAXPROCS(0)
			var mu sync.Mutex
			var wg sync.WaitGroup
			checked, slow, wrong := 0, 0, 0
			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func(w int) {
					defer wg.Done()
					c, s, bad := 0, 0, 0
					for i := w; i < len(in); i += workers {
						x := in[i]
						if !f.valid(x) {
							continue
						}
						c++
						got, ok := f.fast(x)
						if !ok {
							s++
							continue
						}
						if want := f.slow(x); math.Float64bits(got) != math.Float64bits(want) {
							bad++
							if bad <= 3 {
								t.Errorf("%s(%v): the fast pass gives %v (%016x), math/big gives %v (%016x)", f.name, x, got, math.Float64bits(got), want, math.Float64bits(want))
							}
						}
					}
					mu.Lock()
					checked, slow, wrong = checked+c, slow+s, wrong+bad
					mu.Unlock()
				}(w)
			}
			wg.Wait()
			if wrong != 0 {
				t.Errorf("%s: %d of %d results of the fast pass are not correctly rounded", f.name, wrong, checked)
			}
			t.Logf("%-6s %d %s, the slow path gives the result for %d (1 in %.0f)", f.name, checked, set.name, slow, float64(checked)/math.Max(float64(slow), 1))
		}
	}
}
