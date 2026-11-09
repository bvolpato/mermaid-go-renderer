package pie

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// family is the corpus directory with the fixtures of this diagram type.
const family = "pie"

// snippets are the texts that mutate inserts.
var snippets = []string{
	"pie", "showData", "title", "accTitle:", "accDescr {", "}", "\"", "'", ":", "\n", " ", "%%", "-", ".", "0", "17",
	"1e400", "\\", "\x00", "\xff", "é", "😀", "---\n", "%%{init: {\"pie\": {\"textPosition\": \"x\"}}}%%\n",
}

func readFixtures(t testing.TB) []string {
	t.Helper()
	files, err := filepath.Glob("../../../testdata/corpus/mermaid-e2e/" + family + "/*/*.mmd")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	texts := make([]string, len(files))
	for i, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		texts[i] = string(data)
	}
	return texts
}

// mutate returns a variant of text with one to four random edits.
func mutate(text string, rnd *rand.Rand) string {
	for edits := 1 + rnd.Intn(4); edits > 0; edits-- {
		at := rnd.Intn(len(text) + 1)
		switch rnd.Intn(6) {
		case 0: // delete some bytes
			end := min(len(text), at+1+rnd.Intn(4))
			text = text[:at] + text[end:]
		case 1, 2: // insert a snippet
			text = text[:at] + snippets[rnd.Intn(len(snippets))] + text[at:]
		case 3: // cut the end
			text = text[:at]
		case 4: // duplicate a line
			lines := strings.Split(text, "\n")
			i := rnd.Intn(len(lines))
			lines = append(lines[:i+1], lines[i:]...)
			text = strings.Join(lines, "\n")
		default: // swap two lines
			lines := strings.Split(text, "\n")
			i, j := rnd.Intn(len(lines)), rnd.Intn(len(lines))
			lines[i], lines[j] = lines[j], lines[i]
			text = strings.Join(lines, "\n")
		}
	}
	return text
}

// renderNoPanic renders the text. An error is a valid result. A panic is
// not.
func renderNoPanic(t *testing.T, text string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("text %q: panic: %v", text, r)
		}
	}()
	_, _ = render.Render(text, render.Options{})
}

// TestMutatedCorpus renders mutated corpus fixtures. No input may cause a
// panic or a hang (the test timeout).
func TestMutatedCorpus(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	n := 0
	for _, fixture := range readFixtures(t) {
		// The front matter holds a description. Mutate the diagram more
		// often than the description.
		body := fixture
		if i := strings.LastIndex(fixture, "\n---\n"); i >= 0 {
			body = fixture[i+5:]
		}
		for i := 0; i < 25; i++ {
			renderNoPanic(t, mutate(body, rnd))
			n++
		}
		for i := 0; i < 5; i++ {
			renderNoPanic(t, mutate(fixture, rnd))
			n++
		}
	}
	t.Logf("%d mutated inputs", n)
}

// FuzzRender is the same check for the fuzzing engine of Go.
func FuzzRender(f *testing.F) {
	for _, fixture := range readFixtures(f) {
		f.Add(fixture)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 4000 {
			return
		}
		renderNoPanic(t, text)
	})
}
