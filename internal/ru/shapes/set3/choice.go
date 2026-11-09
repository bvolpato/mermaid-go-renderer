// Ported from mermaid 12.2.0, rendering-elements/shapes/choice.ts (MIT).

package set3

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("choice", choice)
}

func choice(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	nodeStyles := ru.Styles2String(node).NodeStyles
	node.Label, node.HasLabel = "", true
	shapeSvg := parent.
		Insert("g").
		Attr("class", ru.GetNodeClasses(node)).
		Attr("id", nodeID(node))

	s := js.Max(28, ru.NN(node.Width, 0))

	points := []ru.Point{
		{X: 0, Y: s / 2},
		{X: s / 2, Y: 0},
		{X: 0, Y: -s / 2},
		{X: -s / 2, Y: 0},
	}

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, roughOptions())

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	choicePath := ru.CreatePathFromPoints(points)
	roughNode, err := rc.Path(choicePath, options)
	if err != nil {
		return nil, err
	}
	choiceShape := shapeSvg.InsertNode(roughNode, ":first-child")

	applyPathStyles(choiceShape, node, nodeStyles)

	node.Width = 28
	node.Height = 28

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectPolygon(ru.NodeBounds(node), points, point)
	}

	return shapeSvg, nil
}
