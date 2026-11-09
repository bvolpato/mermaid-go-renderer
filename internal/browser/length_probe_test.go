package browser

import (
	"encoding/json"
	"os"
	"testing"
)

// TestLengthNumberAgainstChromium replays the probe cases of
// tools/reference/gen-browser-length-probe.mjs: the bounding box of an
// element whose length attributes have numbers that are not float32 values.
// The expected values are what Chromium measured. Every checked value must
// be the same float32.
//
// Blink reads a plain decimal number of a length attribute with the fast
// path of its CSS parser, which keeps 7 digits of the fraction. A number
// with a plus sign or an exponent, and a length of a style declaration, are
// rounded as a whole.
func TestLengthNumberAgainstChromium(t *testing.T) {
	var cases struct {
		Cases []struct {
			probeCase
			Check []int `json:"check"`
		}
	}
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
	readJSON("length-cases.json", &cases)
	readJSON("length-results.json", &results)
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	exact := 0
	for i, c := range cases.Cases {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		got, _ := runQuery(container.Select(c.Queries[0].Sel), c.Queries[0]).([]float64)
		var want []float64
		if err := json.Unmarshal(results[i].Results[0], &want); err != nil || len(got) != 4 || len(want) != 4 {
			t.Errorf("%s: no bounding box: got %v, want %s", c.Name, got, results[i].Results[0])
			continue
		}
		ok := true
		for _, k := range c.Check {
			if got[k] != want[k] {
				ok = false
			}
		}
		if ok {
			exact++
			continue
		}
		t.Errorf("%s (values %v)\n got  %v\n want %v\n %s", c.Name, c.Check, got, want, c.HTML)
	}
	t.Logf("exact=%d of %d", exact, len(cases.Cases))
}
