package venn

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// lines are statements that mutate inserts.
var lines = []string{
	"venn-beta", "title T", "set A", "set B[\"Label\"]:20", "set C:0", "set \"1\"", "set constructor", "set __proto__",
	"union A,B", "union A,B,C[\"ABC\"]:1", "union A,B:-3", "union A,A", "  text \"item\"", "  text x[\"L\"]", "text A,B t",
	"style A fill:#ff6b6b, stroke:#333, stroke-width:2", "style A,B fill:rgba(1,2,3,0.5), color:red", "style A fill:banana",
	"set D:99999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999",
	"%% c", "%%{init: {\"theme\": \"dark\", \"look\": \"handDrawn\"}}%%",
	"%%{init: {\"venn\": {\"width\": \"a\", \"height\": null, \"padding\": \"8\", \"useDebugLayout\": true}}}%%",
	"%%{init: {\"venn\": {\"width\": 0, \"height\": -5}, \"themeVariables\": {\"venn1\": 5, \"background\": \"x\"}}}%%",
	"%%{init: {\"look\": \"handDrawn\", \"handDrawnSeed\": \"x\"}}%%",
}

// mutate changes a few bytes of text: it deletes, repeats, replaces and
// inserts pieces, and sometimes cuts the text.
func mutate(rnd *rand.Rand, text string) string {
	const alphabet = ":;#%{}[]<>\"'\\\n\t -_=,.0123456789abcABCé\U0001F600"
	b := []byte(text)
	for n := 1 + rnd.Intn(4); n > 0; n-- {
		at := rnd.Intn(len(b) + 1)
		switch rnd.Intn(6) {
		case 0: // delete
			end := min(len(b), at+1+rnd.Intn(8))
			b = append(b[:at:at], b[end:]...)
		case 1: // repeat
			end := min(len(b), at+1+rnd.Intn(12))
			piece := append([]byte{}, b[at:end]...)
			b = append(b[:end:end], append(piece, b[end:]...)...)
		case 2: // insert
			runes := []rune(alphabet)
			piece := []byte(string(runes[rnd.Intn(len(runes))]))
			b = append(b[:at:at], append(piece, b[at:]...)...)
		case 3: // replace a byte
			if at < len(b) {
				b[at] = alphabet[rnd.Intn(len(alphabet))]
			}
		case 4: // insert a line
			piece := []byte("\n" + lines[rnd.Intn(len(lines))] + "\n")
			b = append(b[:at:at], append(piece, b[at:]...)...)
		case 5: // cut
			if rnd.Intn(4) == 0 {
				b = b[:at]
			}
		}
	}
	return string(b)
}

// TestMutatedInputs renders the golden inputs and seeded mutations of them.
// A render must return, with a result or an error, and must not panic.
func TestMutatedInputs(t *testing.T) {
	cases := loadGolden(t)
	rnd := rand.New(rand.NewSource(0x76656e6e))
	var texts []string
	for _, c := range cases {
		texts = append(texts, c.Text)
	}
	for i := 0; i < 500; i++ {
		texts = append(texts, mutate(rnd, cases[rnd.Intn(len(cases))].Text))
	}
	rendered := 0
	for i, text := range texts {
		done := make(chan string, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Sprint("panic: ", r)
				}
			}()
			if _, err := render.Render(text, render.Options{}); err == nil {
				rendered++
			}
			done <- ""
		}()
		select {
		case msg := <-done:
			if msg != "" {
				t.Errorf("input %d: %s\n%q", i, msg, text)
			}
		case <-time.After(60 * time.Second):
			t.Fatalf("input %d: the render does not return\n%q", i, text)
		}
	}
	t.Logf("%d inputs: %d render, %d return an error, none panics", len(texts), rendered, len(texts)-rendered)
}
