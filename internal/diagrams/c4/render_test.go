package c4

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// renderCase is one case of testdata/render.json.gz, which
// tools/reference/gen-c4-render.mjs writes from real mermaid in Chromium.
type renderCase struct {
	Name   string          `json:"name"`
	Text   string          `json:"text"`
	Config json.RawMessage `json:"config"`
	Error  *string         `json:"error"`
	SVG    string          `json:"svg"`
}

func loadRenderCases(t testing.TB) []renderCase {
	t.Helper()
	data, err := os.ReadFile("testdata/render.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var cases []renderCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// knownGap describes a case of TestRenderGolden that is not equal byte for
// byte, for a cause outside of this package.
type knownGap struct {
	// worst is the worst verdict that the case can have, with the tolerance
	// of the parity command.
	worst parity.Verdict
	cause string
}

// The causes of the known gaps. The first one was confirmed with a build
// in which the five functions call their correctly rounded versions: the
// cases are then equal byte for byte.
const (
	// js.Sin, js.Cos, js.Tan, js.Atan and js.Asin are the functions of the
	// Go math package. The functions of Chromium are correctly rounded
	// (js.SinCR and the like). internal/rough and the person shape of
	// internal/ru call the first group. The last digits of the rough paths
	// and of the points where a relationship meets a person differ.
	gapMath = "js.Sin, js.Cos, js.Tan, js.Atan and js.Asin are not correctly rounded"
	// The engine has other advances and line heights than Chromium for text
	// that needs a fallback font: emoji and CJK characters.
	gapFallbackFonts = "internal/browser measures text in fallback fonts differently"
)

var knownGaps = map[string]knownGap{
	"look-handdrawn":        {parity.Equivalent, gapMath},
	"look-handdrawn-seed":   {parity.Equivalent, gapMath},
	"look-handdrawn-forest": {parity.Equivalent, gapMath},
	"handdrawn-styles":      {parity.Equivalent, gapMath},
	"site-config":           {parity.Equivalent, gapMath},
	"wrap-rels":             {parity.Equivalent, gapMath},
	"rels":                  {parity.Equivalent, gapMath},
	"dynamic":               {parity.Equivalent, gapMath},
	"labels-fallback-fonts": {parity.Different, gapFallbackFonts},
}

// TestRenderGolden compares the rendered SVG with the SVG of real mermaid
// for diagrams that use the code that the corpus fixtures do not reach: the
// looks handDrawn and neo, the framed rectangle, shape keywords, wrapping,
// the text placements, font config and render errors. The SVG must be equal
// byte for byte, except for the cases of knownGaps. For a text that mermaid
// fails to render, the port must return an error too.
//
// The SVG has the text widths of Chromium on the macOS reference machine. A
// machine with other fonts compares only the errors, and the test is marked
// as skipped.
func TestRenderGolden(t *testing.T) {
	cases := loadRenderCases(t)
	if len(cases) < 40 {
		t.Fatalf("only %d cases", len(cases))
	}
	reference := fonttest.IsMacOSReference()
	exact := 0
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			opts := render.Options{}
			if len(c.Config) > 0 {
				site, err := js.ParseJSON(c.Config)
				if err != nil {
					t.Fatal(err)
				}
				opts.Config = js.AsObj(site)
			}
			got, err := render.Render(c.Text, opts)
			if c.Error != nil {
				if err == nil {
					t.Fatalf("mermaid fails with %q, the port renders the text", *c.Error)
				}
				exact++
				return
			}
			if err != nil {
				t.Fatalf("the port fails with %v, mermaid renders the text", err)
			}
			if !reference {
				return
			}
			if got.SVG == c.SVG {
				exact++
				return
			}
			// The tolerance of the parity command.
			verdict, diff := parity.CompareSVG(got.SVG, c.SVG, 0.01)
			if gap, ok := knownGaps[c.Name]; ok && verdict <= gap.worst {
				t.Logf("known gap: the SVG is %s: %s", verdict, gap.cause)
				return
			}
			if diff != nil && verdict == parity.Different {
				t.Fatalf("the SVG is %s: %s", verdict, diff)
			}
			t.Fatalf("the SVG is %s: the texts differ", verdict)
		})
	}
	t.Logf("%d of %d cases are equal byte for byte", exact, len(cases))
	fonttest.RequireMacOSReference(t)
}

// mulberry32 is the generator of the reference tools.
func mulberry32(seed uint32) func() float64 {
	a := seed
	return func() float64 {
		a += 0x6d2b79f5
		t := a
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		return float64(t^(t>>14)) / 4294967296
	}
}

// mutate changes a diagram text in a few places.
func mutate(rnd func() float64, text string) string {
	specials := []string{
		`"`, "$", "=", "(", ")", "{", "}", ",", ",,", `""`, "\n", " ", "%%", "#", ";", "<", ">", "<br/>",
		"Rel", "Person", "title ", "UpdateElementStyle(", "$shape=\"", "$offsetX=\"", "global", "é", "\U0001F600",
		"\xed\xa0\xbd", "\x00", "$__proto__=\"x\"", "$typeC4Shape=\"x\"", "$label=\"x\"", "$type=\"x\"", "$intersect=\"x\"",
		"$x=\"1e400\"", "$width=\"NaN\"", "UpdateLayoutConfig(\"99999999999\", \"0\")",
	}
	s := []byte(text)
	edits := 1 + int(rnd()*4)
	for i := 0; i < edits; i++ {
		p := int(rnd() * float64(len(s)+1))
		switch int(rnd() * 4) {
		case 0:
			end := min(len(s), p+1+int(rnd()*8))
			s = append(s[:p:p], s[end:]...)
		case 1:
			ins := specials[int(rnd()*float64(len(specials)))]
			s = append(s[:p:p], append([]byte(ins), s[p:]...)...)
		case 2:
			end := min(len(s), p+int(rnd()*40))
			s = append(s[:end:end], append(append([]byte{}, s[p:end]...), s[end:]...)...)
		default:
			lines := strings.Split(string(s), "\n")
			a, b := int(rnd()*float64(len(lines))), int(rnd()*float64(len(lines)))
			lines[a], lines[b] = lines[b], lines[a]
			s = []byte(strings.Join(lines, "\n"))
		}
	}
	return string(s)
}

// renderNoPanic renders a text and fails the test for a panic or for a
// render that does not end.
func renderNoPanic(t *testing.T, name, text string) {
	t.Helper()
	type outcome struct{ panicked any }
	done := make(chan outcome, 1)
	go func() {
		var o outcome
		defer func() {
			o.panicked = recover()
			done <- o
		}()
		_, _ = render.Render(text, render.Options{})
	}()
	select {
	case o := <-done:
		if o.panicked != nil {
			t.Fatalf("%s: panic: %v\ninput:\n%s", name, o.panicked, text)
		}
	case <-time.After(60 * time.Second):
		t.Fatalf("%s: the render does not end\ninput:\n%s", name, text)
	}
}

// TestRenderNoPanic feeds the corpus fixtures, the inputs of the db golden
// file and seeded mutations of both to the whole pipeline. The render must
// end with an SVG or with an error: no panic and no hang.
func TestRenderNoPanic(t *testing.T) {
	var inputs []string
	root := filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", "c4")
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".mmd") {
			if data, err := os.ReadFile(path); err == nil {
				inputs = append(inputs, string(data))
			}
		}
		return nil
	})
	if len(inputs) == 0 {
		t.Fatal("no corpus fixtures")
	}
	for _, c := range loadDBCases(t) {
		inputs = append(inputs, c.Text)
	}
	for _, c := range loadRenderCases(t) {
		inputs = append(inputs, c.Text)
	}
	mutants := 1
	if testing.Short() {
		mutants = 0
	}
	rnd := mulberry32(0xc4c4)
	n := 0
	for _, text := range inputs {
		renderNoPanic(t, "input", text)
		n++
		for m := 0; m < mutants; m++ {
			renderNoPanic(t, "mutant", mutate(rnd, text))
			n++
		}
	}
	t.Logf("%d renders", n)
}
