// Ported from js-yaml/lib/type/null.js, bool.js, int.js, float.js, str.js,
// seq.js and map.js, and from schema/failsafe.js and schema/json.js
// (js-yaml 4.3.0).

package jsyaml

import (
	"math"
	"math/big"
	"regexp"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// yamlType is a Type of the schema: the tag, the kind of node, and the
// functions that test and build the value. The data of resolve and of
// construct is a value of the loader: nil is the null of JavaScript.
type yamlType struct {
	tag       string
	kind      string
	resolve   func(data any) bool
	construct func(data any) any
}

func resolveAny(any) bool { return true }

var (
	strType = &yamlType{
		tag:     "tag:yaml.org,2002:str",
		kind:    "scalar",
		resolve: resolveAny,
		construct: func(data any) any {
			if data != nil {
				return data
			}
			return ""
		},
	}
	seqType = &yamlType{
		tag:     "tag:yaml.org,2002:seq",
		kind:    "sequence",
		resolve: resolveAny,
		construct: func(data any) any {
			if data != nil {
				return data
			}
			return &seq{items: []any{}}
		},
	}
	mapType = &yamlType{
		tag:     "tag:yaml.org,2002:map",
		kind:    "mapping",
		resolve: resolveAny,
		construct: func(data any) any {
			if data != nil {
				return data
			}
			return js.NewObj[any]()
		},
	}
	nullType = &yamlType{
		tag:       "tag:yaml.org,2002:null",
		kind:      "scalar",
		resolve:   resolveYamlNull,
		construct: func(any) any { return nil },
	}
	boolType = &yamlType{
		tag:     "tag:yaml.org,2002:bool",
		kind:    "scalar",
		resolve: resolveYamlBoolean,
		construct: func(data any) any {
			s, _ := data.(string)
			return s == "true" || s == "True" || s == "TRUE"
		},
	}
	intType = &yamlType{
		tag:     "tag:yaml.org,2002:int",
		kind:    "scalar",
		resolve: resolveYamlInteger,
		construct: func(data any) any {
			s, _ := data.(string)
			return parseYamlInteger(s)
		},
	}
	floatType = &yamlType{
		tag:     "tag:yaml.org,2002:float",
		kind:    "scalar",
		resolve: resolveYamlFloat,
		construct: func(data any) any {
			s, _ := data.(string)
			return constructYamlFloat(s)
		},
	}
)

// implicitTypes is the compiled implicit list of the JSON schema.
var implicitTypes = []*yamlType{nullType, boolType, intType, floatType}

// typeMap is the compiled type map of the JSON schema: the types by kind
// and by tag. The kind "fallback" has every type. The schema has no multi
// type.
var typeMap = func() map[string]map[string]*yamlType {
	m := map[string]map[string]*yamlType{
		"scalar":   {},
		"sequence": {},
		"mapping":  {},
		"fallback": {},
	}
	for _, t := range []*yamlType{nullType, boolType, intType, floatType, strType, seqType, mapType} {
		m[t.kind][t.tag] = t
		m["fallback"][t.tag] = t
	}
	return m
}()

// lookupType returns the type for a kind and a tag, or nil.
func lookupType(kind, tag string) *yamlType {
	return typeMap[kind][tag]
}

func resolveYamlNull(data any) bool {
	if data == nil {
		return true
	}
	s, ok := data.(string)
	if !ok {
		return false
	}

	return s == "~" || s == "null" || s == "Null" || s == "NULL"
}

func resolveYamlBoolean(data any) bool {
	s, ok := data.(string)
	if !ok {
		return false
	}

	return s == "true" || s == "True" || s == "TRUE" ||
		s == "false" || s == "False" || s == "FALSE"
}

func isHexCode(c rune) bool {
	return c >= 0x30 /* 0 */ && c <= 0x39 /* 9 */ ||
		c >= 0x41 /* A */ && c <= 0x46 /* F */ ||
		c >= 0x61 /* a */ && c <= 0x66 /* f */
}

func isOctCode(c rune) bool {
	return c >= 0x30 /* 0 */ && c <= 0x37 /* 7 */
}

func isDecCode(c rune) bool {
	return c >= 0x30 /* 0 */ && c <= 0x39 /* 9 */
}

func isFinite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func resolveYamlInteger(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	data := js.UTF16Units(text)

	max := len(data)
	index := 0
	hasDigits := false

	if max == 0 {
		return false
	}

	// at is data[index]: -1 for an index that is out of range.
	at := func(i int) rune {
		if i < max {
			return data[i]
		}
		return -1
	}

	ch := at(index)

	// sign
	if ch == '-' || ch == '+' {
		index++
		ch = at(index)
	}

	if ch == '0' {
		// 0
		if index+1 == max {
			return true
		}
		index++
		ch = at(index)

		// base 2, base 8, base 16

		if ch == 'b' {
			// base 2
			index++

			for ; index < max; index++ {
				ch = data[index]
				if ch != '0' && ch != '1' {
					return false
				}
				hasDigits = true
			}
			return hasDigits && isFinite(parseYamlInteger(text))
		}

		if ch == 'x' {
			// base 16
			index++

			for ; index < max; index++ {
				if !isHexCode(data[index]) {
					return false
				}
				hasDigits = true
			}
			return hasDigits && isFinite(parseYamlInteger(text))
		}

		if ch == 'o' {
			// base 8
			index++

			for ; index < max; index++ {
				if !isOctCode(data[index]) {
					return false
				}
				hasDigits = true
			}
			return hasDigits && isFinite(parseYamlInteger(text))
		}
	}

	// base 10 (except 0)

	for ; index < max; index++ {
		if !isDecCode(data[index]) {
			return false
		}
		hasDigits = true
	}

	if !hasDigits {
		return false
	}

	return isFinite(parseYamlInteger(text))
}

// parseIntDigits is parseInt(digits, radix) for a text that holds only
// digits of the radix: resolveYamlInteger checks that first. V8 rounds the
// mathematical value to the nearest number, also for a text with more
// digits than a number holds.
func parseIntDigits(digits string, radix int) float64 {
	if digits == "" {
		return math.NaN()
	}
	if radix == 10 {
		f, err := strconv.ParseFloat(digits, 64)
		if err != nil && !isRangeError(err) {
			return js.ParseInt(digits, radix)
		}
		return f
	}
	n, ok := new(big.Int).SetString(digits, radix)
	if !ok {
		return js.ParseInt(digits, radix)
	}
	f, _ := new(big.Float).SetInt(n).Float64()
	return f
}

// isRangeError says whether an error of strconv is a range error: the
// result is then an infinity, as in JavaScript.
func isRangeError(err error) bool {
	ne, ok := err.(*strconv.NumError)
	return ok && ne.Err == strconv.ErrRange
}

func parseYamlInteger(data string) float64 {
	value := data
	sign := 1.0

	// first is value[0], or 0 for an empty text.
	first := func() byte {
		if value == "" {
			return 0
		}
		return value[0]
	}

	ch := first()

	if ch == '-' || ch == '+' {
		if ch == '-' {
			sign = -1
		}
		value = value[1:]
		ch = first()
	}

	if value == "0" {
		return 0
	}

	if ch == '0' && len(value) > 1 {
		if value[1] == 'b' {
			return sign * parseIntDigits(value[2:], 2)
		}
		if value[1] == 'x' {
			return sign * parseIntDigits(value[2:], 16)
		}
		if value[1] == 'o' {
			return sign * parseIntDigits(value[2:], 8)
		}
	}

	return sign * parseIntDigits(value, 10)
}

var (
	// yamlFloatPattern is YAML_FLOAT_PATTERN: 2.5e4, 2.5 and integers, .inf
	// and .nan.
	yamlFloatPattern = regexp.MustCompile(
		`^(?:[-+]?(?:[0-9]+)(?:\.[0-9]*)?(?:[eE][-+]?[0-9]+)?` +
			`|\.[0-9]+(?:[eE][-+]?[0-9]+)?` +
			`|[-+]?\.(?:inf|Inf|INF)` +
			`|\.(?:nan|NaN|NAN))$`)

	// yamlFloatSpecialPattern is YAML_FLOAT_SPECIAL_PATTERN.
	yamlFloatSpecialPattern = regexp.MustCompile(
		`^(?:[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
)

func resolveYamlFloat(value any) bool {
	data, ok := value.(string)
	if !ok {
		return false
	}

	if !yamlFloatPattern.MatchString(data) {
		return false
	}

	if isFinite(js.ParseFloat(data)) {
		return true
	}

	return yamlFloatSpecialPattern.MatchString(data)
}

func constructYamlFloat(data string) float64 {
	value := js.ToLowerCase(data)
	sign := 1.0
	if value != "" && value[0] == '-' {
		sign = -1
	}

	if value != "" && (value[0] == '+' || value[0] == '-') {
		value = value[1:]
	}

	if value == ".inf" {
		if sign == 1 {
			return math.Inf(1)
		}
		return math.Inf(-1)
	} else if value == ".nan" {
		return math.NaN()
	}
	return sign * js.ParseFloat(value)
}
