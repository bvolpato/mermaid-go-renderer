package browser

import (
	"encoding/json"
	"os"
	"testing"
)

// TestContainerUnionAgainstChromium replays the probe cases of
// tools/reference/gen-browser-union-probe.mjs: the bounding box of a
// container whose children have boxes with many digits, with and without
// transforms. The expected values are what Chromium measured. Every value
// must be the same float32: gfx::RectF::UnionEvenIfEmpty widens the union
// to the next float32 when left + width is less than the right edge.
func TestContainerUnionAgainstChromium(t *testing.T) {
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
	readJSON("union-cases.json", &cases)
	readJSON("union-results.json", &results)
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	exact := 0
	for i, c := range cases.Cases {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		for qi, q := range c.Queries {
			var got any
			if q.All {
				vals := []any{}
				for _, el := range container.SelectAll(q.Sel) {
					vals = append(vals, runQuery(el, q))
				}
				got = vals
			} else {
				got = runQuery(container.Select(q.Sel), q)
			}
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
