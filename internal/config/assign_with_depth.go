// Ported from packages/mermaid/src/assignWithDepth.ts (mermaid 12.2.0).

package config

import "github.com/bvolpato/mermaid-go-renderer/internal/js"

// isNull reports whether v is the JavaScript null value.
func isNull(v any) bool { return v != nil && js.IsNullish(v) }

// typeofObject is typeof v === 'object'. It is true for null, for an array
// and for a plain object.
func typeofObject(v any) bool { return js.TypeOf(v) == "object" && !isNilObj(v) }

// isNilObj reports whether v is a nil *js.Obj, which stands for undefined.
func isNilObj(v any) bool {
	o, ok := v.(*js.Obj[any])
	return ok && o == nil
}

// includes is Array.prototype.includes.
func includes(arr []any, v any) bool {
	for _, e := range arr {
		if js.SameValueZero(e, v) {
			return true
		}
	}
	return false
}

// objectAssign is Object.assign(dst, src) for a destination that is a plain
// object. The key __proto__ is skipped: the assignment would call the
// prototype setter and add no property.
func objectAssign(dst *js.Obj[any], src any) {
	keys, ok := js.OwnKeys(src)
	if !ok {
		return
	}
	for _, k := range keys {
		if k == "__proto__" {
			continue
		}
		v, _ := js.GetProperty(src, k)
		dst.Set(k, v)
	}
}

// AssignWithDepth extends the functionality of Object.assign with the ability
// to merge arbitrary-depth objects.
//
// For each key in src with path k (recursively) it performs an
// Object.assign(dst[k], src[k]), with one change for an undefined dst[k]:
// dst[k] is initialized to {} and merged with src[k].
//
// Dissimilar types do not clobber: a string in src does not replace an object
// in dst. If src is an array of objects and dst is not an array, each element
// of src is applied to dst in order.
//
// dst is modified in place and returned. When dst is an array the result is
// the array with the new elements, as append returns it. depth is the depth
// to traverse within src and dst for merging; the upstream default is 2.
//
// All values are dynamic JavaScript values (see internal/js/value.go).
func AssignWithDepth(dst, src any, depth int) any {
	srcArr, srcIsArr := src.([]any)
	dstArr, dstIsArr := dst.([]any)
	if srcIsArr && !dstIsArr {
		for _, s := range srcArr {
			AssignWithDepth(dst, s, depth)
		}
		return dst
	} else if srcIsArr && dstIsArr {
		for _, s := range srcArr {
			if !includes(dstArr, s) {
				dstArr = append(dstArr, s)
			}
		}
		return dstArr
	}
	if isNilObj(dst) {
		dst = nil
	}
	if isNilObj(src) {
		src = nil
	}
	if js.IsNullish(dst) || depth <= 0 {
		if !js.IsNullish(dst) && typeofObject(dst) && typeofObject(src) {
			if d, ok := dst.(*js.Obj[any]); ok {
				objectAssign(d, src)
			}
			// Object.assign on an array destination sets properties on the
			// array. This model has no such properties.
			return dst
		}
		return src
	}
	dstObj, dstIsObj := dst.(*js.Obj[any])
	if !js.IsNullish(src) && typeofObject(dst) && typeofObject(src) {
		if !dstIsObj {
			// The destination is an array and the source is a plain object.
			// JavaScript sets properties on the array; see above.
			return dst
		}
		srcObj := src.(*js.Obj[any])
		for _, key := range srcObj.Keys() {
			srcValue := srcObj.Get(key)
			if typeofObject(srcValue) {
				if isNull(srcValue) {
					continue
				}
				if !dstObj.Has(key) {
					dstObj.Set(key, nil)
				}
				if dstObj.Get(key) == nil {
					if js.IsArray(srcValue) {
						dstObj.Set(key, []any{})
					} else {
						dstObj.Set(key, js.NewObj[any]())
					}
				}
				if typeofObject(dstObj.Get(key)) {
					dstObj.Set(key, AssignWithDepth(dstObj.Get(key), srcValue, depth-1))
				}
			} else if !typeofObject(dstObj.Get(key)) {
				dstObj.Set(key, srcValue)
			}
		}
	}
	return dst
}

// assignWithDepth is AssignWithDepth with the upstream default depth of 2.
func assignWithDepth(dst, src any) any { return AssignWithDepth(dst, src, 2) }

// assignToNew is assignWithDepth({}, src).
func assignToNew(src any) *js.Obj[any] {
	if o, ok := src.(*js.Obj[any]); ok && o != nil {
		// The copy of the usual config, without the work of the port below.
		// TestAssignNewAgainstPort compares the two.
		if dst, ok := js.AssignNew(o, 2); ok {
			return dst
		}
	}
	return assignToNewPort(src)
}

// assignToNewPort is assignToNew by the port of assignWithDepth.
func assignToNewPort(src any) *js.Obj[any] {
	dst := js.NewObj[any]()
	assignWithDepth(dst, src)
	return dst
}
