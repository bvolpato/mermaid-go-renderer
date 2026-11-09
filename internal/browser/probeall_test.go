package browser

import (
	"encoding/json"
	"os"
	"testing"
)

// replayProbeAll runs the cases of a DOM probe through the engine, like
// replayProbe, and also handles a query for all elements that match a
// selector. It returns the number of values that are equal to the values
// of Chromium, that are within 0.01 of them, and that differ. With report,
// each value that is not equal is an error of the test.
func replayProbeAll(t *testing.T, casesFile, resultsFile string, report bool) (exact, closeTo, diff int) {
	t.Helper()
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
	readJSON(casesFile, &cases)
	readJSON(resultsFile, &results)
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
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
			switch compare(got, want, 0.01) {
			case "exact":
				exact++
				continue
			case "close":
				closeTo++
			default:
				diff++
			}
			if report {
				gotJSON, _ := json.Marshal(got)
				var compact any
				_ = json.Unmarshal(want, &compact)
				wantJSON, _ := json.Marshal(compact)
				t.Errorf("%s %s(%s)\n got  %s\n want %s\n %s", c.Name, q.Op, q.Sel, gotJSON, wantJSON, c.HTML)
			}
		}
	}
	return exact, closeTo, diff
}
