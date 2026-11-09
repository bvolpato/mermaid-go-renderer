// This file ports no upstream file. It has the code that icon.ts,
// iconSquare.ts, iconRounded.ts, iconCircle.ts and imageSquare.ts of
// rendering-elements/shapes repeat word for word.

package set3

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// errNoThemeVariables is the TypeError of `const { nodeBorder } =
// themeVariables` for a config that has no theme variables.
var errNoThemeVariables = errors.New("Cannot destructure property 'nodeBorder' of 'themeVariables' as it is undefined.")

// iconOptions are the values that the icon shapes read from their third
// parameter, which is destructured: { config: { themeVariables, flowchart } }.
type iconOptions struct {
	themeVariables *js.Obj[any]
	// defaultWidth is flowchart?.wrappingWidth, or undefined (NaN).
	defaultWidth float64
}

func readIconOptions(renderOptions *ru.ShapeRenderOptions) (iconOptions, error) {
	if renderOptions == nil || renderOptions.Config == nil {
		return iconOptions{}, errors.New("Cannot destructure property 'themeVariables' of 'config' as it is undefined.")
	}
	o := iconOptions{
		themeVariables: js.AsObj(renderOptions.Config.Get("themeVariables")),
		defaultWidth:   ru.Undef,
	}
	if v := js.Path(renderOptions.Config, "flowchart", "wrappingWidth"); !js.IsNullish(v) {
		o.defaultWidth = js.ToNumber(v)
	}
	return o, nil
}

// iconSizeOf is the start of the icon shapes:
//
//	const assetHeight = node.assetHeight ?? 48;
//	const assetWidth = node.assetWidth ?? 48;
//	const iconSize = Math.max(assetHeight, assetWidth);
func iconSizeOf(node *ru.Node) float64 {
	assetHeight := ru.NN(node.AssetHeight, 48)
	assetWidth := ru.NN(node.AssetWidth, 48)
	return js.Max(assetHeight, assetWidth)
}

// labelPaddingOf is `node.label ? 8 : 0`.
func labelPaddingOf(node *ru.Node) float64 {
	if node.Label != "" {
		return 8
	}
	return 0
}

// styleOr is `stylesMap.get(key) ?? fallback`: the fallback when the style
// is absent or has no value.
func styleOr(stylesMap *ru.StyleMap, key string, fallback any) any {
	if v, ok := stylesMap.Get(key); ok {
		return v
	}
	return fallback
}

// setIconContent is iconElem.html(`<g>${await getIconSVG(node.icon, {
// height: iconSize, width: iconSize, fallbackPrefix: ” })}</g>`).
func setIconContent(ctx *render.Context, iconElem *dom.Node, node *ru.Node, iconSize float64) error {
	svg, err := ru.GetIconSVG(ctx, node.Icon, map[string]any{
		"height":         iconSize,
		"width":          iconSize,
		"fallbackPrefix": "",
	}, nil)
	if err != nil {
		return err
	}
	iconElem.HTML("<g>" + svg + "</g>")
	return nil
}

// iconTranslate is the transform of the icon element:
//
//	`translate(${-iconWidth / 2 - iconX},${
//	  topLabel
//	    ? bbox.height / 2 + labelPadding / 2 - iconHeight / 2 - iconY
//	    : -bbox.height / 2 - labelPadding / 2 - iconHeight / 2 - iconY
//	})`
func iconTranslate(iconBBox dom.Rect, bbox ru.LabelBBox, labelPadding float64, topLabel bool) string {
	iconWidth := iconBBox.Width
	iconHeight := iconBBox.Height
	iconX := iconBBox.X
	iconY := iconBBox.Y
	var y float64
	if topLabel {
		y = float64(bbox.Height/2) + float64(labelPadding/2) - float64(iconHeight/2) - iconY
	} else {
		y = float64(-bbox.Height/2) - float64(labelPadding/2) - float64(iconHeight/2) - iconY
	}
	return "translate(" + js.Str(float64(-iconWidth/2)-iconX) + "," + js.Str(y) + ")"
}

// insertIcon is the block of icon.ts, iconSquare.ts and iconRounded.ts that
// adds the icon:
//
//	if (node.icon) {
//	  const iconElem = shapeSvg.append('g');
//	  iconElem.html(...);
//	  const iconBBox = iconElem.node()!.getBBox();
//	  iconElem.attr('transform', ...);
//	  iconElem.attr('style', `color: ${stylesMap.get('stroke') ?? nodeBorder};`);
//	}
func insertIcon(ctx *render.Context, shapeSvg *dom.Node, node *ru.Node, iconSize float64, bbox ru.LabelBBox, labelPadding float64, topLabel bool, stylesMap *ru.StyleMap, nodeBorder any) error {
	if node.Icon == "" {
		return nil
	}
	iconElem := shapeSvg.Append("g")
	if err := setIconContent(ctx, iconElem, node, iconSize); err != nil {
		return err
	}
	iconBBox := iconElem.BBox()
	iconElem.Attr("transform", iconTranslate(iconBBox, bbox, labelPadding, topLabel))
	iconElem.Attr("style", "color: "+js.ToString(styleOr(stylesMap, "stroke", nodeBorder))+";")
	return nil
}

// placeLabelAndShape is the end of icon.ts, iconSquare.ts, iconRounded.ts
// and iconCircle.ts: it moves the label below or above the icon, and the
// icon shape to the other side.
//
//	label.attr('transform', `translate(${-bbox.width / 2 - (bbox.x - (bbox.left ?? 0))},${
//	  topLabel ? -outerHeight / 2 : outerHeight / 2 - bbox.height
//	})`);
//	iconShape.attr('transform', `translate(${0},${
//	  topLabel ? bbox.height / 2 + labelPadding / 2 : -bbox.height / 2 - labelPadding / 2
//	})`);
func placeLabelAndShape(label, iconShape *dom.Node, bbox ru.LabelBBox, outerHeight, labelPadding float64, topLabel bool) {
	var labelY float64
	if topLabel {
		labelY = -outerHeight / 2
	} else {
		labelY = float64(outerHeight/2) - bbox.Height
	}
	label.Attr("transform", "translate("+js.Str(labelOffsetX(bbox))+","+js.Str(labelY)+")")

	iconShape.Attr("transform", "translate("+js.Str(0)+","+js.Str(iconShapeOffsetY(bbox, labelPadding, topLabel))+")")
}

// iconShapeOffsetY is `topLabel ? bbox.height / 2 + labelPadding / 2 :
// -bbox.height / 2 - labelPadding / 2`.
func iconShapeOffsetY(bbox ru.LabelBBox, labelPadding float64, topLabel bool) float64 {
	if topLabel {
		return float64(bbox.Height/2) + float64(labelPadding/2)
	}
	return float64(-bbox.Height/2) - float64(labelPadding/2)
}

// iconIntersect is the intersect function of icon.ts, iconSquare.ts,
// iconRounded.ts and imageSquare.ts: the outline of the icon box and of
// the label. width and height are the size of the icon box.
func iconIntersect(node *ru.Node, bbox ru.LabelBBox, width, height, labelPadding float64, topLabel bool) func(point ru.Point) ru.Point {
	return func(point ru.Point) ru.Point {
		if node.Label == "" {
			return ru.IntersectRect(ru.NodeBounds(node), point)
		}
		dx := ru.NN(node.X, 0)
		dy := ru.NN(node.Y, 0)
		nodeHeight := ru.NN(node.Height, 0)
		var points []ru.Point
		if topLabel {
			points = []ru.Point{
				{X: dx - float64(bbox.Width/2), Y: dy - float64(nodeHeight/2)},
				{X: dx + float64(bbox.Width/2), Y: dy - float64(nodeHeight/2)},
				{X: dx + float64(bbox.Width/2), Y: dy - float64(nodeHeight/2) + bbox.Height + labelPadding},
				{X: dx + float64(width/2), Y: dy - float64(nodeHeight/2) + bbox.Height + labelPadding},
				{X: dx + float64(width/2), Y: dy + float64(nodeHeight/2)},
				{X: dx - float64(width/2), Y: dy + float64(nodeHeight/2)},
				{X: dx - float64(width/2), Y: dy - float64(nodeHeight/2) + bbox.Height + labelPadding},
				{X: dx - float64(bbox.Width/2), Y: dy - float64(nodeHeight/2) + bbox.Height + labelPadding},
			}
		} else {
			points = []ru.Point{
				{X: dx - float64(width/2), Y: dy - float64(nodeHeight/2)},
				{X: dx + float64(width/2), Y: dy - float64(nodeHeight/2)},
				{X: dx + float64(width/2), Y: dy - float64(nodeHeight/2) + height},
				{X: dx + float64(bbox.Width/2), Y: dy - float64(nodeHeight/2) + height},
				{X: dx + float64(bbox.Width/2/2), Y: dy + float64(nodeHeight/2)},
				{X: dx - float64(bbox.Width/2), Y: dy + float64(nodeHeight/2)},
				{X: dx - float64(bbox.Width/2), Y: dy - float64(nodeHeight/2) + height},
				{X: dx - float64(width/2), Y: dy - float64(nodeHeight/2) + height},
			}
		}

		return ru.IntersectPolygon(ru.NodeBounds(node), points, point)
	}
}
