// Tests mermaid-cli 12.0.0 src/index.js (MIT) and Puppeteer 25.16.0
// common/PDFOptions.ts (Apache-2.0) against their output in Chromium 156.
package mmdc

import (
	"encoding/json"
	"image/color"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

func TestPrintAgainstMMDC(t *testing.T) {
	data, err := os.ReadFile("testdata/print.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, SVG, Format string
		Size              int
		Viewport          float64
		Width, Height     float64
		Box               dom.Rect
		Pages             int
		Matrix            []float64
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			page, err := Load(c.SVG, Options{Background: "white", SetBackground: true, Size: c.Size, Fonts: font.Embedded()})
			if err != nil {
				t.Fatal(err)
			}
			printed, err := page.Print(c.Viewport, c.Format)
			if err != nil {
				t.Fatal(err)
			}
			if printed.Width != c.Width || printed.Height != c.Height || printed.Pages != c.Pages {
				t.Errorf("paper %g x %g, %d pages; mmdc has %g x %g, %d pages", printed.Width, printed.Height, printed.Pages, c.Width, c.Height, c.Pages)
			}
			// Skia serializes the PDF transform with fewer digits than its
			// float32 values. Unscaled browser layout remains bit-exact.
			tolerance := 0.0
			if len(c.Matrix) > 0 && c.Matrix[0] != 3.125 {
				tolerance = 0.0002
			}
			got := []float64{printed.Box.X, printed.Box.Y, printed.Box.Width, printed.Box.Height}
			want := []float64{c.Box.X, c.Box.Y, c.Box.Width, c.Box.Height}
			for i := range got {
				if math.Abs(got[i]-want[i]) > tolerance {
					t.Errorf("box %v; mmdc has %v (tolerance %g)", printed.Box, c.Box, tolerance)
					break
				}
			}
			// The standalone SVG must use the size that the PDF places.
			standalone, err := Load(printed.SVG, Options{Fonts: font.Embedded()})
			if err != nil {
				t.Fatal(err)
			}
			for name, want := range map[string]float64{"width": printed.Box.Width, "height": printed.Box.Height} {
				got, err := strconv.ParseFloat(standalone.SVG.AttrOr(name, ""), 64)
				if err != nil || got != want {
					t.Errorf("SVG %s = %g (%v), want %g", name, got, err, want)
				}
			}
		})
	}
}

func TestPrintBackground(t *testing.T) {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for _, c := range []struct {
		background    string
		page, diagram color.NRGBA
	}{
		{"white", white, white},
		{"red", color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, A: 255}},
		{"transparent", color.NRGBA{}, color.NRGBA{}},
		{"rgba(0,0,0,0)", white, white},
		{"rgba(255,0,0,0.5)", color.NRGBA{R: 255, G: 127, B: 127, A: 255}, color.NRGBA{R: 255, G: 63, B: 63, A: 255}},
	} {
		t.Run(c.background, func(t *testing.T) {
			page, err := Load(root(100, 100, true), Options{Background: c.background, SetBackground: true, Fonts: font.Embedded()})
			if err != nil {
				t.Fatal(err)
			}
			printed, err := page.Print(0, "")
			if err != nil {
				t.Fatal(err)
			}
			if printed.Page != c.page || printed.Diagram != c.diagram {
				t.Errorf("backgrounds %v and %v, want %v and %v", printed.Page, printed.Diagram, c.page, c.diagram)
			}
		})
	}
}

func TestPrintBounds(t *testing.T) {
	for _, c := range []struct {
		name, svg, format string
		viewport          float64
		size              int
	}{
		{name: "unknown format", format: "A7"},
		{name: "format whitespace", format: " A4"},
		{name: "not a number", viewport: math.NaN()},
		{name: "infinite viewport", viewport: math.Inf(1)},
		{name: "large viewport", viewport: maxPrintDimension + 1},
		{name: "large negative viewport", viewport: -maxPrintDimension - 1},
		{name: "negative size", size: -1},
		{name: "large size", size: maxPrintDimension + 1},
		{name: "large diagram", svg: root(maxPrintDimension+1, 100, false)},
		{name: "large fit paper", svg: root(maxPrintDimension, 100, false)},
		{name: "too many pages", svg: root(100, maxPrintDimension, false), format: "A6"},
		{name: "zero diagram", svg: `<svg width="0" height="0"></svg>`},
	} {
		t.Run(c.name, func(t *testing.T) {
			svg := c.svg
			if svg == "" {
				svg = root(100, 100, true)
			}
			page, err := Load(svg, Options{Size: c.size, Fonts: font.Embedded()})
			if err != nil {
				t.Fatal(err)
			}
			if printed, err := page.Print(c.viewport, c.format); err == nil || printed != nil {
				t.Fatalf("Print returned %v, %v; want an error", printed, err)
			}
		})
	}
	if _, err := (*Page)(nil).Print(0, ""); err == nil {
		t.Error("a nil page must return an error")
	}
	if _, err := (&Page{}).Print(0, ""); err == nil {
		t.Error("an empty page must return an error")
	}
}
