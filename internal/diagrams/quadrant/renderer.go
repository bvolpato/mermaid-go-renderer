// Ported from mermaid 12.2.0, diagrams/quadrant-chart/quadrantRenderer.ts
// (MIT).

package quadrant

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func getDominantBaseLine(horizontalPos string) string {
	if horizontalPos == "top" {
		return "hanging"
	}
	return "middle"
}

func getTextAnchor(verticalPos string) string {
	if verticalPos == "left" {
		return "start"
	}
	return "middle"
}

func getTransformation(data QuadrantTextType) string {
	// data.rotation || 0
	rotation := data.Rotation
	if !js.Truthy(rotation) {
		rotation = 0
	}
	return "translate(" + js.ToString(data.X) + ", " + js.ToString(data.Y) + ") rotate(" + js.Str(rotation) + ")"
}

// draw is draw(txt, id, _version, diagObj).
func draw(ctx *render.Context, rdb render.DB, _ string) error {
	db := rdb.(*DB)
	conf := ctx.Config

	if conf.Get("securityLevel") == "sandbox" {
		return errors.New("securityLevel sandbox is not supported: it renders into an iframe")
	}
	root := ctx.Body

	svg := root.Select(`[id="` + ctx.ID + `"]`)

	group := svg.Append("g").Attr("class", "main")

	// orDefault is value ?? fallback.
	orDefault := func(value, fallback any) any {
		if js.IsNullish(value) {
			return fallback
		}
		return value
	}
	width := orDefault(js.Path(conf, "quadrantChart", "chartWidth"), 500.0)
	height := orDefault(js.Path(conf, "quadrantChart", "chartHeight"), 500.0)

	render.ConfigureSvgSize(svg, js.ToNumber(height), js.ToNumber(width), js.Truthy(orDefault(js.Path(conf, "quadrantChart", "useMaxWidth"), true)))

	svg.Attr("viewBox", "0 0 "+js.ToString(width)+" "+js.ToString(height))

	db.SetHeight(height)
	db.SetWidth(width)

	quadrantData := db.GetQuadrantData()

	quadrantsGroup := group.Append("g").Attr("class", "quadrants")
	borderGroup := group.Append("g").Attr("class", "border")
	dataPointGroup := group.Append("g").Attr("class", "data-points")
	labelGroup := group.Append("g").Attr("class", "labels")
	titleGroup := group.Append("g").Attr("class", "title")

	if quadrantData.Title != nil {
		titleGroup.Append("text").
			Attr("x", 0).
			Attr("y", 0).
			Attr("fill", quadrantData.Title.Fill).
			Attr("font-size", quadrantData.Title.FontSize).
			Attr("dominant-baseline", getDominantBaseLine(quadrantData.Title.HorizontalPos)).
			Attr("text-anchor", getTextAnchor(quadrantData.Title.VerticalPos)).
			Attr("transform", getTransformation(*quadrantData.Title)).
			Text(quadrantData.Title.Text)
	}

	for _, data := range quadrantData.BorderLines {
		borderGroup.Append("line").
			Attr("x1", data.X1).
			Attr("y1", data.Y1).
			Attr("x2", data.X2).
			Attr("y2", data.Y2).
			Style("stroke", data.StrokeFill).
			Style("stroke-width", data.StrokeWidth)
	}

	quadrants := make([]*dom.Node, len(quadrantData.Quadrants))
	for i := range quadrantData.Quadrants {
		quadrants[i] = quadrantsGroup.Append("g").Attr("class", "quadrant")
	}

	for i, data := range quadrantData.Quadrants {
		quadrants[i].Append("rect").
			Attr("x", data.X).
			Attr("y", data.Y).
			Attr("width", data.Width).
			Attr("height", data.Height).
			Attr("fill", data.Fill)
	}

	for i, data := range quadrantData.Quadrants {
		quadrants[i].Append("text").
			Attr("x", 0).
			Attr("y", 0).
			Attr("fill", data.Text.Fill).
			Attr("font-size", data.Text.FontSize).
			Attr("dominant-baseline", getDominantBaseLine(data.Text.HorizontalPos)).
			Attr("text-anchor", getTextAnchor(data.Text.VerticalPos)).
			Attr("transform", getTransformation(data.Text)).
			Text(data.Text.Text)
	}

	for _, data := range quadrantData.AxisLabels {
		label := labelGroup.Append("g").Attr("class", "label")
		label.Append("text").
			Attr("x", 0).
			Attr("y", 0).
			Text(data.Text).
			Attr("fill", data.Fill).
			Attr("font-size", data.FontSize).
			Attr("dominant-baseline", getDominantBaseLine(data.HorizontalPos)).
			Attr("text-anchor", getTextAnchor(data.VerticalPos)).
			Attr("transform", getTransformation(data))
	}

	dataPoints := make([]*dom.Node, len(quadrantData.Points))
	for i := range quadrantData.Points {
		dataPoints[i] = dataPointGroup.Append("g").Attr("class", "data-point")
	}

	for i, data := range quadrantData.Points {
		dataPoints[i].Append("circle").
			Attr("cx", data.X).
			Attr("cy", data.Y).
			Attr("r", data.Radius).
			Attr("fill", data.Fill).
			Attr("stroke", data.StrokeColor).
			Attr("stroke-width", data.StrokeWidth)
	}

	for i, data := range quadrantData.Points {
		dataPoints[i].Append("text").
			Attr("x", 0).
			Attr("y", 0).
			Text(data.Text.Text).
			Attr("fill", data.Text.Fill).
			Attr("font-size", data.Text.FontSize).
			Attr("dominant-baseline", getDominantBaseLine(data.Text.HorizontalPos)).
			Attr("text-anchor", getTextAnchor(data.Text.VerticalPos)).
			Attr("transform", getTransformation(data.Text))
	}
	return nil
}
