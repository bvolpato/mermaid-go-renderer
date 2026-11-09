// Ported from the JSON schema of js-yaml as mermaid 12.2.0 bundles it
// (type/null.js, type/bool.js, type/int.js, type/float.js and the parts of
// loader.js that build mappings and resolve scalars).

package config

import (
	"errors"
	"io"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Upstream parses front matter with js-yaml and its JSON_SCHEMA. This port
// lets gopkg.in/yaml.v3 parse the document into nodes, and then builds the
// JavaScript value with the rules of js-yaml:
//
//   - A plain scalar is null, a boolean, an integer or a float when the
//     js-yaml resolver of that type accepts it, and a string otherwise. The
//     resolvers of yaml.v3 are not used: they also accept YAML 1.1 forms such
//     as 0777, 1_000 and 2001-01-01.
//   - A mapping keeps its key order. Every key becomes a string, as
//     String(key) does. A duplicated key is an error.
//   - The merge key `<<` is an ordinary key: the JSON schema has no merge type.
//   - An alias is the same object as its anchor.
//
// The two parsers can still disagree on what is a syntax error, for example
// on tabs in the indentation.

// yamlException is the YAMLException that js-yaml throws.
func yamlException(format string, args ...any) *Error {
	e := typeError(format, args...)
	e.Name = "YAMLException"
	return e
}

// loadYAML is yaml.load(text, { schema: yaml.JSON_SCHEMA }). The result is
// nil (undefined) for a text with no document.
func loadYAML(text string) (any, error) {
	if strings.ContainsRune(text, 0) {
		return nil, yamlException("null byte is not allowed in input")
	}
	// js-yaml adds a final line break to the input and drops a byte order
	// mark. The line break matters for a block scalar at the end of the text.
	if text != "" {
		if last := text[len(text)-1]; last != '\n' && last != '\r' {
			text += "\n"
		}
		text = strings.TrimPrefix(text, "\ufeff")
	}

	// yaml.v3 follows YAML 1.1 and breaks lines at U+0085, U+2028 and U+2029.
	// js-yaml follows YAML 1.2, where these are content. They travel through
	// yaml.v3 as private-use characters, unless the text already has those.
	restore := func(v any) any { return v }
	if strings.ContainsAny(text, lineBreaks11) && !strings.ContainsAny(text, lineBreakStandIns) {
		text = toStandIns.Replace(text)
		restore = restoreLineBreaks
	}

	doc, found, err := decodeYAML(text)
	if err != nil {
		// Two inputs that js-yaml reads and yaml.v3 rejects are repaired and
		// read again. A text that yaml.v3 accepts is never changed.
		repaired := text
		switch {
		case strings.Contains(err.Error(), "unknown escape character"):
			repaired = unescapeSlashes(text)
		case strings.Contains(err.Error(), "tab character that violates indentation"),
			strings.Contains(err.Error(), "character that cannot start any token"):
			repaired = repairTabs(text)
		}
		if repaired == text {
			return nil, &Error{Name: "YAMLException", Message: err.Error(), Err: err}
		}
		var retryErr error
		doc, found, retryErr = decodeYAML(repaired)
		if retryErr != nil {
			return nil, &Error{Name: "YAMLException", Message: err.Error(), Err: err}
		}
	}
	if !found {
		// js-yaml skips leading spaces and then reads documents while input
		// remains. A text with a line break or a comment is one empty
		// document, which is null. A text with only spaces has no document.
		if strings.TrimLeft(text, " ") == "\n" || text == "" {
			return nil, nil
		}
		return js.Null, nil
	}
	c := &yamlConverter{done: map[*yaml.Node]any{}, active: map[*yaml.Node]bool{}}
	root := doc
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return js.Null, nil
		}
		root = doc.Content[0]
	}
	v, err := c.convert(root)
	if err != nil {
		return nil, err
	}
	return restore(v), nil
}

// decodeYAML reads the single document of a text. found is false when the
// text has no document.
func decodeYAML(text string) (doc *yaml.Node, found bool, err error) {
	dec := yaml.NewDecoder(strings.NewReader(text))
	doc = &yaml.Node{}
	if err := dec.Decode(doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var next yaml.Node
	if err := dec.Decode(&next); err == nil {
		return nil, false, errors.New("expected a single document in the stream, but found more")
	} else if !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	return doc, true, nil
}

const (
	lineBreaks11      = "\u0085\u2028\u2029"
	lineBreakStandIns = "\ue085\ue028\ue029"
)

var (
	toStandIns   = strings.NewReplacer("\u0085", "\ue085", "\u2028", "\ue028", "\u2029", "\ue029")
	fromStandIns = strings.NewReplacer("\ue085", "\u0085", "\ue028", "\u2028", "\ue029", "\u2029")
)

// restoreLineBreaks puts U+0085, U+2028 and U+2029 back into every string
// and key of a value.
func restoreLineBreaks(v any) any {
	switch x := v.(type) {
	case string:
		return fromStandIns.Replace(x)
	case []any:
		for i := range x {
			x[i] = restoreLineBreaks(x[i])
		}
	case *js.Obj[any]:
		keys := x.Keys()
		values := make([]any, len(keys))
		for i, k := range keys {
			values[i] = restoreLineBreaks(x.Get(k))
			x.Delete(k)
		}
		for i, k := range keys {
			x.Set(fromStandIns.Replace(k), values[i])
		}
	}
	return v
}

// unescapeSlashes rewrites the escape \/ of a double-quoted scalar to a plain
// slash. js-yaml knows the escape and yaml.v3 does not. A double-quoted
// scalar is taken to start at a quote that follows the start of a line, a
// space, or one of the indicators "[", "{", "," and ":".
func unescapeSlashes(text string) string {
	var sb strings.Builder
	inDouble := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case inDouble && c == '\\' && i+1 < len(text):
			if text[i+1] == '/' {
				sb.WriteByte('/')
			} else {
				sb.WriteByte(c)
				sb.WriteByte(text[i+1])
			}
			i++
			continue
		case inDouble && c == '"':
			inDouble = false
		case !inDouble && c == '"':
			if i == 0 || strings.IndexByte(" \t\n[{,:", text[i-1]) >= 0 {
				inDouble = true
			}
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

// mappingEntryRe finds the ":" that makes a line a block mapping entry.
var mappingEntryRe = regexp.MustCompile(`:([ \t]|$)`)

// startsBlockCollection reports whether the text after the "-" of a sequence
// entry starts a nested block sequence or a block mapping.
func startsBlockCollection(rest string) bool {
	if rest == "-" || strings.HasPrefix(rest, "- ") || strings.HasPrefix(rest, "-\t") {
		return true
	}
	// A flow collection is a value here, whatever it holds.
	if rest != "" && (rest[0] == '{' || rest[0] == '[') {
		return false
	}
	// Skip a quoted scalar: only a ":" after it makes it a mapping key.
	if rest != "" && (rest[0] == '"' || rest[0] == '\'') {
		quote := rest[0]
		for i := 1; i < len(rest); i++ {
			switch {
			case quote == '"' && rest[i] == '\\':
				i++
			case rest[i] == quote && quote == '\'' && i+1 < len(rest) && rest[i+1] == '\'':
				i++
			case rest[i] == quote:
				return strings.HasPrefix(strings.TrimLeft(rest[i+1:], " \t"), ":")
			}
		}
		return false
	}
	return mappingEntryRe.MatchString(rest)
}

// repairTabs rewrites the uses of the tab that js-yaml accepts and yaml.v3
// rejects. A line that starts with a tab and holds only white space or a
// comment becomes an empty line: js-yaml skips such a line between the
// entries of a block collection. A tab after the "-" of a sequence entry
// becomes a space, unless a block collection follows: js-yaml does not start
// one after a tab.
func repairTabs(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "\t") {
			if rest := strings.TrimLeft(line, " \t"); rest == "" || rest[0] == '#' {
				lines[i] = ""
				continue
			}
		}
		b := []byte(line)
		j := 0
		for j < len(b) && b[j] == ' ' {
			j++
		}
		tabs := false
		for j+1 < len(b) && b[j] == '-' && (b[j+1] == ' ' || b[j+1] == '\t') && !tabs {
			j++
			for j < len(b) && (b[j] == ' ' || b[j] == '\t') {
				tabs = tabs || b[j] == '\t'
				b[j] = ' '
				j++
			}
		}
		if tabs && !startsBlockCollection(string(b[j:])) {
			lines[i] = string(b)
		}
	}
	return strings.Join(lines, "\n")
}

type yamlConverter struct {
	// done maps an anchored node to its value, so that an alias yields the
	// same object.
	done map[*yaml.Node]any
	// active holds the collections that are being built, to find an alias
	// that points at one of its own ancestors.
	active map[*yaml.Node]bool
}

func (c *yamlConverter) convert(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.AliasNode:
		if n.Alias == nil {
			return nil, yamlException("unidentified alias %q", n.Value)
		}
		if c.active[n.Alias] {
			// js-yaml builds a cyclic object here. The dynamic value model
			// has no cycles.
			return nil, yamlException("recursive alias %q is not supported", n.Value)
		}
		if v, ok := c.done[n.Alias]; ok {
			return v, nil
		}
		return c.convert(n.Alias)
	case yaml.ScalarNode:
		v, err := resolveScalar(n)
		if err == nil && n.Anchor != "" {
			c.done[n] = v
		}
		return v, err
	case yaml.SequenceNode:
		if err := checkCollectionTag(n, "!!seq", "sequence"); err != nil {
			return nil, err
		}
		c.active[n] = true
		defer delete(c.active, n)
		items := n.Content
		if n.Style&yaml.FlowStyle == 0 {
			items = swallowAfterEmptyEntry(items)
		}
		arr := make([]any, 0, len(items))
		for _, item := range items {
			v, err := c.convert(item)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		c.done[n] = arr
		return arr, nil
	case yaml.MappingNode:
		if err := checkCollectionTag(n, "!!map", "mapping"); err != nil {
			return nil, err
		}
		c.active[n] = true
		defer delete(c.active, n)
		obj := js.NewObj[any]()
		for i := 0; i+1 < len(n.Content); i += 2 {
			keyValue, err := c.convert(n.Content[i])
			if err != nil {
				return nil, err
			}
			key, err := mappingKey(keyValue)
			if err != nil {
				return nil, err
			}
			value, err := c.convert(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			// storeMappingPair
			if obj.Has(key) {
				return nil, yamlException("duplicated mapping key")
			}
			obj.Set(key, value)
		}
		c.done[n] = obj
		return obj, nil
	}
	return js.Null, nil
}

// isEmptyEntry reports whether a node is an entry with no content.
func isEmptyEntry(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Style == 0 && n.Value == "" && n.Anchor == ""
}

// swallowAfterEmptyEntry reproduces how readBlockSequence of js-yaml reads a
// block sequence that is nested in an entry of a block sequence and ends
// with an empty entry:
//
//   - -
//   - x
//
// After the empty entry js-yaml stays in the inner sequence and takes the
// next "-" as its entry, without a look at the indentation. So the value is
// [[null, "x"]] and not [[null], "x"]. The same happens again when the entry
// that it took ends with an empty entry.
func swallowAfterEmptyEntry(items []*yaml.Node) []*yaml.Node {
	out := make([]*yaml.Node, 0, len(items))
	for i := 0; i < len(items); i++ {
		item := items[i]
		for i+1 < len(items) {
			grown, ok := appendToOpenSequence(item, items[i+1])
			if !ok {
				break
			}
			item = grown
			i++
		}
		out = append(out, item)
	}
	return out
}

// appendToOpenSequence follows the last entries and values down from n. When
// that path ends with an empty entry of a block sequence, it returns a copy
// of n in which that sequence has one more entry.
func appendToOpenSequence(n, entry *yaml.Node) (*yaml.Node, bool) {
	if n.Style&yaml.FlowStyle != 0 || len(n.Content) == 0 || n.Anchor != "" {
		return nil, false
	}
	if n.Kind != yaml.SequenceNode && n.Kind != yaml.MappingNode {
		return nil, false
	}
	last := len(n.Content) - 1
	copied := *n
	copied.Content = append([]*yaml.Node{}, n.Content...)
	if n.Kind == yaml.SequenceNode && isEmptyEntry(n.Content[last]) {
		copied.Content = append(copied.Content, entry)
		return &copied, true
	}
	grown, ok := appendToOpenSequence(n.Content[last], entry)
	if !ok {
		return nil, false
	}
	copied.Content[last] = grown
	return &copied, true
}

// checkCollectionTag rejects an explicit tag that the JSON schema does not
// define for this kind of node.
func checkCollectionTag(n *yaml.Node, want, kind string) error {
	if n.Style&yaml.TaggedStyle == 0 || n.Tag == want {
		return nil
	}
	switch n.Tag {
	case "!!str", "!!null", "!!bool", "!!int", "!!float", "!!seq", "!!map":
		return yamlException("unacceptable node kind for !<%s> tag; it should not be %q", longTag(n.Tag), kind)
	}
	return yamlException("unknown tag !<%s>", longTag(n.Tag))
}

func longTag(tag string) string {
	if strings.HasPrefix(tag, "!!") {
		return "tag:yaml.org,2002:" + tag[2:]
	}
	return tag
}

// mappingKey is the key conversion of storeMappingPair: String(keyNode), with
// an object printed as "[object Object]".
func mappingKey(key any) (string, error) {
	if arr, ok := key.([]any); ok {
		parts := make([]string, len(arr))
		for i, e := range arr {
			if js.IsArray(e) {
				return "", yamlException("nested arrays are not supported inside keys")
			}
			if !js.IsNullish(e) {
				parts[i] = js.ToString(e)
			}
		}
		return strings.Join(parts, ","), nil
	}
	return js.ToString(key), nil
}

// resolveScalar is the scalar part of composeNode.
func resolveScalar(n *yaml.Node) (any, error) {
	plain := n.Style&^yaml.TaggedStyle == 0
	if n.Style&yaml.TaggedStyle != 0 {
		// An explicit tag. An empty plain node has the result null.
		empty := plain && n.Value == ""
		fail := func() (any, error) {
			return nil, yamlException("cannot resolve a node with !<%s> explicit tag", longTag(n.Tag))
		}
		switch n.Tag {
		case "!!str":
			return n.Value, nil
		case "!!null":
			if empty || resolveYamlNull(n.Value) {
				return js.Null, nil
			}
			return fail()
		case "!!bool":
			if !empty && resolveYamlBoolean(n.Value) {
				return constructYamlBoolean(n.Value), nil
			}
			return fail()
		case "!!int":
			if !empty && resolveYamlInteger(n.Value) {
				return parseYamlInteger(n.Value), nil
			}
			return fail()
		case "!!float":
			if !empty && resolveYamlFloat(n.Value) {
				return constructYamlFloat(n.Value), nil
			}
			return fail()
		case "!!map":
			if empty {
				return js.NewObj[any](), nil
			}
			return nil, yamlException(`unacceptable node kind for !<%s> tag; it should be "mapping", not "scalar"`, longTag(n.Tag))
		case "!!seq":
			if empty {
				return []any{}, nil
			}
			return nil, yamlException(`unacceptable node kind for !<%s> tag; it should be "sequence", not "scalar"`, longTag(n.Tag))
		}
		return nil, yamlException("unknown tag !<%s>", longTag(n.Tag))
	}
	if !plain {
		return n.Value, nil
	}
	if n.Value == "" {
		// A node with no content.
		return js.Null, nil
	}
	// The implicit types of the JSON schema, in order.
	switch {
	case resolveYamlNull(n.Value):
		return js.Null, nil
	case resolveYamlBoolean(n.Value):
		return constructYamlBoolean(n.Value), nil
	case resolveYamlInteger(n.Value):
		return parseYamlInteger(n.Value), nil
	case resolveYamlFloat(n.Value):
		return constructYamlFloat(n.Value), nil
	}
	return n.Value, nil
}

func resolveYamlNull(data string) bool {
	return data == "~" || data == "null" || data == "Null" || data == "NULL"
}

func resolveYamlBoolean(data string) bool {
	switch data {
	case "true", "True", "TRUE", "false", "False", "FALSE":
		return true
	}
	return false
}

func constructYamlBoolean(data string) bool {
	return data == "true" || data == "True" || data == "TRUE"
}

func isHexCode(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

func isOctCode(c byte) bool { return c >= '0' && c <= '7' }

func isDecCode(c byte) bool { return c >= '0' && c <= '9' }

func isFinite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func resolveYamlInteger(data string) bool {
	max := len(data)
	index := 0
	hasDigits := false

	if max == 0 {
		return false
	}

	// at reads data[index], which is undefined past the end.
	at := func(i int) byte {
		if i < max {
			return data[i]
		}
		return 0
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
			return hasDigits && isFinite(parseYamlInteger(data))
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
			return hasDigits && isFinite(parseYamlInteger(data))
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
			return hasDigits && isFinite(parseYamlInteger(data))
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

	return isFinite(parseYamlInteger(data))
}

// parseIntDigits is parseInt(digits, radix) for a string that holds only
// digits of the radix. V8 rounds the mathematical value to the nearest
// number.
func parseIntDigits(digits string, radix int) float64 {
	if digits == "" {
		return math.NaN()
	}
	if radix == 10 {
		f, _ := strconv.ParseFloat(digits, 64)
		return f
	}
	n, ok := new(big.Int).SetString(digits, radix)
	if !ok {
		return math.NaN()
	}
	f, _ := new(big.Float).SetInt(n).Float64()
	return f
}

func parseYamlInteger(data string) float64 {
	value := data
	sign := 1.0
	var ch byte
	if value != "" {
		ch = value[0]
	}

	if ch == '-' || ch == '+' {
		if ch == '-' {
			sign = -1
		}
		value = value[1:]
		ch = 0
		if value != "" {
			ch = value[0]
		}
	}

	if value == "0" {
		return 0
	}

	if ch == '0' && len(value) > 1 {
		switch value[1] {
		case 'b':
			return float64(sign * parseIntDigits(value[2:], 2))
		case 'x':
			return float64(sign * parseIntDigits(value[2:], 16))
		case 'o':
			return float64(sign * parseIntDigits(value[2:], 8))
		}
	}

	return float64(sign * parseIntDigits(value, 10))
}

var (
	// 2.5e4, 2.5 and integers
	yamlFloatPattern = regexp.MustCompile(
		`^(?:[-+]?(?:[0-9]+)(?:\.[0-9]*)?(?:[eE][-+]?[0-9]+)?|\.[0-9]+(?:[eE][-+]?[0-9]+)?|[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
	yamlFloatSpecialPattern = regexp.MustCompile(`^(?:[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
)

func resolveYamlFloat(data string) bool {
	if !yamlFloatPattern.MatchString(data) {
		return false
	}

	if isFinite(js.ParseFloat(data)) {
		return true
	}

	return yamlFloatSpecialPattern.MatchString(data)
}

func constructYamlFloat(data string) float64 {
	value := strings.ToLower(data)
	sign := 1.0
	if value[0] == '-' {
		sign = -1
	}

	if value[0] == '+' || value[0] == '-' {
		value = value[1:]
	}

	if value == ".inf" {
		return math.Inf(int(sign))
	} else if value == ".nan" {
		return math.NaN()
	}
	return float64(sign * js.ParseFloat(value))
}
