package jsm_test

import (
	"sort"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
)

type golden struct {
	Pow10, Log10, Special d3test.FS
	Samples               map[string]d3test.FS
}

func load(t *testing.T) golden {
	var g golden
	d3test.Load(t, "testdata/v8math.json.gz", &g)
	return g
}

// TestPowersOfTen checks the arguments for which jsm has exact results of its
// own: Math.pow(10, n) for every integer n, Math.log10 of a power of ten, and
// the special values of Math.pow. The golden values come from Node.js
// (tools/reference/gen-d3-jsm.mjs).
func TestPowersOfTen(t *testing.T) {
	g := load(t)
	if len(g.Pow10) != 2*641 || len(g.Log10) != 2*641 {
		t.Fatalf("got %d and %d values", len(g.Pow10), len(g.Log10))
	}
	for i := 0; i < len(g.Pow10); i += 2 {
		n, want := g.Pow10[i], g.Pow10[i+1]
		if got := jsm.Pow(10, n); !d3test.Same(got, want) {
			t.Errorf("Pow(10, %v) = %v, want %v", n, got, want)
		}
	}
	inexact := 0
	for i := 0; i < len(g.Log10); i += 2 {
		x, want := g.Log10[i], g.Log10[i+1]
		if got := jsm.Log10(x); !d3test.Same(got, want) {
			// Node.js is not exact for the subnormal powers, and jsm then
			// uses the wrapper of internal/js.
			if x >= 1e-311 {
				t.Errorf("Log10(%v) = %v, want %v", x, got, want)
			}
			inexact++
		}
	}
	t.Logf("Log10 of the subnormal powers of ten differs from Node.js in %d cases", inexact)
	for i := 0; i < len(g.Special); i += 3 {
		x, y, want := g.Special[i], g.Special[i+1], g.Special[i+2]
		if got := jsm.Pow(x, y); !d3test.Same(got, want) {
			t.Errorf("Pow(%v, %v) = %v, want %v", x, y, got, want)
		}
	}
}

// TestMeasureNodeMath measures how often the functions of jsm differ from
// the Math functions of Node.js on random arguments. It does not fail. For
// sin, cos, tan, asin, acos, atan and atan2, jsm has the correctly rounded
// value and Node.js does not, so the rate is the error rate of Node.js. For
// the other functions, jsm has Go's math package, which differs from Node.js
// and from a correctly rounded result. The golden tests of each d3 package
// prove case by case that their mismatches come only from these differences.
func TestMeasureNodeMath(t *testing.T) {
	g := load(t)
	unary := map[string]func(float64) float64{
		"sin": jsm.Sin, "cos": jsm.Cos, "tan": jsm.Tan, "asin": jsm.Asin, "acos": jsm.Acos, "atan": jsm.Atan,
		"exp": jsm.Exp, "log": jsm.Log, "log10": jsm.Log10, "log2": jsm.Log2, "cbrt": jsm.Cbrt,
	}
	binary := map[string]func(float64, float64) float64{"atan2": jsm.Atan2, "pow": jsm.Pow}
	var names []string
	for name := range g.Samples {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		vals := g.Samples[name]
		n, differ := 0, 0
		var max int64
		step := 2
		if _, ok := binary[name]; ok {
			step = 3
		}
		for i := 0; i+step <= len(vals); i += step {
			var got float64
			if step == 2 {
				got = unary[name](vals[i])
			} else {
				got = binary[name](vals[i], vals[i+1])
			}
			n++
			if u := d3test.Ulps(got, vals[i+step-1]); u != 0 {
				differ++
				if u > max {
					max = u
				}
			}
		}
		t.Logf("%-6s differs from Node.js in %4d of %d random calls (%.1f%%), max %d ulp", name, differ, n, 100*float64(differ)/float64(n), max)
	}
}
