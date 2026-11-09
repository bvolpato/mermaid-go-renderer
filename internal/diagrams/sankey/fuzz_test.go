package sankey

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// mulberry32 is the generator of the golden scripts.
type mulberry32 uint32

func (m *mulberry32) next() float64 {
	*m += 0x6d2b79f5
	t := uint32(*m)
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

func (m *mulberry32) intn(n int) int { return int(m.next() * float64(n)) }

// mutate makes one to three random edits of a text.
func mutate(rnd *mulberry32, text string, pieces []string) string {
	runes := []rune(text)
	for edits := 1 + rnd.intn(3); edits > 0; edits-- {
		at := rnd.intn(len(runes) + 1)
		switch k := rnd.next(); {
		case k < 0.4:
			piece := []rune(pieces[rnd.intn(len(pieces))])
			runes = append(runes[:at], append(piece, runes[at:]...)...)
		case k < 0.7:
			end := min(len(runes), at+1+rnd.intn(4))
			runes = append(runes[:at], runes[end:]...)
		default:
			piece := []rune(pieces[rnd.intn(len(pieces))])
			end := min(len(runes), at+1)
			runes = append(runes[:at], append(piece, runes[end:]...)...)
		}
	}
	return string(runes)
}

// renderNoPanic renders a text and reports a panic as an error. A hang
// fails the test.
func renderNoPanic(t *testing.T, text string) {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Sprint("panic: ", r)
			}
		}()
		// An error is a valid result for a malformed text.
		_, _ = render.Render(text, render.Options{})
		done <- ""
	}()
	select {
	case msg := <-done:
		if msg != "" {
			t.Errorf("%s\ninput: %q", msg, text)
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("the render hangs\ninput: %q", text)
	}
}

// TestMutatedCorpus feeds mutated corpus inputs to the renderer and requires
// that it neither panics nor hangs.
func TestMutatedCorpus(t *testing.T) {
	files, err := filepath.Glob("../../../testdata/corpus/mermaid-e2e/sankey/*/*.mmd")
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpus fixtures: %v", err)
	}
	pieces := []string{
		",", ",,", "\"", "\"\"", "\n", "\n\n", "\r\n", " ", "\t", "0", "-1", "1e9", "1e400", "NaN", "abc", "sankey", "sankey-beta",
		"%%", "---", "a,b,1", "a,a,1", "<b>", "#quot;", "é", "😀", ":", "{", "}", "nodeAlignment: bogus", "width: -5", "height: 0",
		"nodeWidth: 1e9", "nodePadding: 1e9", "showValues: false", "linkColor: 7", "constructor", "__proto__",
	}
	perFile := 25
	if testing.Short() {
		perFile = 5
	}
	rnd := mulberry32(0x5a4e)
	n := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		renderNoPanic(t, text)
		for i := 0; i < perFile; i++ {
			renderNoPanic(t, mutate(&rnd, text, pieces))
			n++
		}
		// Only the diagram text, without the front matter.
		if i := strings.LastIndex(text, "---\n"); i >= 0 {
			renderNoPanic(t, mutate(&rnd, text[i+4:], pieces))
			n++
		}
	}
	t.Logf("%d mutated inputs of %d fixtures", n, len(files))
}
