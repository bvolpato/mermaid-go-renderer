// Ported from mermaid 12.2.0, rendering-elements/shapes/anchor.ts (MIT).

package set3

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("anchor", anchor)
}

func anchor(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	node.LabelStyle = ru.Styles2String(node).LabelStyles
	// getNodeClasses never gives an empty string, so the fallback class
	// 'anchor' of upstream is never used.
	cssClasses := ru.GetNodeClasses(node)
	shapeSvg := parent.
		Insert("g").
		Attr("class", cssClasses).
		Attr("id", nodeID(node))

	const radius = 1.0

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, roughOptions("fill", "black", "stroke", "none", "fillStyle", "solid"))

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
	}
	roughNode, err := rc.Circle(0, 0, radius*2, options)
	if err != nil {
		return nil, err
	}
	circleElem := shapeSvg.InsertNode(roughNode, ":first-child")
	circleElem.Attr("class", "anchor").Attr("style", ru.CSSStylesAttr(node))

	ru.UpdateNodeBounds(node, circleElem)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectCircle(ru.NodeBounds(node), radius, point)
	}

	return shapeSvg, nil
}
