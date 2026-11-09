// Ported from mermaid 12.2.0, rendering-elements/shapes/windowPane.ts (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("windowPane", windowPane)
}

// windowPaneRectOffset is the width of the frame on the top and the left
// of the shape.
const windowPaneRectOffset = 10.0

func windowPane(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	const rectOffset = windowPaneRectOffset
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles

	paddingX := tern(node.Look == "neo", 16, ru.NN(node.Padding, 0))
	paddingY := tern(node.Look == "neo", 12, ru.NN(node.Padding, 0))

	// When the node has a width or a height, take the padding from them:
	// the label helper does not account for the padding.
	if truthy(node.Width) || truthy(node.Height) {
		node.Width = js.Max(ru.NN(node.Width, 0)-float64(paddingX*2)-rectOffset, 10)
		node.Height = js.Max(ru.NN(node.Height, 0)-float64(paddingY*2)-rectOffset, 10)
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	totalWidth := ru.Or(node.Width, bbox.Width) + float64(paddingX*2) + rectOffset
	totalHeight := ru.Or(node.Height, bbox.Height) + float64(paddingY*2) + rectOffset

	w := totalWidth - rectOffset
	h := totalHeight - rectOffset
	x := float64(-w / 2)
	y := float64(-h / 2)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, roughOptions())

	outerPathPoints := []ru.Point{
		{X: x - rectOffset, Y: y - rectOffset},
		{X: x - rectOffset, Y: y + h},
		{X: x + w, Y: y + h},
		{X: x + w, Y: y - rectOffset},
	}

	n := js.Str
	path := "M" + n(x-rectOffset) + "," + n(y-rectOffset) + " L" + n(x+w) + "," + n(y-rectOffset) + " L" + n(x+w) + "," + n(y+h) + " L" + n(x-rectOffset) + "," + n(y+h) + " L" + n(x-rectOffset) + "," + n(y-rectOffset) + "\n" +
		"                M" + n(x-rectOffset) + "," + n(y) + " L" + n(x+w) + "," + n(y) + "\n" +
		"                M" + n(x) + "," + n(y-rectOffset) + " L" + n(x) + "," + n(y+h)

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	no, err := rc.Path(path, options)
	if err != nil {
		return nil, err
	}

	windowPane := shapeSvg.InsertNode(no, ":first-child")
	windowPane.Attr("transform", "translate("+n(rectOffset/2)+", "+n(rectOffset/2)+")")

	windowPane.Attr("class", "basic label-container outer-path")

	stylePaths(windowPane.SelectAll("path"), node, nodeStyles)

	label.Attr(
		"transform",
		"translate("+n(-float64(bbox.Width/2)+rectOffset/2-(bbox.X-bbox.LeftOr0()))+", "+
			n(-float64(bbox.Height/2)+rectOffset/2-(bbox.Y-bbox.TopOr0()))+")",
	)

	ru.UpdateNodeBounds(node, windowPane)

	node.Intersect = polygonIntersect(node, outerPathPoints)

	return shapeSvg, nil
}
