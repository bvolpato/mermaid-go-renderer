package browser

import (
	"math"
	"testing"
)

// TestLinuxTrigAgainstCLibrary compares linuxSinf, linuxCosf, linuxTanf and
// linuxAtan2f with results of the C library of the reference Linux
// environment (the Playwright image for arm64), bit for bit. The data comes
// from tools/reference/gen-browser-linux-libm.mjs, which calls the library
// in the image.
func TestLinuxTrigAgainstCLibrary(t *testing.T) {
	var data struct {
		Sinf   [][2]uint32 `json:"sinf"`
		Cosf   [][2]uint32 `json:"cosf"`
		Tanf   [][2]uint32 `json:"tanf"`
		Atan2f [][3]uint32 `json:"atan2f"`
	}
	if !readLinuxJSON(t, "linux-libm.json.gz", &data) {
		t.Skip("no data")
	}
	same := func(got float32, want uint32) bool {
		if got != got {
			w := math.Float32frombits(want)
			return w != w
		}
		return math.Float32bits(got) == want
	}
	check := func(name string, cases [][2]uint32, f func(float32) float32) {
		bad := 0
		for _, c := range cases {
			x := math.Float32frombits(c[0])
			if got := f(x); !same(got, c[1]) {
				bad++
				if bad <= 5 {
					t.Errorf("%s(%v [%#x]) = %v [%#x], want %v [%#x]", name, x, c[0], got, math.Float32bits(got), math.Float32frombits(c[1]), c[1])
				}
			}
		}
		t.Logf("%s: %d cases, %d exact", name, len(cases), len(cases)-bad)
		if bad > 5 {
			t.Errorf("%s: %d results differ", name, bad)
		}
	}
	check("sinf", data.Sinf, linuxSinf)
	check("cosf", data.Cosf, linuxCosf)
	check("tanf", data.Tanf, linuxTanf)
	bad := 0
	for _, c := range data.Atan2f {
		y, x := math.Float32frombits(c[0]), math.Float32frombits(c[1])
		if got := linuxAtan2f(y, x); !same(got, c[2]) {
			bad++
			if bad <= 5 {
				t.Errorf("atan2f(%v, %v) = %v [%#x], want %v [%#x]", y, x, got, math.Float32bits(got), math.Float32frombits(c[2]), c[2])
			}
		}
	}
	t.Logf("atan2f: %d cases, %d exact", len(data.Atan2f), len(data.Atan2f)-bad)
	if bad > 5 {
		t.Errorf("atan2f: %d results differ", bad)
	}
}
