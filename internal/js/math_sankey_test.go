package js

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js/crmath"
)

// The d3-sankey port calls Pow for Math.pow(0.99, i). These tests compare
// Pow with the Math.pow of Node.js and of the Chromium of the browser
// oracle for integer exponents, bit for bit. Both golden files are from
// macOS, so the tests call PowDarwin, the function of that platform.

// skipWithoutDarwinPow skips a test of the pow of macOS when the package
// crmath is built without that function.
func skipWithoutDarwinPow(t *testing.T) {
	t.Helper()
	if !crmath.HasDarwinLibm() {
		t.Skip("the golden file is from macOS, and the package crmath does not have the pow of macOS")
	}
}

// powGolden is testdata/math_sankey.json, written by
// tools/reference/gen-sankey-pow.mjs. A row has the bits of the base, the
// exponent, the bits of Math.pow in Node.js, and the bits of the correctly
// rounded power, which the script computes with exact BigInt arithmetic.
type powGolden struct {
	Node   string
	V8     string
	Arch   string
	Cases  [][4]any
	Sankey [][4]any
}

func powRow(t *testing.T, row [4]any) (x float64, n int, native, cr float64) {
	t.Helper()
	parse := func(v any) float64 {
		u, err := strconv.ParseUint(v.(string), 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		return math.Float64frombits(u)
	}
	return parse(row[0]), int(row[1].(float64)), parse(row[2]), parse(row[3])
}

// TestPowIntegerExponent compares Pow with Math.pow of Node.js for integer
// exponents: random bases, powers that are exactly a float64 or the middle
// of two, results near overflow and underflow, and the calls of d3-sankey.
// It also counts the results that are not correctly rounded: the pow of the
// C library that V8 calls is not correctly rounded for every argument.
func TestPowIntegerExponent(t *testing.T) {
	skipWithoutDarwinPow(t)
	data, err := os.ReadFile("testdata/math_sankey.json")
	if err != nil {
		t.Fatal(err)
	}
	var g powGolden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Cases) < 8000 || len(g.Sankey) != 1000 {
		t.Fatalf("golden file has %d cases and %d sankey calls", len(g.Cases), len(g.Sankey))
	}
	for name, rows := range map[string][][4]any{"cases": g.Cases, "Math.pow(0.99, i)": g.Sankey} {
		notRounded := 0
		for _, row := range rows {
			x, n, native, cr := powRow(t, row)
			if got := PowDarwin(x, float64(n)); !sameFloat(got, native) {
				t.Errorf("PowDarwin(%v, %d) = %v (%x), Math.pow in Node.js is %v (%x)", x, n, got, math.Float64bits(got), native, math.Float64bits(native))
			}
			if !sameFloat(native, cr) {
				notRounded++
				if d := int64(math.Float64bits(native)) - int64(math.Float64bits(cr)); d < -1 || d > 1 {
					t.Errorf("Math.pow(%v, %d) = %v in Node.js, the correctly rounded value is %v", x, n, native, cr)
				}
			}
		}
		t.Logf("%s: %d rows equal to Node.js %s (V8 %s, %s); %d of them are not correctly rounded", name, len(rows), g.Node, g.V8, g.Arch, notRounded)
	}
}

// TestPowSankeyChromium compares Pow with Math.pow(0.99, i) of the Chromium
// of the browser oracle, which made the reference renders
// (testdata/math_sankey_chromium.json, from
// tools/reference/gen-sankey-pow-chromium.mjs). The sankey layout of mermaid
// has six iterations, so it calls Math.pow for the exponents 0 to 5.
func TestPowSankeyChromium(t *testing.T) {
	skipWithoutDarwinPow(t)
	data, err := os.ReadFile("testdata/math_sankey_chromium.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		UserAgent string   `json:"userAgent"`
		Pow       []string `json:"pow"`
	}
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Pow) != 1000 {
		t.Fatalf("%d values", len(g.Pow))
	}
	for i, hex := range g.Pow {
		u, err := strconv.ParseUint(hex, 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		want := math.Float64frombits(u)
		if got := PowDarwin(0.99, float64(i)); !sameFloat(got, want) {
			t.Errorf("PowDarwin(0.99, %d) = %v, Chromium has %v", i, got, want)
		}
	}
	t.Logf("Math.pow(0.99, i) for i < %d: equal to %s", len(g.Pow), g.UserAgent)
}
