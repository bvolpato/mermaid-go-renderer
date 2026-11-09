package git

import (
	"encoding/json"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// TestRenderChromiumGolden compares the SVG of render.Render with the SVG
// of upstream mermaid in Chromium, byte for byte, for diagrams that are not
// in the corpus: every theme and direction, the options of the gitGraph
// config, text with markup and site configs. The parity command covers the
// corpus. The golden file comes from tools/reference/gitgraph-db-oracle.mjs.
func TestRenderChromiumGolden(t *testing.T) {
	var golden struct {
		Cases []struct {
			Name   string          `json:"name"`
			Text   string          `json:"text"`
			Config json.RawMessage `json:"config"`
			SVG    string          `json:"svg"`
			Error  string          `json:"error"`
		} `json:"cases"`
	}
	readGolden(t, "render_chromium.json.gz", &golden)
	if len(golden.Cases) < 140 {
		t.Fatalf("golden file has %d cases", len(golden.Cases))
	}
	// The SVG has the text widths of Chromium on the macOS reference
	// machine. A machine with other fonts compares only the errors.
	reference := fonttest.IsMacOSReference()
	exact := 0
	for _, c := range golden.Cases {
		var site *js.Obj[any]
		if len(c.Config) > 0 {
			v, err := js.ParseJSON(c.Config)
			if err != nil {
				t.Fatal(err)
			}
			site = v.(*js.Obj[any])
		}
		res, err := render.Render(c.Text, render.Options{Config: site})
		switch {
		case c.Error != "":
			if err == nil {
				t.Errorf("%s: upstream fails with %q, got an SVG", c.Name, c.Error)
				continue
			}
		case err != nil:
			t.Errorf("%s: %v", c.Name, err)
			continue
		case !reference:
			continue
		case res.SVG != c.SVG:
			i := 0
			for i < len(res.SVG) && i < len(c.SVG) && res.SVG[i] == c.SVG[i] {
				i++
			}
			lo := max(0, i-100)
			t.Errorf("%s: the SVG differs at byte %d\n got  %s\n want %s", c.Name, i,
				res.SVG[lo:min(len(res.SVG), i+140)], c.SVG[lo:min(len(c.SVG), i+140)])
			continue
		}
		exact++
	}
	t.Logf("%d of %d cases are the same", exact, len(golden.Cases))
	fonttest.RequireMacOSReference(t)
}
