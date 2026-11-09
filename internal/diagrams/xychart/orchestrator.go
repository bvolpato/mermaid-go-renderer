// Ported from mermaid 12.2.0, diagrams/xychart/chartBuilder/orchestrator.ts
// and diagrams/xychart/chartBuilder/index.ts (MIT).

package xychart

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// orchestrator is Orchestrator.
type orchestrator struct {
	// The components, in the order of the componentStore object.
	title  *chartTitle
	plot   plot
	legend chartComponent
	xAxis  axis
	yAxis  axis

	chartConfig *js.Obj[any]
	chartData   *xyChartData
}

func newOrchestrator(
	chartConfig *js.Obj[any],
	chartData *xyChartData,
	chartThemeConfig *js.Obj[any],
	tmpSVGGroup *dom.Node,
) (*orchestrator, error) {
	o := &orchestrator{chartConfig: chartConfig, chartData: chartData}
	o.title = getChartTitleComponent(chartConfig, chartData, chartThemeConfig, tmpSVGGroup)
	o.plot = getPlotComponent(chartConfig, chartData, chartThemeConfig)
	o.legend = getChartLegendComponent(chartConfig, chartData, chartThemeConfig, tmpSVGGroup)
	var err error
	o.xAxis, err = getAxis(
		chartData.xAxis,
		chartConfig.Get("xAxis"),
		xyChartAxisThemeConfig{
			titleColor:    chartThemeConfig.Get("xAxisTitleColor"),
			labelColor:    chartThemeConfig.Get("xAxisLabelColor"),
			tickColor:     chartThemeConfig.Get("xAxisTickColor"),
			axisLineColor: chartThemeConfig.Get("xAxisLineColor"),
		},
		tmpSVGGroup,
	)
	if err != nil {
		return nil, err
	}
	o.yAxis, err = getAxis(
		chartData.yAxis,
		chartConfig.Get("yAxis"),
		xyChartAxisThemeConfig{
			titleColor:    chartThemeConfig.Get("yAxisTitleColor"),
			labelColor:    chartThemeConfig.Get("yAxisLabelColor"),
			tickColor:     chartThemeConfig.Get("yAxisTickColor"),
			axisLineColor: chartThemeConfig.Get("yAxisLineColor"),
		},
		tmpSVGGroup,
	)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// hasBarPlot is this.chartData.plots.some((p) => isBarPlot(p)).
func (o *orchestrator) hasBarPlot() bool {
	for _, p := range o.chartData.plots {
		if isBarPlot(p) {
			return true
		}
	}
	return false
}

func (o *orchestrator) calculateVerticalSpace() {
	availableWidth := num(o.chartConfig.Get("width"))
	availableHeight := num(o.chartConfig.Get("height"))
	plotX := 0.0
	plotY := 0.0
	legendSpace := dimension{width: 0, height: 0}
	plotReservedSpacePercent := num(o.chartConfig.Get("plotReservedSpacePercent"))
	chartWidth := math.Floor((availableWidth * plotReservedSpacePercent) / 100)
	chartHeight := math.Floor(
		(availableHeight * plotReservedSpacePercent) / 100,
	)
	spaceUsed := o.plot.calculateSpace(dimension{
		width:  chartWidth,
		height: chartHeight,
	})
	availableWidth -= spaceUsed.width
	availableHeight -= spaceUsed.height

	spaceUsed = o.title.calculateSpace(dimension{
		width:  num(o.chartConfig.Get("width")),
		height: availableHeight,
	})
	plotY = spaceUsed.height
	availableHeight -= spaceUsed.height
	o.xAxis.setAxisPosition("bottom")
	spaceUsed = o.xAxis.calculateSpace(dimension{
		width:  availableWidth,
		height: availableHeight,
	})
	availableHeight -= spaceUsed.height
	o.yAxis.setAxisPosition("left")
	spaceUsed = o.yAxis.calculateSpace(dimension{
		width:  availableWidth,
		height: availableHeight,
	})
	plotX = spaceUsed.width
	availableWidth -= spaceUsed.width
	legendSpace = o.legend.calculateSpace(dimension{
		width:  availableWidth,
		height: chartHeight,
	})
	availableWidth -= legendSpace.width
	if availableWidth > 0 {
		chartWidth += availableWidth
		availableWidth = 0
	}
	if availableHeight > 0 {
		chartHeight += availableHeight
		availableHeight = 0
	}
	o.plot.calculateSpace(dimension{
		width:  chartWidth,
		height: chartHeight,
	})

	o.plot.setBoundingBoxXY(point{x: plotX, y: plotY})
	o.title.alignTo(plotX, plotX+chartWidth)
	o.legend.setBoundingBoxXY(point{
		x: plotX + chartWidth,
		y: plotY + js.Max((chartHeight-legendSpace.height)/2, 0),
	})
	o.xAxis.setRange([2]float64{plotX, plotX + chartWidth})
	o.xAxis.setBoundingBoxXY(point{x: plotX, y: plotY + chartHeight})
	o.yAxis.setRange([2]float64{plotY, plotY + chartHeight})
	o.yAxis.setBoundingBoxXY(point{x: 0, y: plotY})
	if o.hasBarPlot() {
		o.xAxis.recalculateOuterPaddingToDrawBar()
	}
}

func (o *orchestrator) calculateHorizontalSpace() {
	availableWidth := num(o.chartConfig.Get("width"))
	availableHeight := num(o.chartConfig.Get("height"))
	titleYEnd := 0.0
	plotX := 0.0
	plotY := 0.0
	legendSpace := dimension{width: 0, height: 0}
	plotReservedSpacePercent := num(o.chartConfig.Get("plotReservedSpacePercent"))
	chartWidth := math.Floor((availableWidth * plotReservedSpacePercent) / 100)
	chartHeight := math.Floor(
		(availableHeight * plotReservedSpacePercent) / 100,
	)
	spaceUsed := o.plot.calculateSpace(dimension{
		width:  chartWidth,
		height: chartHeight,
	})
	availableWidth -= spaceUsed.width
	availableHeight -= spaceUsed.height

	spaceUsed = o.title.calculateSpace(dimension{
		width:  num(o.chartConfig.Get("width")),
		height: availableHeight,
	})
	titleYEnd = spaceUsed.height
	availableHeight -= spaceUsed.height
	o.xAxis.setAxisPosition("left")
	spaceUsed = o.xAxis.calculateSpace(dimension{
		width:  availableWidth,
		height: availableHeight,
	})
	availableWidth -= spaceUsed.width
	plotX = spaceUsed.width
	o.yAxis.setAxisPosition("top")
	spaceUsed = o.yAxis.calculateSpace(dimension{
		width:  availableWidth,
		height: availableHeight,
	})
	availableHeight -= spaceUsed.height
	plotY = titleYEnd + spaceUsed.height
	legendSpace = o.legend.calculateSpace(dimension{
		width:  availableWidth,
		height: chartHeight,
	})
	availableWidth -= legendSpace.width
	if availableWidth > 0 {
		chartWidth += availableWidth
		availableWidth = 0
	}
	if availableHeight > 0 {
		chartHeight += availableHeight
		availableHeight = 0
	}
	o.plot.calculateSpace(dimension{
		width:  chartWidth,
		height: chartHeight,
	})

	o.plot.setBoundingBoxXY(point{x: plotX, y: plotY})
	o.title.alignTo(plotX, plotX+chartWidth)
	o.legend.setBoundingBoxXY(point{
		x: plotX + chartWidth,
		y: plotY + js.Max((chartHeight-legendSpace.height)/2, 0),
	})
	o.yAxis.setRange([2]float64{plotX, plotX + chartWidth})
	o.yAxis.setBoundingBoxXY(point{x: plotX, y: titleYEnd})
	o.xAxis.setRange([2]float64{plotY, plotY + chartHeight})
	o.xAxis.setBoundingBoxXY(point{x: 0, y: plotY})
	if o.hasBarPlot() {
		o.xAxis.recalculateOuterPaddingToDrawBar()
	}
}

func (o *orchestrator) calculateSpace() {
	if js.StrictEquals(o.chartConfig.Get("chartOrientation"), "horizontal") {
		o.calculateHorizontalSpace()
	} else {
		o.calculateVerticalSpace()
	}
}

func (o *orchestrator) getDrawableElement() ([]drawableElem, error) {
	o.calculateSpace()
	drawableElem := []drawableElem{}
	o.plot.setAxes(o.xAxis, o.yAxis)
	// Object.values(this.componentStore): title, plot, legend, xAxis, yAxis.
	for _, component := range []chartComponent{o.title, o.plot, o.legend, o.xAxis, o.yAxis} {
		elements, err := component.getDrawableElements()
		if err != nil {
			return nil, err
		}
		drawableElem = append(drawableElem, elements...)
	}
	return drawableElem, nil
}

// xyChartBuilderBuild is XYChartBuilder.build(config, chartData,
// chartThemeConfig, tmpSVGGroup).
func xyChartBuilderBuild(
	config *js.Obj[any],
	chartData *xyChartData,
	chartThemeConfig *js.Obj[any],
	tmpSVGGroup *dom.Node,
) ([]drawableElem, error) {
	orchestrator, err := newOrchestrator(config, chartData, chartThemeConfig, tmpSVGGroup)
	if err != nil {
		return nil, err
	}
	return orchestrator.getDrawableElement()
}
