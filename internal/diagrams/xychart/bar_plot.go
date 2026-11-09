// Ported from mermaid 12.2.0,
// diagrams/xychart/chartBuilder/components/plot/barPlot.ts (MIT).

package xychart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// barPlot is BarPlot.
type barPlot struct {
	barData      *plotData
	boundingRect *boundingRect
	xAxis        axis
	yAxis        axis
	// orientation is chartConfig.chartOrientation.
	orientation any
	plotIndex   int
}

func (p *barPlot) getDrawableElement() []drawableElem {
	// finalData is [number, number][]. A scale gives undefined (nil) for a
	// value that it does not know.
	finalData := make([][2]any, len(p.barData.data))
	for i, d := range p.barData.data {
		finalData[i] = [2]any{
			p.xAxis.getScaleValue(d.x),
			p.yAxis.getScaleValue(d.y),
		}
	}

	barPaddingPercent := 0.05

	barWidth := js.Min(p.xAxis.getAxisOuterPadding()*2, p.xAxis.getTickDistance()) *
		(1 - barPaddingPercent)
	barWidthHalf := float64(barWidth / 2)

	groupTexts := []string{"plot", "bar-plot-" + js.Str(float64(p.plotIndex))}
	rects := make([]rectElem, len(finalData))
	if js.StrictEquals(p.orientation, "horizontal") {
		for i, data := range finalData {
			rects[i] = rectElem{
				x:           p.boundingRect.x,
				y:           num(data[0]) - barWidthHalf,
				height:      barWidth,
				width:       num(data[1]) - p.boundingRect.x,
				fill:        p.barData.fill,
				strokeWidth: 0.0,
				strokeFill:  p.barData.fill,
			}
		}
		return []drawableElem{drawableElemOfRects(groupTexts, rects)}
	}
	for i, data := range finalData {
		rects[i] = rectElem{
			x:           num(data[0]) - barWidthHalf,
			y:           data[1],
			width:       barWidth,
			height:      p.boundingRect.y + p.boundingRect.height - num(data[1]),
			fill:        p.barData.fill,
			strokeWidth: 0.0,
			strokeFill:  p.barData.fill,
		}
	}
	return []drawableElem{drawableElemOfRects(groupTexts, rects)}
}
