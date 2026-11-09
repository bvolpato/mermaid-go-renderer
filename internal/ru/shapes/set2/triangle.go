// Ported from mermaid 12.2.0, rendering-elements/shapes/triangle.ts (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

func init() {
	ru.RegisterShape("triangle", triangle)
}

const (
	triangleMinHeight = 10
	triangleMinWidth  = 10
)

func triangle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", float64(nodePadding*2), nodePadding)
	if truthy(node.Width) || truthy(node.Height) {
		node.Width = (ru.NN(node.Width, 0) - labelPaddingX) / 2
		if node.Width < triangleMinWidth {
			node.Width = triangleMinWidth
		}
		node.Height = ru.NN(node.Height, 0)
		if node.Height < triangleMinHeight {
			node.Height = triangleMinHeight
		}
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label
	// This is evaluate of diagrams/common/common.ts on flowchart.htmlLabels.
	// The label helper reads the root htmlLabels.
	useHtmlLabels := sanitize.Evaluate(ctx.Cfg("flowchart", "htmlLabels"))
	w := ru.Or(node.Width, bbox.Width) + labelPaddingX
	h := tern(truthy(node.Height), node.Height, w+bbox.Height)
	tw := h
	points := []ru.Point{
		{X: 0, Y: 0},
		{X: tw, Y: 0},
		{X: tw / 2, Y: -h},
	}

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())
	pathData := ru.CreatePathFromPoints(points)
	roughNode, err := rc.Path(pathData, options)
	if err != nil {
		return nil, err
	}

	polygon := shapeSvg.
		InsertNode(roughNode, ":first-child").
		Attr("transform", "translate("+js.Str(-h/2)+", "+js.Str(h/2)+")").
		Attr("class", "outer-path")

	stylePaths(childPaths(polygon), node, nodeStyles)

	node.Width = w
	node.Height = h

	ru.UpdateNodeBounds(node, polygon)

	paddingDivisor := tern(useHtmlLabels, 2, 1)
	label.Attr(
		"transform",
		"translate("+js.Str(float64(-bbox.Width/2)-(bbox.X-bbox.LeftOr0()))+", "+
			js.Str(float64(h/2)-(bbox.Height+ru.NN(node.Padding, 0)/paddingDivisor-(bbox.Y-bbox.TopOr0())))+")",
	)

	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
