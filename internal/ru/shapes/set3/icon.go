// Ported from mermaid 12.2.0, rendering-elements/shapes/icon.ts (MIT).

package set3

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("icon", icon)
}

func icon(ctx *render.Context, parent *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) (*dom.Node, error) {
	opts, err := readIconOptions(renderOptions)
	if err != nil {
		return nil, err
	}
	node.LabelStyle = ru.Styles2String(node).LabelStyles
	iconSize := iconSizeOf(node)
	node.Width = js.Max(iconSize, ru.NN(opts.defaultWidth, 0))
	res, err := ru.LabelHelper(ctx, parent, node, "icon-shape default")
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, label := res.ShapeSvg, res.BBox, res.Label

	topLabel := node.Pos == "t"

	height := iconSize
	width := iconSize
	if opts.themeVariables == nil {
		return nil, errNoThemeVariables
	}
	nodeBorder := opts.themeVariables.Get("nodeBorder")
	stylesMap := ru.CompileStyles(node)

	x := -width / 2
	y := -height / 2

	labelPadding := labelPaddingOf(node)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, roughOptions("stroke", "none", "fill", "none"))

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}

	iconNode, err := rc.Rectangle(x, y, width, height, options)
	if err != nil {
		return nil, err
	}

	outerWidth := js.Max(width, bbox.Width)
	outerHeight := height + bbox.Height + labelPadding

	outerNode, err := rc.Rectangle(-outerWidth/2, -outerHeight/2, outerWidth, outerHeight, spread(options, "fill", "transparent", "stroke", "none"))
	if err != nil {
		return nil, err
	}

	iconShape := shapeSvg.InsertNode(iconNode, ":first-child")
	outerShape := shapeSvg.InsertNode(outerNode)

	if err := insertIcon(ctx, shapeSvg, node, iconSize, bbox, labelPadding, topLabel, stylesMap, nodeBorder); err != nil {
		return nil, err
	}

	placeLabelAndShape(label, iconShape, bbox, outerHeight, labelPadding, topLabel)

	ru.UpdateNodeBounds(node, outerShape)

	node.Intersect = iconIntersect(node, bbox, width, height, labelPadding, topLabel)

	return shapeSvg, nil
}
