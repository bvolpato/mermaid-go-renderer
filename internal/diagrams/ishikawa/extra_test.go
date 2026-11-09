package ishikawa

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
// mermaid, each with its cause.
var knownDifferences = map[string]string{}

// TestExtraFixturesAgainstMermaid renders the fixtures of testdata/extra
// and compares each SVG with the SVG that real mermaid renders in Chromium.
// The fixtures cover config options and branches that the corpus does not
// have. A result must be byte-identical, or equal with numbers within the
// tolerance of the parity command. A fixture that mermaid does not render
// must return an error. Regenerate the reference with
// node tools/reference/gen-venn-ishikawa-extra.mjs.
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
		SVGs   map[string]string `json:"svgs"`
		Errors map[string]string `json:"errors"`
	}
	if err := json.NewDecoder(zr).Decode(&ref); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("testdata/extra/*.mmd")
	if err != nil || len(files) == 0 || len(files) != len(ref.SVGs)+len(ref.Errors) {
		t.Fatalf("%d fixtures, %d references, error %v", len(files), len(ref.SVGs)+len(ref.Errors), err)
	}
	// The SVG has the text widths of Chromium on the macOS reference
	// machine. A machine with other fonts compares only the errors.
	reference := fonttest.IsMacOSReference()
	exact, equivalent, failed := 0, 0, 0
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".mmd")
		text, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		res, err := render.Render(string(text), render.Options{})
		if message, fails := ref.Errors[name]; fails {
			if err == nil {
				t.Errorf("%s: no error, mermaid has %q", name, message)
			} else {
				failed++
			}
			continue
		}
		want, ok := ref.SVGs[name]
		if !ok {
			t.Errorf("%s: no reference", name)
			continue
		}
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
	t.Logf("%d fixtures: %d exact, %d equivalent, %d fail as in mermaid", len(files), exact, equivalent, failed)
	fonttest.RequireMacOSReference(t)
}
