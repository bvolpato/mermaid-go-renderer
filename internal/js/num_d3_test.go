package js

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// loadGzipJSON reads a golden file that tools/reference/gen-d3-*.mjs wrote.
func loadGzipJSON(t *testing.T, path string, v any) {
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
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

// TestToExponentialAndToPrecision compares with values recorded from V8 by
// tools/reference/gen-d3-jsnum.mjs.
func TestToExponentialAndToPrecision(t *testing.T) {
	var g struct {
		Groups struct {
			Nums []struct {
				X  string
				E0 string
				E  int
				Es string
				P  int
				Ps string
			}
		}
	}
	loadGzipJSON(t, "testdata/d3_num.json.gz", &g)
	if len(g.Groups.Nums) < 5000 {
		t.Fatalf("only %d cases", len(g.Groups.Nums))
	}
	for _, c := range g.Groups.Nums {
		x := fromBits(t, c.X)
		if got := ToExponential(x, -1); got != c.E0 {
			t.Errorf("ToExponential(%v) = %q, want %q", x, got, c.E0)
		}
		if got := ToExponential(x, c.E); got != c.Es {
			t.Errorf("ToExponential(%v, %d) = %q, want %q", x, c.E, got, c.Es)
		}
		if got := ToPrecision(x, c.P); got != c.Ps {
			t.Errorf("ToPrecision(%v, %d) = %q, want %q", x, c.P, got, c.Ps)
		}
	}
}
