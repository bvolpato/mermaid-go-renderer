// Ported from mermaid 12.2.0, diagrams/requirement/requirementRenderer.ts
// (MIT).

package requirement

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// layoutName is the argument of getRegisteredLayoutAlgorithm for the
// `layout` of the config. Its default parameter is the empty string.
func layoutName(layout any) string {
	if layout == nil {
		return ""
	}
	return js.ToString(layout)
}

// spacing is `conf?.<key> ?? 50`.
func spacing(conf *js.Obj[any], key string) float64 {
	if v := js.Path(conf, key); !js.IsNullish(v) {
		return js.ToNumber(v)
	}
	return 50
}

// layoutData is the part of draw that builds the layout data: getData()
// and the properties that the renderer adds.
func layoutData(ctx *render.Context, d *DB) *ru.LayoutData {
	// const { securityLevel, state: conf, layout, look } = getConfig();
	conf := js.AsObj(ctx.Cfg("state"))
	layout := ctx.Cfg("layout")
	look := ctx.Cfg("look")

	data4Layout := d.GetData()

	data4Layout.Type = ctx.Type
	data4Layout.LayoutAlgorithm = ru.GetRegisteredLayoutAlgorithm(layoutName(layout), "")

	data4Layout.NodeSpacing = spacing(conf, "nodeSpacing")
	data4Layout.RankSpacing = spacing(conf, "rankSpacing")
	if js.StrictEquals(look, "neo") {
		data4Layout.Markers = []string{"requirement_contains_neo", "requirement_arrow_neo"}
	} else {
		data4Layout.Markers = []string{"requirement_contains", "requirement_arrow"}
	}
	data4Layout.DiagramID = ctx.ID
	return data4Layout
}

// draw is draw(text, id, _version, diag).
func draw(ctx *render.Context, d *DB) error {
	conf := js.AsObj(ctx.Cfg("state"))

	data4Layout := layoutData(ctx, d)

	// Create the root SVG - the element is the div containing the SVG element
	svg := ctx.GetDiagramElement()

	if err := ru.Render(ctx, data4Layout, svg); err != nil {
		return err
	}
	const padding = 8
	// conf?.titleTopMargin ?? 25
	titleTopMargin := 25.0
	if v := js.Path(conf, "titleTopMargin"); !js.IsNullish(v) {
		titleTopMargin = js.ToNumber(v)
	}
	render.InsertTitle(svg, "requirementDiagramTitleText", titleTopMargin, d.GetDiagramTitle())

	// conf?.useMaxWidth ?? true
	useMaxWidth := true
	if v := js.Path(conf, "useMaxWidth"); !js.IsNullish(v) {
		useMaxWidth = js.Truthy(v)
	}
	render.SetupViewPortForSVG(svg, padding, "requirementDiagram", useMaxWidth)
	return nil
}
