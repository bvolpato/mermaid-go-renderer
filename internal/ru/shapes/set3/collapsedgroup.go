// Ported from mermaid 12.2.0, rendering-elements/shapes/collapsedGroup.ts
// (MIT).

package set3

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

func init() {
	ru.RegisterShape("collapsedGroup", collapsedGroup)
}

const (
	// indicatorRowHeight is the height of the row with the ellipsis below
	// the title.
	indicatorRowHeight = 20.0
	// separatorGap is the gap between the title and the indicator row.
	separatorGap = 8.0
	// collapsedMinWidth is the minimum width of the shape.
	collapsedMinWidth = 80.0
	// collapsedRadius is the corner radius of the expanded subgraph.
	collapsedRadius = 8.0
)

// collapsedStyle is CollapsedStyle. strokeWidth is undefined (NaN) for the
// default look, which takes the stroke width of the style sheet.
type collapsedStyle struct {
	rx          float64
	fill        any
	stroke      any
	strokeWidth float64
	cssClass    string
}

// getCollapsedStyle is getCollapsedStyle(containerType): the styling of a
// collapsed container. Agentflow tags its containers, so that a collapsed
// node looks like its expanded cluster.
func getCollapsedStyle(ctx *render.Context, containerType any) collapsedStyle {
	themeVariables := js.AsObj(ctx.Cfg("themeVariables"))
	get := func(name string) any {
		if themeVariables == nil {
			return nil
		}
		return themeVariables.Get(name)
	}
	clusterBkg := get("clusterBkg")
	clusterBorder := get("clusterBorder")

	if t, ok := containerType.(string); ok && t == "flow" {
		stroke := get("flowContainerStroke")
		if !js.Truthy(stroke) {
			stroke = get("secondaryBorderColor")
		}
		return collapsedStyle{rx: 10, fill: "none", stroke: stroke, strokeWidth: 0.75, cssClass: "flow-collapsed"}
	}
	return collapsedStyle{rx: collapsedRadius, fill: clusterBkg, stroke: clusterBorder, strokeWidth: ru.Undef, cssClass: "collapsed-group"}
}

// collapsedGroup draws a collapsed subgraph (flowchart `@{ view: collapsed
// }`): the title, a separator line and three dots.
func collapsedGroup(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	var containerType any
	if node.Metadata != nil {
		containerType = node.Metadata.Get("containerType")
	}
	style := getCollapsedStyle(ctx, containerType)
	fill, stroke := style.fill, style.stroke

	nodeStyles := ru.Styles2String(node).NodeStyles

	res, err := labelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox := res.ShapeSvg, res.BBox

	// A collapsed subgraph is a container, so it takes the palette slot
	// that its expanded form has.
	ru.StampColorSlot(shapeSvg, node.ColorIndex, ctx.Cfg("theme"), ctx.Cfg("themeVariables", "borderColorArray"))

	padding := ru.NN(node.Padding, 8)
	titleHeight := bbox.Height
	totalWidth := js.Max(bbox.Width+float64(padding*2), collapsedMinWidth, ru.NN(node.Width, 0))
	totalHeight := js.Max(
		titleHeight+separatorGap+indicatorRowHeight+float64(padding*2),
		ru.NN(node.Height, 0),
	)
	x := float64(-totalWidth / 2)
	y := float64(-totalHeight / 2)

	// labelHelper centers the label at (0,0). Move it up into the title
	// area, so that the separator and the dots have room below it.
	const labelShiftY = -(separatorGap + indicatorRowHeight) / 2
	labelEl := shapeSvg.Select(".label")
	useHtmlLabels := node.UseHtmlLabels
	if !node.UseHtmlLabelsSet {
		useHtmlLabels = sanitize.GetEffectiveHtmlLabels(ctx.Config)
	}
	if useHtmlLabels {
		labelEl.Attr("transform", "translate("+js.Str(-bbox.Width/2)+", "+js.Str(float64(-bbox.Height/2)+labelShiftY)+")")
	} else {
		labelEl.Attr("transform", "translate(0, "+js.Str(float64(-bbox.Height/2)+labelShiftY)+")")
	}

	var rect *dom.Node
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		overrides := roughOptions("fill", fill, "stroke", stroke)
		if !ru.IsUndef(style.strokeWidth) {
			overrides.Set("strokeWidth", style.strokeWidth)
		}
		if f, ok := fill.(string); ok && f == "none" {
			overrides.Set("fillWeight", 0.0)
		} else {
			overrides.Set("fillStyle", "solid")
		}
		roughOpts := ru.UserNodeOverrides(ctx, node, overrides)
		roughNode, err := rc.Path(
			ru.CreateRoundedRectPathD(x, y, totalWidth, totalHeight, style.rx),
			roughOpts,
		)
		if err != nil {
			return nil, err
		}
		rect = shapeSvg.InsertNode(roughNode, ":first-child")
		rect.
			Attr("class", "basic label-container "+style.cssClass).
			Attr("style", ru.CSSStylesAttr(node))
	} else {
		rect = shapeSvg.Insert("rect", ":first-child")
		rect.
			Attr("class", "basic label-container "+style.cssClass).
			Attr("style", nodeStyles).
			Attr("rx", style.rx).
			Attr("ry", style.rx).
			Attr("x", x).
			Attr("y", y).
			Attr("width", totalWidth).
			Attr("height", totalHeight).
			Attr("fill", attrValue(fill)).
			Attr("stroke", attrValue(stroke))

		if !ru.IsUndef(style.strokeWidth) {
			rect.Attr("stroke-width", js.Str(style.strokeWidth)+"px")
		}
	}

	// The separator line between the title and the indicator row.
	separatorY := y + padding + titleHeight + separatorGap
	shapeSvg.
		Append("line").
		Attr("class", "collapsed-separator").
		Attr("x1", x+8).
		Attr("y1", separatorY).
		Attr("x2", x+totalWidth-8).
		Attr("y2", separatorY).
		Attr("stroke", attrValue(stroke)).
		Attr("stroke-dasharray", "3, 3")

	// The three dots, centered in the indicator row.
	dotY := separatorY + indicatorRowHeight/2
	const dotRadius = 2.5
	const dotSpacing = 10.0
	for i := -1.0; i <= 1; i++ {
		shapeSvg.
			Append("circle").
			Attr("class", "collapsed-indicator").
			Attr("cx", i*dotSpacing).
			Attr("cy", dotY).
			Attr("r", dotRadius).
			Attr("fill", attrValue(stroke))
	}

	ru.UpdateNodeBounds(node, rect)

	node.CalcIntersect = func(bounds ru.Bounds, point ru.Point) ru.Point {
		return ru.IntersectRect(bounds, point)
	}

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}

	return shapeSvg, nil
}
