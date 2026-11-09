package js

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// TestCorrectRounding compares SinCR, CosCR, TanCR, AsinCR, AtanCR, AcosCR
// and Atan2CR with the correctly rounded values that mpmath computed with
// 600 bits (tools/reference/gen-d3-trig-cr.py). The d3 port calls these
// functions.
func TestCorrectRounding(t *testing.T) {
	var g struct{ Trig, Asin, Atan, Atan2 string }
	loadGzipJSON(t, "testdata/d3_trig_cr.json.gz", &g)
	trig := decodeFloatList(t, g.Trig)
	if len(trig) < 4*5000 {
		t.Fatalf("only %d values", len(trig))
	}
	goSin, goCos, goTan := 0, 0, 0
	for i := 0; i+4 <= len(trig); i += 4 {
		x, sin, cos, tan := trig[i], trig[i+1], trig[i+2], trig[i+3]
		if got := SinCR(x); !sameFloat(got, sin) {
			t.Errorf("SinCR(%v) = %v, want %v", x, got, sin)
		}
		if got := CosCR(x); !sameFloat(got, cos) {
			t.Errorf("CosCR(%v) = %v, want %v", x, got, cos)
		}
		if got := TanCR(x); !sameFloat(got, tan) {
			t.Errorf("TanCR(%v) = %v, want %v", x, got, tan)
		}
		if !sameFloat(Sin(x), sin) {
			goSin++
		}
		if !sameFloat(Cos(x), cos) {
			goCos++
		}
		if !sameFloat(Tan(x), tan) {
			goTan++
		}
	}
	t.Logf("%d arguments: js.Sin is not correctly rounded for %d, js.Cos for %d, js.Tan for %d", len(trig)/4, goSin, goCos, goTan)

	asin := decodeFloatList(t, g.Asin)
	goAsin, goAcos := 0, 0
	for i := 0; i+3 <= len(asin); i += 3 {
		if got := AsinCR(asin[i]); !sameFloat(got, asin[i+1]) {
			t.Errorf("AsinCR(%v) = %v, want %v", asin[i], got, asin[i+1])
		}
		if got := AcosCR(asin[i]); !sameFloat(got, asin[i+2]) {
			t.Errorf("AcosCR(%v) = %v, want %v", asin[i], got, asin[i+2])
		}
		if !sameFloat(Asin(asin[i]), asin[i+1]) {
			goAsin++
		}
		if !sameFloat(Acos(asin[i]), asin[i+2]) {
			goAcos++
		}
	}
	atan := decodeFloatList(t, g.Atan)
	goAtan := 0
	for i := 0; i+2 <= len(atan); i += 2 {
		if got := AtanCR(atan[i]); !sameFloat(got, atan[i+1]) {
			t.Errorf("AtanCR(%v) = %v, want %v", atan[i], got, atan[i+1])
		}
		if !sameFloat(Atan(atan[i]), atan[i+1]) {
			goAtan++
		}
	}
	atan2 := decodeFloatList(t, g.Atan2)
	goAtan2 := 0
	for i := 0; i+3 <= len(atan2); i += 3 {
		y, x, want := atan2[i], atan2[i+1], atan2[i+2]
		if got := Atan2CR(y, x); !sameFloat(got, want) {
			t.Errorf("Atan2CR(%v, %v) = %v, want %v", y, x, got, want)
		}
		if !sameFloat(Atan2(y, x), want) {
			goAtan2++
		}
	}
	t.Logf("not correctly rounded: js.Asin for %d and js.Acos for %d of %d arguments, js.Atan for %d of %d, js.Atan2 for %d of %d",
		goAsin, goAcos, len(asin)/3, goAtan, len(atan)/2, goAtan2, len(atan2)/3)
	if !sameFloat(AcosCR(2), math.NaN()) || !sameFloat(AcosCR(math.NaN()), math.NaN()) || !sameFloat(Atan2CR(math.NaN(), 1), math.NaN()) {
		t.Error("AcosCR and Atan2CR must return NaN outside of their domain")
	}
}

// TestSinCosChromium compares SinCR and CosCR with the values that Chromium
// computed for the reference renders of the pie and radar charts
// (tools/reference/gen-d3-chromium-trig.mjs).
func TestSinCosChromium(t *testing.T) {
	data, err := os.ReadFile("testdata/d3_chromium_trig.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Cases []struct {
			Src, A, F, X, Y string
			NodeAgrees      bool
		}
	}
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Cases) < 50 {
		t.Fatalf("only %d cases", len(g.Cases))
	}
	node, wrappers := 0, 0
	for _, c := range g.Cases {
		a, f, x, y := fromBits(t, c.A), fromBits(t, c.F), fromBits(t, c.X), fromBits(t, c.Y)
		if got := float64(CosCR(a) * f); !sameFloat(got, x) {
			t.Errorf("%s: cos(%v) * %v = %v, Chromium has %v", c.Src, a, f, got, x)
		}
		if got := float64(SinCR(a) * f); !sameFloat(got, y) {
			t.Errorf("%s: sin(%v) * %v = %v, Chromium has %v", c.Src, a, f, got, y)
		}
		if c.NodeAgrees {
			node++
		}
		if sameFloat(float64(Cos(a)*f), x) && sameFloat(float64(Sin(a)*f), y) {
			wrappers++
		}
	}
	t.Logf("%d positions from Chromium: SinCR and CosCR give all of them, Node.js gives %d, js.Sin and js.Cos give %d", len(g.Cases), node, wrappers)
}
