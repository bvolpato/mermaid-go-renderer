// Ported from mermaid 12.2.0,
// diagrams/xychart/chartBuilder/components/axis/index.ts (MIT).

package xychart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// axisPosition is AxisPosition: "left", "right", "top" or "bottom".
type axisPosition string

// axis is Axis.
type axis interface {
	chartComponent
	// getScaleValue returns a number, or nil where upstream returns
	// undefined: the scale does not know the value.
	getScaleValue(value any) any
	setAxisPosition(axisPosition axisPosition)
	getAxisOuterPadding() float64
	getTickDistance() float64
	recalculateOuterPaddingToDrawBar()
	setRange(rng [2]float64)
}

// getAxis is getAxis(data, axisConfig, axisThemeConfig, tmpSVGGroup).
func getAxis(
	data *axisDataType,
	axisConfig any,
	axisThemeConfig xyChartAxisThemeConfig,
	tmpSVGGroup *dom.Node,
) (axis, error) {
	textDimensionCalculator := &textDimensionCalculatorWithFont{parentGroup: tmpSVGGroup}
	if isBandAxisData(data) {
		return newBandAxis(
			axisConfig,
			axisThemeConfig,
			data.categories,
			data.title,
			textDimensionCalculator,
		)
	}
	return newLinearAxis(
		axisConfig,
		axisThemeConfig,
		[2]float64{data.min, data.max},
		data.title,
		textDimensionCalculator,
	)
}
