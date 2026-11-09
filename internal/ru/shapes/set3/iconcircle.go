// Ported from mermaid 12.2.0, rendering-elements/shapes/iconCircle.ts (MIT).

package set3

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("iconCircle", iconCircle)
}

func iconCircle(ctx *render.Context, parent *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) (*dom.Node, error) {
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

	const padding = 20.0
	labelPadding := labelPaddingOf(node)

	topLabel := node.Pos == "t"

	if opts.themeVariables == nil {
		return nil, errNoThemeVariables
	}
	nodeBorder, mainBkg := opts.themeVariables.Get("nodeBorder"), opts.themeVariables.Get("mainBkg")
	stylesMap := ru.CompileStyles(node)
	rc := ru.NewRoughSVG(ctx, shapeSvg)
	options := ru.UserNodeOverrides(ctx, node, roughOptions())

	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}
	// const fill = stylesMap.get('fill'); options.stroke = fill ?? mainBkg;
	options.Set("stroke", styleOr(stylesMap, "fill", mainBkg))

	iconElem := shapeSvg.Append("g")
	if node.Icon != "" {
		if err := setIconContent(ctx, iconElem, node, iconSize); err != nil {
			return nil, err
		}
	}
	iconBBox := iconElem.BBox()
	iconWidth := iconBBox.Width
	iconHeight := iconBBox.Height

	diameter := float64(js.Max(iconWidth, iconHeight)*math.Sqrt2) + padding*2
	iconNode, err := rc.Circle(0, 0, diameter, options)
	if err != nil {
		return nil, err
	}

	outerWidth := js.Max(diameter, bbox.Width)
	outerHeight := diameter + bbox.Height + labelPadding

	outerNode, err := rc.Rectangle(-outerWidth/2, -outerHeight/2, outerWidth, outerHeight, spread(options, "fill", "transparent", "stroke", "none"))
	if err != nil {
		return nil, err
	}

	iconShape := shapeSvg.InsertNode(iconNode, ":first-child")
	outerShape := shapeSvg.InsertNode(outerNode)
	iconElem.Attr("transform", iconTranslate(iconBBox, bbox, labelPadding, topLabel))
	iconElem.Attr("style", "color: "+js.ToString(styleOr(stylesMap, "stroke", nodeBorder))+";")

	placeLabelAndShape(label, iconShape, bbox, outerHeight, labelPadding, topLabel)

	ru.UpdateNodeBounds(node, outerShape)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}

	return shapeSvg, nil
}
