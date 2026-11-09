// Ported from mermaid 12.2.0, rendering-elements/shapes/lightningBolt.ts
// (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("lightningBolt", lightningBolt)
}

func lightningBolt(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.Label, node.HasLabel = "", true
	node.LabelStyle = labelStyles
	shapeSvg := insertShapeGroup(parent, node)
	width := js.Max(35, ru.NN(node.Width, 0))
	height := js.Max(35, ru.NN(node.Height, 0))
	const gap = 7.0

	points := []ru.Point{
		{X: width, Y: 0},
		{X: 0, Y: height + gap/2},
		{X: width - 2*gap, Y: height + gap/2},
		{X: 0, Y: float64(2 * height)},
		{X: width, Y: height - gap/2},
		{X: 2 * gap, Y: height - gap/2},
	}

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	linePath := ru.CreatePathFromPoints(points)
	lineNode, err := rc.Path(linePath, options)
	if err != nil {
		return nil, err
	}

	lightningBolt := shapeSvg.InsertNode(lineNode, ":first-child")
	lightningBolt.Attr("class", "outer-path")

	stylePaths(lightningBolt.SelectAll("path"), node, nodeStyles)

	lightningBolt.Attr("transform", "translate(-"+js.Str(width/2)+","+js.Str(-height)+")")

	ru.UpdateNodeBounds(node, lightningBolt)

	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
