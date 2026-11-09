// Ported from mermaid 12.2.0, diagrams/class/classRenderer-v3-unified.ts
// (MIT).

package class

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// getClasses is getClasses(text, diagramObj): diagramObj.db.getClasses().
// The result is the map of the classes of the diagram, and not the map of
// the classDef statements. mermaidAPI reads the id and the styles of each
// entry, so a class with styles gets CSS rules for `.<class id>`. A class
// node has no textStyles.
func getClasses(d *DB) *js.Map[string, *render.ClassDef] {
	out := js.NewMap[string, *render.ClassDef]()
	d.GetClasses().Each(func(key string, c *ClassNode) {
		out.Set(key, &render.ClassDef{ID: c.ID, Styles: c.Styles})
	})
	return out
}

// draw is draw(text, id, _version, diag).
func draw(ctx *render.Context, d *DB) error {
	config := ctx.Config
	// const { securityLevel, state: conf, layout } = getConfig();
	conf := js.AsObj(config.Get("state"))
	layout := ""
	if v := config.Get("layout"); !js.IsNullish(v) {
		layout = js.ToString(v)
	}

	// Set the diagram ID for DOM element uniqueness across multiple diagrams
	d.SetDiagramId(ctx.ID)

	// The getData method provided in all supported diagrams is used to extract the data from the parsed structure
	// into the Layout data format
	data4Layout := d.GetData()
	if d.err != nil {
		return d.err
	}

	// Create the root SVG - the element is the div containing the SVG element
	svg := ctx.GetDiagramElement()

	data4Layout.Type = ctx.Type
	data4Layout.LayoutAlgorithm = ru.GetRegisteredLayoutAlgorithm(layout, "")

	// conf?.nodeSpacing || 50
	spacing := func(key string) float64 {
		if conf != nil {
			if v := conf.Get(key); js.Truthy(v) {
				return js.ToNumber(v)
			}
		}
		return 50
	}
	data4Layout.NodeSpacing = spacing("nodeSpacing")
	data4Layout.RankSpacing = spacing("rankSpacing")
	data4Layout.Markers = []string{"aggregation", "extension", "composition", "dependency", "lollipop"}
	data4Layout.DiagramID = ctx.ID
	if err := ru.Render(ctx, data4Layout, svg); err != nil {
		return err
	}
	const padding = 8
	// conf?.titleTopMargin ?? 25
	titleTopMargin := 25.0
	if conf != nil {
		if v := conf.Get("titleTopMargin"); !js.IsNullish(v) {
			titleTopMargin = js.ToNumber(v)
		}
	}
	render.InsertTitle(svg, "classDiagramTitleText", titleTopMargin, d.GetDiagramTitle())

	// conf?.useMaxWidth ?? true
	useMaxWidth := true
	if conf != nil {
		if v := conf.Get("useMaxWidth"); !js.IsNullish(v) {
			useMaxWidth = js.Truthy(v)
		}
	}
	render.SetupViewPortForSVG(svg, padding, "classDiagram", useMaxWidth)
	return nil
}
