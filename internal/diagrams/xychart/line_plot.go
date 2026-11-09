// Ported from mermaid 12.2.0,
// diagrams/xychart/chartBuilder/components/plot/linePlot.ts (MIT).

package xychart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/shape"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// linePlot is LinePlot.
type linePlot struct {
	plotData *plotData
	xAxis    axis
	yAxis    axis
	// orientation is chartConfig.chartOrientation.
	orientation any
	plotIndex   int
}

func (p *linePlot) getDrawableElement() []drawableElem {
	// finalData is [number, number][]. A scale gives undefined (nil) for a
	// value that it does not know.
	finalData := make([][2]any, len(p.plotData.data))
	for i, d := range p.plotData.data {
		finalData[i] = [2]any{
			p.xAxis.getScaleValue(d.x),
			p.yAxis.getScaleValue(d.y),
		}
	}

	// d3 converts the result of an accessor with the unary + operator.
	first := func(d [2]any, _ int, _ [][2]any) float64 { return num(d[0]) }
	second := func(d [2]any, _ int, _ [][2]any) float64 { return num(d[1]) }
	horizontal := js.StrictEquals(p.orientation, "horizontal")
	var path string
	if horizontal {
		path = shape.NewLine[[2]any]().
			Y(first).
			X(second).
			Path(finalData)
	} else {
		path = shape.NewLine[[2]any]().
			X(first).
			Y(second).
			Path(finalData)
	}
	if path == "" {
		return []drawableElem{}
	}

	groupTexts := []string{"plot", "line-plot-" + js.Str(float64(p.plotIndex))}
	elements := []drawableElem{
		drawableElemOfPaths(groupTexts, []pathElem{
			{
				path:        path,
				strokeFill:  p.plotData.strokeFill,
				strokeWidth: p.plotData.strokeWidth,
			},
		}),
	}

	if p.plotData.hasPointLabels && len(p.plotData.pointLabels) > 0 {
		labelOffset := 10.0
		fontSize := 12.0
		textData := []textElem{}

		for i, d := range finalData {
			px, py := d[0], d[1]
			// pointLabels[i] is undefined after the end of the labels.
			label := ""
			if i < len(p.plotData.pointLabels) {
				label = p.plotData.pointLabels[i]
			}
			if label == "" {
				continue
			}

			if horizontal {
				textData = append(textData, textElem{
					x:             js.Add(py, labelOffset),
					y:             px,
					text:          label,
					fill:          p.plotData.strokeFill,
					verticalPos:   "middle",
					horizontalPos: "left",
					fontSize:      fontSize,
					rotation:      0,
				})
			} else {
				textData = append(textData, textElem{
					x:             px,
					y:             num(py) - labelOffset,
					text:          label,
					fill:          p.plotData.strokeFill,
					verticalPos:   "middle",
					horizontalPos: "center",
					fontSize:      fontSize,
					rotation:      0,
				})
			}
		}

		if len(textData) > 0 {
			elements = append(elements, drawableElemOfTexts(
				[]string{"plot", "line-plot-" + js.Str(float64(p.plotIndex)), "labels"},
				textData,
			))
		}
	}

	return elements
}
