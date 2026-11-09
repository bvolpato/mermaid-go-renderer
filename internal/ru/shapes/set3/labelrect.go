// Ported from mermaid 12.2.0, rendering-elements/shapes/labelRect.ts (MIT).
// The roundedRect function of that file is not in the shape map of shapes.ts:
// the roundedRect shape is rendering-elements/shapes/roundedRect.ts.

package set3

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("labelRect", labelRect)
}

func labelRect(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	res, err := labelHelper(ctx, parent, node, "label")
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	// Add the rect.
	rect := shapeSvg.Insert("rect", ":first-child")

	// Hide the rect: only the label is wanted.
	const totalWidth = 0.1
	const totalHeight = 0.1
	rect.Attr("width", totalWidth).Attr("height", totalHeight)
	shapeSvg.Attr("class", "label edgeLabel")
	label.Attr(
		"transform",
		"translate("+js.Str(-float64(bbox.Width/2)-(bbox.X-bbox.LeftOr0()))+", "+js.Str(-float64(bbox.Height/2)-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, rect)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}

	return shapeSvg, nil
}
