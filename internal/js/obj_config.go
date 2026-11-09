// This file ports no upstream file. It has a copy of a plain object that
// gives the result of assignWithDepth({}, src) of mermaid 12.2.0.

package js

// This file has the copy that the config port makes many times for each
// render: assignWithDepth({}, src) of mermaid/src/assignWithDepth.ts.

// AssignNew is assignWithDepth({}, src, { depth }) of mermaid for a plain
// object src: a copy of src in which the objects of the first depth+1 levels
// are new objects and the deeper values are the values of src.
//
// It gives the result of the port of assignWithDepth in internal/config for
// a new destination, without the work of that port: an object of the copy
// shares its keys with the object of src, and all objects of the copy come
// from two allocations. The second result is false when src has a value for
// which the copy is not a plain copy of the properties: null, which
// assignWithDepth skips, a key __proto__, which Object.assign skips, or a
// value of a type that is not a dynamic value. The caller then uses the
// port.
//
// src must be frozen, or belong to the calling goroutine: see Obj.Copy.
func AssignNew(src *Obj[any], depth int) (*Obj[any], bool) {
	var objs, vals int
	if !assignNewCount(src, depth, &objs, &vals) {
		return nil, false
	}
	a := assignArena{objs: make([]Obj[any], objs), vals: make([]any, vals)}
	return a.copy(src, depth), true
}

// assignNewCount counts the objects and the values of the copy. It returns
// false for a tree that AssignNew does not copy.
func assignNewCount(o *Obj[any], depth int, objs, vals *int) bool {
	*objs++
	*vals += len(o.vals)
	if depth <= 0 {
		// Object.assign(dst, src) of the port skips this key.
		return o.find("__proto__") < 0
	}
	for _, v := range o.vals {
		switch x := v.(type) {
		case nil, bool, float64, string, int:
		case *Obj[any]:
			// A nil pointer stands for undefined: the port copies it as it is.
			if x != nil && !assignNewCount(x, depth-1, objs, vals) {
				return false
			}
		case []any:
			*vals += len(x)
		default:
			// null, and types that are not dynamic values.
			return false
		}
	}
	return true
}

// assignArena has the storage of one copy.
type assignArena struct {
	objs []Obj[any]
	vals []any
}

func (a *assignArena) take(n int) []any {
	vals := a.vals[:n:n]
	a.vals = a.vals[n:]
	return vals
}

func (a *assignArena) copy(o *Obj[any], depth int) *Obj[any] {
	c := &a.objs[0]
	a.objs = a.objs[1:]
	o.copyInto(c, a.take(len(o.vals)))
	if depth <= 0 {
		return c
	}
	for i, v := range c.vals {
		switch x := v.(type) {
		case *Obj[any]:
			if x != nil {
				c.vals[i] = a.copy(x, depth-1)
			}
		case []any:
			// assignWithDepth([], src): the elements of src that the new
			// array does not have yet, in order.
			out := a.take(len(x))[:0]
			for _, e := range x {
				if !arrayIncludes(out, e) {
					out = append(out, e)
				}
			}
			c.vals[i] = out[:len(out):len(out)]
		}
	}
	return c
}

// arrayIncludes is Array.prototype.includes.
func arrayIncludes(arr []any, v any) bool {
	for _, e := range arr {
		if SameValueZero(e, v) {
			return true
		}
	}
	return false
}

// ReplaceObjects calls fn for every value of o that is an object or an
// array, and sets the property to the result. o must not be frozen.
func ReplaceObjects(o *Obj[any], fn func(v any) any) {
	if o.flags&objFrozen != 0 {
		panic("js: ReplaceObjects on a frozen object")
	}
	for i, v := range o.vals {
		switch x := v.(type) {
		case *Obj[any]:
			if x != nil {
				o.vals[i] = fn(v)
			}
		case []any:
			o.vals[i] = fn(v)
		}
	}
}
