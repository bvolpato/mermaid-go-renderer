// Ported from mermaid 12.2.0,
// rendering-util/rendering-elements/shapes/kanbanItem.ts (MIT).

package kanban

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("kanbanItem", kanbanItem)
}

// colorFromPriority is colorFromPriority(priority). The result is nil for
// "Medium" (no stroke) and for a priority that the switch does not have.
func colorFromPriority(priority any) any {
	switch priority {
	case "Very High":
		return "red"
	case "High":
		return "orange"
	case "Medium":
		return nil // no stroke
	case "Low":
		return "blue"
	case "Very Low":
		return "lightblue"
	}
	return nil
}

// labelText is the label argument of insertLabel for the ticket and the
// assigned person: the property of the node, or the empty string when it is
// falsy. decodeEntities calls a string method on the
// label, so a truthy value that is not a string is an error.
func labelText(kanbanNode *ru.Node, key string) (string, error) {
	v := kanbanNode.Get(key)
	if !js.Truthy(v) {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", typeError("text.replace is not a function")
	}
	return s, nil
}

// kanbanItem is kanbanItem(parent, kanbanNode, { config }).
func kanbanItem(ctx *render.Context, parent *dom.Node, kanbanNode *ru.Node, options *ru.ShapeRenderOptions) (*dom.Node, error) {
	var config *js.Obj[any]
	if options != nil {
		config = options.Config
	}
	styles := ru.Styles2String(kanbanNode)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	kanbanNode.LabelStyle = labelStyles

	const labelPaddingX = 10
	orgWidth := kanbanNode.Width
	kanbanNode.Width = ru.NN(kanbanNode.Width, 200) - 10

	res, err := ru.LabelHelper(ctx, parent, kanbanNode, ru.GetNodeClasses(kanbanNode))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, labelElTitle := res.ShapeSvg, res.BBox, res.Label
	padding := ru.Or(kanbanNode.Padding, 10)

	ticketUrl := ""
	var link *dom.Node

	if ticket := kanbanNode.Get("ticket"); js.Truthy(ticket) {
		if ticketBaseUrl := js.Path(config, "kanban", "ticketBaseUrl"); js.Truthy(ticketBaseUrl) {
			base, ok := ticketBaseUrl.(string)
			if !ok {
				return nil, typeError("config?.kanban?.ticketBaseUrl.replace is not a function")
			}
			ticketUrl = js.ReplaceString(base, "#TICKET#", js.ToString(ticket))
			link = shapeSvg.
				Insert("svg:a", ":first-child").
				Attr("class", "kanban-ticket-link").
				Attr("xlink:href", ticketUrl).
				Attr("target", "_blank")
		}
	}

	insertOptions := ru.InsertLabelOptions{
		UseHtmlLabels:    kanbanNode.UseHtmlLabels,
		UseHtmlLabelsSet: kanbanNode.UseHtmlLabelsSet,
		LabelStyle:       kanbanNode.LabelStyle,
		Width:            kanbanNode.Width,
		Img:              kanbanNode.Img,
		Padding:          ru.Or(kanbanNode.Padding, 8),
		CenterLabel:      false,
	}
	ticketText, err := labelText(kanbanNode, "ticket")
	if err != nil {
		return nil, err
	}
	var ticketLabel *ru.LabelResult
	if link != nil {
		ticketLabel, err = ru.InsertLabel(ctx, link, ticketText, insertOptions)
	} else {
		ticketLabel, err = ru.InsertLabel(ctx, shapeSvg, ticketText, insertOptions)
	}
	if err != nil {
		return nil, err
	}
	labelEl, bbox2 := ticketLabel.Label, ticketLabel.BBox
	assignedText, err := labelText(kanbanNode, "assigned")
	if err != nil {
		return nil, err
	}
	assignedLabel, err := ru.InsertLabel(ctx, shapeSvg, assignedText, insertOptions)
	if err != nil {
		return nil, err
	}
	labelElAssigned, bboxAssigned := assignedLabel.Label, assignedLabel.BBox
	kanbanNode.Width = orgWidth
	const labelPaddingY = 10
	totalWidth := ru.Or(kanbanNode.Width, 0)
	heightAdj := float64(js.Max(bbox2.Height, bboxAssigned.Height) / 2)
	totalHeight := js.Max(bbox.Height+labelPaddingY*2, ru.Or(kanbanNode.Height, 0)) + heightAdj
	x := float64(-totalWidth / 2)
	y := float64(-totalHeight / 2)
	labelElTitle.Attr(
		"transform",
		"translate("+js.Str(padding-float64(totalWidth/2))+", "+js.Str(-heightAdj-float64(bbox.Height/2))+")",
	)
	labelEl.Attr(
		"transform",
		"translate("+js.Str(padding-float64(totalWidth/2))+", "+js.Str(-heightAdj+float64(bbox.Height/2))+")",
	)
	labelElAssigned.Attr(
		"transform",
		"translate("+
			js.Str(padding+float64(totalWidth/2)-bboxAssigned.Width-2*labelPaddingX)+
			", "+
			js.Str(-heightAdj+float64(bbox.Height/2))+
			")",
	)

	var rect *dom.Node

	rx, ry := kanbanNode.Rx, kanbanNode.Ry

	if kanbanNode.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		roughOptions := ru.UserNodeOverrides(ctx, kanbanNode, js.NewObj[any]())

		var roughNode *dom.Node
		if js.Truthy(rx) || js.Truthy(ry) {
			roughNode, err = rc.Path(ru.CreateRoundedRectPathD(x, y, totalWidth, totalHeight, ru.Or(rx, 0)), roughOptions)
		} else {
			roughNode, err = rc.Rectangle(x, y, totalWidth, totalHeight, roughOptions)
		}
		if err != nil {
			return nil, err
		}

		rect = shapeSvg.InsertNode(roughNode, ":first-child")
		rect.Attr("class", "basic label-container").Attr("style", ru.CSSStylesAttr(kanbanNode))
	} else {
		rect = shapeSvg.Insert("rect", ":first-child")

		rect.
			Attr("class", "basic label-container __APA__").
			Attr("style", nodeStyles).
			Attr("rx", ru.NN(rx, 5)).
			Attr("ry", ru.NN(ry, 5)).
			Attr("x", x).
			Attr("y", y).
			Attr("width", totalWidth).
			Attr("height", totalHeight)

		if priority := kanbanNode.Get("priority"); js.Truthy(priority) {
			line := shapeSvg.Append("line")
			lineX := x + 2

			y1 := y + math.Floor(ru.NN(rx, 0)/2)
			y2 := y + totalHeight - math.Floor(ru.NN(rx, 0)/2)
			line.
				Attr("x1", lineX).
				Attr("y1", y1).
				Attr("x2", lineX).
				Attr("y2", y2).
				Attr("stroke-width", "4").
				Attr("stroke", colorFromPriority(priority))
		}
	}

	ru.UpdateNodeBounds(kanbanNode, rect)
	kanbanNode.Height = totalHeight

	kanbanNode.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(kanbanNode), point)
	}

	return shapeSvg, nil
}
