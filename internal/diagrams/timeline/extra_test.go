package timeline

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// knownDifferences names the fixtures of testdata/extra that differ from
// mermaid for a reason outside of this package.
var knownDifferences = map[string]string{
	// The text has an emoji. The engine has the advance of an emoji glyph
	// and not yet its ink bounds, so the box of the text is too small.
	"lr-unicode": "the bounding box of text with an emoji glyph",
}

// TestExtraFixturesAgainstMermaid renders the fixtures of testdata/extra
// and compares each SVG with the SVG that real mermaid renders in Chromium.
// The fixtures cover config options and branches that the corpus does not
// have. A result must be byte-identical, or equal with numbers within the
// tolerance of the parity command. Regenerate the reference with
// node tools/reference/gen-timeline-journey-extra.mjs.
func TestExtraFixturesAgainstMermaid(t *testing.T) {
	f, err := os.Open("testdata/extra-ref.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		SVGs map[string]string `json:"svgs"`
	}
	if err := json.NewDecoder(zr).Decode(&ref); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("testdata/extra/*.mmd")
	if err != nil || len(files) == 0 || len(files) != len(ref.SVGs) {
		t.Fatalf("%d fixtures, %d references, error %v", len(files), len(ref.SVGs), err)
	}
	// The SVG has the text widths of Chromium on the macOS reference
	// machine. A machine with other fonts checks only that each fixture
	// renders.
	reference := fonttest.IsMacOSReference()
	exact, equivalent := 0, 0
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".mmd")
		text, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		want, ok := ref.SVGs[name]
		if !ok {
			t.Errorf("%s: no reference", name)
			continue
		}
		res, err := render.Render(string(text), render.Options{})
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !reference {
			continue
		}
		verdict, diff := parity.CompareSVG(res.SVG, want, 0.01)
		switch verdict {
		case parity.Exact:
			exact++
		case parity.Equivalent:
			equivalent++
		default:
			if reason, known := knownDifferences[name]; known {
				t.Logf("%s: different, known: %s", name, reason)
				continue
			}
			t.Errorf("%s: %s", name, diff)
		}
	}
	t.Logf("%d fixtures: %d exact, %d equivalent", len(files), exact, equivalent)
	fonttest.RequireMacOSReference(t)
}
