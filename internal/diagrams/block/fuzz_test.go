package block

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// mulberry32 is a small seeded generator, so that the test makes the same
// inputs each time.
type mulberry32 uint32

func (r *mulberry32) next() float64 {
	*r += 0x6d2b79f5
	t := uint32(*r)
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

func (r *mulberry32) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() * float64(n))
}

var fuzzSnippets = []string{
	"columns 3", "columns auto", "columns 1", "columns 0", "columns 99999999999", "space", "space:2", "space:300",
	"A", `B["a label"]`, `C("round")`, `D(("circle"))`, `E{"diamond"}`, `F{{"hexagon"}}`, `G(["stadium"])`,
	`H[["subroutine"]]`, `I[("cylinder")]`, `J((("double")))`, `K>"odd"]`, `L[/"lean right"/]`, `M[\"lean left"\]`,
	`N[/"trapezoid"\]`, `O[\"inverted"/]`, `P<["arrow"]>(right)`, `Q<["both"]>(x, y)`, `R<[" "]>(down)`, `S<["bad"]>(nowhere)`,
	"A:2", `B["wide"]:3`, "A:0", "A:99999999999999999999", "A --> B", "A --- B", `A -- "label" --> B`, `A == "thick" ==> B`,
	`A -. "dotted" .-> B`, "A x--x B", "A o--o B", "A <--> B", "A ~~~ B", "A --> B --> C", "E --> A", "A --> E",
	"block", "block:group1", "block:group2:2", "block:group3 --> B", "end",
	"classDef blue fill:#66f,stroke:#333,stroke-width:2px;", "classDef green fill:#6f6,color:#000", "classDef 1 fill:red",
	"class A blue", "class A,B green", "class A, B green", "class A, root blue", "class E 1",
	"style A fill:#f9f,stroke:#333", "style B color:red", "style Missing fill:red",
	`A["a #9829; b"]`, `A["<b>bold</b> and a=b"]`, `A["<img src='x.png'/>"]`, `A["fa:fa-car"]`, `A["$$x^2$$"]`,
	"root", "undefined", "__proto__", "accTitle: x", "%% comment", "\t", "\r", `"`, "`", "<[", "]>(", "((", "))", "-)", "(-",
}

// mutate changes a text at the level of the lines and of the characters.
func mutate(r *mulberry32, text string) string {
	lines := strings.Split(text, "\n")
	pick := func() int { return r.intn(len(lines)) }
	for edits := 1 + r.intn(4); edits > 0; edits-- {
		if len(lines) == 0 {
			lines = []string{""}
		}
		p := pick()
		switch r.intn(9) {
		case 0:
			lines = append(lines[:p], lines[p+1:]...)
		case 1:
			q := pick()
			lines = append(lines[:p], append([]string{lines[q]}, lines[p:]...)...)
		case 2:
			q := pick()
			lines[p], lines[q] = lines[q], lines[p]
		case 3, 4:
			lines = append(lines[:p], append([]string{"  " + fuzzSnippets[r.intn(len(fuzzSnippets))]}, lines[p:]...)...)
		case 5:
			// Delete some characters of a line.
			units := js.UTF16Units(lines[p])
			at := r.intn(len(units) + 1)
			end := min(len(units), at+1+r.intn(4))
			lines[p] = js.FromUTF16Units(append(append([]rune{}, units[:at]...), units[end:]...))
		case 6:
			// Insert a snippet inside a line.
			units := js.UTF16Units(lines[p])
			at := r.intn(len(units) + 1)
			lines[p] = js.FromUTF16Units(units[:at]) + fuzzSnippets[r.intn(len(fuzzSnippets))] + js.FromUTF16Units(units[at:])
		case 7:
			// Repeat a line many times.
			repeated := make([]string, 2+r.intn(20))
			for i := range repeated {
				repeated[i] = lines[p]
			}
			lines = append(lines[:p], append(repeated, lines[p:]...)...)
		default:
			lines = lines[:p]
		}
	}
	return strings.Join(lines, "\n")
}

// corpusTexts returns the block fixtures of the corpus, by path.
func corpusTexts(tb testing.TB) map[string]string {
	tb.Helper()
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "block")
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".mmd") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(path)] = string(data)
		return nil
	})
	if err != nil {
		tb.Fatal(err)
	}
	if len(out) == 0 {
		tb.Fatal("no block fixture in the corpus")
	}
	return out
}

// renderWithin renders a text and reports a panic or a render that takes
// longer than the limit.
func renderWithin(t *testing.T, name, text string, limit time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := renderCase(text, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil && strings.HasPrefix(err.Error(), "panic: ") {
			t.Errorf("%s: %v\n%s", name, err, indent(text))
		}
	case <-time.After(limit):
		t.Fatalf("%s: no result after %v\n%s", name, limit, indent(text))
	}
}

// TestMutatedCorpusNoPanicNoHang renders mutations of the corpus fixtures:
// the renderer must return a result or an error, and never panic or hang.
func TestMutatedCorpusNoPanicNoHang(t *testing.T) {
	texts := corpusTexts(t)
	names := make([]string, 0, len(texts))
	for name := range texts {
		names = append(names, name)
	}
	sort.Strings(names)
	mutants := 6
	if testing.Short() {
		mutants = 1
	}
	r := mulberry32(0x626c6f63)
	n := 0
	for _, name := range names {
		for i := 0; i < mutants; i++ {
			n++
			renderWithin(t, fmt.Sprintf("%s#%d", filepath.Base(name), i), mutate(&r, texts[name]), time.Minute)
		}
	}
	t.Logf("%d mutated inputs", n)
}

// TestHostileInputs renders texts that make upstream use all the memory or
// run for hours: the port must return an error soon.
func TestHostileInputs(t *testing.T) {
	nested := func(depth int) string {
		return "block\n" + strings.Repeat("block\n", depth) + "A\n" + strings.Repeat("end\n", depth)
	}
	cases := []struct {
		name, text string
		// wantError is a part of the error message, or "" for a text that
		// must render.
		wantError string
	}{
		{"space-large", "block\n  space:9999999\n  A\n", "space statements span"},
		{"space-infinite", "block\n  space:" + strings.Repeat("9", 400) + "\n  A\n", "Map maximum size exceeded"},
		{"space-over-map", "block\n  space:16777216\n  A\n", "Map maximum size exceeded"},
		{"space-many", "block\n" + strings.Repeat("  space:9000\n", 4) + "  A\n", "space statements span"},
		{"space-at-limit", "block\n  columns 3\n  space:200 A\n", ""},
		{"nested-40", nested(40), "nested too deep"},
		{"nested-12", nested(12), ""},
		{"width-infinite", "block\n  A:" + strings.Repeat("9", 400) + " B\n", "Position must be a non-negative integer.Infinity"},
		{"columns-infinite", "block\n  columns " + strings.Repeat("9", 400) + "\n  A B\n", "Columns must be an integer !== 0."},
		{"columns-zero", "block\n  columns 0\n  A B\n", "Columns must be an integer !== 0."},
	}
	for _, c := range cases {
		start := time.Now()
		done := make(chan error, 1)
		go func() {
			_, err := renderCase(c.text, nil)
			done <- err
		}()
		select {
		case err := <-done:
			switch {
			case err != nil && strings.HasPrefix(err.Error(), "panic: "):
				t.Errorf("%s: %v", c.name, err)
			case c.wantError == "" && err != nil:
				t.Errorf("%s: error %q, want none", c.name, err)
			case c.wantError != "" && (err == nil || !strings.Contains(errorMessage(err), c.wantError)):
				t.Errorf("%s: error %v, want %q", c.name, err, c.wantError)
			}
			t.Logf("%s: %v", c.name, time.Since(start).Round(time.Millisecond))
		case <-time.After(time.Minute):
			t.Fatalf("%s: no result after a minute", c.name)
		}
	}
}
