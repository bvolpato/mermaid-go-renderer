package crmath

// The golden file testdata/chromium_math_linux.bin.gz has the results of the
// Math functions of Chromium on Linux for the arguments of gen_test.go. It
// comes from tools/reference/gen-jsmath.mjs --platform linux, which runs
// Chromium in the image mcr.microsoft.com/playwright:v1.64.1-noble. The
// format is that of the golden file of macOS (golden_test.go).

import (
	"math"
	"os"
	"strings"
	"testing"
)

// loadMathGoldenLinux reads the golden file of Chromium on Linux. Set
// JSMATH_GOLDEN_LINUX to read another file.
func loadMathGoldenLinux(t testing.TB) (userAgent string, funcs map[string]*mathGolden) {
	t.Helper()
	path := os.Getenv("JSMATH_GOLDEN_LINUX")
	if path == "" {
		path = "testdata/chromium_math_linux.bin.gz"
	}
	userAgent, funcs = loadMathGoldenFile(t, path)
	if !strings.Contains(userAgent, "Linux") {
		t.Fatalf("%s is not from Chromium on Linux: %s", path, userAgent)
	}
	return userAgent, funcs
}

// TestMathChromiumLinux compares every Math function with the results of
// Chromium on Linux, bit for bit, with PlatformLinux as the current
// platform. Set JSMATH_GOLDEN_LINUX to test another golden file.
func TestMathChromiumLinux(t *testing.T) {
	userAgent, golden := loadMathGoldenLinux(t)
	t.Logf("golden file from %s", userAgent)
	checkMathGolden(t, golden, PlatformLinux)
}

// TestMathPlatformFuncs compares the functions of one platform with the
// golden file of that platform. These functions do not read the current
// platform, so the test selects the other platform.
func TestMathPlatformFuncs(t *testing.T) {
	defer SetPlatform(CurrentPlatform())
	_, linux := loadMathGoldenLinux(t)
	_, darwin := loadMathGolden(t)
	for _, c := range []struct {
		name     string
		golden   map[string]*mathGolden
		other    Platform
		pow      func(x, y float64) float64
		tanh     func(x float64) float64
		compiled bool
	}{
		{"linux", linux, PlatformDarwin, PowLinux, TanhLinux, true},
		{"darwin", darwin, PlatformLinux, PowDarwin, TanhDarwin, HasDarwinLibm()},
	} {
		if !c.compiled {
			t.Logf("%s: not compared: the package does not have the functions of macOS", c.name)
			continue
		}
		SetPlatform(c.other)
		for _, f := range []struct {
			name string
			fn   func(a []float64) float64
		}{
			{"pow", mathBinary(c.pow)},
			{"tanh", mathUnary(c.tanh)},
		} {
			g := c.golden[f.name]
			if g == nil {
				t.Errorf("%s %s: no golden data", c.name, f.name)
				continue
			}
			arity := mathArity(f.name)
			in := mathInputs(f.name, len(g.out))
			if g.arity != arity || mathInputHash(in) != g.hash {
				t.Errorf("%s %s: the golden file has other arguments. Run tools/reference/gen-jsmath.mjs", c.name, f.name)
				continue
			}
			differ := 0
			for i, want := range g.out {
				a := in[i*arity : (i+1)*arity]
				if got := f.fn(a); !mathSame(got, want) {
					differ++
					if differ <= 5 {
						t.Errorf("%s %s(%v) = %v (%016x), Chromium has %v (%016x)", c.name, f.name, a, got, math.Float64bits(got), math.Float64frombits(want), want)
					}
				}
			}
			if differ != 0 {
				t.Errorf("%s %s: %d of %d results differ from Chromium", c.name, f.name, differ, len(g.out))
			}
		}
	}
}

// TestMathPlatformsDiffer compares the golden files of the two platforms.
// Only pow and tanh may differ: the package has one implementation for each
// other function. The test reports the number of results that differ.
func TestMathPlatformsDiffer(t *testing.T) {
	_, linux := loadMathGoldenLinux(t)
	_, darwin := loadMathGolden(t)
	for _, f := range mathFuncs {
		l, d := linux[f.name], darwin[f.name]
		if l == nil || d == nil {
			t.Errorf("%s: no golden data", f.name)
			continue
		}
		if l.arity != d.arity || l.hash != d.hash || len(l.out) != len(d.out) {
			t.Logf("%-6s not compared: the two golden files have other arguments", f.name)
			continue
		}
		differ := 0
		maxUlp := uint64(0)
		for i := range l.out {
			if l.out[i] == d.out[i] {
				continue
			}
			differ++
			if n, ok := mathUlpDistance(math.Float64frombits(l.out[i]), math.Float64frombits(d.out[i])); !ok {
				t.Errorf("%s: case %d differs by a sign, NaN or infinity: Linux has %016x, macOS has %016x", f.name, i, l.out[i], d.out[i])
			} else if n > maxUlp {
				maxUlp = n
			}
		}
		switch {
		case differ == 0:
		case f.name == "pow" || f.name == "tanh":
			t.Logf("%-6s %d cases, Linux and macOS differ for %d (%.3f%%), by at most %d ulp", f.name, len(l.out), differ, 100*float64(differ)/float64(len(l.out)), maxUlp)
		default:
			t.Errorf("%s: Linux and macOS differ for %d of %d cases, by at most %d ulp. The package has one function for both platforms", f.name, differ, len(l.out), maxUlp)
		}
	}
}

// TestMathPowLinuxSpecial has arguments of pow that the generator of the
// golden file does not give. The results are those of Chromium on Linux,
// from one run of tools/reference/jsmath-page.js in the Linux image.
func TestMathPowLinuxSpecial(t *testing.T) {
	const (
		nan  = 0x7ff8000000000000
		snan = 0x7ff0000000000001 // a signaling NaN
		one  = 0x3ff0000000000000
	)
	f := math.Float64frombits
	for _, c := range []struct {
		x, y float64
		want uint64
	}{
		// exp_inline rounds 128/log(2) times y log(x) to an integer. For
		// these arguments that product is an integer plus one half, and a
		// tie that goes to the even integer gives another last bit.
		{0x1.b33f69b706695p-5, 0x1.2a5e45c5ea383p+7, 0x1873d0e544ede2e5},
		{0x1.19bd0c62fe6c8p+8, 0x1.d5592996c8585p+5, 0x5dc5f6a320dceb2b},
		{0x1.153152e2ce8fdp-4, 0x1.3daf7c9d179dbp+7, 0x195cf3155b5babaa},
		{0x1.af6e5ebd3439ep+7, -0x1.2a5e6fd4b5242p+2, 0x3dacf3155b5bab72},
		{0x1.012924001a679p+8, 0x1.2b65f8e0a8685p+6, 0x65637f26231e769f},
		{0x1.50debd640a956p-4, -0x1.b8c42559f4697p+1, 0x40b542e2f4f6ad2b},
		{0x1.d69292aca1b84p-6, -0x1.88ca5eed7c56bp+6, 0x5f5eb9f4867ccac0},
		{0x1.db9388d8c21f3p+7, -0x1.866692a5aaa99p+6, 0x0fc814d2add105f2},
		// The result is below 2^-738: specialcase computes scale + scale*tmp
		// with a product that is not fused. A fused operation gives another
		// last bit for these arguments.
		{0x1.55dba50dfcc2dp+39, -0x1.6354dab1ba379p+4, 0x0938790ba5267b78},
		{0x1.e51515220684fp-38, 0x1.b737a71f456a0p+4, 0x00520346274207f8},
		{0x1.90ee560418937p+9, -0x1.87ae147ae147bp+6, 0x04e45330fd667c50},
		{0x1.f3424de5a1eb0p-3, 0x1.f569d296b6e2ap+8, 0x001ec927834d4d0c},
		// A signaling NaN gives the result of a quiet NaN. The pow of glibc
		// returns NaN for the first four cases.
		{f(snan), 0, one},
		{f(snan), math.Copysign(0, -1), one},
		{f(snan | 1<<63), 0, one},
		{f(0x7ff7ffffffffffff), 0, one},
		{f(nan), 0, one},
		{f(snan), 1, nan},
		{f(snan), 2, nan},
		{f(snan), 0.5, nan},
		{1, f(snan), nan},
		{2, f(snan), nan},
	} {
		if got := PowLinux(c.x, c.y); !mathSame(got, c.want) {
			t.Errorf("PowLinux(%x, %x) = %v (%016x), Chromium has %v (%016x)", c.x, c.y, got, math.Float64bits(got), f(c.want), c.want)
		}
	}
	if got := linuxPow(f(snan), 0); !math.IsNaN(got) {
		t.Errorf("linuxPow(signaling NaN, 0) = %v, the pow of glibc returns NaN", got)
	}
}
