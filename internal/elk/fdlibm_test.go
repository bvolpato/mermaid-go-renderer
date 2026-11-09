package elk

import (
	"compress/gzip"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// mathGolden is testdata/math.json.gz, written by tools/reference/gen-elk.mjs
// --math. It has arguments and results as bit patterns. The results come from
// tools/reference/elk-fdlibm.mjs, the fdlibm functions of V8 in JavaScript,
// which is also what the golden layouts were computed with. Native has the
// number of rows for which the native Math function of the node build that
// wrote the file differs from fdlibm.
type mathGolden struct {
	V8     string      `json:"v8"`
	Arch   string      `json:"arch"`
	Sin    [][2]string `json:"sin"`
	Cos    [][2]string `json:"cos"`
	Acos   [][2]string `json:"acos"`
	Log    [][2]string `json:"log"`
	Atan2  [][3]string `json:"atan2"`
	Pow    [][3]string `json:"pow"`
	Native map[string]struct {
		Rows   int `json:"rows"`
		Differ int `json:"differ"`
		MaxUlp int `json:"maxUlp"`
	} `json:"native"`
}

func loadMathGolden(t testing.TB) *mathGolden {
	t.Helper()
	f, err := os.Open("testdata/math.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var g mathGolden
	if err := json.NewDecoder(zr).Decode(&g); err != nil {
		t.Fatal(err)
	}
	return &g
}

func fromHex(t testing.TB, s string) float64 {
	u, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return math.Float64frombits(u)
}

// sameBits compares two results. Every NaN is the same result: JavaScript
// has one NaN value.
func sameBits(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	return math.Float64bits(a) == math.Float64bits(b)
}

// TestFdlibmMatchesV8 compares the ports with the fdlibm functions of V8 in
// JavaScript, bit for bit. It also reports how often three other sets of
// functions differ from fdlibm: the native functions of node, the Go math
// package (the Math of goja), and the functions of internal/js for Chromium
// (the Math that a program gets).
func TestFdlibmMatchesV8(t *testing.T) {
	g := loadMathGolden(t)
	for name, n := range g.Native {
		t.Logf("native Math.%s of node (V8 %s, %s): %d of %d rows differ from fdlibm, at most %d ulp", name, g.V8, g.Arch, n.Differ, n.Rows, n.MaxUlp)
		// The native functions are the same algorithm, compiled with or
		// without fused multiply-add (pow: or the pow of the C library).
		// A larger distance means that the reference itself is wrong.
		if n.MaxUlp > 1 {
			t.Errorf("native Math.%s differs from the reference by %d ulp", name, n.MaxUlp)
		}
	}
	unary := []struct {
		name string
		rows [][2]string
		fn   func(float64) float64
		goFn func(float64) float64
		crFn func(float64) float64
	}{
		{"sin", g.Sin, fdlibmSin, math.Sin, js.Sin},
		{"cos", g.Cos, fdlibmCos, math.Cos, js.Cos},
		{"acos", g.Acos, fdlibmAcos, math.Acos, js.Acos},
		{"log", g.Log, fdlibmLog, math.Log, js.Log},
	}
	for _, u := range unary {
		if len(u.rows) < 1000 {
			t.Errorf("%s: only %d golden rows", u.name, len(u.rows))
		}
		bad, goBad, crBad := 0, 0, 0
		for _, r := range u.rows {
			x, want := fromHex(t, r[0]), fromHex(t, r[1])
			if !sameBits(u.crFn(x), want) {
				crBad++
			}
			if got := u.fn(x); !sameBits(got, want) {
				if bad < 5 {
					t.Errorf("%s(%v) = %v (%#x), fdlibm in V8 has %v (%#x)", u.name, x, got, math.Float64bits(got), want, math.Float64bits(want))
				}
				bad++
			}
			if !sameBits(u.goFn(x), want) {
				goBad++
			}
		}
		t.Logf("%s: %d rows, %d differences. The Go math package differs in %d rows, the function of internal/js (Chromium) in %d rows.", u.name, len(u.rows), bad, goBad, crBad)
		if bad > 0 {
			t.Errorf("%s: %d of %d results differ from fdlibm in V8", u.name, bad, len(u.rows))
		}
	}
	binary := []struct {
		name string
		rows [][3]string
		fn   func(float64, float64) float64
		goFn func(float64, float64) float64
		crFn func(float64, float64) float64
	}{
		{"atan2", g.Atan2, fdlibmAtan2, math.Atan2, js.Atan2},
		{"pow", g.Pow, fdlibmPow, math.Pow, js.Pow},
	}
	for _, b := range binary {
		if len(b.rows) < 1000 {
			t.Errorf("%s: only %d golden rows", b.name, len(b.rows))
		}
		bad, goBad, crBad := 0, 0, 0
		for _, r := range b.rows {
			x, y, want := fromHex(t, r[0]), fromHex(t, r[1]), fromHex(t, r[2])
			if !sameBits(b.crFn(x, y), want) {
				crBad++
			}
			if got := b.fn(x, y); !sameBits(got, want) {
				if bad < 5 {
					t.Errorf("%s(%v, %v) = %v (%#x), fdlibm in V8 has %v (%#x)", b.name, x, y, got, math.Float64bits(got), want, math.Float64bits(want))
				}
				bad++
			}
			if !sameBits(b.goFn(x, y), want) {
				goBad++
			}
		}
		t.Logf("%s: %d rows, %d differences. The Go math package differs in %d rows, the function of internal/js (Chromium) in %d rows.", b.name, len(b.rows), bad, goBad, crBad)
		if bad > 0 {
			t.Errorf("%s: %d of %d results differ from fdlibm in V8", b.name, bad, len(b.rows))
		}
	}
}

// TestTwoOverPiTable checks the table of 2/pi against a value that the test
// computes with Machin's formula.
func TestTwoOverPiTable(t *testing.T) {
	const prec = 1800
	// pi = 16*atan(1/5) - 4*atan(1/239)
	atanInv := func(n int64) *big.Float {
		x := new(big.Float).SetPrec(prec).Quo(big.NewFloat(1).SetPrec(prec), new(big.Float).SetPrec(prec).SetInt64(n))
		x2 := new(big.Float).SetPrec(prec).Mul(x, x)
		sum := new(big.Float).SetPrec(prec).Set(x)
		term := new(big.Float).SetPrec(prec).Set(x)
		for k := int64(3); ; k += 2 {
			term.Mul(term, x2)
			d := new(big.Float).SetPrec(prec).Quo(term, new(big.Float).SetPrec(prec).SetInt64(k))
			if d.MantExp(nil) < -prec-8 {
				break
			}
			if (k/2)%2 == 1 {
				sum.Sub(sum, d)
			} else {
				sum.Add(sum, d)
			}
		}
		return sum
	}
	pi := new(big.Float).SetPrec(prec).Mul(big.NewFloat(16).SetPrec(prec), atanInv(5))
	pi.Sub(pi, new(big.Float).SetPrec(prec).Mul(big.NewFloat(4).SetPrec(prec), atanInv(239)))
	v := new(big.Float).SetPrec(prec).Quo(big.NewFloat(2).SetPrec(prec), pi)
	for i, want := range twoOverPi {
		v.Mul(v, big.NewFloat(1<<24).SetPrec(prec))
		digit, _ := v.Int64()
		if int32(digit) != want {
			t.Fatalf("twoOverPi[%d] = %#x, computed %#x", i, want, digit)
		}
		v.Sub(v, new(big.Float).SetPrec(prec).SetInt64(digit))
	}
	for n, want := range npio2Hw {
		got := hiWord(float64(n+1) * (math.Pi / 2))
		if got != want {
			t.Errorf("npio2Hw[%d] = %#x, computed %#x", n, want, got)
		}
	}
}
