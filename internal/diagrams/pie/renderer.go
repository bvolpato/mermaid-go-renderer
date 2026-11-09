// Ported from mermaid 12.2.0, diagrams/pie/pieRenderer.ts (MIT).

package pie

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/scale"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/shape"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// createPieArcs is createPieArcs(sections).
func createPieArcs(sections *js.Map[string, float64]) []shape.PieArcDatum[D3Section] {
	sum := 0.0
	for _, val := range sections.Values() {
		sum = sum + val
	}

	pieData := []D3Section{}
	sections.Each(func(label string, value float64) {
		d := D3Section{Label: label, Value: value}
		// Remove values < 1%
		if (d.Value/sum)*100 >= 1 {
			pieData = append(pieData, d)
		}
	})

	pie := shape.NewPie[D3Section]().
		Value(func(d D3Section, _ int, _ []D3Section) float64 { return d.Value }).
		Sort(nil)
	return pie.Arcs(pieData)
}

// draw is draw(text, id, _version, diagObj): it draws a pie chart with the
// data of the db.
func draw(ctx *render.Context, rdb render.DB, _ string) error {
	db := rdb.(*DB)
	globalConfig := ctx.Config
	pieConfig := utils.CleanAndMerge(db.GetConfig(), globalConfig.Get("pie"))
	const (
		MARGIN           = 40.0
		LEGEND_RECT_SIZE = 18.0
		LEGEND_SPACING   = 4.0
	)
	height := 450.0
	pieWidth := height
	svg := ctx.SelectSvgElement()
	group := svg.Append("g")
	group.Attr("transform", "translate("+js.Str(pieWidth/2)+","+js.Str(height/2)+")")

	// themeVariable is themeVariables[key].
	themeVariable := func(key string) any { return js.Path(globalConfig, "themeVariables", key) }
	outerStrokeWidth, _, ok := utils.ParseFontSize(themeVariable("pieOuterStrokeWidth"))
	if !ok {
		outerStrokeWidth = 2
	}

	legendPosition := pieConfig.Get("legendPosition")

	textPosition := js.ToNumber(pieConfig.Get("textPosition"))
	innerHole := 0.0
	if donutHole := js.ToNumber(pieConfig.Get("donutHole")); donutHole > 0 && donutHole <= 0.9 {
		innerHole = donutHole
	}
	radius := float64(js.Min(pieWidth, height)/2) - MARGIN
	// Shape helper to build arcs:
	arcGenerator := shape.NewArc[shape.PieArcDatum[D3Section]]().
		InnerRadius(innerHole * radius).
		OuterRadius(radius)
	labelArcGenerator := shape.NewArc[shape.PieArcDatum[D3Section]]().
		InnerRadius(radius * textPosition).
		OuterRadius(radius * textPosition)

	pie := group.Append("g")

	pie.Append("circle").
		Attr("cx", 0).
		Attr("cy", 0).
		Attr("r", radius+float64(outerStrokeWidth/2)).
		Attr("class", "pieOuterCircle")

	sections := db.GetSections()
	arcs := createPieArcs(sections)

	myGeneratedColors := []any{
		themeVariable("pie1"),
		themeVariable("pie2"),
		themeVariable("pie3"),
		themeVariable("pie4"),
		themeVariable("pie5"),
		themeVariable("pie6"),
		themeVariable("pie7"),
		themeVariable("pie8"),
		themeVariable("pie9"),
		themeVariable("pie10"),
		themeVariable("pie11"),
		themeVariable("pie12"),
	}
	sum := 0.0
	sections.Each(func(_ string, section float64) {
		sum += section
	})

	// Filter out arcs that would render as 0%
	filteredArcs := []shape.PieArcDatum[D3Section]{}
	for _, datum := range arcs {
		if js.ToFixed((datum.Data.Value/sum)*100, 0) != "0" {
			filteredArcs = append(filteredArcs, datum)
		}
	}

	// Set the color scale
	color := scale.NewOrdinal[string, any]().Range(myGeneratedColors...).Domain(sections.Keys()...)

	// Build the pie chart: each part of the pie is a path that we build using the arc function.
	highlightSlice := pieConfig.Get("highlightSlice")
	for _, datum := range filteredArcs {
		className := "pieCircle"
		if js.StrictEquals(highlightSlice, "hover") {
			className += " highlightedOnHover"
		} else if js.StrictEquals(highlightSlice, datum.Data.Label) {
			className += " highlighted"
		}
		pie.Append("path").
			Attr("d", arcGenerator.Path(datum)).
			Attr("fill", color.Scale(datum.Data.Label)).
			Attr("class", className)
	}

	// Now add the section text.
	// Use the centroid method to get the best coordinates.
	for _, datum := range filteredArcs {
		x, y := labelArcGenerator.Centroid(datum)
		pie.Append("text").
			Text(js.ToFixed((datum.Data.Value/sum)*100, 0)+"%").
			Attr("transform", "translate("+js.Str(x)+","+js.Str(y)+")").
			Style("text-anchor", "middle").
			Attr("class", "slice")
	}

	titleText := group.Append("text").
		Text(db.GetDiagramTitle()).
		Attr("x", 0).
		Attr("y", -(height-50)/2).
		Attr("class", "pieTitleText")

	// Add the legends/annotations for each section
	allSectionData := []D3Section{}
	sections.Each(func(label string, value float64) {
		allSectionData = append(allSectionData, D3Section{Label: label, Value: value})
	})

	// Draw legend
	legend := make([]*dom.Node, len(allSectionData))
	for i := range allSectionData {
		legend[i] = group.Append("g").Attr("class", "legend")
	}

	for i, d := range allSectionData {
		legend[i].Append("rect").
			Attr("width", LEGEND_RECT_SIZE).
			Attr("height", LEGEND_RECT_SIZE).
			Style("fill", color.Scale(d.Label)).
			Style("stroke", color.Scale(d.Label))
	}

	legendTexts := make([]*dom.Node, len(allSectionData))
	for i, d := range allSectionData {
		text := d.Label
		if db.GetShowData() {
			text = d.Label + " [" + js.Str(d.Value) + "]"
		}
		legendTexts[i] = legend[i].Append("text").
			Attr("x", LEGEND_RECT_SIZE+LEGEND_SPACING).
			Attr("y", LEGEND_RECT_SIZE-LEGEND_SPACING).
			Text(text)
	}

	widths := make([]float64, len(legendTexts))
	for i, node := range legendTexts {
		widths[i] = node.BoundingClientRect().Width
	}
	longestTextWidth := js.Max(widths...)

	chartAndLegendHeight := height
	chartAndLegendWidth := pieWidth + MARGIN

	legendHeight := LEGEND_RECT_SIZE + LEGEND_SPACING
	n := float64(len(allSectionData))
	totalLegendHeight := float64(n * legendHeight)

	switch legendPosition {
	case "center":
		for index, g := range legend {
			offset := float64((legendHeight * n) / 2)
			horizontal := float64(-longestTextWidth/2) - (LEGEND_RECT_SIZE + LEGEND_SPACING)
			vertical := float64(float64(index)*legendHeight) - offset
			g.Attr("transform", "translate("+js.Str(horizontal)+","+js.Str(vertical)+")")
		}
	case "top":
		chartAndLegendHeight += totalLegendHeight

		for index, g := range legend {
			offset := radius
			horizontal := float64(-longestTextWidth/2) - (LEGEND_RECT_SIZE + LEGEND_SPACING)
			vertical := float64(float64(index)*legendHeight) - offset
			g.Attr("transform", "translate("+js.Str(horizontal)+", "+js.Str(vertical)+")")
		}
		pie.Attr("transform", "translate(0, "+js.Str(totalLegendHeight+legendHeight)+")")
	case "bottom":
		chartAndLegendHeight += totalLegendHeight

		for index, g := range legend {
			offset := -radius - legendHeight
			horizontal := float64(-longestTextWidth/2) - (LEGEND_RECT_SIZE + LEGEND_SPACING)
			vertical := float64(float64(index)*legendHeight) - offset
			g.Attr("transform", "translate("+js.Str(horizontal)+","+js.Str(vertical)+")")
		}
	case "left":
		chartAndLegendWidth += LEGEND_RECT_SIZE + LEGEND_SPACING + longestTextWidth

		for index, g := range legend {
			offset := float64((legendHeight * n) / 2)
			horizontal := -radius - (LEGEND_RECT_SIZE + LEGEND_SPACING)
			vertical := float64(float64(index)*legendHeight) - offset
			g.Attr("transform", "translate("+js.Str(horizontal)+","+js.Str(vertical)+")")
		}
		pie.Attr("transform", "translate("+js.Str(longestTextWidth+LEGEND_RECT_SIZE+LEGEND_SPACING)+", 0)")
	default: // "right"
		chartAndLegendWidth += LEGEND_RECT_SIZE + LEGEND_SPACING + longestTextWidth

		for index, g := range legend {
			offset := float64((legendHeight * n) / 2)
			horizontal := 12 * LEGEND_RECT_SIZE
			vertical := float64(float64(index)*legendHeight) - offset
			g.Attr("transform", "translate("+js.Str(horizontal)+","+js.Str(vertical)+")")
		}
	}

	// Measure title width to ensure it's not clipped
	titleWidth := titleText.BoundingClientRect().Width

	// Title is centered at pieWidth/2 in SVG coords: expand viewBox to contain it
	titleLeft := pieWidth/2 - float64(titleWidth/2)
	titleRight := pieWidth/2 + float64(titleWidth/2)

	viewBoxX := js.Min(0, titleLeft)
	viewBoxRight := js.Max(chartAndLegendWidth, titleRight)
	totalWidth := viewBoxRight - viewBoxX

	svg.Attr("viewBox", js.Str(viewBoxX)+" 0 "+js.Str(totalWidth)+" "+js.Str(chartAndLegendHeight))
	render.ConfigureSvgSize(svg, chartAndLegendHeight, totalWidth, js.Truthy(pieConfig.Get("useMaxWidth")))
	return nil
}
