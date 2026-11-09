package browser

import (
	"encoding/json"
	"os"
	"testing"
)

// TestCSSVarAgainstChromium replays the probe cases of
// tools/reference/gen-browser-cssvar-probe.mjs: the CSS function var() with
// custom properties that are defined, not defined, chained or cyclic, with
// and without a fallback. The expected values are what Chromium measured.
// A reference without a value makes the declaration invalid at
// computed-value time: the property then inherits, or takes its initial
// value.
func TestCSSVarAgainstChromium(t *testing.T) {
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
	readJSON("cssvar-cases.json", &cases)
	readJSON("cssvar-results.json", &results)
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	exact, closeTo := 0, 0
	for i, c := range cases.Cases {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		for qi, q := range c.Queries {
			got := runQuery(container.Select(q.Sel), q)
			want := results[i].Results[qi]
			// The tolerance is the one of the parity command.
			switch compare(got, want, 0.01) {
			case "exact":
				exact++
			case "close":
				closeTo++
			default:
				gotJSON, _ := json.Marshal(got)
				t.Errorf("%s %s(%s)\n got  %s\n want %s\n %s", c.Name, q.Op, q.Sel, gotJSON, want, c.HTML)
			}
		}
	}
	t.Logf("exact=%d close(<=0.01)=%d", exact, closeTo)
}
