// Based on the OpenType MATH specification and Chromium 156.0.8078.4,
// third_party/blink/renderer/platform/fonts/opentype/open_type_math_support.cc
// and core/layout/mathml/{math_layout_utils,math_scripts_layout_algorithm,
// math_fraction_layout_algorithm,math_under_over_layout_algorithm}.cc.
// The Chromium files have the BSD-3-Clause license (LICENSE-Chromium.txt).
// The table reader is original code, not a port of HarfBuzz.

package font

import "math"

// mathConstantNames has the field order of the OpenType MathConstants table.
var mathConstantNames = [...]string{
	"ScriptPercentScaleDown", "ScriptScriptPercentScaleDown",
	"DelimitedSubFormulaMinHeight", "DisplayOperatorMinHeight",
	"MathLeading", "AxisHeight", "AccentBaseHeight", "FlattenedAccentBaseHeight",
	"SubscriptShiftDown", "SubscriptTopMax", "SubscriptBaselineDropMin",
	"SuperscriptShiftUp", "SuperscriptShiftUpCramped", "SuperscriptBottomMin",
	"SuperscriptBaselineDropMax", "SubSuperscriptGapMin",
	"SuperscriptBottomMaxWithSubscript", "SpaceAfterScript",
	"UpperLimitGapMin", "UpperLimitBaselineRiseMin", "LowerLimitGapMin",
	"LowerLimitBaselineDropMin", "StackTopShiftUp", "StackTopDisplayStyleShiftUp",
	"StackBottomShiftDown", "StackBottomDisplayStyleShiftDown", "StackGapMin",
	"StackDisplayStyleGapMin", "StretchStackTopShiftUp", "StretchStackBottomShiftDown",
	"StretchStackGapAboveMin", "StretchStackGapBelowMin", "FractionNumeratorShiftUp",
	"FractionNumeratorDisplayStyleShiftUp", "FractionDenominatorShiftDown",
	"FractionDenominatorDisplayStyleShiftDown", "FractionNumeratorGapMin",
	"FractionNumDisplayStyleGapMin", "FractionRuleThickness", "FractionDenominatorGapMin",
	"FractionDenomDisplayStyleGapMin", "SkewedFractionHorizontalGap",
	"SkewedFractionVerticalGap", "OverbarVerticalGap", "OverbarRuleThickness",
	"OverbarExtraAscender", "UnderbarVerticalGap", "UnderbarRuleThickness",
	"UnderbarExtraDescender", "RadicalVerticalGap", "RadicalDisplayStyleVerticalGap",
	"RadicalRuleThickness", "RadicalExtraAscender", "RadicalKernBeforeDegree",
	"RadicalKernAfterDegree", "RadicalDegreeBottomRaisePercent",
}

func mathValidSize(size float64) bool {
	return size >= 0 && !math.IsNaN(size) && !math.IsInf(size, 0)
}

// MathConstant returns a named OpenType MATH constant at size px.
// Distances are unhinted pixels. The three names ending in Percent or
// PercentScaleDown return ratios, so 80 percent is 0.8.
//
// The boolean reports whether the value came from a complete MATH constants
// table. Without that table, the value is a fallback derived from this face.
// Unknown names, invalid sizes, and nil faces return zero and false.
// Device adjustments and variation adjustments in MATH records are ignored.
// Non-Linux faces loaded from bytes cannot recover their MATH table.
// Use a file-backed face on those platforms when MATH data is needed.
func (f *Face) MathConstant(name string, size float64) (float64, bool) {
	if f == nil || f.gt == nil || f.upem <= 0 || !mathValidSize(size) {
		return 0, false
	}
	index := -1
	for i, candidate := range mathConstantNames {
		if candidate == name {
			index = i
			break
		}
	}
	if index < 0 {
		return 0, false
	}
	d := f.mathSource().table
	if len(d) >= 10 {
		if c, ok := d.resolve(int(d.u16(4)), 214); ok {
			switch index {
			case 0, 1:
				return float64(c.i16(index*2)) / 100, true
			case 2, 3:
				return float64(c.u16(index*2)) * size / f.upem, true
			case 55:
				return float64(c.i16(212)) / 100, true
			default:
				return float64(c.i16(8+(index-4)*4)) * size / f.upem, true
			}
		}
	}
	return f.mathConstantFallback(name, size), false
}

// UnderlineThickness returns the unhinted post-table underline thickness
// in pixels. A missing or nonpositive thickness falls back to 0.05 em.
// MathML uses this value as the rule thickness when MATH data is absent.
func (f *Face) UnderlineThickness(size float64) float64 {
	if f == nil || f.upem <= 0 || !mathValidSize(size) {
		return 0
	}
	if f.underlineThickness > 0 {
		return f.underlineThickness * size / f.upem
	}
	return size / 20
}

func (f *Face) mathConstantFallback(name string, size float64) float64 {
	x := f.Metrics(size).XHeight
	rule := f.UnderlineThickness(size)
	switch name {
	case "ScriptPercentScaleDown":
		return 0.71
	case "ScriptScriptPercentScaleDown":
		return float64(0.71 * 0.71)
	case "RadicalDegreeBottomRaisePercent":
		return 0.6
	case "AxisHeight", "AccentBaseHeight", "SubscriptBaselineDropMin", "SuperscriptBaselineDropMax":
		return x / 2
	case "FlattenedAccentBaseHeight", "SuperscriptShiftUp", "SuperscriptShiftUpCramped":
		return x
	case "SubscriptShiftDown":
		return x / 3
	case "SubscriptTopMax", "SuperscriptBottomMaxWithSubscript":
		return 4 * x / 5
	case "SuperscriptBottomMin":
		return x / 4
	case "SubSuperscriptGapMin", "SpaceAfterScript":
		return size / 5
	case "DelimitedSubFormulaMinHeight", "DisplayOperatorMinHeight":
		return size
	case "FractionNumeratorGapMin", "FractionDenominatorGapMin", "FractionRuleThickness",
		"OverbarRuleThickness", "OverbarExtraAscender", "UnderbarRuleThickness",
		"UnderbarExtraDescender", "RadicalRuleThickness", "RadicalExtraAscender":
		return rule
	case "StackGapMin", "FractionNumDisplayStyleGapMin", "FractionDenomDisplayStyleGapMin",
		"OverbarVerticalGap", "UnderbarVerticalGap":
		return 3 * rule
	case "StackDisplayStyleGapMin":
		return 7 * rule
	case "RadicalVerticalGap":
		return 5 * rule / 4
	case "RadicalDisplayStyleVerticalGap":
		return rule + float64(x/4)
	case "RadicalKernBeforeDegree":
		return 5 * size / 18
	case "RadicalKernAfterDegree":
		return -10 * size / 18
	default:
		return 0
	}
}

// MathFamily returns the available family used for the CSS math generic.
// Linux uses the existing fontconfig resolution of Latin Modern Math.
// macOS uses the existing generic chain, whose default primary face is Times.
// Unicode fallback can still select STIX for characters that Times lacks.
// Windows prefers Cambria Math and falls back to the library's serif face.
// The result can be passed to Resolve or Match without changing generic rules.
func (l *Library) MathFamily() string {
	if l == nil {
		return ""
	}
	if l.platform == PlatformLinux || l.platform == PlatformDarwin {
		if f := l.Resolve([]string{"math"}, 400, false).Primary(); f != nil {
			return f.Family
		}
	} else {
		if f := l.Match("Cambria Math", 400, false); f != nil {
			return f.Family
		}
	}
	if f := l.Resolve([]string{"serif"}, 400, false).Primary(); f != nil {
		return f.Family
	}
	return ""
}
