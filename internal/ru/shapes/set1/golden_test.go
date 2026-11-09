package set1

import (
	"path/filepath"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// TestShapesAgainstMermaid replays the golden files of this package: cases
// recorded from the real insertNode of mermaid in Chromium.
func TestShapesAgainstMermaid(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.json*")
	rutest.RunShapeGolden(t, files...)
}

// TestShapeRandomDraws checks that a shape draws as many random numbers as
// upstream does. A shape that roughjs draws with roughness 0 calls
// Math.random, but its markup does not depend on the numbers, so the markup
// test does not see a wrong number of calls. The numbers that a later part
// of the diagram draws depend on it.
func TestShapeRandomDraws(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.json*")
	skipped := 0
	for _, f := range files {
		for _, c := range rutest.LoadShapeCases(t, f) {
			if c.Error != "" {
				continue
			}
			cfgVal, err := js.ParseJSON(c.Config)
			if err != nil {
				t.Fatalf("%s: %v", c.Name, err)
			}
			cfg := js.AsObj(cfgVal)
			nodeVal, err := js.ParseJSON(c.Node)
			if err != nil {
				t.Fatalf("%s: %v", c.Name, err)
			}
			// The same page as rutest.RunShapeCase builds.
			ctx := render.NewContext(render.ContextOptions{Config: cfg})
			ctx.Type = c.Type
			ctx.SVG.RemoveChildren()
			style := ctx.Doc.CreateElement("style")
			style.Text(c.CSS)
			ctx.SVG.AppendChild(style)
			g := ctx.SVG.Append("g").Attr("class", "nodes")
			node := ru.NodeFromJS(js.AsObj(nodeVal))
			if _, err := ru.InsertNode(ctx, g, node, &ru.ShapeRenderOptions{Config: cfg, Dir: c.Dir}); err != nil {
				t.Errorf("%s: %v", c.Name, err)
				continue
			}
			if rutest.SkipComparison(c.HTML) {
				// A shape with the hand-drawn look draws numbers for
				// each line of its fill, and the size of the text gives
				// the number of lines.
				skipped++
				continue
			}
			// The next number of the generator tells how many were drawn.
			ref := render.NewContext(render.ContextOptions{Config: cfg})
			for i := 0; i < c.Random; i++ {
				ref.Random()
			}
			if got, want := ctx.Random(), ref.Random(); got != want {
				t.Errorf("%s: the shape did not draw %d random numbers, as upstream does", c.Name, c.Random)
			}
		}
	}
	rutest.RequireFonts(t, skipped)
}
