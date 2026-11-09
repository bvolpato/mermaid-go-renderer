// Ported from mermaid 12.2.0,
// rendering-elements/shapes/multiWaveEdgedRectangle.ts (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("multiWaveEdgedRectangle", multiWaveEdgedRectangle)
}

func multiWaveEdgedRectangle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 16, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 12, nodePadding)
	adjustFinalHeight := true

	if truthy(node.Width) || truthy(node.Height) {
		adjustFinalHeight = false
		node.Width = ru.NN(node.Width, 0) - float64(labelPaddingX*2)
		node.Height = ru.NN(node.Height, 0) - float64(labelPaddingY*3)
	}

	w := js.Max(bbox.Width, ru.NN(node.Width, 0)) + float64(labelPaddingX*2)
	h := js.Max(bbox.Height, ru.NN(node.Height, 0)) + float64(labelPaddingY*3)
	waveAmplitude := tern(node.Look == "neo", float64(h/4), float64(h/8))
	finalH := h + tern(adjustFinalHeight, float64(waveAmplitude/2), float64(-waveAmplitude/2))
	x := float64(-w / 2)
	y := float64(-finalH / 2)
	const rectOffset = 10.0

	wavePoints := ru.GenerateFullSineWavePoints(
		x-rectOffset,
		y+finalH+rectOffset,
		x+w-rectOffset,
		y+finalH+rectOffset,
		waveAmplitude,
		0.8,
	)

	lastWavePoint := wavePoints[len(wavePoints)-1]

	outerPathPoints := []ru.Point{
		{X: x - rectOffset, Y: y + rectOffset},
		{X: x - rectOffset, Y: y + finalH + rectOffset},
	}
	outerPathPoints = append(outerPathPoints, wavePoints...)
	outerPathPoints = append(outerPathPoints,
		ru.Point{X: x + w - rectOffset, Y: lastWavePoint.Y - rectOffset},
		ru.Point{X: x + w, Y: lastWavePoint.Y - rectOffset},
		ru.Point{X: x + w, Y: lastWavePoint.Y - 2*rectOffset},
		ru.Point{X: x + w + rectOffset, Y: lastWavePoint.Y - 2*rectOffset},
		ru.Point{X: x + w + rectOffset, Y: y - rectOffset},
		ru.Point{X: x + rectOffset, Y: y - rectOffset},
		ru.Point{X: x + rectOffset, Y: y},
		ru.Point{X: x, Y: y},
		ru.Point{X: x, Y: y + rectOffset},
	)

	innerPathPoints := []ru.Point{
		{X: x, Y: y + rectOffset},
		{X: x + w - rectOffset, Y: y + rectOffset},
		{X: x + w - rectOffset, Y: lastWavePoint.Y - rectOffset},
		{X: x + w, Y: lastWavePoint.Y - rectOffset},
		{X: x + w, Y: y},
		{X: x, Y: y},
	}

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	outerPath := ru.CreatePathFromPoints(outerPathPoints)
	outerNode, err := rc.Path(outerPath, options)
	if err != nil {
		return nil, err
	}
	innerPath := ru.CreatePathFromPoints(innerPathPoints)
	innerNode, err := rc.Path(innerPath, options)
	if err != nil {
		return nil, err
	}

	shape := shapeSvg.InsertNode(outerNode, ":first-child")
	shape.InsertNode(innerNode)

	shape.Attr("class", "basic label-container outer-path")

	stylePaths(shape.SelectAll("path"), node, nodeStyles)

	shape.Attr("transform", "translate(0,"+js.Str(-waveAmplitude/2)+")")

	label.Attr(
		"transform",
		"translate("+js.Str(-float64(bbox.Width/2)-rectOffset-(bbox.X-bbox.LeftOr0()))+", "+
			js.Str(-float64(bbox.Height/2)+rectOffset-waveAmplitude-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, shape)

	node.Intersect = polygonIntersect(node, outerPathPoints)

	return shapeSvg, nil
}
