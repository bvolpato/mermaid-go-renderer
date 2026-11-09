// Ported from mermaid 12.2.0, diagrams/quadrant-chart/quadrantBuilder.ts
// (MIT).

package quadrant

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/scale"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/theme"
)

// StylesObject holds the styles of a point or of a class. A nil pointer is
// a property that the object does not have.
type StylesObject struct {
	Radius      *float64
	Color       *string
	StrokeColor *string
	StrokeWidth *string
}

// merge is { ...classStyles, ...point }: the properties of s win over the
// properties of classStyles.
func (s StylesObject) merge(classStyles StylesObject) StylesObject {
	if s.Radius == nil {
		s.Radius = classStyles.Radius
	}
	if s.Color == nil {
		s.Color = classStyles.Color
	}
	if s.StrokeColor == nil {
		s.StrokeColor = classStyles.StrokeColor
	}
	if s.StrokeWidth == nil {
		s.StrokeWidth = classStyles.StrokeWidth
	}
	return s
}

// QuadrantPointInputType is a point of the diagram text. X and Y are the
// texts of the tokens, as the parser gives them to the db.
type QuadrantPointInputType struct {
	X, Y      string
	Text      string
	ClassName string
	StylesObject
}

// QuadrantTextType is a text with its position. Fill and FontSize are the
// values of a theme variable and of a config value. X and Y are numbers, or
// nil for undefined.
type QuadrantTextType struct {
	Text          string
	Fill          any
	X, Y          any
	VerticalPos   string // "left", "center" or "right"
	HorizontalPos string // "top", "middle" or "bottom"
	FontSize      any
	Rotation      float64
}

// QuadrantPointType is a point with its position. X and Y are numbers, or
// nil for undefined: the scale of d3 returns undefined for a value that is
// not a number.
type QuadrantPointType struct {
	X, Y        any
	Fill        any
	Radius      any
	Text        QuadrantTextType
	StrokeColor any
	StrokeWidth any
}

// QuadrantQuadrantsType is one of the four quadrants.
type QuadrantQuadrantsType struct {
	Text   QuadrantTextType
	X, Y   float64
	Width  float64
	Height float64
	Fill   any
}

// QuadrantLineType is a border line.
type QuadrantLineType struct {
	StrokeWidth    any
	StrokeFill     any
	X1, Y1, X2, Y2 float64
}

// QuadrantBuildType is the result of QuadrantBuilder.Build. A nil Title is
// undefined.
type QuadrantBuildType struct {
	Points      []QuadrantPointType
	Quadrants   []QuadrantQuadrantsType
	AxisLabels  []QuadrantTextType
	Title       *QuadrantTextType
	BorderLines []QuadrantLineType
}

// QuadrantBuilderData holds the texts and the points of the diagram.
type QuadrantBuilderData struct {
	TitleText       string
	Quadrant1Text   string
	Quadrant2Text   string
	Quadrant3Text   string
	Quadrant4Text   string
	XAxisLeftText   string
	XAxisRightText  string
	YAxisBottomText string
	YAxisTopText    string
	Points          []QuadrantPointInputType
}

type calculateSpaceData struct {
	xAxisSpace    struct{ top, bottom float64 }
	yAxisSpace    struct{ left, right float64 }
	titleSpace    struct{ top float64 }
	quadrantSpace struct {
		quadrantLeft       float64
		quadrantTop        float64
		quadrantWidth      float64
		quadrantHalfWidth  float64
		quadrantHeight     float64
		quadrantHalfHeight float64
	}
}

// QuadrantBuilder computes the positions of the parts of the chart. The
// config and the theme config are dynamic objects, as upstream merges the
// config of the user into them without a check.
type QuadrantBuilder struct {
	config      *js.Obj[any]
	themeConfig *js.Obj[any]
	data        QuadrantBuilderData
	classes     *js.Map[string, StylesObject]
}

// NewQuadrantBuilder is new QuadrantBuilder().
func NewQuadrantBuilder() *QuadrantBuilder {
	return &QuadrantBuilder{
		config:      getDefaultConfig(),
		themeConfig: getDefaultThemeConfig(),
		data:        getDefaultData(),
		classes:     js.NewMap[string, StylesObject](),
	}
}

// getDefaultData is getDefaultData().
func getDefaultData() QuadrantBuilderData {
	return QuadrantBuilderData{Points: []QuadrantPointInputType{}}
}

// getDefaultConfig is getDefaultConfig().
func getDefaultConfig() *js.Obj[any] {
	section := config.DefaultSection("quadrantChart")
	// or is defaultConfig.quadrantChart?.key || fallback.
	or := func(key string, fallback any) any {
		if v := js.Path(section, key); js.Truthy(v) {
			return v
		}
		return fallback
	}
	c := js.NewObj[any]()
	c.Set("showXAxis", true)
	c.Set("showYAxis", true)
	c.Set("showTitle", true)
	c.Set("chartHeight", or("chartWidth", 500.0))
	c.Set("chartWidth", or("chartHeight", 500.0))
	c.Set("titlePadding", or("titlePadding", 10.0))
	c.Set("titleFontSize", or("titleFontSize", 20.0))
	c.Set("quadrantPadding", or("quadrantPadding", 5.0))
	c.Set("xAxisLabelPadding", or("xAxisLabelPadding", 5.0))
	c.Set("yAxisLabelPadding", or("yAxisLabelPadding", 5.0))
	c.Set("xAxisLabelFontSize", or("xAxisLabelFontSize", 16.0))
	c.Set("yAxisLabelFontSize", or("yAxisLabelFontSize", 16.0))
	c.Set("quadrantLabelFontSize", or("quadrantLabelFontSize", 16.0))
	c.Set("quadrantTextTopPadding", or("quadrantTextTopPadding", 5.0))
	c.Set("pointTextPadding", or("pointTextPadding", 5.0))
	c.Set("pointLabelFontSize", or("pointLabelFontSize", 12.0))
	c.Set("pointRadius", or("pointRadius", 5.0))
	c.Set("xAxisPosition", or("xAxisPosition", "top"))
	c.Set("yAxisPosition", or("yAxisPosition", "left"))
	c.Set("quadrantInternalBorderStrokeWidth", or("quadrantInternalBorderStrokeWidth", 1.0))
	c.Set("quadrantExternalBorderStrokeWidth", or("quadrantExternalBorderStrokeWidth", 2.0))
	return c
}

// getDefaultThemeConfig is getDefaultThemeConfig(): the variables of the
// default theme.
func getDefaultThemeConfig() *js.Obj[any] {
	// defaultThemeVariables is getThemeVariables() of themes/theme-default.js.
	defaultThemeVariables, err := theme.GetThemeVariables("default", nil)
	t := js.NewObj[any]()
	for _, key := range []string{
		"quadrant1Fill",
		"quadrant2Fill",
		"quadrant3Fill",
		"quadrant4Fill",
		"quadrant1TextFill",
		"quadrant2TextFill",
		"quadrant3TextFill",
		"quadrant4TextFill",
		"quadrantPointFill",
		"quadrantPointTextFill",
		"quadrantXAxisTextFill",
		"quadrantYAxisTextFill",
		"quadrantTitleFill",
		"quadrantInternalBorderStrokeFill",
		"quadrantExternalBorderStrokeFill",
	} {
		if err == nil {
			t.Set(key, defaultThemeVariables.Get(key))
		} else {
			t.Set(key, nil)
		}
	}
	return t
}

// AddPoints is addPoints(points): the new points come first.
func (b *QuadrantBuilder) AddPoints(points []QuadrantPointInputType) {
	b.data.Points = append(append([]QuadrantPointInputType{}, points...), b.data.Points...)
}

// AddClass is addClass(className, styles).
func (b *QuadrantBuilder) AddClass(className string, styles StylesObject) {
	b.classes.Set(className, styles)
}

// SetConfig is setConfig(config): { ...this.config, ...config }.
func (b *QuadrantBuilder) SetConfig(config *js.Obj[any]) {
	b.config = spread(b.config, config)
}

// SetThemeConfig is setThemeConfig(themeConfig).
func (b *QuadrantBuilder) SetThemeConfig(themeConfig *js.Obj[any]) {
	b.themeConfig = spread(b.themeConfig, themeConfig)
}

// spread is { ...a, ...b }.
func spread(a, b *js.Obj[any]) *js.Obj[any] {
	out := js.NewObj[any]()
	a.Each(func(key string, v any) { out.Set(key, v) })
	if b != nil {
		b.Each(func(key string, v any) { out.Set(key, v) })
	}
	return out
}

// num is the value of a config key in arithmetic.
func (b *QuadrantBuilder) num(key string) float64 {
	return js.ToNumber(b.config.Get(key))
}

// calculateSpace is calculateSpace(xAxisPosition, showXAxis, showYAxis,
// showTitle).
func (b *QuadrantBuilder) calculateSpace(xAxisPosition any, showXAxis, showYAxis, showTitle bool) calculateSpaceData {
	var space calculateSpaceData
	xAxisSpaceCalculation := float64(b.num("xAxisLabelPadding")*2) + b.num("xAxisLabelFontSize")
	if js.StrictEquals(xAxisPosition, "top") && showXAxis {
		space.xAxisSpace.top = xAxisSpaceCalculation
	}
	if js.StrictEquals(xAxisPosition, "bottom") && showXAxis {
		space.xAxisSpace.bottom = xAxisSpaceCalculation
	}

	yAxisSpaceCalculation := float64(b.num("yAxisLabelPadding")*2) + b.num("yAxisLabelFontSize")
	if js.StrictEquals(b.config.Get("yAxisPosition"), "left") && showYAxis {
		space.yAxisSpace.left = yAxisSpaceCalculation
	}
	if js.StrictEquals(b.config.Get("yAxisPosition"), "right") && showYAxis {
		space.yAxisSpace.right = yAxisSpaceCalculation
	}

	titleSpaceCalculation := b.num("titleFontSize") + float64(b.num("titlePadding")*2)
	if showTitle {
		space.titleSpace.top = titleSpaceCalculation
	}

	quadrantLeft := b.num("quadrantPadding") + space.yAxisSpace.left
	quadrantTop := b.num("quadrantPadding") + space.xAxisSpace.top + space.titleSpace.top
	quadrantWidth := b.num("chartWidth") - float64(b.num("quadrantPadding")*2) - space.yAxisSpace.left - space.yAxisSpace.right
	quadrantHeight := b.num("chartHeight") -
		float64(b.num("quadrantPadding")*2) -
		space.xAxisSpace.top -
		space.xAxisSpace.bottom -
		space.titleSpace.top

	quadrantHalfWidth := float64(quadrantWidth / 2)
	quadrantHalfHeight := float64(quadrantHeight / 2)
	space.quadrantSpace.quadrantLeft = quadrantLeft
	space.quadrantSpace.quadrantTop = quadrantTop
	space.quadrantSpace.quadrantWidth = quadrantWidth
	space.quadrantSpace.quadrantHalfWidth = quadrantHalfWidth
	space.quadrantSpace.quadrantHeight = quadrantHeight
	space.quadrantSpace.quadrantHalfHeight = quadrantHalfHeight
	return space
}

// getAxisLabels is getAxisLabels(xAxisPosition, showXAxis, showYAxis,
// spaceData).
func (b *QuadrantBuilder) getAxisLabels(xAxisPosition any, showXAxis, showYAxis bool, spaceData calculateSpaceData) []QuadrantTextType {
	q := spaceData.quadrantSpace
	titleSpace := spaceData.titleSpace
	quadrantHalfHeight, quadrantHeight, quadrantLeft := q.quadrantHalfHeight, q.quadrantHeight, q.quadrantLeft
	quadrantHalfWidth, quadrantTop, quadrantWidth := q.quadrantHalfWidth, q.quadrantTop, q.quadrantWidth

	drawXAxisLabelsInMiddle := b.data.XAxisRightText != ""
	drawYAxisLabelsInMiddle := b.data.YAxisTopText != ""

	axisLabels := []QuadrantTextType{}

	// middle is drawLabelsInMiddle ? half / 2 : 0.
	middle := func(inMiddle bool, half float64) float64 {
		if inMiddle {
			return float64(half / 2)
		}
		return 0
	}
	pos := func(inMiddle bool) string {
		if inMiddle {
			return "center"
		}
		return "left"
	}
	xAxisY := func() float64 {
		if js.StrictEquals(xAxisPosition, "top") {
			return b.num("xAxisLabelPadding") + titleSpace.top
		}
		return b.num("xAxisLabelPadding") +
			quadrantTop +
			quadrantHeight +
			b.num("quadrantPadding")
	}
	yAxisX := func() float64 {
		if js.StrictEquals(b.config.Get("yAxisPosition"), "left") {
			return b.num("yAxisLabelPadding")
		}
		return b.num("yAxisLabelPadding") +
			quadrantLeft +
			quadrantWidth +
			b.num("quadrantPadding")
	}

	if b.data.XAxisLeftText != "" && showXAxis {
		axisLabels = append(axisLabels, QuadrantTextType{
			Text:          b.data.XAxisLeftText,
			Fill:          b.themeConfig.Get("quadrantXAxisTextFill"),
			X:             quadrantLeft + middle(drawXAxisLabelsInMiddle, quadrantHalfWidth),
			Y:             xAxisY(),
			FontSize:      b.config.Get("xAxisLabelFontSize"),
			VerticalPos:   pos(drawXAxisLabelsInMiddle),
			HorizontalPos: "top",
			Rotation:      0,
		})
	}
	if b.data.XAxisRightText != "" && showXAxis {
		axisLabels = append(axisLabels, QuadrantTextType{
			Text:          b.data.XAxisRightText,
			Fill:          b.themeConfig.Get("quadrantXAxisTextFill"),
			X:             quadrantLeft + quadrantHalfWidth + middle(drawXAxisLabelsInMiddle, quadrantHalfWidth),
			Y:             xAxisY(),
			FontSize:      b.config.Get("xAxisLabelFontSize"),
			VerticalPos:   pos(drawXAxisLabelsInMiddle),
			HorizontalPos: "top",
			Rotation:      0,
		})
	}

	if b.data.YAxisBottomText != "" && showYAxis {
		axisLabels = append(axisLabels, QuadrantTextType{
			Text:          b.data.YAxisBottomText,
			Fill:          b.themeConfig.Get("quadrantYAxisTextFill"),
			X:             yAxisX(),
			Y:             quadrantTop + quadrantHeight - middle(drawYAxisLabelsInMiddle, quadrantHalfHeight),
			FontSize:      b.config.Get("yAxisLabelFontSize"),
			VerticalPos:   pos(drawYAxisLabelsInMiddle),
			HorizontalPos: "top",
			Rotation:      -90,
		})
	}
	if b.data.YAxisTopText != "" && showYAxis {
		axisLabels = append(axisLabels, QuadrantTextType{
			Text:          b.data.YAxisTopText,
			Fill:          b.themeConfig.Get("quadrantYAxisTextFill"),
			X:             yAxisX(),
			Y:             quadrantTop + quadrantHalfHeight - middle(drawYAxisLabelsInMiddle, quadrantHalfHeight),
			FontSize:      b.config.Get("yAxisLabelFontSize"),
			VerticalPos:   pos(drawYAxisLabelsInMiddle),
			HorizontalPos: "top",
			Rotation:      -90,
		})
	}
	return axisLabels
}

// getQuadrants is getQuadrants(spaceData).
func (b *QuadrantBuilder) getQuadrants(spaceData calculateSpaceData) []QuadrantQuadrantsType {
	q := spaceData.quadrantSpace
	quadrantHalfHeight, quadrantLeft, quadrantHalfWidth, quadrantTop := q.quadrantHalfHeight, q.quadrantLeft, q.quadrantHalfWidth, q.quadrantTop

	text := func(text string, fillKey string) QuadrantTextType {
		return QuadrantTextType{
			Text:          text,
			Fill:          b.themeConfig.Get(fillKey),
			X:             0.0,
			Y:             0.0,
			FontSize:      b.config.Get("quadrantLabelFontSize"),
			VerticalPos:   "center",
			HorizontalPos: "middle",
			Rotation:      0,
		}
	}
	quadrants := []QuadrantQuadrantsType{
		{
			Text:   text(b.data.Quadrant1Text, "quadrant1TextFill"),
			X:      quadrantLeft + quadrantHalfWidth,
			Y:      quadrantTop,
			Width:  quadrantHalfWidth,
			Height: quadrantHalfHeight,
			Fill:   b.themeConfig.Get("quadrant1Fill"),
		},
		{
			Text:   text(b.data.Quadrant2Text, "quadrant2TextFill"),
			X:      quadrantLeft,
			Y:      quadrantTop,
			Width:  quadrantHalfWidth,
			Height: quadrantHalfHeight,
			Fill:   b.themeConfig.Get("quadrant2Fill"),
		},
		{
			Text:   text(b.data.Quadrant3Text, "quadrant3TextFill"),
			X:      quadrantLeft,
			Y:      quadrantTop + quadrantHalfHeight,
			Width:  quadrantHalfWidth,
			Height: quadrantHalfHeight,
			Fill:   b.themeConfig.Get("quadrant3Fill"),
		},
		{
			Text:   text(b.data.Quadrant4Text, "quadrant4TextFill"),
			X:      quadrantLeft + quadrantHalfWidth,
			Y:      quadrantTop + quadrantHalfHeight,
			Width:  quadrantHalfWidth,
			Height: quadrantHalfHeight,
			Fill:   b.themeConfig.Get("quadrant4Fill"),
		},
	}
	for i := range quadrants {
		quadrant := &quadrants[i]
		quadrant.Text.X = quadrant.X + float64(quadrant.Width/2)
		// place the text in the center of the box
		if len(b.data.Points) == 0 {
			quadrant.Text.Y = quadrant.Y + float64(quadrant.Height/2)
			quadrant.Text.HorizontalPos = "middle"
			// place the text top of the quadrant square
		} else {
			quadrant.Text.Y = quadrant.Y + b.num("quadrantTextTopPadding")
			quadrant.Text.HorizontalPos = "top"
		}
	}

	return quadrants
}

// scaled is scale(value) of a linear scale of d3 for the text of a token:
// the scale converts the text to a number and returns undefined for NaN.
func scaled(s *scale.Linear[float64], value string) any {
	x := js.Number(value)
	if math.IsNaN(x) {
		return nil
	}
	return s.Scale(x)
}

// getQuadrantPoints is getQuadrantPoints(spaceData).
func (b *QuadrantBuilder) getQuadrantPoints(spaceData calculateSpaceData) []QuadrantPointType {
	q := spaceData.quadrantSpace
	quadrantHeight, quadrantLeft, quadrantTop, quadrantWidth := q.quadrantHeight, q.quadrantLeft, q.quadrantTop, q.quadrantWidth

	xAxis := scale.NewLinear().
		Domain(0, 1).
		Range(quadrantLeft, quadrantWidth+quadrantLeft)

	yAxis := scale.NewLinear().
		Domain(0, 1).
		Range(quadrantHeight+quadrantTop, quadrantTop)

	points := make([]QuadrantPointType, len(b.data.Points))
	for i, point := range b.data.Points {
		if classStyles, ok := b.classes.Lookup(point.ClassName); ok {
			point.StylesObject = point.StylesObject.merge(classStyles)
		}
		x := scaled(xAxis, point.X)
		y := scaled(yAxis, point.Y)
		props := QuadrantPointType{
			X:      x,
			Y:      y,
			Fill:   b.themeConfig.Get("quadrantPointFill"),
			Radius: b.config.Get("pointRadius"),
			Text: QuadrantTextType{
				Text:          point.Text,
				Fill:          b.themeConfig.Get("quadrantPointTextFill"),
				X:             x,
				Y:             js.ToNumber(y) + b.num("pointTextPadding"),
				VerticalPos:   "center",
				HorizontalPos: "top",
				FontSize:      b.config.Get("pointLabelFontSize"),
				Rotation:      0,
			},
			StrokeColor: b.themeConfig.Get("quadrantPointFill"),
			StrokeWidth: "0px",
		}
		if point.Color != nil {
			props.Fill = *point.Color
		}
		if point.Radius != nil {
			props.Radius = *point.Radius
		}
		if point.StrokeColor != nil {
			props.StrokeColor = *point.StrokeColor
		}
		if point.StrokeWidth != nil {
			props.StrokeWidth = *point.StrokeWidth
		}
		points[i] = props
	}
	return points
}

// getBorders is getBorders(spaceData).
func (b *QuadrantBuilder) getBorders(spaceData calculateSpaceData) []QuadrantLineType {
	halfExternalBorderWidth := float64(b.num("quadrantExternalBorderStrokeWidth") / 2)
	q := spaceData.quadrantSpace
	quadrantHalfHeight, quadrantHeight, quadrantLeft := q.quadrantHalfHeight, q.quadrantHeight, q.quadrantLeft
	quadrantHalfWidth, quadrantTop, quadrantWidth := q.quadrantHalfWidth, q.quadrantTop, q.quadrantWidth

	externalFill := b.themeConfig.Get("quadrantExternalBorderStrokeFill")
	externalWidth := b.config.Get("quadrantExternalBorderStrokeWidth")
	internalFill := b.themeConfig.Get("quadrantInternalBorderStrokeFill")
	internalWidth := b.config.Get("quadrantInternalBorderStrokeWidth")
	return []QuadrantLineType{
		// top border
		{
			StrokeFill:  externalFill,
			StrokeWidth: externalWidth,
			X1:          quadrantLeft - halfExternalBorderWidth,
			Y1:          quadrantTop,
			X2:          quadrantLeft + quadrantWidth + halfExternalBorderWidth,
			Y2:          quadrantTop,
		},
		// right border
		{
			StrokeFill:  externalFill,
			StrokeWidth: externalWidth,
			X1:          quadrantLeft + quadrantWidth,
			Y1:          quadrantTop + halfExternalBorderWidth,
			X2:          quadrantLeft + quadrantWidth,
			Y2:          quadrantTop + quadrantHeight - halfExternalBorderWidth,
		},
		// bottom border
		{
			StrokeFill:  externalFill,
			StrokeWidth: externalWidth,
			X1:          quadrantLeft - halfExternalBorderWidth,
			Y1:          quadrantTop + quadrantHeight,
			X2:          quadrantLeft + quadrantWidth + halfExternalBorderWidth,
			Y2:          quadrantTop + quadrantHeight,
		},
		// left border
		{
			StrokeFill:  externalFill,
			StrokeWidth: externalWidth,
			X1:          quadrantLeft,
			Y1:          quadrantTop + halfExternalBorderWidth,
			X2:          quadrantLeft,
			Y2:          quadrantTop + quadrantHeight - halfExternalBorderWidth,
		},
		// vertical inner border
		{
			StrokeFill:  internalFill,
			StrokeWidth: internalWidth,
			X1:          quadrantLeft + quadrantHalfWidth,
			Y1:          quadrantTop + halfExternalBorderWidth,
			X2:          quadrantLeft + quadrantHalfWidth,
			Y2:          quadrantTop + quadrantHeight - halfExternalBorderWidth,
		},
		// horizontal inner border
		{
			StrokeFill:  internalFill,
			StrokeWidth: internalWidth,
			X1:          quadrantLeft + halfExternalBorderWidth,
			Y1:          quadrantTop + quadrantHalfHeight,
			X2:          quadrantLeft + quadrantWidth - halfExternalBorderWidth,
			Y2:          quadrantTop + quadrantHalfHeight,
		},
	}
}

// getTitle is getTitle(showTitle).
func (b *QuadrantBuilder) getTitle(showTitle bool) *QuadrantTextType {
	if showTitle {
		return &QuadrantTextType{
			Text:          b.data.TitleText,
			Fill:          b.themeConfig.Get("quadrantTitleFill"),
			FontSize:      b.config.Get("titleFontSize"),
			HorizontalPos: "top",
			VerticalPos:   "center",
			Rotation:      0,
			Y:             b.config.Get("titlePadding"),
			X:             b.num("chartWidth") / 2,
		}
	}
	return nil
}

// Build is build().
func (b *QuadrantBuilder) Build() QuadrantBuildType {
	showXAxis := js.Truthy(b.config.Get("showXAxis")) && (b.data.XAxisLeftText != "" || b.data.XAxisRightText != "")
	showYAxis := js.Truthy(b.config.Get("showYAxis")) && (b.data.YAxisTopText != "" || b.data.YAxisBottomText != "")
	showTitle := js.Truthy(b.config.Get("showTitle")) && b.data.TitleText != ""

	xAxisPosition := b.config.Get("xAxisPosition")
	if len(b.data.Points) > 0 {
		xAxisPosition = "bottom"
	}

	calculatedSpace := b.calculateSpace(xAxisPosition, showXAxis, showYAxis, showTitle)

	return QuadrantBuildType{
		Points:      b.getQuadrantPoints(calculatedSpace),
		Quadrants:   b.getQuadrants(calculatedSpace),
		AxisLabels:  b.getAxisLabels(xAxisPosition, showXAxis, showYAxis, calculatedSpace),
		BorderLines: b.getBorders(calculatedSpace),
		Title:       b.getTitle(showTitle),
	}
}
