// Ported from mermaid 12.2.0,
// diagrams/xychart/chartBuilder/components/chartTitle.ts (MIT).

package xychart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// chartTitle is ChartTitle.
type chartTitle struct {
	boundingRect            boundingRect
	showChartTitle          bool
	titleWidth              float64
	textDimensionCalculator textDimensionCalculator
	chartConfig             *js.Obj[any]
	chartData               *xyChartData
	chartThemeConfig        *js.Obj[any]
}

func newChartTitle(
	textDimensionCalculator textDimensionCalculator,
	chartConfig *js.Obj[any],
	chartData *xyChartData,
	chartThemeConfig *js.Obj[any],
) *chartTitle {
	return &chartTitle{
		textDimensionCalculator: textDimensionCalculator,
		chartConfig:             chartConfig,
		chartData:               chartData,
		chartThemeConfig:        chartThemeConfig,
		boundingRect: boundingRect{
			x:      0,
			y:      0,
			width:  0,
			height: 0,
		},
		showChartTitle: false,
	}
}

func (c *chartTitle) setBoundingBoxXY(p point) {
	c.boundingRect.x = p.x
	c.boundingRect.y = p.y
}

func (c *chartTitle) calculateSpace(availableSpace dimension) dimension {
	titleDimension := c.textDimensionCalculator.getMaxDimension(
		[]string{c.chartData.title},
		c.chartConfig.Get("titleFontSize"),
	)
	widthRequired := js.Max(titleDimension.width, availableSpace.width)
	heightRequired := titleDimension.height + float64(2*num(c.chartConfig.Get("titlePadding")))
	if heightRequired <= availableSpace.height &&
		js.Truthy(c.chartConfig.Get("showTitle")) &&
		c.chartData.title != "" {
		c.boundingRect.width = widthRequired
		c.boundingRect.height = heightRequired
		c.titleWidth = titleDimension.width
		c.showChartTitle = true
	}

	return dimension{
		width:  c.boundingRect.width,
		height: c.boundingRect.height,
	}
}

// alignTo centres the title over [start, end], shifted as needed to keep it
// inside the chart. The title has its own row above the plot and legend, so
// it may use the full chart width.
func (c *chartTitle) alignTo(start, end float64) {
	width := num(c.chartConfig.Get("width"))
	halfWidth := float64(js.Min(c.titleWidth, width) / 2)
	center := js.Min(
		js.Max((start+end)/2, halfWidth),
		width-halfWidth,
	)
	c.boundingRect.x = center - halfWidth
	c.boundingRect.width = halfWidth * 2
}

func (c *chartTitle) getDrawableElements() ([]drawableElem, error) {
	drawableElem := []drawableElem{}
	if c.showChartTitle {
		drawableElem = append(drawableElem, drawableElemOfTexts(
			[]string{"chart-title"},
			[]textElem{
				{
					fontSize:      c.chartConfig.Get("titleFontSize"),
					text:          c.chartData.title,
					verticalPos:   "middle",
					horizontalPos: "center",
					x:             c.boundingRect.x + float64(c.boundingRect.width/2),
					y:             c.boundingRect.y + float64(c.boundingRect.height/2),
					fill:          c.chartThemeConfig.Get("titleColor"),
					rotation:      0,
				},
			},
		))
	}
	return drawableElem, nil
}

// getChartTitleComponent is getChartTitleComponent(chartConfig, chartData,
// chartThemeConfig, tmpSVGGroup).
func getChartTitleComponent(
	chartConfig *js.Obj[any],
	chartData *xyChartData,
	chartThemeConfig *js.Obj[any],
	tmpSVGGroup *dom.Node,
) *chartTitle {
	textDimensionCalculator := &textDimensionCalculatorWithFont{parentGroup: tmpSVGGroup}
	return newChartTitle(textDimensionCalculator, chartConfig, chartData, chartThemeConfig)
}
