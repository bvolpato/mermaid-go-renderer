// Ported from mermaid 12.2.0, rendering-elements/shapes/text.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("text", text)
}

func text(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	totalWidth := js.Max(bbox.Width+ru.NN(node.Padding, 0), ru.Or(node.Width, 0))
	totalHeight := js.Max(bbox.Height+ru.NN(node.Padding, 0), ru.Or(node.Height, 0))
	x := -totalWidth / 2
	y := -totalHeight / 2

	rect := shapeSvg.Insert("rect", ":first-child")

	rect.
		Attr("class", "text").
		Attr("style", styles.NodeStyles).
		Attr("rx", 0).
		Attr("ry", 0).
		Attr("x", x).
		Attr("y", y).
		Attr("width", totalWidth).
		Attr("height", totalHeight)

	ru.UpdateNodeBounds(node, rect)

	rectIntersect(node)

	return shapeSvg, nil
}
