// Ported from es-toolkit/dist/compat/object/merge.mjs, mergeWith.mjs and
// dist/object/clone.mjs (es-toolkit 1.45.1), which mermaid's cleanAndMerge
// calls.
//
// 1.45.1 is the version inside the mermaid 12.2.0 bundle. It reads a null
// element of a source array as undefined (source[i] = source[i] ?? undefined).
// es-toolkit 1.53.1, the version in node_modules, keeps the null. Do not port
// that change.

package config

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// maxArrayLike bounds the array that merge builds from an object with a
// `length` property. Upstream loops up to that length without a bound.
const maxArrayLike = 1 << 20

// cleanAndMerge is cleanAndMerge in utils.ts: merge({}, defaultData, data).
func cleanAndMerge(defaultData, data any) (*js.Obj[any], error) {
	return merge(js.NewObj[any](), defaultData, data)
}

// merge merges the properties of one or more source objects into the target
// object (merge in es-toolkit/compat).
//
// It performs a deep merge, recursively merging nested objects and arrays. A
// source that is not an object is skipped. A source value of null replaces
// the target value.
func merge(object *js.Obj[any], sources ...any) (*js.Obj[any], error) {
	var result any = object
	for _, source := range sources {
		merged, err := mergeWithDeep(result, source, map[any]any{})
		if err != nil {
			return nil, err
		}
		result = merged
	}
	return result.(*js.Obj[any]), nil
}

// isArrayLikeObject is isArrayLikeObject: an object with a length that is a
// safe non-negative integer. Arrays are handled before this is called.
func isArrayLikeObject(value any) (int, bool) {
	o, ok := value.(*js.Obj[any])
	if !ok || o == nil {
		return 0, false
	}
	length, ok := o.Get("length").(float64)
	if !ok || length < 0 || length != math.Trunc(length) || length > 9007199254740991 {
		return 0, false
	}
	return int(length), true
}

// clone is clone of es-toolkit (dist/object/clone.mjs): the shallow copy that
// mergeWithDeep returns for a source that it has seen before.
func clone(v any) any {
	switch x := v.(type) {
	case []any:
		return append([]any{}, x...)
	case *js.Obj[any]:
		out := js.NewObj[any]()
		x.Each(func(k string, e any) { out.Set(k, e) })
		return out
	}
	return v
}

// setKey is target[key] = value, where the target is an object or an array.
// It returns the target, which is a new slice when an array grows. A key that
// is not an index is lost on an array: this model has no array properties.
func setKey(target any, key string, value any) any {
	switch t := target.(type) {
	case *js.Obj[any]:
		t.Set(key, value)
	case []any:
		if i, ok := js.IsArrayIndex(key); ok {
			for int(i) >= len(t) {
				t = append(t, nil)
			}
			t[i] = value
			return t
		}
	}
	return target
}

func mergeWithDeep(target, source any, stack map[any]any) (any, error) {
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
		// source = source.slice(), and source[i] = source[i] ?? undefined: a
		// null element reads as undefined.
		copied := make([]any, len(arr))
		for i, e := range arr {
			if !js.IsNullish(e) {
				copied[i] = e
			}
		}
		source = copied
	}
	// For an array source the keys are its indices.
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
				if length > maxArrayLike {
					return nil, &Error{Name: "RangeError", Message: "config: array-like object is too long"}
				}
				cloned := make([]any, length)
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
			merged, err := mergeWithDeep(targetValue, sourceValue, stack)
			if err != nil {
				return nil, err
			}
			value = merged
		case js.IsObject(targetValue) && js.IsObject(sourceValue):
			// One of the two is a plain object: the source is, because it is
			// not an array.
			merged, err := mergeWithDeep(targetValue, sourceValue, stack)
			if err != nil {
				return nil, err
			}
			value = merged
		case js.IsNullish(targetValue) && js.IsObject(sourceValue):
			merged, err := mergeWithDeep(js.NewObj[any](), sourceValue, stack)
			if err != nil {
				return nil, err
			}
			value = merged
		case targetValue == nil || sourceValue != nil:
			value = sourceValue
		default:
			continue
		}
		target = setKey(target, key, value)
		if hasID {
			// The stack maps the source to its target. Keep it on the
			// current slice when an array target grows.
			stack[sourceID] = target
		}
	}
	return target, nil
}
