package er

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// corpusTexts returns the fixtures of the ER family of the corpus.
func corpusTexts(tb testing.TB) []string {
	tb.Helper()
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "er-diagram")
	var texts []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".mmd") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		texts = append(texts, string(data))
		return nil
	})
	if err != nil {
		tb.Fatal(err)
	}
	if len(texts) == 0 {
		tb.Fatal("no fixtures of the ER family")
	}
	return texts
}

// mulberry32 is a small seeded random generator.
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

// hostileTokens are texts that a mutation puts into a fixture.
var hostileTokens = []string{
	"{", "}", "[", "]", "\"", "`", "~", ":", ":::", ",", "#", ";", "%%", "||--o{", "}|..|{", "u--", "1", "1.5",
	"subgraph", "end", "direction LR", "classDef", "class", "style", "accTitle:", "accDescr {", "erDiagram",
	"\n", "\r\n", "\t", "\x00", " ", " ", "😀", "\xed\xa0\x80", "\xff", "<script>x</script>", "<br/>",
	"&lt;", "#35;", "~T~", "PK", "FK, UK", "\"unterminated", "`unterminated", "$", "@", "\\",
	"style A fill:red,stroke:,color", "classDef a,b fill:#f00;", "A[", "A[]", "A:::", "---", "%%{init: {\"look\": \"handDrawn\"}}%%",
}

// mutate changes a text at random places: it deletes, repeats and swaps
// parts, inserts hostile tokens and cuts the text.
func mutate(r *mulberry32, text string) string {
	b := []byte(text)
	edits := 1 + r.intn(4)
	for i := 0; i < edits; i++ {
		switch r.intn(6) {
		case 0: // delete a part
			if len(b) > 0 {
				at := r.intn(len(b))
				n := 1 + r.intn(8)
				if at+n > len(b) {
					n = len(b) - at
				}
				b = append(b[:at:at], b[at+n:]...)
			}
		case 1: // repeat a part
			if len(b) > 0 {
				at := r.intn(len(b))
				n := 1 + r.intn(40)
				if at+n > len(b) {
					n = len(b) - at
				}
				part := append([]byte{}, b[at:at+n]...)
				b = append(b[:at:at], append(part, b[at:]...)...)
			}
		case 2, 3: // insert a token
			at := r.intn(len(b) + 1)
			token := hostileTokens[r.intn(len(hostileTokens))]
			b = append(b[:at:at], append([]byte(token), b[at:]...)...)
		case 4: // swap two lines
			lines := strings.Split(string(b), "\n")
			p, q := r.intn(len(lines)), r.intn(len(lines))
			lines[p], lines[q] = lines[q], lines[p]
			b = []byte(strings.Join(lines, "\n"))
		default: // cut the text
			if len(b) > 0 {
				b = b[:r.intn(len(b))]
			}
		}
	}
	return string(b)
}

// hostileInputs are inputs that are large, deep or malformed. The sizes
// keep the time of the layouts short: the ELK layout needs about 17 s for a
// chain of 400 entities, and it stops at its time limit of 30 s for 300
// nested subgraphs.
func hostileInputs() []string {
	var deep strings.Builder
	deep.WriteString("erDiagram\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&deep, "subgraph S%d\n", i)
	}
	deep.WriteString("A ||--o{ B : x\n")
	deep.WriteString(strings.Repeat("end\n", 30))

	var wide strings.Builder
	wide.WriteString("erDiagram\nBIG {\n")
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&wide, "  type%d~T~ name%d PK, FK \"comment %d\"\n", i, i, i)
	}
	wide.WriteString("}\n")

	var many strings.Builder
	many.WriteString("erDiagram\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&many, "E%d ||--o{ E%d : r%d\n", i, i+1, i)
	}

	return []string{
		"",
		"erDiagram",
		"erDiagram\n",
		"erDiagram\n" + strings.Repeat("{", 2000),
		"erDiagram\n" + strings.Repeat("A {\n", 500),
		"erDiagram\n" + strings.Repeat("\"", 3001),
		"erDiagram\nA {\n" + strings.Repeat("`", 999) + "\n}",
		"erDiagram\n" + strings.Repeat("A", 20000),
		"erDiagram\n\"" + strings.Repeat("A ", 5000) + "\"",
		"erDiagram\nA[" + strings.Repeat("~", 400) + "] {\n " + strings.Repeat("~T", 200) + " n\n}",
		"erDiagram\nA {\n " + strings.Repeat("a~b~,", 300) + " n\n}",
		"erDiagram\nclassDef " + strings.Repeat("a,", 2000) + "b fill:red\n",
		"erDiagram\nstyle A " + strings.Repeat("fill:red,", 2000) + "x:y\nA",
		"erDiagram\nA ||--o{ B : \"" + strings.Repeat("<b>", 500) + "\"",
		"erDiagram\n\xff\xfe\xfd",
		"erDiagram\nA\xed\xa0\x80B ||--o{ C : \xed\xb0\x80",
		"erDiagram\nA\x00B",
		"---\nconfig:\n  er:\n    diagramPadding: -1000\n    entityPadding: -50\n    minEntityWidth: 1e308\n---\nerDiagram\nA {\n string x\n}\nB",
		"---\nconfig:\n  er: null\n---\nerDiagram\nA {\n string x\n}\nB",
		"---\nconfig:\n  flowchart: null\n---\nerDiagram\nA",
		"---\nconfig:\n  themeVariables: null\n---\nerDiagram\nA",
		"---\nconfig:\n  theme: redux-color\n  themeVariables:\n    borderColorArray: []\n---\nerDiagram\nA {\n string x\n}\nB",
		"---\nconfig:\n  theme: redux-color\n  themeVariables:\n    borderColorArray: 5\n---\nerDiagram\nA {\n string x\n}\nB",
		"---\nconfig:\n  htmlLabels: \"false\"\n---\nerDiagram\nA {\n string x\n}\nB",
		"---\nconfig:\n  htmlLabels: \"\"\n---\nerDiagram\nA {\n string x\n}\nB",
		"---\nconfig:\n  look: handDrawn\n  fontSize: -5\n---\nerDiagram\nA {\n string x\n}\nB",
		"---\nconfig:\n  look: handDrawn\n  er:\n    diagramPadding: NaN\n---\nerDiagram\nA {\n string x\n}",
		"---\ntitle: T\nconfig:\n  er:\n    titleTopMargin: x\n    useMaxWidth: 0\n---\nerDiagram\nA",
		deep.String(),
		wide.String(),
		many.String(),
	}
}

// renderNoPanic renders a text and reports a panic or a render that does
// not end. An error of the render is a correct result.
func renderNoPanic(text string) (problem string) {
	done := make(chan string, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				buf := make([]byte, 4096)
				done <- fmt.Sprintf("panic: %v\n%s", p, buf[:runtime.Stack(buf, false)])
			}
		}()
		_, _ = render.Render(text, render.Options{})
		done <- ""
	}()
	select {
	case problem = <-done:
		return problem
	case <-time.After(2 * time.Minute):
		return "the render did not end within 2 minutes"
	}
}

// TestNoPanicOnMalformedInput feeds mutated fixtures of the corpus and
// hostile inputs to the whole pipeline: the parser, the db, the renderer,
// the layouts and the shapes. Nothing may panic or hang.
func TestNoPanicOnMalformedInput(t *testing.T) {
	texts := corpusTexts(t)
	rnd := mulberry32(0x6572)
	inputs := hostileInputs()
	mutants := 2
	if testing.Short() {
		mutants = 1
	}
	for _, text := range texts {
		for i := 0; i < mutants; i++ {
			inputs = append(inputs, mutate(&rnd, text))
		}
	}

	problems := make([]string, len(inputs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i := range inputs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			problems[i] = renderNoPanic(inputs[i])
		}(i)
	}
	wg.Wait()
	for i, problem := range problems {
		if problem != "" {
			text := inputs[i]
			if len(text) > 400 {
				text = text[:400] + "..."
			}
			t.Errorf("input %d: %s\n    text: %q", i, problem, text)
		}
	}
	t.Logf("%d inputs", len(inputs))
}

// FuzzParse parses a text and builds the layout data. Nothing may panic.
// Run it with: go test ./internal/diagrams/er -run '^$' -fuzz FuzzParse
func FuzzParse(f *testing.F) {
	for _, text := range []string{
		"erDiagram\nCUSTOMER ||--o{ ORDER : places\n",
		"erDiagram\nA[Alias]:::cls {\n  string id PK \"c\"\n  type~T~ g FK, UK\n}\nclassDef cls fill:#f00,color:#fff\n",
		"erDiagram\ndirection LR\nsubgraph S [T]\nA }|..|{ B : \"x y\"\nend\nstyle A fill:red\nclass A,B c1,c2\n",
		"erDiagram\naccTitle: t\naccDescr {\n d\n}\n1 ||--|| 2.5 : n\nu u--u u : m\n",
	} {
		f.Add(text)
	}
	resolved, err := config.Resolve("erDiagram\nA", nil)
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, text string) {
		ctx := render.NewContext(render.ContextOptions{Config: resolved.State.GetConfig(), ConfigState: resolved.State})
		ctx.Type = "er"
		d := NewDB(ctx)
		if err := parse(d, render.EncodeEntities(text)+"\n"); err != nil {
			return
		}
		if _, err := layoutData(ctx, d); err != nil {
			t.Fatalf("layout data: %v", err)
		}
	})
}
