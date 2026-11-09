package sequence

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// TestExtraRenderAgainstMermaid renders the diagrams of
// testdata/extra_render.json.gz and compares each SVG with the SVG that
// mermaid.render() returns in Chromium. tools/reference/gen-sequence-extra.mjs
// writes the file: hand-written diagrams for the parts of the renderer that
// no fixture of the corpus uses. The corpus itself is the job of the parity
// command.
//
// The SVG must be byte-identical. A diagram that mermaid fails to render
// must fail here too.
//
// knownDivergences lists the diagrams that differ because of a package that
// the sequence diagram does not own, with the cause. The test reports them
// in the log. For a difference in the style element, the rest of the SVG
// must still be byte-identical.
var knownDivergences = map[string]string{
	"font-family-empty": "style: for fontFamily \"\" the shared createCssStyles writes the rule :root{--mermaid-font-family:;}, and Chromium drops a rule with an empty value",
	"unicode":           "engine: the bounding box of a text with an emoji glyph is 19.37 px high here and 22 px in Chromium, so the note is 3 px lower",
}

var reStyleElement = regexp.MustCompile(`(?s)<style>.*?</style>`)

func TestExtraRenderAgainstMermaid(t *testing.T) {
	golden, ok := readGzipJSON(t, "extra_render.json.gz").([]any)
	if !ok || len(golden) < 60 {
		t.Fatalf("extra_render.json.gz has %d diagrams", len(golden))
	}
	only := os.Getenv("SEQ_EXTRA")
	// The SVG has the text widths of Chromium on the macOS reference
	// machine. A machine with other fonts compares only the errors.
	reference := fonttest.IsMacOSReference()
	exact, known, failures := 0, 0, 0
	for _, c := range golden {
		rec := c.(*js.Obj[any])
		name := js.ToString(rec.Get("name"))
		if only != "" && only != name {
			continue
		}
		text := js.ToString(rec.Get("text"))
		got, err := render.Render(text, render.Options{})
		if wantErr, isErr := rec.Get("error").(string); isErr {
			failures++
			if err == nil {
				t.Errorf("%s: no error, mermaid fails with %q", name, wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: error %v", name, err)
			continue
		}
		if !reference {
			continue
		}
		want := js.ToString(rec.Get("svg"))
		verdict, diff := parity.CompareSVG(got.SVG, want, 0)
		if verdict != parity.Exact {
			if dir := os.Getenv("SEQ_EXTRA_OUT"); dir != "" {
				_ = os.WriteFile(dir+"/"+name+".got.svg", []byte(got.SVG), 0o644)
				_ = os.WriteFile(dir+"/"+name+".want.svg", []byte(want), 0o644)
			}
			cause, isKnown := knownDivergences[name]
			if !isKnown {
				t.Errorf("%s: %s: %v", name, verdict, diff)
				continue
			}
			if strings.HasPrefix(cause, "style:") &&
				reStyleElement.ReplaceAllString(got.SVG, "") != reStyleElement.ReplaceAllString(want, "") {
				t.Errorf("%s: differs outside of the style element: %v", name, diff)
				continue
			}
			known++
			t.Logf("%s: known difference (%s): %v", name, cause, diff)
			continue
		}
		if _, isKnown := knownDivergences[name]; isKnown {
			t.Logf("%s: byte-identical now, remove it from knownDivergences", name)
		}
		exact++
	}
	t.Logf("%d diagrams, %d byte-identical, %d known differences, %d fail in mermaid", len(golden), exact, known, failures)
	fonttest.RequireMacOSReference(t)
}
