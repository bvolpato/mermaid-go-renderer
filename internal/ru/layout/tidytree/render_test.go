package tidytree_test

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/mmdc"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// mmdcCase is one diagram of testdata/mmdc.json.gz: what mermaid-cli 12
// writes for it in the Linux reference image (tools/reference/gen-tidytree.mjs
// --mmdc).
type mmdcCase struct {
	Name        string `json:"name"`
	Text        string `json:"text"`
	DiagramType string `json:"diagramType"`
	// SVG is the .svg file of mmdc --no-font-embed.
	SVG string `json:"svg"`
	// Error is the message of the error when mermaid fails.
	Error string `json:"error"`
}

func readMmdcCases(t *testing.T) []mmdcCase {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "mmdc.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Mermaid string     `json:"mermaid"`
		Mmdc    string     `json:"mmdc"`
		Cases   []mmdcCase `json:"cases"`
	}
	if err := json.NewDecoder(zr).Decode(&golden); err != nil {
		t.Fatal(err)
	}
	if golden.Mermaid != "12.2.0" || golden.Mmdc != "12.0.0" {
		t.Fatalf("the golden file is from mermaid %s and mermaid-cli %s", golden.Mermaid, golden.Mmdc)
	}
	if len(golden.Cases) < 41 {
		t.Fatalf("the golden file has %d diagrams", len(golden.Cases))
	}
	return golden.Cases
}

// renderPage renders a diagram in a page of the given kind with the
// embedded fonts, which are the fonts of the Linux reference image. For the
// page of mermaid-cli the result is the file of mmdc --no-font-embed.
func renderPage(text string, page render.Page) (string, error) {
	fonts := font.Embedded()
	if page == render.PageMermaidCLI {
		fonts = fonts.WithWebFonts()
	}
	got, err := render.Render(text, render.Options{Fonts: fonts, Page: page})
	if err != nil {
		return "", err
	}
	if page != render.PageMermaidCLI {
		return got.SVG, nil
	}
	file, err := mmdc.Load(got.SVG, mmdc.Options{Background: "white", SetBackground: true})
	if err != nil {
		return "", err
	}
	return file.XML(), nil
}

// firstDifference describes the first byte at which two texts differ.
func firstDifference(got, want string) string {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	from := max(i-60, 0)
	return fmt.Sprintf("at byte %d\n got:  ...%s\n want: ...%s", i, got[from:min(i+80, len(got))], want[from:min(i+80, len(want))])
}

// TestMermaidCLIGolden renders diagrams with `layout: tidy-tree` in the
// page of mermaid-cli and compares the bytes with the files that
// mermaid-cli wrote: diagrams of each type that can ask for the layout,
// the looks, graphs that are not trees, and graphs for which the layout
// throws.
func TestMermaidCLIGolden(t *testing.T) {
	exact, agreeError := 0, 0
	types := map[string]int{}
	for _, c := range readMmdcCases(t) {
		got, err := renderPage(c.Text, render.PageMermaidCLI)
		if c.Error != "" {
			switch {
			case err == nil:
				t.Errorf("%s: got a result, want the error %q", c.Name, c.Error)
			case err.Error() != c.Error:
				t.Errorf("%s: got the error %q, want %q", c.Name, err, c.Error)
			default:
				agreeError++
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: got the error %q, want a result", c.Name, err)
			continue
		}
		if got != c.SVG {
			t.Errorf("%s: the result differs %s", c.Name, firstDifference(got, c.SVG))
			continue
		}
		exact++
		types[c.DiagramType]++
	}
	t.Logf("%d byte-exact, %d with the error of mermaid; byte-exact by diagram type: %v", exact, agreeError, types)
}

// otherLayout replaces the layout that a diagram asks for, in its front
// matter or in a directive, with a name that no page registers.
func otherLayout(t *testing.T, text string) string {
	t.Helper()
	if strings.Count(text, "tidy-tree") != 1 {
		t.Fatalf("the diagram does not ask for the layout one time:\n%s", text)
	}
	return strings.Replace(text, "tidy-tree", "no-such-layout", 1)
}

// layoutNotUsed has the diagrams that name the layout and do not get it in
// the page of mermaid-cli. The mindmap reads the layout from a copy of the
// config in which it sets "cose-bilkent" when the user set no top-level
// layout. So `mindmap.layout` alone does not select a layout.
var layoutNotUsed = map[string]bool{"mindmap-scoped-layout": true}

// TestLayoutIsOnlyInThePageOfMermaidCLI checks the selection. A plain page
// does not have the layout: there `layout: tidy-tree` gives what a name
// that nothing registers gives, which is the fallback layout of the
// diagram. The page of mermaid-cli has it: there the two results differ,
// but for the diagrams of layoutNotUsed.
func TestLayoutIsOnlyInThePageOfMermaidCLI(t *testing.T) {
	checked := 0
	for _, c := range readMmdcCases(t) {
		if c.Error != "" {
			continue
		}
		other := otherLayout(t, c.Text)
		plain, err := renderPage(c.Text, render.PagePlain)
		if err != nil {
			t.Errorf("%s: plain page: %v", c.Name, err)
			continue
		}
		plainOther, err := renderPage(other, render.PagePlain)
		if err != nil {
			t.Errorf("%s: plain page, other layout: %v", c.Name, err)
			continue
		}
		if plain != plainOther {
			t.Errorf("%s: a plain page uses the layout: the result differs from the fallback %s", c.Name, firstDifference(plain, plainOther))
		}
		cliOther, err := renderPage(other, render.PageMermaidCLI)
		if err != nil {
			t.Errorf("%s: page of mermaid-cli, other layout: %v", c.Name, err)
			continue
		}
		if (cliOther == c.SVG) != layoutNotUsed[c.Name] {
			if layoutNotUsed[c.Name] {
				t.Errorf("%s: the page of mermaid-cli uses the layout: the result differs from the fallback %s", c.Name, firstDifference(c.SVG, cliOther))
			} else {
				t.Errorf("%s: the page of mermaid-cli gives the same result with and without the layout", c.Name)
			}
		}
		checked++
	}
	if checked < 20 {
		t.Errorf("%d diagrams checked", checked)
	}
}

// pieces are lines that a mutation inserts: more branches, cycles, groups
// and nodes with no edge.
var pieces = []string{
	"  A --> B", "  B --> A", "  A --> A", "  X", "  subgraph G", "  end", "    child", "      deeper", "  id((Circle))",
	"  id))Bang((", "  root --> Z", "  A --> B & C", "  Idle --> [*]", "  class Foo", "",
}

// mutate removes, repeats, swaps and inserts lines of the body of a
// diagram. It keeps the front matter, which asks for the layout.
func mutate(rnd *rand.Rand, text string) string {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return text
	}
	front, body, ok := strings.Cut(rest, "---\n")
	if !ok {
		return text
	}
	lines := strings.Split(body, "\n")
	for n := 1 + rnd.Intn(4); n > 0; n-- {
		// The first line has the keyword of the diagram.
		at := 1 + rnd.Intn(len(lines))
		switch rnd.Intn(5) {
		case 0:
			if at < len(lines) {
				lines = append(lines[:at:at], lines[at+1:]...)
			}
		case 1:
			if at < len(lines) {
				lines = append(lines[:at+1:at+1], append([]string{lines[at]}, lines[at+1:]...)...)
			}
		case 2:
			if at < len(lines) {
				to := 1 + rnd.Intn(len(lines)-1)
				lines[at], lines[to] = lines[to], lines[at]
			}
		default:
			at = min(at, len(lines))
			lines = append(lines[:at:at], append([]string{pieces[rnd.Intn(len(pieces))]}, lines[at:]...)...)
		}
	}
	return "---\n" + front + "---\n" + strings.Join(lines, "\n")
}

// TestMutatedDiagrams renders seeded mutations of the diagrams with the
// layout in the page of mermaid-cli. A render must return, with a result
// or an error, and must not panic.
func TestMutatedDiagrams(t *testing.T) {
	cases := readMmdcCases(t)
	rnd := rand.New(rand.NewSource(0x74696479))
	count := 150
	if testing.Short() {
		count = 30
	}
	rendered, failed := 0, 0
	for i := 0; i < count; i++ {
		text := mutate(rnd, cases[rnd.Intn(len(cases))].Text)
		done := make(chan string, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Sprint("panic: ", r)
				}
			}()
			if _, err := renderPage(text, render.PageMermaidCLI); err != nil {
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
		case <-time.After(2 * time.Minute):
			t.Fatalf("input %d: the render does not return\n--- text ---\n%s", i, text)
		}
	}
	t.Logf("%d inputs: %d rendered, %d with an error", count, rendered, failed)
	if rendered < count/3 {
		t.Errorf("%d of %d inputs rendered: the mutations do not reach the layout", rendered, count)
	}
}
