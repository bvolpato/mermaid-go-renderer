// Ported from mermaid 12.2.0,
// rendering-elements/shapes/taggedWaveEdgedRectangle.ts (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("taggedWaveEdgedRectangle", taggedWaveEdgedRectangle)
}

func taggedWaveEdgedRectangle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label
	w := js.Max(bbox.Width+float64(ru.NN(node.Padding, 0)*2), ru.NN(node.Width, 0))
	h := js.Max(bbox.Height+float64(ru.NN(node.Padding, 0)*2), ru.NN(node.Height, 0))
	waveAmplitude := float64(h / 8)
	tagWidth := float64(0.2 * w)
	tagHeight := float64(0.2 * h)
	finalH := h + waveAmplitude

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	points := []ru.Point{{X: float64(-w/2) - float64((w/2)*0.1), Y: finalH / 2}}
	points = append(points, ru.GenerateFullSineWavePoints(
		float64(-w/2)-float64((w/2)*0.1),
		finalH/2,
		float64(w/2)+float64((w/2)*0.1),
		finalH/2,
		waveAmplitude,
		0.8,
	)...)
	points = append(points,
		ru.Point{X: float64(w/2) + float64((w/2)*0.1), Y: -finalH / 2},
		ru.Point{X: float64(-w/2) - float64((w/2)*0.1), Y: -finalH / 2},
	)

	x := float64(-w/2) + float64((w/2)*0.1)
	y := float64(-finalH/2) - float64(tagHeight*0.4)

	tagPoints := []ru.Point{
		{X: x + w - tagWidth, Y: (y + h) * 1.3},
		{X: x + w, Y: y + h - tagHeight},
		{X: x + w, Y: (y + h) * 0.9},
	}
	tagPoints = append(tagPoints, ru.GenerateFullSineWavePoints(
		x+w,
		(y+h)*1.25,
		x+w-tagWidth,
		(y+h)*1.3,
		-h*0.02,
		0.5,
	)...)

	waveEdgeRectPath := ru.CreatePathFromPoints(points)
	waveEdgeRectNode, err := rc.Path(waveEdgeRectPath, options)
	if err != nil {
		return nil, err
	}

	taggedWaveEdgeRectPath := ru.CreatePathFromPoints(tagPoints)
	taggedWaveEdgeRectNode, err := rc.Path(taggedWaveEdgeRectPath, spread(options, "fillStyle", "solid"))
	if err != nil {
		return nil, err
	}

	waveEdgeRect := shapeSvg.InsertNode(taggedWaveEdgeRectNode, ":first-child")
	waveEdgeRect.InsertNode(waveEdgeRectNode, ":first-child")

	waveEdgeRect.Attr("class", "basic label-container outer-path")

	stylePaths(waveEdgeRect.SelectAll("path"), node, nodeStyles)

	waveEdgeRect.Attr("transform", "translate(0,"+js.Str(-waveAmplitude/2)+")")
	label.Attr(
		"transform",
		"translate("+js.Str(float64(-w/2)+ru.NN(node.Padding, 0)-(bbox.X-bbox.LeftOr0()))+","+
			js.Str(float64(-h/2)+ru.NN(node.Padding, 0)-float64(waveAmplitude/2)-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, waveEdgeRect)
	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
