package browser

import (
	"encoding/json"
	"os"
	"testing"
)

// TestPathLengthAgainstChromium replays the probe cases of
// tools/reference/gen-browser-pathlength-probe.mjs: getTotalLength and
// getPointAtLength of paths as the edges of mermaid have them. The expected
// values are what Chromium measured. Every value must be the same float32.
//
// The length of a line is (end - start).Length() of gfx::Vector2dF, which
// computes the squares and their sum in double precision. The center of an
// arc is computed in double precision from delta.LengthSquared(). With
// float32 arithmetic in these two places, one length in seven of the lines
// and curves differs, and nearly every arc of a line hop whose chord is
// shorter than its diameter.
func TestPathLengthAgainstChromium(t *testing.T) {
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
	readJSON("pathlength-cases.json", &cases)
	readJSON("pathlength-results.json", &results)
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	exact, lengths, lengthsExact := 0, 0, 0
	for i, c := range cases.Cases {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		for qi, q := range c.Queries {
			got := runQuery(container.Select(q.Sel), q)
			want := results[i].Results[qi]
			ok := compare(got, want, 0) == "exact"
			if q.Op == "len" {
				lengths++
				if ok {
					lengthsExact++
				}
			}
			if ok {
				exact++
				continue
			}
			gotJSON, _ := json.Marshal(got)
			var compact any
			_ = json.Unmarshal(want, &compact)
			wantJSON, _ := json.Marshal(compact)
			t.Errorf("%s %s(%s)\n got  %s\n want %s\n %s", c.Name, q.Op, q.Arg, gotJSON, wantJSON, c.HTML)
		}
	}
	t.Logf("exact=%d, lengths: %d of %d exact", exact, lengthsExact, lengths)
}
