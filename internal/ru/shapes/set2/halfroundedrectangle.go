// Ported from mermaid 12.2.0,
// rendering-elements/shapes/halfRoundedRectangle.ts (MIT).

package set2

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("halfRoundedRectangle", halfRoundedRectangle)
}

func halfRoundedRectangle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	const minWidth, minHeight = 15.0, 10.0

	paddingX := tern(node.Look == "neo", 16, ru.NN(node.Padding, 0))
	paddingY := tern(node.Look == "neo", 12, ru.NN(node.Padding, 0))

	if truthy(node.Width) || truthy(node.Height) {
		node.Height = ru.NN(node.Height, 0) - float64(paddingY*2)
		if node.Height < minHeight {
			node.Height = minHeight
		}

		node.Width = ru.NN(node.Width, 0) - float64(paddingX*2)
		if node.Width < minWidth {
			node.Width = minWidth
		}
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	h := tern(truthy(node.Height), node.Height, js.Max(minHeight, bbox.Height)) + float64(paddingY*2)
	radius := float64(h / 2)
	// The label is centered, so it has to clear the half circle at the
	// right at its own top and bottom corners, and not only at the vertical
	// middle, where the cap is widest.
	capClearance := radius - math.Sqrt(js.Max(0, js.Pow(radius, 2)-js.Pow(bbox.Height/2, 2)))
	// Doubled: the label is centered, so a wider shape moves the center of
	// the label to the right too.
	w := tern(truthy(node.Width), node.Width, js.Max(minWidth, bbox.Width)+float64(capClearance*2)) + float64(paddingX*2)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	points := []ru.Point{
		{X: -w / 2, Y: -h / 2},
		{X: float64(w/2) - radius, Y: -h / 2},
	}
	points = append(points, ru.GenerateCirclePoints(float64(-w/2)+radius, 0, radius, 50, 90, 270)...)
	points = append(points,
		ru.Point{X: float64(w/2) - radius, Y: h / 2},
		ru.Point{X: -w / 2, Y: h / 2},
	)

	pathData := ru.CreatePathFromPoints(points)
	shapeNode, err := rc.Path(pathData, options)
	if err != nil {
		return nil, err
	}
	polygon := shapeSvg.InsertNode(shapeNode, ":first-child")
	polygon.Attr("class", "basic label-container outer-path")

	stylePaths(childPaths(polygon), node, nodeStyles)

	ru.UpdateNodeBounds(node, polygon)

	node.Intersect = polygonIntersect(node, points)
	return shapeSvg, nil
}
