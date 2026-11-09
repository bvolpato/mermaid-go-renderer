// This file ports no upstream file. It has the model of a dynamic JavaScript
// value, its conversions, and JSON.parse and JSON.stringify.

package js

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// A dynamic JavaScript value is one of these Go types:
//
//	nil               undefined (also an absent property)
//	Null              null
//	bool              boolean
//	float64           number
//	string            string
//	[]any             array
//	*Obj[any]         plain object, with JavaScript property order
//
// Config objects and theme variables use this model, because upstream code
// merges, enumerates and prints them with JavaScript semantics.

type nullType struct{}

// Null is the JavaScript null value. A nil interface is undefined.
var Null = nullType{}

// IsNullish reports whether v is undefined or null.
func IsNullish(v any) bool {
	if v == nil {
		return true
	}
	_, ok := v.(nullType)
	return ok
}

// Truthy is the ToBoolean abstract operation.
func Truthy(v any) bool {
	switch x := v.(type) {
	case nil, nullType:
		return false
	case bool:
		return x
	case float64:
		return x != 0 && !math.IsNaN(x)
	case int:
		return x != 0
	case string:
		return x != ""
	}
	return true
}

// ToString is String(v).
func ToString(v any) string {
	switch x := v.(type) {
	case nil:
		return "undefined"
	case nullType:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return Str(x)
	case int:
		return Str(float64(x))
	case string:
		return x
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if !IsNullish(e) {
				parts[i] = ToString(e)
			}
		}
		return strings.Join(parts, ",")
	case *Obj[any]:
		return "[object Object]"
	}
	return fmt.Sprint(v)
}

// ToNumber is Number(v).
func ToNumber(v any) float64 {
	switch x := v.(type) {
	case nil:
		return math.NaN()
	case nullType:
		return 0
	case bool:
		if x {
			return 1
		}
		return 0
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		return Number(x)
	case []any:
		if len(x) == 0 {
			return 0
		}
		if len(x) == 1 {
			return ToNumber(x[0])
		}
	}
	return math.NaN()
}

// TypeOf is the typeof operator.
func TypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "undefined"
	case bool:
		return "boolean"
	case float64, int:
		return "number"
	case string:
		return "string"
	}
	return "object"
}

// AsObj returns v as an object, or nil when v is not a plain object.
func AsObj(v any) *Obj[any] {
	o, _ := v.(*Obj[any])
	return o
}

// Path reads a nested property, as a.b.c does with optional chaining.
func Path(v any, keys ...string) any {
	for _, k := range keys {
		o := AsObj(v)
		if o == nil {
			return nil
		}
		v = o.Get(k)
	}
	return v
}

// Clone is a deep copy (structuredClone for the dynamic value model).
func Clone(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = Clone(e)
		}
		return out
	case *Obj[any]:
		// The copy has the slots of x, so its properties have the order of
		// x. Clone does not write to x: other goroutines can read x.
		out := x.copyOwned()
		for i, e := range out.vals {
			switch e.(type) {
			case []any, *Obj[any]:
				out.vals[i] = Clone(e)
			}
		}
		return out
	}
	return v
}

// FreezeValue makes every object of a tree of dynamic values immutable (see
// Obj.Freeze), so that many goroutines can read the tree. It returns the
// tree. An array has no frozen state: FreezeValue returns it with no spare
// capacity, so that an append to it makes a new array, and the caller must
// not assign to its elements.
//
// A tree can have a part that is frozen already, and that other goroutines
// read at this moment: a config shares its deep values with the config that
// it comes from. FreezeValue writes nothing to such a part.
func FreezeValue(v any) any {
	switch x := v.(type) {
	case []any:
		for i, e := range x {
			switch e := e.(type) {
			case *Obj[any]:
				FreezeValue(e)
			case []any:
				// The element changes only when the array had spare
				// capacity, so it was not frozen before.
				if frozen := FreezeValue(e).([]any); cap(frozen) != cap(e) {
					x[i] = frozen
				}
			}
		}
		return x[:len(x):len(x)]
	case *Obj[any]:
		if x == nil || x.Frozen() {
			return v
		}
		// Freeze first: it removes the deleted slots.
		x.Freeze()
		for i, e := range x.vals {
			switch e.(type) {
			case []any, *Obj[any]:
				x.vals[i] = FreezeValue(e)
			}
		}
	}
	return v
}

// ParseJSON is JSON.parse. Objects keep the key order of a JavaScript object.
func ParseJSON(data []byte) (any, error) {
	// Most texts are plain: see plainJSON.
	p := plainJSON{data: data}
	if v, ok := p.value(); ok && p.end() {
		return v, nil
	}
	return parseJSONDecoder(data)
}

// parseJSONDecoder is ParseJSON by the decoder of encoding/json.
func parseJSONDecoder(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseJSONValue(dec)
	if err != nil {
		return nil, err
	}
	// JSON.parse rejects anything after the value except white space. The
	// decoder stops at the end of the value, so check the rest of the input.
	rest := data[dec.InputOffset():]
	if len(bytes.TrimLeft(rest, " \t\r\n")) != 0 {
		return nil, fmt.Errorf("json: unexpected data after the top-level value")
	}
	return v, nil
}

func parseJSONValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := NewObj[any]()
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				val, err := parseJSONValue(dec)
				if err != nil {
					return nil, err
				}
				obj.Set(keyTok.(string), val)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return obj, nil
		case '[':
			arr := []any{}
			for dec.More() {
				val, err := parseJSONValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return arr, nil
		}
		return nil, fmt.Errorf("json: unexpected delimiter %v", t)
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return nil, err
		}
		return f, nil
	case string:
		return t, nil
	case bool:
		return t, nil
	case nil:
		return Null, nil
	}
	return nil, fmt.Errorf("json: unexpected token %v", tok)
}

// plainJSON parses the JSON texts that need no decoding: every string has
// only printable ASCII characters and no escape. For such a text it gives
// the value that the decoder of encoding/json gives, without the tokens of
// that decoder. For any other text, and for every text that is not valid,
// value or end returns false, and ParseJSON uses the decoder: so the
// decoder still defines the result and the error.
type plainJSON struct {
	data  []byte
	pos   int
	depth int
	// keys has the last property names. The names of a JSON text repeat,
	// and each distinct name is one string.
	keys [64]string
}

// maxPlainDepth is the deepest nesting that plainJSON reads.
const maxPlainDepth = 256

func (p *plainJSON) space() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

// end reports whether only white space follows.
func (p *plainJSON) end() bool {
	p.space()
	return p.pos == len(p.data)
}

// str reads a string at the opening quote. It returns the bytes between
// the quotes.
func (p *plainJSON) str() ([]byte, bool) {
	start := p.pos + 1
	for i := start; i < len(p.data); i++ {
		c := p.data[i]
		if c == '"' {
			p.pos = i + 1
			return p.data[start:i], true
		}
		if c == '\\' || c < 0x20 || c >= 0x7f {
			return nil, false
		}
	}
	return nil, false
}

func (p *plainJSON) key(b []byte) string {
	if len(b) == 0 || len(b) > 24 {
		return string(b)
	}
	slot := &p.keys[(len(b)*31+int(b[0])*7+int(b[len(b)-1]))&63]
	if *slot != string(b) {
		*slot = string(b)
	}
	return *slot
}

func (p *plainJSON) literal(word string, v any) (any, bool) {
	if len(p.data)-p.pos < len(word) || string(p.data[p.pos:p.pos+len(word)]) != word {
		return nil, false
	}
	p.pos += len(word)
	return v, true
}

// number reads a number of the JSON grammar.
func (p *plainJSON) number() (any, bool) {
	d := p.data
	i := p.pos
	digits := func() bool {
		start := i
		for i < len(d) && d[i] >= '0' && d[i] <= '9' {
			i++
		}
		return i > start
	}
	if i < len(d) && d[i] == '-' {
		i++
	}
	if i < len(d) && d[i] == '0' {
		i++
	} else if !digits() {
		return nil, false
	}
	if i < len(d) && d[i] == '.' {
		i++
		if !digits() {
			return nil, false
		}
	}
	if i < len(d) && (d[i] == 'e' || d[i] == 'E') {
		i++
		if i < len(d) && (d[i] == '+' || d[i] == '-') {
			i++
		}
		if !digits() {
			return nil, false
		}
	}
	// The number ends at a delimiter or at the end of the text.
	if i < len(d) {
		switch d[i] {
		case ' ', '\t', '\n', '\r', ',', ']', '}':
		default:
			return nil, false
		}
	}
	f, err := strconv.ParseFloat(string(d[p.pos:i]), 64)
	if err != nil {
		// A number that is out of range: the decoder gives the error.
		return nil, false
	}
	p.pos = i
	return f, true
}

func (p *plainJSON) value() (any, bool) {
	p.space()
	if p.pos >= len(p.data) {
		return nil, false
	}
	switch c := p.data[p.pos]; c {
	case '{':
		if p.depth++; p.depth > maxPlainDepth {
			return nil, false
		}
		p.pos++
		obj := NewObj[any]()
		p.space()
		if p.pos < len(p.data) && p.data[p.pos] == '}' {
			p.pos++
			p.depth--
			return obj, true
		}
		for {
			p.space()
			if p.pos >= len(p.data) || p.data[p.pos] != '"' {
				return nil, false
			}
			name, ok := p.str()
			if !ok {
				return nil, false
			}
			p.space()
			if p.pos >= len(p.data) || p.data[p.pos] != ':' {
				return nil, false
			}
			p.pos++
			val, ok := p.value()
			if !ok {
				return nil, false
			}
			obj.Set(p.key(name), val)
			p.space()
			if p.pos >= len(p.data) {
				return nil, false
			}
			switch p.data[p.pos] {
			case ',':
				p.pos++
			case '}':
				p.pos++
				p.depth--
				return obj, true
			default:
				return nil, false
			}
		}
	case '[':
		if p.depth++; p.depth > maxPlainDepth {
			return nil, false
		}
		p.pos++
		arr := []any{}
		p.space()
		if p.pos < len(p.data) && p.data[p.pos] == ']' {
			p.pos++
			p.depth--
			return arr, true
		}
		for {
			val, ok := p.value()
			if !ok {
				return nil, false
			}
			arr = append(arr, val)
			p.space()
			if p.pos >= len(p.data) {
				return nil, false
			}
			switch p.data[p.pos] {
			case ',':
				p.pos++
			case ']':
				p.pos++
				p.depth--
				return arr, true
			default:
				return nil, false
			}
		}
	case '"':
		b, ok := p.str()
		if !ok {
			return nil, false
		}
		return string(b), true
	case 't':
		return p.literal("true", true)
	case 'f':
		return p.literal("false", false)
	case 'n':
		return p.literal("null", Null)
	default:
		if c == '-' || (c >= '0' && c <= '9') {
			return p.number()
		}
	}
	return nil, false
}

// StringifyJSON is JSON.stringify(v) with no indentation. A property whose
// value is undefined is left out, and undefined in an array prints as null.
func StringifyJSON(v any) string {
	var sb strings.Builder
	writeJSON(&sb, v)
	return sb.String()
}

func writeJSON(sb *strings.Builder, v any) {
	switch x := v.(type) {
	case nil, nullType:
		sb.WriteString("null")
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			sb.WriteString("null")
		} else {
			sb.WriteString(Str(x))
		}
	case int:
		sb.WriteString(Str(float64(x)))
	case string:
		writeJSONString(sb, x)
	case []any:
		sb.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				sb.WriteByte(',')
			}
			writeJSON(sb, e)
		}
		sb.WriteByte(']')
	case *Obj[any]:
		sb.WriteByte('{')
		first := true
		x.Each(func(k string, e any) {
			if e == nil {
				return
			}
			if !first {
				sb.WriteByte(',')
			}
			first = false
			writeJSONString(sb, k)
			sb.WriteByte(':')
			writeJSON(sb, e)
		})
		sb.WriteByte('}')
	default:
		sb.WriteString("null")
	}
}

// writeJSONString quotes a string as JSON.stringify does. It does not escape
// '<', '>' or '&', which encoding/json escapes by default.
func writeJSONString(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	plain := true
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c >= 0x80 || c == '"' || c == '\\' {
			plain = false
			break
		}
	}
	if plain {
		// ASCII with nothing to escape: the text as it is.
		sb.WriteString(s)
		sb.WriteByte('"')
		return
	}
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\b':
			sb.WriteString(`\b`)
		case '\f':
			sb.WriteString(`\f`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(sb, `\u%04x`, r)
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
}
