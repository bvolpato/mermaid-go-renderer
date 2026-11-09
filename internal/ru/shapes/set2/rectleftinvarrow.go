// Ported from mermaid 12.2.0, rendering-elements/shapes/rectLeftInvArrow.ts
// (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("rect_left_inv_arrow", rectLeftInvArrow)
}

// rectLeftInvArrow is rect_left_inv_arrow of rectLeftInvArrow.ts.
func rectLeftInvArrow(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 21, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 12, nodePadding)
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	labelWidth := bbox.Width + tern(node.Look == "neo", float64(labelPaddingX*2), labelPaddingX)
	h := js.Max(
		bbox.Height+tern(node.Look == "neo", float64(labelPaddingY*2), labelPaddingY),
		ru.NN(node.Height, 0),
	)
	notchWidth := float64(h / 4)
	w := js.Max(labelWidth, ru.NN(node.Width, 0)-notchWidth)

	x := float64(-w / 2)
	y := float64(-h / 2)
	notch := float64(y / 2)

	points := []ru.Point{
		{X: x + notch, Y: y},
		{X: x, Y: 0},
		{X: x + notch, Y: -y},
		{X: -x, Y: -y},
		{X: -x, Y: y},
	}

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	pathData := ru.CreatePathFromPoints(points)
	roughNode, err := rc.Path(pathData, options)
	if err != nil {
		return nil, err
	}

	polygon := shapeSvg.InsertNode(roughNode, ":first-child")

	polygon.Attr("class", "basic label-container outer-path")

	stylePaths(polygon.SelectAll("path"), node, nodeStyles)

	polygon.Attr("transform", "translate("+js.Str(-notch/2)+",0)")

	label.Attr(
		"transform",
		"translate("+js.Str(float64(-notch/2)-float64(bbox.Width/2)-(bbox.X-bbox.LeftOr0()))+", "+
			js.Str(-float64(bbox.Height/2)-(bbox.Y-bbox.TopOr0()))+")",
	)
	ru.UpdateNodeBounds(node, polygon)

	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
