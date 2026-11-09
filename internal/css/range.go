// This file ports no upstream file. It has the properties for which Chromium
// rejects a negative number, from the cases of
// internal/dom/testdata/style_range.json.

package css

// nonNegative lists the properties whose value range has no negative
// numbers. Chromium rejects a negative number for them in the parser, so
// that the declaration is dropped. The list is what Chromium does for the
// cases of internal/dom/testdata/style_range.json.
var nonNegative = map[string]bool{
	"width": true, "height": true,
	"min-width": true, "min-height": true,
	"max-width": true, "max-height": true,
	"padding-top": true, "padding-right": true, "padding-bottom": true, "padding-left": true,
	"row-gap": true, "column-gap": true,
	"flex-basis": true, "flex-grow": true, "flex-shrink": true,
	"line-height": true, "tab-size": true,
	"stroke-width": true, "stroke-miterlimit": true,
	"r": true, "rx": true, "ry": true,
}

// rejectsNegative reports whether the value is a negative number, with or
// without a unit, for a property that allows none. The value is in lower
// case. A negative zero is zero. A math function such as calc() is not a
// number here: Chromium keeps it and clamps its result when it is used.
func rejectsNegative(name, lower string) bool {
	if !nonNegative[name] {
		return false
	}
	f, _, ok := parseDimension(lower)
	return ok && f < 0
}
