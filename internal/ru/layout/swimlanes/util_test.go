package swimlanes

import (
	"math"
	"math/rand"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// TestToFixed compares the fast toFixed with js.ToFixed, which has its own
// golden test against V8. The inputs have many ties (multiples of 1/64).
func TestToFixed(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	values := []float64{0, math.Copysign(0, -1), 0.5, -0.5, 1.5, 2.5, 0.05, 0.25, 0.0625, -0.0625, 0.0005, -0.0004, 1e15, -1e15, 1e21, 1e-7, math.NaN(), math.Inf(1), math.Inf(-1)}
	for i := 0; i < 20000; i++ {
		switch i % 4 {
		case 0:
			values = append(values, float64(rng.Intn(1<<20)-(1<<19))/64)
		case 1:
			values = append(values, float64(rng.Intn(1<<16)-(1<<15))/16)
		case 2:
			values = append(values, float64(rng.Float64()*2000)-1000)
		default:
			values = append(values, float64(rng.Intn(4001)-2000)/1000)
		}
	}
	// The numbers around a change of the third digit, and the numbers whose
	// product with 1000 is not exact. The last group is too large for the
	// path of three digits.
	for i := 0; i < 12000; i++ {
		var limit float64
		switch i % 3 {
		case 0:
			limit = (float64(rng.Intn(4_000_000)) + 0.5) / 1000
		case 1:
			limit = (float64(rng.Int63n(1_000_000_000_000_000)) + 0.5) / 1000
		default:
			limit = (float64(rng.Int63n(4_000_000_000_000_000)) + 0.5) / 1000
		}
		down, up := limit, limit
		values = append(values, limit, -limit)
		for step := 0; step < 2; step++ {
			down, up = math.Nextafter(down, 0), math.Nextafter(up, math.Inf(1))
			values = append(values, down, -down, up, -up)
		}
	}
	for _, v := range values {
		for _, digits := range []int{0, 1, 3} {
			if got, want := toFixed(v, digits), js.ToFixed(v, digits); got != want {
				t.Fatalf("toFixed(%v, %d) = %q, want %q", v, digits, got, want)
			}
		}
	}
}

// TestPathKey3 compares the keys that are written into one buffer with the
// keys by their definition: toFixed of each coordinate, joined.
func TestPathKey3(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	coordinate := func() float64 {
		switch rng.Intn(8) {
		case 0:
			return float64(rng.Intn(1<<20)-(1<<19)) / 64
		case 1:
			return []float64{0, math.Copysign(0, -1), 0.0005, -0.0005, 1e15, -1e16, 1e21, math.NaN(), math.Inf(1), math.Inf(-1)}[rng.Intn(10)]
		case 2:
			return float64(rng.Intn(4001)-2000) / 1000
		}
		return float64(rng.Float64()*4000) - 2000
	}
	for i := 0; i < 20000; i++ {
		points := make([]Point, rng.Intn(30))
		want := ""
		for j := range points {
			points[j] = Point{X: coordinate(), Y: coordinate()}
			if j > 0 {
				want += "|"
			}
			want += js.ToFixed(points[j].X, 3) + "," + js.ToFixed(points[j].Y, 3)
		}
		if got := pathKey3(points); got != want {
			t.Fatalf("pathKey3(%v) = %q, want %q", points, got, want)
		}
		if len(points) > 0 {
			p := points[0]
			if got, want := pointKey3(p), js.ToFixed(p.X, 3)+","+js.ToFixed(p.Y, 3); got != want {
				t.Fatalf("pointKey3(%v) = %q, want %q", p, got, want)
			}
		}
	}
}
