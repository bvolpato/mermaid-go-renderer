package kanban

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// renderCase is one case of testdata/render.json.gz, which
// tools/reference/gen-kanban-render.mjs wrote with mermaid 12.2.0 in
// Chromium: a text and the SVG that mermaid.render returns for it, or the
// error.
type renderCase struct {
	Name  string `json:"name"`
	Text  string `json:"text"`
	Type  string `json:"type"`
	SVG   string `json:"svg"`
	Error string `json:"error"`
}

func loadRenderCases(tb testing.TB) []renderCase {
	tb.Helper()
	f, err := os.Open("testdata/render.json.gz")
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	var cases []renderCase
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		tb.Fatal(err)
	}
	return cases
}

// TestRenderAgainstMermaid renders kanban texts that have items, shape data
// and config options, which the corpus does not have, and compares the SVG
// with the SVG of mermaid: the same text, or the same tree with numbers
// within 0.01.
func TestRenderAgainstMermaid(t *testing.T) {
	cases := loadRenderCases(t)
	if len(cases) < 50 {
		t.Fatalf("only %d cases", len(cases))
	}
	// The SVG has the text widths of Chromium on the macOS reference
	// machine. A machine with other fonts compares only the errors.
	reference := fonttest.IsMacOSReference()
	exact, equivalent, rejected := 0, 0, 0
	for _, c := range cases {
		result, err := render.Render(c.Text, render.Options{})
		if c.Error != "" {
			rejected++
			if err == nil {
				t.Errorf("%s: no error, mermaid fails with %q\n%s", c.Name, c.Error, c.Text)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: error %v, mermaid renders the text\n%s", c.Name, err, c.Text)
			continue
		}
		if !reference {
			continue
		}
		if result.SVG == c.SVG {
			exact++
			continue
		}
		verdict, diff := parity.CompareSVG(result.SVG, c.SVG, 0.01)
		if verdict == parity.Different {
			t.Errorf("%s: the SVG differs: %s\n%s", c.Name, diff.String(), c.Text)
			continue
		}
		equivalent++
	}
	t.Logf("%d cases: %d exact, %d equivalent, %d that mermaid and the port reject", len(cases), exact, equivalent, rejected)
	fonttest.RequireMacOSReference(t)
}
