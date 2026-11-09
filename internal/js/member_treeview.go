// This file ports no mermaid file. It adds the property access v?.[name] on
// a dynamic value, with the properties that the value inherits from its
// prototype. The treeView diagram reads its icon maps this way: a file that
// has the name "constructor" finds the constructor of the map.

package js

// Inherited is the value of a property that a value inherits from its
// prototype: a method such as toString, or the prototype itself for
// __proto__. It is truthy, and it is not a string or a number.
type Inherited struct {
	// Name is the name of the property.
	Name string
	// Kind is the type of the value that has the property: "object",
	// "array", "string", "number" or "boolean". The property __proto__ of
	// a string is String.prototype, a String object with the value "", and
	// the one of an array is Array.prototype, an empty array.
	Kind string
}

// The names of the properties of the built-in prototypes in the V8 of
// Chromium 156 (Object.getOwnPropertyNames), without length.
var (
	objectPrototypeNames = nameSet(
		"constructor", "__defineGetter__", "__defineSetter__", "hasOwnProperty", "__lookupGetter__",
		"__lookupSetter__", "isPrototypeOf", "propertyIsEnumerable", "toString", "valueOf", "__proto__",
		"toLocaleString",
	)
	stringPrototypeNames = nameSet(
		"constructor", "anchor", "at", "big", "blink", "bold", "charAt", "charCodeAt", "codePointAt", "concat",
		"endsWith", "fontcolor", "fontsize", "fixed", "includes", "indexOf", "isWellFormed", "italics",
		"lastIndexOf", "link", "localeCompare", "match", "matchAll", "normalize", "padEnd", "padStart", "repeat",
		"replace", "replaceAll", "search", "slice", "small", "split", "strike", "sub", "substr", "substring", "sup",
		"startsWith", "toString", "toWellFormed", "trim", "trimStart", "trimLeft", "trimEnd", "trimRight",
		"toLocaleLowerCase", "toLocaleUpperCase", "toLowerCase", "toUpperCase", "valueOf",
	)
	arrayPrototypeNames = nameSet(
		"constructor", "at", "concat", "copyWithin", "fill", "find", "findIndex", "findLast", "findLastIndex",
		"lastIndexOf", "pop", "push", "reverse", "shift", "unshift", "slice", "sort", "splice", "includes",
		"indexOf", "join", "keys", "entries", "values", "forEach", "filter", "flat", "flatMap", "map", "every",
		"some", "reduce", "reduceRight", "toReversed", "toSorted", "toSpliced", "with", "toLocaleString",
		"toString",
	)
	numberPrototypeNames = nameSet(
		"constructor", "toExponential", "toFixed", "toPrecision", "toString", "valueOf", "toLocaleString",
	)
	booleanPrototypeNames = nameSet("constructor", "toString", "valueOf")
)

func nameSet(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// GetMember is v?.[name] for a dynamic value and a property name. The
// result is nil (undefined) for undefined, for null and for a property that
// does not exist. An own property gives its value: a property of an object,
// an element or the length of an array, a code unit or the length of a
// string. A property of the prototype of the value gives an Inherited.
func GetMember(v any, name string) any {
	inherited := func(kind string, prototypes ...map[string]bool) any {
		for _, names := range prototypes {
			if names[name] {
				return Inherited{Name: name, Kind: kind}
			}
		}
		return nil
	}
	switch x := v.(type) {
	case *Obj[any]:
		if x == nil {
			return nil
		}
		if value, ok := x.Lookup(name); ok {
			return value
		}
		return inherited("object", objectPrototypeNames)
	case []any:
		if i, ok := IsArrayIndex(name); ok {
			if int(i) < len(x) {
				return x[i]
			}
			return nil
		}
		if name == "length" {
			return float64(len(x))
		}
		return inherited("array", arrayPrototypeNames, objectPrototypeNames)
	case string:
		if i, ok := IsArrayIndex(name); ok {
			if units := UTF16Units(x); int(i) < len(units) {
				return FromUTF16Units(units[i : i+1])
			}
			return nil
		}
		if name == "length" {
			return float64(UTF16Len(x))
		}
		return inherited("string", stringPrototypeNames, objectPrototypeNames)
	case float64, int:
		return inherited("number", numberPrototypeNames, objectPrototypeNames)
	case bool:
		return inherited("boolean", booleanPrototypeNames, objectPrototypeNames)
	}
	return nil
}
