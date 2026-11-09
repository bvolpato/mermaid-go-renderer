// Ported from mermaid 12.2.0,
// diagrams/xychart/chartBuilder/components/legend.ts (MIT).

package xychart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

const (
	legendMarkerToFontRatio        = 0.75
	legendItemSpacingToFontRatio   = 0.5
	legendMarkerSpacingToFontRatio = 0.35
)

// legendLayout is LegendLayout. fontSize is the config value as it is.
type legendLayout struct {
	fontSize      any
	markerSize    float64
	markerSpacing float64
	itemSpacing   float64
}

// getLegendLayout is getLegendLayout(fontSize).
func getLegendLayout(fontSize any) legendLayout {
	return legendLayout{
		fontSize:      fontSize,
		markerSize:    num(fontSize) * legendMarkerToFontRatio,
		markerSpacing: num(fontSize) * legendMarkerSpacingToFontRatio,
		itemSpacing:   num(fontSize) * legendItemSpacingToFontRatio,
	}
}

// chartLegend is ChartLegend.
type chartLegend struct {
	boundingRect            boundingRect
	visiblePlots            []*plotData
	textDimensionCalculator textDimensionCalculator
	chartConfig             *js.Obj[any]
	chartData               *xyChartData
	chartThemeConfig        *js.Obj[any]
}

func newChartLegend(
	textDimensionCalculator textDimensionCalculator,
	chartConfig *js.Obj[any],
	chartData *xyChartData,
	chartThemeConfig *js.Obj[any],
) *chartLegend {
	return &chartLegend{
		boundingRect:            boundingRect{x: 0, y: 0, width: 0, height: 0},
		visiblePlots:            []*plotData{},
		textDimensionCalculator: textDimensionCalculator,
		chartConfig:             chartConfig,
		chartData:               chartData,
		chartThemeConfig:        chartThemeConfig,
	}
}

func (c *chartLegend) setBoundingBoxXY(p point) {
	c.boundingRect.x = p.x
	c.boundingRect.y = p.y
}

func (c *chartLegend) calculateSpace(availableSpace dimension) dimension {
	c.visiblePlots = []*plotData{}
	if js.Truthy(c.chartConfig.Get("showLegend")) {
		for _, plot := range c.chartData.plots {
			if plot.title != "" {
				c.visiblePlots = append(c.visiblePlots, plot)
			}
		}
	}

	if len(c.visiblePlots) == 0 {
		c.boundingRect.width = 0
		c.boundingRect.height = 0
		return dimension{width: 0, height: 0}
	}

	layout := getLegendLayout(c.chartConfig.Get("legendFontSize"))
	fontSize, markerSize, markerSpacing, itemSpacing := layout.fontSize, layout.markerSize, layout.markerSpacing, layout.itemSpacing
	titles := make([]string, len(c.visiblePlots))
	for i, plot := range c.visiblePlots {
		titles[i] = plot.title
	}
	textDimension := c.textDimensionCalculator.getMaxDimension(titles, fontSize)

	legendPadding := c.chartConfig.Get("legendPadding")
	count := float64(len(c.visiblePlots))
	widthRequired := float64(num(legendPadding)*2) + markerSize + markerSpacing + textDimension.width
	heightRequired := float64(num(legendPadding)*2) +
		float64(count*num(fontSize)) +
		float64((count-1)*itemSpacing)

	if widthRequired <= availableSpace.width && heightRequired <= availableSpace.height {
		c.boundingRect.width = widthRequired
		c.boundingRect.height = heightRequired
	} else {
		c.visiblePlots = []*plotData{}
		c.boundingRect.width = 0
		c.boundingRect.height = 0
	}

	return dimension{
		width:  c.boundingRect.width,
		height: c.boundingRect.height,
	}
}

func (c *chartLegend) getDrawableElements() ([]drawableElem, error) {
	if len(c.visiblePlots) == 0 {
		return []drawableElem{}, nil
	}

	layout := getLegendLayout(c.chartConfig.Get("legendFontSize"))
	fontSize, markerSize, markerSpacing, itemSpacing := layout.fontSize, layout.markerSize, layout.markerSpacing, layout.itemSpacing
	// The + operator has a config value as an operand in the next three
	// lines: the result is a string when the config value is a string.
	rowHeight := js.Add(fontSize, itemSpacing)
	startX := js.Add(c.boundingRect.x, c.chartConfig.Get("legendPadding"))
	startY := js.Add(c.boundingRect.y, c.chartConfig.Get("legendPadding"))
	barMarkers := []rectElem{}
	lineMarkers := []pathElem{}

	for index, plot := range c.visiblePlots {
		if isBarPlot(plot) {
			barMarkers = append(barMarkers, rectElem{
				x:           startX,
				y:           js.Add(startY, float64(index)*num(rowHeight)),
				width:       markerSize,
				height:      markerSize,
				fill:        plot.fill,
				strokeFill:  plot.fill,
				strokeWidth: 0.0,
			})
		} else {
			markerY := js.Add(js.Add(startY, float64(index)*num(rowHeight)), markerSize/2)
			lineMarkers = append(lineMarkers, pathElem{
				path:        "M " + js.ToString(startX) + "," + js.ToString(markerY) + " L " + js.ToString(js.Add(startX, markerSize)) + "," + js.ToString(markerY),
				strokeFill:  plot.strokeFill,
				strokeWidth: plot.strokeWidth,
			})
		}
	}

	labels := make([]textElem, len(c.visiblePlots))
	for index, plot := range c.visiblePlots {
		labels[index] = textElem{
			text:          plot.title,
			x:             js.Add(js.Add(startX, markerSize), markerSpacing),
			y:             js.Add(js.Add(startY, float64(index)*num(rowHeight)), markerSize/2),
			fill:          c.chartThemeConfig.Get("legendTextColor"),
			fontSize:      fontSize,
			rotation:      0,
			verticalPos:   "middle",
			horizontalPos: "left",
		}
	}

	return []drawableElem{
		drawableElemOfRects([]string{"legend", "markers"}, barMarkers),
		drawableElemOfPaths([]string{"legend", "markers"}, lineMarkers),
		drawableElemOfTexts([]string{"legend", "label"}, labels),
	}, nil
}

// getChartLegendComponent is getChartLegendComponent(chartConfig, chartData,
// chartThemeConfig, tmpSVGGroup).
func getChartLegendComponent(
	chartConfig *js.Obj[any],
	chartData *xyChartData,
	chartThemeConfig *js.Obj[any],
	tmpSVGGroup *dom.Node,
) *chartLegend {
	textDimensionCalculator := &textDimensionCalculatorWithFont{parentGroup: tmpSVGGroup}
	return newChartLegend(textDimensionCalculator, chartConfig, chartData, chartThemeConfig)
}
