// Ported from mermaid 12.2.0, mermaid/src/diagrams/common/colorThemeGate.ts
// (MIT).

package styles

import (
	"math"
	"unicode/utf16"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// defaultColorSlots is how many palette slots to emit when the theme does not say.
const defaultColorSlots = 12

// maxColorSlots is the upper bound on slots. It bounds the loop.
const maxColorSlots = 64

// inColorThemes is COLOR_THEMES.has(theme): the themes that carry a
// categorical palette for per-item colouring.
func inColorThemes(theme any) bool {
	return is(theme, "redux-color") || is(theme, "redux-dark-color")
}

// hasPalette reports whether palette is an array that is not empty.
func hasPalette(palette any) bool {
	arr, ok := palette.([]any)
	return ok && len(arr) > 0
}

// isColorTheme reports whether theme renders per-item colour at all.
func isColorTheme(theme, palette any) bool {
	return !js.IsNullish(theme) && inColorThemes(theme) && hasPalette(palette)
}

// safeLook validates look before a template puts it into a selector. Every
// real look is a bare word. Any other value gives "classic".
func safeLook(look any) string {
	str := ""
	switch x := look.(type) {
	case string:
		str = x
	case float64:
		str = js.Str(x)
	case int:
		str = js.Str(float64(x))
	}
	if str == "" {
		return "classic"
	}
	// SAFE_LOOK is /^[\w-]+$/.
	for i := 0; i < len(str); i++ {
		c := str[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return "classic"
		}
	}
	return str
}

// paletteSlotCount is the number of slots that a palette has.
func paletteSlotCount(palette any) int {
	if arr, ok := palette.([]any); ok {
		return len(arr)
	}
	return 0
}

// colorSlotCount is the number of palette slots that a style sheet emits.
// Pass nil for palette when the JavaScript call has one argument.
func colorSlotCount(themeColorLimit, palette any) int {
	if hasPalette(palette) {
		return paletteSlotCount(palette)
	}
	limit, ok := themeColorLimit.(float64)
	if ok && limit == math.Trunc(limit) && !math.IsInf(limit, 0) && limit > 0 && limit <= maxColorSlots {
		return int(limit)
	}
	return defaultColorSlots
}

// paletteAt is palette[i % palette.length] for a value that a user can set to
// anything. JavaScript throws when the value is undefined or null.
func paletteAt(palette any, i int) any {
	switch x := palette.(type) {
	case []any:
		if len(x) == 0 {
			return nil
		}
		return x[i%len(x)]
	case string:
		units := utf16.Encode([]rune(x))
		if len(units) == 0 {
			return nil
		}
		return string(utf16.Decode(units[i%len(units) : i%len(units)+1]))
	}
	if js.IsNullish(palette) {
		throwf("TypeError: Cannot read properties of %s (reading 'length')", js.ToString(palette))
	}
	return nil
}
