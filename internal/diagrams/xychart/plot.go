// Ported from mermaid 12.2.0,
// diagrams/xychart/chartBuilder/components/plot/index.ts (MIT).

package xychart

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// plot is Plot.
type plot interface {
	chartComponent
	setAxes(xAxis, yAxis axis)
}

// basePlot is BasePlot.
type basePlot struct {
	boundingRect     boundingRect
	xAxis            axis
	yAxis            axis
	chartConfig      *js.Obj[any]
	chartData        *xyChartData
	chartThemeConfig *js.Obj[any]
}

func newBasePlot(chartConfig *js.Obj[any], chartData *xyChartData, chartThemeConfig *js.Obj[any]) *basePlot {
	return &basePlot{
		chartConfig:      chartConfig,
		chartData:        chartData,
		chartThemeConfig: chartThemeConfig,
		boundingRect: boundingRect{
			x:      0,
			y:      0,
			width:  0,
			height: 0,
		},
	}
}

func (p *basePlot) setAxes(xAxis, yAxis axis) {
	p.xAxis = xAxis
	p.yAxis = yAxis
}

func (p *basePlot) setBoundingBoxXY(pt point) {
	p.boundingRect.x = pt.x
	p.boundingRect.y = pt.y
}

func (p *basePlot) calculateSpace(availableSpace dimension) dimension {
	p.boundingRect.width = availableSpace.width
	p.boundingRect.height = availableSpace.height

	return dimension{
		width:  p.boundingRect.width,
		height: p.boundingRect.height,
	}
}

func (p *basePlot) getDrawableElements() ([]drawableElem, error) {
	if !(p.xAxis != nil && p.yAxis != nil) {
		return nil, errors.New("Axes must be passed to render Plots")
	}
	drawableElem := []drawableElem{}
	for i, plot := range p.chartData.plots {
		switch plot.typ {
		case "line":
			linePlot := &linePlot{
				plotData:    plot,
				xAxis:       p.xAxis,
				yAxis:       p.yAxis,
				orientation: p.chartConfig.Get("chartOrientation"),
				plotIndex:   i,
			}
			drawableElem = append(drawableElem, linePlot.getDrawableElement()...)
		case "bar":
			barPlot := &barPlot{
				barData:      plot,
				boundingRect: &p.boundingRect,
				xAxis:        p.xAxis,
				yAxis:        p.yAxis,
				orientation:  p.chartConfig.Get("chartOrientation"),
				plotIndex:    i,
			}
			drawableElem = append(drawableElem, barPlot.getDrawableElement()...)
		}
	}
	return drawableElem, nil
}

// getPlotComponent is getPlotComponent(chartConfig, chartData,
// chartThemeConfig).
func getPlotComponent(chartConfig *js.Obj[any], chartData *xyChartData, chartThemeConfig *js.Obj[any]) plot {
	return newBasePlot(chartConfig, chartData, chartThemeConfig)
}
