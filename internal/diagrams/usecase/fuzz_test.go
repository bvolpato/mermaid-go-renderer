package usecase

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

// fuzzAlphabet has the characters that the grammar of the diagram reacts to.
var fuzzAlphabet = []string{
	"[", "]", "(", ")", "{", "}", "|", "-", "=", ".", ">", "<", "&", ":", ";", "@", "\"", "'", "`", "#", ",", "~", "x", "o",
	"\n", " ", "\t", "\r\n", "%%", "@{", "}\n", "-->", "---", "<--", "--o", "x--", "..>", "--|>", ":::", "<<", ">>", "\"`", "`\"",
	"end", "systemBoundary ", "direction TB", "actor ", "note for ", "json ", "style ", "classDef ", "class ", "default",
	"accTitle: ", "accDescr {", "usecase-beta", "include", "extend", "true", "false", "type: hollow", "business: true",
	"icon: \"fa:x\"", "animate: true", "é", "😀", " ", "ﬂ°", "¶ß", "&amp;", "<br/>", "0", "9", "1.5px", "\\,",
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

// renderToLayout runs a text through the pipeline up to the layout: the
// parser, the db and the part of the draw function that builds the layout
// data.
func renderToLayout(text string) {
	cp := &capture{stop: true}
	loc := newCapture(cp)
	defer captures.Delete(loc)
	_, _ = render.Render(text, render.Options{Location: loc})
}

// renderAll runs a text through the whole pipeline, with the real layout.
func renderAll(text string) {
	_, _ = render.Render(text, render.Options{})
}

// TestMutatedCorpus feeds mutated corpus fixtures and hostile texts to the
// renderer. The requirement is: no panic and no hang. An error is a good
// result. The mutated fixtures go through the whole pipeline, with the
// layout and the shapes. The hostile texts, which are large, stop at the
// layout.
func TestMutatedCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "usecase")
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

	type job struct {
		text string
		run  func(text string)
	}
	const mutantsPerFixture = 3
	rnd := rand.New(rand.NewSource(0x75636173))
	var jobs []job
	for _, f := range fixtures {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		// The fixtures of the corpus have escaped stereotypes (&lt;&lt;),
		// which the parser rejects. With the real characters the mutants
		// reach the stereotype branches of the shapes too.
		text := strings.NewReplacer("&lt;", "<", "&gt;", ">").Replace(string(data))
		jobs = append(jobs, job{text, renderAll})
		for i := 0; i < mutantsPerFixture; i++ {
			jobs = append(jobs, job{mutateText(rnd, text), renderAll})
		}
	}
	// Texts that aim at the limits of the lexer, the parser and the db.
	uc := func(body string) string { return "usecase-beta\n" + body + "\n" }
	hostile := []string{
		"", "usecase-beta", "usecase", "usecase-beta\n@{", uc("A@{"), uc("A@{\n"), uc("actor"), uc("\""), uc("'"), uc("\"`"),
		uc("<<"), uc("A <<"), uc("A <<\n>>"), uc("json J@{"), uc("json J@{\"a\":"), uc("accDescr {"),
		uc(strings.Repeat("actor A\n", 2000)),
		uc(strings.Repeat("A --> B\n", 2000)),
		uc("A(" + strings.Repeat("x ", 10000) + ")"),
		uc("A" + strings.Repeat("-", 20000) + "> B"),
		uc("A " + strings.Repeat("--> A ", 3000)),
		uc("A -- " + strings.Repeat("label ", 5000) + "--> B"),
		uc("A -- " + strings.Repeat("label ", 5000)),
		uc("actor " + strings.Repeat("A, ", 5000) + "B"),
		uc("classDef " + strings.Repeat("a,", 5000) + "b fill:red"),
		uc("classDef a " + strings.Repeat("fill:red,", 5000) + "stroke:blue"),
		uc("A@{" + strings.Repeat("\n", 5000) + "}"),
		uc("A@{ " + strings.Repeat("business: true, ", 3000) + "}"),
		uc("systemBoundary S\n" + strings.Repeat("  A\n", 3000) + "end"),
		uc(strings.Repeat("systemBoundary S\n", 500) + strings.Repeat("end\n", 500)),
		uc("json J@{\"a\":" + strings.Repeat("[", 10000) + strings.Repeat("]", 10000) + "}"),
		uc("json J@{" + strings.Repeat("\"a\":{", 5000) + strings.Repeat("}", 5000) + "}"),
		uc("json J@{\"a\": [" + strings.Repeat("1,", 20000) + "2]}"),
		uc("json J@{\"position 99999999999999999999999999999999999999\": x}"),
		uc("json J@{\"line 99999999999999999999 column 99999999999999999999\": x, \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaa\": 1}"),
		uc("json J@{\"a\": 1e99999, \"b\": -1e99999, \"c\": 123456789012345678901234567890}"),
		uc("json J@{\"" + strings.Repeat("\\u0041", 5000) + "\": 1}"),
		uc("A <<" + strings.Repeat("s", 20000) + ">>"),
		uc("note for A " + strings.Repeat("\"x\" ", 2000)),
		uc("\"`" + strings.Repeat("**x** ", 3000) + "`\""),
		uc(strings.Repeat("\"A b\"\n", 3)),
		"---\nconfig:\n  usecase: 5\n  look: [a]\n  layout: {a: b}\n---\nusecase-beta\nA --> B\n",
		"---\nconfig:\n  usecase:\n    nodeSpacing: x\n    rankSpacing: [1]\n    wrappingWidth: {a: 1}\n    minNodeWidth: yes\n    diagramPadding: abc\n    useMaxWidth: [0]\n    actorFontSize: {}\n    actorFontFamily: 5\n---\nusecase-beta\nactor A\nA --> B\n",
		"---\nconfig:\n  themeVariables: 5\n  theme: [x]\n---\nusecase-beta\nactor A\nA --> B(Biz)@{ business: true }\n",
	}
	for _, text := range hostile {
		jobs = append(jobs, job{text, renderToLayout})
	}

	failures := make([]string, len(jobs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, j job) {
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
				j.run(j.text)
			}()
			select {
			case problem := <-done:
				failures[i] = problem
			case <-time.After(2 * time.Minute):
				failures[i] = "no result after 2 minutes"
			}
		}(i, j)
	}
	wg.Wait()
	for i, problem := range failures {
		if problem != "" {
			text := jobs[i].text
			if len(text) > 600 {
				text = text[:600] + "..."
			}
			t.Errorf("%s\n--- text ---\n%s", problem, text)
		}
	}
	t.Logf("%d texts: no panic and no hang", len(jobs))
}
