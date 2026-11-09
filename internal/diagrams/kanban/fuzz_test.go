package kanban

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// lines are statements that mutate inserts.
var lines = []string{
	"kanban", "todo[Todo]", "  item[An item]", "    deeper", "id(Rounded)", "id((Circle))", "id))Bang((", "id)Cloud(",
	"id{{Hexagon}}", "  ::icon(fa fa-book)", "  :::urgent large", "%% c", "id[\"`**md** text`\"]", "id[\"quoted\"]",
	"  a[Item]@{ assigned: 'knsv', ticket: MC-1, priority: 'High' }",
	"  a[Item]@{ label: [x, y], icon: 5, priority: 3 }",
	"  a[Item]@{\n    assigned: a\n    ticket:\n      b: 1\n  }",
	"  a@{ shape: rect }", "@{", "}",
	"%%{init: {\"theme\": \"redux\", \"look\": \"handDrawn\"}}%%",
	"%%{init: {\"kanban\": {\"sectionWidth\": \"x\", \"ticketBaseUrl\": 5}, \"mindmap\": {\"padding\": null}}}%%",
	"%%{init: {\"kanban\": {\"ticketBaseUrl\": \"https://x/#TICKET#\"}, \"htmlLabels\": false}}%%",
	"Word <br> " + "long text that wraps over several lines of an item ",
}

// mutate changes a few bytes of text: it deletes, repeats, replaces and
// inserts pieces, and sometimes cuts the text.
func mutate(rnd *rand.Rand, text string) string {
	const alphabet = ":;#%{}[]()<>@\"'`\\\n\t -_=,.0123456789abcxyzé\U0001F600"
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

// TestMutatedInputs runs the golden inputs and seeded mutations of them
// through the whole render: the parser, the db and the renderer. A render
// must return, with a result or an error, and must not panic.
func TestMutatedInputs(t *testing.T) {
	g := loadGolden(t)
	rnd := rand.New(rand.NewSource(0x6b616e62))
	var texts []string
	for i := range g.Cases {
		texts = append(texts, caseText(t, &g.Cases[i]))
	}
	for _, c := range loadRenderCases(t) {
		texts = append(texts, c.Text)
	}
	n := len(texts)
	mutants := 600
	if testing.Short() {
		mutants = 200
	}
	for i := 0; i < mutants; i++ {
		texts = append(texts, mutate(rnd, texts[rnd.Intn(n)]))
	}
	for i, text := range texts {
		done := make(chan string, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Sprint("panic: ", r)
				}
			}()
			_, _ = render.Render(text, render.Options{})
			done <- ""
		}()
		select {
		case msg := <-done:
			if msg != "" {
				t.Errorf("input %d: %s\n--- text ---\n%s", i, msg, text)
			}
		case <-time.After(60 * time.Second):
			t.Fatalf("input %d: the render does not return\n--- text ---\n%s", i, text)
		}
	}
	t.Logf("%d inputs", len(texts))
}
