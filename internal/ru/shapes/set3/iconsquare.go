// Ported from mermaid 12.2.0, rendering-elements/shapes/iconSquare.ts (MIT).

package set3

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("iconSquare", iconSquare)
}

func iconSquare(ctx *render.Context, parent *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) (*dom.Node, error) {
	return iconBox(ctx, parent, node, renderOptions, 0.1, "")
}

// iconBox is the body of iconSquare.ts and of iconRounded.ts. The two
// files differ in the corner radius of the icon box, and iconRounded sets
// a class on it.
func iconBox(ctx *render.Context, parent *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions, radius float64, shapeClass string) (*dom.Node, error) {
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
	shapeSvg, bbox, halfPadding, label := res.ShapeSvg, res.BBox, res.HalfPadding, res.Label

	topLabel := node.Pos == "t"

	height := iconSize + float64(halfPadding*2)
	width := iconSize + float64(halfPadding*2)
	if opts.themeVariables == nil {
		return nil, errNoThemeVariables
	}
	nodeBorder, mainBkg := opts.themeVariables.Get("nodeBorder"), opts.themeVariables.Get("mainBkg")
	stylesMap := ru.CompileStyles(node)

	x := -width / 2
	y := -height / 2

	labelPadding := labelPaddingOf(node)

	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, roughOptions())

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}
	// const fill = stylesMap.get('fill'); options.stroke = fill ?? mainBkg;
	options.Set("stroke", styleOr(stylesMap, "fill", mainBkg))

	iconNode, err := rc.Path(ru.CreateRoundedRectPathD(x, y, width, height, radius), options)
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
	if shapeClass != "" {
		iconShape.Attr("class", shapeClass)
	}
	outerShape := shapeSvg.InsertNode(outerNode)

	if err := insertIcon(ctx, shapeSvg, node, iconSize, bbox, labelPadding, topLabel, stylesMap, nodeBorder); err != nil {
		return nil, err
	}

	placeLabelAndShape(label, iconShape, bbox, outerHeight, labelPadding, topLabel)

	ru.UpdateNodeBounds(node, outerShape)

	node.Intersect = iconIntersect(node, bbox, width, height, labelPadding, topLabel)

	return shapeSvg, nil
}
