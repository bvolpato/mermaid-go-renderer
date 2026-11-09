package set2

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// TestShapesRandomDraws checks that a shape takes as many numbers from
// Math.random as upstream does. The golden files record the number of
// draws of each case. The markup of one node does not show a draw too many
// at the end, or a draw whose value has no effect, but the next node of a
// diagram would get other numbers.
//
// The test replays a case as rutest.RunShapeCase does. Then the next number
// of the generator must be the number that follows the draws of upstream.
func TestShapesRandomDraws(t *testing.T) {
	checked, skipped := 0, 0
	for _, shape := range shapeFiles(t) {
		for _, c := range rutest.LoadShapeCases(t, "testdata/"+shape+".json.gz") {
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
			got := ctx.Random()
			want := render.NewContext(render.ContextOptions{Config: cfg, RandomSkip: c.Random}).Random()
			if got != want {
				t.Errorf("%s: the shape did not take %d random numbers, as upstream does", c.Name, c.Random)
			}
			checked++
		}
	}
	t.Logf("%d cases", checked)
	rutest.RequireFonts(t, skipped)
}
