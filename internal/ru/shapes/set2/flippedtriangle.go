// Ported from mermaid 12.2.0, rendering-elements/shapes/flippedTriangle.ts
// (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("flippedTriangle", flippedTriangle)
}

const (
	flippedTriangleMinHeight = 10
	flippedTriangleMinWidth  = 10
)

func flippedTriangle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles

	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", float64(nodePadding*2), nodePadding)
	if truthy(node.Width) || truthy(node.Height) {
		node.Height = ru.NN(node.Height, 0)
		if node.Height < flippedTriangleMinHeight {
			node.Height = flippedTriangleMinHeight
		}

		node.Width = ru.NN(node.Width, 0) - labelPaddingX - float64(labelPaddingX/2)
		if node.Width < flippedTriangleMinWidth {
			node.Width = flippedTriangleMinWidth
		}
	}
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	w := ru.Or(node.Width, bbox.Width) + labelPaddingX
	h := tern(truthy(node.Height), node.Height, w+bbox.Height)

	tw := h

	points := []ru.Point{
		{X: 0, Y: -h},
		{X: tw, Y: -h},
		{X: tw / 2, Y: 0},
	}

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())
	pathData := ru.CreatePathFromPoints(points)
	roughNode, err := rc.Path(pathData, options)
	if err != nil {
		return nil, err
	}

	flippedTriangle := shapeSvg.
		InsertNode(roughNode, ":first-child").
		Attr("transform", "translate("+js.Str(-h/2)+", "+js.Str(h/2)+")").
		Attr("class", "outer-path")

	stylePaths(childPaths(flippedTriangle), node, nodeStyles)

	node.Width = w
	node.Height = h

	ru.UpdateNodeBounds(node, flippedTriangle)

	label.Attr(
		"transform",
		"translate("+js.Str(float64(-bbox.Width/2)-(bbox.X-bbox.LeftOr0()))+", "+
			js.Str(float64(-h/2)+float64(ru.NN(node.Padding, 0)/2)+(bbox.Y-bbox.TopOr0()))+")",
	)

	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
