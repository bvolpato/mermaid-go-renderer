// Ported from packages/mermaid/src/themes/theme-helpers.js,
// themes/erDiagram-oldHardcodedValues.ts and the calculate method and
// getThemeVariables function of every themes/theme-*.js (mermaid 12.2.0).

// Package theme computes mermaid's theme variables.
//
// Each upstream theme is a class that sets some properties in its
// constructor, takes the user's overrides, and derives the other properties in
// updateColors. The files theme_*.go are a mechanical translation of those
// classes, written by tools/reference/gen-theme-go.mjs. This file holds the
// parts that the themes share.
//
// Theme variables are dynamic JavaScript values (see internal/js/value.go):
// a user can override any variable with any value, and the upstream code
// reacts to that with JavaScript semantics (truthiness, string concatenation,
// exceptions). The translation keeps those semantics.
package theme

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/khroma"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

// Values that were hardcoded in src/diagrams/er/styles.js
// (erDiagram-oldHardcodedValues.ts).
const (
	oldAttributeBackgroundColorOdd  = "#ffffff"
	oldAttributeBackgroundColorEven = "#f2f2f2"
)

// maxLoopSteps bounds the loops that run up to THEME_COLOR_LIMIT. A user can
// set that variable, and upstream then loops without a bound. This port
// returns an error instead of running out of memory.
const maxLoopSteps = 1 << 20

// Error is an exception that the JavaScript theme code throws, for example
// for a variable that is not a valid color.
type Error struct {
	Message   string
	TypeError bool
	Err       error // the khroma error, when there is one
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Err }

// thrown carries an Error through a panic, as a JavaScript throw does. It
// never leaves this package: GetThemeVariables recovers it.
type thrown struct{ err *Error }

func throwTypeError(format string, args ...any) {
	panic(thrown{&Error{Message: fmt.Sprintf(format, args...), TypeError: true}})
}

func throwKhroma(err error) {
	e := &Error{Message: err.Error(), Err: err}
	if ke, ok := err.(*khroma.Error); ok {
		e.TypeError = ke.TypeError
	}
	panic(thrown{e})
}

// themeVars is an instance of a Theme class ("this" in the upstream code).
type themeVars struct {
	o             *js.Obj[any]
	steps         int
	protoReplaced bool
}

func newThemeVars() *themeVars { return &themeVars{o: js.NewObj[any]()} }

// get is this[k].
func (t *themeVars) get(k string) any { return t.o.Get(k) }

// set is this[k] = v for a key that the theme code names.
func (t *themeVars) set(k string, v any) { t.o.Set(k, v) }

// assign is this[k] = v for a key that comes from the user's overrides.
func (t *themeVars) assign(k string, v any) {
	if k == "__proto__" {
		// The assignment calls the __proto__ setter. An object or null
		// replaces the prototype, and the methods of the class with it.
		// Another value is ignored.
		if js.TypeOf(v) == "object" {
			t.protoReplaced = true
		}
		return
	}
	t.o.Set(k, v)
}

// guard counts one iteration of a loop that has a dynamic bound.
func (t *themeVars) guard() {
	t.steps++
	if t.steps > maxLoopSteps {
		panic(thrown{&Error{Message: "theme: THEME_COLOR_LIMIT is too large"}})
	}
}

// themeClass is one entry of the object that themes/index.js exports.
type themeClass struct {
	construct    func() *themeVars
	updateColors func(*themeVars)
	calculate    func(t *themeVars, overrides any, updateColors func(*themeVars))
}

// callUpdateColors is this.updateColors(). An override named updateColors
// shadows the method, and a replaced prototype loses it.
func callUpdateColors(t *themeVars, updateColors func(*themeVars)) {
	if t.protoReplaced || t.o.Has("updateColors") {
		throwTypeError("this.updateColors is not a function")
	}
	updateColors(t)
}

// calculate is Theme.calculate, as most themes define it.
func calculate(t *themeVars, overrides any, updateColors func(*themeVars)) {
	if js.TypeOf(overrides) != "object" {
		// Calculate colors form base colors
		callUpdateColors(t, updateColors)
		return
	}

	keys, ok := js.OwnKeys(overrides)
	if !ok {
		throwTypeError("Cannot convert undefined or null to object")
	}

	// Copy values from overrides, this is mainly for base colors
	for _, k := range keys {
		t.assign(k, member(overrides, k))
	}

	// Calculate colors form base colors
	callUpdateColors(t, updateColors)
	// Copy values from overrides again in case of an override of derived value
	for _, k := range keys {
		t.assign(k, applyOverride(t.get(k), member(overrides, k)))
	}
}

// calculateBase is Theme.calculate of theme-base.js.
//
// `base` is the one theme documented as modifiable, so an explicit override
// has to be the thing that actually paints. Turning the gradient off when
// `nodeBorder` is overridden makes the override win. An explicit
// `useGradient` still takes precedence.
func calculateBase(t *themeVars, overrides any, updateColors func(*themeVars)) {
	calculate(t, overrides, updateColors)
	if js.TypeOf(overrides) != "object" {
		return
	}
	if js.HasOwn(overrides, "nodeBorder") && !js.HasOwn(overrides, "useGradient") {
		t.set("useGradient", false)
	}
}

// calculateDefault is Theme.calculate of theme-default.js.
func calculateDefault(t *themeVars, overrides any, updateColors func(*themeVars)) {
	// for all keys in this object, if it is 'calculated' then set it to undefined
	for _, k := range t.o.Keys() {
		if js.StrictEquals(t.get(k), "calculated") {
			t.set(k, nil)
		}
	}
	calculate(t, overrides, updateColors)
}

// Names returns the theme names in the order of themes/index.js.
func Names() []string { return append([]string(nil), themeNames...) }

// Has is Object.hasOwn(theme, name): it reports whether name is a theme.
func Has(name string) bool {
	_, ok := themes[name]
	return ok
}

// GetThemeVariables is theme[name].getThemeVariables(userOverrides).
//
// userOverrides is a dynamic JavaScript value: nil for undefined, or an
// object in the normal case. The result is a new object. As upstream, it can
// share nested objects and arrays with userOverrides.
//
// The error is a *Error when the JavaScript code throws, for example for an
// override that is not a color.
func GetThemeVariables(name string, userOverrides any) (vars *js.Obj[any], err error) {
	class, ok := themes[name]
	if !ok {
		return nil, fmt.Errorf("theme: unknown theme %q", name)
	}
	if o, isObj := userOverrides.(*js.Obj[any]); isObj && o == nil {
		userOverrides = nil
	}
	// The variables are a function of the theme and of the content of the
	// overrides. A render asks for the same ones several times, and renders
	// with the same theme ask for them again.
	buf := keyBuffers.Get().(*[]byte)
	defer keyBuffers.Put(buf)
	key, hasKey := appendOverridesKey(append(append((*buf)[:0], name...), 0), userOverrides)
	*buf = key[:0]
	if hasKey {
		if kept, hit := variables.GetBytes(key); hit {
			return kept.result(userOverrides), nil
		}
	}
	vars, err = computeThemeVariables(class, userOverrides)
	if hasKey && err == nil && memo.Enabled() {
		if kept, ok := keepVariables(vars, userOverrides); ok {
			variables.Put(string(key), kept)
		}
	}
	return vars, err
}

// variables has the results of GetThemeVariables by the theme name and the
// content of the overrides. See appendOverridesKey.
var variables = memo.New[*keptVariables](128)

// keyBuffers has buffers for the keys of variables: the key of a full set
// of overrides has about 10 kB.
var keyBuffers = sync.Pool{New: func() any { return new([]byte) }}

// keptVariables is a result of GetThemeVariables in the cache.
type keptVariables struct {
	// vars is the result, frozen.
	vars *js.Obj[any]
	// shared lists the places of the result that hold a value of the
	// overrides itself. Upstream keeps an override that is an array, or an
	// object where the theme has no object, in the result by reference. A
	// later call with equal overrides gets its own values in these places.
	shared []sharedValue
}

// sharedValue is result[key] (or result[key][sub] when hasSub is set), which
// is overrides[override].
type sharedValue struct {
	key      string
	sub      string
	hasSub   bool
	override string
}

// keepVariables makes the cache entry of a result. The second result is
// false when the result holds a value of the overrides in a place that the
// entry cannot describe.
func keepVariables(vars *js.Obj[any], overrides any) (*keptVariables, bool) {
	kept := &keptVariables{}
	// The objects and arrays of the overrides, by identity.
	var own map[any]string
	if o, isObj := overrides.(*js.Obj[any]); isObj && o != nil {
		o.Each(func(k string, v any) {
			if id, ok := js.Identity(v); ok {
				if own == nil {
					own = map[any]string{}
				}
				own[id] = k
			}
		})
	}
	ok := true
	if own != nil {
		// deep reports whether a value below the described places is a
		// value of the overrides.
		var deep func(v any) bool
		deep = func(v any) bool {
			if id, isRef := js.Identity(v); isRef {
				if _, shared := own[id]; shared {
					return true
				}
			}
			found := false
			switch x := v.(type) {
			case *js.Obj[any]:
				if x != nil {
					x.Each(func(_ string, e any) { found = found || deep(e) })
				}
			case []any:
				for _, e := range x {
					found = found || deep(e)
				}
			}
			return found
		}
		vars.Each(func(k string, v any) {
			id, isRef := js.Identity(v)
			if !isRef {
				return
			}
			if from, shared := own[id]; shared {
				kept.shared = append(kept.shared, sharedValue{key: k, override: from})
				return
			}
			switch x := v.(type) {
			case *js.Obj[any]:
				x.Each(func(k2 string, v2 any) {
					id2, isRef2 := js.Identity(v2)
					if !isRef2 {
						return
					}
					if from, shared := own[id2]; shared {
						kept.shared = append(kept.shared, sharedValue{key: k, sub: k2, hasSub: true, override: from})
					} else if deep(v2) {
						ok = false
					}
				})
			case []any:
				for _, e := range x {
					if deep(e) {
						ok = false
					}
				}
			}
		})
	}
	if !ok {
		return nil, false
	}
	kept.vars = js.FreezeValue(js.Clone(vars)).(*js.Obj[any])
	return kept, true
}

// result returns the variables for a call with the given overrides: a copy
// that the caller owns, with the values of these overrides in the shared
// places.
func (kept *keptVariables) result(overrides any) *js.Obj[any] {
	out := js.Clone(kept.vars).(*js.Obj[any])
	if len(kept.shared) > 0 {
		o := overrides.(*js.Obj[any])
		for _, s := range kept.shared {
			if s.hasSub {
				out.Get(s.key).(*js.Obj[any]).Set(s.sub, o.Get(s.override))
			} else {
				out.Set(s.key, o.Get(s.override))
			}
		}
	}
	return out
}

// appendOverridesKey appends the exact content of the overrides of
// GetThemeVariables to b: the keys in order and the values with their types.
//
// The second result is false for overrides that have no key, because the
// result is then not a function of the content alone. Upstream keeps some
// values of the overrides in the result as they are, by reference. The
// cache entry describes that for a value of the overrides (see
// keptVariables), and not for a value inside of one. So overrides have a key
// when each value is a primitive value, an array of primitive values, or an
// object of primitive values, and no object or array is there two times.
func appendOverridesKey(b []byte, overrides any) ([]byte, bool) {
	if overrides == nil {
		return append(b, 'u'), true
	}
	o, isObj := overrides.(*js.Obj[any])
	if !isObj {
		return b, false
	}
	ok := true
	// seen has the objects and arrays of the overrides, by identity.
	var seen []any
	once := func(v any) {
		if id, isRef := js.Identity(v); isRef {
			for _, s := range seen {
				if s == id {
					ok = false
				}
			}
			seen = append(seen, id)
		}
	}
	// primitive appends a primitive value. It returns false for a value
	// that is not one.
	primitive := func(v any) bool {
		switch x := v.(type) {
		case nil:
			b = append(b, 'u')
		case bool:
			if x {
				b = append(b, 't')
			} else {
				b = append(b, 'f')
			}
		case float64:
			b = binary.LittleEndian.AppendUint64(append(b, 'd'), math.Float64bits(x))
		case int:
			b = binary.LittleEndian.AppendUint64(append(b, 'i'), uint64(x))
		case string:
			b = binary.AppendUvarint(append(b, 's'), uint64(len(x)))
			b = append(b, x...)
		default:
			if v != any(js.Null) {
				return false
			}
			b = append(b, 'n')
		}
		return true
	}
	b = binary.AppendUvarint(append(b, 'o'), uint64(o.Len()))
	o.Each(func(k string, v any) {
		b = binary.AppendUvarint(b, uint64(len(k)))
		b = append(b, k...)
		switch x := v.(type) {
		case *js.Obj[any]:
			if x == nil {
				b = append(b, 'z')
				return
			}
			once(x)
			b = binary.AppendUvarint(append(b, 'o'), uint64(x.Len()))
			x.Each(func(k2 string, v2 any) {
				b = binary.AppendUvarint(b, uint64(len(k2)))
				b = append(b, k2...)
				if p, isNilObj := v2.(*js.Obj[any]); isNilObj && p == nil {
					b = append(b, 'z')
				} else if !primitive(v2) {
					ok = false
				}
			})
		case []any:
			once(x)
			b = binary.AppendUvarint(append(b, 'a'), uint64(len(x)))
			for _, e := range x {
				if !primitive(e) {
					ok = false
				}
			}
		default:
			if !primitive(v) {
				ok = false
			}
		}
	})
	return b, ok
}

// computeThemeVariables runs the theme code.
func computeThemeVariables(class themeClass, userOverrides any) (vars *js.Obj[any], err error) {
	defer func() {
		if r := recover(); r != nil {
			th, isThrown := r.(thrown)
			if !isThrown {
				panic(r)
			}
			vars, err = nil, th.err
		}
	}()
	t := class.construct()
	class.calculate(t, userOverrides, class.updateColors)
	return t.o, nil
}

// Variables is theme[name].getThemeVariables(overrides) for an object. A nil
// overrides is undefined.
func Variables(name string, overrides *js.Obj[any]) (*js.Obj[any], error) {
	if overrides == nil {
		return GetThemeVariables(name, nil)
	}
	return GetThemeVariables(name, overrides)
}

// mkBorder is mkBorder in theme-helpers.js.
func mkBorder(col, darkMode any) any {
	if js.Truthy(darkMode) {
		return adjust(col, khroma.S(-40), khroma.L(10))
	}
	return adjust(col, khroma.S(-40), khroma.L(-10))
}

// isPlainObject is isPlainObject in theme-helpers.js.
func isPlainObject(value any) bool {
	o, ok := value.(*js.Obj[any])
	return ok && o != nil
}

// applyOverride is applyOverride in theme-helpers.js: the value a theme
// variable takes once the user's override is applied.
//
// An object-valued variable such as `xyChart` keeps every key the theme
// generated and takes the user's keys on top, so overriding one of them does
// not discard the rest.
func applyOverride(generated, override any) any {
	if isPlainObject(generated) && isPlainObject(override) {
		merged := js.NewObj[any]()
		generated.(*js.Obj[any]).Each(func(k string, v any) { merged.Set(k, v) })
		override.(*js.Obj[any]).Each(func(k string, v any) { merged.Set(k, v) })
		return merged
	}
	return override
}

// The functions below are the JavaScript operators that the translated theme
// code uses on dynamic values.

// The translator emits these helpers for syntax that no theme uses today.
// The reference keeps them, so that a new upstream version translates
// without changes here.
var _ = []any{truthyNum, str, and, andf, add}

func truthy(v any) bool { return js.Truthy(v) }

func truthyNum(x float64) bool { return x != 0 && !math.IsNaN(x) }

func num(v any) float64 { return js.ToNumber(v) }

func str(v any) string { return js.ToString(v) }

// or is a || b for an operand b that has no side effect.
func or(a, b any) any {
	if js.Truthy(a) {
		return a
	}
	return b
}

// orf is a || b. b is evaluated only when a is falsy.
func orf(a any, b func() any) any {
	if js.Truthy(a) {
		return a
	}
	return b()
}

// and is a && b for an operand b that has no side effect.
func and(a, b any) any {
	if !js.Truthy(a) {
		return a
	}
	return b
}

// andf is a && b. b is evaluated only when a is truthy.
func andf(a any, b func() any) any {
	if !js.Truthy(a) {
		return a
	}
	return b()
}

// nullish is a ?? b for an operand b that has no side effect.
func nullish(a, b any) any {
	if js.IsNullish(a) {
		return b
	}
	return a
}

// nullishf is a ?? b. b is evaluated only when a is undefined or null.
func nullishf(a any, b func() any) any {
	if js.IsNullish(a) {
		return b()
	}
	return a
}

// cond is test ? a : b for operands that have no side effect.
func cond(test bool, a, b any) any {
	if test {
		return a
	}
	return b
}

// condNum is test ? a : b for two numbers.
func condNum(test bool, a, b float64) float64 {
	if test {
		return a
	}
	return b
}

// condf is test ? a : b. Only the chosen operand is evaluated.
func condf(test bool, a, b func() any) any {
	if test {
		return a()
	}
	return b()
}

func add(a, b any) any { return js.Add(a, b) }

func compare(a any, op string, b any) bool { return js.Compare(a, op, b) }

func strictEquals(a, b any) bool { return js.StrictEquals(a, b) }

// member is v[key]. It throws a TypeError for undefined and null.
func member(v any, key string) any {
	value, ok := js.GetProperty(v, key)
	if !ok {
		throwTypeError("Cannot read properties of %s (reading '%s')", js.ToString(v), key)
	}
	return value
}

// optMember is v?.[key].
func optMember(v any, key string) any {
	value, _ := js.GetProperty(v, key)
	return value
}

// obj is an object literal: keys and values alternate.
func obj(kv ...any) *js.Obj[any] {
	o := js.NewObj[any]()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1])
	}
	return o
}

// The khroma functions below throw the khroma error, as the JavaScript
// functions do.

func adjust(color any, channels ...khroma.ChannelValue) any {
	s, err := khroma.Adjust(color, channels...)
	if err != nil {
		throwKhroma(err)
	}
	return s
}

func darken(color any, amount float64) any {
	s, err := khroma.Darken(color, amount)
	if err != nil {
		throwKhroma(err)
	}
	return s
}

func lighten(color any, amount float64) any {
	s, err := khroma.Lighten(color, amount)
	if err != nil {
		throwKhroma(err)
	}
	return s
}

func invert(color any) any {
	s, err := khroma.Invert(color)
	if err != nil {
		throwKhroma(err)
	}
	return s
}

func isDark(color any) bool {
	dark, err := khroma.IsDark(color)
	if err != nil {
		throwKhroma(err)
	}
	return dark
}
