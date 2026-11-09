package main

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
)

// The test of this file renders with the whole pipeline: this program
// links the shapes, the edges, the clusters and the layouts of the unified
// renderer. The tests of the class package cover the parser, the db and the
// shape without a layout.

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
	"class Epsilon {", "  +int count$", "  #run(int n) bool*", "  <<service>>", "}",
	"Alpha <|-- Beta", "Alpha *-- Gamma : owns", `Alpha "1" --> "many" Beta : has`, "Alpha ()-- Beta", "Alpha --() Beta",
	"Alpha --> Alpha", "Alpha : +int age", "Alpha : makeAbstract()*", "<<interface>> Alpha",
	`note "A free note"`, `note for Alpha "A note\nwith two lines"`, "namespace Space {", "namespace Outer.Mid.Leaf {",
	`namespace Named["A namespace label"] {`, "direction LR", "direction BT", "classDef styled fill:#f9f,stroke:#333",
	`cssClass "Alpha,Beta" styled`, "style Alpha fill:#bbf,stroke:#f66,color:#fff,stroke-dasharray: 5 5",
	`link Alpha "https://example.com" "Link tip" _blank`, `callback Alpha "onAlpha" "A tooltip"`,
	"Alpha : <b>bold</b><br/>line", "Alpha : `code` *it* **bold**",
}

// mutate changes the lines of a fixture after the diagram keyword, and
// with a lower chance the front matter.
func mutate(rnd *mulberry32, text string) string {
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
	edits := 1 + rnd.intn(3)
	for i := 0; i < edits && len(lines) > start; i++ {
		p := pick()
		switch rnd.intn(6) {
		case 0:
			lines = append(lines[:p], lines[p+1:]...)
		case 1:
			q := pick()
			if q < len(lines) {
				lines[p], lines[q] = lines[q], lines[p]
			}
		case 2, 3:
			lines = append(lines[:p], append([]string{"  " + fuzzSnippets[rnd.intn(len(fuzzSnippets))]}, lines[p:]...)...)
		case 4:
			units := js.UTF16Units(lines[p])
			at := rnd.intn(len(units) + 1)
			end := at + 1 + rnd.intn(4)
			if end > len(units) {
				end = len(units)
			}
			lines[p] = js.FromUTF16Units(units[:at]) + js.FromUTF16Units(units[end:])
		default:
			lines = lines[:p]
		}
	}
	return strings.Join(lines, "\n")
}

// TestNoPanicOnMutatedInput renders mutations of the fixtures of the
// class-diagram family with the layouts linked in. The renderer must
// return, with an SVG or with an error.
func TestNoPanicOnMutatedInput(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "testdata", "corpus", "mermaid-e2e", "class-diagram")
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
	mutants := 2
	if testing.Short() {
		mutants = 1
	}
	type input struct{ name, text string }
	var inputs []input
	rnd := mulberry32(0x636c6173)
	for _, f := range fixtures {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < mutants; i++ {
			inputs = append(inputs, input{fmt.Sprintf("%s#%d", filepath.Base(f), i), mutate(&rnd, string(data))})
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []string
	rendered := 0
	sem := make(chan struct{}, runtime.NumCPU())
	for _, in := range inputs {
		wg.Add(1)
		sem <- struct{}{}
		go func(in input) {
			defer wg.Done()
			defer func() { <-sem }()
			type result struct {
				failure string
				ok      bool
			}
			done := make(chan result, 1)
			go func() {
				defer func() {
					if p := recover(); p != nil {
						done <- result{failure: fmt.Sprintf("panic: %v\n%s", p, debug.Stack())}
					}
				}()
				_, err := render.Render(in.text, render.Options{})
				done <- result{ok: err == nil}
			}()
			var res result
			select {
			case res = <-done:
			case <-time.After(180 * time.Second):
				res.failure = "no result after 180 s"
			}
			msg, ok := res.failure, res.ok
			mu.Lock()
			defer mu.Unlock()
			if msg != "" {
				failures = append(failures, fmt.Sprintf("%s: %s\ninput:\n%s", in.name, msg, in.text))
			} else if ok {
				rendered++
			}
		}(in)
	}
	wg.Wait()
	sort.Strings(failures)
	for _, msg := range failures {
		t.Error(msg)
	}
	t.Logf("%d inputs, %d rendered", len(inputs), rendered)
}
