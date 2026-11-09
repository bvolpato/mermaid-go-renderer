package mmdc

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// root is the svg element that mermaid makes for a diagram of the given
// size, without content.
func root(width, height float64, useMaxWidth bool) string {
	size := fmt.Sprintf(`width="100%%" style="max-width: %gpx;"`, width)
	if !useMaxWidth {
		size = fmt.Sprintf(`width="%g" height="%g"`, width, height)
	}
	return fmt.Sprintf(`<svg id="my-svg" %s xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %g %g"><g></g></svg>`, size, width, height)
}

// TestScreenshotSize compares the size of the screenshot with the size of
// the PNG files that real mmdc 12.0.0 wrote for diagrams of these sizes
// (cmd/mmdg/testdata/mmdc/png.json). The svg elements here have no text, so
// the test needs no fonts.
func TestScreenshotSize(t *testing.T) {
	for _, c := range []struct {
		name          string
		width, height float64
		useMaxWidth   bool
		size          int
		// wantW and wantH are the size of the PNG of mmdc in CSS px.
		wantW, wantH int
		// boxW is the width of the diagram in the image.
		boxW float64
	}{
		{"small", 358, 70, true, 0, 358, 70, 350},
		{"tall", 168, 504, true, 0, 168, 504, 160},
		{"pie", 547, 450, true, 0, 547, 450, 539},
		// The page is 800 px wide and the body has margins of 8 px.
		{"wide", 1896, 64, true, 0, 784, 27, 776},
		{"fixed width", 1896, 64, false, 0, 1896, 64, 1896},
		{"small with --size 300", 358, 70, true, 300, 300, 59, 292},
		{"wide with --size 300", 1896, 64, true, 300, 300, 11, 292},
		{"tall with --size 300", 168, 504, true, 300, 100, 300, 92},
		{"pie with --size 300", 547, 450, true, 300, 300, 247, 292},
	} {
		t.Run(c.name, func(t *testing.T) {
			page, err := Load(root(c.width, c.height, c.useMaxWidth), Options{Background: "white", SetBackground: true, Size: c.size, Fonts: font.Embedded()})
			if err != nil {
				t.Fatal(err)
			}
			viewport := float64(DefaultViewportWidth)
			if c.size > 0 {
				viewport = float64(c.size + 16)
			}
			shot, err := page.Screenshot(viewport)
			if err != nil {
				t.Fatal(err)
			}
			if shot.Width != c.wantW || shot.Height != c.wantH {
				t.Errorf("image %dx%d, mmdc has %dx%d", shot.Width, shot.Height, c.wantW, c.wantH)
			}
			if shot.Box.Width != c.boxW || shot.Box.X != 0 {
				t.Errorf("diagram at x %g with width %g, want 0 and %g", shot.Box.X, shot.Box.Width, c.boxW)
			}
			if !strings.Contains(shot.SVG, fmt.Sprintf(`width="%g"`, c.boxW)) || strings.Contains(shot.SVG, "max-width") || strings.Contains(shot.SVG, "background-color") {
				t.Errorf("the svg for the rasterizer has not the size of the box: %.200s", shot.SVG)
			}
		})
	}
}

func TestScreenshotBackground(t *testing.T) {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for _, c := range []struct {
		background    string
		transparent   bool
		page, diagram color.NRGBA
	}{
		{"white", false, white, white},
		{"red", false, color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, A: 255}},
		{"transparent", true, color.NRGBA{}, color.NRGBA{}},
		// Only the exact word gives an image without a background.
		{"rgba(0,0,0,0)", false, white, white},
		// The page and the svg element both paint a color with alpha.
		{"rgba(255,0,0,0.5)", false, color.NRGBA{R: 255, G: 128, B: 128, A: 255}, color.NRGBA{R: 255, G: 64, B: 64, A: 255}},
		// The body takes no hex color without the #. The svg element does.
		{"F0F0F0", false, white, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"notacolor", false, white, white},
	} {
		transparent, page, diagram := backgroundColors(Options{Background: c.background, SetBackground: true})
		// The rounding of a mix of two colors was not compared with mmdc.
		near := func(a, b color.NRGBA) bool {
			d := func(x, y uint8) bool { return x-y <= 1 || y-x <= 1 }
			return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && a.A == b.A
		}
		if transparent != c.transparent || !near(page, c.page) || !near(diagram, c.diagram) {
			t.Errorf("%q: transparent %v, page %v, diagram %v, want %v, %v, %v", c.background, transparent, page, diagram, c.transparent, c.page, c.diagram)
		}
	}
}

func TestTitleAndDesc(t *testing.T) {
	for _, c := range []struct{ svg, title, desc string }{
		{`<svg><title id="t">The title</title><desc id="d">The <b>description</b></desc><g></g></svg>`, "The title", "The description"},
		{`<svg><desc>first</desc><title>not the first child</title><desc>last</desc></svg>`, "", "last"},
		{`<svg> <title>after a text node</title></svg>`, "", ""},
		{`<svg><g><title>deep</title><desc>deep</desc></g></svg>`, "", ""},
	} {
		page, err := Load(c.svg, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if page.Title() != c.title || page.Desc() != c.desc {
			t.Errorf("%s: title %q, desc %q, want %q, %q", c.svg, page.Title(), page.Desc(), c.title, c.desc)
		}
	}
	if _, err := Load("<p>no diagram</p>", Options{}); err == nil {
		t.Error("markup without an svg element must be an error")
	}
}
