package agentflow

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

// fuzzAlphabet has the characters that the grammar of agentflow reacts to.
var fuzzAlphabet = []string{
	"[", "]", "(", ")", "{", "}", "|", "-", "=", ".", ">", "<", "&", ":", ";", "@", "\"", "`", "#", ",", "~", "x", "^", "%",
	"\n", " ", "\t", "%%", "@{", "}\n", "-->", "--x", "-.-", "-- t -->", ":::", "end", "flow ", "global\n", "connector ", "direction LR",
	"click ", "call ", "href ", "style ", "classDef ", "class ", "linkStyle ", "default", "interpolate ", "accTitle: ",
	"accDescr {", "agentflow-beta ", "é", "😀", " ", "ﬂ°", "¶ß", "&amp;", "<br/>", "shape: ", "label: ", "view: collapsed", "0", "9",
	"tool", "decision", ": |\n", "&a ", "*a", "!!str ", "__proto__", "constructor", "hasOwnProperty", "connectors",
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
// then builds the layout data and reads the db, as the renderer does. It
// runs the text two times: with the text that Diagram.fromText chooses, and
// with the text of the pipeline.
func exercise(text string) {
	for _, pipeline := range []bool{false, true} {
		o := renderCapture(text, nil, &capture{pipeline: pipeline})
		if o.cp == nil {
			continue
		}
		d := o.cp.db
		data := o.cp.data
		for _, n := range data.Nodes {
			_ = js.StringifyJSON(nodeJSON(n))
		}
		for _, e := range data.Edges {
			_ = js.StringifyJSON(edgeJSON(e))
		}
		_ = js.StringifyJSON(dbState(d))
		// getData can run more than one time: the diagnostics stay the same.
		before := len(d.GetDiagnostics())
		d.GetData()
		if after := len(d.GetDiagnostics()); after != before {
			panic(fmt.Sprintf("a second getData changes the number of diagnostics from %d to %d", before, after))
		}
	}
	_ = stripLineTrailingCommas(text)
	_ = prepareText(text)
}

// TestMutatedCorpus feeds mutated corpus fixtures and hostile texts to the
// parser and the db. The requirement is: no panic and no hang. An error is
// a good result.
func TestMutatedCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "agentflow")
	var fixtures []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".mmd") {
			fixtures = append(fixtures, path)
		}
		return nil
	})
	sort.Strings(fixtures)
	if len(fixtures) == 0 {
		t.Fatal("no corpus fixtures")
	}

	const mutantsPerFixture = 12
	rnd := rand.New(rand.NewSource(0x6167666c))
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
	nested := strings.Repeat("flow s\n", 3000) + "a\n" + strings.Repeat("end\n", 3000)
	var chain strings.Builder
	for i := 0; i < 1500; i++ {
		fmt.Fprintf(&chain, "flow f%d\n", i)
	}
	chain.WriteString("a\n")
	chain.WriteString(strings.Repeat("end\n", 1500))
	chain.WriteString("f0@{ view: collapsed }\nf700@{ view: collapsed }\nx --> a\n")
	cyclic := "agentflow-beta TB\nflow A\nB\nend\nflow B\nC\nend\nflow C\nA\nend\nA@{ view: collapsed }\nB@{ view: collapsed }\nA --> B\n"
	texts = append(texts,
		"", "agentflow-beta", "agentflow-beta TB", "agentflow-beta TB\n@{", "agentflow-beta TB\nA@{", "agentflow-beta TB\nA@{\n",
		"agentflow-beta TB\n"+nested,
		"agentflow-beta TB\n"+chain.String(),
		"agentflow-beta TB\n"+strings.Repeat("a --> b & c & d\n", 400),
		"agentflow-beta TB\n"+strings.Repeat("a", 20000)+" --> b",
		"agentflow-beta TB\nA["+strings.Repeat("[", 5000),
		"agentflow-beta TB\nA@{ shape: "+strings.Repeat("x", 5000)+" }",
		"agentflow-beta TB\nA@{ label: "+strings.Repeat("[", 500)+strings.Repeat("]", 500)+" }",
		"agentflow-beta TB\nA@{\n"+strings.Repeat("  a:\n", 300)+"}",
		"agentflow-beta TB\nA@{ w: 1e999, h: -1e999, label: .nan, icon: .inf }",
		"agentflow-beta TB\nA@{\n  a: &x [*x]\n  label: *x\n}",
		"agentflow-beta TB\nA@{\n  label: &x\n    - *x\n}",
		"agentflow-beta TB\nA@{\n"+strings.Repeat("  k: \"v\",\n", 3000)+"}",
		"agentflow-beta TB\nlinkStyle 99999999999999999999 stroke:red\nA --> B",
		"agentflow-beta TB\nA --> B\nlinkStyle 4294967295 stroke:red",
		"agentflow-beta TB\nclick A call "+strings.Repeat("f(", 300),
		"agentflow-beta TB\nclick A call f("+strings.Repeat(`",`, 2000)+")",
		cyclic,
		"agentflow-beta TB\nflow A\nA\nend\nflow A\nA\nend\nA --> A\nA@{ view: collapsed }",
		"agentflow-beta TB\nA e@--> B\ne@{ animate: [1, 2], animation: [x], curve: [] }\ne --> e",
		"agentflow-beta TB\nglobal\n"+strings.Repeat("flow g\nx\nend\n", 200)+"end\nflow g\nx\nend",
		"agentflow-beta TB\n"+strings.Repeat("connector c[\"C\"]\nc@{ a: 1 }\n", 500),
		"agentflow-beta TB\nflow   [\"empty id\"]\nA\nend\n  @{ view: collapsed }",
		"---\nconfig:\n  flowchart: 5\n  agentflow: x\n  look: [a]\n  layout: {a: b}\n---\nagentflow-beta TB\nflow S\nA --> B\nend",
		"---\nconfig:\n  flowchart:\n    padding: x\n    curve: 5\n    inheritDir: yes\n  agentflow:\n    minNodeWidth: [1]\n    wrappingWidth: {a: 1}\n    nodeSpacing: x\n---\nagentflow-beta TB\nflow S\nA --> B\nend",
		"---\nconfig:\n  themeVariables:\n    borderColorArray: 5\n---\nagentflow-beta TB\nflow S\nA --> B\nend",
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
