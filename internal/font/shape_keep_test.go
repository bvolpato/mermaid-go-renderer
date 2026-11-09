package font

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

func runsText(runs []Run) string {
	out := ""
	for _, r := range runs {
		out += fmt.Sprintf("[%s %d %x:", r.Face.Path, r.Start, r.Width)
		for _, g := range r.Glyphs {
			out += fmt.Sprintf(" %d/%d/%x/%x/%x", g.ID, g.Cluster, g.Advance, g.XOffset, g.YOffset)
		}
		out += "]"
	}
	return out
}

// TestKeptShapes compares ShapeText, which keeps its results, with the
// shaping of every call: for many texts, sizes and chains, each asked for
// several times and from many goroutines.
func TestKeptShapes(t *testing.T) {
	lib := newLibrary(PlatformLinux, embedFonts, nil)
	chains := []*Chain{
		lib.Resolve(mermaidFamilies, 400, false),
		lib.Resolve(mermaidFamilies, 700, false),
		lib.Resolve([]string{"monospace"}, 400, true),
		lib.Resolve([]string{"serif"}, 400, false),
	}
	words := []string{"Start", "Decision", "Yes", "No", "A", "fi", "AVA", "To", " ", "  ", "é", "中文", "😀", "x́", "1.5", "—", "\t", "ffi"}
	rng := rand.New(rand.NewSource(1))
	type shapeCase struct {
		chain *Chain
		text  []rune
		size  float64
		want  string
	}
	var cases []shapeCase
	for i := 0; i < 400; i++ {
		text := ""
		for n := 1 + rng.Intn(5); n > 0; n-- {
			text += words[rng.Intn(len(words))]
		}
		if i%50 == 0 {
			// A text that is too long to keep.
			for len([]rune(text)) <= maxKeptShape {
				text += " " + text + "long"
			}
		}
		c := shapeCase{chain: chains[rng.Intn(len(chains))], text: []rune(text), size: []float64{16, 14, 12.5, 16.79, 20}[rng.Intn(5)]}
		c.want = runsText(c.chain.shapeText(c.text, c.size))
		cases = append(cases, c)
	}
	memo.Purge()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(g)))
			for i := 0; i < 2000; i++ {
				c := cases[rng.Intn(len(cases))]
				if got := runsText(c.chain.ShapeText(c.text, c.size)); got != c.want {
					t.Errorf("ShapeText(%q, %v) = %s, want %s", string(c.text), c.size, got, c.want)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	if shapedTexts.Len() == 0 {
		t.Error("no shaping was kept")
	}
	// Two texts with the same hash and length: the entry of one must not
	// answer for the other.
	a, b := []rune("ab"), []rune("cd")
	key := shapeKey{chain: chains[0], size: 16, hash: hashRunes(a), length: 2}
	shapedTexts.Put(key, &shapedText{text: b, runs: chains[0].shapeText(b, 16)})
	if got, want := runsText(chains[0].ShapeText(a, 16)), runsText(chains[0].shapeText(a, 16)); got != want {
		t.Errorf("an entry with the same hash answered for another text")
	}
}
