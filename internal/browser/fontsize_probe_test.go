package browser

import (
	"encoding/json"
	"os"
	"testing"
)

// replayProbe runs the cases of a DOM probe through the engine and
// requires the values that Chromium returned, within the tolerance of the
// parity command.
func replayProbe(t *testing.T, casesFile, resultsFile string) {
	t.Helper()
	replayProbeWith(t, casesFile, resultsFile, probeOptions{})
}

// probeOptions changes what replayProbeWith requires.
type probeOptions struct {
	// exactOnly requires numbers that are equal to the numbers of Chromium.
	exactOnly bool
	// untested returns the indexes of the numbers of a result that the
	// probe does not test, or nil. The numbers at these indexes are not
	// compared.
	untested func(q probeQuery) []int
}

// dropIndexes removes the numbers at the given indexes from a JSON array
// of numbers.
func dropIndexes(v any, indexes []int) any {
	data, _ := json.Marshal(v)
	var list []any
	if json.Unmarshal(data, &list) != nil {
		return v
	}
	var out []any
	for i, e := range list {
		drop := false
		for _, idx := range indexes {
			drop = drop || idx == i
		}
		if !drop {
			out = append(out, e)
		}
	}
	return out
}

// replayProbeWith is replayProbe with options.
func replayProbeWith(t *testing.T, casesFile, resultsFile string, opts probeOptions) {
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
	exact, closeTo := 0, 0
	for i, c := range cases.Cases {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		for qi, q := range c.Queries {
			got := runQuery(container.Select(q.Sel), q)
			want := results[i].Results[qi]
			if opts.untested != nil {
				if indexes := opts.untested(q); len(indexes) > 0 {
					var wantValue any
					_ = json.Unmarshal(want, &wantValue)
					got = dropIndexes(got, indexes)
					want, _ = json.Marshal(dropIndexes(wantValue, indexes))
				}
			}
			switch compare(got, want, 0.01) {
			case "exact":
				exact++
			case "close":
				closeTo++
				if !opts.exactOnly {
					continue
				}
				fallthrough
			default:
				gotJSON, _ := json.Marshal(got)
				var compact any
				_ = json.Unmarshal(want, &compact)
				wantJSON, _ := json.Marshal(compact)
				t.Errorf("%s %s(%s)\n got  %s\n want %s\n %s", c.Name, q.Op, q.Sel, gotJSON, wantJSON, c.HTML)
			}
		}
	}
	t.Logf("exact=%d close(<=0.01)=%d", exact, closeTo)
}

// TestExUnitAgainstChromium replays the probe cases of
// tools/reference/gen-browser-ex-probe.mjs: lengths in the CSS unit ex, in
// font-size and in other properties and attributes. The ex of an element is
// the x-height of its primary font, and the ex of font-size is the x-height
// of the font of the parent.
func TestExUnitAgainstChromium(t *testing.T) {
	replayProbe(t, "ex-cases.json", "ex-results.json")
}

// TestFontSizePrecisionAgainstChromium replays the probe cases of
// tools/reference/gen-browser-fontsize-probe.mjs: font sizes with more than
// two decimals. Blink shapes text with the font size cut to 1/100 px, and
// prints the computed font size with 6 significant digits.
//
// The probe does not test the height of the box of the inline span #s. The
// engine gives an inline element the height of its line box, and Chromium
// gives it the ascent plus the descent of its font. The two differ for a
// font that has a line gap, such as Arial.
func TestFontSizePrecisionAgainstChromium(t *testing.T) {
	replayProbeWith(t, "fontsize-cases.json", "fontsize-results.json", probeOptions{
		untested: func(q probeQuery) []int {
			if q.Op == "rect" && q.Sel == "#s" {
				return []int{3}
			}
			return nil
		},
	})
}

// TestPathBoundsAgainstChromium replays the probe cases of
// tools/reference/gen-browser-pathbounds-probe.mjs: the bounding boxes of
// lines, polylines, polygons and paths with decimal coordinates, alone and
// united with other elements. The values must be equal to the float32
// values of Chromium: Blink converts the bounds of a path to a rectangle
// with gfx::RectF::SetByBounds, which can add one float32 step to the width
// or the height.
func TestPathBoundsAgainstChromium(t *testing.T) {
	replayProbeWith(t, "pathbounds-cases.json", "pathbounds-results.json", probeOptions{exactOnly: true})
}
