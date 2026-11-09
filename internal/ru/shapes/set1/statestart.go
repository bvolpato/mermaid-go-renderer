// Ported from mermaid 12.2.0, rendering-elements/shapes/stateStart.ts (MIT).

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("stateStart", stateStart)
}

// stateCircleSize sets the size of a start or end state: at least 14, and
// 14 when the node has no size.
func stateCircleSize(node *ru.Node) {
	if js.Truthy(node.Width) || js.Truthy(node.Height) {
		if ru.NN(node.Width, 0) < 14 {
			node.Width = 14
		}

		if ru.NN(node.Height, 0) < 14 {
			node.Height = 14
		}
	}

	if !js.Truthy(node.Width) {
		node.Width = 14
	}

	if !js.Truthy(node.Height) {
		node.Height = 14
	}
}

// smallDropShadow is the style that a small state circle gets when the
// theme has node shadows.
func smallDropShadow(parent *dom.Node) string {
	svgID := ownerSVGElementID(parent)
	filterID := "drop-shadow-small"
	if svgID != "" {
		filterID = svgID + "-drop-shadow-small"
	}
	return "filter:url(#" + filterID + ")"
}

func stateStart(ctx *render.Context, parent *dom.Node, node *ru.Node, options *ru.ShapeRenderOptions) (*dom.Node, error) {
	lineColor := themeVariable(options, "lineColor")
	nodeShadow := themeVariable(options, "nodeShadow")

	stateCircleSize(node)

	shapeSvg := parent.
		Insert("g").
		Attr("class", "node default").
		Attr("id", nodeID(node))

	var circle *dom.Node
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		roughNode, err := rc.Circle(0, 0, node.Width, ru.SolidStateFill(ctx, lineColor))
		if err != nil {
			return nil, err
		}
		circle = shapeSvg.InsertNode(roughNode)
		// Center the circle around its coordinate.
		circle.
			Attr("class", "state-start").
			Attr("r", ru.NN(node.Width, 7)/2).
			Attr("width", ru.NN(node.Width, 14)).
			Attr("height", ru.NN(node.Height, 14))
	} else {
		circle = shapeSvg.Insert("circle", ":first-child")
		// Center the circle around its coordinate.
		circle.
			Attr("class", "state-start").
			Attr("r", ru.NN(node.Width, 7)/2).
			Attr("width", ru.NN(node.Width, 14)).
			Attr("height", ru.NN(node.Height, 14))
	}

	if node.Width < 25 && js.Truthy(nodeShadow) && node.Look != "handDrawn" {
		circle.Attr("style", smallDropShadow(parent))
	}

	ru.UpdateNodeBounds(node, circle)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectCircle(ru.NodeBounds(node), ru.NN(node.Width, 7)/2, point)
	}

	return shapeSvg, nil
}
