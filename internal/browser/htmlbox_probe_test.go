package browser

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestHTMLBoxAgainstChromium replays the probe cases of
// tools/reference/gen-browser-htmlbox-probe.mjs: the rectangles of the HTML
// in a foreignObject, for the structures that the labels of mermaid have.
//
// The cases have six styles of the div. The first three are the styles of
// createText (a table-cell that does not wrap, a table that wraps, and the
// table-cell with no line height). Their values must be equal to the
// values of Chromium, but for one text: CJK characters in a div with no
// font family, where the default serif font of the page needs a fallback
// font that the engine does not choose as Chromium does.
//
// The other three styles are not styles of mermaid, and the engine does not
// lay them out fully: a div of display: inline-block is a block here and
// not a box on a line, and the margins of a p do not collapse through a
// plain block. The test keeps the number of their differences from growing.
func TestHTMLBoxAgainstChromium(t *testing.T) {
	macOSProbe(t)
	var cases struct{ Cases []probeCase }
	var results []probeResult
	readJSON := func(name string, v any) {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, v); err != nil {
			t.Fatal(err)
		}
	}
	readJSON("htmlbox-cases.json", &cases)
	readJSON("htmlbox-results.json", &results)
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	// known is the number of cases of each of the other styles that differ.
	known := map[string]int{"s3": 26, "s4": 17, "s5": 17}
	exact := 0
	differ := map[string]int{}
	for i, c := range cases.Cases {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		vals := []any{}
		for _, el := range container.SelectAll(c.Queries[0].Sel) {
			vals = append(vals, runQuery(el, c.Queries[0]))
		}
		if compare(vals, results[i].Results[0], 0) == "exact" {
			exact++
			continue
		}
		style := strings.Split(c.Name, "-")[1]
		if _, other := known[style]; other {
			differ[style]++
			continue
		}
		if strings.Contains(c.HTML, "你好") && !strings.Contains(c.HTML, "font-family") {
			continue
		}
		got, _ := json.Marshal(vals)
		var compact any
		_ = json.Unmarshal(results[i].Results[0], &compact)
		want, _ := json.Marshal(compact)
		t.Errorf("%s\n got  %s\n want %s\n %s", c.Name, got, want, c.HTML)
	}
	for style, limit := range known {
		if differ[style] > limit {
			t.Errorf("style %s: %d cases differ, more than the %d that are known", style, differ[style], limit)
		}
	}
	t.Logf("%d cases: exact=%d, other styles that differ: %v", len(cases.Cases), exact, differ)
}
