// Ported from mermaid 12.2.0, rendering-elements/shapes/slopedRect.ts (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("slopedRect", slopedRect)
}

func slopedRect(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 16, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 12, nodePadding)

	// When the node has a width or a height, take the padding from them:
	// the label helper does not account for the padding.
	if truthy(node.Width) || truthy(node.Height) {
		node.Width = js.Max(ru.NN(node.Width, 0)-float64(labelPaddingX*2), 10)
		node.Height = js.Max(ru.NN(node.Height, 0)/1.5-float64(labelPaddingY*2), 10)
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	totalWidth := ru.Or(node.Width, bbox.Width) + float64(labelPaddingX*2)
	totalHeight := float64((ru.Or(node.Height, bbox.Height) + float64(labelPaddingY*2)) * 1.5)

	w := totalWidth
	h := totalHeight / 1.5
	x := float64(-w / 2)
	y := float64(-h / 2)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	points := []ru.Point{
		{X: x, Y: y},
		{X: x, Y: y + h},
		{X: x + w, Y: y + h},
		{X: x + w, Y: y - float64(h/2)},
	}

	pathData := ru.CreatePathFromPoints(points)
	shapeNode, err := rc.Path(pathData, options)
	if err != nil {
		return nil, err
	}

	polygon := shapeSvg.InsertNode(shapeNode, ":first-child")
	polygon.Attr("class", "basic label-container  outer-path")

	stylePaths(childPaths(polygon), node, nodeStyles)

	polygon.Attr("transform", "translate(0, "+js.Str(h/4)+")")
	label.Attr(
		"transform",
		"translate("+js.Str(float64(-w/2)+ru.NN(node.Padding, 0)-(bbox.X-bbox.LeftOr0()))+", "+
			js.Str(float64(-h/4)+ru.NN(node.Padding, 0)-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, polygon)

	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
