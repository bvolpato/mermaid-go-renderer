// Ported from mermaid 12.2.0, rendering-elements/shapes/curlyBraceRight.ts
// (MIT).

package set2

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("curlyBraceRight", curlyBraceRight)
}

// curlyBraceRightCirclePoints is the generateCirclePoints of
// curlyBraceRight.ts. It does not negate the points, as the one of util.ts
// does.
func curlyBraceRightCirclePoints(centerX, centerY, radius float64, numPoints int, startAngle, endAngle float64) []ru.Point {
	var points []ru.Point

	// Convert the angles to radians.
	startAngleRad := float64(startAngle*math.Pi) / 180
	endAngleRad := float64(endAngle*math.Pi) / 180

	// The angle range in radians.
	angleRange := endAngleRad - startAngleRad

	// The angle step.
	angleStep := angleRange / float64(numPoints-1)

	for i := 0; i < numPoints; i++ {
		angle := startAngleRad + float64(float64(i)*angleStep)
		x := centerX + float64(radius*js.Cos(angle))
		y := centerY + float64(radius*js.Sin(angle))
		points = append(points, ru.Point{X: x, Y: y})
	}

	return points
}

func curlyBraceRight(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label
	labelPaddingX := tern(node.Look == "neo", 18, ru.NN(node.Padding, 0))
	labelPaddingY := tern(node.Look == "neo", 12, ru.NN(node.Padding, 0))
	w := bbox.Width + tern(node.Look == "neo", float64(labelPaddingX*2), labelPaddingX)
	h := bbox.Height + tern(node.Look == "neo", float64(labelPaddingY*2), labelPaddingY)
	radius := js.Max(5, float64(h*0.1))

	gcp := curlyBraceRightCirclePoints

	var points []ru.Point
	points = append(points, gcp(w/2, -h/2, radius, 20, -90, 0)...)
	points = append(points, ru.Point{X: float64(w/2) + radius, Y: -radius})
	points = append(points, gcp(float64(w/2)+float64(radius*2), -radius, radius, 20, -180, -270)...)
	points = append(points, gcp(float64(w/2)+float64(radius*2), radius, radius, 20, -90, -180)...)
	points = append(points, ru.Point{X: float64(w/2) + radius, Y: h / 2})
	points = append(points, gcp(w/2, h/2, radius, 20, 0, 90)...)

	var rectPoints []ru.Point
	rectPoints = append(rectPoints,
		ru.Point{X: -w / 2, Y: float64(-h/2) - radius},
		ru.Point{X: w / 2, Y: float64(-h/2) - radius},
	)
	rectPoints = append(rectPoints, gcp(w/2, -h/2, radius, 20, -90, 0)...)
	rectPoints = append(rectPoints, ru.Point{X: float64(w/2) + radius, Y: -radius})
	rectPoints = append(rectPoints, gcp(float64(w/2)+float64(radius*2), -radius, radius, 20, -180, -270)...)
	rectPoints = append(rectPoints, gcp(float64(w/2)+float64(radius*2), radius, radius, 20, -90, -180)...)
	rectPoints = append(rectPoints, ru.Point{X: float64(w/2) + radius, Y: h / 2})
	rectPoints = append(rectPoints, gcp(w/2, h/2, radius, 20, 0, 90)...)
	rectPoints = append(rectPoints,
		ru.Point{X: w / 2, Y: float64(h/2) + radius},
		ru.Point{X: -w / 2, Y: float64(h/2) + radius},
	)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions("fill", "none"))
	curlyBraceRightPath := ru.CreatePathFromPoints(points)
	newCurlyBracePath := strings.Replace(curlyBraceRightPath, "Z", "", 1)
	curlyBraceRightNode, err := rc.Path(newCurlyBracePath, options)
	if err != nil {
		return nil, err
	}
	rectPath := ru.CreatePathFromPoints(rectPoints)
	rectShape, err := rc.Path(rectPath, spread(options))
	if err != nil {
		return nil, err
	}
	curlyBraceRightShape := shapeSvg.Insert("g", ":first-child")
	curlyBraceRightShape.InsertNode(rectShape, ":first-child").Attr("stroke-opacity", 0)
	curlyBraceRightShape.InsertNode(curlyBraceRightNode, ":first-child")
	curlyBraceRightShape.Attr("class", "text")

	stylePaths(curlyBraceRightShape.SelectAll("path"), node, nodeStyles)

	curlyBraceRightShape.Attr("transform", "translate("+js.Str(-radius)+", 0)")

	label.Attr(
		"transform",
		"translate("+js.Str(float64(-w/2)+float64(ru.NN(node.Padding, 0)/2)-(bbox.X-bbox.LeftOr0()))+","+
			js.Str(float64(-h/2)+float64(ru.NN(node.Padding, 0)/2)-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, curlyBraceRightShape)

	node.Intersect = polygonIntersect(node, rectPoints)

	return shapeSvg, nil
}
