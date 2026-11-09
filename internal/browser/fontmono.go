// Ported from Chromium 156.0.8078.4 (Blink), in part: the font size of an
// element whose font family is only monospace. The functions and the two
// table rows of this file follow these parts of Blink:
//
//   - FontBuilder::CheckForGenericFamilyChange:
//     core/css/resolver/font_builder.cc (LGPL-2.0-or-later).
//   - FontSizeFunctions::FontSizeForKeyword and kStrictFontSizeTable:
//     core/css/font_size_functions.cc (LGPL-2.0-or-later).
//   - FontDescription::IsMonospace: platform/fonts/font_description.h
//     (LGPL-2.0-or-later).
//
// The paths are in third_party/blink/renderer. The license of each file is
// the license in its header at that version. See NOTICE in this directory.

package browser

import "strings"

// This file has the font size of an element whose font family is only
// `monospace`. Chromium has two default font sizes, 16px and 13px for
// monospace, and FontBuilder::CheckForGenericFamilyChange scales a font
// size that is not absolute when an element changes between the two kinds
// of family. testdata/fontmono-cases.json has the values of Chromium.

// fixedFontScale is the default size of a monospace font divided by the
// default font size: 13 / 16.
const fixedFontScale = float32(13) / 16

// fontSizeKeywordIndex gives the place of an absolute-size keyword in the
// rows of the tables of FontSizeFunctions::FontSizeForKeyword, from 1.
var fontSizeKeywordIndex = map[string]int{
	"xx-small": 1, "x-small": 2, "small": 3, "medium": 4, "large": 5, "x-large": 6, "xx-large": 7, "xxx-large": 8,
}

// The rows of kStrictFontSizeTable of Blink for the two default sizes.
var (
	keywordSizes     = [...]float64{9, 10, 13, 16, 18, 24, 32, 48}
	keywordSizesMono = [...]float64{9, 10, 12, 13, 16, 18, 26, 39}
)

// keywordFontSize is FontSizeForKeyword: the size of a keyword for a
// proportional family or for monospace.
func keywordFontSize(keyword int, mono bool) float64 {
	if mono {
		return keywordSizesMono[keyword-1]
	}
	return keywordSizes[keyword-1]
}

// isMonospace is FontDescription::IsMonospace: the family list is the
// generic family monospace and nothing else.
func isMonospace(families []string) bool {
	return len(families) == 1 && strings.EqualFold(families[0], "monospace")
}

// fontSizeIsRelative reports whether a font-size value depends on the font
// size of the parent: a percentage, smaller, larger, or a length in a
// font-relative unit. Such a size is absolute only when the size of the
// parent is. A length in rem depends on the root element and is absolute.
func fontSizeIsRelative(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "smaller" || v == "larger" || strings.HasSuffix(v, "%") {
		return true
	}
	if strings.HasSuffix(v, "rem") || strings.HasSuffix(v, "rlh") {
		return false
	}
	for _, unit := range []string{"em", "ex", "ch", "ic", "lh", "cap"} {
		if strings.HasSuffix(v, unit) {
			return true
		}
	}
	return false
}

// genericFamilyChange is FontBuilder::CheckForGenericFamilyChange for the
// style ts of an element with the style parent of its parent.
func genericFamilyChange(ts, parent *textStyle) {
	if ts.sizeAbsolute {
		return
	}
	mono, parentMono := isMonospace(ts.families), isMonospace(parent.families)
	if mono == parentMono {
		return
	}
	switch {
	case ts.sizeKeyword != 0:
		ts.size = keywordFontSize(ts.sizeKeyword, mono)
	case parentMono:
		ts.size = float64(float32(ts.size) / fixedFontScale)
	default:
		ts.size = float64(float32(ts.size) * fixedFontScale)
	}
}
