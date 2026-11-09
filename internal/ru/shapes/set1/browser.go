// Ported from mermaid 12.2.0, rendering-elements/shapes/browser.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("browser", browser)
}

// browser is the browser shape: a rounded window with a chrome bar of dots
// and an address-bar hint.
func browser(ctx *render.Context, parent *dom.Node, node *ru.Node, options *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	accent := accentColor(options)
	padding := ru.NN(node.Padding, 12)
	const barHeight = 18
	const radius = 12
	w := js.Max(bbox.Width+float64(padding*2), ru.NN(node.Width, 0), 90)
	h := js.Max(bbox.Height+float64(padding*2)+barHeight, ru.NN(node.Height, 0))
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
		Append("line").
		Attr("x1", -w/2).
		Attr("y1", top+barHeight).
		Attr("x2", w/2).
		Attr("y2", top+barHeight).
		Attr("style", "stroke:"+accent+";stroke-width:1px")

	for i := 0; i < 3; i++ {
		group.
			Append("circle").
			Attr("cx", float64(-w/2)+12+float64(i*9)).
			Attr("cy", top+barHeight/2).
			Attr("r", 2.5).
			Attr("style", "fill:"+accent+";stroke:none")
	}

	group.
		Append("rect").
		Attr("class", "browser-address-bar").
		Attr("x", float64(-w/2)+44).
		Attr("y", top+4).
		Attr("width", js.Max(w-56, 10)).
		Attr("height", barHeight-8).
		Attr("rx", 3).
		Attr("ry", 3).
		Attr("style", "fill:none;stroke:"+accent+";stroke-width:1px;opacity:0.6")

	ru.UpdateNodeBounds(node, group)

	bodyCenterY := top + barHeight + float64((h-barHeight)/2)
	label.Attr("transform", "translate("+js.Str(labelX(bbox))+", "+
		js.Str(bodyCenterY-float64(bbox.Height/2)-(bbox.Y-bbox.TopOr0()))+")")

	rectIntersect(node)

	return shapeSvg, nil
}
