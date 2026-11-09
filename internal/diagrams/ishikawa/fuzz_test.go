package ishikawa

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// lines are statements that mutate inserts.
var lines = []string{
	"ishikawa-beta", "ishikawa", "Effect", "  Cause", "    Sub cause", "      Deeper", "\tTabbed", "  Line one<br>line two<br/>three",
	"  A very long label that wraps over several lines of fifteen characters", "  <b>bold</b> <script>x</script>", "  ", "",
	"%% c", "  %% indented comment", "%%{init: {\"theme\": \"dark\", \"look\": \"handDrawn\"}}%%",
	"%%{init: {\"fontSize\": \"x\", \"ishikawa\": {\"diagramPadding\": \"a\", \"useMaxWidth\": null}}}%%",
	"%%{init: {\"fontSize\": 0, \"ishikawa\": {\"diagramPadding\": -500}}}%%",
	"%%{init: {\"look\": \"handDrawn\", \"handDrawnSeed\": \"x\", \"themeVariables\": {\"lineColor\": 5, \"mainBkg\": null}}}%%",
	"%%{init: {\"fontSize\": 1e9}}%%",
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
	rnd := rand.New(rand.NewSource(0x69736869))
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
