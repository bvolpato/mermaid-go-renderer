// Ported from mermaid 12.2.0, rendering-elements/shapes/stateEnd.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("stateEnd", stateEnd)
}

func stateEnd(ctx *render.Context, parent *dom.Node, node *ru.Node, options *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles
	lineColor := themeVariable(options, "lineColor")
	stateBorder := themeVariable(options, "stateBorder")
	nodeBorder := themeVariable(options, "nodeBorder")
	nodeShadow := themeVariable(options, "nodeShadow")

	stateCircleSize(node)

	shapeSvg := parent.
		Insert("g").
		Attr("class", "node default").
		Attr("id", nodeID(node))

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	roughOptions := ru.UserNodeOverrides(ctx, node, nil)

	if node.Look != "handDrawn" {
		roughOptions.Set("roughness", 0.0)
		roughOptions.Set("fillStyle", "solid")
	}

	outerOptions := spread(roughOptions)
	outerOptions.Set("stroke", lineColor)
	outerOptions.Set("strokeWidth", 2.0)
	roughNode, err := rc.Circle(0, 0, node.Width, outerOptions)
	if err != nil {
		return nil, err
	}
	innerFill := stateBorder
	if js.IsNullish(innerFill) {
		innerFill = nodeBorder
	}
	innerNodeRadius := float64(ru.NN(node.Width, 0)*5) / 14
	innerOptions := spread(roughOptions)
	innerOptions.Set("fill", innerFill)
	innerOptions.Set("stroke", innerFill)
	innerOptions.Set("strokeWidth", 2.0)
	innerOptions.Set("fillStyle", "solid")
	roughInnerNode, err := rc.Circle(0, 0, innerNodeRadius, innerOptions)
	if err != nil {
		return nil, err
	}
	circle := shapeSvg.InsertNode(roughNode, ":first-child")
	circle.InsertNode(roughInnerNode)

	if node.Look != "handDrawn" {
		circle.Attr("class", "outer-path")
	}

	if hasCSSStyles(node) {
		circle.SelectAll("path").Attr("style", cssStylesString(node))
	}

	if styles.NodeStyles != "" {
		circle.SelectAll("path").Attr("style", styles.NodeStyles)
	}

	if node.Width < 25 && js.Truthy(nodeShadow) && node.Look != "handDrawn" {
		circle.Attr("style", smallDropShadow(parent))
	}

	ru.UpdateNodeBounds(node, circle)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectCircle(ru.NodeBounds(node), ru.NN(node.Width, 0)/2, point)
	}

	return shapeSvg, nil
}
