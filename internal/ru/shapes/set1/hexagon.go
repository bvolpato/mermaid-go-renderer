// Ported from mermaid 12.2.0, rendering-elements/shapes/hexagon.ts (MIT).

package set1

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("hexagon", hexagon)
}

func createHexagonPathD(x, y, width, height, m float64) string {
	n := js.Str
	return strings.Join([]string{
		"M" + n(x+m) + "," + n(y),
		"L" + n(x+width-m) + "," + n(y),
		"L" + n(x+width) + "," + n(y-float64(height/2)),
		"L" + n(x+width-m) + "," + n(y-height),
		"L" + n(x+m) + "," + n(y-height),
		"L" + n(x) + "," + n(y-float64(height/2)),
		"Z",
	}, " ")
}

func hexagon(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	f := 4.0
	if node.Look == "neo" {
		f = 3.5
	}
	node.LabelStyle = styles.LabelStyles
	nodePadding := ru.NN(node.Padding, 0)
	const wa = 70
	const ha = 32

	labelPaddingX, labelPaddingY := nodePadding, nodePadding
	if node.Look == "neo" {
		labelPaddingX, labelPaddingY = wa, ha
	}
	if js.Truthy(node.Width) || js.Truthy(node.Height) {
		originalHeight := ru.NN(node.Height, 0)
		m := originalHeight / f
		node.Width = ru.NN(node.Width, 0) - float64(2*m) - labelPaddingY
		node.Height = ru.NN(node.Height, 0) - labelPaddingX
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	h := bbox.Height
	if js.Truthy(node.Height) {
		h = node.Height
	}
	h += labelPaddingX
	m := h / f

	w := bbox.Width
	if js.Truthy(node.Width) {
		w = node.Width
	}
	w = w + float64(2*m) + labelPaddingY
	points := []ru.Point{
		{X: m, Y: 0},
		{X: w - m, Y: 0},
		{X: w, Y: -h / 2},
		{X: w - m, Y: -h},
		{X: m, Y: -h},
		{X: 0, Y: -h / 2},
	}

	var polygon *dom.Node

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, nil)
		pathData := createHexagonPathD(0, 0, w, h, m)
		roughNode, err := rc.Path(pathData, options)
		if err != nil {
			return nil, err
		}

		polygon = shapeSvg.
			InsertNode(roughNode, ":first-child").
			Attr("transform", "translate("+js.Str(-w/2)+", "+js.Str(h/2)+")")

		if hasCSSStyles(node) {
			polygon.Attr("style", cssStylesString(node))
		}
	} else {
		polygon = ru.InsertPolygonShape(shapeSvg, w, h, points)
	}

	if styles.NodeStyles != "" {
		polygon.Attr("style", styles.NodeStyles)
	}

	node.Width = w
	node.Height = h

	ru.UpdateNodeBounds(node, polygon)

	polygonIntersect(node, points)

	return shapeSvg, nil
}
