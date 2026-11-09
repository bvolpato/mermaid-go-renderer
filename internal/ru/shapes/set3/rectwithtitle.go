// Ported from mermaid 12.2.0, rendering-elements/shapes/rectWithTitle.ts
// (MIT).

package set3

import (
	"errors"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

func init() {
	ru.RegisterShape("rectWithTitle", rectWithTitle)
}

func rectWithTitle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	var classes string
	if node.CSSClasses == "" {
		classes = "node default"
	} else {
		classes = "node " + node.CSSClasses
	}

	// Add the outer g element.
	shapeSvg := parent.
		Insert("g").
		Attr("class", classes).
		Attr("id", nodeID(node))

	// Create the title label and insert it after the rect.
	g := shapeSvg.Insert("g")

	label := shapeSvg.Insert("g").Attr("class", "label").Attr("style", nodeStyles)

	description := node.Description

	title := node.Label

	text, err := ru.CreateLabel(ctx, label, title, node.LabelStyle, true, true)
	if err != nil {
		return nil, err
	}
	var bbox dom.Rect
	if sanitize.GetEffectiveHtmlLabels(ctx.Config) {
		div := text.FirstElementChild()
		bbox = div.BoundingClientRect()
		text.Attr("width", bbox.Width)
		text.Attr("height", bbox.Height)
	}
	textRows := description
	titleBox := text.BBox()
	descr, err := ru.CreateLabel(ctx, label, strings.Join(textRows, "<br/>"), node.LabelStyle, true, true)
	if err != nil {
		return nil, err
	}

	// Upstream reads the first child with or without HTML labels. An SVG
	// label with no text has no child.
	div := descr.FirstElementChild()
	if div == nil {
		return nil, errors.New("Cannot read properties of undefined (reading 'getBoundingClientRect')")
	}
	bbox = div.BoundingClientRect()
	descr.Attr("width", bbox.Width)
	descr.Attr("height", bbox.Height)

	halfPadding := float64(ru.Or(node.Padding, 0) / 2)
	descrX := 0.0
	if !(bbox.Width > titleBox.Width) {
		descrX = (titleBox.Width - bbox.Width) / 2
	}
	descr.Attr(
		"transform",
		"translate( "+js.Str(descrX)+", "+js.Str(titleBox.Height+halfPadding+5)+")",
	)
	textX := 0.0
	if !(bbox.Width < titleBox.Width) {
		textX = -(titleBox.Width - bbox.Width) / 2
	}
	text.Attr(
		"transform",
		"translate( "+js.Str(textX)+", "+js.Str(0)+")",
	)

	// The bounding box of the title and the text.
	bbox = label.BBox()

	// Center the label.
	label.Attr(
		"transform",
		"translate("+js.Str(-bbox.Width/2)+", "+js.Str(float64(-bbox.Height/2)-halfPadding+3)+")",
	)

	totalWidth := bbox.Width + ru.Or(node.Padding, 0)
	totalHeight := bbox.Height + ru.Or(node.Padding, 0)
	x := float64(-bbox.Width/2) - halfPadding
	y := float64(-bbox.Height/2) - halfPadding
	var rect *dom.Node
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, roughOptions())
		roughNode, err := rc.Path(
			ru.CreateRoundedRectPathD(x, y, totalWidth, totalHeight, ru.Or(node.Rx, 0)),
			options,
		)
		if err != nil {
			return nil, err
		}

		roughLine := rc.Line(
			float64(-bbox.Width/2)-halfPadding,
			float64(-bbox.Height/2)-halfPadding+titleBox.Height+halfPadding,
			float64(bbox.Width/2)+halfPadding,
			float64(-bbox.Height/2)-halfPadding+titleBox.Height+halfPadding,
			options,
		)

		shapeSvg.InsertNode(roughLine, ":first-child")
		rect = shapeSvg.InsertNode(roughNode, ":first-child")
	} else {
		rect = g.Insert("rect", ":first-child")
		innerLine := g.Insert("line")
		rect.
			Attr("class", "outer title-state").
			Attr("style", nodeStyles).
			Attr("x", float64(-bbox.Width/2)-halfPadding).
			Attr("y", float64(-bbox.Height/2)-halfPadding).
			Attr("width", bbox.Width+ru.Or(node.Padding, 0)).
			Attr("height", bbox.Height+ru.Or(node.Padding, 0))

		innerLine.
			Attr("class", "divider").
			Attr("x1", float64(-bbox.Width/2)-halfPadding).
			Attr("x2", float64(bbox.Width/2)+halfPadding).
			Attr("y1", float64(-bbox.Height/2)-halfPadding+titleBox.Height+halfPadding).
			Attr("y2", float64(-bbox.Height/2)-halfPadding+titleBox.Height+halfPadding)
	}
	ru.UpdateNodeBounds(node, rect)

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}

	return shapeSvg, nil
}
