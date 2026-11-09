package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// TestImagesAgainstChromium replays the probe cases of
// tools/reference/gen-browser-image-probe.mjs: the rectangles of the HTML
// of a label with img elements. The rectangles are read two times, as
// Chromium read them: while each image is pending, where an image is 0 x 0,
// and after CompleteImages, where an image has its natural size, or the
// 16 x 16 of the broken image icon when it cannot load. The images are data
// URLs, which the engine reads itself. Every value must be equal to the
// value of Chromium.
//
// The engine has no pending state for a data URL, because the load of such
// an image ends before mermaid measures a label. The probe reads the
// rectangles at once, when the image is still pending. So for the first
// read the test gives each image a source that is not a data URL: the
// layout of a pending image does not depend on its source.
func TestImagesAgainstChromium(t *testing.T) {
	macOSProbe(t)
	var cases struct{ Cases []probeCase }
	var results []struct {
		Name   string      `json:"name"`
		Before [][]float64 `json:"before"`
		After  [][]float64 `json:"after"`
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
	readJSON("image-cases.json", &cases)
	readJSON("image-results.json", &results)
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	rects := func(container *dom.Node) [][]float64 {
		out := [][]float64{}
		for _, el := range container.SelectAll("#fo *") {
			b := el.BoundingClientRect()
			out = append(out, []float64{b.X, b.Y, b.Width, b.Height})
		}
		return out
	}
	exact := 0
	check := func(name, state, html string, got, want [][]float64) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) == string(wantJSON) {
			exact++
			return
		}
		t.Errorf("%s, %s\n got  %s\n want %s\n %s", name, state, gotJSON, wantJSON, html)
	}
	for i, c := range cases.Cases {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		images := container.SelectAll("img")
		sources := make([]string, len(images))
		for k, img := range images {
			sources[k] = img.AttrOr("src", "")
			if sources[k] != "" {
				img.Attr("src", fmt.Sprintf("pending:%d", k))
			}
		}
		check(c.Name, "pending", c.HTML, rects(container), results[i].Before)
		for k, img := range images {
			if sources[k] != "" {
				img.Attr("src", sources[k])
			}
		}
		doc.Engine.(*Engine).CompleteImages(container)
		check(c.Name, "complete", c.HTML, rects(container), results[i].After)
	}
	t.Logf("exact=%d of %d", exact, 2*len(cases.Cases))
}
