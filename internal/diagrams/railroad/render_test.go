package railroad

import (
	"encoding/json"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// renderCase is one case of testdata/render.json.gz, which
// tools/reference/gen-railroad-render.mjs wrote with mermaid 12.2.0 in
// Chromium: a text, an optional site config, and the SVG that mermaid.render
// returns for it, or the error.
type renderCase struct {
	Name  string          `json:"name"`
	Text  string          `json:"text"`
	Site  json.RawMessage `json:"site"`
	Type  string          `json:"type"`
	SVG   string          `json:"svg"`
	Error *string         `json:"error"`
}

// TestRenderAgainstMermaid renders texts that reach the branches of the
// renderer and the config options that the corpus does not have, and
// requires the SVG text of mermaid.
func TestRenderAgainstMermaid(t *testing.T) {
	var golden struct {
		Cases []renderCase `json:"cases"`
	}
	loadGzipJSON(t, "testdata/render.json.gz", &golden)
	if len(golden.Cases) < 120 {
		t.Fatalf("only %d cases", len(golden.Cases))
	}
	// The SVG has the text widths of Chromium on the macOS reference
	// machine. A machine with other fonts compares only the errors and the
	// diagram type.
	reference := fonttest.IsMacOSReference()
	same, rejected := 0, 0
	for _, c := range golden.Cases {
		result, err := render.Render(c.Text, render.Options{Config: siteConfig(t, c.Site)})
		if c.Error != nil {
			rejected++
			if err == nil {
				t.Errorf("%s: no error, mermaid fails with %q\n%s", c.Name, *c.Error, c.Text)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: error %v, mermaid renders the text\n%s", c.Name, err, c.Text)
			continue
		}
		if result.Type != c.Type {
			t.Errorf("%s: type %q, want %q", c.Name, result.Type, c.Type)
			continue
		}
		if !reference {
			continue
		}
		want := c.SVG
		if result.SVG != want {
			i := 0
			for i < len(want) && i < len(result.SVG) && want[i] == result.SVG[i] {
				i++
			}
			lo, hi := max(0, i-80), func(s string) int { return min(len(s), i+100) }
			t.Errorf("%s: the SVG differs at byte %d\n got  …%s…\n want …%s…\n%s", c.Name, i, result.SVG[lo:hi(result.SVG)], want[lo:hi(want)], c.Text)
			continue
		}
		same++
	}
	t.Logf("%d cases: %d with the same SVG, %d that mermaid and the port reject", len(golden.Cases), same, rejected)
	fonttest.RequireMacOSReference(t)
}
