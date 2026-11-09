// Ported from mermaid 12.2.0, rendering-elements/shapes/card.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("card", card)
}

// cardNotchSize is the size of the notch on the top left corner.
const cardNotchSize = 12

func card(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX, labelPaddingY := nodePadding, nodePadding
	if node.Look == "neo" {
		labelPaddingX, labelPaddingY = 28, 24
	}
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	extraW := labelPaddingX + cardNotchSize
	extraH := labelPaddingY
	if node.Look == "neo" {
		extraW = float64(labelPaddingX * 2)
		extraH = float64(labelPaddingY * 2)
	}
	w := ru.NN(node.Width, bbox.Width) + extraW
	h := ru.NN(node.Height, bbox.Height) + extraH
	const left = 0.0
	right := w
	top := -h
	const bottom = 0.0
	points := []ru.Point{
		{X: left + cardNotchSize, Y: top},
		{X: right, Y: top},
		{X: right, Y: bottom},
		{X: left, Y: bottom},
		{X: left, Y: top + cardNotchSize},
		{X: left + cardNotchSize, Y: top},
	}

	var polygon *dom.Node

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, nil)
		pathData := ru.CreatePathFromPoints(points)
		roughNode, err := rc.Path(pathData, options)
		if err != nil {
			return nil, err
		}

		polygon = shapeSvg.
			InsertNode(roughNode, ":first-child").
			Attr("transform", "translate("+js.Str(-w/2)+", "+js.Str(h/2)+")")

		if hasCSSStyles(node) {
			polygon.Attr("style", cssStylesString(node))
		}
	} else {
		polygon = ru.InsertPolygonShape(shapeSvg, w, h, points)
	}

	if styles.NodeStyles != "" {
		polygon.Attr("style", styles.NodeStyles)
	}

	ru.UpdateNodeBounds(node, polygon)

	polygonIntersect(node, points)

	return shapeSvg, nil
}
