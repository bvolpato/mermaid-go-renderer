package class

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
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
	"class Alpha", "class Beta~T~", `class Gamma["A label"]`, "class Delta:::styled", "class Eta <<interface>>",
	"class Epsilon {", "  +int count$", "  #run(int n) bool*", "  <<service>>", "}", "{",
	"class `Quoted name`", "class A.B.C", "Alpha <|-- Beta", "Alpha *-- Gamma : owns", `Alpha "1" --> "many" Beta : has`,
	"Alpha ()-- Beta", "Alpha --() Beta", "Alpha ()..() Beta", "Alpha : +int age", "Alpha : makeAbstract()*",
	"Alpha : foo)", "Alpha : (x)", "Alpha : <<abstract>>", "<<interface>> Alpha", "<<enumeration>> Missing",
	`note "A free note"`, `note for Alpha "A note\nwith two lines"`, "namespace Space {", "namespace Outer.Mid.Leaf {",
	`namespace Named["A namespace label"] {`, "direction LR", "direction BT", "classDef styled fill:#f9f,stroke:#333",
	`cssClass "Alpha,Beta" styled`, "style Alpha fill:#bbf,stroke:#f66,color:#fff,stroke-dasharray: 5 5", "Alpha:::styled",
	`callback Alpha "onAlpha" "A tooltip"`, `link Alpha "https://example.com" "Link tip" _blank`,
	`click Alpha call onAlpha("x", 2) "Call tip"`, `click Alpha href "https://example.com/a" _self`,
	"accTitle: The title", "accDescr {\n  multi\n  line\n}", "%% comment", "Alpha : #35; hash #lt;b#gt;",
	"Alpha : $$x^2$$", "Alpha : <b>bold</b><br/>line", "Alpha : `code` *it* **bold**",
}

var fuzzTokens = []string{
	"<|--", "--|>", "*--", "o--", "..>", "<..", "()", "--", "..", ":", ":::", "~", "<<", ">>", "{", "}", "[", "]",
	`"`, "`", "#", "%%", "<br/>", "\n", "\t", " ", "class", "namespace", "note", "note for", "style", "classDef",
	"cssClass", "click", "call", "href", "link", "callback", "direction", "é", "\U0001F600", "$", "*", "+", "-", "(", ")",
}

// mutateClass changes a fixture: the lines after the diagram keyword, and
// with a lower chance the front matter.
func mutateClass(rnd *mulberry32, text string) string {
	lines := strings.Split(text, "\n")
	start := 0
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "classDiagram") {
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

// drawClassBoxes parses a text and draws every node of the layout data
// that has the shape classBox. The layouts of the unified renderer are
// separate packages, so this covers the shape of this package for every
// input, also when no layout is linked into the test.
func drawClassBoxes(text string) {
	p, err := parseText(text, nil, false, 0)
	if err != nil || p.db == nil {
		return
	}
	data := p.db.GetData()
	g := p.ctx.SVG.Append("g").Attr("class", "nodes")
	for _, node := range data.Nodes {
		if node.Shape == "classBox" {
			_, _ = ru.InsertNode(p.ctx, g, node, &ru.ShapeRenderOptions{Config: p.ctx.Config})
		}
	}
}

// runNoPanic runs fn and reports a panic or a hang. An error of the
// renderer is a valid result.
func runNoPanic(name, text string, fn func(text string)) string {
	done := make(chan string, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- fmt.Sprintf("panic: %v\n%s", p, debug.Stack())
			}
		}()
		fn(text)
		done <- ""
	}()
	select {
	case msg := <-done:
		if msg != "" {
			return fmt.Sprintf("%s: %s\ninput:\n%s", name, msg, text)
		}
		return ""
	case <-time.After(120 * time.Second):
		return fmt.Sprintf("%s: no result after 120 s\ninput:\n%s", name, text)
	}
}

// TestNoPanicOnMutatedInput renders mutations of the fixtures of the
// class-diagram family, and the hand-written inputs of the db golden file.
// The renderer must return, with an SVG or with an error.
func TestNoPanicOnMutatedInput(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "class-diagram")
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
	type input struct{ name, text string }
	var inputs []input
	rnd := mulberry32(0x66757a7a)
	for _, f := range fixtures {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < mutants; i++ {
			inputs = append(inputs, input{fmt.Sprintf("%s#%d", filepath.Base(f), i), mutateClass(&rnd, string(data))})
		}
	}
	g := loadGolden(t)
	for i := range g.Cases {
		if c := &g.Cases[i]; c.Text != nil && strings.HasPrefix(c.Name, "extra") {
			inputs = append(inputs, input{c.Name, *c.Text})
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []string
	sem := make(chan struct{}, runtime.NumCPU())
	for _, in := range inputs {
		wg.Add(1)
		sem <- struct{}{}
		go func(in input) {
			defer wg.Done()
			defer func() { <-sem }()
			for _, fn := range []func(string){
				func(text string) { _, _ = render.Render(text, render.Options{}) },
				drawClassBoxes,
			} {
				if msg := runNoPanic(in.name, in.text, fn); msg != "" {
					mu.Lock()
					failures = append(failures, msg)
					mu.Unlock()
				}
			}
		}(in)
	}
	wg.Wait()
	sort.Strings(failures)
	for _, msg := range failures {
		t.Error(msg)
	}
	t.Logf("%d inputs", len(inputs))
}
