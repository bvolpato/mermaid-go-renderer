// Ported from mermaid 12.2.0, rendering-elements/shapes/circle.ts (MIT).

package set1

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("circle", circle)
}

// circle draws the circle shape. The padding of the options is the padding
// that mindmapCircle passes. An undefined padding is NaN. A padding of 0 is
// also taken as undefined, because the zero value of ShapeRenderOptions has
// it: mindmapCircle passes `node.padding ?? 0`, and half of that padding is
// then 0 too, so the result is the same.
func circle(ctx *render.Context, parent *dom.Node, node *ru.Node, options *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles
	// A min width would inflate the radius on both axes. That leaves short
	// labels in a large, empty circle.
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node), ru.LabelHelperOptions{IgnoreMinWidth: true})
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, halfPadding := res.ShapeSvg, res.BBox, res.HalfPadding

	// Calculate the radius based on the look.
	const labelPadding = 16
	padding := halfPadding
	if options != nil && js.Truthy(options.Padding) {
		padding = options.Padding
	}
	// Half the diagonal of the label box, not half its width: a circle
	// sized from the width alone clips every label that is taller than it
	// is wide, which is what a narrow wrappingWidth produces.
	labelRadius := float64(math.Sqrt(js.Pow(bbox.Width, 2)+js.Pow(bbox.Height, 2)) / 2)
	// The diagonal already contains the label, so neo needs only one
	// padding step.
	radius := labelRadius + padding
	if node.Look == "neo" {
		radius = labelRadius + labelPadding
	}

	var circleElem *dom.Node

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, nil)
		roughNode, err := rc.Circle(0, 0, radius*2, options)
		if err != nil {
			return nil, err
		}

		circleElem = shapeSvg.InsertNode(roughNode, ":first-child")
		circleElem.Attr("class", "basic label-container").Attr("style", ru.CSSStylesAttr(node))
	} else {
		circleElem = shapeSvg.
			Insert("circle", ":first-child").
			Attr("class", "basic label-container").
			Attr("style", styles.NodeStyles).
			Attr("r", radius).
			Attr("cx", 0).
			Attr("cy", 0)
	}

	ru.UpdateNodeBounds(node, circleElem)
	node.CalcIntersect = func(bounds ru.Bounds, point ru.Point) ru.Point {
		radius := bounds.Width / 2
		return ru.IntersectCircle(bounds, radius, point)
	}
	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectCircle(ru.NodeBounds(node), radius, point)
	}

	return shapeSvg, nil
}
