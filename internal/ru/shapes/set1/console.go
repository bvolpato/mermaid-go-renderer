// Ported from mermaid 12.2.0, rendering-elements/shapes/console.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("consoleWindow", consoleWindow)
}

// consoleWindow is the console shape: a rounded terminal window with a
// command prompt glyph in the top-left corner.
func consoleWindow(ctx *render.Context, parent *dom.Node, node *ru.Node, options *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	accent := accentColor(options)
	padding := ru.NN(node.Padding, 12)
	const glyphBand = 20
	const radius = 12
	w := js.Max(bbox.Width+float64(padding*2), ru.NN(node.Width, 0), 90)
	h := js.Max(bbox.Height+float64(padding*2)+glyphBand, ru.NN(node.Height, 0))
	top := float64(-h / 2)

	group := shapeSvg.Insert("g", ":first-child").Attr("class", "basic label-container")

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		roughNode, err := rc.Path(
			ru.CreateRoundedRectPathD(-w/2, top, w, h, radius),
			ru.UserNodeOverrides(ctx, node, nil),
		)
		if err != nil {
			return nil, err
		}
		group.AppendChild(roughNode)
		if hasCSSStyles(node) {
			group.Attr("style", cssStylesString(node))
		}
	} else {
		group.
			Append("rect").
			Attr("x", -w/2).
			Attr("y", top).
			Attr("width", w).
			Attr("height", h).
			Attr("rx", radius).
			Attr("ry", radius).
			Attr("style", styles.NodeStyles)
	}

	group.
		Append("text").
		Attr("x", float64(-w/2)+12).
		Attr("y", top+16).
		Attr("class", "console-glyph").
		Attr("style", "font-family:monospace;font-weight:bold;font-size:14px;fill:"+accent).
		Text(">_")

	ru.UpdateNodeBounds(node, group)

	bodyCenterY := top + glyphBand + float64((h-glyphBand)/2)
	label.Attr("transform", "translate("+js.Str(labelX(bbox))+", "+
		js.Str(bodyCenterY-float64(bbox.Height/2)-(bbox.Y-bbox.TopOr0()))+")")

	rectIntersect(node)

	return shapeSvg, nil
}
