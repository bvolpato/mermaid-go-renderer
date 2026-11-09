// Ported from mermaid 12.2.0, rendering-elements/shapes/multiRect.ts (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("multiRect", multiRect)
}

func multiRect(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 16, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 12, nodePadding)
	rectOffset := tern(node.Look == "neo", 10, 5)

	// When the node has a width or a height, take the padding from them:
	// the label helper does not account for the padding.
	if truthy(node.Width) || truthy(node.Height) {
		node.Width = js.Max(ru.NN(node.Width, 0)-float64(labelPaddingX*2)-float64(2*rectOffset), 10)
		node.Height = js.Max(ru.NN(node.Height, 0)-float64(labelPaddingY*2)-float64(2*rectOffset), 10)
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	totalWidth := ru.Or(node.Width, bbox.Width) + float64(labelPaddingX*2) + float64(2*rectOffset)
	totalHeight := ru.Or(node.Height, bbox.Height) + float64(labelPaddingY*2) + float64(2*rectOffset)

	w := totalWidth - float64(2*rectOffset)
	h := totalHeight - float64(2*rectOffset)

	x := float64(-w / 2)
	y := float64(-h / 2)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, roughOptions())

	outerPathPoints := []ru.Point{
		{X: x - rectOffset, Y: y + rectOffset},
		{X: x - rectOffset, Y: y + h + rectOffset},
		{X: x + w - rectOffset, Y: y + h + rectOffset},
		{X: x + w - rectOffset, Y: y + h},
		{X: x + w, Y: y + h},
		{X: x + w, Y: y + h - rectOffset},
		{X: x + w + rectOffset, Y: y + h - rectOffset},
		{X: x + w + rectOffset, Y: y - rectOffset},
		{X: x + rectOffset, Y: y - rectOffset},
		{X: x + rectOffset, Y: y},
		{X: x, Y: y},
		{X: x, Y: y + rectOffset},
	}

	innerPathPoints := []ru.Point{
		{X: x, Y: y + rectOffset},
		{X: x + w - rectOffset, Y: y + rectOffset},
		{X: x + w - rectOffset, Y: y + h},
		{X: x + w, Y: y + h},
		{X: x + w, Y: y},
		{X: x, Y: y},
	}

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	outerPath := ru.CreatePathFromPoints(outerPathPoints)
	outerNode, err := rc.Path(outerPath, options)
	if err != nil {
		return nil, err
	}
	innerPath := ru.CreatePathFromPoints(innerPathPoints)
	innerNode, err := rc.Path(innerPath, options)
	if err != nil {
		return nil, err
	}

	if node.Look != "handDrawn" {
		outerNode = ru.MergePaths(ctx.Doc, outerNode)
		innerNode = ru.MergePaths(ctx.Doc, innerNode)
	}

	multiRect := shapeSvg.Insert("g", ":first-child")
	multiRect.InsertNode(outerNode)
	multiRect.InsertNode(innerNode)

	multiRect.Attr("class", "basic label-container outer-path")

	stylePaths(multiRect.SelectAll("path"), node, nodeStyles)

	label.Attr(
		"transform",
		"translate("+js.Str(-float64(bbox.Width/2)-rectOffset-(bbox.X-bbox.LeftOr0()))+", "+
			js.Str(-float64(bbox.Height/2)+rectOffset-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, multiRect)

	node.Intersect = polygonIntersect(node, outerPathPoints)

	return shapeSvg, nil
}
