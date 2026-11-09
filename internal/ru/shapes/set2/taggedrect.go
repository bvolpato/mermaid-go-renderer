// Ported from mermaid 12.2.0, rendering-elements/shapes/taggedRect.ts (MIT).

package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("taggedRect", taggedRect)
}

// tagRatio is the width and the height of the tag in comparison to the
// height of the node.
const tagRatio = 0.2

func taggedRect(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles

	nodePadding := ru.NN(node.Padding, 0)
	labelPaddingX := tern(node.Look == "neo", 16, nodePadding)
	labelPaddingY := tern(node.Look == "neo", 12, nodePadding)

	// When the node has a width or a height, take the padding from them:
	// the label helper does not account for the padding.
	if truthy(node.Width) || truthy(node.Height) {
		node.Height = js.Max(ru.NN(node.Height, 0)-float64(labelPaddingY*2), 10)
		node.Width = js.Max(
			ru.NN(node.Width, 0)-float64(labelPaddingX*2)-float64(tagRatio*(node.Height+float64(labelPaddingY*2))),
			10,
		)
	}

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	totalHeight := ru.Or(node.Height, bbox.Height) + float64(labelPaddingY*2)
	tagWidth := float64(tagRatio * totalHeight)
	tagHeight := float64(tagRatio * totalHeight)
	totalWidth := ru.Or(node.Width, bbox.Width) + float64(labelPaddingX*2) + tagWidth

	w := totalWidth - tagWidth
	h := totalHeight
	x := float64(-w / 2)
	y := float64(-h / 2)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, roughOptions())

	rectPoints := []ru.Point{
		{X: x - float64(tagWidth/2), Y: y},
		{X: x + w + float64(tagWidth/2), Y: y},
		{X: x + w + float64(tagWidth/2), Y: y + h},
		{X: x - float64(tagWidth/2), Y: y + h},
	}

	tagPoints := []ru.Point{
		{X: x + w - float64(tagWidth/2), Y: y + h},
		{X: x + w + float64(tagWidth/2), Y: y + h},
		{X: x + w + float64(tagWidth/2), Y: y + h - tagHeight},
	}

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	rectPath := ru.CreatePathFromPoints(rectPoints)
	rectNode, err := rc.Path(rectPath, options)
	if err != nil {
		return nil, err
	}

	tagPath := ru.CreatePathFromPoints(tagPoints)
	tagNode, err := rc.Path(tagPath, spread(options, "fillStyle", "solid"))
	if err != nil {
		return nil, err
	}

	taggedRect := shapeSvg.InsertNode(tagNode, ":first-child")
	taggedRect.InsertNode(rectNode, ":first-child")

	taggedRect.Attr("class", "basic label-container outer-path")

	stylePaths(taggedRect.SelectAll("path"), node, nodeStyles)

	ru.UpdateNodeBounds(node, taggedRect)

	node.Intersect = polygonIntersect(node, rectPoints)

	return shapeSvg, nil
}
