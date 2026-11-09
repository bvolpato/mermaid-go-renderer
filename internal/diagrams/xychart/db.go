// Ported from mermaid 12.2.0, diagrams/xychart/xychartDb.ts (MIT).

package xychart

import (
	"errors"
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/theme"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// db is the state of xychartDb.ts. Upstream keeps it in module-level
// variables. Here every render has its own.
type db struct {
	render.CommonDB
	ctx *render.Context

	plotIndex   int
	tmpSVGGroup *dom.Node

	// xyChartConfig is XYChartConfig and xyChartThemeConfig is
	// XYChartThemeConfig: the objects that cleanAndMerge returns.
	xyChartConfig      *js.Obj[any]
	xyChartThemeConfig *js.Obj[any]
	xyChartData        *xyChartData
	plotColorPalette   []string
	hasSetXAxis        bool
	hasSetYAxis        bool

	// initErr is the error that upstream throws from clear(), before the
	// parser runs.
	initErr error
	// drawable is the last result of getDrawableElem.
	drawable []drawableElem
}

// normalTextType is NormalTextType: the value of the "text" rule of the
// grammar.
type normalTextType struct {
	typ  string
	text string
}

// parsedDataPoint is ParsedDataPoint.
type parsedDataPoint struct {
	value float64
	label string
}

// newDB returns the db after clear().
func newDB(ctx *render.Context) *db {
	d := &db{ctx: ctx}
	d.Sanitize = ctx.SanitizeText
	d.clear()
	return d
}

// getChartDefaultThemeConfig is getChartDefaultThemeConfig().
func (d *db) getChartDefaultThemeConfig() (*js.Obj[any], error) {
	defaultThemeVariables, err := theme.GetThemeVariables("default", nil)
	if err != nil {
		return nil, err
	}
	themeVariables := d.ctx.Config.Get("themeVariables")
	xyChart, err := prop(themeVariables, "xyChart")
	if err != nil {
		return nil, err
	}
	return utils.CleanAndMerge(defaultThemeVariables.Get("xyChart"), xyChart), nil
}

// getChartDefaultConfig is getChartDefaultConfig().
func (d *db) getChartDefaultConfig() *js.Obj[any] {
	return utils.CleanAndMerge(config.DefaultSection("xyChart"), d.ctx.Config.Get("xyChart"))
}

// getChartDefaultData is getChartDefaultData().
func getChartDefaultData() *xyChartData {
	return &xyChartData{
		yAxis: &axisDataType{
			typ:   "linear",
			title: "",
			min:   math.Inf(1),
			max:   math.Inf(-1),
		},
		xAxis: &axisDataType{
			typ:        "band",
			title:      "",
			categories: []string{},
		},
		title: "",
		plots: []*plotData{},
	}
}

// textSanitizer is textSanitizer(text).
func (d *db) textSanitizer(text string) string {
	return d.ctx.SanitizeText(js.TrimSpace(text))
}

// setTmpSVGG is setTmpSVGG(SVGG).
func (d *db) setTmpSVGG(svgg *dom.Node) {
	d.tmpSVGGroup = svgg
}

// setOrientation is setOrientation(orientation).
func (d *db) setOrientation(orientation string) {
	if orientation == "horizontal" {
		d.xyChartConfig.Set("chartOrientation", "horizontal")
	} else {
		d.xyChartConfig.Set("chartOrientation", "vertical")
	}
}

// setXAxisTitle is setXAxisTitle(title).
func (d *db) setXAxisTitle(title normalTextType) {
	d.xyChartData.xAxis.title = d.textSanitizer(title.text)
}

// setXAxisRangeData is setXAxisRangeData(min, max).
func (d *db) setXAxisRangeData(min, max float64) {
	d.xyChartData.xAxis = &axisDataType{typ: "linear", title: d.xyChartData.xAxis.title, min: min, max: max}
	d.hasSetXAxis = true
}

// setXAxisBand is setXAxisBand(categories).
func (d *db) setXAxisBand(categories []normalTextType) {
	texts := make([]string, len(categories))
	for i, c := range categories {
		texts[i] = d.textSanitizer(c.text)
	}
	d.xyChartData.xAxis = &axisDataType{
		typ:        "band",
		title:      d.xyChartData.xAxis.title,
		categories: texts,
	}
	d.hasSetXAxis = true
}

// setYAxisTitle is setYAxisTitle(title).
func (d *db) setYAxisTitle(title normalTextType) {
	d.xyChartData.yAxis.title = d.textSanitizer(title.text)
}

// setYAxisRangeData is setYAxisRangeData(min, max).
func (d *db) setYAxisRangeData(min, max float64) {
	d.xyChartData.yAxis = &axisDataType{typ: "linear", title: d.xyChartData.yAxis.title, min: min, max: max}
	d.hasSetYAxis = true
}

// setYAxisRangeFromPlotData is setYAxisRangeFromPlotData(data). This function
// does not set hasSetYAxis as there can be multiple data, so the range is
// calculated accordingly.
func (d *db) setYAxisRangeFromPlotData(data []float64) {
	minValue := js.Min(data...)
	maxValue := js.Max(data...)
	prevMinValue := math.Inf(1)
	prevMaxValue := math.Inf(-1)
	if isLinearAxisData(d.xyChartData.yAxis) {
		prevMinValue = d.xyChartData.yAxis.min
		prevMaxValue = d.xyChartData.yAxis.max
	}
	d.xyChartData.yAxis = &axisDataType{
		typ:   "linear",
		title: d.xyChartData.yAxis.title,
		min:   js.Min(prevMinValue, minValue),
		max:   js.Max(prevMaxValue, maxValue),
	}
}

// transformDataWithoutCategory is transformDataWithoutCategory(data).
func (d *db) transformDataWithoutCategory(data []float64) []plotDatum {
	retData := []plotDatum{}
	if len(data) == 0 {
		return retData
	}
	if !d.hasSetXAxis {
		prevMinValue := math.Inf(1)
		prevMaxValue := math.Inf(-1)
		if isLinearAxisData(d.xyChartData.xAxis) {
			prevMinValue = d.xyChartData.xAxis.min
			prevMaxValue = d.xyChartData.xAxis.max
		}
		d.setXAxisRangeData(js.Min(prevMinValue, 1), js.Max(prevMaxValue, float64(len(data))))
	}

	// When a band axis is defined, truncate data to match the number of
	// categories to prevent orphaned bars/lines from rendering in unlabeled
	// chart space. This also ensures the Y-axis range is computed only from
	// visible data points.
	if isBandAxisData(d.xyChartData.xAxis) && len(data) > len(d.xyChartData.xAxis.categories) {
		data = data[:len(d.xyChartData.xAxis.categories)]
	}

	if !d.hasSetYAxis {
		d.setYAxisRangeFromPlotData(data)
	}

	if isBandAxisData(d.xyChartData.xAxis) {
		categories := d.xyChartData.xAxis.categories
		retData = make([]plotDatum, len(categories))
		for i, c := range categories {
			// data[i] is undefined after the end of the data.
			var y any
			if i < len(data) {
				y = data[i]
			}
			retData[i] = plotDatum{x: c, y: y}
		}
	}

	if isLinearAxisData(d.xyChartData.xAxis) {
		min := d.xyChartData.xAxis.min
		max := d.xyChartData.xAxis.max
		if len(data) == 1 {
			// For backwards compatibility, avoid divide-by-zero error
			retData = []plotDatum{{x: js.Str(min), y: data[0]}}
		} else {
			step := (max - min) / float64(len(data)-1)
			retData = make([]plotDatum, len(data))
			for index, datum := range data {
				retData[index] = plotDatum{x: js.Str(min + float64(float64(index)*step)), y: datum}
			}
		}
	}

	return retData
}

// getPlotColorFromPalette is getPlotColorFromPalette(plotIndex).
func (d *db) getPlotColorFromPalette(plotIndex int) string {
	if plotIndex == 0 {
		return d.plotColorPalette[0]
	}
	return d.plotColorPalette[plotIndex%len(d.plotColorPalette)]
}

// setLineData is setLineData(title, data).
func (d *db) setLineData(title normalTextType, data []parsedDataPoint) {
	values := make([]float64, len(data))
	for i, p := range data {
		values[i] = p.value
	}
	labels := make([]string, len(data))
	for i, p := range data {
		if p.label != "" {
			labels[i] = d.textSanitizer(p.label)
		}
	}
	plot := d.transformDataWithoutCategory(values)
	hasAnyLabel := false
	for _, l := range labels {
		if l != "" {
			hasAnyLabel = true
			break
		}
	}
	p := &plotData{
		typ:         "line",
		title:       d.textSanitizer(title.text),
		strokeFill:  d.getPlotColorFromPalette(d.plotIndex),
		strokeWidth: 2,
		data:        plot,
	}
	if hasAnyLabel {
		p.pointLabels, p.hasPointLabels = labels, true
	}
	d.xyChartData.plots = append(d.xyChartData.plots, p)
	d.plotIndex++
}

// setBarData is setBarData(title, data).
func (d *db) setBarData(title normalTextType, data []parsedDataPoint) {
	values := make([]float64, len(data))
	for i, p := range data {
		values[i] = p.value
	}
	plot := d.transformDataWithoutCategory(values)
	d.xyChartData.plots = append(d.xyChartData.plots, &plotData{
		typ:   "bar",
		title: d.textSanitizer(title.text),
		fill:  d.getPlotColorFromPalette(d.plotIndex),
		data:  plot,
	})
	d.plotIndex++
}

// getDrawableElem is getDrawableElem().
func (d *db) getDrawableElem() ([]drawableElem, error) {
	if len(d.xyChartData.plots) == 0 {
		return nil, errors.New("No Plot to render, please provide a plot with some data")
	}
	d.xyChartData.title = d.GetDiagramTitle()
	shapes, err := xyChartBuilderBuild(d.xyChartConfig, d.xyChartData, d.xyChartThemeConfig, d.tmpSVGGroup)
	if err != nil {
		return nil, err
	}
	d.drawable = shapes
	return shapes, nil
}

// getChartThemeConfig is getChartThemeConfig().
func (d *db) getChartThemeConfig() *js.Obj[any] {
	return d.xyChartThemeConfig
}

// getChartConfig is getChartConfig().
func (d *db) getChartConfig() *js.Obj[any] {
	return d.xyChartConfig
}

// getXYChartData is getXYChartData().
func (d *db) getXYChartData() *xyChartData {
	return d.xyChartData
}

// clear is clear().
func (d *db) clear() {
	d.ClearCommon()
	d.plotIndex = 0
	d.xyChartConfig = d.getChartDefaultConfig()
	d.xyChartData = getChartDefaultData()
	d.initErr = nil
	themeConfig, err := d.getChartDefaultThemeConfig()
	if err != nil {
		d.initErr = err
		themeConfig = js.NewObj[any]()
	}
	d.xyChartThemeConfig = themeConfig
	// plotColorPalette.split(',').map((color) => color.trim())
	paletteValue := themeConfig.Get("plotColorPalette")
	palette, isString := paletteValue.(string)
	if !isString {
		if d.initErr == nil {
			if js.IsNullish(paletteValue) {
				d.initErr = &typeError{"Cannot read properties of " + js.ToString(paletteValue) + " (reading 'split')"}
			} else {
				d.initErr = &typeError{"xyChartThemeConfig.plotColorPalette.split is not a function"}
			}
		}
		palette = ""
	}
	parts := strings.Split(palette, ",")
	d.plotColorPalette = make([]string, len(parts))
	for i, color := range parts {
		d.plotColorPalette[i] = js.TrimSpace(color)
	}
	d.hasSetXAxis = false
	d.hasSetYAxis = false
}
