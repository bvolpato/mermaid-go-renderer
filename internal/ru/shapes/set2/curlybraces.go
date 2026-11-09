// Ported from mermaid 12.2.0, rendering-elements/shapes/curlyBraces.ts (MIT).

package set2

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("curlyBraces", curlyBraces)
}

// The generateCirclePoints of curlyBraces.ts is the same code as the one of
// util.ts: ru.GenerateCirclePoints.

func curlyBraces(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
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

	gcp := ru.GenerateCirclePoints

	var leftCurlyBracePoints []ru.Point
	leftCurlyBracePoints = append(leftCurlyBracePoints, gcp(w/2, -h/2, radius, 30, -90, 0)...)
	leftCurlyBracePoints = append(leftCurlyBracePoints, ru.Point{X: float64(-w/2) - radius, Y: radius})
	leftCurlyBracePoints = append(leftCurlyBracePoints, gcp(float64(w/2)+float64(radius*2), -radius, radius, 20, -180, -270)...)
	leftCurlyBracePoints = append(leftCurlyBracePoints, gcp(float64(w/2)+float64(radius*2), radius, radius, 20, -90, -180)...)
	leftCurlyBracePoints = append(leftCurlyBracePoints, ru.Point{X: float64(-w/2) - radius, Y: -h / 2})
	leftCurlyBracePoints = append(leftCurlyBracePoints, gcp(w/2, h/2, radius, 20, 0, 90)...)

	var rightCurlyBracePoints []ru.Point
	rightCurlyBracePoints = append(rightCurlyBracePoints, gcp(float64(-w/2)+radius+float64(radius/2), -h/2, radius, 20, -90, -180)...)
	rightCurlyBracePoints = append(rightCurlyBracePoints, ru.Point{X: float64(w/2) - float64(radius/2), Y: radius})
	rightCurlyBracePoints = append(rightCurlyBracePoints, gcp(float64(-w/2)-float64(radius/2), -radius, radius, 20, 0, 90)...)
	rightCurlyBracePoints = append(rightCurlyBracePoints, gcp(float64(-w/2)-float64(radius/2), radius, radius, 20, -90, 0)...)
	rightCurlyBracePoints = append(rightCurlyBracePoints, ru.Point{X: float64(w/2) - float64(radius/2), Y: -radius})
	rightCurlyBracePoints = append(rightCurlyBracePoints, gcp(float64(-w/2)+radius+float64(radius/2), h/2, radius, 30, -180, -270)...)

	var rectPoints []ru.Point
	rectPoints = append(rectPoints,
		ru.Point{X: w / 2, Y: float64(-h/2) - radius},
		ru.Point{X: -w / 2, Y: float64(-h/2) - radius},
	)
	rectPoints = append(rectPoints, gcp(w/2, -h/2, radius, 20, -90, 0)...)
	rectPoints = append(rectPoints, ru.Point{X: float64(-w/2) - radius, Y: -radius})
	rectPoints = append(rectPoints, gcp(float64(w/2)+float64(radius*2), -radius, radius, 20, -180, -270)...)
	rectPoints = append(rectPoints, gcp(float64(w/2)+float64(radius*2), radius, radius, 20, -90, -180)...)
	rectPoints = append(rectPoints, ru.Point{X: float64(-w/2) - radius, Y: h / 2})
	rectPoints = append(rectPoints, gcp(w/2, h/2, radius, 20, 0, 90)...)
	rectPoints = append(rectPoints,
		ru.Point{X: -w / 2, Y: float64(h/2) + radius},
		ru.Point{X: float64(w/2) - radius - float64(radius/2), Y: float64(h/2) + radius},
	)
	rectPoints = append(rectPoints, gcp(float64(-w/2)+radius+float64(radius/2), -h/2, radius, 20, -90, -180)...)
	rectPoints = append(rectPoints, ru.Point{X: float64(w/2) - float64(radius/2), Y: radius})
	rectPoints = append(rectPoints, gcp(float64(-w/2)-float64(radius/2), -radius, radius, 20, 0, 90)...)
	rectPoints = append(rectPoints, gcp(float64(-w/2)-float64(radius/2), radius, radius, 20, -90, 0)...)
	rectPoints = append(rectPoints, ru.Point{X: float64(w/2) - float64(radius/2), Y: -radius})
	rectPoints = append(rectPoints, gcp(float64(-w/2)+radius+float64(radius/2), h/2, radius, 30, -180, -270)...)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions("fill", "none"))
	leftCurlyBracePath := ru.CreatePathFromPoints(leftCurlyBracePoints)
	newLeftCurlyBracePath := strings.Replace(leftCurlyBracePath, "Z", "", 1)
	leftCurlyBraceNode, err := rc.Path(newLeftCurlyBracePath, options)
	if err != nil {
		return nil, err
	}
	rightCurlyBracePath := ru.CreatePathFromPoints(rightCurlyBracePoints)
	newRightCurlyBracePath := strings.Replace(rightCurlyBracePath, "Z", "", 1)
	rightCurlyBraceNode, err := rc.Path(newRightCurlyBracePath, options)
	if err != nil {
		return nil, err
	}
	rectPath := ru.CreatePathFromPoints(rectPoints)
	rectShape, err := rc.Path(rectPath, spread(options))
	if err != nil {
		return nil, err
	}
	curlyBracesShape := shapeSvg.Insert("g", ":first-child")
	curlyBracesShape.InsertNode(rectShape, ":first-child").Attr("stroke-opacity", 0)
	curlyBracesShape.InsertNode(leftCurlyBraceNode, ":first-child")
	curlyBracesShape.InsertNode(rightCurlyBraceNode, ":first-child")
	curlyBracesShape.Attr("class", "text")

	stylePaths(curlyBracesShape.SelectAll("path"), node, nodeStyles)

	curlyBracesShape.Attr("transform", "translate("+js.Str(radius-float64(radius/4))+", 0)")

	label.Attr(
		"transform",
		"translate("+js.Str(float64(-w/2)+float64(ru.NN(node.Padding, 0)/2)-(bbox.X-bbox.LeftOr0()))+","+
			js.Str(float64(-h/2)+float64(ru.NN(node.Padding, 0)/2)-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, curlyBracesShape)

	node.Intersect = polygonIntersect(node, rectPoints)

	return shapeSvg, nil
}
