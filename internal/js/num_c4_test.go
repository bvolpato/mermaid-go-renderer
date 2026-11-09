package js

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

// TestParseIntAuto compares ParseIntAuto with parseInt of V8 for the cases
// of testdata/c4_parseint.json, which tools/reference/gen-c4-parseint.mjs
// writes. The results are compared by bits. Each NaN is the same value.
func TestParseIntAuto(t *testing.T) {
	data, err := os.ReadFile("testdata/c4_parseint.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases [][2]string
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 800 {
		t.Fatalf("only %d cases", len(cases))
	}
	for _, c := range cases {
		u, err := strconv.ParseUint(c[1], 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		want := math.Float64frombits(u)
		got := ParseIntAuto(c[0])
		if math.IsNaN(want) && math.IsNaN(got) {
			continue
		}
		if math.Float64bits(got) != u {
			t.Errorf("ParseIntAuto(%q) = %v, want %v", c[0], got, want)
		}
	}
}
