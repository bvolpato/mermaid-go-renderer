package cosebilkent_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/mindmap"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/cosebilkent"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
)

// corpusTexts reads the mindmap fixtures of the corpus.
func corpusTexts(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "..", "testdata", "corpus", "mermaid-e2e", "mindmap")
	var texts []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".mmd") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			texts = append(texts, string(data))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) == 0 {
		t.Fatal("no mindmap fixture")
	}
	return texts
}

// lines are statements that mutate inserts.
var lines = []string{
	"child", "deeper", "id[Square]", "id(Rounded)", "id((Circle))", "id))Bang((", "id)Cloud(",
	"id{{Hexagon}}", "::icon(fa fa-book)", ":::urgent large", "[no id]", "id[\"`**bold** and a long text that wraps`\"]",
}

// mutate changes the tree of a mindmap: it deletes, repeats and inserts
// lines below the root, and changes their indentation, so that the tree
// gets other shapes. It keeps the lines up to the root.
func mutate(rnd *rand.Rand, text string) string {
	all := strings.Split(text, "\n")
	// The first line of the tree is the line after the keyword.
	root := 0
	for i, l := range all {
		if strings.HasPrefix(strings.TrimSpace(l), "mindmap") {
			root = i + 1
			break
		}
	}
	for root < len(all) && strings.TrimSpace(all[root]) == "" {
		root++
	}
	if root >= len(all) {
		return text
	}
	base := len(all[root]) - len(strings.TrimLeft(all[root], " \t"))
	indent := func() string { return strings.Repeat(" ", base+1+rnd.Intn(8)) }
	body := append([]string{}, all[root+1:]...)
	for n := 1 + rnd.Intn(6); n > 0; n-- {
		at := rnd.Intn(len(body) + 1)
		switch rnd.Intn(5) {
		case 0:
			if at < len(body) {
				body = append(body[:at:at], body[at+1:]...)
			}
		case 1:
			if at < len(body) {
				body = append(body[:at+1:at+1], append([]string{body[at]}, body[at+1:]...)...)
			}
		case 2, 3:
			body = append(body[:at:at], append([]string{indent() + lines[rnd.Intn(len(lines))]}, body[at:]...)...)
		case 4:
			if at < len(body) {
				body[at] = indent() + strings.TrimLeft(body[at], " \t")
			}
		}
	}
	return strings.Join(append(all[:root+1:root+1], body...), "\n")
}

// TestMutatedMindmaps renders mindmaps and seeded mutations of them with the
// real layout: the nodes, the cytoscape layout and the edges. A render must
// return, with a result or an error, and must not panic.
func TestMutatedMindmaps(t *testing.T) {
	// The small fixtures: a layout of a large one takes seconds.
	var small []string
	for _, text := range corpusTexts(t) {
		if strings.Count(text, "\n") <= 24 {
			small = append(small, text)
		}
	}
	rnd := rand.New(rand.NewSource(0x636f7365))
	count := 40
	if testing.Short() {
		count = 8
	}
	// The fixtures themselves are the business of the parity command.
	var texts []string
	for i := 0; i < count; i++ {
		texts = append(texts, mutate(rnd, small[rnd.Intn(len(small))]))
	}
	rendered, failed := 0, 0
	for i, text := range texts {
		done := make(chan string, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Sprint("panic: ", r)
				}
			}()
			if _, err := render.Render(text, render.Options{}); err != nil {
				failed++
			} else {
				rendered++
			}
			done <- ""
		}()
		select {
		case msg := <-done:
			if msg != "" {
				t.Errorf("input %d: %s\n--- text ---\n%s", i, msg, text)
			}
		case <-time.After(5 * time.Minute):
			t.Fatalf("input %d: the render does not return\n--- text ---\n%s", i, text)
		}
	}
	t.Logf("%d inputs: %d rendered, %d with an error", len(texts), rendered, failed)
	if rendered < len(texts)/2 {
		t.Errorf("%d of %d inputs rendered: the mutations do not reach the layout", rendered, len(texts))
	}
}
