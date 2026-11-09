// Ported from mermaid 12.2.0, rendering-elements/shapes/hourglass.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("hourglass", hourglass)
}

func hourglass(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.Label, node.HasLabel = "", true
	node.LabelStyle = styles.LabelStyles
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg := res.ShapeSvg

	w := js.Max(30, ru.NN(node.Width, 0))
	h := js.Max(30, ru.NN(node.Height, 0))

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, nil)

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	points := []ru.Point{
		{X: 0, Y: 0},
		{X: w, Y: 0},
		{X: 0, Y: h},
		{X: w, Y: h},
	}

	pathData := ru.CreatePathFromPoints(points)
	shapeNode, err := rc.Path(pathData, options)
	if err != nil {
		return nil, err
	}
	polygon := shapeSvg.InsertNode(shapeNode, ":first-child")
	polygon.Attr("class", "basic label-container outer-path")

	if hasCSSStyles(node) && node.Look != "handDrawn" {
		selectChildren(polygon, "path").Attr("style", cssStylesString(node))
	}

	if styles.NodeStyles != "" && node.Look != "handDrawn" {
		selectChildren(polygon, "path").Attr("style", styles.NodeStyles)
	}

	polygon.Attr("transform", "translate("+js.Str(-w/2)+", "+js.Str(-h/2)+")")

	ru.UpdateNodeBounds(node, polygon)

	polygonIntersect(node, points)

	return shapeSvg, nil
}
