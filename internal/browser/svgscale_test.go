package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

type svgScaleCase struct {
	Name    string            `json:"name"`
	HTML    string            `json:"html"`
	Queries []probeQuery      `json:"queries"`
	Results []json.RawMessage `json:"results"`
}

// TestSVGScaleAgainstChromium replays the probe cases of
// tools/reference/gen-browser-svgscale.mjs: SVG content whose user space is
// not the CSS pixel space of the page. The expected values are what Chromium
// measured.
//
//   - root: a root svg with a viewBox, width="100%" and max-width, as the
//     xychart renderer makes it before it measures text. The values are the
//     box of the svg, getScreenCTM of a group, and the box and the length of
//     text. Chromium lays out the text with a font size that is scaled to
//     the screen, and maps the result back.
//   - rootattr: other ways to size the root svg.
//   - group: text and shapes below a group with a transform.
//   - offset: no transform. getBoundingClientRect adds the offset of the
//     page to each corner in float32, so that the size can change in the
//     last bit.
func TestSVGScaleAgainstChromium(t *testing.T) {
	macOSProbe(t)
	data, err := os.ReadFile("testdata/svgscale.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []svgScaleCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
	filter := os.Getenv("MMDG_PROBE_FILTER")
	type stat struct{ exact, close, diff int }
	stats := map[string]*stat{}
	for _, c := range cases {
		if filter != "" && !strings.Contains(c.Name, filter) {
			continue
		}
		kind := c.Name[:strings.IndexByte(c.Name, '-')]
		doc := NewDocument()
		engine := doc.Engine.(*Engine)
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		for qi, q := range c.Queries {
			key := kind + "/" + q.Op
			st := stats[key]
			if st == nil {
				st = &stat{}
				stats[key] = st
			}
			el := container.Select(q.Sel)
			var got any
			if q.Op == "ctm" {
				got = screenCTMQuery(engine, el)
			} else {
				got = runQuery(el, q)
			}
			// The values are compared bit for bit.
			result := compare(got, c.Results[qi], 0.01)
			switch result {
			case "exact":
				st.exact++
				continue
			case "close":
				st.close++
			default:
				st.diff++
			}
			gotJSON, _ := json.Marshal(got)
			message := fmt.Sprintf("%s %s(%s): %s\n got  %s\n want %s", c.Name, q.Op, q.Sel, result, gotJSON, c.Results[qi])
			if reason, known := svgScaleKnown[c.Name]; known {
				t.Logf("known divergence (%s): %s", reason, message)
			} else {
				t.Errorf("%s\n %s", message, c.HTML)
			}
		}
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := stats[k]
		t.Logf("%-18s exact=%4d close=%4d diff=%4d", k, s.exact, s.close, s.diff)
	}
}

// svgScaleKnown lists the cases that the engine does not reproduce, with
// the cause.
var svgScaleKnown = map[string]string{
	// The font has the size 9.98 px. The sum of the glyph advances of the
	// engine is 1/65536 px above a multiple of 1/64 px, and the sum of
	// Chromium is not above it, so that the width is 1/64 px smaller in
	// Chromium. The cause is in the advances of the font at this size.
	"root-785x500-10": "advance sum at a 1/64 px boundary",
}

func screenCTMQuery(e *Engine, el *dom.Node) any {
	if el == nil {
		return nil
	}
	m, ok := e.ScreenCTM(el)
	if !ok {
		return nil
	}
	return m[:]
}
