package er

import (
	"path/filepath"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// serifCJKFallback is the cause of the known divergences. erBox gives each
// label the width calculateTextWidth(text, config) + 100, and an HTML label
// has it as its max-width. calculateTextWidth measures the text in the
// default font of the browser, a serif font: the font family of the default
// config ends with a semicolon, so the browser does not accept it. For a
// character that the serif font does not have, Chromium takes the fallback
// font from the serif list of the system: a Han character is 16 px wide at
// 16 px, and a Hangul syllable 15.984375 px. The font library of this port
// (internal/font) has one fallback list for every font: it gives 16.328125 px
// and 13.84375 px, which are the values of Chromium for a sans-serif font.
const serifCJKFallback = "the width of Han and Hangul text in a serif font: internal/font has no serif CJK fallback"

// knownDivergences are the cases of the golden files for which this port
// has another result than upstream, each with the cause. The test fails
// when such a case agrees: remove it then.
var knownDivergences = map[string]string{
	"erBox/set-han-html":  serifCJKFallback,
	"erBox/name-han-html": serifCJKFallback,
}

// TestShapeAgainstMermaid replays the golden files of the shape erBox:
// cases recorded from the real insertNode of mermaid in Chromium by
// tools/reference/gen-shape-cases-er.mjs. It compares the markup, the size
// of the node and the intersections.
func TestShapeAgainstMermaid(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("testdata", "erBox*.json.gz"))
	if len(files) == 0 {
		t.Fatal("no golden files of the shape erBox")
	}
	seen := map[string]bool{}
	skipped := 0
	for _, f := range files {
		cases := rutest.LoadShapeCases(t, f)
		counts := map[string]int{}
		for _, c := range cases {
			r := rutest.RunShapeCase(c)
			cause, known := knownDivergences[c.Name]
			if known {
				seen[c.Name] = true
			}
			switch {
			case rutest.SkipComparison(c.HTML):
				// The case has text, and this machine does not have the
				// fonts of the reference machine: only an error fails
				// the test.
				skipped++
				if r.Status == "error" || r.Status == "not-ported" {
					t.Errorf("%s: %s\n    %s", r.Name, r.Status, r.Detail)
				}
			case known:
				if r.Status == "exact" || r.Status == "equivalent" {
					t.Errorf("%s: %s, but it is a known divergence (%s): remove it from knownDivergences", c.Name, r.Status, cause)
				} else {
					r.Status = "known-divergence"
				}
			case r.Status != "exact" && r.Status != "equivalent":
				t.Errorf("%s: %s\n    %s", r.Name, r.Status, r.Detail)
			}
			counts[r.Status]++
		}
		t.Logf("%s: %d cases: exact=%d equivalent=%d known-divergence=%d different=%d error=%d not-ported=%d",
			f, len(cases), counts["exact"], counts["equivalent"], counts["known-divergence"], counts["different"], counts["error"], counts["not-ported"])
	}
	for name := range knownDivergences {
		if !seen[name] {
			t.Errorf("knownDivergences has %q, which is not a case of the golden files", name)
		}
	}
	rutest.RequireFonts(t, skipped)
}
