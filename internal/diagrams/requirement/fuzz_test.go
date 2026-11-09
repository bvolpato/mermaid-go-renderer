package requirement

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

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// fuzzAlphabet has the pieces that the grammar of a requirement diagram
// reacts to.
var fuzzAlphabet = []string{
	"{", "}", ":", ":::", ",", "-", "->", "<-", "\"", "#", ";", "%", "%%", "=", "<", ">", ".", "\n", " ", "\t", "\r\n",
	"requirementDiagram\n", "requirement ", "functionalRequirement ", "interfaceRequirement ", "performanceRequirement ",
	"physicalRequirement ", "designConstraint ", "element ", "id: ", "text: ", "risk: ", "verifymethod: ", "type: ",
	"docref: ", "high", "low", "medium", "test", "analysis", "inspection", "demonstration", " - contains -> ",
	" <- copies - ", " - derives -> ", " - satisfies -> ", " - verifies -> ", " - refines -> ", " - traces -> ",
	"direction LR", "direction TB\n", "style ", "classDef ", "class ", "default", "fill:#f00", "color:blue",
	"stroke-width:4px", "accTitle: ", "accDescr: ", "accDescr {", "title ", "é", "😀", "日本", " ", "ﬂ°", "¶ß",
	"&amp;", "<br/>", "<b>", "**", "_", "`", "fa:fa-user", "$$x$$", "0", "9", "---\n", "config:\n  look: handDrawn\n",
	"config:\n  htmlLabels: false\n", "config:\n  layout: dagre\n", "%%{init: {\"theme\": \"dark\"}}%%\n",
}

// hostileTexts are texts that no mutation of a fixture gives.
var hostileTexts = []string{
	"",
	"requirementDiagram",
	"requirementDiagram\n",
	"requirement",
	"requirementDiagram\n" + strings.Repeat("requirement r {\n}\n", 40),
	"requirementDiagram\n" + strings.Repeat("{", 200),
	"requirementDiagram\n" + strings.Repeat("}", 200),
	"requirementDiagram\nrequirement \"" + strings.Repeat("a", 3000) + "\" {\nid: 1\n}\n",
	"requirementDiagram\nrequirement r {\ntext: \"" + strings.Repeat("word ", 400) + "\"\n}\n",
	"requirementDiagram\nrequirement r {\ntext: \"" + strings.Repeat("**b** _i_ ", 100) + "\"\n}\n",
	"requirementDiagram\nrequirement r {\n" + strings.Repeat("id: 1\n", 300) + "}\n",
	"requirementDiagram\nrequirement a {\nid: 1\n}\n" + strings.Repeat("a - contains -> a\n", 60),
	"requirementDiagram\nrequirement a {\nid: 1\n}\na - contains -> b\nc <- traces - a\nd - copies -> e\n",
	"requirementDiagram\nrequirement a {\nid: 1\n}\nelement a {\ntype: x\n}\na - traces -> a\n",
	"requirementDiagram\nrequirement \" \" {\nid: 1\n}\n",
	"requirementDiagram\nrequirement \"\t\" {\nid: \" \"\ntext: \"​\"\n}\n",
	"requirementDiagram\nrequirement r {\nid: 1\n}\nstyle r " + strings.Repeat("fill:#f00,", 200) + "color:red\n",
	"requirementDiagram\nrequirement r {\nid: 1\n}\nclassDef " + strings.Repeat("a,", 200) + "b fill:red\nclass r " + strings.Repeat("a,", 200) + "b\n",
	"requirementDiagram\nrequirement r:::" + strings.Repeat("c,", 100) + "d {\nid: 1\n}\n",
	"requirementDiagram\nrequirement r {\ntext: \"$$x^2$$\"\n}\n",
	"requirementDiagram\nrequirement r {\ntext: \"<img src=x onerror=alert(1)><script>alert(1)</script><svg onload=x>\"\n}\n",
	"requirementDiagram\nrequirement r {\ntext: \"</div></foreignObject></g></svg><svg>\"\n}\n",
	"requirementDiagram\nrequirement \"\xff\xfe\" {\nid: \"\xed\xa0\x80\"\n}\n",
	"requirementDiagram\nrequirement \"a\x00b\" {\nid: 1\n}\n",
	"requirementDiagram\naccDescr {\n",
	"requirementDiagram\naccTitle: " + strings.Repeat("<b>", 500) + "\n",
	"---\ntitle: " + strings.Repeat("t", 2000) + "\n---\nrequirementDiagram\nrequirement r {\nid: 1\n}\n",
	"---\nconfig:\n  state: 5\n  themeVariables: null\n---\nrequirementDiagram\nrequirement r {\nid: 1\n}\n",
	"---\nconfig:\n  themeVariables:\n    borderColorArray: 7\n---\nrequirementDiagram\nrequirement r {\nid: 1\n}\n",
	"---\nconfig:\n  state:\n    nodeSpacing: x\n    rankSpacing: [1]\n    titleTopMargin: {a: 1}\n    useMaxWidth: 0\n---\nrequirementDiagram\nrequirement r {\nid: 1\n}\n",
	"---\nconfig:\n  look: 5\n  layout: [elk]\n  htmlLabels: maybe\n  fontSize: big\n  fontFamily: 7\n---\nrequirementDiagram\nrequirement r {\nid: 1\n}\nelement e {\ntype: t\n}\nr - traces -> e\n",
	"---\nconfig:\n  handDrawnSeed: -1\n  look: handDrawn\n---\nrequirementDiagram\nrequirement r {\nid: 1\n}\n",
}

// validStatements are statements that a diagram can end with. A fixture
// with some of them at its end still parses, so the text gets to the db,
// the layout data and the shape.
var validStatements = []string{
	"requirement extra_req {\nid: 9\ntext: an extra text\nrisk: low\nverifymethod: analysis\n}",
	"functionalRequirement \"a quoted name\" {\nid: \"q: 1, 2\"\ntext: \"**bold** _italic_ `code` <b>b</b> <br/> x\"\n}",
	"performanceRequirement only_name {\n}",
	"interfaceRequirement verify_only {\nverifymethod: demonstration\n}",
	"physicalRequirement long_text {\ntext: \"" + strings.Repeat("many words ", 30) + "\"\n}",
	"designConstraint Ünï_日本_😀 {\nid: \"#35; ﬂ°lt¶ß\"\ntext: \"fa:fa-user icon and &amp; entity\"\n}",
	"requirement test_req {\nid: other\n}",
	"requirement 42 {\nid: 1\n}",
	"element extra_el {\ntype: simulation\ndocref: reqs/x\n}",
	"element \"quoted element\" {\ntype: \"a type\"\n}",
	"element empty_el {\n}",
	"element test_entity {\ndocref: again\n}",
	"extra_req - contains -> extra_el",
	"extra_req - copies -> extra_req",
	"extra_el - derives -> test_req",
	"test_entity - satisfies -> test_req",
	"test_req - verifies -> nowhere",
	"nowhere - refines -> nothing",
	"extra_req <- traces - extra_el",
	"\"a quoted name\" - traces -> \"quoted element\"",
	"direction LR",
	"direction BT",
	"direction RL",
	"direction TB",
	"classDef important fill:#f96,stroke:#333,stroke-width:4px,color:#00f",
	"classDef c1,c2 color:#fff,fill:#00f",
	"classDef default fill:#eee,color:#111,font-size:20px",
	"class extra_req important",
	"class extra_req,extra_el,test_req c1,c2",
	"style extra_req fill:#ffa,stroke:#000,color:green",
	"style test_req,extra_el stroke-width:2px,stroke:red",
	"extra_req:::important",
	"test_entity:::c1,c2",
	"requirement styled:::important {\nid: 2\n}",
	"element styled_el:::c1,c2 {\ntype: x\n}",
	"accTitle: The title",
	"accDescr: The description",
	"accDescr {\n  several\n  lines\n}",
}

// extendText adds statements to the end of a diagram.
func extendText(rnd *rand.Rand, text string) string {
	var sb strings.Builder
	sb.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		sb.WriteString("\n")
	}
	for i, n := 0, 1+rnd.Intn(6); i < n; i++ {
		sb.WriteString(validStatements[rnd.Intn(len(validStatements))])
		sb.WriteString("\n")
	}
	return sb.String()
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
	return string(b)
}

// exercise renders a text: the parser, the db, the layout data, and, with
// the layout of the tests, the shape of every node, the title and the
// viewport. An error is a valid outcome. The result says whether the text
// rendered.
func exercise(text string) bool {
	_, err := render.Render(text, render.Options{})
	return err == nil
}

// TestMutatedCorpus feeds mutated corpus fixtures and hostile texts to the
// diagram. The requirement is: no panic and no hang. An error is fine.
func TestMutatedCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "requirement")
	var fixtures []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".mmd") {
			fixtures = append(fixtures, path)
		}
		return nil
	})
	sort.Strings(fixtures)
	if len(fixtures) == 0 {
		t.Fatal("no fixtures of the requirement family")
	}
	mutants := 6
	if testing.Short() {
		mutants = 1
	}
	rnd := rand.New(rand.NewSource(0x72657146))
	texts := append([]string{}, hostileTexts...)
	for _, f := range fixtures {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		// Most mutants change only the diagram, and not the front matter:
		// a change of the front matter rarely gets to the parser.
		text := string(data)
		head, body := "", text
		if strings.HasPrefix(text, "---\n") {
			if at := strings.Index(text[4:], "\n---\n"); at >= 0 {
				head, body = text[:4+at+5], text[4+at+5:]
			}
		}
		for i := 0; i < mutants; i++ {
			switch {
			case i == 5:
				texts = append(texts, mutateText(rnd, text))
			case i%2 == 0:
				texts = append(texts, extendText(rnd, text))
			default:
				texts = append(texts, head+mutateText(rnd, extendText(rnd, body)))
			}
		}
	}

	type failure struct {
		text   string
		reason string
	}
	var (
		mu       sync.Mutex
		failures []failure
		rendered int
		wg       sync.WaitGroup
	)
	sem := make(chan struct{}, runtime.NumCPU())
	for _, text := range texts {
		wg.Add(1)
		sem <- struct{}{}
		go func(text string) {
			defer wg.Done()
			defer func() { <-sem }()
			done := make(chan string, 1)
			go func() {
				defer func() {
					if p := recover(); p != nil {
						done <- fmt.Sprintf("panic: %v", p)
						return
					}
					done <- ""
				}()
				if exercise(text) {
					mu.Lock()
					rendered++
					mu.Unlock()
				}
			}()
			select {
			case reason := <-done:
				if reason != "" {
					mu.Lock()
					failures = append(failures, failure{text, reason})
					mu.Unlock()
				}
			case <-time.After(60 * time.Second):
				mu.Lock()
				failures = append(failures, failure{text, "no result after 60 s"})
				mu.Unlock()
			}
		}(text)
	}
	wg.Wait()
	for i, f := range failures {
		if i >= 10 {
			break
		}
		text := f.text
		if len(text) > 600 {
			text = text[:600] + "..."
		}
		t.Errorf("%s\n%q", f.reason, text)
	}
	t.Logf("%d texts: %d rendered, %d returned an error, %d failures", len(texts), rendered, len(texts)-rendered-len(failures), len(failures))
	if rendered == 0 {
		t.Error("no text rendered: the test does not reach the shape")
	}
}
