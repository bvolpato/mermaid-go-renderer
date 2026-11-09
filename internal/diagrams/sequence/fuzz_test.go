package sequence

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// mulberry32 is the seeded generator of the reference scripts.
type mulberry32 uint32

func (m *mulberry32) next() float64 {
	*m += 0x6d2b79f5
	t := uint32(*m)
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

func (m *mulberry32) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(m.next() * float64(n))
}

var fuzzSnippets = []string{
	"Alice->>Bob: hi", "Bob-->>-Alice: bye", "Alice->>+Bob: start", "activate Alice", "deactivate Alice",
	"Note over Alice,Bob: wrap: a note", "Note left of Zed: left", "loop every minute", "alt maybe", "else otherwise",
	"opt", "par one", "and two", "par_over x", "critical danger", "option fallback", "break stop", "rect rgb(200, 220, 255)",
	"rect", "end", "autonumber", "autonumber 5 2", "autonumber off", "create participant Zed", "create actor Y",
	"destroy Bob", "destroy Alice", "box Aqua Group", "box", "participant Alice as wrap:The first one",
	`actor Q@{ "type": "queue" }`, `participant W@{ "type": "nothing" }`, `participant V@{ "type": 5, "alias": 7 }`,
	`links Alice: {"Docs": "https://example.com"}`, "link Alice: Wiki @ https://example.org/wiki",
	`properties Alice: {"class": "internal", "icon": "@computer"}`, `properties Bob: {"icon": 5}`, "title A title",
	"accTitle: Acc", "Alice ()->>() Bob: both", "Bob //-- Alice: stick", "Alice<<->>Alice: self", ";", "#35; text",
	"%% comment", "Alice->>Bob: $$x^2$$", "participant $$a$$", "Alice->>Bob: <br/><br>", "Alice->>Bob:",
}

var fuzzTokens = []string{
	"->>", "-->>", "-x", "--)", "<<->>", "()", "+", "-", ":", ";", ",", "@{", "}", "#", "%%", "<br/>", "\n", "\t", " ",
	"end", "loop", "alt", "else", "par", "and", "rect", "box", "note", "over", "left of", "right of", "as", "wrap:",
	"create", "destroy", "participant", "actor", "activate", "deactivate", "autonumber", "title", "é", "\U0001F600",
}

// mutateSequence changes a fixture: the lines after the diagram keyword, and
// with a lower chance the front matter.
func mutateSequence(rnd *mulberry32, text string) string {
	lines := strings.Split(text, "\n")
	start := 0
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "sequenceDiagram") {
			start = i + 1
			break
		}
	}
	if rnd.intn(8) == 0 {
		start = 0
	}
	pick := func() int { return start + rnd.intn(len(lines)-start) }
	edits := 1 + rnd.intn(4)
	for i := 0; i < edits && len(lines) > start; i++ {
		p := pick()
		switch rnd.intn(8) {
		case 0:
			lines = append(lines[:p], lines[p+1:]...)
		case 1:
			q := pick()
			if q < len(lines) {
				lines = append(lines[:p], append([]string{lines[q]}, lines[p:]...)...)
			}
		case 2:
			q := pick()
			if q < len(lines) {
				lines[p], lines[q] = lines[q], lines[p]
			}
		case 3:
			lines = append(lines[:p], append([]string{"  " + fuzzSnippets[rnd.intn(len(fuzzSnippets))]}, lines[p:]...)...)
		case 4:
			units := js.UTF16Units(lines[p])
			at := rnd.intn(len(units) + 1)
			end := at + 1 + rnd.intn(4)
			if end > len(units) {
				end = len(units)
			}
			lines[p] = js.FromUTF16Units(units[:at]) + js.FromUTF16Units(units[end:])
		case 5:
			units := js.UTF16Units(lines[p])
			at := rnd.intn(len(units) + 1)
			lines[p] = js.FromUTF16Units(units[:at]) + fuzzTokens[rnd.intn(len(fuzzTokens))] + js.FromUTF16Units(units[at:])
		case 6:
			lines = lines[:p]
		default:
			lines[p] = lines[p] + " " + lines[p]
		}
	}
	return strings.Join(lines, "\n")
}

// renderNoPanic renders a text and reports a panic or a hang. An error of
// the renderer is a valid result.
func renderNoPanic(t *testing.T, name, text string) {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- fmt.Sprintf("panic: %v\n%s", p, debug.Stack())
			}
		}()
		_, _ = render.Render(text, render.Options{})
		done <- ""
	}()
	select {
	case msg := <-done:
		if msg != "" {
			t.Errorf("%s: %s\ninput:\n%s", name, msg, text)
		}
	case <-time.After(90 * time.Second):
		t.Fatalf("%s: no result after 90 s\ninput:\n%s", name, text)
	}
}

// TestNoPanicOnMutatedInput renders mutations of the fixtures of the
// sequence family, and the hand-written inputs of the db golden file. The
// renderer must return, with an SVG or with an error.
func TestNoPanicOnMutatedInput(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "sequence")
	var fixtures []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".mmd") {
			fixtures = append(fixtures, path)
		}
		return nil
	})
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no fixtures under %s: %v", root, err)
	}
	sort.Strings(fixtures)
	mutants := 3
	if testing.Short() {
		mutants = 1
	}
	rnd := mulberry32(0x66757a7a)
	n := 0
	for _, f := range fixtures {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < mutants; i++ {
			renderNoPanic(t, fmt.Sprintf("%s#%d", filepath.Base(f), i), mutateSequence(&rnd, string(data)))
			n++
		}
	}
	golden, _ := readGzipJSON(t, "db.json.gz").([]any)
	for _, c := range golden {
		rec := c.(*js.Obj[any])
		if text, ok := rec.Get("text").(string); ok && strings.HasPrefix(js.ToString(rec.Get("name")), "extra") {
			renderNoPanic(t, js.ToString(rec.Get("name")), text)
			n++
		}
	}
	t.Logf("%d inputs", n)
}
