// This file ports no upstream file. It has the helpers for the JavaScript
// values that the grammar and stateDb.ts pass around.
//
// The parser builds plain JavaScript objects, and the db changes them in
// place, shares them and copies them with JSON. The statements, the layout
// nodes and the layout edges are therefore dynamic values here too:
//
//	string         the text of a token, the value of an accessibility
//	               statement, an id
//	[]any          a document (the values of its lines), or a list of
//	               descriptions or of classes
//	*js.Obj[any]   a statement, a note, a node or an edge
//
// A property that is undefined is nil.

package state

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Error is an exception that the upstream JavaScript throws: an Error of
// stateDb.ts, or a TypeError of the JavaScript engine for a value that the
// code does not expect.
type Error struct {
	// Name is the JavaScript error class: "Error" or "TypeError".
	Name string
	// Message is error.message.
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(message string) *Error { return &Error{Name: "Error", Message: message} }

func typeError(message string) *Error { return &Error{Name: "TypeError", Message: message} }

// newObj builds an object from keys and values, in the order of an object
// literal. A nil value is a property that is undefined.
func newObj(pairs ...any) *js.Obj[any] {
	o := js.NewObj[any]()
	for i := 0; i+1 < len(pairs); i += 2 {
		o.Set(pairs[i].(string), pairs[i+1])
	}
	return o
}

// prop is v[key] for a value of a document. A line of a document is an
// object or a string, and a string has none of the properties that the db
// reads.
func prop(v any, key string) any {
	if o, ok := v.(*js.Obj[any]); ok && o != nil {
		return o.Get(key)
	}
	return nil
}

// strProp is v[key] when it is a string.
func strProp(v any, key string) (string, bool) {
	s, ok := prop(v, key).(string)
	return s, ok
}

// clone is clone(o): JSON.parse(JSON.stringify(o)). A property that is
// undefined is left out, and undefined in an array is null.
func clone(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			if e == nil {
				out[i] = js.Null
			} else {
				out[i] = clone(e)
			}
		}
		return out
	case *js.Obj[any]:
		if x == nil {
			return nil
		}
		out := js.NewObj[any]()
		x.Each(func(k string, e any) {
			if e != nil {
				out.Set(k, clone(e))
			}
		})
		return out
	}
	return v
}

// lengthOf is v.length for a string or an array. The second result is false
// for a value that has no length.
func lengthOf(v any) (int, bool) {
	switch x := v.(type) {
	case string:
		return js.UTF16Len(x), true
	case []any:
		return len(x), true
	}
	return 0, false
}

// anyStrings converts a list of strings to a JavaScript array.
func anyStrings(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}
