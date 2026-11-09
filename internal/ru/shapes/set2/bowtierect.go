// Ported from mermaid 12.2.0, rendering-elements/shapes/bowTieRect.ts (MIT).

package set2

import (
	"errors"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("bowTieRect", bowTieRect)
}

func generateArcPoints(x1, y1, x2, y2, rx, ry float64, clockwise bool) ([]ru.Point, error) {
	const numPoints = 20
	// The midpoint.
	midX := float64((x1 + x2) / 2)
	midY := float64((y1 + y2) / 2)

	// The angle of the line that connects the points.
	angle := js.Atan2(y2-y1, x2-x1)

	// The transformed coordinates for the ellipse.
	dx := (x2 - x1) / 2
	dy := (y2 - y1) / 2

	// Scale to the unit circle.
	transformedX := dx / rx
	transformedY := dy / ry

	// The distance between the points on the unit circle.
	distance := math.Sqrt(js.Pow(transformedX, 2) + js.Pow(transformedY, 2))

	// Check that the ellipse can be drawn with the given radii.
	if distance > 1 {
		return nil, errors.New("The given radii are too small to create an arc between the points.")
	}

	// The distance from the midpoint to the center of the ellipse.
	scaledCenterDistance := math.Sqrt(1 - js.Pow(distance, 2))

	// The center of the ellipse.
	sign := tern(clockwise, -1, 1)
	centerX := midX + float64(scaledCenterDistance*ry*js.Sin(angle)*sign)
	centerY := midY - float64(scaledCenterDistance*rx*js.Cos(angle)*sign)

	// The start and end angles on the ellipse.
	startAngle := js.Atan2((y1-centerY)/ry, (x1-centerX)/rx)
	endAngle := js.Atan2((y2-centerY)/ry, (x2-centerX)/rx)

	// Adjust the angles for clockwise and counterclockwise.
	angleRange := endAngle - startAngle
	if clockwise && angleRange < 0 {
		angleRange += 2 * math.Pi
	}
	if !clockwise && angleRange > 0 {
		angleRange -= 2 * math.Pi
	}

	// Generate the points.
	var points []ru.Point
	for i := 0; i < numPoints; i++ {
		t := float64(i) / (numPoints - 1)
		angle := startAngle + float64(t*angleRange)
		x := centerX + float64(rx*js.Cos(angle))
		y := centerY + float64(ry*js.Sin(angle))
		points = append(points, ru.Point{X: x, Y: y})
	}

	return points, nil
}

// calculateArcSagitta returns the sagitta of an arc of an ellipse, given
// its chord and its radii.
func calculateArcSagitta(chord, radiusX, radiusY float64) float64 {
	axes := []float64{radiusX, radiusY}
	js.SortFunc(axes, func(a, b float64) float64 { return b - a })
	semiMajorAxis, semiMinorAxis := axes[0], axes[1]
	return float64(semiMinorAxis * (1 - math.Sqrt(1-js.Pow(chord/semiMajorAxis/2, 2))))
}

func bowTieRect(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 16, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 12, nodePadding)

	calcTotalHeight := func(labelHeight float64) float64 { return labelHeight + labelPaddingY }
	calcEllipseRadius := func(totalHeight float64) (float64, float64) {
		ry := totalHeight / 2
		rx := ry / (2.5 + totalHeight/50)
		return rx, ry
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	totalHeight := calcTotalHeight(ru.Or(node.Height, bbox.Height))
	rx, ry := calcEllipseRadius(totalHeight)
	sagitta := calculateArcSagitta(totalHeight, rx, ry)
	totalWidth := ru.Or(node.Width, bbox.Width) + float64(labelPaddingX*2) + sagitta

	w := totalWidth - sagitta
	h := totalHeight

	leftArc, err := generateArcPoints(-w/2, -h/2, -w/2, h/2, rx, ry, false)
	if err != nil {
		return nil, err
	}
	rightArc, err := generateArcPoints(w/2, h/2, w/2, -h/2, rx, ry, true)
	if err != nil {
		return nil, err
	}
	points := []ru.Point{
		{X: w / 2, Y: -h / 2},
		{X: -w / 2, Y: -h / 2},
	}
	points = append(points, leftArc...)
	points = append(points, ru.Point{X: w / 2, Y: h / 2})
	points = append(points, rightArc...)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())
	bowTieRectPath := ru.CreatePathFromPoints(points)
	bowTieRectShapePath, err := rc.Path(bowTieRectPath, options)
	if err != nil {
		return nil, err
	}
	bowTieRectShape := shapeSvg.InsertNode(bowTieRectShapePath, ":first-child")

	bowTieRectShape.Attr("class", "basic label-container outer-path")

	stylePaths(bowTieRectShape.SelectAll("path"), node, nodeStyles)

	bowTieRectShape.Attr("transform", "translate("+js.Str(rx/2)+", 0)")

	ru.UpdateNodeBounds(node, bowTieRectShape)

	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
