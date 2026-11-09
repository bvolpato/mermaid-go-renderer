// Ported from mermaid 12.2.0,
// rendering-util/rendering-elements/clusters/swimlane.js (MIT).

package clusters

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// swimlane is the cluster shape of a lane. The title is a strip on top of
// the lane, or with the direction LR a strip on the left with the text
// rotated. The extra properties "direction" and "swimlaneContentTop" of the
// node are node.direction and node.swimlaneContentTop.
func swimlane(ctx *render.Context, parent *dom.Node, node *ru.Node) (*ru.ClusterResult, error) {
	theme := ctx.Cfg("theme")
	handDrawnSeed := ctx.Cfg("handDrawnSeed")
	clusterBkg := ctx.Cfg("themeVariables", "clusterBkg")
	clusterBorder := ctx.Cfg("themeVariables", "clusterBorder")
	borderColorArray := ctx.Cfg("themeVariables", "borderColorArray")
	laneStroke := clusterBorder

	styles := ru.Styles2String(node)
	labelStyles, nodeStyles, borderStyles, backgroundStyles := styles.LabelStyles, styles.NodeStyles, styles.BorderStyles, styles.BackgroundStyles

	// Add outer g element.
	shapeSvg := parent.
		Insert("g").
		Attr("class", "cluster swimlane "+node.CSSClasses).
		Attr("id", domIDOrID(node)).
		Attr("data-id", node.ID).
		Attr("data-et", "cluster").
		Attr("data-look", strAttr(node.Look))

	// The color slot of the lane. It does nothing unless the theme has a
	// palette.
	ru.StampColorSlot(shapeSvg, node.ColorIndex, theme, borderColorArray)

	useHtmlLabels := config.Evaluate(ctx.Cfg("flowchart", "htmlLabels"))

	// With a left-to-right layout the title is on the left, rotated.
	direction, _ := node.Get("direction").(string)
	isLR := direction == "LR"

	// Create the label and insert it after the rects.
	labelEl := shapeSvg.Insert("g").Attr("class", "cluster-label swimlane-label")

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
	bbox := labelBBox(text, useHtmlLabels)

	padding := ru.NN(node.Padding, 0)
	width := node.Width
	if node.Width <= bbox.Width+padding {
		width = bbox.Width + padding
	}
	if node.Width <= bbox.Width+padding {
		node.Diff = float64((width-node.Width)/2) - padding
	} else {
		node.Diff = -padding
	}

	height := node.Height
	laneTop := node.Y - float64(height/2)
	laneBottom := node.Y + float64(height/2)
	laneLeft := node.X - float64(width/2)

	// The top of the content area across all lanes, which the swimlanes
	// layout computes: the y of the highest node in the pool.
	contentTop := laneTop + float64(height/3)
	if v := node.Get("swimlaneContentTop"); v != nil {
		contentTop = js.ToNumber(v)
	}

	// The size of the title band. For LR the title is on the left, for TB it
	// is on top. TB uses a smaller internal padding, so that the gap between
	// the title band and the first row of nodes is larger.
	titlePaddingY := 0.0
	if isLR {
		titlePaddingY = 4
	}
	desiredTitleSize := bbox.Height + float64(2*titlePaddingY)

	var titleRect, bodyRect *dom.Node

	if isLR {
		// LR layout: the title band is a vertical strip on the left. The text
		// is rotated, so the width of the title is the height of the label.
		titleWidth := js.Max(desiredTitleSize, bbox.Height+float64(2*titlePaddingY))
		bodyX := laneLeft + titleWidth
		bodyWidth := js.Max(0, width-titleWidth)

		if node.Look == "handDrawn" {
			rc := ru.NewRoughSVG(ctx, shapeSvg)
			titleOptions := ru.UserNodeOverrides(ctx, node, obj(
				"roughness", 0.7,
				"fill", clusterBkg,
				"stroke", laneStroke,
				"seed", handDrawnSeed,
			))
			bodyOptions := ru.UserNodeOverrides(ctx, node, obj(
				"roughness", 0.7,
				"fill", "none",
				"stroke", laneStroke,
				"seed", handDrawnSeed,
			))

			roughTitle, err := rc.Rectangle(laneLeft, laneTop, titleWidth, height, titleOptions)
			if err != nil {
				return nil, err
			}
			// The same classes as the classic look, so that CSS can tell the
			// two halves apart.
			titleRect = shapeSvg.InsertNode(roughTitle, ":first-child").Attr("class", "swimlane-title")
			roughBody, err := rc.Rectangle(bodyX, laneTop, bodyWidth, height, bodyOptions)
			if err != nil {
				return nil, err
			}
			bodyRect = shapeSvg.InsertNode(roughBody, ":first-child").Attr("class", "swimlane-body")

			titleRect.Select("path:nth-child(2)").Attr("style", strings.Join(borderStyles, ";"))
			titleRect.Select("path").Attr("style", strings.Replace(strings.Join(backgroundStyles, ";"), "fill", "stroke", 1))
		} else {
			titleRect = shapeSvg.Insert("rect", ":first-child")
			bodyRect = shapeSvg.Insert("rect", ":first-child")

			titleRect.
				Attr("class", "swimlane-title").
				Attr("style", nodeStyles).
				Attr("x", laneLeft).
				Attr("y", laneTop).
				Attr("width", titleWidth).
				Attr("height", ru.NumAttr(height)).
				Attr("fill", dynAttr(clusterBkg)).
				Attr("stroke", dynAttr(laneStroke))

			bodyRect.
				Attr("class", "swimlane-body").
				Attr("style", nodeStyles).
				Attr("x", bodyX).
				Attr("y", laneTop).
				Attr("width", bodyWidth).
				Attr("height", ru.NumAttr(height)).
				Attr("fill", "none").
				Attr("stroke", dynAttr(laneStroke))
		}

		// Position the label: center it in the title band and rotate it by
		// -90 degrees. The label then reads from bottom to top.
		labelCenterX := laneLeft + float64(titleWidth/2)
		labelCenterY := node.Y
		labelEl.Attr("transform",
			"translate("+js.Str(labelCenterX)+", "+js.Str(labelCenterY)+") rotate(-90) translate("+js.Str(-bbox.Width/2)+", "+js.Str(-bbox.Height/2)+")")
	} else {
		// TB layout (the default): the title band is a horizontal strip on top.
		headerMaxHeight := js.Max(0, contentTop-laneTop)
		titleHeight := js.Min(desiredTitleSize, headerMaxHeight)

		// The body of the lane starts where the title band ends, so that
		// their borders touch.
		bodyY := laneTop + titleHeight
		contentHeight := js.Max(0, laneBottom-bodyY)

		x := node.X - float64(width/2)

		if node.Look == "handDrawn" {
			rc := ru.NewRoughSVG(ctx, shapeSvg)
			titleOptions := ru.UserNodeOverrides(ctx, node, obj(
				"roughness", 0.7,
				"fill", clusterBkg,
				"stroke", laneStroke,
				"fillWeight", 3.0,
				"seed", handDrawnSeed,
			))
			bodyOptions := ru.UserNodeOverrides(ctx, node, obj(
				"roughness", 0.7,
				"fill", "none",
				"stroke", laneStroke,
				"seed", handDrawnSeed,
			))

			roughTitle, err := rc.Rectangle(x, laneTop, width, titleHeight, titleOptions)
			if err != nil {
				return nil, err
			}
			titleRect = shapeSvg.InsertNode(roughTitle, ":first-child").Attr("class", "swimlane-title")
			roughBody, err := rc.Rectangle(x, bodyY, width, contentHeight, bodyOptions)
			if err != nil {
				return nil, err
			}
			bodyRect = shapeSvg.InsertNode(roughBody, ":first-child").Attr("class", "swimlane-body")

			titleRect.Select("path:nth-child(2)").Attr("style", strings.Join(borderStyles, ";"))
			titleRect.Select("path").Attr("style", strings.Replace(strings.Join(backgroundStyles, ";"), "fill", "stroke", 1))
		} else {
			titleRect = shapeSvg.Insert("rect", ":first-child")
			bodyRect = shapeSvg.Insert("rect", ":first-child")

			titleRect.
				Attr("class", "swimlane-title").
				Attr("style", nodeStyles).
				Attr("x", x).
				Attr("y", laneTop).
				Attr("width", ru.NumAttr(width)).
				Attr("height", titleHeight).
				Attr("fill", dynAttr(clusterBkg)).
				Attr("stroke", dynAttr(laneStroke))

			bodyRect.
				Attr("class", "swimlane-body").
				Attr("style", nodeStyles).
				Attr("x", x).
				Attr("y", bodyY).
				Attr("width", ru.NumAttr(width)).
				Attr("height", contentHeight).
				Attr("fill", "none").
				Attr("stroke", dynAttr(laneStroke))
		}

		// Place the label centered in the title band.
		labelX := node.X - float64(bbox.Width/2)
		labelY := laneTop + float64((titleHeight-bbox.Height)/2)
		labelEl.Attr("transform", translate(labelX, labelY))
	}
	_ = bodyRect

	if labelStyles != "" {
		labelEl.Select("span").Attr("style", labelStyles)
	}

	node.OffsetX = 0
	node.Width = width
	node.Height = height
	// Used by the layout engine to position the subgraph in its parent.
	node.OffsetY = bbox.Height - float64(padding/2)

	setIntersect(node)

	return &ru.ClusterResult{Cluster: shapeSvg, LabelBBox: bbox, HasLabelBBox: true}, nil
}
