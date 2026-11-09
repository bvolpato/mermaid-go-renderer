package flowchart

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// fuzzAlphabet has the characters that the grammar of a flowchart reacts to.
var fuzzAlphabet = []string{
	"[", "]", "(", ")", "{", "}", "|", "-", "=", ".", ">", "<", "&", ":", ";", "@", "\"", "`", "#", ",", "~", "x", "o",
	"\n", " ", "\t", "%%", "@{", "}\n", "-->", "---", "==>", "-.->", "~~~", ":::", "end", "subgraph ", "direction LR",
	"click ", "call ", "href ", "style ", "classDef ", "class ", "linkStyle ", "default", "interpolate ", "accTitle: ",
	"accDescr {", "graph ", "flowchart ", "é", "😀", " ", "ﬂ°", "¶ß", "&amp;", "<br/>", "shape: ", "label: ", "0", "9",
}

// mutateText changes a text at random places: it removes, repeats, moves
// and inserts pieces.
func mutateText(rnd *rand.Rand, text string) string {
	b := []byte(text)
	edits := 1 + rnd.Intn(4)
	for i := 0; i < edits; i++ {
		at := rnd.Intn(len(b) + 1)
		switch rnd.Intn(6) {
		case 0: // remove a piece
			n := min(len(b)-at, 1+rnd.Intn(12))
			b = append(b[:at:at], b[at+n:]...)
		case 1: // repeat a piece
			n := min(len(b)-at, 1+rnd.Intn(40))
			piece := append([]byte{}, b[at:at+n]...)
			b = append(b[:at:at], append(piece, b[at:]...)...)
		case 2, 3: // insert a piece of the alphabet
			piece := fuzzAlphabet[rnd.Intn(len(fuzzAlphabet))]
			b = append(b[:at:at], append([]byte(piece), b[at:]...)...)
		case 4: // cut the text
			if at > 0 {
				b = b[:at]
			}
		case 5: // move a line
			lines := strings.Split(string(b), "\n")
			from, to := rnd.Intn(len(lines)), rnd.Intn(len(lines))
			lines[from], lines[to] = lines[to], lines[from]
			b = []byte(strings.Join(lines, "\n"))
		}
	}
	return strings.ToValidUTF8(string(b), "�")
}

// exercise runs a text through the pipeline until the draw function, and
// then builds the layout data and reads the db, as the renderer does.
func exercise(text string) {
	o := renderToDraw(text, nil, false)
	if o.cp == nil {
		return
	}
	d := o.cp.db
	data := o.cp.data
	for _, n := range data.Nodes {
		_ = js.StringifyJSON(NodeJSON(n))
	}
	for _, e := range data.Edges {
		_ = js.StringifyJSON(EdgeJSON(e))
	}
	_ = checkLabels(data)
	d.IndexNodes()
	d.GetVertices().Each(func(id string, _ *FlowVertex) { _ = d.LookUpDomId(id) })
}

// TestMutatedCorpus feeds mutated corpus fixtures and hostile texts to the
// parser and the db. The requirement is: no panic and no hang. An error is
// a good result.
func TestMutatedCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e")
	var fixtures []string
	for _, family := range []string{"flowchart", "swimlanes"} {
		_ = filepath.WalkDir(filepath.Join(root, family), func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".mmd") {
				fixtures = append(fixtures, path)
			}
			return nil
		})
	}
	sort.Strings(fixtures)
	if len(fixtures) == 0 {
		t.Fatal("no corpus fixtures")
	}

	const mutantsPerFixture = 3
	rnd := rand.New(rand.NewSource(0x666c6f77))
	var texts []string
	for _, f := range fixtures {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < mutantsPerFixture; i++ {
			texts = append(texts, mutateText(rnd, string(data)))
		}
	}
	// Texts that aim at the limits of the db.
	nested := strings.Repeat("subgraph s\n", 3000) + "a\n" + strings.Repeat("end\n", 3000)
	cyclic := "flowchart TD\nsubgraph A\nB\nend\nsubgraph B\nC\nend\nsubgraph C\nA\nend\nA@{ view: collapsed }\nB@{ view: collapsed }\nA --> B\n"
	texts = append(texts,
		"", "flowchart", "graph", "flowchart TD", "flowchart TD\n@{", "flowchart TD\nA@{", "flowchart TD\nA@{\n",
		"flowchart TD\n"+nested,
		"flowchart TD\n"+strings.Repeat("a --> b & c & d\n", 400),
		"flowchart TD\n"+strings.Repeat("a", 20000)+" --> b",
		"flowchart TD\nA["+strings.Repeat("[", 5000),
		"flowchart TD\nA@{ shape: "+strings.Repeat("x", 5000)+" }",
		"flowchart TD\nA@{ label: "+strings.Repeat("[", 500)+strings.Repeat("]", 500)+" }",
		"flowchart TD\nA@{ w: 1e999, h: -1e999, label: .nan, icon: .inf }",
		"flowchart TD\nlinkStyle 99999999999999999999 stroke:red\nA --> B",
		"flowchart TD\nA --> B\nlinkStyle 4294967295 stroke:red",
		"flowchart TD\nclick A call "+strings.Repeat("f(", 300),
		"flowchart TD\nclick A call f("+strings.Repeat(`",`, 2000)+")",
		cyclic,
		"flowchart TD\nsubgraph A\nA\nend\nsubgraph A\nA\nend\nA --> A\nA@{ view: collapsed }",
		"flowchart TD\nA e@--> B\ne@{ animate: [1, {a: b}], animation: {x: 1}, curve: [] }\ne --> e",
		"---\nconfig:\n  flowchart: 5\n  look: [a]\n  layout: {a: b}\n---\nflowchart TD\nsubgraph S\nA --> B\nend",
		"---\nconfig:\n  flowchart:\n    padding: x\n    minNodeWidth: [1]\n    curve: 5\n    inheritDir: yes\n---\nflowchart TD\nsubgraph S\nA --> B\nend",
	)

	type failure struct {
		text    string
		problem string
	}
	failures := make([]failure, len(texts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, text := range texts {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, text string) {
			defer wg.Done()
			defer func() { <-sem }()
			done := make(chan string, 1)
			go func() {
				defer func() {
					if p := recover(); p != nil {
						buf := make([]byte, 4096)
						buf = buf[:runtime.Stack(buf, false)]
						done <- fmt.Sprintf("panic: %v\n%s", p, buf)
						return
					}
					done <- ""
				}()
				exercise(text)
			}()
			select {
			case problem := <-done:
				failures[i] = failure{text, problem}
			case <-time.After(2 * time.Minute):
				failures[i] = failure{text, "no result after 2 minutes"}
			}
		}(i, text)
	}
	wg.Wait()
	for _, f := range failures {
		if f.problem != "" {
			t.Errorf("%s\n--- text ---\n%s", f.problem, clip(f.text))
		}
	}
	t.Logf("%d texts: no panic and no hang", len(texts))
}
