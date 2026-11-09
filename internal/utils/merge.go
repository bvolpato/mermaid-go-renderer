// Ported from es-toolkit 1.45.1, the version that the mermaid 12.2.0 bundle
// has: dist/compat/object/merge.mjs, dist/compat/object/mergeWith.mjs and
// dist/object/clone.mjs. cleanAndMerge of utils.ts calls merge.
//
// The values are the dynamic values of internal/js: undefined, null,
// booleans, numbers, strings, arrays and plain objects. The branches of
// upstream for other kinds of objects (typed arrays, arguments objects,
// buffers) cannot be reached with these values and are left out.

package utils

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// maxArrayLength bounds an array that merge makes or extends. Upstream has
// no bound: it makes a sparse array, or loops up to the length of an
// array-like object.
const maxArrayLength = 1 << 20

// MergeError is an error of CleanAndMergeErr. Name is the name of the
// JavaScript error that upstream throws ("RangeError").
type MergeError struct {
	Name    string
	Message string
}

func (e *MergeError) Error() string { return e.Name + ": " + e.Message }

var (
	errInvalidArrayLength = &MergeError{Name: "RangeError", Message: "Invalid array length"}
	errArrayTooLong       = &MergeError{Name: "RangeError", Message: "merge: the array is too long"}
)

// merger holds the mode of one merge. A strict merge stops with an error
// where upstream throws, or where an array passes maxArrayLength. A merge
// that is not strict ignores the assignment instead.
type merger struct {
	strict bool
}

// mergeStack is the Map of mergeWithDeep that maps a source to its target.
// The keys are identities of objects and arrays.
type mergeStack map[any]any

// merge is merge(object, ...sources): mergeWith with a customizer that
// returns undefined.
func (m merger) merge(object any, sources ...any) (any, error) {
	result := object
	for _, source := range sources {
		merged, err := m.mergeWithDeep(result, source, mergeStack{})
		if err != nil {
			return nil, err
		}
		result = merged
	}
	return result, nil
}

// isArrayLikeObject is isArrayLikeObject for a value that is not an array:
// an object with a length that is a safe integer and not negative.
func isArrayLikeObject(value any) (float64, bool) {
	o, ok := value.(*js.Obj[any])
	if !ok || o == nil {
		return 0, false
	}
	length, ok := o.Get("length").(float64)
	if !ok || length < 0 || length != math.Trunc(length) || length > 9007199254740991 {
		return 0, false
	}
	return length, true
}

// clone is clone of es-toolkit (dist/object/clone.mjs): a shallow copy.
func clone(v any) any {
	switch x := v.(type) {
	case []any:
		return append([]any{}, x...)
	case *js.Obj[any]:
		if x == nil {
			return v
		}
		out := js.NewObj[any]()
		x.Each(func(k string, e any) { out.Set(k, e) })
		return out
	}
	return v
}

// setKey is target[key] = value for a target that is an object or an array.
// It returns the target, which is a new slice when an array grows or
// shrinks. On an array, a key that is not an index or "length" is a
// property of the array object upstream. This model has no such properties,
// so the value is lost.
func (m merger) setKey(target any, key string, value any) (any, error) {
	switch t := target.(type) {
	case *js.Obj[any]:
		t.Set(key, value)
	case []any:
		if i, ok := js.IsArrayIndex(key); ok {
			if int64(i) >= maxArrayLength {
				if m.strict {
					return nil, errArrayTooLong
				}
				return t, nil
			}
			for int(i) >= len(t) {
				t = append(t, nil)
			}
			t[i] = value
			return t, nil
		}
		if key == "length" {
			// The length setter of an array: the value must convert to an
			// integer from 0 to 2^32-1, or the setter throws a RangeError.
			n := js.ToNumber(value)
			if !(n >= 0 && n == math.Trunc(n) && n <= 4294967295) {
				if m.strict {
					return nil, errInvalidArrayLength
				}
				return t, nil
			}
			if n > maxArrayLength {
				if m.strict {
					return nil, errArrayTooLong
				}
				return t, nil
			}
			for len(t) < int(n) {
				t = append(t, nil)
			}
			return t[:int(n):int(n)], nil
		}
	}
	return target, nil
}

func (m merger) mergeWithDeep(target, source any, stack mergeStack) (any, error) {
	if !js.IsObject(source) {
		return target, nil
	}
	sourceID, hasID := js.Identity(source)
	if hasID {
		if seen, ok := stack[sourceID]; ok {
			return clone(seen), nil
		}
		stack[sourceID] = target
	}
	if arr, ok := source.([]any); ok {
		// source = source.slice(), and source[i] = source[i] ?? void 0.
		copied := make([]any, len(arr))
		for i, e := range arr {
			if !js.IsNullish(e) {
				copied[i] = e
			}
		}
		source = copied
	}
	sourceKeys, _ := js.OwnKeys(source)
	for _, key := range sourceKeys {
		if key == "__proto__" { // isUnsafeProperty
			continue
		}
		sourceValue, _ := js.GetProperty(source, key)
		targetValue, _ := js.GetProperty(target, key)
		if js.IsArray(sourceValue) {
			if arr, ok := targetValue.([]any); ok {
				targetValue = append([]any{}, arr...)
			} else if length, ok := isArrayLikeObject(targetValue); ok {
				if length > maxArrayLength {
					if m.strict {
						return nil, errArrayTooLong
					}
					length = 0
				}
				cloned := make([]any, int(length))
				for i := range cloned {
					cloned[i], _ = js.GetProperty(targetValue, js.Str(float64(i)))
				}
				targetValue = cloned
			} else {
				targetValue = []any{}
			}
		}
		var value any
		switch {
		case js.IsArray(sourceValue):
			merged, err := m.mergeWithDeep(targetValue, sourceValue, stack)
			if err != nil {
				return nil, err
			}
			value = merged
		case js.IsObject(targetValue) && js.IsObject(sourceValue):
			// The source value is a plain object here, because it is not
			// an array, so the isPlainObject test of upstream is true.
			merged, err := m.mergeWithDeep(targetValue, sourceValue, stack)
			if err != nil {
				return nil, err
			}
			value = merged
		case js.IsNullish(targetValue) && js.IsObject(sourceValue):
			merged, err := m.mergeWithDeep(js.NewObj[any](), sourceValue, stack)
			if err != nil {
				return nil, err
			}
			value = merged
		case targetValue == nil || sourceValue != nil:
			value = sourceValue
		default:
			continue
		}
		updated, err := m.setKey(target, key, value)
		if err != nil {
			return nil, err
		}
		target = updated
		if hasID {
			// Keep the stack on the current slice when an array target
			// grows: upstream holds the array object itself.
			stack[sourceID] = target
		}
	}
	return target, nil
}
