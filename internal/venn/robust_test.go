package venn

import (
	"errors"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
)

// malformedAreas makes data that mermaid does not make: areas of sets that
// have no circle, areas with no set, sizes that are not finite, and the set
// ids that are properties of Object.prototype.
func malformedAreas(rnd *rand.Rand) []*Area {
	ids := []string{"A", "B", "C", "D", "E", "", "1", "10", "constructor", "__proto__", "toString", "a,b", "é😀"}
	sizes := []float64{0, math.Copysign(0, -1), -1, 10, 2.5, 1e300, 1e-300, 5e-324, math.NaN(), math.Inf(1), math.Inf(-1)}
	areas := []*Area{}
	for n := rnd.Intn(9); n > 0; n-- {
		a := &Area{Sets: []string{}}
		k := rnd.Intn(5)
		if rnd.Intn(3) > 0 {
			k = 1 + rnd.Intn(2)
		}
		for ; k > 0; k-- {
			a.Sets = append(a.Sets, ids[rnd.Intn(len(ids))])
		}
		if rnd.Intn(2) == 0 {
			a.Size = sizes[rnd.Intn(len(sizes))]
		} else {
			a.Size = rnd.Float64() * 30
		}
		if rnd.Intn(4) == 0 {
			a.Label, a.HasLabel = []string{"", "L", "two words", " lead", "trail "}[rnd.Intn(5)], true
		}
		if rnd.Intn(8) == 0 {
			a.Weight, a.HasWeight = sizes[rnd.Intn(len(sizes))], true
		}
		areas = append(areas, a)
	}
	return areas
}

// TestMalformedInput runs the exported functions on seeded malformed data.
// A call must return, with a result or an error, and must not panic.
func TestMalformedInput(t *testing.T) {
	rnd := rand.New(rand.NewSource(0x76656e6e))
	failed := 0
	const n = 1500
	for i := 0; i < n; i++ {
		areas := malformedAreas(rnd)
		width := []float64{800, 0, -5, math.NaN(), math.Inf(1), 1e-300}[rnd.Intn(6)]
		done := make(chan error, 2)
		go func() {
			env := &Env{Random: rnd.Float64}
			_, err := Layout(env, areas, LayoutOptions{Width: width, Height: 450, Padding: 8})
			done <- err
			doc := browser.NewDocument()
			_, err = Diagram(env, doc.CreateElement("div"), areas, DiagramOptions{Width: width, Height: "450"})
			done <- err
		}()
		for j := 0; j < 2; j++ {
			select {
			case err := <-done:
				if err != nil {
					failed++
					var e *Error
					if !errors.As(err, &e) {
						t.Errorf("input %d: the error %v is not an *Error", i, err)
					}
				}
			case <-time.After(60 * time.Second):
				t.Fatalf("input %d: the call does not return", i)
			}
		}
	}
	if failed == 0 || failed == 2*n {
		t.Fatalf("%d of %d calls fail: the inputs do not cover both outcomes", failed, 2*n)
	}
	t.Logf("%d calls: %d return an error, none panics", 2*n, failed)
}

// TestMaxWork checks the bound of the work: a diagram of many sets fails
// with an error at once, where venn.js runs for minutes.
func TestMaxWork(t *testing.T) {
	areas := []*Area{}
	for i := 0; i < 400; i++ {
		areas = append(areas, &Area{Sets: []string{"s" + string(rune('a'+i%26)) + string(rune('a'+i/26))}, Size: 10})
	}
	start := time.Now()
	_, err := Layout(nil, areas, LayoutOptions{Width: 800, Height: 450, Padding: 8})
	var e *Error
	if !errors.As(err, &e) || e.Name != "Hang" {
		t.Fatalf("400 sets: error %v, want the error of the bound", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("400 sets: the error comes after %v", d)
	}

	// A small bound stops a small diagram, and the default does not.
	small := []*Area{
		{Sets: []string{"A"}, Size: 10},
		{Sets: []string{"B"}, Size: 10},
		{Sets: []string{"C"}, Size: 10},
		{Sets: []string{"A", "B"}, Size: 2.5},
	}
	if _, err := Layout(&Env{MaxWork: 50}, small, LayoutOptions{Width: 800, Height: 450, Padding: 8}); !errors.As(err, &e) || e.Name != "Hang" {
		t.Fatalf("bound 50: error %v, want the error of the bound", err)
	}
	if _, err := Layout(nil, small, LayoutOptions{Width: 800, Height: 450, Padding: 8}); err != nil {
		t.Fatalf("default bound: %v", err)
	}
}
