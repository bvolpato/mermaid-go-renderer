package browser

import (
	"encoding/json"
	"os"
	"testing"
)

// TestRootRatioAgainstChromium replays the probe cases of
// tools/reference/gen-browser-rootratio-probe.mjs: a root svg whose height
// comes from the ratio of its viewBox, for viewBox sizes with decimals. The
// expected values are what Chromium measured: the box of the svg, and the
// box and the length of a text in it, which depend on the scale of the svg.
// Every value must be the same float32.
//
// Blink keeps the ratio as two LayoutUnits (LayoutRatioFromSizeF): the
// sizes of the viewBox when both are multiples of 1/64, and else a
// continued fraction of width / height.
func TestRootRatioAgainstChromium(t *testing.T) {
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
	readJSON("rootratio-cases.json", &cases)
	readJSON("rootratio-results.json", &results)
	if len(cases.Cases) < 200 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	exact := 0
	for i, c := range cases.Cases {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		for qi, q := range c.Queries {
			got := runQuery(container.Select(q.Sel), q)
			want := results[i].Results[qi]
			if compare(got, want, 0) == "exact" {
				exact++
				continue
			}
			gotJSON, _ := json.Marshal(got)
			var compact any
			_ = json.Unmarshal(want, &compact)
			wantJSON, _ := json.Marshal(compact)
			t.Errorf("%s %s(%s)\n got  %s\n want %s\n %s", c.Name, q.Op, q.Sel, gotJSON, wantJSON, c.HTML)
		}
	}
	t.Logf("exact=%d", exact)
}
