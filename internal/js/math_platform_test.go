package js

import (
	"math"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js/crmath"
)

// TestMathPlatform checks that Pow and Tanh follow the platform switch, also
// through a function value that a caller took before the switch. The
// engines that run JavaScript install such function values as Math.pow and
// Math.tanh.
func TestMathPlatform(t *testing.T) {
	defer SetPlatform(CurrentPlatform())
	pow, tanh := Pow, Tanh
	same := func(a, b float64) bool {
		return math.Float64bits(a) == math.Float64bits(b) || (math.IsNaN(a) && math.IsNaN(b))
	}
	// The arguments: random numbers from a xorshift generator. Linux and
	// macOS differ for about 8 percent of the results of tanh and for about
	// 0.1 percent of the results of pow.
	s := uint64(0x6d6d6467)
	next := func() float64 {
		s ^= s << 13
		s ^= s >> 7
		s ^= s << 17
		return float64(s>>11) / (1 << 53)
	}
	type pair struct{ x, y float64 }
	var pows []pair
	var tanhs []float64
	for i := 0; i < 20000; i++ {
		pows = append(pows, pair{float64(next() * 100), float64(next()*8) - 4})
		tanhs = append(tanhs, float64(next()*10)-5)
	}
	for _, c := range []struct {
		platform Platform
		pow      func(x, y float64) float64
		tanh     func(x float64) float64
		crPow    func(x, y float64) float64
		crTanh   func(x float64) float64
	}{
		{PlatformLinux, PowLinux, TanhLinux, crmath.PowLinux, crmath.TanhLinux},
		{PlatformDarwin, PowDarwin, TanhDarwin, crmath.PowDarwin, crmath.TanhDarwin},
	} {
		SetPlatform(c.platform)
		if got := CurrentPlatform(); got != c.platform || crmath.CurrentPlatform() != c.platform {
			t.Fatalf("the current platform is %v after SetPlatform(%v)", got, c.platform)
		}
		for _, a := range pows {
			want := c.crPow(a.x, a.y)
			if got := Pow(a.x, a.y); !same(got, want) {
				t.Fatalf("%v: Pow(%v, %v) = %v, crmath has %v", c.platform, a.x, a.y, got, want)
			}
			if got := pow(a.x, a.y); !same(got, want) {
				t.Fatalf("%v: the function value of Pow gives %v for (%v, %v), crmath has %v", c.platform, got, a.x, a.y, want)
			}
			if got := c.pow(a.x, a.y); !same(got, want) {
				t.Fatalf("%v: the function of the platform gives %v for (%v, %v), crmath has %v", c.platform, got, a.x, a.y, want)
			}
		}
		for _, x := range tanhs {
			want := c.crTanh(x)
			if got := Tanh(x); !same(got, want) {
				t.Fatalf("%v: Tanh(%v) = %v, crmath has %v", c.platform, x, got, want)
			}
			if got := tanh(x); !same(got, want) {
				t.Fatalf("%v: the function value of Tanh gives %v for %v, crmath has %v", c.platform, got, x, want)
			}
			if got := c.tanh(x); !same(got, want) {
				t.Fatalf("%v: the function of the platform gives %v for %v, crmath has %v", c.platform, got, x, want)
			}
		}
	}
	if !crmath.HasDarwinLibm() {
		return
	}
	powDiffer, tanhDiffer := 0, 0
	for _, a := range pows {
		if !same(PowLinux(a.x, a.y), PowDarwin(a.x, a.y)) {
			powDiffer++
		}
	}
	for _, x := range tanhs {
		if !same(TanhLinux(x), TanhDarwin(x)) {
			tanhDiffer++
		}
	}
	if powDiffer == 0 || tanhDiffer == 0 {
		t.Errorf("Linux and macOS differ for %d results of pow and %d results of tanh: the test has no argument that tells them apart", powDiffer, tanhDiffer)
	}
	t.Logf("Linux and macOS differ for %d of %d results of pow and %d of %d results of tanh", powDiffer, len(pows), tanhDiffer, len(tanhs))
}
