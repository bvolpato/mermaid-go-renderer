// Ported from skrifa 0.46.2 (src/outline/autohint/metrics/mod.rs, and the
// helpers of Target in src/outline/hint.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/aflatin.h, src/autofit/afcjk.h, src/autofit/aftypes.h), which
// has the FreeType License (LICENSE-FreeType.txt).

package font

import (
	"math"
	"sort"
)

// ahMaxWidths is MAX_WIDTHS: the largest number of widths, for Latin and
// for CJK.
const ahMaxWidths = 16

// ahDimension is Dimension: the dimension of an axis.
type ahDimension uint8

const (
	// ahHorizontal has the metrics and the geometry in the horizontal
	// direction.
	ahHorizontal ahDimension = 0
	// ahVertical has the metrics and the geometry in the vertical
	// direction.
	ahVertical ahDimension = 1
)

// ahSmoothMode is SmoothMode: the mode of a smooth hinting target.
type ahSmoothMode uint8

const (
	ahSmoothNormal ahSmoothMode = iota
	ahSmoothLight
	ahSmoothLcd
	ahSmoothVerticalLcd
)

// ahTarget is Target: the settings of the target of hinting. mono selects
// the strong style for rendering without antialiasing. The other fields are
// the fields of the smooth target.
type ahTarget struct {
	mono                  bool
	mode                  ahSmoothMode
	symmetricRendering    bool
	preserveLinearMetrics bool
}

func (t ahTarget) isLight() bool { return !t.mono && t.mode == ahSmoothLight }

func (t ahTarget) isLcd() bool { return !t.mono && t.mode == ahSmoothLcd }

func (t ahTarget) isVerticalLcd() bool { return !t.mono && t.mode == ahSmoothVerticalLcd }

func (t ahTarget) preserveLinear() bool { return !t.mono && t.preserveLinearMetrics }

// ahUnscaledAxisMetrics is UnscaledAxisMetrics: the metrics of one axis in
// font units.
type ahUnscaledAxisMetrics struct {
	dim          ahDimension
	widths       []int32
	widthMetrics ahWidthMetrics
	blues        []ahUnscaledBlue
}

// maxWidth returns the largest width, when there is one.
func (m *ahUnscaledAxisMetrics) maxWidth() (int32, bool) {
	if len(m.widths) == 0 {
		return 0, false
	}
	return m.widths[len(m.widths)-1], true
}

// ahScaledAxisMetrics is ScaledAxisMetrics: the scaled metrics of one axis.
type ahScaledAxisMetrics struct {
	dim ahDimension
	// scale converts font units to 26.6 px in the direction of the axis.
	scale int32
	// delta is a shift in 1/64 px in the direction of the axis.
	delta        int32
	widths       []ahScaledWidth
	widthMetrics ahWidthMetrics
	blues        []ahScaledBlue
}

// ahUnscaledStyleMetrics is UnscaledStyleMetrics: the metrics of one style
// in font units.
type ahUnscaledStyleMetrics struct {
	// classIx is the index of the style class.
	classIx uint16
	// digitsHaveSameWidth says that the digits have one width.
	digitsHaveSameWidth bool
	axes                [2]ahUnscaledAxisMetrics
}

func (m *ahUnscaledStyleMetrics) styleClass() *ahStyleClass {
	return &ahStyleClasses[m.classIx]
}

// ahScaledStyleMetrics is ScaledStyleMetrics: the scaled metrics of one
// style.
type ahScaledStyleMetrics struct {
	scale ahScale
	axes  [2]ahScaledAxisMetrics
}

// ahWidthMetrics is WidthMetrics: the metrics of the stems along one axis.
type ahWidthMetrics struct {
	// edgeDistanceThreshold is used to make edges.
	edgeDistanceThreshold int32
	// standardWidth is the default thickness of a stem.
	standardWidth int32
	// isExtraLight says that the standard width is very small.
	isExtraLight bool
}

// ahScaledWidth is ScaledWidth: a scaled stem width.
type ahScaledWidth struct {
	// scaled is the width after the scale.
	scaled int32
	// fitted is the width on the pixel grid.
	fitted int32
}

// ahScaleFlags is ScaleFlags: the flags that select how scaling and hinting
// work.
type ahScaleFlags uint32

const (
	// ahScaleHorizontalSnap snaps the widths of stems.
	ahScaleHorizontalSnap ahScaleFlags = 1 << 0
	// ahScaleVerticalSnap snaps the heights of stems.
	ahScaleVerticalSnap ahScaleFlags = 1 << 1
	// ahScaleStemAdjust adjusts the widths and heights of stems.
	ahScaleStemAdjust ahScaleFlags = 1 << 2
	// ahScaleMono is for rendering without antialiasing.
	ahScaleMono ahScaleFlags = 1 << 3
	// ahScaleNoHorizontal disables horizontal hinting.
	ahScaleNoHorizontal ahScaleFlags = 1 << 4
	// ahScaleNoVertical disables vertical hinting.
	ahScaleNoVertical ahScaleFlags = 1 << 5
	// ahScaleNoAdvance disables the hinting of advances.
	ahScaleNoAdvance ahScaleFlags = 1 << 6
)

func (f ahScaleFlags) contains(other ahScaleFlags) bool { return f&other == other }

// ahScale is Scale: the scaling parameters. The computation of the metrics
// can change them.
type ahScale struct {
	flags ahScaleFlags
	// xScale and yScale convert font units to 26.6 px.
	xScale, yScale int32
	// xDelta and yDelta are in 1/64 px.
	xDelta, yDelta int32
	// size is the font size in px per em.
	size       float32
	unitsPerEm int32
}

// ahF32ToI32 is the cast of a float32 to a 32 bit integer in Rust: the
// fraction is cut, a value outside of the range gives the nearest limit, and
// a value that is not a number gives 0.
func ahF32ToI32(v float32) int32 {
	switch {
	case v != v:
		return 0
	case v >= 2147483648:
		return math.MaxInt32
	case v <= -2147483648:
		return math.MinInt32
	}
	return int32(v)
}

// newAHScale is Scale::new: the first scaling parameters for a font size and
// a hinting target.
func newAHScale(size float32, unitsPerEm int32, isItalic bool, target ahTarget, group ahScriptGroup) ahScale {
	scale := ahFixedDiv(ahF32ToI32(size*64.0), unitsPerEm)
	var flags ahScaleFlags
	isMono := target.mono
	isLight := target.isLight() || target.preserveLinear()
	// Snap vertical stems for rendering without antialiasing and for
	// horizontal LCD rendering.
	if isMono || target.isLcd() {
		flags |= ahScaleHorizontalSnap
	}
	// Snap horizontal stems for rendering without antialiasing and for
	// vertical LCD rendering.
	if isMono || target.isVerticalLcd() {
		flags |= ahScaleVerticalSnap
	}
	// Adjust stems to full pixels unless in the LCD or light modes.
	if !(target.isLcd() || isLight) {
		flags |= ahScaleStemAdjust
	}
	if isMono {
		flags |= ahScaleMono
	}
	if group == ahGroupDefault {
		// No horizontal hinting at all for LCD, for light hinting and for
		// italic fonts.
		if target.isLcd() || isLight || isItalic {
			flags |= ahScaleNoHorizontal
		}
	} else {
		// CJK does not hint advances.
		flags |= ahScaleNoAdvance
	}
	if group != ahGroupDefault {
		flags |= ahScaleNoAdvance
	}
	return ahScale{
		xScale:     scale,
		yScale:     scale,
		size:       size,
		unitsPerEm: unitsPerEm,
		flags:      flags,
	}
}

// ahSortAndQuantizeWidths is sort_and_quantize_widths: it sorts the widths
// and replaces each cluster of near widths with one value. The arithmetic
// is the arithmetic of FreeType, as it is.
func ahSortAndQuantizeWidths(widths []int32, threshold int32) []int32 {
	if len(widths) <= 1 {
		return widths
	}
	sort.Slice(widths, func(i, j int) bool { return widths[i] < widths[j] })
	table := widths
	curIx := 0
	curVal := table[curIx]
	lastIx := len(table) - 1
	// Use the mean value of each cluster that is not larger than the
	// threshold.
	for ix := 1; ix < len(table); ix++ {
		if (table[ix]-curVal) > threshold || ix == lastIx {
			var sum int32
			// The loop needs one more item at the end of the array.
			if (table[ix]-curVal <= threshold) && ix == lastIx {
				ix++
			}
			for k := curIx; k < ix; k++ {
				sum += table[k]
				table[k] = 0
			}
			table[curIx] = sum / int32(ix)
			if ix < lastIx {
				curIx = ix + 1
				curVal = table[curIx]
			}
		}
	}
	curIx = 1
	// Remove the zero values.
	for ix := 1; ix < len(table); ix++ {
		if table[ix] != 0 {
			table[curIx] = table[ix]
			curIx++
		}
	}
	return widths[:curIx]
}
