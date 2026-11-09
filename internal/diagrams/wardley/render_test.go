package wardley

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// renderCase is one case of testdata/render.json.gz, which
// tools/reference/gen-treeview-cynefin-wardley-db.mjs wrote with mermaid
// 12.2.0 in Chromium: a text and the SVG that mermaid.render returns for
// it, or the error.
type renderCase struct {
	Name  string  `json:"name"`
	Text  string  `json:"text"`
	SVG   string  `json:"svg"`
	Error *string `json:"error"`
}

// engineDivergences are the cases whose SVG differs because the measurement
// engine (internal/browser, internal/font) differs from Chromium, with the
// cause. The renderer of this package is not the cause. A case that renders
// the same SVG again must leave the list.
var engineDivergences = map[string]string{}

// TestRenderAgainstMermaid renders texts with config options and inputs
// that the corpus does not have, and requires the SVG text of mermaid. The
// parity command covers the corpus.
func TestRenderAgainstMermaid(t *testing.T) {
	var cases []renderCase
	readGolden(t, "render.json.gz", &cases)
	if len(cases) < 150 {
		t.Fatalf("the golden file has %d cases", len(cases))
	}
	// The SVG has the text widths of Chromium on the macOS reference
	// machine. A machine with other fonts compares only the errors.
	reference := fonttest.IsMacOSReference()
	same, rejected, known := 0, 0, 0
	for _, c := range cases {
		result, err := render.Render(c.Text, render.Options{})
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
		if !reference {
			continue
		}
		want := c.SVG
		if reason, ok := engineDivergences[c.Name]; ok {
			if result.SVG == want {
				t.Errorf("%s: the SVG is the same now, remove the case from engineDivergences (%s)", c.Name, reason)
			}
			known++
			continue
		}
		if result.SVG != want {
			i := 0
			for i < len(want) && i < len(result.SVG) && want[i] == result.SVG[i] {
				i++
			}
			lo, hi := max(0, i-100), func(s string) int { return min(len(s), i+140) }
			t.Errorf("%s: the SVG differs at byte %d\n got  …%s…\n want …%s…\n%s", c.Name, i, result.SVG[lo:hi(result.SVG)], want[lo:hi(want)], c.Text)
			continue
		}
		same++
	}
	t.Logf("%d cases: %d with the same SVG, %d that mermaid and the port reject, %d with a difference of the measurement engine",
		len(cases), same, rejected, known)
	fonttest.RequireMacOSReference(t)
}
