// Ported from mermaid 12.2.0, rendering-elements/shapes/crossedCircle.ts
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
	ru.RegisterShape("crossedCircle", crossedCircle)
}

// createLine is createLine of crossedCircle.ts: the two diagonals of the
// circle as path data. The line break and the indentation are part of the
// template literal.
func createLine(r float64) string {
	xAxis45 := js.Cos(math.Pi / 4) // The cosine of 45 degrees.
	yAxis45 := js.Sin(math.Pi / 4) // The sine of 45 degrees.
	lineLength := float64(r * 2)

	pointQ1 := ru.Point{X: (lineLength / 2) * xAxis45, Y: (lineLength / 2) * yAxis45}   // Quadrant I
	pointQ2 := ru.Point{X: -(lineLength / 2) * xAxis45, Y: (lineLength / 2) * yAxis45}  // Quadrant II
	pointQ3 := ru.Point{X: -(lineLength / 2) * xAxis45, Y: -(lineLength / 2) * yAxis45} // Quadrant III
	pointQ4 := ru.Point{X: (lineLength / 2) * xAxis45, Y: -(lineLength / 2) * yAxis45}  // Quadrant IV

	n := js.Str
	return "M " + n(pointQ2.X) + "," + n(pointQ2.Y) + " L " + n(pointQ4.X) + "," + n(pointQ4.Y) + "\n" +
		"                   M " + n(pointQ1.X) + "," + n(pointQ1.Y) + " L " + n(pointQ3.X) + "," + n(pointQ3.Y)
}

func crossedCircle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	node.Label, node.HasLabel = "", true
	shapeSvg := insertShapeGroup(parent, node)
	radius := js.Max(30, ru.NN(node.Width, 0))

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	circleNode, err := rc.Circle(0, 0, radius*2, options)
	if err != nil {
		return nil, err
	}
	linePath := createLine(radius)
	lineNode, err := rc.Path(linePath, options)
	if err != nil {
		return nil, err
	}

	crossedCircle := shapeSvg.InsertNode(circleNode, ":first-child")
	crossedCircle.InsertNode(lineNode)
	crossedCircle.Attr("class", "outer-path")

	stylePaths(crossedCircle.SelectAll("path"), node, nodeStyles)

	ru.UpdateNodeBounds(node, crossedCircle)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectCircle(ru.NodeBounds(node), radius, point)
	}

	return shapeSvg, nil
}
