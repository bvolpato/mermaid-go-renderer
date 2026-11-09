// Ported from mermaid 12.2.0, diagrams/xychart/xychartRenderer.ts (MIT).

package xychart

import (
	"errors"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// maxFontSizeSteps bounds the loops that look for the font size of the data
// labels. Upstream decreases the size by one until the label fits, with no
// bound: a bar that is very large makes the browser hang, and a size above
// 2^53 never ends. The port returns an error for a bar that needs more
// steps than this.
const maxFontSizeSteps = 1 << 20

var errFontSizeSteps = errors.New("xychart: the font size of the data labels does not converge")

// barItem is BarItem.
type barItem struct {
	data  *rectElem
	label string
}

// enter is selection.selectAll(name).data(data).enter(): the indexes of the
// data that have no element yet. d3 binds the data by index to the elements
// that the selector matches, so that the first elements of the data stay
// with the existing elements.
func enter(shapeGroup *dom.Node, name string, dataLength int) int {
	existing := len(shapeGroup.SelectAll(name))
	if existing > dataLength {
		return dataLength
	}
	return existing
}

// draw is draw(txt, id, _version, diagObj).
func draw(ctx *render.Context, d *db) error {
	themeConfig := d.getChartThemeConfig()
	chartConfig := d.getChartConfig()
	plots := d.getXYChartData().plots
	if len(plots) == 0 {
		return &typeError{"Cannot read properties of undefined (reading 'data')"}
	}
	labelData := make([]any, len(plots[0].data))
	for i, data := range plots[0].data {
		labelData[i] = data.y
	}
	getDominantBaseLine := func(horizontalPos string) string {
		if horizontalPos == "top" {
			return "text-before-edge"
		}
		return "middle"
	}

	getTextAnchor := func(verticalPos string) string {
		if verticalPos == "left" {
			return "start"
		}
		if verticalPos == "right" {
			return "end"
		}
		return "middle"
	}

	getTextTransformation := func(data *textElem) string {
		rotation := data.rotation
		if rotation == 0 || math.IsNaN(rotation) {
			// data.rotation || 0
			rotation = 0
		}
		return "translate(" + js.ToString(data.x) + ", " + js.ToString(data.y) + ") rotate(" + js.Str(rotation) + ")"
	}

	svg := ctx.SelectSvgElement()
	group := svg.Append("g").Attr("class", "main")
	background := group.
		Append("rect").
		Attr("width", attrValue(chartConfig.Get("width"))).
		Attr("height", attrValue(chartConfig.Get("height"))).
		Attr("class", "background")

	configureSvgSize(svg, chartConfig.Get("height"), chartConfig.Get("width"), true)

	svg.Attr("viewBox", "0 0 "+js.ToString(chartConfig.Get("width"))+" "+js.ToString(chartConfig.Get("height")))

	background.Attr("fill", attrValue(themeConfig.Get("backgroundColor")))

	d.setTmpSVGG(svg.Append("g").Attr("class", "mermaid-tmp-group"))

	shapes, err := d.getDrawableElem()
	if err != nil {
		return err
	}

	groups := map[string]*dom.Node{}

	getGroup := func(gList []string) *dom.Node {
		elem := group
		prefix := ""
		for i := range gList {
			parent := group
			if i > 0 && groups[prefix] != nil {
				parent = groups[prefix]
			}
			prefix += gList[i]
			elem = groups[prefix]
			if elem == nil {
				elem = parent.Append("g").Attr("class", gList[i])
				groups[prefix] = elem
			}
		}
		return elem
	}

	for si := range shapes {
		shape := &shapes[si]
		if shape.dataLength() == 0 {
			continue
		}

		shapeGroup := getGroup(shape.groupTexts)

		switch shape.typ {
		case "rect":
			for i := enter(shapeGroup, "rect", len(shape.rects)); i < len(shape.rects); i++ {
				data := &shape.rects[i]
				shapeGroup.
					Append("rect").
					Attr("x", attrValue(data.x)).
					Attr("y", attrValue(data.y)).
					Attr("width", data.width).
					Attr("height", data.height).
					Attr("fill", attrValue(data.fill)).
					Attr("stroke", attrValue(data.strokeFill)).
					Attr("stroke-width", attrValue(data.strokeWidth))
			}

			if js.Truthy(chartConfig.Get("showDataLabel")) {
				showDataLabelOutsideBar := js.Truthy(chartConfig.Get("showDataLabelOutsideBar"))

				// Filter out bars that have zero width or height.
				validItems := []barItem{}
				for i := range shape.rects {
					// labelData[i].toString()
					if i >= len(labelData) || js.IsNullish(labelData[i]) {
						return &typeError{"Cannot read properties of undefined (reading 'toString')"}
					}
					item := barItem{data: &shape.rects[i], label: js.ToString(labelData[i])}
					if item.data.width > 0 && item.data.height > 0 {
						validItems = append(validItems, item)
					}
				}

				if js.StrictEquals(chartConfig.Get("chartOrientation"), "horizontal") {
					// Factor to approximate each character's width.
					charWidthFactor := 0.7

					rightMargin := 10.0

					// Helper function to check if the text fits horizontally
					// with a 10px right margin.
					fitsHorizontally := func(item barItem, fontSize float64) bool {
						data, label := item.data, item.label
						// Approximate the text width.
						textWidth := fontSize * float64(js.UTF16Len(label)) * charWidthFactor
						// The available width is the bar's width minus a 10px
						// right margin.
						return textWidth <= data.width-rightMargin
					}

					// For each valid bar, start with an initial candidate font
					// size (70% of the bar's height), then reduce it until the
					// text fits horizontally.
					candidateFontSizes := make([]float64, len(validItems))
					for i, item := range validItems {
						data := item.data
						fontSize := data.height * 0.7
						// Decrease fontSize until the text fits horizontally.
						steps := 0
						for !fitsHorizontally(item, fontSize) && fontSize > 0 {
							fontSize -= 1
							if steps++; steps > maxFontSizeSteps {
								return errFontSizeSteps
							}
						}
						candidateFontSizes[i] = fontSize
					}

					// Choose the smallest candidate font size across all valid
					// bars for uniformity.
					uniformFontSize := math.Floor(js.Min(candidateFontSizes...))

					determineLabelXPosition := func(item barItem) any {
						if showDataLabelOutsideBar {
							return js.Add(js.Add(item.data.x, item.data.width), rightMargin)
						}
						return num(js.Add(item.data.x, item.data.width)) - rightMargin
					}

					textAnchor := "end"
					if showDataLabelOutsideBar {
						textAnchor = "start"
					}
					for i := enter(shapeGroup, "text", len(validItems)); i < len(validItems); i++ {
						item := validItems[i]
						shapeGroup.
							Append("text").
							Attr("x", attrValue(determineLabelXPosition(item))).
							Attr("y", attrValue(js.Add(item.data.y, item.data.height/2))).
							Attr("text-anchor", textAnchor).
							Attr("dominant-baseline", "middle").
							Attr("fill", attrValue(themeConfig.Get("dataLabelColor"))).
							Attr("font-size", js.Str(uniformFontSize)+"px").
							Text(item.label)
					}
				} else {
					yOffset := 10.0

					// Helper function that checks if the text with a given
					// fontSize fits within the bar boundaries.
					fitsInBar := func(item barItem, fontSize float64, yOffset float64) bool {
						data, label := item.data, item.label
						charWidthFactor := 0.7
						textWidth := fontSize * float64(js.UTF16Len(label)) * charWidthFactor

						// Compute horizontal boundaries using the center.
						centerX := js.Add(data.x, data.width/2)
						leftEdge := num(centerX) - float64(textWidth/2)
						rightEdge := js.Add(centerX, textWidth/2)

						// Check that text doesn't overflow horizontally.
						horizontalFits := js.Compare(leftEdge, ">=", data.x) &&
							js.Compare(rightEdge, "<=", js.Add(data.x, data.width))

						// For vertical placement, we use 'dominant-baseline:
						// hanging' so that y marks the top of the text. Thus,
						// the bottom edge is y + yOffset + fontSize.
						verticalFits := js.Compare(
							js.Add(js.Add(data.y, yOffset), fontSize),
							"<=",
							js.Add(data.y, data.height),
						)

						return horizontalFits && verticalFits
					}

					// For each valid item, start with a candidate font size
					// based on the width, then reduce it until the text fits
					// within both the horizontal and vertical boundaries.
					candidateFontSizes := make([]float64, len(validItems))
					for i, item := range validItems {
						data, label := item.data, item.label
						fontSize := data.width / (float64(js.UTF16Len(label)) * 0.7)

						// Decrease the font size until the text fits or
						// fontSize reaches 0.
						steps := 0
						for !fitsInBar(item, fontSize, yOffset) && fontSize > 0 {
							fontSize -= 1
							if steps++; steps > maxFontSizeSteps {
								return errFontSizeSteps
							}
						}
						candidateFontSizes[i] = fontSize
					}

					// Choose the smallest candidate across all valid bars for
					// uniformity.
					uniformFontSize := math.Floor(js.Min(candidateFontSizes...))

					determineLabelYPosition := func(item barItem) any {
						if showDataLabelOutsideBar {
							return num(item.data.y) - yOffset
						}
						return js.Add(item.data.y, yOffset)
					}

					dominantBaseline := "hanging"
					if showDataLabelOutsideBar {
						dominantBaseline = "auto"
					}
					// Render text only for valid items.
					for i := enter(shapeGroup, "text", len(validItems)); i < len(validItems); i++ {
						item := validItems[i]
						shapeGroup.
							Append("text").
							Attr("x", attrValue(js.Add(item.data.x, item.data.width/2))).
							Attr("y", attrValue(determineLabelYPosition(item))).
							Attr("text-anchor", "middle").
							Attr("dominant-baseline", dominantBaseline).
							Attr("fill", attrValue(themeConfig.Get("dataLabelColor"))).
							Attr("font-size", js.Str(uniformFontSize)+"px").
							Text(item.label)
					}
				}
			}
		case "text":
			for i := enter(shapeGroup, "text", len(shape.texts)); i < len(shape.texts); i++ {
				data := &shape.texts[i]
				shapeGroup.
					Append("text").
					Attr("x", 0).
					Attr("y", 0).
					Attr("fill", attrValue(data.fill)).
					Attr("font-size", attrValue(data.fontSize)).
					Attr("dominant-baseline", getDominantBaseLine(data.verticalPos)).
					Attr("text-anchor", getTextAnchor(data.horizontalPos)).
					Attr("transform", getTextTransformation(data)).
					Text(data.text)
			}
		case "path":
			for i := enter(shapeGroup, "path", len(shape.paths)); i < len(shape.paths); i++ {
				data := &shape.paths[i]
				var fill any = "none"
				if js.Truthy(data.fill) {
					fill = data.fill
				}
				shapeGroup.
					Append("path").
					Attr("d", data.path).
					Attr("fill", fill).
					Attr("stroke", attrValue(data.strokeFill)).
					Attr("stroke-width", attrValue(data.strokeWidth))
			}
		}
	}
	return nil
}

// configureSvgSize is configureSvgSize(svgElem, height, width, useMaxWidth)
// of setupGraphViewbox.js. The height and the width are config values that
// can have any type, so that this function does not call
// render.ConfigureSvgSize, which takes numbers.
func configureSvgSize(svgElem *dom.Node, height, width any, useMaxWidth bool) {
	if h, ok := height.(float64); ok {
		if w, ok := width.(float64); ok {
			render.ConfigureSvgSize(svgElem, h, w, useMaxWidth)
			return
		}
	}
	if useMaxWidth {
		svgElem.Attr("width", "100%")
		svgElem.Attr("style", "max-width: "+js.ToString(width)+"px;")
	} else {
		svgElem.Attr("height", attrValue(height))
		svgElem.Attr("width", attrValue(width))
	}
}
