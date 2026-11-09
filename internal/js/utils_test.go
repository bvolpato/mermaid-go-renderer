package js

import (
	"math"
	"testing"
)

// The golden file testdata/utils.json.gz comes from
// tools/reference/oracle-utils.mjs: it has the results of
// Number.prototype.toString(radix), escape and unescape in Chromium.

// goldenNumber reads a number of the golden file. JSON cannot hold NaN, the
// infinities and -0, so they have the tagged form {"$n": "<text>"}.
func goldenNumber(t *testing.T, v any) float64 {
	t.Helper()
	switch x := v.(type) {
	case float64:
		return x
	case map[string]any:
		switch x["$n"] {
		case "NaN":
			return math.NaN()
		case "Infinity":
			return math.Inf(1)
		case "-Infinity":
			return math.Inf(-1)
		case "-0":
			return math.Copysign(0, -1)
		}
	}
	t.Fatalf("not a number: %v", v)
	return 0
}

func TestToStringRadix(t *testing.T) {
	var g struct {
		ToStringRadix []struct {
			X     any
			Radix int
			Out   string
		}
	}
	loadGzipJSON(t, "testdata/utils.json.gz", &g)
	if len(g.ToStringRadix) < 3000 {
		t.Fatalf("only %d cases", len(g.ToStringRadix))
	}
	fails := 0
	for _, c := range g.ToStringRadix {
		x := goldenNumber(t, c.X)
		if got := ToStringRadix(x, c.Radix); got != c.Out {
			fails++
			if fails <= 10 {
				t.Errorf("(%v).toString(%d): got %q, want %q", x, c.Radix, got, c.Out)
			}
		}
	}
	if fails > 0 {
		t.Errorf("%d of %d cases fail", fails, len(g.ToStringRadix))
	}
	if got := ToStringRadix(1, 37); got != "" {
		t.Errorf("a radix out of range gives %q", got)
	}
}

func TestEscapeAndUnescape(t *testing.T) {
	var g struct {
		Escape []struct {
			S   string
			Out string
		}
		Unescape []struct {
			S string
			// Out is the result as UTF-16 code units, because the result
			// can have a lone surrogate.
			Out []rune
		}
	}
	loadGzipJSON(t, "testdata/utils.json.gz", &g)
	if len(g.Escape) < 1500 || len(g.Unescape) < 1500 {
		t.Fatalf("only %d and %d cases", len(g.Escape), len(g.Unescape))
	}
	fails := 0
	for _, c := range g.Escape {
		if got := Escape(c.S); got != c.Out {
			fails++
			if fails <= 10 {
				t.Errorf("escape(%q): got %q, want %q", c.S, got, c.Out)
			}
		}
		// unescape reverts escape.
		if got := Unescape(c.Out); got != c.S {
			fails++
			if fails <= 10 {
				t.Errorf("unescape(%q): got %q, want %q", c.Out, got, c.S)
			}
		}
	}
	for _, c := range g.Unescape {
		want := FromUTF16Units(c.Out)
		if got := Unescape(c.S); got != want {
			fails++
			if fails <= 10 {
				t.Errorf("unescape(%q): got %q, want %q", c.S, got, want)
			}
		}
	}
	if fails > 0 {
		t.Errorf("%d cases fail", fails)
	}
}
