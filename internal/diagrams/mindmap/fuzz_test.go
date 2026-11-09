package mindmap

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// lines are statements that mutate inserts.
var lines = []string{
	"mindmap", "root", "  child", "    deeper", "id[Square]", "id(Rounded)", "id((Circle))", "id))Bang((", "id)Cloud(",
	"id{{Hexagon}}", "  ::icon(fa fa-book)", "  :::urgent large", "%% c", "id[\"`**md** text`\"]", "id[\"quoted\"]",
	"%%{init: {\"theme\": \"redux\", \"look\": \"neo\"}}%%",
	"%%{init: {\"mindmap\": {\"padding\": \"x\", \"maxNodeWidth\": null}, \"layout\": 5}}%%",
	"%%{init: {\"mindmap\": {\"colorBy\": \"depth\"}, \"htmlLabels\": false}}%%",
	"Word <br> " + "long text that wraps over several lines of a node ",
}

// mutate changes a few bytes of text: it deletes, repeats, replaces and
// inserts pieces, and sometimes cuts the text.
func mutate(rnd *rand.Rand, text string) string {
	const alphabet = ":;#%{}[]()<>\"'`\\\n\t -_=,.0123456789abcxyzé\U0001F600"
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
// through the parser, the db and the renderer up to the layout. A render
// must return, with a result or an error, and must not panic. The layout
// itself has the same test with the real cytoscape layout
// (internal/ru/layout/cosebilkent).
func TestMutatedInputs(t *testing.T) {
	g := loadGolden(t)
	rnd := rand.New(rand.NewSource(0x6d696e64))
	var texts []string
	for i := range g.Cases {
		texts = append(texts, caseText(t, &g.Cases[i]))
	}
	n := len(texts)
	for i := 0; i < 2000; i++ {
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
			_, _ = renderToLayout(text, nil)
			done <- ""
		}()
		select {
		case msg := <-done:
			if msg != "" {
				t.Errorf("input %d: %s\n--- text ---\n%s", i, msg, text)
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("input %d: the render does not return\n--- text ---\n%s", i, text)
		}
	}
	t.Logf("%d inputs", len(texts))
}
