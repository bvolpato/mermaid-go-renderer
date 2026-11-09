// Package mindmap is the mindmap diagram: the diagram type mindmap.
//
// Ported from mermaid 12.2.0, diagrams/mindmap/mindmap-definition.ts,
// detector.ts and mindmapRenderer.ts (MIT). The other files of the package
// port mindmapDb.ts, mindmapTypes.ts and the semantic actions of
// parser/mindmap.jison.
//
// The diagram is drawn by the unified renderer (internal/ru) with the layout
// cose-bilkent. The edges, the markers, the shapes and the layouts register
// themselves there: a program must link those packages too
// (internal/ru/edges, internal/ru/clusters, internal/ru/shapes/set1 and
// internal/ru/layout/cosebilkent, and the other layouts for a mindmap whose
// config asks for one of them).
package mindmap

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	render.Register(&render.Definition{
		ID:  "mindmap",
		New: func(ctx *render.Context) render.DB { return NewDB(ctx) },
		Parse: func(_ *render.Context, db render.DB, text string) error {
			return parseText(db.(*DB), text)
		},
		Draw: func(ctx *render.Context, db render.DB, _ string) error {
			return draw(ctx, db.(*DB))
		},
	})
}

// layoutName is the argument of getRegisteredLayoutAlgorithm for the
// `layout` of the config. Its default parameter is the empty string.
func layoutName(layout any) string {
	if layout == nil {
		return ""
	}
	return js.ToString(layout)
}

// layoutData is the part of draw that builds the layout data: getData() and
// the properties that the renderer adds. The result is nil when the mindmap
// has no node: draw then returns before it draws anything.
func layoutData(ctx *render.Context, d *DB) *ru.LayoutData {
	// The getData method provided in all supported diagrams is used to extract the data from the parsed structure
	// into the Layout data format
	data4Layout := d.GetData()

	data4Layout.Type = ctx.Type
	data4Layout.LayoutAlgorithm = ru.GetRegisteredLayoutAlgorithm(layoutName(data4Layout.Config.Get("layout")), "cose-bilkent")

	data4Layout.DiagramID = ctx.ID

	mm := d.GetMindmap()
	if mm == nil {
		return nil
	}

	for _, node := range data4Layout.Nodes {
		switch node.Shape {
		case "rounded":
			node.Radius = 15
			node.Taper = 15
			node.Stroke = "none"
			node.Width = 0
			node.Padding = 15
		case "circle":
			node.Padding = 10
		case "rect":
			node.Width = 0
			node.Padding = 10
		case "hexagon":
			node.Width = 0
			node.Height = 0
		}
	}
	return data4Layout
}

// draw is draw(text, id, _version, diagObj) of mindmapRenderer.ts.
func draw(ctx *render.Context, d *DB) error {
	registerLocalShapes()

	// Create the root SVG - the element is the div containing the SVG element
	svg := ctx.GetDiagramElement()

	data4Layout := layoutData(ctx, d)
	if data4Layout == nil {
		return nil
	}

	// Use the unified rendering system
	if err := ru.Render(ctx, data4Layout, svg); err != nil {
		return err
	}

	// For mindmap, the linearGradient used by the neo theme must live inside <defs>
	// so browsers treat it as a valid SVG paint server (url(#id-gradient) references).
	// render.ts appends it to the SVG root for other diagram types; mindmap adds its
	// own properly-scoped <defs> entry here to avoid relying on that placement.
	themeVariables := js.AsObj(ctx.Cfg("themeVariables"))
	if themeVariables == nil {
		return &Error{Name: "TypeError", Message: "Cannot destructure property 'useGradient' of 'themeVariables' as it is undefined."}
	}
	useGradient := themeVariables.Get("useGradient")
	gradientStart := themeVariables.Get("gradientStart")
	gradientStop := themeVariables.Get("gradientStop")
	if js.Truthy(useGradient) && js.Truthy(gradientStart) && js.Truthy(gradientStop) {
		svgID := svg.AttrOr("id", "null")
		gradient := svg.
			Append("defs").
			Append("linearGradient").
			Attr("id", svgID+"-gradient").
			Attr("gradientUnits", "objectBoundingBox").
			Attr("x1", "0%").
			Attr("y1", "0%").
			Attr("x2", "100%").
			Attr("y2", "0%")

		gradient.
			Append("stop").
			Attr("offset", "0%").
			Attr("stop-color", js.ToString(gradientStart)).
			Attr("stop-opacity", 1)
		gradient.
			Append("stop").
			Attr("offset", "100%").
			Attr("stop-color", js.ToString(gradientStop)).
			Attr("stop-opacity", 1)
	}

	// Setup the view box and size of the svg element using config from data4Layout
	render.SetupViewPortForSVG(
		svg,
		js.ToNumber(mindmapConfig(ctx, data4Layout.Config, "padding")),
		"mindmapDiagram",
		js.Truthy(mindmapConfig(ctx, data4Layout.Config, "useMaxWidth")),
	)
	return nil
}
