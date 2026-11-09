// Ported from mermaid 12.2.0, rendering-elements/shapes/curlyBraceLeft.ts
// (MIT).

package set2

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("curlyBraceLeft", curlyBraceLeft)
}

// The generateCirclePoints of curlyBraceLeft.ts is the same code as the one
// of util.ts: ru.GenerateCirclePoints.

func curlyBraceLeft(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label
	paddingX := ru.NN(node.Padding, 0)
	if node.Look == "neo" {
		paddingX = 18
	}
	paddingY := ru.NN(node.Padding, 0)
	if node.Look == "neo" {
		paddingY = 12
	}
	w := bbox.Width + paddingX
	h := bbox.Height + paddingY
	radius := js.Max(5, float64(h*0.1))

	var points []ru.Point
	points = append(points, ru.GenerateCirclePoints(w/2, -h/2, radius, 30, -90, 0)...)
	points = append(points, ru.Point{X: float64(-w/2) - radius, Y: radius})
	points = append(points, ru.GenerateCirclePoints(float64(w/2)+float64(radius*2), -radius, radius, 20, -180, -270)...)
	points = append(points, ru.GenerateCirclePoints(float64(w/2)+float64(radius*2), radius, radius, 20, -90, -180)...)
	points = append(points, ru.Point{X: float64(-w/2) - radius, Y: -h / 2})
	points = append(points, ru.GenerateCirclePoints(w/2, h/2, radius, 20, 0, 90)...)

	var rectPoints []ru.Point
	rectPoints = append(rectPoints,
		ru.Point{X: w / 2, Y: float64(-h/2) - radius},
		ru.Point{X: -w / 2, Y: float64(-h/2) - radius},
	)
	rectPoints = append(rectPoints, ru.GenerateCirclePoints(w/2, -h/2, radius, 20, -90, 0)...)
	rectPoints = append(rectPoints, ru.Point{X: float64(-w/2) - radius, Y: -radius})
	rectPoints = append(rectPoints, ru.GenerateCirclePoints(float64(w/2)+float64(w*0.1), -radius, radius, 20, -180, -270)...)
	rectPoints = append(rectPoints, ru.GenerateCirclePoints(float64(w/2)+float64(w*0.1), radius, radius, 20, -90, -180)...)
	rectPoints = append(rectPoints, ru.Point{X: float64(-w/2) - radius, Y: h / 2})
	rectPoints = append(rectPoints, ru.GenerateCirclePoints(w/2, h/2, radius, 20, 0, 90)...)
	rectPoints = append(rectPoints,
		ru.Point{X: -w / 2, Y: float64(h/2) + radius},
		ru.Point{X: w / 2, Y: float64(h/2) + radius},
	)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions("fill", "none"))
	curlyBraceLeftPath := ru.CreatePathFromPoints(points)
	newCurlyBracePath := strings.Replace(curlyBraceLeftPath, "Z", "", 1)
	curlyBraceLeftNode, err := rc.Path(newCurlyBracePath, options)
	if err != nil {
		return nil, err
	}
	rectPath := ru.CreatePathFromPoints(rectPoints)
	rectShape, err := rc.Path(rectPath, spread(options))
	if err != nil {
		return nil, err
	}
	curlyBraceLeftShape := shapeSvg.Insert("g", ":first-child")
	curlyBraceLeftShape.InsertNode(rectShape, ":first-child").Attr("stroke-opacity", 0)
	curlyBraceLeftShape.InsertNode(curlyBraceLeftNode, ":first-child")
	curlyBraceLeftShape.Attr("class", "text")

	stylePaths(curlyBraceLeftShape.SelectAll("path"), node, nodeStyles)

	curlyBraceLeftShape.Attr("transform", "translate("+js.Str(radius)+", 0)")

	label.Attr(
		"transform",
		"translate("+js.Str(float64(-w/2)+radius-(bbox.X-bbox.LeftOr0()))+","+
			js.Str(float64(-h/2)+float64(ru.NN(node.Padding, 0)/2)-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, curlyBraceLeftShape)

	node.Intersect = polygonIntersect(node, rectPoints)

	return shapeSvg, nil
}
