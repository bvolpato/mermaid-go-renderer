// Ported from mermaid 12.2.0, rendering-elements/shapes/waveRectangle.ts
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
	ru.RegisterShape("waveRectangle", waveRectangle)
}

func waveRectangle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 16, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 20, nodePadding)

	if truthy(node.Width) || truthy(node.Height) {
		node.Width = ru.NN(node.Width, 0)
		if node.Width < 20 {
			node.Width = 20
		}

		node.Height = ru.NN(node.Height, 0)
		if node.Height < 10 {
			node.Height = 10
		}

		// Adjust for the wave amplitude.
		waveAmplitude := js.Min(float64(node.Height*0.2), float64(node.Height/4))
		node.Height = math.Ceil(node.Height - labelPaddingY - float64(waveAmplitude*(20.0/9)))
		node.Width = node.Width - float64(labelPaddingX*2)
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	w := ru.Or(node.Width, bbox.Width) + float64(labelPaddingX*2)
	h := ru.Or(node.Height, bbox.Height) + labelPaddingY

	waveAmplitude := float64(h / 8)
	finalH := h + float64(waveAmplitude*2)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := solidOptions(ctx, node, roughOptions())

	points := []ru.Point{{X: -w / 2, Y: finalH / 2}}
	points = append(points, ru.GenerateFullSineWavePoints(-w/2, finalH/2, w/2, finalH/2, waveAmplitude, 1)...)
	points = append(points, ru.Point{X: w / 2, Y: -finalH / 2})
	points = append(points, ru.GenerateFullSineWavePoints(w/2, -finalH/2, -w/2, -finalH/2, waveAmplitude, -1)...)

	waveRectPath := ru.CreatePathFromPoints(points)
	waveRectNode, err := rc.Path(waveRectPath, options)
	if err != nil {
		return nil, err
	}

	waveRect := shapeSvg.InsertNode(waveRectNode, ":first-child")

	waveRect.Attr("class", "basic label-container")

	stylePaths(waveRect.SelectAll("path"), node, nodeStyles)

	ru.UpdateNodeBounds(node, waveRect)
	node.Intersect = polygonIntersect(node, points)

	return shapeSvg, nil
}
