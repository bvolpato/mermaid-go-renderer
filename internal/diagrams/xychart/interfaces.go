// Ported from mermaid 12.2.0, diagrams/xychart/chartBuilder/interfaces.ts
// (MIT).

package xychart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The chart config (XYChartConfig), the axis config (XYChartAxisConfig) and
// the theme config (XYChartThemeConfig) are plain objects that upstream
// builds with cleanAndMerge. The user can put a value of any type in them,
// so that they stay dynamic values here: a *js.Obj[any], read with prop.

// xyChartAxisThemeConfig is XYChartAxisThemeConfig.
type xyChartAxisThemeConfig struct {
	titleColor    any
	labelColor    any
	tickColor     any
	axisLineColor any
}

// chartComponent is ChartComponent.
type chartComponent interface {
	calculateSpace(availableSpace dimension) dimension
	setBoundingBoxXY(p point)
	getDrawableElements() ([]drawableElem, error)
}

// plotDatum is one element of SimplePlotDataType: [string, number]. The
// number is nil where upstream has undefined: a band axis that has more
// categories than the plot has values.
type plotDatum struct {
	x string
	y any
}

// plotData is PlotData: LinePlotData or BarPlotData.
type plotData struct {
	// typ is "line" or "bar".
	typ string
	// title is the title of this plot, or "" if there is no title.
	title string
	data  []plotDatum

	// Fields of a line plot.
	strokeFill     string
	strokeWidth    float64
	pointLabels    []string
	hasPointLabels bool

	// Field of a bar plot.
	fill string
}

// isBarPlot is isBarPlot(data).
func isBarPlot(data *plotData) bool {
	return data.typ == "bar"
}

// axisDataType is AxisDataType: BandAxisDataType or LinearAxisDataType.
type axisDataType struct {
	// typ is "band" or "linear".
	typ   string
	title string
	// categories is the field of a band axis.
	categories []string
	// min and max are the fields of a linear axis.
	min, max float64
}

// isBandAxisData is isBandAxisData(data).
func isBandAxisData(data *axisDataType) bool {
	return data.typ == "band"
}

// isLinearAxisData is isLinearAxisData(data).
func isLinearAxisData(data *axisDataType) bool {
	return data.typ == "linear"
}

// xyChartData is XYChartData.
type xyChartData struct {
	xAxis *axisDataType
	yAxis *axisDataType
	title string
	plots []*plotData
}

// dimension is Dimension.
type dimension struct {
	width, height float64
}

// boundingRect is BoundingRect.
type boundingRect struct {
	x, y, width, height float64
}

// point is Point.
type point struct {
	x, y float64
}

// The elements below hold dynamic values where upstream can produce a value
// that is not a number: the + operator on a config value that is a string
// gives a string, and a scale gives undefined for a value that it does not
// know. A float64 is a number, a string is a string and nil is undefined.

// rectElem is RectElem.
type rectElem struct {
	x, y          any
	width, height float64
	fill          any
	strokeWidth   any
	strokeFill    any
}

// textElem is TextElem.
type textElem struct {
	text          string
	x, y          any
	fill          any
	verticalPos   string // "top" or "middle"
	horizontalPos string // "left", "center" or "right"
	fontSize      any
	rotation      float64
}

// pathElem is PathElem.
type pathElem struct {
	path        string
	fill        any
	strokeWidth any
	strokeFill  any
}

// drawableElem is DrawableElem. typ is "rect", "text" or "path", and the
// slice of that type holds the data.
type drawableElem struct {
	groupTexts []string
	typ        string
	rects      []rectElem
	texts      []textElem
	paths      []pathElem
}

// dataLength is shape.data.length.
func (d *drawableElem) dataLength() int {
	switch d.typ {
	case "rect":
		return len(d.rects)
	case "text":
		return len(d.texts)
	case "path":
		return len(d.paths)
	}
	return 0
}

// typeError is a JavaScript TypeError that the ported code throws.
type typeError struct{ message string }

func (e *typeError) Error() string { return e.message }

// prop is the property read o.key. The second result is false where
// JavaScript throws a TypeError: o is undefined or null.
func prop(o any, key string) (any, error) {
	v, ok := js.GetProperty(o, key)
	if !ok {
		return nil, &typeError{"Cannot read properties of " + js.ToString(o) + " (reading '" + key + "')"}
	}
	return v, nil
}

// get is the property read o.key for an object that the caller knows is not
// undefined or null.
func get(o any, key string) any {
	v, _ := js.GetProperty(o, key)
	return v
}

// num is the conversion of a value that JavaScript uses as a number: the
// operand of *, / and -, and the argument of the Math functions.
func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case []any, *js.Obj[any]:
		// ToPrimitive gives the string form of an object.
		return js.Number(js.ToString(x))
	}
	return js.ToNumber(v)
}

// attrValue converts a value for dom.Node.Attr. d3 removes the attribute for
// null and for undefined.
func attrValue(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// drawableElemOfRects returns the DrawableElem of type "rect".
func drawableElemOfRects(groupTexts []string, data []rectElem) drawableElem {
	return drawableElem{groupTexts: groupTexts, typ: "rect", rects: data}
}

// drawableElemOfTexts returns the DrawableElem of type "text".
func drawableElemOfTexts(groupTexts []string, data []textElem) drawableElem {
	return drawableElem{groupTexts: groupTexts, typ: "text", texts: data}
}

// drawableElemOfPaths returns the DrawableElem of type "path".
func drawableElemOfPaths(groupTexts []string, data []pathElem) drawableElem {
	return drawableElem{groupTexts: groupTexts, typ: "path", paths: data}
}
