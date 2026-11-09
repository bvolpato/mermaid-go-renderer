package dom

import (
	"encoding/json"
	"os"
	"testing"
)

// TestTypeSelectorAgainstChromium replays the probe cases of
// tools/reference/gen-dom-type-selector-probe.mjs: type selectors in every
// case on SVG and HTML elements, with querySelectorAll, matches and closest.
// The expected values are what Chromium returned.
func TestTypeSelectorAgainstChromium(t *testing.T) {
	var cases struct {
		Cases []struct {
			Name   string     `json:"name"`
			HTML   string     `json:"html"`
			Target string     `json:"target"`
			Ops    [][]string `json:"ops"`
		} `json:"cases"`
	}
	var results []struct {
		Name    string `json:"name"`
		Results []any  `json:"results"`
	}
	readJSON := func(name string, v any) {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, v); err != nil {
			t.Fatal(err)
		}
	}
	readJSON("type-selector-cases.json", &cases)
	readJSON("type-selector-results.json", &results)
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	for i, c := range cases.Cases {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		el := container.Select(c.Target)
		if el == nil {
			t.Fatalf("%s: no target %q", c.Name, c.Target)
		}
		for k, op := range c.Ops {
			var got any
			switch op[0] {
			case "query":
				got = float64(len(container.SelectAll(op[1])))
			case "matches":
				got = el.Matches(op[1])
			case "closest":
				got = el.Closest(op[1]) != nil
			default:
				t.Fatalf("%s: unknown operation %q", c.Name, op[0])
			}
			if want := results[i].Results[k]; got != want {
				t.Errorf("%s: %s(%q) = %v, Chromium has %v", c.Name, op[0], op[1], got, want)
			}
		}
	}
}
