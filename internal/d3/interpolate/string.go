// Ported from d3-interpolate 3.0.1, d3-interpolate/src/string.js and
// d3-interpolate/src/value.js (ISC).

package interpolate

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/color"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

var reNumber = regexp.MustCompile(`[-+]?(?:\d+\.?\d*|\.?\d+)(?:[eE][-+]?\d+)?`)

// String is d3.interpolateString(a, b): it interpolates the numbers that are
// embedded in the two strings.
func String(a, b string) func(t float64) string {
	type part struct {
		text   string
		number bool // a placeholder for an interpolated number (null in d3)
	}
	type numberAt struct {
		i int
		x func(t float64) float64
	}
	var s []part
	var q []numberAt
	bi := 0 // scan index for next number in b
	// append adds a string constant, coalescing with a previous string.
	add := func(text string) {
		if n := len(s); n > 0 && !s[n-1].number {
			s[n-1].text += text
		} else {
			s = append(s, part{text: text})
		}
	}

	am := reNumber.FindAllStringIndex(a, -1)
	bm := reNumber.FindAllStringIndex(b, -1)
	// Interpolate pairs of numbers in a & b.
	for k := 0; k < len(am) && k < len(bm); k++ {
		if bs := bm[k][0]; bs > bi { // a string precedes the next number in b
			add(b[bi:bs])
		}
		na, nb := a[am[k][0]:am[k][1]], b[bm[k][0]:bm[k][1]]
		if na == nb { // numbers in a & b match
			add(nb)
		} else { // interpolate non-matching numbers
			s = append(s, part{number: true})
			q = append(q, numberAt{i: len(s) - 1, x: Number(js.Number(na), js.Number(nb))})
		}
		bi = bm[k][1]
	}

	// Add remains of b.
	if bi < len(b) {
		add(b[bi:])
	}

	// Special optimization for only a single match.
	// Otherwise, interpolate each of the numbers and rejoin the string.
	if len(s) < 2 {
		if len(q) > 0 {
			x := q[0].x
			return func(t float64) string { return js.Str(x(t)) }
		}
		return func(float64) string { return b }
	}
	return func(t float64) string {
		var sb strings.Builder
		k := 0
		for i, p := range s {
			if p.number {
				for q[k].i != i {
					k++
				}
				sb.WriteString(js.Str(q[k].x(t)))
			} else {
				sb.WriteString(p.text)
			}
		}
		return sb.String()
	}
}

// Value is d3.interpolate(a, b) for the dynamic values of internal/js. The
// type of b selects the interpolator:
//
//	nil, js.Null, bool   the constant b
//	float64              Number
//	string               RGB if b is a color, otherwise String
//	color.Color          RGB
//	[]any                an interpolator for each element (interpolateArray)
//	*js.Obj[any]         an interpolator for each property (interpolateObject)
//
// d3 also handles Date objects and typed arrays; the port has no value types
// for them, so the callers use Date and Number directly.
func Value(a, b any) func(t float64) any {
	switch bv := b.(type) {
	case float64:
		f := Number(js.ToNumber(a), bv)
		return func(t float64) any { return f(t) }
	case string:
		if c := color.Parse(bv); c != nil {
			f := RGB(toColor(a), c)
			return func(t float64) any { return f(t) }
		}
		f := String(js.ToString(a), bv)
		return func(t float64) any { return f(t) }
	case color.Color:
		f := RGB(toColor(a), bv)
		return func(t float64) any { return f(t) }
	case []any:
		return genericArray(a, bv)
	case *js.Obj[any]:
		return object(a, bv)
	}
	return func(float64) any { return b }
}

// toColor is the argument conversion of d3.rgb(value).
func toColor(v any) color.Color {
	if c, ok := v.(color.Color); ok {
		return c
	}
	return color.Parse(js.ToString(v))
}

// genericArray ports genericArray of array.js.
func genericArray(a any, b []any) func(t float64) any {
	aa, _ := a.([]any)
	nb := len(b)
	na := nb
	if len(aa) < na {
		na = len(aa)
	}
	x := make([]func(t float64) any, na)
	for i := 0; i < na; i++ {
		x[i] = Value(aa[i], b[i])
	}
	return func(t float64) any {
		c := make([]any, nb)
		for i := 0; i < na; i++ {
			c[i] = x[i](t)
		}
		copy(c[na:], b[na:])
		return c
	}
}

// object ports object.js.
func object(a any, b *js.Obj[any]) func(t float64) any {
	ao, _ := a.(*js.Obj[any])
	if ao == nil {
		ao = js.NewObj[any]()
	}
	type entry struct {
		key string
		f   func(t float64) any
	}
	var interp []entry
	constants := js.NewObj[any]()
	b.Each(func(k string, v any) {
		if ao.Has(k) {
			interp = append(interp, entry{k, Value(ao.Get(k), v)})
		} else {
			constants.Set(k, v)
		}
	})
	return func(t float64) any {
		c := js.NewObj[any]()
		constants.Each(func(k string, v any) { c.Set(k, v) })
		for _, e := range interp {
			c.Set(e.key, e.f(t))
		}
		return c
	}
}
