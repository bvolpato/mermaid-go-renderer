package class

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// shapeGolden is the golden file of the classBox shape. A case has the
// format of rutest.ShapeCase, but its config and its style sheet are
// indexes into the lists of the file.
type shapeGolden struct {
	Configs []json.RawMessage `json:"configs"`
	Styles  []string          `json:"styles"`
	Cases   []struct {
		rutest.ShapeCase
		Config *int `json:"config"`
		CSS    *int `json:"css"`
	} `json:"cases"`
}

func loadShapeCases(t testing.TB) []rutest.ShapeCase {
	t.Helper()
	f, err := os.Open("testdata/classbox.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var g shapeGolden
	if err := json.NewDecoder(zr).Decode(&g); err != nil {
		t.Fatal(err)
	}
	cases := make([]rutest.ShapeCase, len(g.Cases))
	for i, c := range g.Cases {
		cases[i] = c.ShapeCase
		if c.Config != nil {
			cases[i].Config = g.Configs[*c.Config]
		}
		if c.CSS != nil {
			cases[i].CSS = g.Styles[*c.CSS]
		}
	}
	return cases
}

// knownShapeDivergences are the cases that differ from upstream because of
// another package. The value has two texts that the description of the
// difference must contain. The test fails when the case has another
// difference, or none.
var knownShapeDivergences = map[string][2]string{
	// The first member of the class is "+你好 世界". addText gives
	// createText the width calculateTextWidth(text) + 50. The font family
	// of the config ends in a semicolon, so the second measurement of
	// calculateTextDimensions has no valid font family and uses the
	// default font of the page, Times. Chromium takes the Han characters
	// from a serif font then (an advance of 1 em), and internal/font takes
	// them from the sans-serif font of the system for every font (an
	// advance of 1.02 em). The rounded width is 83 here and 82 upstream.
	"classBox/unicode-neo":     {"max-width: 133px", "max-width: 132px"},
	"classBox/unicode-classic": {"max-width: 133px", "max-width: 132px"},
}

// TestClassBoxAgainstMermaid replays the cases of the classBox shape that
// tools/reference/gen-class-shapes.mjs recorded from the real insertNode of
// mermaid in Chromium: it compares the markup, the size of the node and
// the intersections.
func TestClassBoxAgainstMermaid(t *testing.T) {
	cases := loadShapeCases(t)
	counts := map[string]int{}
	known, skipped := 0, 0
	for _, c := range cases {
		r := rutest.RunShapeCase(c)
		counts[r.Status]++
		if rutest.SkipComparison(c.HTML) {
			// The case has text, and this machine does not have the fonts
			// of the reference machine: only an error fails the test.
			skipped++
			if r.Status == "error" || r.Status == "not-ported" {
				t.Errorf("%s: %s\n    %s", r.Name, r.Status, r.Detail)
			}
			continue
		}
		if want, ok := knownShapeDivergences[r.Name]; ok {
			known++
			if r.Status != "different" || !strings.Contains(r.Detail, want[0]) || !strings.Contains(r.Detail, want[1]) {
				t.Errorf("%s: known divergence: got %s %q, want a difference with %q and %q", r.Name, r.Status, r.Detail, want[0], want[1])
			}
			continue
		}
		if r.Status != "exact" && r.Status != "equivalent" {
			t.Errorf("%s: %s\n    %s", r.Name, r.Status, r.Detail)
		}
	}
	t.Logf("%d cases: exact=%d equivalent=%d different=%d error=%d not-ported=%d, known divergences=%d",
		len(cases), counts["exact"], counts["equivalent"], counts["different"], counts["error"], counts["not-ported"], known)
	rutest.RequireFonts(t, skipped)
}
