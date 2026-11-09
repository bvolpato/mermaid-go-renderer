package architecture

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// family is the corpus directory with the fixtures of this diagram type.
const family = "architecture"

// renderLimit is the time limit of one render of a mutated input. A layout
// takes up to some seconds of CPU time, and the machine can be busy.
const renderLimit = 4 * time.Minute

// snippets are the texts that mutate inserts.
var snippets = []string{
	"architecture-beta", "group", "service", "junction", "align", "row", "column", " in ", ":", "--", "-", "---\n",
	"L", "R", "T", "B", "{group}", "<", ">", "(server)", "(database)", "(nosuchicon)", "(a:b)", "(", ")", "[Title]",
	"[\"quoted\"]", "[]", "[", "]", "\"", "'", "\"txt\"", "\\", "\n", " ", "\t", "%%", ";", ",", ".", "0", "17",
	"x", "db", "api", "title T", "accTitle: A", "accDescr { d }", "\x00", "\xff", "é", "😀",
	"  service z(disk)[Z]\n", "  group zg(cloud)[ZG]\n", "  junction zj\n", "  db:L -- R:server\n", "  z:T -[lbl]-> B:db\n",
	"  align row db server\n", "  align column db db\n",
	"%%{init: {\"architecture\": {\"iconSize\": \"x\", \"padding\": null, \"fontSize\": -3}}}%%\n",
	"%%{init: {\"architecture\": {\"seed\": 0, \"randomize\": true, \"numIter\": 3}}}%%\n",
	"%%{init: {\"architecture\": {\"iconSize\": 0, \"nodeSeparation\": -5, \"idealEdgeLengthMultiplier\": 1e300}}}%%\n",
	"%%{init: {\"fontSize\": \"big\", \"fontFamily\": \"nosuchfont\"}}%%\n",
}

func readFixtures(t testing.TB) []string {
	t.Helper()
	files, err := filepath.Glob("../../../testdata/corpus/mermaid-e2e/" + family + "/*/*.mmd")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	texts := make([]string, len(files))
	for i, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		texts[i] = string(data)
	}
	return texts
}

// mutate returns a variant of text with one to four random edits.
func mutate(text string, rnd *rand.Rand) string {
	for edits := 1 + rnd.Intn(4); edits > 0; edits-- {
		at := rnd.Intn(len(text) + 1)
		switch rnd.Intn(6) {
		case 0: // delete some bytes
			end := min(len(text), at+1+rnd.Intn(4))
			text = text[:at] + text[end:]
		case 1, 2: // insert a snippet
			text = text[:at] + snippets[rnd.Intn(len(snippets))] + text[at:]
		case 3: // cut the end
			text = text[:at]
		case 4: // duplicate a line
			lines := strings.Split(text, "\n")
			i := rnd.Intn(len(lines))
			lines = append(lines[:i+1], lines[i:]...)
			text = strings.Join(lines, "\n")
		default: // swap two lines
			lines := strings.Split(text, "\n")
			i, j := rnd.Intn(len(lines)), rnd.Intn(len(lines))
			lines[i], lines[j] = lines[j], lines[i]
			text = strings.Join(lines, "\n")
		}
	}
	return text
}

// renderNoPanic renders the text. An error is a valid result. A panic is
// not, and neither is a render that does not end.
func renderNoPanic(t *testing.T, text string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan string, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Sprint("panic: ", r)
			}
		}()
		_, _ = render.Render(text, render.Options{Context: ctx})
		done <- ""
	}()
	select {
	case msg := <-done:
		if msg != "" {
			t.Errorf("text %q: %s", text, msg)
		}
	case <-time.After(renderLimit):
		t.Fatalf("text %q: the render does not end", text)
	}
}

// TestMutatedCorpus renders mutated corpus fixtures. No input may cause a
// panic or a hang.
func TestMutatedCorpus(t *testing.T) {
	bodies, whole := 6, 2
	if testing.Short() {
		bodies, whole = 2, 0
	}
	rnd := rand.New(rand.NewSource(1))
	var texts []string
	for _, fixture := range readFixtures(t) {
		// The front matter holds a description. Mutate the diagram more
		// often than the description.
		body := fixture
		if i := strings.LastIndex(fixture, "\n---\n"); i >= 0 {
			body = fixture[i+5:]
		}
		for i := 0; i < bodies; i++ {
			texts = append(texts, mutate(body, rnd))
		}
		for i := 0; i < whole; i++ {
			texts = append(texts, mutate(fixture, rnd))
		}
	}
	t.Run("inputs", func(t *testing.T) {
		for i, text := range texts {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				t.Parallel()
				renderNoPanic(t, text)
			})
		}
	})
	t.Logf("%d mutated inputs", len(texts))
}

// TestMalformedInputs renders texts that reach the error paths of the db
// and of the layout, and config values of a wrong type. No input may cause
// a panic.
func TestMalformedInputs(t *testing.T) {
	cfg := func(directive string) string {
		return "%%{init: {\"architecture\": " + directive + "}}%%\narchitecture-beta\n group g(cloud)[G]\n service a(server)[A] in g\n service b \"txt\" [B]\n junction j\n a{group}:R -[x]-> L:b\n b:B -- T:j\n"
	}
	for _, text := range []string{
		"architecture-beta",
		"architecture-beta\nservice a\na:L -- R:a\n",
		"architecture-beta\nservice a\nservice b\na:L -- R:b\na:L -- R:b\n",
		"architecture-beta\ngroup g\nservice a\ng:L -- R:a\n",
		"architecture-beta\nservice a\nservice b\nservice c\na:R -- L:b\nb:R -- L:c\nalign column a c\n",
		"architecture-beta\nservice a\nservice b\nalign row a b\nalign row b a\nalign column a b\n",
		"architecture-beta\ngroup g\ngroup h in g\ngroup i in h\njunction j in i\n",
		cfg(`{"iconSize": "x"}`),
		cfg(`{"iconSize": null, "padding": null, "fontSize": null}`),
		cfg(`{"iconSize": -80, "padding": -40, "fontSize": -16}`),
		cfg(`{"iconSize": 0, "padding": 0, "fontSize": 0}`),
		cfg(`{"iconSize": 1e308, "padding": 1e308}`),
		cfg(`{"fontSize": "16px", "padding": "40px"}`),
		cfg(`{"seed": "abc", "randomize": "yes", "numIter": "many"}`),
		cfg(`{"seed": 0, "randomize": true, "numIter": 0}`),
		cfg(`{"nodeSeparation": -1, "idealEdgeLengthMultiplier": -1, "edgeElasticity": -1}`),
		cfg(`{"nodeSeparation": null, "idealEdgeLengthMultiplier": null, "edgeElasticity": null, "numIter": null, "seed": null}`),
		cfg(`{"useMaxWidth": "x", "iconSize": [1, 2], "padding": {"a": 1}}`),
		cfg(`"text"`),
		cfg(`[1, 2, 3]`),
		cfg(`null`),
	} {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			renderNoPanic(t, text)
		})
	}
}

// FuzzRender is the same check for the fuzzing engine of Go.
func FuzzRender(f *testing.F) {
	// The seeds are the short fixtures: a layout of a long one takes seconds.
	for _, fixture := range readFixtures(f) {
		if len(fixture) < 700 {
			f.Add(fixture)
		}
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 4000 {
			return
		}
		renderNoPanic(t, text)
	})
}
