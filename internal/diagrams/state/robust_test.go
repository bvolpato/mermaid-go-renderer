package state

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// corpusTexts returns the texts of the state diagram fixtures of the corpus.
func corpusTexts(tb testing.TB) []string {
	tb.Helper()
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e")
	var files []string
	for _, family := range []string{"state-diagram", "state-diagram-v2"} {
		err := filepath.WalkDir(filepath.Join(root, family), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".mmd") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			tb.Fatal(err)
		}
	}
	sort.Strings(files)
	texts := make([]string, len(files))
	for i, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			tb.Fatal(err)
		}
		texts[i] = string(data)
	}
	return texts
}

// prng is the mulberry32 generator, with a fixed seed.
type prng uint32

func (p *prng) next() uint32 {
	*p += 0x6d2b79f5
	t := uint32(*p)
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return t ^ (t >> 14)
}

func (p *prng) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(p.next() % uint32(n))
}

// fragments are the pieces that a mutation inserts: the tokens of the
// grammar, and bytes that are not valid text.
var fragments = []string{
	"[*]", "-->", "--", ":", "::", ":::", "{", "}", "\"", "\n", " ", "\t", ";", ",", "%%", "#", "<<fork>>", "<<join>>",
	"<<choice>>", "[[fork]]", "state ", "state \"x\" as ", " as ", "note ", "note right of ", "note left of ", "end note",
	"classDef ", "class ", "style ", "direction LR", "direction ", "click ", "href ", "accTitle: ", "accDescr: ", "accDescr {",
	"hide empty description", "scale 10 width", "stateDiagram", "stateDiagram-v2", "---", "<br/>", "<b>", "</b>", "<script>",
	"&amp;", "#35;", "root", "nl", "\r", "\x00", "\xff", "\xed\xa0\x80", "\u2028", "😀", "é", "\ufeff",
}

// mutate changes a text at a few random places.
func mutate(p *prng, text string) string {
	b := []byte(text)
	for edits := 1 + p.intn(4); edits > 0; edits-- {
		at := p.intn(len(b) + 1)
		switch p.intn(6) {
		case 0: // delete a few bytes
			end := min(at+1+p.intn(8), len(b))
			b = append(b[:at:at], b[end:]...)
		case 1, 2: // insert a fragment
			frag := fragments[p.intn(len(fragments))]
			b = append(b[:at:at], append([]byte(frag), b[at:]...)...)
		case 3: // repeat a part
			end := min(at+1+p.intn(40), len(b))
			part := append([]byte{}, b[at:end]...)
			b = append(b[:end:end], append(part, b[end:]...)...)
		case 4: // cut the end
			if at > len(b)/2 {
				b = b[:at]
			}
		case 5: // swap two lines
			lines := strings.Split(string(b), "\n")
			i, j := p.intn(len(lines)), p.intn(len(lines))
			lines[i], lines[j] = lines[j], lines[i]
			b = []byte(strings.Join(lines, "\n"))
		}
	}
	return string(b)
}

// tryInput parses and draws a text. It returns a description of a panic,
// or "".
func tryInput(text string) (problem string) {
	defer func() {
		if r := recover(); r != nil {
			problem = fmt.Sprintf("panic: %v", r)
		}
	}()
	// The steps of the pipeline up to the layout data.
	if p, err := parseText(text, nil, false, 0); err == nil && p.db != nil {
		_, _ = layoutData(p.ctx, p.db)
	}
	// The whole pipeline, with the stub layout of draw_test.go.
	_, _ = render.Render(text, render.Options{})
	return ""
}

// TestMutatedInputs feeds mutated corpus fixtures to the parser, the db and
// the renderer. No input may cause a panic or a hang.
func TestMutatedInputs(t *testing.T) {
	texts := corpusTexts(t)
	if len(texts) == 0 {
		t.Fatal("no corpus fixtures")
	}
	mutants := 6
	if testing.Short() {
		mutants = 1
	}
	const limit = 30 * time.Second
	p := prng(0x73746174)
	count, failed := 0, 0
	for _, text := range texts {
		for i := 0; i < mutants; i++ {
			input := mutate(&p, text)
			count++
			done := make(chan string, 1)
			go func() { done <- tryInput(input) }()
			select {
			case problem := <-done:
				if problem != "" {
					failed++
					if failed <= 10 {
						t.Errorf("%s\n%s", problem, indent(input))
					}
				}
			case <-time.After(limit):
				t.Fatalf("no result after %v\n%s", limit, indent(input))
			}
		}
	}
	t.Logf("%d mutated inputs, %d with a panic", count, failed)
}
