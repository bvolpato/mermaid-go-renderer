package shape

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// TestPieAgainstChromium runs the geometry of the pie renderer of mermaid
// (pieRenderer.ts) and compares the slice paths and the label positions with
// the SVG that Chromium rendered. The label positions are arc centroids
// without rounding, so that they show the last bit of Math.cos and Math.sin.
// The data comes from the reference renders
// (tools/reference/gen-d3-chromium-trig.mjs).
func TestPieAgainstChromium(t *testing.T) {
	data, err := os.ReadFile("testdata/chromium_pie.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Pies []struct {
			File          string
			Values        []float64
			TextPosition  float64
			InnerHole     float64
			Paths, Labels []string
		}
	}
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Pies) < 20 {
		t.Fatalf("only %d pie charts", len(g.Pies))
	}
	slices := 0
	for _, tc := range g.Pies {
		sum := 0.0
		for _, v := range tc.Values {
			sum += v
		}
		// Remove values < 1%.
		var pieData []float64
		for _, v := range tc.Values {
			if v/sum*100 >= 1 {
				pieData = append(pieData, v)
			}
		}
		const radius = 185.0
		arcGenerator := NewArc[PieArcDatum[float64]]().InnerRadius(float64(tc.InnerHole * radius)).OuterRadius(radius)
		labelArcGenerator := NewArc[PieArcDatum[float64]]().
			InnerRadius(float64(radius * tc.TextPosition)).OuterRadius(float64(radius * tc.TextPosition))
		i := 0
		for _, datum := range NewPie[float64]().Sort(nil).Arcs(pieData) {
			// Filter out arcs that would render as 0%.
			if js.ToFixed(float64(datum.Data/sum*100), 0) == "0" {
				continue
			}
			if i >= len(tc.Paths) {
				t.Errorf("%s: more slices than Chromium has", tc.File)
				break
			}
			if got := arcGenerator.Path(datum); got != tc.Paths[i] {
				t.Errorf("%s slice %d: path %q, Chromium has %q", tc.File, i, got, tc.Paths[i])
			}
			x, y := labelArcGenerator.Centroid(datum)
			if got := js.Str(x) + "," + js.Str(y); got != tc.Labels[i] {
				t.Errorf("%s slice %d: label at %q, Chromium has %q", tc.File, i, got, tc.Labels[i])
			}
			i++
		}
		if i != len(tc.Paths) {
			t.Errorf("%s: %d slices, Chromium has %d", tc.File, i, len(tc.Paths))
		}
		slices += i
	}
	t.Logf("%d pie charts, %d slices: every path and label position is the one of Chromium", len(g.Pies), slices)
}
