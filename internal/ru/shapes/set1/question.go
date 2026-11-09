// Ported from mermaid 12.2.0, rendering-elements/shapes/question.ts (MIT).

package set1

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("question", question)
}

func createDecisionBoxPathD(x, y, size float64) string {
	n := js.Str
	return strings.Join([]string{
		"M" + n(x+float64(size/2)) + "," + n(y),
		"L" + n(x+size) + "," + n(y-float64(size/2)),
		"L" + n(x+float64(size/2)) + "," + n(y-size),
		"L" + n(x) + "," + n(y-float64(size/2)),
		"Z",
	}, " ")
}

func question(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	w := bbox.Width + ru.NN(node.Padding, 0)
	h := bbox.Height + ru.NN(node.Padding, 0)
	s := w + h
	const adjustment = 0.5

	points := []ru.Point{
		{X: s / 2, Y: 0},
		{X: s, Y: -s / 2},
		{X: s / 2, Y: -s},
		{X: 0, Y: -s / 2},
	}

	var polygon *dom.Node
	transform := "translate(" + js.Str(float64(-s/2)+adjustment) + ", " + js.Str(s/2) + ")"

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, nil)
		pathData := createDecisionBoxPathD(0, 0, s)
		roughNode, err := rc.Path(pathData, options)
		if err != nil {
			return nil, err
		}

		polygon = shapeSvg.
			InsertNode(roughNode, ":first-child").
			Attr("transform", transform)

		if hasCSSStyles(node) {
			polygon.Attr("style", cssStylesString(node))
		}
	} else {
		polygon = ru.InsertPolygonShape(shapeSvg, s, s, points)
		polygon.Attr("transform", transform)
	}

	if styles.NodeStyles != "" {
		polygon.Attr("style", styles.NodeStyles)
	}

	ru.UpdateNodeBounds(node, polygon)
	node.CalcIntersect = func(bounds ru.Bounds, point ru.Point) ru.Point {
		s := bounds.Width

		// The points of the polygon.
		points := []ru.Point{
			{X: s / 2, Y: 0},
			{X: s, Y: -s / 2},
			{X: s / 2, Y: -s},
			{X: 0, Y: -s / 2},
		}

		// Calculate the intersection point.
		return ru.IntersectPolygon(bounds, points, point)
	}

	node.Intersect = func(point ru.Point) ru.Point {
		return node.CalcIntersect(ru.NodeBounds(node), point)
	}

	return shapeSvg, nil
}
