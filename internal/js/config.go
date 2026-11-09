// This file ports no upstream file. It has the operators, the property
// access and the identity test of ECMAScript on dynamic values.

package js

import (
	"math"
	"strings"
	"unicode/utf16"
	"unsafe"
)

// This file holds the JavaScript semantics that the config, theme and khroma
// ports need on dynamic values: operators, property access and identity.

// IsWhiteSpace reports whether r is matched by \s in a regular expression and
// removed by String.prototype.trim (WhiteSpace and LineTerminator).
func IsWhiteSpace(r rune) bool { return isJSSpace(r) }

// TrimStart is String.prototype.trimStart.
func TrimStart(s string) string { return strings.TrimLeftFunc(s, isJSSpace) }

// TrimEnd is String.prototype.trimEnd.
func TrimEnd(s string) string { return strings.TrimRightFunc(s, isJSSpace) }

// IsArray is Array.isArray.
func IsArray(v any) bool {
	_, ok := v.([]any)
	return ok
}

// IsObject reports whether typeof v is "object" and v is not null: a plain
// object or an array.
func IsObject(v any) bool {
	switch x := v.(type) {
	case []any:
		return true
	case *Obj[any]:
		return x != nil
	}
	return false
}

// arrayIdentity identifies an array by its backing store. Two slices with the
// same first element and length are the same JavaScript array. An empty array
// has no identity in this model.
type arrayIdentity struct {
	data *any
	n    int
}

// Identity returns a comparable key for the identity of an object or an
// array, as used by ===, Array.prototype.includes and Map keys. The second
// result is false for a primitive and for an empty array.
func Identity(v any) (any, bool) {
	switch x := v.(type) {
	case *Obj[any]:
		return x, x != nil
	case []any:
		if len(x) == 0 {
			return nil, false
		}
		return arrayIdentity{unsafe.SliceData(x), len(x)}, true
	}
	return nil, false
}

// StrictEquals is the === operator. Objects and arrays compare by identity.
func StrictEquals(a, b any) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case nullType:
		_, ok := b.(nullType)
		return ok
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case float64:
		switch y := b.(type) {
		case float64:
			return x == y
		case int:
			return x == float64(y)
		}
		return false
	case int:
		return StrictEquals(float64(x), b)
	case string:
		y, ok := b.(string)
		return ok && x == y
	}
	ia, oka := Identity(a)
	ib, okb := Identity(b)
	return oka && okb && ia == ib
}

// SameValueZero is the comparison of Array.prototype.includes: it is === with
// NaN equal to NaN.
func SameValueZero(a, b any) bool {
	if x, ok := a.(float64); ok && math.IsNaN(x) {
		y, ok := b.(float64)
		return ok && math.IsNaN(y)
	}
	return StrictEquals(a, b)
}

// isPrimitiveString reports whether ToPrimitive(v) is a string. An array or a
// plain object converts to a string.
func isPrimitiveString(v any) bool {
	switch v.(type) {
	case string, []any, *Obj[any]:
		return true
	}
	return false
}

// Add is the + operator.
func Add(a, b any) any {
	if isPrimitiveString(a) || isPrimitiveString(b) {
		return ToString(a) + ToString(b)
	}
	return ToNumber(a) + ToNumber(b)
}

// LessThan is the abstract relational comparison a < b. The second result is
// false when the comparison is undefined (an operand is NaN). The other
// operators derive from it: a > b is LessThan(b, a), a <= b is !LessThan(b, a)
// and a >= b is !LessThan(a, b), each false when the comparison is undefined.
func LessThan(a, b any) (less, defined bool) {
	if isPrimitiveString(a) && isPrimitiveString(b) {
		return CompareStrings(ToString(a), ToString(b)) < 0, true
	}
	x, y := toNumeric(a), toNumeric(b)
	if math.IsNaN(x) || math.IsNaN(y) {
		return false, false
	}
	return x < y, true
}

// toNumeric is ToNumber(ToPrimitive(v, number)).
func toNumeric(v any) float64 {
	switch v.(type) {
	case []any, *Obj[any]:
		return Number(ToString(v))
	}
	return ToNumber(v)
}

// Compare evaluates a relational operator: "<", ">", "<=" or ">=".
func Compare(a any, op string, b any) bool {
	switch op {
	case "<":
		less, _ := LessThan(a, b)
		return less
	case ">":
		less, _ := LessThan(b, a)
		return less
	case "<=":
		less, defined := LessThan(b, a)
		return defined && !less
	case ">=":
		less, defined := LessThan(a, b)
		return defined && !less
	}
	return false
}

// GetProperty is the property read v[key] for a dynamic value. The second
// result is false when v is undefined or null, where JavaScript throws a
// TypeError. Only own properties exist in this model, plus the indices and
// the length of strings and arrays.
func GetProperty(v any, key string) (any, bool) {
	switch x := v.(type) {
	case nil, nullType:
		return nil, false
	case *Obj[any]:
		if x == nil {
			return nil, false
		}
		return x.Get(key), true
	case []any:
		if key == "length" {
			return float64(len(x)), true
		}
		if i, ok := IsArrayIndex(key); ok && int(i) < len(x) {
			return x[i], true
		}
	case string:
		if key == "length" {
			return float64(UTF16Len(x)), true
		}
		if i, ok := IsArrayIndex(key); ok {
			units := utf16.Encode([]rune(x))
			if int(i) < len(units) {
				return string(utf16.Decode(units[i : i+1])), true
			}
		}
	}
	return nil, true
}

// OwnKeys is Object.keys(v) for a dynamic value. The second result is false
// when v is undefined or null, where JavaScript throws a TypeError.
func OwnKeys(v any) ([]string, bool) {
	switch x := v.(type) {
	case nil, nullType:
		return nil, false
	case *Obj[any]:
		if x == nil {
			return nil, false
		}
		return x.Keys(), true
	case []any:
		keys := make([]string, len(x))
		for i := range x {
			keys[i] = uitoa(uint32(i))
		}
		return keys, true
	case string:
		n := UTF16Len(x)
		keys := make([]string, n)
		for i := range keys {
			keys[i] = uitoa(uint32(i))
		}
		return keys, true
	}
	return nil, true
}

// HasOwn is Object.hasOwn(v, key) for a dynamic value that is not nullish.
func HasOwn(v any, key string) bool {
	switch x := v.(type) {
	case *Obj[any]:
		return x != nil && x.Has(key)
	case []any:
		if key == "length" {
			return true
		}
		i, ok := IsArrayIndex(key)
		return ok && int(i) < len(x)
	case string:
		if key == "length" {
			return true
		}
		i, ok := IsArrayIndex(key)
		return ok && int(i) < UTF16Len(x)
	}
	return false
}
