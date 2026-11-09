package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// corpusTexts reads the fixtures of the gitGraph corpus family.
func corpusTexts(t testing.TB) []string {
	t.Helper()
	var texts []string
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "git")
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
		t.Fatal("no corpus fixtures")
	}
	return texts
}

// mutator makes malformed variants of a text from a seeded generator.
type mutator struct{ state uint32 }

func (m *mutator) next() uint32 {
	m.state += 0x6d2b79f5
	t := m.state
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return t ^ (t >> 14)
}

func (m *mutator) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(m.next() % uint32(n))
}

var fuzzWords = []string{
	"commit", "branch", "merge", "checkout", "switch", "cherry-pick", "gitGraph", "gitGraph:", "LR", "TB", "BT",
	"id:", "msg:", "tag:", "type:", "order:", "parent:", ":", "NORMAL", "REVERSE", "HIGHLIGHT",
	`"`, `'`, `"x"`, `"<br>"`, `"<b>x</b>"`, `"\`, `\`, "0", "12.5", "99999999999999999999", "-1",
	"%%", "%%{", "}%%", "---", "title", "accTitle:", "accDescr {", "}", "\n", "\r", "\t", " ", "\x00", "\xff", "é", "😀",
	"main", "develop", "a/b.c-", "#lt;", "ﬂ°°35¶ß", "<", ">", "&", "=",
}

func (m *mutator) mutate(text string) string {
	switch m.intn(8) {
	case 0:
		at := m.intn(len(text) + 1)
		end := min(len(text), at+1+m.intn(8))
		return text[:at] + text[end:]
	case 1, 2:
		at := m.intn(len(text) + 1)
		return text[:at] + fuzzWords[m.intn(len(fuzzWords))] + text[at:]
	case 3:
		at := m.intn(len(text) + 1)
		return text[:at] + " " + fuzzWords[m.intn(len(fuzzWords))] + " " + text[at:]
	case 4:
		lines := strings.Split(text, "\n")
		i, j := m.intn(len(lines)), m.intn(len(lines))
		lines[i], lines[j] = lines[j], lines[i]
		return strings.Join(lines, "\n")
	case 5:
		lines := strings.Split(text, "\n")
		i := m.intn(len(lines))
		lines = append(lines[:i+1], lines[i:]...)
		return strings.Join(lines, "\n")
	case 6:
		return text[:m.intn(len(text)+1)]
	default:
		if text == "" {
			return text
		}
		b := []byte(text)
		b[m.intn(len(b))] = byte(m.next())
		return string(b)
	}
}

// TestMutatedCorpusDoesNotPanic renders mutated corpus inputs. A malformed
// input must give an error or a result: no panic and no hang.
func TestMutatedCorpusDoesNotPanic(t *testing.T) {
	texts := corpusTexts(t)
	m := &mutator{state: 0x67697433}
	variants := 4
	if testing.Short() {
		variants = 1
	}
	renderOne := func(text string) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		_, _ = render.Render(text, render.Options{})
		return nil
	}
	total := 0
	for _, text := range texts {
		for v := 0; v < variants; v++ {
			mutated := text
			for k := 1 + m.intn(3); k > 0; k-- {
				mutated = m.mutate(mutated)
			}
			total++
			done := make(chan error, 1)
			go func() { done <- renderOne(mutated) }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("%v\ninput: %q", err, mutated)
				}
			case <-time.After(30 * time.Second):
				t.Fatalf("the render does not end\ninput: %q", mutated)
			}
		}
	}
	t.Logf("%d mutated inputs", total)
}

// TestParserMalformedInputs feeds hostile texts to the parser alone.
func TestParserMalformedInputs(t *testing.T) {
	inputs := []string{
		"", "\x00", "\xff\xfe", "gitGraph\xff", "gitGraph\ncommit id: \"\xff\"", "gitGraph\ncommit id: \"\\",
		"gitGraph\naccDescr {", "gitGraph\naccDescr", "gitGraph\n---", "gitGraph\n%%{", "gitGraph\n" + strings.Repeat("commit\n", 5000),
		"gitGraph\ncommit " + strings.Repeat(`tag: "t" `, 5000), "gitGraph\nbranch " + strings.Repeat("a", 100000),
		"gitGraph\n" + strings.Repeat("---\n", 3000), "gitGraph\n" + strings.Repeat("%%{ x ", 3000),
		strings.Repeat("\n", 100000) + "gitGraph", "gitGraph\ncommit id: " + strings.Repeat(`"`, 9999),
	}
	for _, input := range inputs {
		ast, err := parseGitGraphText(input)
		if err == nil && ast == nil {
			t.Errorf("no result and no error for %q", input[:min(len(input), 40)])
		}
	}
}
