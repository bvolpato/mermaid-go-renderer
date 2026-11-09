// Ported from mermaid 12.2.0, rendering-elements/shapes/curvedTrapezoid.ts
// (MIT).

package set2

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("curvedTrapezoid", curvedTrapezoid)
}

func curvedTrapezoid(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 16, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 12, nodePadding)
	const minWidth, minHeight = 20.0, 5.0
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox
	h := js.Max(minHeight, bbox.Height+float64(labelPaddingY*2), ru.NN(node.Height, 0))
	radius := float64(h / 2)
	// The label is centered between a chevron of width h/4 and a cap of
	// radius h/2, so reserve the larger of the two on both sides.
	capClearance := radius - math.Sqrt(js.Max(0, js.Pow(radius, 2)-js.Pow(bbox.Height/2, 2)))
	sideClearance := js.Max(h/4, capClearance)
	w := js.Max(
		minWidth,
		bbox.Width+float64(labelPaddingX*2)+float64(sideClearance*2),
		float64((bbox.Width+float64(labelPaddingX*2))*1.25),
		ru.NN(node.Width, 0),
	)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	totalWidth, totalHeight := w, h
	rw := totalWidth - radius
	tw := totalHeight / 4

	points := []ru.Point{
		{X: rw, Y: 0},
		{X: tw, Y: 0},
		{X: 0, Y: totalHeight / 2},
		{X: tw, Y: totalHeight},
		{X: rw, Y: totalHeight},
	}
	points = append(points, ru.GenerateCirclePoints(-rw, -totalHeight/2, radius, 50, 270, 90)...)

	pathData := ru.CreatePathFromPoints(points)
	shapeNode, err := rc.Path(pathData, options)
	if err != nil {
		return nil, err
	}

	polygon := shapeSvg.InsertNode(shapeNode, ":first-child")
	polygon.Attr("class", "basic label-container outer-path")

	stylePaths(childPaths(polygon), node, nodeStyles)

	polygon.Attr("transform", "translate("+js.Str(-w/2)+", "+js.Str(-h/2)+")")

	ru.UpdateNodeBounds(node, polygon)

	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
