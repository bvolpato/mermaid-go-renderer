// Ported from mermaid 12.2.0, rendering-util/rendering-elements/clusters.js
// (MIT).

// Package clusters is the port of the cluster rendering of mermaid's
// unified renderer: rendering-elements/clusters.js and
// rendering-elements/clusters/swimlane.js. A cluster is a group node (a
// subgraph, a composite state, a namespace, a kanban section) that a layout
// draws as a frame around its children.
//
// The cluster shapes register themselves with ru.RegisterClusterShape under
// their upstream names: rect, squareRect, roundedWithTitle, noteGroup,
// divider, kanbanSection, flowGroup, usecaseSystemBoundary and swimlane.
//
// A cluster shape reads the laid-out box of the node (node.X, node.Y,
// node.Width, node.Height, node.Padding) and writes these fields:
//
//   - node.Diff, node.OffsetX and node.OffsetY,
//   - node.Width and node.Height (the size of the drawn frame),
//   - node.LabelBBox (roundedWithTitle and usecaseSystemBoundary only),
//   - node.Intersect (the intersection with the frame rectangle).
//
// The functions take the render context first, where upstream reads module
// state or the global config.
package clusters

import (
	"errors"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

func init() {
	// The shapes map of clusters.js.
	ru.RegisterClusterShape("rect", rect)
	ru.RegisterClusterShape("squareRect", rect)
	ru.RegisterClusterShape("roundedWithTitle", roundedWithTitle)
	ru.RegisterClusterShape("noteGroup", noteGroup)
	ru.RegisterClusterShape("divider", divider)
	ru.RegisterClusterShape("kanbanSection", kanbanSection)
	ru.RegisterClusterShape("flowGroup", flowGroup)
	ru.RegisterClusterShape("usecaseSystemBoundary", usecaseSystemBoundary)
	ru.RegisterClusterShape("swimlane", swimlane)
}

// cssClassesText is node.cssClasses in a string concatenation: an
// undefined value prints as "undefined".
func cssClassesText(node *ru.Node) string {
	if !node.CSSClassesSet && node.CSSClasses == "" {
		return "undefined"
	}
	return node.CSSClasses
}

// strAttr is a string property as the value of selection.attr: an
// undefined property (the empty string) removes the attribute.
func strAttr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// dynAttr is a dynamic value as the value of selection.attr: undefined and
// null remove the attribute.
func dynAttr(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	if f, ok := v.(float64); ok {
		return f
	}
	return js.ToString(v)
}

// domIDOrID is `node.domId ?? node.id`.
func domIDOrID(node *ru.Node) string {
	if node.DomID != "" {
		return node.DomID
	}
	return node.ID
}

// obj builds a JavaScript object from keys and values, in order.
func obj(kv ...any) *js.Obj[any] {
	o := js.NewObj[any]()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1])
	}
	return o
}

// labelBBox is the size of a created label. Every cluster shape has this
// block: the getBBox of the label, or with HTML labels the
// getBoundingClientRect of the div, which is then also the size of the
// foreignObject.
func labelBBox(text *dom.Node, useHtmlLabels bool) dom.Rect {
	if useHtmlLabels {
		div := text.FirstElementChild()
		bbox := div.BoundingClientRect()
		text.Attr("width", bbox.Width)
		text.Attr("height", bbox.Height)
		return bbox
	}
	return text.BBox()
}

// createText is createText(el, text, options) as the cluster shapes call
// it: without the config argument. The markdown functions of upstream then
// get an empty config, so `markdownAutoWrap: false` has no effect on the
// title of a cluster. ru.CreateText always uses the config of the render.
// That one config value is the only one that makes a difference, so this
// function hides it during the call.
func createText(ctx *render.Context, el *dom.Node, text string, o ru.TextOptions) (*dom.Node, error) {
	if v, ok := ctx.Config.Lookup("markdownAutoWrap"); !ok || v != false {
		return ru.CreateText(ctx, el, text, o)
	}
	saved := ctx.Config
	config := js.NewObj[any]()
	saved.Each(func(k string, v any) {
		if k != "markdownAutoWrap" {
			config.Set(k, v)
		}
	})
	ctx.Config = config
	defer func() { ctx.Config = saved }()
	return ru.CreateText(ctx, el, text, o)
}

func translate(x, y float64) string {
	return "translate(" + js.Str(x) + ", " + js.Str(y) + ")"
}

func setIntersect(node *ru.Node) {
	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}
}

func rect(ctx *render.Context, parent *dom.Node, node *ru.Node) (*ru.ClusterResult, error) {
	siteConfig := ctx.Config
	theme := ctx.Cfg("theme")
	handDrawnSeed := ctx.Cfg("handDrawnSeed")
	clusterBkg := ctx.Cfg("themeVariables", "clusterBkg")
	clusterBorder := ctx.Cfg("themeVariables", "clusterBorder")
	borderColorArray := ctx.Cfg("themeVariables", "borderColorArray")

	styles := ru.Styles2String(node)
	labelStyles, nodeStyles, borderStyles, backgroundStyles := styles.LabelStyles, styles.NodeStyles, styles.BorderStyles, styles.BackgroundStyles

	// Add outer g element.
	shapeSvg := parent.
		Insert("g").
		Attr("class", "cluster "+cssClassesText(node)).
		Attr("id", strAttr(node.DomID)).
		Attr("data-look", strAttr(node.Look))

	// The color slot of the container. It does nothing unless the theme has
	// a palette.
	ru.StampColorSlot(shapeSvg, node.ColorIndex, theme, borderColorArray)

	useHtmlLabels := sanitize.GetEffectiveHtmlLabels(siteConfig)

	// Create the label and insert it after the rect.
	labelEl := shapeSvg.Insert("g").Attr("class", "cluster-label ")

	var text *dom.Node
	var err error
	if node.LabelType == "markdown" {
		text, err = createText(ctx, labelEl, node.Label, ru.TextOptions{
			Style:         node.LabelStyle,
			UseHtmlLabels: useHtmlLabels,
			Markdown:      true,
			IsNode:        true,
			Width:         node.Width,
		})
	} else {
		text, err = ru.CreateLabel(ctx, labelEl, node.Label, node.LabelStyle, false, true)
	}
	if err != nil {
		return nil, err
	}

	// Get the size of the label.
	bbox := labelBBox(text, sanitize.GetEffectiveHtmlLabels(siteConfig))

	width := node.Width
	if node.Width <= bbox.Width+node.Padding {
		width = bbox.Width + node.Padding
	}
	if node.Width <= bbox.Width+node.Padding {
		node.Diff = float64((width-node.Width)/2) - node.Padding
	} else {
		node.Diff = -node.Padding
	}

	height := node.Height
	x := node.X - float64(width/2)
	y := node.Y - float64(height/2)

	var rect *dom.Node
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, obj(
			"roughness", 0.7,
			"fill", clusterBkg,
			"stroke", clusterBorder,
			"fillWeight", 3.0,
			"seed", handDrawnSeed,
		))
		roughNode, err := rc.Path(ru.CreateRoundedRectPathD(x, y, width, height, 0), options)
		if err != nil {
			return nil, err
		}
		rect = shapeSvg.InsertNode(roughNode, ":first-child")
		// Should we affect the options instead of doing this?
		rect.Select("path:nth-child(2)").Attr("style", strings.Join(borderStyles, ";"))
		rect.Select("path").Attr("style", strings.Replace(strings.Join(backgroundStyles, ";"), "fill", "stroke", 1))
	} else {
		// Add the rect.
		rect = shapeSvg.Insert("rect", ":first-child")
		// Center the rect around its coordinate.
		rect.
			Attr("style", nodeStyles).
			Attr("rx", ru.NumAttr(node.Rx)).
			Attr("ry", ru.NumAttr(node.Ry)).
			Attr("x", x).
			Attr("y", y).
			Attr("width", ru.NumAttr(width)).
			Attr("height", ru.NumAttr(height))
	}
	subGraphTitleTopMargin := utils.GetSubGraphTitleMargins(siteConfig).SubGraphTitleTopMargin
	// This puts the label on top of the box instead of inside it.
	labelEl.Attr("transform", translate(node.X-float64(bbox.Width/2), node.Y-float64(node.Height/2)+subGraphTitleTopMargin))

	if labelStyles != "" {
		labelEl.Select("span").Attr("style", labelStyles)
	}

	rectBox := rect.BBox()
	node.OffsetX = 0
	node.Width = rectBox.Width
	node.Height = rectBox.Height
	// Used by the layout engine to position the subgraph in its parent.
	node.OffsetY = bbox.Height - float64(node.Padding/2)

	setIntersect(node)

	return &ru.ClusterResult{Cluster: shapeSvg, LabelBBox: bbox, HasLabelBBox: true}, nil
}

// noteGroup is a cluster that is not visible: it groups a note with the
// state that the note is for.
func noteGroup(_ *render.Context, parent *dom.Node, node *ru.Node) (*ru.ClusterResult, error) {
	// Add outer g element.
	shapeSvg := parent.Insert("g").Attr("class", "note-cluster").Attr("id", strAttr(node.DomID))

	// Add the rect.
	rect := shapeSvg.Insert("rect", ":first-child")

	padding := float64(0 * node.Padding)
	halfPadding := float64(padding / 2)

	// Center the rect around its coordinate.
	rect.
		Attr("rx", ru.NumAttr(node.Rx)).
		Attr("ry", ru.NumAttr(node.Ry)).
		Attr("x", node.X-float64(node.Width/2)-halfPadding).
		Attr("y", node.Y-float64(node.Height/2)-halfPadding).
		Attr("width", node.Width+padding).
		Attr("height", node.Height+padding).
		Attr("fill", "none")

	rectBox := rect.BBox()
	node.Width = rectBox.Width
	node.Height = rectBox.Height

	setIntersect(node)

	return &ru.ClusterResult{Cluster: shapeSvg, LabelBBox: dom.Rect{Width: 0, Height: 0}, HasLabelBBox: true}, nil
}

func roundedWithTitle(ctx *render.Context, parent *dom.Node, node *ru.Node) (*ru.ClusterResult, error) {
	siteConfig := ctx.Config

	theme := ctx.Cfg("theme")
	handDrawnSeed := ctx.Cfg("handDrawnSeed")
	altBackground := ctx.Cfg("themeVariables", "altBackground")
	borderColorArray := ctx.Cfg("themeVariables", "borderColorArray")
	compositeBackground := ctx.Cfg("themeVariables", "compositeBackground")
	compositeTitleBackground := ctx.Cfg("themeVariables", "compositeTitleBackground")
	nodeBorder := ctx.Cfg("themeVariables", "nodeBorder")

	// Add outer g element.
	shapeSvg := parent.
		Insert("g").
		Attr("class", ru.StrAttr(node.CSSClasses, node.CSSClassesSet || node.CSSClasses != "")).
		Attr("id", strAttr(node.DomID)).
		Attr("data-id", node.ID).
		Attr("data-look", strAttr(node.Look))

	// The color slot of the composite. It does nothing unless the theme has
	// a palette.
	ru.StampColorSlot(shapeSvg, node.ColorIndex, theme, borderColorArray)

	// Add the rect.
	outerRectG := shapeSvg.Insert("g", ":first-child")

	// Create the label and insert it after the rect.
	label := shapeSvg.Insert("g").Attr("class", "cluster-label")
	innerRect := shapeSvg.Append("rect")

	text, err := ru.CreateLabel(ctx, label, node.Label, node.LabelStyle, false, true)
	if err != nil {
		return nil, err
	}

	// Get the size of the label.
	bbox := labelBBox(text, sanitize.GetEffectiveHtmlLabels(siteConfig))

	// Rounded With Title.
	padding := float64(0 * node.Padding)
	halfPadding := float64(padding / 2)

	width := node.Width
	if node.Width <= bbox.Width+node.Padding {
		width = bbox.Width + node.Padding
	}
	width = width + padding
	if node.Width <= bbox.Width+node.Padding {
		node.Diff = float64((width-node.Width)/2) - node.Padding
	} else {
		node.Diff = -node.Padding
	}

	height := node.Height + padding
	innerHeight := node.Height + padding - bbox.Height - 6
	x := node.X - float64(width/2)
	y := node.Y - float64(height/2)
	node.Width = width
	innerY := node.Y - float64(node.Height/2) - halfPadding + bbox.Height + 2

	// Add the rect.
	var rect *dom.Node
	if node.Look == "handDrawn" {
		if !node.CSSClassesSet && node.CSSClasses == "" {
			return nil, errors.New("Cannot read properties of undefined (reading 'includes')")
		}
		isAlt := strings.Contains(node.CSSClasses, "statediagram-cluster-alt")
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		var roughOuterNode *dom.Node
		if js.Truthy(node.Rx) || js.Truthy(node.Ry) {
			roughOuterNode, err = rc.Path(ru.CreateRoundedRectPathD(x, y, width, height, 10), obj(
				"roughness", 0.7,
				"fill", compositeTitleBackground,
				"fillStyle", "solid",
				"stroke", nodeBorder,
				"seed", handDrawnSeed,
			))
		} else {
			roughOuterNode, err = rc.Rectangle(x, y, width, height, obj("seed", handDrawnSeed))
		}
		if err != nil {
			return nil, err
		}

		rect = shapeSvg.InsertNode(roughOuterNode, ":first-child")
		var innerFill any = compositeBackground
		innerFillStyle := "solid"
		if isAlt {
			innerFill = altBackground
			innerFillStyle = "hachure"
		}
		roughInnerNode, err := rc.Rectangle(x, innerY, width, innerHeight, obj(
			"fill", innerFill,
			"fillStyle", innerFillStyle,
			"stroke", nodeBorder,
			"seed", handDrawnSeed,
		))
		if err != nil {
			return nil, err
		}

		rect = shapeSvg.InsertNode(roughOuterNode, ":first-child")
		innerRect = shapeSvg.InsertNode(roughInnerNode)
		// The classic branch gives its two rects the classes outer and inner.
		// rough wraps each shape in a g with no class, so the same names are
		// set here.
		rect.Attr("class", "outer")
		innerRect.Attr("class", "inner")
	} else {
		rect = outerRectG.Insert("rect", ":first-child")
		outerRectClass := "outer"

		// Center the rect around its coordinate.
		rect.
			Attr("class", outerRectClass).
			Attr("x", x).
			Attr("y", y).
			Attr("width", width).
			Attr("height", height).
			Attr("data-look", strAttr(node.Look))
		innerRect.
			Attr("class", "inner").
			Attr("x", x).
			Attr("y", innerY).
			Attr("width", width).
			Attr("height", innerHeight)
	}

	labelOffset := 3.0
	if sanitize.GetEffectiveHtmlLabels(siteConfig) {
		labelOffset = 0
	}
	label.Attr("transform", translate(node.X-float64(bbox.Width/2), y+1-labelOffset))

	rectBox := rect.BBox()
	node.Height = rectBox.Height
	node.OffsetX = 0
	// Used by the layout engine to position the subgraph in its parent.
	node.OffsetY = bbox.Height - float64(node.Padding/2)
	node.LabelBBox = &ru.Bounds{X: bbox.X, Y: bbox.Y, Width: bbox.Width, Height: bbox.Height}

	setIntersect(node)

	return &ru.ClusterResult{Cluster: shapeSvg, LabelBBox: bbox, HasLabelBBox: true}, nil
}

func kanbanSection(ctx *render.Context, parent *dom.Node, node *ru.Node) (*ru.ClusterResult, error) {
	siteConfig := ctx.Config
	handDrawnSeed := ctx.Cfg("handDrawnSeed")
	clusterBkg := ctx.Cfg("themeVariables", "clusterBkg")
	clusterBorder := ctx.Cfg("themeVariables", "clusterBorder")

	styles := ru.Styles2String(node)
	labelStyles, nodeStyles, borderStyles, backgroundStyles := styles.LabelStyles, styles.NodeStyles, styles.BorderStyles, styles.BackgroundStyles

	// Add outer g element.
	shapeSvg := parent.
		Insert("g").
		Attr("class", "cluster "+cssClassesText(node)).
		Attr("id", strAttr(node.DomID)).
		Attr("data-look", strAttr(node.Look))

	useHtmlLabels := sanitize.GetEffectiveHtmlLabels(siteConfig)

	// Create the label and insert it after the rect.
	labelEl := shapeSvg.Insert("g").Attr("class", "cluster-label ")

	text, err := createText(ctx, labelEl, node.Label, ru.TextOptions{
		Style:         node.LabelStyle,
		UseHtmlLabels: useHtmlLabels,
		Markdown:      true,
		IsNode:        true,
		Width:         node.Width,
	})
	if err != nil {
		return nil, err
	}

	// Get the size of the label.
	bbox := labelBBox(text, sanitize.GetEffectiveHtmlLabels(siteConfig))

	width := node.Width
	if node.Width <= bbox.Width+node.Padding {
		width = bbox.Width + node.Padding
	}
	if node.Width <= bbox.Width+node.Padding {
		node.Diff = float64((width-node.Width)/2) - node.Padding
	} else {
		node.Diff = -node.Padding
	}

	height := node.Height
	x := node.X - float64(width/2)
	y := node.Y - float64(height/2)

	var rect *dom.Node
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, obj(
			"roughness", 0.7,
			"fill", clusterBkg,
			"stroke", clusterBorder,
			"fillWeight", 4.0,
			"seed", handDrawnSeed,
		))
		roughNode, err := rc.Path(ru.CreateRoundedRectPathD(x, y, width, height, node.Rx), options)
		if err != nil {
			return nil, err
		}
		rect = shapeSvg.InsertNode(roughNode, ":first-child")
		// Should we affect the options instead of doing this?
		rect.Select("path:nth-child(2)").Attr("style", strings.Join(borderStyles, ";"))
		rect.Select("path").Attr("style", strings.Replace(strings.Join(backgroundStyles, ";"), "fill", "stroke", 1))
	} else {
		// Add the rect.
		rect = shapeSvg.Insert("rect", ":first-child")
		// Center the rect around its coordinate.
		rect.
			Attr("style", nodeStyles).
			Attr("rx", ru.NumAttr(node.Rx)).
			Attr("ry", ru.NumAttr(node.Ry)).
			Attr("x", x).
			Attr("y", y).
			Attr("width", ru.NumAttr(width)).
			Attr("height", ru.NumAttr(height))
	}
	subGraphTitleTopMargin := utils.GetSubGraphTitleMargins(siteConfig).SubGraphTitleTopMargin
	// This puts the label on top of the box instead of inside it.
	labelEl.Attr("transform", translate(node.X-float64(bbox.Width/2), node.Y-float64(node.Height/2)+subGraphTitleTopMargin))

	if labelStyles != "" {
		labelEl.Select("span").Attr("style", labelStyles)
	}

	rectBox := rect.BBox()
	node.OffsetX = 0
	node.Width = rectBox.Width
	node.Height = rectBox.Height
	// Used by the layout engine to position the subgraph in its parent.
	node.OffsetY = bbox.Height - float64(node.Padding/2)

	setIntersect(node)

	return &ru.ClusterResult{Cluster: shapeSvg, LabelBBox: bbox, HasLabelBBox: true}, nil
}

func divider(ctx *render.Context, parent *dom.Node, node *ru.Node) (*ru.ClusterResult, error) {
	theme := ctx.Cfg("theme")
	handDrawnSeed := ctx.Cfg("handDrawnSeed")
	altBackground := ctx.Cfg("themeVariables", "altBackground")
	nodeBorder := ctx.Cfg("themeVariables", "nodeBorder")
	borderColorArray := ctx.Cfg("themeVariables", "borderColorArray")

	// Sibling regions of one composite share a slot.
	shapeSvg := parent.
		Insert("g").
		Attr("class", ru.StrAttr(node.CSSClasses, node.CSSClassesSet || node.CSSClasses != "")).
		Attr("id", strAttr(node.DomID)).
		Attr("data-look", strAttr(node.Look))

	ru.StampColorSlot(shapeSvg, node.ColorIndex, theme, borderColorArray)

	// Add the rect.
	outerRectG := shapeSvg.Insert("g", ":first-child")

	padding := float64(0 * node.Padding)

	width := node.Width + padding

	node.Diff = -node.Padding

	height := node.Height + padding
	x := node.X - float64(width/2)
	y := node.Y - float64(height/2)
	node.Width = width

	// Add the rect.
	var rect *dom.Node
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		// The fill is the value of the theme, with the fallback that
		// rect.divider gets from CSS under the other looks. The fill is solid:
		// a hachure fill is drawn as stroked lines, and then a style sheet
		// cannot tell the fill from the outline.
		var fill any = altBackground
		if js.IsNullish(fill) {
			fill = "#efefef"
		}
		roughOuterNode, err := rc.Rectangle(x, y, width, height, obj(
			"fill", fill,
			"fillStyle", "solid",
			"roughness", 0.5,
			"strokeLineDash", []any{5.0},
			"stroke", nodeBorder,
			"seed", handDrawnSeed,
		))
		if err != nil {
			return nil, err
		}

		rect = shapeSvg.InsertNode(roughOuterNode, ":first-child").Attr("class", "divider")
	} else {
		rect = outerRectG.Insert("rect", ":first-child")
		outerRectClass := "divider"

		// Center the rect around its coordinate.
		rect.
			Attr("class", outerRectClass).
			Attr("x", x).
			Attr("y", y).
			Attr("width", width).
			Attr("height", height).
			Attr("data-look", strAttr(node.Look))
	}

	rectBox := rect.BBox()
	node.Height = rectBox.Height
	node.OffsetX = 0
	// Used by the layout engine to position the subgraph in its parent.
	node.OffsetY = 0

	setIntersect(node)

	return &ru.ClusterResult{Cluster: shapeSvg}, nil
}

// containerGroupOptions are the options of createContainerGroup.
type containerGroupOptions struct {
	// cssClass is the CSS class suffix, for example "flow-cluster".
	cssClass string
	// rx is the corner radius.
	rx float64
	// fill is the fill color, or "none" or "transparent".
	fill string
	// stroke is the stroke color.
	stroke any
	// strokeWidth is the stroke width in px.
	strokeWidth float64
	// strokeDash is an optional dash array.
	strokeDash []float64
	// roughness is the roughness for rough.
	roughness float64
}

// createContainerGroup is the shared helper for the container cluster
// shapes of agentflow. It draws a labeled rounded rect.
func createContainerGroup(ctx *render.Context, parent *dom.Node, node *ru.Node, opts containerGroupOptions) (*ru.ClusterResult, error) {
	siteConfig := ctx.Config
	theme := ctx.Cfg("theme")
	handDrawnSeed := ctx.Cfg("handDrawnSeed")
	borderColorArray := ctx.Cfg("themeVariables", "borderColorArray")

	styles := ru.Styles2String(node)
	labelStyles, borderStyles := styles.LabelStyles, styles.BorderStyles

	shapeSvg := parent.
		Insert("g").
		Attr("class", "cluster "+opts.cssClass+" "+cssClassesText(node)).
		// domId is made per render. The id of the author is not unique across
		// the diagrams of a page.
		Attr("id", domIDOrID(node)).
		Attr("data-look", strAttr(node.Look))

	// The color slot of the container, as in rect.
	ru.StampColorSlot(shapeSvg, node.ColorIndex, theme, borderColorArray)

	useHtmlLabels := sanitize.GetEffectiveHtmlLabels(siteConfig)
	labelEl := shapeSvg.Insert("g").Attr("class", "cluster-label")

	bbox := dom.Rect{Width: 0, Height: 0}
	hasLabel := js.TrimSpace(node.Label) != ""
	if hasLabel {
		var text *dom.Node
		var err error
		if node.LabelType == "markdown" {
			text, err = createText(ctx, labelEl, node.Label, ru.TextOptions{
				Style:         node.LabelStyle,
				UseHtmlLabels: useHtmlLabels,
				Markdown:      true,
				IsNode:        true,
				Width:         node.Width,
			})
		} else {
			text, err = ru.CreateLabel(ctx, labelEl, node.Label, node.LabelStyle, false, true)
		}
		if err != nil {
			return nil, err
		}
		bbox = labelBBox(text, useHtmlLabels)
	}

	width := node.Width
	if node.Width <= bbox.Width+node.Padding {
		width = bbox.Width + node.Padding
	}
	if node.Width <= bbox.Width+node.Padding {
		node.Diff = float64((width-node.Width)/2) - node.Padding
	} else {
		node.Diff = -node.Padding
	}

	height := node.Height
	x := node.X - float64(width/2)
	y := node.Y - float64(height/2)

	var rectEl *dom.Node
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		o := obj(
			"roughness", opts.roughness,
			"fill", opts.fill,
			"stroke", opts.stroke,
			"strokeWidth", opts.strokeWidth,
			"seed", handDrawnSeed,
		)
		if opts.fill == "none" {
			o.Set("fillWeight", 0.0)
		} else {
			o.Set("fillStyle", "solid")
		}
		if opts.strokeDash != nil {
			dash := make([]any, len(opts.strokeDash))
			for i, d := range opts.strokeDash {
				dash[i] = d
			}
			o.Set("strokeLineDash", dash)
		}
		roughOpts := ru.UserNodeOverrides(ctx, node, o)
		roughNode, err := rc.Path(ru.CreateRoundedRectPathD(x, y, width, height, opts.rx), roughOpts)
		if err != nil {
			return nil, err
		}
		rectEl = shapeSvg.InsertNode(roughNode, ":first-child")
		rectEl.Select("path:nth-child(2)").Attr("style", strings.Join(borderStyles, ";"))
	} else {
		rectEl = shapeSvg.Insert("rect", ":first-child")
		rectEl.
			Attr("rx", opts.rx).
			Attr("ry", opts.rx).
			Attr("x", x).
			Attr("y", y).
			Attr("width", ru.NumAttr(width)).
			Attr("height", ru.NumAttr(height)).
			Attr("fill", opts.fill).
			Attr("stroke", dynAttr(opts.stroke)).
			Attr("stroke-width", js.Str(opts.strokeWidth)+"px")
		if opts.strokeDash != nil {
			parts := make([]string, len(opts.strokeDash))
			for i, d := range opts.strokeDash {
				parts[i] = js.Str(d)
			}
			rectEl.Attr("stroke-dasharray", strings.Join(parts, ", "))
		}
	}

	if hasLabel {
		subGraphTitleTopMargin := utils.GetSubGraphTitleMargins(siteConfig).SubGraphTitleTopMargin
		labelEl.Attr("transform", translate(node.X-float64(bbox.Width/2), node.Y-float64(node.Height/2)+subGraphTitleTopMargin))
		if labelStyles != "" {
			labelEl.Select("span").Attr("style", labelStyles)
		}
	}

	rectBox := rectEl.BBox()
	node.OffsetX = 0
	node.Width = rectBox.Width
	node.Height = rectBox.Height
	node.OffsetY = bbox.Height - float64(node.Padding/2)

	setIntersect(node)

	return &ru.ClusterResult{Cluster: shapeSvg, LabelBBox: bbox, HasLabelBBox: true}, nil
}

// flowGroup is the flow group of agentflow: a grouping that is
// transparent, with a solid 0.75px border and rx=10.
func flowGroup(ctx *render.Context, parent *dom.Node, node *ru.Node) (*ru.ClusterResult, error) {
	stroke := ctx.Cfg("themeVariables", "flowContainerStroke")
	if !js.Truthy(stroke) {
		stroke = ctx.Cfg("themeVariables", "secondaryBorderColor")
	}
	result, err := createContainerGroup(ctx, parent, node, containerGroupOptions{
		cssClass:    "flow-cluster",
		rx:          10,
		fill:        "none",
		stroke:      stroke,
		strokeWidth: 0.75,
		roughness:   0.7,
	})
	if err != nil {
		return nil, err
	}
	// flowGroup returns the promise of createContainerGroup and does not
	// await it. Its own promise then settles one turn of the microtask
	// queue after that promise. When createContainerGroup had nothing to
	// await (a cluster without a label), its promise is already settled at
	// the return, and following it takes two turns.
	if js.TrimSpace(node.Label) != "" {
		ctx.Await()
	} else {
		ctx.AwaitN(2)
	}
	return result, nil
}

// UsecaseSystemBoundaryGeometry is the result of
// getUsecaseSystemBoundaryGeometry.
type UsecaseSystemBoundaryGeometry struct {
	// BoundaryType is "rect" or "package".
	BoundaryType string
	Width        float64
	Height       float64
	// X and Y are the top left corner of the boundary.
	X float64
	Y float64
	// BodyY and BodyHeight are the frame below the tab of a package.
	BodyY      float64
	BodyHeight float64
	TabHeight  float64
	TabWidth   float64
}

// boundaryTypeOf is `node.boundaryType || 'rect'`.
func boundaryTypeOf(node *ru.Node) string {
	if v := node.Get("boundaryType"); js.Truthy(v) {
		return js.ToString(v)
	}
	return "rect"
}

// GetUsecaseSystemBoundaryGeometry is
// getUsecaseSystemBoundaryGeometry(node, labelBBox): the frame of a use
// case system boundary for a node box and the measured size of its label.
// The boundary type is the extra property "boundaryType" of the node.
func GetUsecaseSystemBoundaryGeometry(node *ru.Node, labelBBox dom.Rect) UsecaseSystemBoundaryGeometry {
	boundaryType := boundaryTypeOf(node)
	isPackage := boundaryType == "package"
	horizontalLabelPadding := node.Padding
	if isPackage {
		horizontalLabelPadding = 20
	}
	tabHeight := 0.0
	if isPackage {
		tabHeight = labelBBox.Height + 10
	}
	width := js.Max(node.Width, labelBBox.Width+horizontalLabelPadding)
	height := node.Height
	if isPackage {
		height = js.Max(node.Height, tabHeight+float64(node.Padding*2))
	}
	x := node.X - float64(width/2)
	y := node.Y - float64(height/2)

	tabWidth := 0.0
	if isPackage {
		tabWidth = js.Min(width, js.Max(80, labelBBox.Width+20))
	}
	return UsecaseSystemBoundaryGeometry{
		BoundaryType: boundaryType,
		Width:        width,
		Height:       height,
		X:            x,
		Y:            y,
		BodyY:        y + tabHeight,
		BodyHeight:   height - tabHeight,
		TabHeight:    tabHeight,
		TabWidth:     tabWidth,
	}
}

// usecaseSystemBoundary is the cluster shape for the system boundaries of
// a use case diagram.
func usecaseSystemBoundary(ctx *render.Context, parent *dom.Node, node *ru.Node) (*ru.ClusterResult, error) {
	siteConfig := ctx.Config
	theme := ctx.Cfg("theme")
	handDrawnSeed := ctx.Cfg("handDrawnSeed")
	clusterBkg := ctx.Cfg("themeVariables", "clusterBkg")
	clusterBorder := ctx.Cfg("themeVariables", "clusterBorder")
	borderColorArray := ctx.Cfg("themeVariables", "borderColorArray")
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	stylesMap := ru.CompileStyles(node)

	boundaryType := boundaryTypeOf(node)
	shapeSvg := parent.
		Insert("g").
		Attr("class", "cluster usecase-system-boundary usecase-system-boundary-"+boundaryType+" "+cssClassesText(node)).
		Attr("id", domIDOrID(node)).
		Attr("data-boundary-type", boundaryType).
		Attr("data-look", strAttr(node.Look))

	// The color slot of the container, as in rect. usecase/styles.ts has the
	// rules that paint it.
	ru.StampColorSlot(shapeSvg, node.ColorIndex, theme, borderColorArray)

	useHtmlLabels := sanitize.GetEffectiveHtmlLabels(siteConfig)
	labelEl := shapeSvg.Insert("g").Attr("class", "cluster-label system-boundary-title")
	text, err := createText(ctx, labelEl, node.Label, ru.TextOptions{
		Style:         labelStyles,
		UseHtmlLabels: useHtmlLabels,
		Markdown:      true,
		IsNode:        true,
		Width:         200,
	})
	if err != nil {
		return nil, err
	}

	bbox := labelBBox(text, useHtmlLabels)

	geometry := GetUsecaseSystemBoundaryGeometry(node, bbox)
	width, height, x, y := geometry.Width, geometry.Height, geometry.X, geometry.Y
	bodyY, bodyHeight, tabHeight, tabWidth := geometry.BodyY, geometry.BodyHeight, geometry.TabHeight, geometry.TabWidth
	if node.Width <= bbox.Width+node.Padding {
		node.Diff = float64((width-node.Width)/2) - node.Padding
	} else {
		node.Diff = -node.Padding
	}

	// stylesMap.get(key) || fallback.
	styleOr := func(key string, fallback any) any {
		if v, ok := stylesMap.Get(key); ok && v != "" {
			return v
		}
		return fallback
	}
	var strokeWidth any = 1.0
	if v, ok := stylesMap.Get("stroke-width"); ok {
		if s := strings.Replace(v, "px", "", 1); s != "" {
			strokeWidth = s
		}
	}
	roughOptions := ru.UserNodeOverrides(ctx, node, obj(
		"fill", styleOr("fill", clusterBkg),
		"stroke", styleOr("stroke", clusterBorder),
		"strokeWidth", strokeWidth,
		"seed", handDrawnSeed,
	))

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		roughBody, err := rc.Rectangle(x, bodyY, width, bodyHeight, roughOptions)
		if err != nil {
			return nil, err
		}
		shapeSvg.InsertNode(roughBody, ":first-child").Attr("class", "boundary-body label-container")

		if boundaryType == "package" {
			roughTab, err := rc.Rectangle(x, y, tabWidth, tabHeight, roughOptions)
			if err != nil {
				return nil, err
			}
			shapeSvg.
				InsertNode(roughTab, ":first-child").
				Attr("class", "boundary-tab system-boundary-package-tab label-container")
		}
	} else {
		shapeSvg.
			Insert("rect", ":first-child").
			Attr("class", "boundary-body label-container").
			Attr("style", nodeStyles).
			Attr("x", x).
			Attr("y", bodyY).
			Attr("width", width).
			Attr("height", bodyHeight)

		if boundaryType == "package" {
			shapeSvg.
				Insert("rect", ":first-child").
				Attr("class", "boundary-tab system-boundary-package-tab label-container").
				Attr("style", nodeStyles).
				Attr("x", x).
				Attr("y", y).
				Attr("width", tabWidth).
				Attr("height", tabHeight)
		}
	}

	subGraphTitleTopMargin := utils.GetSubGraphTitleMargins(siteConfig).SubGraphTitleTopMargin
	if boundaryType == "package" {
		labelEl.Attr("transform", translate(x+float64((tabWidth-bbox.Width)/2), y+float64((tabHeight-bbox.Height)/2)))
	} else {
		labelEl.Attr("transform", translate(node.X-float64(bbox.Width/2), y+subGraphTitleTopMargin))
	}

	if labelStyles != "" {
		labelEl.Attr("style", labelStyles)
		labelEl.Select("span").Attr("style", labelStyles)
	}

	node.OffsetX = 0
	node.Width = width
	node.Height = height
	node.OffsetY = bbox.Height - float64(node.Padding/2)
	node.LabelBBox = &ru.Bounds{X: bbox.X, Y: bbox.Y, Width: bbox.Width, Height: bbox.Height}
	setIntersect(node)

	return &ru.ClusterResult{Cluster: shapeSvg, LabelBBox: bbox, HasLabelBBox: true}, nil
}

// ClusterPaintsTitle is clusterPaintsTitle(shape): whether a cluster shape
// paints its label as a title strip above its children. An empty shape
// stands for undefined and is the default shape "rect".
func ClusterPaintsTitle(shape string) bool {
	// UNTITLED_CLUSTER_SHAPES. A note group's label is the text of the note,
	// which the note node inside it paints.
	switch shape {
	case "noteGroup", "divider":
		return false
	}
	return true
}

// clusterElems is the module-level Map clusterElems of clusters.js, kept
// per render: node id to the result of the cluster shape.
func clusterElems(ctx *render.Context) map[string]*ru.ClusterResult {
	return ctx.Value("ru.clusters.clusterElems", func() any { return map[string]*ru.ClusterResult{} }).(map[string]*ru.ClusterResult)
}

// isClusterShape reports whether shape is a key of the shapes map.
func isClusterShape(shape string) bool {
	switch shape {
	case "rect", "squareRect", "roundedWithTitle", "noteGroup", "divider", "kanbanSection",
		"flowGroup", "usecaseSystemBoundary", "swimlane":
		return true
	}
	return false
}

// InsertCluster is insertCluster(elem, node): it draws the cluster of a
// group node into elem with the shape of the node (node.Shape, or "rect"
// when it is empty) and records the result for PositionCluster.
//
// The node must have its laid-out box: X, Y, Width and Height. The result
// has the group of the cluster and the measured box of its label. See the
// package comment for the node fields that the shape writes.
//
// A shape that mermaid does not have is an error (upstream throws a
// TypeError). A shape that is not ported yet is a *ru.NotPortedError.
func InsertCluster(ctx *render.Context, elem *dom.Node, node *ru.Node) (*ru.ClusterResult, error) {
	shape := node.Shape
	if shape == "" {
		shape = "rect"
	}
	if !isClusterShape(shape) {
		return nil, errors.New("shapes[shape] is not a function")
	}
	handler, err := ru.LookupClusterShape(shape)
	if err != nil {
		return nil, err
	}
	cluster, err := handler(ctx, elem, node)
	if err != nil {
		return nil, err
	}
	// await shapes[shape](elem, node). A shape that is not async also takes
	// one turn of the microtask queue here.
	ctx.Await()
	clusterElems(ctx)[node.ID] = cluster
	// The await of the caller. See render/async.go for the convention.
	ctx.Await()
	return cluster, nil
}

// GetClusterTitleWidth is getClusterTitleWidth(elem, node): the width of
// the title of a cluster. It creates the label in elem, measures it and
// removes it again.
//
// Upstream does not await createLabel here, so its function throws a
// TypeError on every call, and nothing calls it. This port does what the
// function is written to do.
func GetClusterTitleWidth(ctx *render.Context, elem *dom.Node, node *ru.Node) (float64, error) {
	label, err := ru.CreateLabel(ctx, elem, node.Label, node.LabelStyle, false, true)
	if err != nil {
		return 0, err
	}
	width := label.BBox().Width
	label.Remove()
	return width, nil
}

// Clear is clear(): it forgets the clusters that InsertCluster recorded.
func Clear(ctx *render.Context) {
	clear(clusterElems(ctx))
}

// PositionCluster is positionCluster(node): it moves the group of a cluster
// that InsertCluster drew to (node.X, node.Y). A node without a recorded
// cluster is an error (upstream throws a TypeError).
func PositionCluster(ctx *render.Context, node *ru.Node) error {
	el := clusterElems(ctx)[node.ID]
	if el == nil {
		return errors.New("Cannot read properties of undefined (reading 'cluster')")
	}
	el.Cluster.Attr("transform", "translate("+js.Str(node.X)+", "+js.Str(node.Y)+")")
	return nil
}
