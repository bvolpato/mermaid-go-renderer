// Ported from mermaid 12.2.0, rendering-elements/shapes/trapezoidalPentagon.ts
// (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("trapezoidalPentagon", trapezoidalPentagon)
}

func trapezoidalPentagon(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 16, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 12, nodePadding)
	const minWidth, minHeight = 15.0, 5.0
	if truthy(node.Width) || truthy(node.Height) {
		node.Height = ru.NN(node.Height, 0) - float64(labelPaddingY*2)
		if node.Height < minHeight {
			node.Height = minHeight
		}

		node.Width = ru.NN(node.Width, 0) - float64(labelPaddingX*2)
		if node.Width < minWidth {
			node.Width = minWidth
		}
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox
	w := ru.Or(node.Width, bbox.Width) + float64(labelPaddingX*2)
	h := ru.Or(node.Height, bbox.Height) + float64(labelPaddingY*2)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	points := []ru.Point{
		{X: (-w / 2) * 0.8, Y: -h / 2},
		{X: (w / 2) * 0.8, Y: -h / 2},
		{X: w / 2, Y: (-h / 2) * 0.6},
		{X: w / 2, Y: h / 2},
		{X: -w / 2, Y: h / 2},
		{X: -w / 2, Y: (-h / 2) * 0.6},
	}

	pathData := ru.CreatePathFromPoints(points)
	shapeNode, err := rc.Path(pathData, options)
	if err != nil {
		return nil, err
	}

	polygon := shapeSvg.InsertNode(shapeNode, ":first-child")
	polygon.Attr("class", "basic label-container outer-path")

	stylePaths(childPaths(polygon), node, nodeStyles)

	ru.UpdateNodeBounds(node, polygon)

	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
