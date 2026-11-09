// Ported from mermaid 12.2.0, rendering-elements/shapes/subroutine.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("subroutine", subroutine)
}

// subroutineFrameWidth is the width of the frame on the left and on the
// right side of the shape.
const subroutineFrameWidth = 8

func subroutine(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	nodePadding := ru.NN(node.Padding, 8)
	labelPaddingX, labelPaddingY := nodePadding, nodePadding
	if node.Look == "neo" {
		labelPaddingX, labelPaddingY = 28, 12
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	totalWidth := js.Max(bbox.Width+2*subroutineFrameWidth+labelPaddingX, ru.NN(node.Width, 0))
	totalHeight := js.Max(bbox.Height+labelPaddingY, ru.NN(node.Height, 0))

	w := totalWidth - 2*subroutineFrameWidth
	h := totalHeight
	x := float64(-totalWidth / 2)
	y := float64(-totalHeight / 2)

	points := []ru.Point{
		{X: 0, Y: 0},
		{X: w, Y: 0},
		{X: w, Y: -h},
		{X: 0, Y: -h},
		{X: 0, Y: 0},
		{X: -8, Y: 0},
		{X: w + 8, Y: 0},
		{X: w + 8, Y: -h},
		{X: -8, Y: -h},
		{X: -8, Y: 0},
	}

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, nil)

		roughNode, err := rc.Rectangle(x, y, w+16, h, options)
		if err != nil {
			return nil, err
		}
		l1 := rc.Line(x+subroutineFrameWidth, y, x+subroutineFrameWidth, y+h, options)
		l2 := rc.Line(x+subroutineFrameWidth+w, y, x+subroutineFrameWidth+w, y+h, options)

		shapeSvg.InsertNode(l1, ":first-child")
		shapeSvg.InsertNode(l2, ":first-child")
		rect := shapeSvg.InsertNode(roughNode, ":first-child")
		rect.Attr("class", "basic label-container").Attr("style", ru.CSSStylesAttr(node))
		ru.UpdateNodeBounds(node, rect)
	} else {
		el := ru.InsertPolygonShape(shapeSvg, w, h, points)
		if styles.NodeStyles != "" {
			el.Attr("style", styles.NodeStyles)
		}
		ru.UpdateNodeBounds(node, el)
	}

	polygonIntersect(node, points)

	return shapeSvg, nil
}
