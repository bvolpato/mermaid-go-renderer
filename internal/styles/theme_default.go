// The theme variables of the default theme: getThemeVariables() of
// mermaid/src/themes/theme-default.js with no overrides.

package styles

import (
	_ "embed"
	"fmt"
	"math"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The style functions of treemap, cynefin, radar, wardley and railroad merge
// the theme variables of the config over the variables of the default theme.
// The call has no arguments in mermaid, so its result is a constant.
// tools/reference/gen-styles.mjs writes that constant from the built mermaid.
//
//go:embed theme_default.json
var themeDefaultJSON []byte

var themeDefault = sync.OnceValue(func() *js.Obj[any] {
	v, err := js.ParseJSON(themeDefaultJSON)
	if err != nil {
		panic(fmt.Sprintf("styles: theme_default.json: %v", err))
	}
	o := js.AsObj(decodeTagged(v))
	if o == nil {
		panic("styles: theme_default.json is not an object")
	}
	freeze(o)
	return o
})

// freeze reads the keys of every object in v. An object of internal/js sorts
// its index keys on the first read. After this call a read does not write, so
// goroutines can share the value.
func freeze(v any) {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			freeze(e)
		}
	case *js.Obj[any]:
		for _, key := range x.Keys() {
			freeze(x.Get(key))
		}
	}
}

// getThemeVariables gives the theme variables of the default theme. The
// result is shared. Do not change it.
func getThemeVariables() *js.Obj[any] {
	return themeDefault()
}

// taggedKey marks a value that JSON cannot hold. The generator scripts write
// undefined, NaN, Infinity and -0 as {"\u0001js": "NaN"}.
const taggedKey = "\u0001js"

// decodeTagged replaces each tagged value in a parsed JSON tree.
func decodeTagged(v any) any {
	switch x := v.(type) {
	case []any:
		for i, e := range x {
			x[i] = decodeTagged(e)
		}
		return x
	case *js.Obj[any]:
		if x.Len() == 1 && x.Has(taggedKey) {
			switch tag, _ := x.Get(taggedKey).(string); tag {
			case "undefined":
				return nil
			case "NaN":
				return math.NaN()
			case "Infinity":
				return math.Inf(1)
			case "-Infinity":
				return math.Inf(-1)
			case "-0":
				return math.Copysign(0, -1)
			}
		}
		for _, key := range x.Keys() {
			x.Set(key, decodeTagged(x.Get(key)))
		}
		return x
	}
	return v
}
