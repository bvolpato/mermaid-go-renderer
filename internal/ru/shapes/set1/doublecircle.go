// Ported from mermaid 12.2.0, rendering-elements/shapes/doubleCircle.ts (MIT).

package set1

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("doublecircle", doublecircle)
}

func doublecircle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	gap := 5.0
	if node.Look == "neo" {
		gap = 12
	}
	node.LabelStyle = styles.LabelStyles
	padding := ru.NN(node.Padding, 0)
	labelPadding := padding
	if node.Look == "neo" {
		labelPadding = 16
	}
	// The shape is sized from the label alone. See circle.go for why the
	// min width is skipped.
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node), ru.LabelHelperOptions{IgnoreMinWidth: true})
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox
	// The label sits inside the inner ring, so that is the circle that is
	// sized from the diagonal of the label box. The outer ring is the gap
	// beyond it.
	labelRadius := float64(math.Sqrt(js.Pow(bbox.Width, 2)+js.Pow(bbox.Height, 2)) / 2)
	innerRadius := labelRadius
	if js.Truthy(node.Width) {
		innerRadius = float64(node.Width / 2)
	}
	innerRadius += labelPadding
	outerRadius := innerRadius + gap

	var circleGroup *dom.Node

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		outer := js.NewObj[any]()
		outer.Set("roughness", 0.2)
		outer.Set("strokeWidth", 2.5)
		outerOptions := ru.UserNodeOverrides(ctx, node, outer)

		inner := js.NewObj[any]()
		inner.Set("roughness", 0.2)
		inner.Set("strokeWidth", 1.5)
		innerOptions := ru.UserNodeOverrides(ctx, node, inner)
		outerRoughNode, err := rc.Circle(0, 0, outerRadius*2, outerOptions)
		if err != nil {
			return nil, err
		}
		innerRoughNode, err := rc.Circle(0, 0, innerRadius*2, innerOptions)
		if err != nil {
			return nil, err
		}

		circleGroup = shapeSvg.Insert("g", ":first-child")
		circleGroup.
			Attr("class", ru.StrAttr(node.CSSClasses, node.CSSClassesSet || node.CSSClasses != "")).
			Attr("style", ru.CSSStylesAttr(node))

		circleGroup.AppendChild(outerRoughNode)
		circleGroup.AppendChild(innerRoughNode)
	} else {
		circleGroup = shapeSvg.Insert("g", ":first-child")

		outerCircle := circleGroup.Insert("circle", ":first-child")
		innerCircle := circleGroup.Insert("circle")
		circleGroup.Attr("class", "basic label-container").Attr("style", styles.NodeStyles)

		outerCircle.
			Attr("class", "outer-circle").
			Attr("style", styles.NodeStyles).
			Attr("r", outerRadius).
			Attr("cx", 0).
			Attr("cy", 0)

		innerCircle.
			Attr("class", "inner-circle").
			Attr("style", styles.NodeStyles).
			Attr("r", innerRadius).
			Attr("cx", 0).
			Attr("cy", 0)
	}

	ru.UpdateNodeBounds(node, circleGroup)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectCircle(ru.NodeBounds(node), outerRadius, point)
	}

	return shapeSvg, nil
}
