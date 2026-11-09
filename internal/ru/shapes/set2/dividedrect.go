// Ported from mermaid 12.2.0, rendering-elements/shapes/dividedRect.ts (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("dividedRectangle", dividedRectangle)
}

func dividedRectangle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles

	paddingX := tern(node.Look == "neo", 16, ru.NN(node.Padding, 0))
	paddingY := tern(node.Look == "neo", 16, ru.NN(node.Padding, 0))
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label
	w := bbox.Width + paddingX
	h := bbox.Height + paddingY
	rectOffset := float64(h * 0.2)

	x := float64(-w / 2)
	y := float64(-h/2) - float64(rectOffset/2)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	pts := []ru.Point{
		{X: x, Y: y + rectOffset},
		{X: -x, Y: y + rectOffset},
		{X: -x, Y: -y},
		{X: x, Y: -y},
		{X: x, Y: y},
		{X: -x, Y: y},
		{X: -x, Y: y + rectOffset},
	}

	poly, err := rc.Polygon(roughPoints(pts), options)
	if err != nil {
		return nil, err
	}

	polygon := shapeSvg.InsertNode(poly, ":first-child")
	polygon.Attr("class", "basic label-container outer-path")

	stylePaths(polygon.SelectAll("path"), node, nodeStyles)

	label.Attr(
		"transform",
		"translate("+js.Str(x+float64(ru.NN(node.Padding, 0)/2)-(bbox.X-bbox.LeftOr0()))+", "+
			js.Str(y+rectOffset+float64(ru.NN(node.Padding, 0)/2)-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, polygon)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}

	return shapeSvg, nil
}
