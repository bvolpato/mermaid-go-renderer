package crmath

// The golden file testdata/chromium_math.bin.gz has the results of the Math
// functions of Chromium on macOS for the arguments of gen_test.go. It comes
// from tools/reference/gen-jsmath.mjs. The golden file of Chromium on Linux
// and its test are in linux_libm_test.go.

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"math"
	"os"
	"testing"
)

// mathFunc is a Math function of the golden file.
type mathFunc struct {
	name string
	// exact is true for a function that IEEE 754 defines exactly. The golden
	// file of the repository has fewer cases for it.
	exact bool
	// fn is the function of this package.
	fn func(a []float64) float64
	// std is the function of Go's math package, or nil.
	std func(a []float64) float64
}

func mathUnary(f func(float64) float64) func([]float64) float64 {
	if f == nil {
		return nil
	}
	return func(a []float64) float64 { return f(a[0]) }
}

func mathBinary(f func(a, b float64) float64) func([]float64) float64 {
	return func(a []float64) float64 { return f(a[0], a[1]) }
}

func mathVariadic(f func(...float64) float64) func([]float64) float64 {
	return func(a []float64) float64 { return f(a...) }
}

var mathFuncs = []mathFunc{
	{"sin", false, mathUnary(Sin), mathUnary(math.Sin)},
	{"cos", false, mathUnary(Cos), mathUnary(math.Cos)},
	{"tan", false, mathUnary(Tan), mathUnary(math.Tan)},
	{"asin", false, mathUnary(Asin), mathUnary(math.Asin)},
	{"acos", false, mathUnary(Acos), mathUnary(math.Acos)},
	{"atan", false, mathUnary(Atan), mathUnary(math.Atan)},
	{"atan2", false, mathBinary(Atan2), mathBinary(math.Atan2)},
	{"exp", false, mathUnary(Exp), mathUnary(math.Exp)},
	{"expm1", false, mathUnary(Expm1), mathUnary(math.Expm1)},
	{"log", false, mathUnary(Log), mathUnary(math.Log)},
	{"log1p", false, mathUnary(Log1p), mathUnary(math.Log1p)},
	{"log2", false, mathUnary(Log2), mathUnary(math.Log2)},
	{"log10", false, mathUnary(Log10), mathUnary(math.Log10)},
	{"pow", false, mathBinary(Pow), mathBinary(math.Pow)},
	{"cbrt", false, mathUnary(Cbrt), mathUnary(math.Cbrt)},
	{"sinh", false, mathUnary(Sinh), mathUnary(math.Sinh)},
	{"cosh", false, mathUnary(Cosh), mathUnary(math.Cosh)},
	{"tanh", false, mathUnary(Tanh), mathUnary(math.Tanh)},
	{"asinh", false, mathUnary(Asinh), mathUnary(math.Asinh)},
	{"acosh", false, mathUnary(Acosh), mathUnary(math.Acosh)},
	{"atanh", false, mathUnary(Atanh), mathUnary(math.Atanh)},
	{"hypot", false, mathVariadic(Hypot), mathBinary(math.Hypot)},
	{"hypot1", true, mathVariadic(Hypot), nil},
	{"hypot3", true, mathVariadic(Hypot), nil},
	{"hypot4", true, mathVariadic(Hypot), nil},
	{"sqrt", true, mathUnary(Sqrt), mathUnary(math.Sqrt)},
	{"fround", true, mathUnary(Fround), nil},
	{"round", true, mathUnary(Round), nil},
	{"trunc", true, mathUnary(Trunc), mathUnary(math.Trunc)},
	{"sign", true, mathUnary(Sign), nil},
	{"clz32", true, mathUnary(Clz32), nil},
	{"imul", true, mathBinary(Imul), nil},
}

// mathGolden has the results of Chromium for one function.
type mathGolden struct {
	arity int
	hash  uint64
	out   []uint64
}

// loadMathGolden reads a golden file. The format is: the magic "JSMATHGD",
// the user agent (a uint16 length and the text), the number of functions
// (uint32), and for each function the name (a length byte and the text), the
// arity (a byte), the number of cases (uint32), the hash of the arguments
// (uint64) and the results. The results are 8 planes: plane k has byte k of
// the little-endian bits of every result. A NaN result is 0x7ff8000000000000.
func loadMathGolden(t testing.TB) (userAgent string, funcs map[string]*mathGolden) {
	t.Helper()
	path := os.Getenv("JSMATH_GOLDEN")
	if path == "" {
		path = "testdata/chromium_math.bin.gz"
	}
	return loadMathGoldenFile(t, path)
}

// loadMathGoldenFile reads the golden file with a path.
func loadMathGoldenFile(t testing.TB, path string) (userAgent string, funcs map[string]*mathGolden) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("JSMATHGD")) {
		t.Fatalf("%s is not a golden file of the Math functions", path)
	}
	p := 8
	n := int(binary.LittleEndian.Uint16(data[p:]))
	p += 2
	userAgent = string(data[p : p+n])
	p += n
	count := int(binary.LittleEndian.Uint32(data[p:]))
	p += 4
	funcs = map[string]*mathGolden{}
	for i := 0; i < count; i++ {
		n = int(data[p])
		name := string(data[p+1 : p+1+n])
		p += 1 + n
		g := &mathGolden{arity: int(data[p])}
		cases := int(binary.LittleEndian.Uint32(data[p+1:]))
		g.hash = binary.LittleEndian.Uint64(data[p+5:])
		p += 13
		g.out = make([]uint64, cases)
		for k := 0; k < 8; k++ {
			for j := 0; j < cases; j++ {
				g.out[j] |= uint64(data[p+j]) << (8 * uint(k))
			}
			p += cases
		}
		funcs[name] = g
	}
	return userAgent, funcs
}

func mathSame(got float64, want uint64) bool {
	if math.IsNaN(got) {
		return want == 0x7ff8000000000000
	}
	return math.Float64bits(got) == want
}

// TestMathChromium compares every Math function with the results of
// Chromium on macOS, bit for bit. It also reports how often Go's math
// package gives another result. Set JSMATH_GOLDEN to test another golden
// file.
func TestMathChromium(t *testing.T) {
	userAgent, golden := loadMathGolden(t)
	t.Logf("golden file from %s", userAgent)
	checkMathGolden(t, golden, PlatformDarwin)
}

// checkMathGolden compares every Math function with a golden file, with the
// platform of that file as the current platform.
func checkMathGolden(t *testing.T, golden map[string]*mathGolden, p Platform) {
	defer SetPlatform(CurrentPlatform())
	SetPlatform(p)
	for _, f := range mathFuncs {
		if p == PlatformDarwin && !HasDarwinLibm() && (f.name == "pow" || f.name == "tanh") {
			// The module has no port of the pow and the tanh of macOS.
			t.Logf("%-6s not compared: the package does not have this function of macOS", f.name)
			continue
		}
		g := golden[f.name]
		if g == nil {
			t.Errorf("%s: no golden data", f.name)
			continue
		}
		arity := mathArity(f.name)
		in := mathInputs(f.name, len(g.out))
		if g.arity != arity || mathInputHash(in) != g.hash {
			t.Errorf("%s: the golden file has other arguments. Run tools/reference/gen-jsmath.mjs", f.name)
			continue
		}
		differ, stdDiffer, stdSpecial := 0, 0, 0
		maxStd := uint64(0)
		for i, want := range g.out {
			a := in[i*arity : (i+1)*arity]
			if got := f.fn(a); !mathSame(got, want) {
				differ++
				if differ <= 5 {
					t.Errorf("%s(%v) = %v (%016x), Chromium has %v (%016x)", f.name, a, got, math.Float64bits(got), math.Float64frombits(want), want)
				}
			}
			if f.std != nil {
				if s := f.std(a); !mathSame(s, want) {
					stdDiffer++
					if d, ok := mathUlpDistance(s, math.Float64frombits(want)); !ok {
						stdSpecial++
					} else if d > maxStd {
						maxStd = d
					}
				}
			}
		}
		if differ != 0 {
			t.Errorf("%s: %d of %d results differ from Chromium", f.name, differ, len(g.out))
		}
		if f.std != nil {
			t.Logf("%-6s %d cases, Go's math package differs for %d (%.2f%%): %d by a sign, NaN or infinity, the others by at most %d ulp", f.name, len(g.out), stdDiffer, 100*float64(stdDiffer)/float64(len(g.out)), stdSpecial, maxStd)
		} else {
			t.Logf("%-6s %d cases", f.name, len(g.out))
		}
	}
}

// mathUlpDistance is the number of float64 steps from a to b. ok is false
// when the numbers do not have a distance: one is NaN or infinite, or the
// signs differ.
func mathUlpDistance(a, b float64) (d uint64, ok bool) {
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) || math.Signbit(a) != math.Signbit(b) {
		return 0, false
	}
	ua, ub := math.Float64bits(a)&^(1<<63), math.Float64bits(b)&^(1<<63)
	if ua > ub {
		return ua - ub, true
	}
	return ub - ua, true
}
