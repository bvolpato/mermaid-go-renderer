// Ported from mermaid 12.2.0,
// diagrams/xychart/chartBuilder/components/axis/baseAxis.ts (MIT).

package xychart

import (
	"errors"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

const (
	barWidthToTickWidthRatio          = 0.7
	maxOuterPaddingPercentForWrtLabel = 0.2
)

// axisImpl has the abstract methods of BaseAxis.
type axisImpl interface {
	getScaleValue(value any) any
	recalculateScale()
	getTickValues() []any
}

// baseAxis is BaseAxis. axisConfig is XYChartAxisConfig: an object in the
// normal case, and any other value that is not null when the user gives one.
type baseAxis struct {
	impl axisImpl

	boundingRect                 boundingRect
	axisPosition                 axisPosition
	rng                          [2]float64
	showTitle                    bool
	showLabel                    bool
	showTick                     bool
	showAxisLine                 bool
	outerPadding                 float64
	titleTextHeight              float64
	labelTextHeight              float64
	normalizedLabelRotationInRad float64

	axisConfig              any
	title                   string
	textDimensionCalculator textDimensionCalculator
	axisThemeConfig         xyChartAxisThemeConfig

	// labelDimension is the result of getLabelDimension.
	labelDimension    dimension
	hasLabelDimension bool
}

func newBaseAxis(
	impl axisImpl,
	axisConfig any,
	title string,
	textDimensionCalculator textDimensionCalculator,
	axisThemeConfig xyChartAxisThemeConfig,
) (*baseAxis, error) {
	a := &baseAxis{
		impl:                    impl,
		axisConfig:              axisConfig,
		title:                   title,
		textDimensionCalculator: textDimensionCalculator,
		axisThemeConfig:         axisThemeConfig,
	}
	a.rng = [2]float64{0, 10}
	a.boundingRect = boundingRect{x: 0, y: 0, width: 0, height: 0}
	a.axisPosition = "left"
	// The first read of a property of the axis config. It throws when the
	// config is null.
	labelRotation, err := prop(axisConfig, "labelRotation")
	if err != nil {
		return nil, err
	}
	if js.Compare(labelRotation, ">=", -90.0) && js.Compare(labelRotation, "<=", 90.0) {
		a.normalizedLabelRotationInRad = (num(labelRotation) * math.Pi) / 180
	} else {
		a.normalizedLabelRotationInRad = 0
	}
	return a, nil
}

// cfg reads a property of the axis config.
func (a *baseAxis) cfg(key string) any {
	return get(a.axisConfig, key)
}

func (a *baseAxis) setRange(rng [2]float64) {
	a.rng = rng
	if a.axisPosition == "left" || a.axisPosition == "right" {
		a.boundingRect.height = rng[1] - rng[0]
	} else {
		a.boundingRect.width = rng[1] - rng[0]
	}
	a.impl.recalculateScale()
}

func (a *baseAxis) getRange() [2]float64 {
	return [2]float64{a.rng[0] + a.outerPadding, a.rng[1] - a.outerPadding}
}

func (a *baseAxis) setAxisPosition(axisPosition axisPosition) {
	a.axisPosition = axisPosition
	a.setRange(a.rng)
}

func (a *baseAxis) getTickDistance() float64 {
	rng := a.getRange()
	return math.Abs(rng[0]-rng[1]) / float64(len(a.impl.getTickValues()))
}

func (a *baseAxis) getAxisOuterPadding() float64 {
	return a.outerPadding
}

func (a *baseAxis) getLabelDimension() dimension {
	// Upstream measures the labels again on each call, and the bottom axis
	// calls this function two times for each label when the labels have a
	// rotation. The ticks, the font size and the document do not change
	// between the calls, so the port measures one time.
	if a.hasLabelDimension {
		return a.labelDimension
	}
	ticks := a.impl.getTickValues()
	texts := make([]string, len(ticks))
	for i, tick := range ticks {
		texts[i] = js.ToString(tick)
	}
	a.labelDimension = a.textDimensionCalculator.getMaxDimension(texts, a.cfg("labelFontSize"))
	a.hasLabelDimension = true
	return a.labelDimension
}

func (a *baseAxis) recalculateOuterPaddingToDrawBar() {
	if barWidthToTickWidthRatio*a.getTickDistance() > a.outerPadding*2 {
		a.outerPadding = math.Floor((barWidthToTickWidthRatio * a.getTickDistance()) / 2)
	}
	a.impl.recalculateScale()
}

func (a *baseAxis) calculateSpaceIfDrawnHorizontally(availableSpace dimension) {
	availableHeight := availableSpace.height
	if js.Truthy(a.cfg("showAxisLine")) && availableHeight > num(a.cfg("axisLineWidth")) {
		availableHeight -= num(a.cfg("axisLineWidth"))
		a.showAxisLine = true
	}
	if js.Truthy(a.cfg("showLabel")) {
		spaceRequired := a.getLabelDimension()
		maxPadding := maxOuterPaddingPercentForWrtLabel * availableSpace.width
		a.outerPadding = js.Min(spaceRequired.width/2, maxPadding)

		heightRequired := spaceRequired.height
		if a.axisPosition == "bottom" && a.normalizedLabelRotationInRad != 0 {
			heightRequired = js.Max(
				heightRequired,
				math.Abs(js.SinCR(a.normalizedLabelRotationInRad)*spaceRequired.width)+
					math.Abs(js.CosCR(a.normalizedLabelRotationInRad)*spaceRequired.height),
			)
		}
		heightRequired += float64(num(a.cfg("labelPadding")) * 2)

		a.labelTextHeight = spaceRequired.height
		if heightRequired <= availableHeight {
			availableHeight -= heightRequired
			a.showLabel = true
		}
	}
	if js.Truthy(a.cfg("showTick")) && availableHeight >= num(a.cfg("tickLength")) {
		a.showTick = true
		availableHeight -= num(a.cfg("tickLength"))
	}
	if js.Truthy(a.cfg("showTitle")) && a.title != "" {
		spaceRequired := a.textDimensionCalculator.getMaxDimension(
			[]string{a.title},
			a.cfg("titleFontSize"),
		)
		heightRequired := spaceRequired.height + float64(num(a.cfg("titlePadding"))*2)
		a.titleTextHeight = spaceRequired.height
		if heightRequired <= availableHeight {
			availableHeight -= heightRequired
			a.showTitle = true
		}
	}
	a.boundingRect.width = availableSpace.width
	a.boundingRect.height = availableSpace.height - availableHeight
}

func (a *baseAxis) calculateSpaceIfDrawnVertical(availableSpace dimension) {
	availableWidth := availableSpace.width
	if js.Truthy(a.cfg("showAxisLine")) && availableWidth > num(a.cfg("axisLineWidth")) {
		availableWidth -= num(a.cfg("axisLineWidth"))
		a.showAxisLine = true
	}
	if js.Truthy(a.cfg("showLabel")) {
		spaceRequired := a.getLabelDimension()
		maxPadding := maxOuterPaddingPercentForWrtLabel * availableSpace.height
		a.outerPadding = js.Min(spaceRequired.height/2, maxPadding)
		widthRequired := spaceRequired.width + float64(num(a.cfg("labelPadding"))*2)
		if widthRequired <= availableWidth {
			availableWidth -= widthRequired
			a.showLabel = true
		}
	}
	if js.Truthy(a.cfg("showTick")) && availableWidth >= num(a.cfg("tickLength")) {
		a.showTick = true
		availableWidth -= num(a.cfg("tickLength"))
	}
	if js.Truthy(a.cfg("showTitle")) && a.title != "" {
		spaceRequired := a.textDimensionCalculator.getMaxDimension(
			[]string{a.title},
			a.cfg("titleFontSize"),
		)
		widthRequired := spaceRequired.height + float64(num(a.cfg("titlePadding"))*2)
		a.titleTextHeight = spaceRequired.height
		if widthRequired <= availableWidth {
			availableWidth -= widthRequired
			a.showTitle = true
		}
	}
	a.boundingRect.width = availableSpace.width - availableWidth
	a.boundingRect.height = availableSpace.height
}

func (a *baseAxis) calculateSpace(availableSpace dimension) dimension {
	if a.axisPosition == "left" || a.axisPosition == "right" {
		a.calculateSpaceIfDrawnVertical(availableSpace)
	} else {
		a.calculateSpaceIfDrawnHorizontally(availableSpace)
	}
	a.impl.recalculateScale()
	return dimension{
		width:  a.boundingRect.width,
		height: a.boundingRect.height,
	}
}

func (a *baseAxis) setBoundingBoxXY(p point) {
	a.boundingRect.x = p.x
	a.boundingRect.y = p.y
}

// calculateOffsetByRotation is calculateOffsetByRotation(reference). The
// reference is "width" or "height".
func (a *baseAxis) calculateOffsetByRotation(reference string) float64 {
	rotation := a.normalizedLabelRotationInRad
	if rotation == 0 {
		return 0
	}

	labelDimension := a.getLabelDimension()
	value := labelDimension.width
	if reference == "height" {
		value = labelDimension.height
	}
	return (js.SinCR(rotation) * value) / 2
}

// or0 is the expression (flag ? value : 0).
func or0(flag bool, value any) any {
	if flag {
		return value
	}
	return 0.0
}

func (a *baseAxis) getDrawableElementsForLeftAxis() []drawableElem {
	drawableElement := []drawableElem{}
	if a.showAxisLine {
		x := a.boundingRect.x + a.boundingRect.width - float64(num(a.cfg("axisLineWidth"))/2)
		drawableElement = append(drawableElement, drawableElemOfPaths(
			[]string{"left-axis", "axisl-line"},
			[]pathElem{
				{
					path: "M " + js.Str(x) + "," + js.Str(a.boundingRect.y) + " L " + js.Str(x) + "," +
						js.Str(a.boundingRect.y+a.boundingRect.height) + " ",
					strokeFill:  a.axisThemeConfig.axisLineColor,
					strokeWidth: a.cfg("axisLineWidth"),
				},
			},
		))
	}
	if a.showLabel {
		ticks := a.impl.getTickValues()
		texts := make([]textElem, len(ticks))
		for i, tick := range ticks {
			texts[i] = textElem{
				text: js.ToString(tick),
				x: a.boundingRect.x +
					a.boundingRect.width -
					num(or0(a.showLabel, a.cfg("labelPadding"))) -
					num(or0(a.showTick, a.cfg("tickLength"))) -
					num(or0(a.showAxisLine, a.cfg("axisLineWidth"))),
				y:             a.impl.getScaleValue(tick),
				fill:          a.axisThemeConfig.labelColor,
				fontSize:      a.cfg("labelFontSize"),
				rotation:      0,
				verticalPos:   "middle",
				horizontalPos: "right",
			}
		}
		drawableElement = append(drawableElement, drawableElemOfTexts([]string{"left-axis", "label"}, texts))
	}
	if a.showTick {
		x := a.boundingRect.x +
			a.boundingRect.width -
			num(or0(a.showAxisLine, a.cfg("axisLineWidth")))
		ticks := a.impl.getTickValues()
		paths := make([]pathElem, len(ticks))
		for i, tick := range ticks {
			paths[i] = pathElem{
				path: "M " + js.Str(x) + "," + js.ToString(a.impl.getScaleValue(tick)) + " L " +
					js.Str(x-num(a.cfg("tickLength"))) + "," + js.ToString(a.impl.getScaleValue(tick)),
				strokeFill:  a.axisThemeConfig.tickColor,
				strokeWidth: a.cfg("tickWidth"),
			}
		}
		drawableElement = append(drawableElement, drawableElemOfPaths([]string{"left-axis", "ticks"}, paths))
	}
	if a.showTitle {
		drawableElement = append(drawableElement, drawableElemOfTexts(
			[]string{"left-axis", "title"},
			[]textElem{
				{
					text:          a.title,
					x:             js.Add(a.boundingRect.x, a.cfg("titlePadding")),
					y:             a.boundingRect.y + float64(a.boundingRect.height/2),
					fill:          a.axisThemeConfig.titleColor,
					fontSize:      a.cfg("titleFontSize"),
					rotation:      270,
					verticalPos:   "top",
					horizontalPos: "center",
				},
			},
		))
	}
	return drawableElement
}

func (a *baseAxis) getDrawableElementsForBottomAxis() []drawableElem {
	drawableElement := []drawableElem{}
	if a.showAxisLine {
		y := a.boundingRect.y + float64(num(a.cfg("axisLineWidth"))/2)
		drawableElement = append(drawableElement, drawableElemOfPaths(
			[]string{"bottom-axis", "axis-line"},
			[]pathElem{
				{
					path: "M " + js.Str(a.boundingRect.x) + "," + js.Str(y) + " L " +
						js.Str(a.boundingRect.x+a.boundingRect.width) + "," + js.Str(y),
					strokeFill:  a.axisThemeConfig.axisLineColor,
					strokeWidth: a.cfg("axisLineWidth"),
				},
			},
		))
	}
	if a.showLabel {
		ticks := a.impl.getTickValues()
		texts := make([]textElem, len(ticks))
		for i, tick := range ticks {
			texts[i] = textElem{
				text: js.ToString(tick),
				x:    js.Add(a.impl.getScaleValue(tick), a.calculateOffsetByRotation("height")),
				y: js.Add(
					js.Add(
						js.Add(
							js.Add(a.boundingRect.y, a.cfg("labelPadding")),
							or0(a.showTick, a.cfg("tickLength")),
						),
						or0(a.showAxisLine, a.cfg("axisLineWidth")),
					),
					math.Abs(a.calculateOffsetByRotation("width")),
				),
				fill:          a.axisThemeConfig.labelColor,
				fontSize:      a.cfg("labelFontSize"),
				rotation:      (a.normalizedLabelRotationInRad * 180) / math.Pi,
				verticalPos:   "top",
				horizontalPos: "center",
			}
		}
		drawableElement = append(drawableElement, drawableElemOfTexts([]string{"bottom-axis", "label"}, texts))
	}
	if a.showTick {
		y := js.Add(a.boundingRect.y, or0(a.showAxisLine, a.cfg("axisLineWidth")))
		ticks := a.impl.getTickValues()
		paths := make([]pathElem, len(ticks))
		for i, tick := range ticks {
			paths[i] = pathElem{
				path: "M " + js.ToString(a.impl.getScaleValue(tick)) + "," + js.ToString(y) + " L " +
					js.ToString(a.impl.getScaleValue(tick)) + "," + js.ToString(js.Add(y, a.cfg("tickLength"))),
				strokeFill:  a.axisThemeConfig.tickColor,
				strokeWidth: a.cfg("tickWidth"),
			}
		}
		drawableElement = append(drawableElement, drawableElemOfPaths([]string{"bottom-axis", "ticks"}, paths))
	}
	if a.showTitle {
		drawableElement = append(drawableElement, drawableElemOfTexts(
			[]string{"bottom-axis", "title"},
			[]textElem{
				{
					text: a.title,
					x:    a.rng[0] + float64((a.rng[1]-a.rng[0])/2),
					y: a.boundingRect.y +
						a.boundingRect.height -
						num(a.cfg("titlePadding")) -
						a.titleTextHeight,
					fill:          a.axisThemeConfig.titleColor,
					fontSize:      a.cfg("titleFontSize"),
					rotation:      0,
					verticalPos:   "top",
					horizontalPos: "center",
				},
			},
		))
	}
	return drawableElement
}

func (a *baseAxis) getDrawableElementsForTopAxis() []drawableElem {
	drawableElement := []drawableElem{}
	if a.showAxisLine {
		y := a.boundingRect.y + a.boundingRect.height - float64(num(a.cfg("axisLineWidth"))/2)
		drawableElement = append(drawableElement, drawableElemOfPaths(
			[]string{"top-axis", "axis-line"},
			[]pathElem{
				{
					path: "M " + js.Str(a.boundingRect.x) + "," + js.Str(y) + " L " +
						js.Str(a.boundingRect.x+a.boundingRect.width) + "," + js.Str(y),
					strokeFill:  a.axisThemeConfig.axisLineColor,
					strokeWidth: a.cfg("axisLineWidth"),
				},
			},
		))
	}
	if a.showLabel {
		ticks := a.impl.getTickValues()
		texts := make([]textElem, len(ticks))
		for i, tick := range ticks {
			titleSpace := 0.0
			if a.showTitle {
				titleSpace = a.titleTextHeight + float64(num(a.cfg("titlePadding"))*2)
			}
			texts[i] = textElem{
				text: js.ToString(tick),
				x:    a.impl.getScaleValue(tick),
				y: js.Add(
					a.boundingRect.y+titleSpace,
					a.cfg("labelPadding"),
				),
				fill:          a.axisThemeConfig.labelColor,
				fontSize:      a.cfg("labelFontSize"),
				rotation:      0,
				verticalPos:   "top",
				horizontalPos: "center",
			}
		}
		drawableElement = append(drawableElement, drawableElemOfTexts([]string{"top-axis", "label"}, texts))
	}
	if a.showTick {
		y := a.boundingRect.y
		ticks := a.impl.getTickValues()
		paths := make([]pathElem, len(ticks))
		for i, tick := range ticks {
			paths[i] = pathElem{
				path: "M " + js.ToString(a.impl.getScaleValue(tick)) + "," +
					js.Str(y+a.boundingRect.height-num(or0(a.showAxisLine, a.cfg("axisLineWidth")))) +
					" L " + js.ToString(a.impl.getScaleValue(tick)) + "," +
					js.Str(y+
						a.boundingRect.height-
						num(a.cfg("tickLength"))-
						num(or0(a.showAxisLine, a.cfg("axisLineWidth")))),
				strokeFill:  a.axisThemeConfig.tickColor,
				strokeWidth: a.cfg("tickWidth"),
			}
		}
		drawableElement = append(drawableElement, drawableElemOfPaths([]string{"top-axis", "ticks"}, paths))
	}
	if a.showTitle {
		drawableElement = append(drawableElement, drawableElemOfTexts(
			[]string{"top-axis", "title"},
			[]textElem{
				{
					text:          a.title,
					x:             a.boundingRect.x + float64(a.boundingRect.width/2),
					y:             js.Add(a.boundingRect.y, a.cfg("titlePadding")),
					fill:          a.axisThemeConfig.titleColor,
					fontSize:      a.cfg("titleFontSize"),
					rotation:      0,
					verticalPos:   "top",
					horizontalPos: "center",
				},
			},
		))
	}
	return drawableElement
}

func (a *baseAxis) getDrawableElements() ([]drawableElem, error) {
	if a.axisPosition == "left" {
		return a.getDrawableElementsForLeftAxis(), nil
	}
	if a.axisPosition == "right" {
		return nil, errors.New("Drawing of right axis is not implemented")
	}
	if a.axisPosition == "bottom" {
		return a.getDrawableElementsForBottomAxis(), nil
	}
	if a.axisPosition == "top" {
		return a.getDrawableElementsForTopAxis(), nil
	}
	return []drawableElem{}, nil
}
