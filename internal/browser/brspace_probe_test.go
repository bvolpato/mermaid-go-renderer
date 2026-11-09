package browser

import (
	"encoding/json"
	"os"
	"testing"
)

// TestBrSpaceAgainstChromium replays the probe cases of
// tools/reference/gen-browser-brspace-probe.mjs: the rectangle of a label
// div whose text has a collapsible space in front of a br element. The
// expected values are what Chromium measured. A collapsible space at the
// end of a line is removed, and a br element ends the line: the space does
// not count for the width.
func TestBrSpaceAgainstChromium(t *testing.T) {
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
	readJSON("brspace-cases.json", &cases)
	readJSON("brspace-results.json", &results)
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
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
	t.Logf("exact=%d of %d", exact, len(cases.Cases))
}
