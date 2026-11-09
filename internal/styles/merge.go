// Ported from es-toolkit/dist/compat/object/mergeWith.mjs (es-toolkit 1.45.1)
// and from cleanAndMerge in mermaid/src/utils.ts.
//
// 1.45.1 is the version inside the mermaid 12.2.0 bundle. It reads a null
// element of a source array as undefined (source[i] = source[i] ?? undefined).
// es-toolkit 1.53.1, the version in node_modules, keeps the null. Do not port
// that change.

package styles

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// cleanAndMerge is merge({}, defaultData, data): a deep copy of defaultData
// with data merged into it.
func cleanAndMerge(defaultData, data any) *js.Obj[any] {
	result := js.NewObj[any]()
	mergeWithDeep(result, defaultData)
	mergeWithDeep(result, data)
	return result
}

// mergeWithDeep merges source into target, for a target that is a plain
// object. The values are the dynamic values of internal/js: there are no
// class instances, typed arrays or cycles.
func mergeWithDeep(target *js.Obj[any], source any) {
	switch src := source.(type) {
	case *js.Obj[any]:
		if src == nil {
			return
		}
		for _, key := range src.Keys() {
			if key == "__proto__" {
				continue
			}
			target.Set(key, mergeValue(target.Get(key), target.Has(key), src.Get(key)))
		}
	case []any:
		// Object.keys of an array gives its indices.
		for i, sourceValue := range src {
			key := itoa(i)
			target.Set(key, mergeValue(target.Get(key), target.Has(key), sourceElement(sourceValue)))
		}
	}
}

// sourceElement is source[i] ?? undefined for an element of a source array:
// mergeWithDeep copies the array and reads null as undefined.
func sourceElement(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// mergeArray merges the array source into the array target by index.
func mergeArray(target []any, source []any) []any {
	out := make([]any, max(len(target), len(source)))
	copy(out, target)
	for i, sourceValue := range source {
		out[i] = mergeValue(out[i], i < len(target), sourceElement(sourceValue))
	}
	return out
}

// arrayLikeElements is the array that mergeWithDeep makes from a target that
// is an array-like object (isArrayLikeObject): an object with a length that is
// a safe integer and not negative. It returns nil for every other value.
// Upstream has no bound on the length. The port takes a length above maxLoop
// as no length.
func arrayLikeElements(value any) []any {
	o, ok := value.(*js.Obj[any])
	if !ok || o == nil {
		return nil
	}
	length, ok := o.Get("length").(float64)
	if !ok || length < 0 || length != math.Trunc(length) || length > maxLoop {
		return nil
	}
	out := make([]any, int(length))
	for i := range out {
		out[i] = o.Get(itoa(i))
	}
	return out
}

// mergeValue gives the new value of one property. hasTarget tells whether the
// target has the property at all.
func mergeValue(targetValue any, hasTarget bool, sourceValue any) any {
	if sourceArray, ok := sourceValue.([]any); ok {
		targetArray, isArray := targetValue.([]any)
		if !isArray {
			targetArray = arrayLikeElements(targetValue)
		}
		return mergeArray(targetArray, sourceArray)
	}
	if sourceObj, ok := sourceValue.(*js.Obj[any]); ok && sourceObj != nil {
		switch tv := targetValue.(type) {
		case *js.Obj[any]:
			if tv != nil {
				mergeWithDeep(tv, sourceObj)
				return tv
			}
		case []any:
			// es-toolkit sets the properties of the object on the array. The
			// value model has no array with named properties, so the port keeps
			// the index properties and drops the others.
			out := append([]any(nil), tv...)
			for _, key := range sourceObj.Keys() {
				if i, isIndex := js.IsArrayIndex(key); isIndex && int(i) < maxLoop {
					for len(out) <= int(i) {
						out = append(out, nil)
					}
					out[i] = mergeValue(out[i], true, sourceObj.Get(key))
				}
			}
			return out
		}
		if js.IsNullish(targetValue) {
			copied := js.NewObj[any]()
			mergeWithDeep(copied, sourceObj)
			return copied
		}
		// The target is a string, a number or a boolean: the source replaces it.
		return js.Clone(sourceValue)
	}
	if !hasTarget || targetValue == nil || sourceValue != nil {
		return sourceValue
	}
	return targetValue
}
