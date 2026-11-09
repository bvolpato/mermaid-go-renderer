// JavaScript semantics that the style templates of mermaid depend on.

package styles

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// options is the argument of a diagram style function: the theme variables
// plus theme, look and svgId. A style function of mermaid can also call
// getConfig(), so options carries the config too.
//
// Upstream makes this object with spreads: { ...themeVariables, theme, look }
// and then { ...options, svgId }. A style function reads properties by name,
// and a few functions set properties. So the port does not copy the theme
// variables: it keeps the properties that were added or set in a second
// object, which a read looks at first.
type options struct {
	// base has the properties that the spread copies. It is not changed.
	base *js.Obj[any]
	// over has the properties that were set after the spread, or nil.
	over   *js.Obj[any]
	config *js.Obj[any]
}

// Get is options[key].
func (o *options) Get(key string) any {
	if o.over != nil {
		if v, ok := o.over.Lookup(key); ok {
			return v
		}
	}
	return o.base.Get(key)
}

// Set is options[key] = v.
func (o *options) Set(key string, v any) {
	if o.over == nil {
		o.over = js.NewObj[any]()
	}
	o.over.Set(key, v)
}

// with returns { ...options, key: v }: a copy that has one more property.
// A change of the copy does not change o.
func (o *options) with(key string, v any) *options {
	c := &options{base: o.base, over: js.NewObj[any](), config: o.config}
	if o.over != nil {
		o.over.Each(c.over.Set)
	}
	c.over.Set(key, v)
	return c
}

// object returns the options as one object, with the properties in the
// order of the spreads, for a style function that enumerates them.
func (o *options) object() *js.Obj[any] {
	out := js.NewObj[any]()
	o.base.Each(out.Set)
	if o.over != nil {
		o.over.Each(out.Set)
	}
	return out
}

// s prints a property as a template literal does. A missing property prints
// as "undefined".
func (o *options) s(key string) string {
	return js.ToString(o.Get(key))
}

// throwError is a JavaScript exception that a style template throws. The
// templates panic with it, and the exported functions recover it and return
// it as an error.
type throwError struct {
	err error
}

// throw stops the template with an error, as a JavaScript exception does.
func throw(err error) {
	panic(throwError{err})
}

func throwf(format string, args ...any) {
	throw(fmt.Errorf(format, args...))
}

// catch turns a throwError panic into an error. Use it with defer.
func catch(err *error) {
	if r := recover(); r != nil {
		te, ok := r.(throwError)
		if !ok {
			panic(r)
		}
		*err = te.err
	}
}

// mergedProperty is cleanAndMerge(defaultData, data)[key] for a caller that
// reads one property of the merged object. It merges that property alone, by
// the rules of cleanAndMerge: a style function that needs one theme variable
// does not copy all of them.
func mergedProperty(defaultData, data any, key string) any {
	var value any
	has := false
	for _, source := range []any{defaultData, data} {
		src, isObj := source.(*js.Obj[any])
		if !isObj {
			if _, isArray := source.([]any); isArray {
				// The keys of an array are its indices: the whole merge.
				return cleanAndMerge(defaultData, data).Get(key)
			}
			continue
		}
		// mergeWithDeep skips __proto__.
		if src == nil || key == "__proto__" || !src.Has(key) {
			continue
		}
		value = mergeValue(value, has, src.Get(key))
		has = true
	}
	return value
}

// text is a JavaScript string that a loop extends with +=. The loop adds
// the parts to a buffer, so that it does not copy the text for each part.
type text struct{ sb strings.Builder }

// add is text += part.
func (t *text) add(part string) { t.sb.WriteString(part) }

func (t *text) String() string { return t.sb.String() }

// s is String(v), the conversion of a template literal.
func s(v any) string {
	return js.ToString(v)
}

// itoa prints an integer as JavaScript prints it.
func itoa(i int) string {
	return strconv.Itoa(i)
}

// or is a || b.
func or(a, b any) any {
	if js.Truthy(a) {
		return a
	}
	return b
}

// nn is a ?? b.
func nn(a, b any) any {
	if js.IsNullish(a) {
		return b
	}
	return a
}

// cond is c ? a : b for values that are already computed.
func cond(c bool, a, b any) any {
	if c {
		return a
	}
	return b
}

// is is v === str.
func is(v any, str string) bool {
	x, ok := v.(string)
	return ok && x == str
}

// obj is the object in v, or an empty object when v is not an object. It is
// for spreads and destructuring, which read nothing from a value that is not
// an object.
func obj(v any) *js.Obj[any] {
	if o := js.AsObj(v); o != nil {
		return o
	}
	return js.NewObj[any]()
}

// prop is v[key]. JavaScript throws when v is undefined or null.
func prop(v any, key string) any {
	switch x := v.(type) {
	case *js.Obj[any]:
		if x != nil {
			return x.Get(key)
		}
	case []any:
		if key == "length" {
			return float64(len(x))
		}
		if i, ok := js.IsArrayIndex(key); ok && int(i) < len(x) {
			return x[i]
		}
		return nil
	case string:
		if key == "length" {
			return float64(js.UTF16Len(x))
		}
		return nil
	}
	if js.IsNullish(v) {
		throwf("TypeError: Cannot read properties of %s (reading '%s')", js.ToString(v), key)
	}
	return nil
}

// includes is v?.includes(search). It gives false for undefined and null. A
// value that has no includes method throws.
func includes(v any, search string) bool {
	switch x := v.(type) {
	case string:
		return strings.Contains(x, search)
	case []any:
		for _, e := range x {
			if is(e, search) {
				return true
			}
		}
		return false
	}
	if js.IsNullish(v) {
		return false
	}
	throwf("TypeError: %s.includes is not a function", js.ToString(v))
	return false
}

// replace is v.replace(search, replacement) with a string pattern: it replaces
// the first match and expands the "$" patterns of the replacement. A value
// that is not a string has no replace method, so it throws.
func replace(v any, search, replacement string) string {
	str, ok := v.(string)
	if !ok {
		throwf("TypeError: %s.replace is not a function", js.ToString(v))
	}
	position := strings.Index(str, search)
	if position < 0 {
		return str
	}
	var sb strings.Builder
	sb.WriteString(str[:position])
	for i := 0; i < len(replacement); i++ {
		c := replacement[i]
		if c != '$' || i+1 >= len(replacement) {
			sb.WriteByte(c)
			continue
		}
		switch replacement[i+1] {
		case '$':
			sb.WriteByte('$')
			i++
		case '&':
			sb.WriteString(search)
			i++
		case '`':
			sb.WriteString(str[:position])
			i++
		case '\'':
			sb.WriteString(str[position+len(search):])
			i++
		default:
			// "$1" stays as written, because a string pattern has no groups.
			sb.WriteByte(c)
		}
	}
	sb.WriteString(str[position+len(search):])
	return sb.String()
}

// maxLoop is the largest count that a loop over a theme value can have. The
// templates of mermaid loop on THEME_COLOR_LIMIT with no bound, so a count
// such as Infinity never ends there. The port returns an error.
const maxLoop = 10000

// loopCount is the number of rounds of "for (let i = 0; i < v; i++)".
func loopCount(v any) int {
	n := js.ToNumber(v)
	if math.IsNaN(n) || n <= 0 {
		return 0
	}
	if n > maxLoop {
		throwf("styles: the loop limit %s is too large", js.ToString(v))
	}
	return int(math.Ceil(n))
}

// elem is arr[i] for an array. An index outside the array gives undefined.
func elem(arr []any, i int) any {
	if i < 0 || i >= len(arr) {
		return nil
	}
	return arr[i]
}

// num makes a JavaScript number from an integer.
func num(i int) any {
	return float64(i)
}

// objOf makes an object from keys and values in order.
func objOf(keysAndValues ...any) *js.Obj[any] {
	o := js.NewObj[any]()
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		o.Set(keysAndValues[i].(string), keysAndValues[i+1])
	}
	return o
}

// add prints v + n for a number n. The + operator joins text when v is a
// string, an array or an object, and adds numbers for all other values.
func add(v any, n float64) string {
	switch v.(type) {
	case string, []any, *js.Obj[any]:
		return js.ToString(v) + js.Str(n)
	}
	return js.Str(js.ToNumber(v) + n)
}
