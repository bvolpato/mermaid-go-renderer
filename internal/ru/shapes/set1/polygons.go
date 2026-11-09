// Ported from mermaid 12.2.0, rendering-elements/shapes/leanRight.ts,
// leanLeft.ts, trapezoid.ts and invertedTrapezoid.ts (MIT). The four upstream
// functions have the same body. They differ in the points of the polygon, and
// inv_trapezoid doubles the label padding.

package set1

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("lean_right", leanRight)
	ru.RegisterShape("lean_left", leanLeft)
	ru.RegisterShape("trapezoid", trapezoid)
	ru.RegisterShape("inv_trapezoid", invTrapezoid)
}

func leanRight(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	return slantedPolygon(ctx, parent, node, 1, func(w, h float64) []ru.Point {
		return []ru.Point{
			{X: float64(-3*h) / 6, Y: 0},
			{X: w, Y: 0},
			{X: w + float64(3*h)/6, Y: -h},
			{X: 0, Y: -h},
		}
	})
}

func leanLeft(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	return slantedPolygon(ctx, parent, node, 1, func(w, h float64) []ru.Point {
		return []ru.Point{
			{X: 0, Y: 0},
			{X: w + float64(3*h)/6, Y: 0},
			{X: w, Y: -h},
			{X: -float64(3*h) / 6, Y: -h},
		}
	})
}

func trapezoid(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	return slantedPolygon(ctx, parent, node, 1, func(w, h float64) []ru.Point {
		return []ru.Point{
			{X: float64(-3*h) / 6, Y: 0},
			{X: w + float64(3*h)/6, Y: 0},
			{X: w, Y: -h},
			{X: 0, Y: -h},
		}
	})
}

func invTrapezoid(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	return slantedPolygon(ctx, parent, node, 2, func(w, h float64) []ru.Point {
		return []ru.Point{
			{X: 0, Y: 0},
			{X: w, Y: 0},
			{X: w + float64(3*h)/6, Y: -h},
			{X: float64(-3*h) / 6, Y: -h},
		}
	})
}

// slantedPolygon is the common body of lean_right, lean_left, trapezoid and
// inv_trapezoid. paddingFactor multiplies both label paddings: it is 2 for
// inv_trapezoid and 1 for the other shapes.
func slantedPolygon(ctx *render.Context, parent *dom.Node, node *ru.Node, paddingFactor float64, makePoints func(w, h float64) []ru.Point) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingY := nodePadding
	labelPaddingX := nodePadding
	if node.Look == "neo" {
		labelPaddingX = float64(nodePadding * 2)
	}
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox
	h := js.Max(bbox.Height+float64(labelPaddingY*paddingFactor), ru.NN(node.Height, 0))
	w := js.Max(bbox.Width+float64(labelPaddingX*paddingFactor), ru.NN(node.Width, 0)-h)

	points := makePoints(w, h)

	var polygon *dom.Node

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, nil)
		pathData := ru.CreatePathFromPoints(points)
		roughNode, err := rc.Path(pathData, options)
		if err != nil {
			return nil, err
		}

		polygon = shapeSvg.
			InsertNode(roughNode, ":first-child").
			Attr("transform", "translate("+js.Str(-w/2)+", "+js.Str(h/2)+")")

		if hasCSSStyles(node) {
			polygon.Attr("style", cssStylesString(node))
		}
	} else {
		polygon = ru.InsertPolygonShape(shapeSvg, w, h, points)
	}

	if styles.NodeStyles != "" {
		polygon.Attr("style", styles.NodeStyles)
	}

	node.Width = w
	node.Height = h

	ru.UpdateNodeBounds(node, polygon)

	polygonIntersect(node, points)

	return shapeSvg, nil
}
