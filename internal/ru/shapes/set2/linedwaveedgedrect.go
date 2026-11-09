// Ported from mermaid 12.2.0, rendering-elements/shapes/linedWaveEdgedRect.ts
// (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("linedWaveEdgedRect", linedWaveEdgedRect)
}

func linedWaveEdgedRect(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles

	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 16, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 12, nodePadding)
	if truthy(node.Width) || truthy(node.Height) {
		originalWidth := node.Width
		node.Width = float64(ru.NN(originalWidth, 0)*10)/11 - float64(labelPaddingX*2)
		if node.Width < 10 {
			node.Width = 10
		}
		node.Height = ru.NN(node.Height, 0) - float64(labelPaddingY*2)
		if node.Height < 10 {
			node.Height = 10
		}
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label
	w := ru.Or(node.Width, bbox.Width) + float64(labelPaddingX*2)
	h := ru.Or(node.Height, bbox.Height) + float64(labelPaddingY*2)
	waveAmplitude := tern(node.Look == "neo", float64(h/4), float64(h/8))
	finalH := h + waveAmplitude

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	points := []ru.Point{
		{X: float64(-w/2) - float64((w/2)*0.1), Y: -finalH / 2},
		{X: float64(-w/2) - float64((w/2)*0.1), Y: finalH / 2},
	}
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
		ru.Point{X: -w / 2, Y: -finalH / 2},
		ru.Point{X: -w / 2, Y: (finalH / 2) * 1.1},
		ru.Point{X: -w / 2, Y: -finalH / 2},
	)

	poly, err := rc.Polygon(roughPoints(points), options)
	if err != nil {
		return nil, err
	}

	waveEdgeRect := shapeSvg.InsertNode(poly, ":first-child")

	waveEdgeRect.Attr("class", "basic label-container outer-path")

	stylePaths(waveEdgeRect.SelectAll("path"), node, nodeStyles)

	waveEdgeRect.Attr("transform", "translate(0,"+js.Str(-waveAmplitude/2)+")")
	label.Attr(
		"transform",
		"translate("+js.Str(float64(-w/2)+ru.NN(node.Padding, 0)+float64(((w/2)*0.1)/2)-(bbox.X-bbox.LeftOr0()))+","+
			js.Str(float64(-h/2)+ru.NN(node.Padding, 0)-waveAmplitude-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, waveEdgeRect)
	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
