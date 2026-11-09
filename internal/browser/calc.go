// Ported from Chromium 156.0.8078.4 (Blink), in part: the length of a CSS
// math function in a property. The functions of this file follow the
// functions of Blink that their comments name:
//
//   - CSSMathFunctionValue::ComputeLengthPx:
//     core/css/css_math_function_value.cc (BSD-3-Clause).
//   - ComputeFontSize and StyleBuilderConverterBase::ConvertFontSize:
//     core/css/resolver/style_builder_converter.cc (BSD-3-Clause, with the
//     copyright of Google in the file).
//
// The paths are in third_party/blink/renderer. See NOTICE in this directory.
// internal/css/calc.go has the expression itself.

package browser

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
)

// This file gives the engine the math functions of CSS in a length: calc(),
// min(), max() and clamp(). The sizing classes of Font Awesome have them:
// font-size: calc(20 / 16 * 1em).
//
// CSSMathFunctionValue::ComputeLengthPx: the value of a math function that
// is a length is the sum of its lengths in px, as a float32
// (clampCalcPx). testdata/pagesheets-cases.json.gz has the values of
// Chromium.

// maxCalcPx is the largest length of a math function: the largest
// LayoutUnit of Blink as a float32. A division by zero gives it.
const maxCalcPx = 33554432

// clampCalcPx returns the length of a math function as a float32. A value
// that is not a number is 0, and an infinite value is the largest length.
func clampCalcPx(px float64) float64 {
	switch {
	case math.IsNaN(px):
		return 0
	case px > maxCalcPx:
		return maxCalcPx
	case px < -maxCalcPx:
		return -maxCalcPx
	}
	return float64(float32(px))
}

// evalCalc evaluates a math function for an element with the text style
// ts: em is the font size of ts, and ex is the x-height of its font. The
// font is resolved only for a function that has the unit ex.
func (e *Engine) evalCalc(v string, ts *textStyle) (css.CalcValue, bool) {
	return css.EvalCalc(v, func(number float64, unit string) (float64, bool) {
		if unit == "ex" {
			return float64(number * e.exSize(ts)), true
		}
		return unitLength(number, unit, ts.size)
	})
}

// calcFontSize is ComputeFontSize for a math function: em, ex and a
// percentage refer to the font of the parent. A math function makes the
// font size absolute (ConvertFontSize).
func (e *Engine) calcFontSize(v string, parent *textStyle) (float64, bool) {
	if !css.IsMathFunction(v) {
		return 0, false
	}
	c, ok := e.evalCalc(v, parent)
	if !ok || c.IsNumber {
		return 0, false
	}
	px := c.Px
	if c.HasPercent {
		// CalculationValue::Evaluate with the font size of the parent.
		px += float64(c.Percent * parent.size / 100)
	}
	// A font size is not negative.
	return max(clampCalcPx(px), 0), true
}
