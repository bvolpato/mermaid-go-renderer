package font

import (
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
)

// TestWebFontsConcurrently shapes and measures text in web fonts from many
// goroutines and compares with the result of one goroutine. With the race
// detector it checks that the faces of web fonts are safe for concurrent
// use.
func TestWebFontsConcurrently(t *testing.T) {
	lib := NewLibraryFor(PlatformLinux).WithWebFonts()
	type style struct {
		family string
		weight int
		italic bool
		size   float64
	}
	styles := []style{
		{`"Recursive Variable", arial, sans-serif`, 400, false, 14},
		{`"Recursive Variable", arial, sans-serif`, 700, true, 16},
		{`"Open Sans", sans-serif`, 400, false, 14},
		{`"Open Sans", sans-serif`, 700, false, 13.5},
		{`"Font Awesome 7 Free"`, 900, false, 14},
	}
	texts := []string{"Hello World", "Việt ĲĳŁł café", "A.B.C. (a) [b]", " ", "Привет"}
	measure := func() []float64 {
		var out []float64
		for _, s := range styles {
			chain := lib.Resolve(css.SplitFontFamily(s.family), s.weight, s.italic)
			m := chain.Primary().Metrics(s.size)
			out = append(out, m.Ascent, m.Descent, m.XHeight)
			for _, text := range texts {
				for _, run := range chain.shapeText([]rune(text), s.size) {
					out = append(out, run.Width)
					for _, g := range run.Glyphs {
						l, tp, r, b, _, _ := run.Face.ScalerBounds(g.ID, s.size)
						out = append(out, float64(l), float64(tp), float64(r), float64(b))
					}
				}
			}
		}
		return out
	}
	var wg sync.WaitGroup
	results := make([][]float64, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = measure()
		}()
	}
	wg.Wait()
	want := measure()
	for i, got := range results {
		if len(got) != len(want) {
			t.Fatalf("goroutine %d: %d values, want %d", i, len(got), len(want))
		}
		for j := range got {
			if got[j] != want[j] {
				t.Fatalf("goroutine %d: value %d is %v, want %v", i, j, got[j], want[j])
			}
		}
	}
}
