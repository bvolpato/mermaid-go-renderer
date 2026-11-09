// Ported from js-yaml/lib/loader.js (js-yaml 4.3.0).

package jsyaml

import (
	"strconv"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

const (
	contextFlowIn   = 1
	contextFlowOut  = 2
	contextBlockIn  = 3
	contextBlockOut = 4

	chompingClip  = 1
	chompingStrip = 2
	chompingKeep  = 3
)

// null is the value of a nullable string of the loader state that is null:
// the tag, the anchor, the kind and the version. The input has no NUL
// character, so no tag and no anchor has this value.
const null = "\x00"

// maxDepth is the default of the option maxDepth.
const maxDepth = 100

// seq is an array of the document while the loader builds it. An anchor
// can name the array before its entries are read, so the loader needs a
// reference. export changes it to a []any.
type seq struct {
	items []any
}

// The values of the loader are: nil (the null of JavaScript), bool,
// float64, string, *seq and *js.Obj[any].

func isEol(c int) bool {
	return c == 0x0A /* LF */ || c == 0x0D /* CR */
}

func isWhiteSpace(c int) bool {
	return c == 0x09 /* Tab */ || c == 0x20 /* Space */
}

func isWsOrEol(c int) bool {
	return c == 0x09 || c == 0x20 || c == 0x0A || c == 0x0D
}

func isFlowIndicator(c int) bool {
	return c == 0x2C /* , */ ||
		c == 0x5B /* [ */ ||
		c == 0x5D /* ] */ ||
		c == 0x7B /* { */ ||
		c == 0x7D /* } */
}

func fromHexCode(c int) int {
	if c >= 0x30 /* 0 */ && c <= 0x39 /* 9 */ {
		return c - 0x30
	}

	lc := c | 0x20

	if lc >= 0x61 /* a */ && lc <= 0x66 /* f */ {
		return lc - 0x61 + 10
	}

	return -1
}

func escapedHexLen(c int) int {
	if c == 0x78 /* x */ {
		return 2
	}
	if c == 0x75 /* u */ {
		return 4
	}
	if c == 0x55 /* U */ {
		return 8
	}
	return 0
}

func fromDecimalCode(c int) int {
	if c >= 0x30 /* 0 */ && c <= 0x39 /* 9 */ {
		return c - 0x30
	}

	return -1
}

// simpleEscapeSequence is simpleEscapeSequence(c). The second result is
// false for a character with no escape sequence.
func simpleEscapeSequence(c int) (rune, bool) {
	switch c {
	case 0x30: /* 0 */
		return 0x00, true
	case 0x61: /* a */
		return 0x07, true
	case 0x62: /* b */
		return 0x08, true
	case 0x74: /* t */
		return 0x09, true
	case 0x09: /* Tab */
		return 0x09, true
	case 0x6E: /* n */
		return 0x0A, true
	case 0x76: /* v */
		return 0x0B, true
	case 0x66: /* f */
		return 0x0C, true
	case 0x72: /* r */
		return 0x0D, true
	case 0x65: /* e */
		return 0x1B, true
	case 0x20: /* Space */
		return ' ', true
	case 0x22: /* " */
		return 0x22, true
	case 0x2F: /* / */
		return '/', true
	case 0x5C: /* \ */
		return 0x5C, true
	case 0x4E: /* N */
		return 0x85, true
	case 0x5F: /* _ */
		return 0xA0, true
	case 0x4C: /* L */
		return 0x2028, true
	case 0x50: /* P */
		return 0x2029, true
	}
	return 0, false
}

// appendCodepoint is result += charFromCodepoint(c): the code units of a
// code point.
func appendCodepoint(units []rune, c int) []rune {
	if c <= 0xFFFF {
		return append(units, rune(c))
	}
	// Encode UTF-16 surrogate pair
	// https://en.wikipedia.org/wiki/UTF-16#Code_points_U.2B010000_to_U.2B10FFFF
	return append(units,
		rune(((c-0x010000)>>10)+0xD800),
		rune(((c-0x010000)&0x03FF)+0xDC00),
	)
}

// setProperty is setProperty(object, key, value). An object of this port
// has no prototype, so the key __proto__ is a key like the others.
func setProperty(object *js.Obj[any], key string, value any) {
	object.Set(key, value)
}

// anchorEntry is an entry of an anchor transaction: the value of an anchor
// before the transaction changed it.
type anchorEntry struct {
	existed bool
	value   any
}

// state is State: the state of the loader.
type state struct {
	// input is the code units of the input, with a NUL at the end.
	input []rune
	// length is the length of the input without the NUL.
	length int

	position       int
	line           int
	lineStart      int
	lineIndent     int
	depth          int
	firstTabInLine int

	documents             []any
	anchorMapTransactions []map[string]anchorEntry

	version         string
	checkLineBreaks bool
	tagMap          map[string]string
	anchorMap       map[string]any

	tag    string
	anchor string
	kind   string
	// result is a value of the document, or the units of a scalar that the
	// loader reads (resultUnits) while isUnits is true.
	result any
}

// at is state.input.charCodeAt(i). The result is -1 for an index that is
// out of range, where JavaScript gives NaN: no comparison is true.
func (s *state) at(i int) int {
	if i < 0 || i >= len(s.input) {
		return -1
	}
	return int(s.input[i])
}

// slice is state.input.slice(start, end).
func (s *state) slice(start, end int) []rune {
	n := len(s.input)
	start = min(max(start, 0), n)
	end = min(max(end, 0), n)
	if start >= end {
		return nil
	}
	return s.input[start:end]
}

func (s *state) generateError(message string) *Exception {
	// omit trailing \0
	buffer := s.input
	if len(buffer) > 0 {
		buffer = buffer[:len(buffer)-1]
	}
	mark := &Mark{
		Buffer:   js.FromUTF16Units(buffer),
		Position: s.position,
		Line:     s.line,
		Column:   s.position - s.lineStart,
	}

	mark.Snippet = makeSnippet(buffer, mark.Position, mark.Line)

	return newException(message, mark)
}

// throwError is throwError(state, message). The exception is a panic that
// Load recovers.
func (s *state) throwError(message string) {
	panic(s.generateError(message))
}

// throwWarning is throwWarning(state, message). The loader has no warning
// handler, so it does nothing.
func (s *state) throwWarning(string) {}

func (s *state) storeAnchor(name string, value any) {
	transactions := s.anchorMapTransactions

	if len(transactions) != 0 {
		transaction := transactions[len(transactions)-1]

		if _, ok := transaction[name]; !ok {
			existing, existed := s.anchorMap[name]
			transaction[name] = anchorEntry{existed: existed, value: existing}
		}
	}

	s.anchorMap[name] = value
}

func (s *state) beginAnchorTransaction() {
	s.anchorMapTransactions = append(s.anchorMapTransactions, map[string]anchorEntry{})
}

func (s *state) commitAnchorTransaction() {
	n := len(s.anchorMapTransactions)
	transaction := s.anchorMapTransactions[n-1]
	s.anchorMapTransactions = s.anchorMapTransactions[:n-1]
	transactions := s.anchorMapTransactions

	if len(transactions) == 0 {
		return
	}

	parent := transactions[len(transactions)-1]

	for name, entry := range transaction {
		if _, ok := parent[name]; !ok {
			parent[name] = entry
		}
	}
}

func (s *state) rollbackAnchorTransaction() {
	n := len(s.anchorMapTransactions)
	transaction := s.anchorMapTransactions[n-1]
	s.anchorMapTransactions = s.anchorMapTransactions[:n-1]

	// Each entry restores its own name, so the order has no effect.
	for name, entry := range transaction {
		if entry.existed {
			s.anchorMap[name] = entry.value
		} else {
			delete(s.anchorMap, name)
		}
	}
}

// snapshot is the result of snapshotState.
type snapshot struct {
	position       int
	line           int
	lineStart      int
	lineIndent     int
	firstTabInLine int
	tag            string
	anchor         string
	kind           string
	result         any
}

func (s *state) snapshotState() *snapshot {
	return &snapshot{
		position:       s.position,
		line:           s.line,
		lineStart:      s.lineStart,
		lineIndent:     s.lineIndent,
		firstTabInLine: s.firstTabInLine,
		tag:            s.tag,
		anchor:         s.anchor,
		kind:           s.kind,
		result:         s.result,
	}
}

func (s *state) restoreState(snap *snapshot) {
	s.position = snap.position
	s.line = snap.line
	s.lineStart = snap.lineStart
	s.lineIndent = snap.lineIndent
	s.firstTabInLine = snap.firstTabInLine
	s.tag = snap.tag
	s.anchor = snap.anchor
	s.kind = snap.kind
	s.result = snap.result
}

// isTagHandle is PATTERN_TAG_HANDLE.test(s): /^(?:!|!!|![0-9A-Za-z-]+!)$/.
func isTagHandle(units []rune) bool {
	n := len(units)
	if n == 0 || units[0] != '!' {
		return false
	}
	if n == 1 {
		return true
	}
	if units[n-1] != '!' {
		return false
	}
	for _, u := range units[1 : n-1] {
		if !(u >= '0' && u <= '9' || u >= 'A' && u <= 'Z' || u >= 'a' && u <= 'z' || u == '-') {
			return false
		}
	}
	return true
}

// isURIChar says whether a code unit is in the class
// [0-9a-z\-#;/?:@&=+$,_.!~*'()\[\]] with the flag i. Without the flag u, a
// character that is not ASCII does not match an ASCII letter.
func isURIChar(u rune) bool {
	switch {
	case u >= '0' && u <= '9', u >= 'a' && u <= 'z', u >= 'A' && u <= 'Z':
		return true
	}
	switch u {
	case '-', '#', ';', '/', '?', ':', '@', '&', '=', '+', '$', ',', '_', '.', '!', '~', '*', '\'', '(', ')', '[', ']':
		return true
	}
	return false
}

func isHexUnit(u rune) bool {
	return u >= '0' && u <= '9' || u >= 'a' && u <= 'f' || u >= 'A' && u <= 'F'
}

// isTagURI is PATTERN_TAG_URI.test(s):
// /^(?:!|[^,\[\]{}])(?:%[0-9a-f]{2}|[0-9a-z\-#;/?:@&=+$,_.!~*'()\[\]])*$/i.
func isTagURI(units []rune) bool {
	if len(units) == 0 {
		return false
	}
	// The first alternative is one code unit that is not a flow indicator.
	// The dot of the class does not exclude a line break.
	if isFlowIndicator(int(units[0])) {
		return false
	}
	for i := 1; i < len(units); {
		u := units[i]
		if u == '%' && i+2 < len(units) && isHexUnit(units[i+1]) && isHexUnit(units[i+2]) {
			i += 3
			continue
		}
		if !isURIChar(u) {
			return false
		}
		i++
	}
	return true
}

// hasFlowIndicator is PATTERN_FLOW_INDICATORS.test(s): /[,\[\]{}]/.
func hasFlowIndicator(units []rune) bool {
	for _, u := range units {
		if isFlowIndicator(int(u)) {
			return true
		}
	}
	return false
}

// decodeURIComponent is decodeURIComponent(s). The second result is false
// when JavaScript throws a URIError.
func decodeURIComponent(units []rune) ([]rune, bool) {
	hasPercent := false
	for _, u := range units {
		if u == '%' {
			hasPercent = true
			break
		}
	}
	if !hasPercent {
		return units, true
	}
	hexByte := func(i int) (byte, bool) {
		if i+2 >= len(units) || units[i] != '%' || !isHexUnit(units[i+1]) || !isHexUnit(units[i+2]) {
			return 0, false
		}
		return byte(fromHexCode(int(units[i+1]))<<4 | fromHexCode(int(units[i+2]))), true
	}
	out := make([]rune, 0, len(units))
	for i := 0; i < len(units); {
		if units[i] != '%' {
			out = append(out, units[i])
			i++
			continue
		}
		b, ok := hexByte(i)
		if !ok {
			return nil, false
		}
		i += 3
		if b < 0x80 {
			out = append(out, rune(b))
			continue
		}
		n := 0
		switch {
		case b&0xE0 == 0xC0:
			n = 2
		case b&0xF0 == 0xE0:
			n = 3
		case b&0xF8 == 0xF0:
			n = 4
		default:
			return nil, false
		}
		octets := []byte{b}
		for k := 1; k < n; k++ {
			c, ok := hexByte(i)
			if !ok || c&0xC0 != 0x80 {
				return nil, false
			}
			octets = append(octets, c)
			i += 3
		}
		r, size := utf8.DecodeRune(octets)
		if r == utf8.RuneError || size != n {
			return nil, false
		}
		out = appendCodepoint(out, int(r))
	}
	return out, true
}

// handleYamlDirective is directiveHandlers.YAML.
func (s *state) handleYamlDirective(args [][]rune) {
	if s.version != null {
		s.throwError("duplication of %YAML directive")
	}

	if len(args) != 1 {
		s.throwError("YAML directive accepts exactly one argument")
	}

	// match = /^([0-9]+)\.([0-9]+)$/.exec(args[0])
	arg := args[0]
	dot := -1
	valid := len(arg) > 0
	for i, u := range arg {
		switch {
		case u >= '0' && u <= '9':
		case u == '.' && dot < 0 && i > 0:
			dot = i
		default:
			valid = false
		}
	}
	if !valid || dot < 0 || dot == len(arg)-1 {
		s.throwError("ill-formed argument of the YAML directive")
	}

	major := js.ParseInt(js.FromUTF16Units(arg[:dot]), 10)
	minor := js.ParseInt(js.FromUTF16Units(arg[dot+1:]), 10)

	if major != 1 {
		s.throwError("unacceptable YAML version of the document")
	}

	s.version = js.FromUTF16Units(arg)
	s.checkLineBreaks = minor < 2

	if minor != 1 && minor != 2 {
		s.throwWarning("unsupported YAML version of the document")
	}
}

// handleTagDirective is directiveHandlers.TAG.
func (s *state) handleTagDirective(args [][]rune) {
	if len(args) != 2 {
		s.throwError("TAG directive accepts exactly two arguments")
	}

	handle := args[0]
	prefix := args[1]

	if !isTagHandle(handle) {
		s.throwError("ill-formed tag handle (first argument) of the TAG directive")
	}

	if _, ok := s.tagMap[js.FromUTF16Units(handle)]; ok {
		s.throwError(`there is a previously declared suffix for "` + js.FromUTF16Units(handle) + `" tag handle`)
	}

	if !isTagURI(prefix) {
		s.throwError("ill-formed tag prefix (second argument) of the TAG directive")
	}

	decoded, ok := decodeURIComponent(prefix)
	if !ok {
		s.throwError("tag prefix is malformed: " + js.FromUTF16Units(prefix))
	}

	s.tagMap[js.FromUTF16Units(handle)] = js.FromUTF16Units(decoded)
}

// isNonPrintable is PATTERN_NON_PRINTABLE.test(segment). The pattern matches
// a control character (U+0000 to U+0008, U+000B, U+000C, U+000E to U+001F,
// U+007F to U+0084, U+0086 to U+009F), U+FFFE, U+FFFF, a high surrogate
// that no low surrogate follows, and a low surrogate that no high surrogate
// precedes.
func isNonPrintable(units []rune) bool {
	for i, u := range units {
		switch {
		case u <= 0x08, u == 0x0B, u == 0x0C, u >= 0x0E && u <= 0x1F, u >= 0x7F && u <= 0x84, u >= 0x86 && u <= 0x9F, u == 0xFFFE, u == 0xFFFF:
			return true
		case u >= 0xD800 && u <= 0xDBFF:
			if i+1 >= len(units) || units[i+1] < 0xDC00 || units[i+1] > 0xDFFF {
				return true
			}
		case u >= 0xDC00 && u <= 0xDFFF:
			if i == 0 || units[i-1] < 0xD800 || units[i-1] > 0xDBFF {
				return true
			}
		}
	}
	return false
}

// units returns the code units of the scalar that the loader reads.
func (s *state) units() []rune {
	u, _ := s.result.([]rune)
	return u
}

// captureSegment is captureSegment(state, start, end, checkJson). The
// result of the state is the code units of a scalar here.
func (s *state) captureSegment(start, end int, checkJSON bool) {
	if start < end {
		result := s.slice(start, end)

		if checkJSON {
			for _, character := range result {
				if !(character == 0x09 || character >= 0x20 && character <= 0x10FFFF) {
					s.throwError("expected valid JSON character")
				}
			}
		} else if isNonPrintable(result) {
			s.throwError("the stream contains non-printable characters")
		}

		s.result = append(s.units(), result...)
	}
}

// keyString is the conversion of a key node to a string in
// storeMappingPair.
func (s *state) keyString(keyNode any) string {
	// The single key (i.e. a non-sequence) can be an object too.
	if list, ok := keyNode.(*seq); ok {
		// keyNode = Array.prototype.slice.call(keyNode)
		parts := make([]any, len(list.items))
		for index, item := range list.items {
			if _, isArray := item.(*seq); isArray {
				s.throwError("nested arrays are not supported inside keys")
			}

			if _, isObject := item.(*js.Obj[any]); isObject {
				item = "[object Object]"
			}
			parts[index] = item
		}
		return js.ToString(nullsOf(parts))
	}

	// Avoid code execution in load() via toString property
	// (still use its own toString for arrays, timestamps,
	// and whatever user schema extensions happen to have @@toStringTag)
	if _, isObject := keyNode.(*js.Obj[any]); isObject {
		return "[object Object]"
	}

	return js.ToString(nullOf(keyNode))
}

// nullOf converts a value of the loader to a value of internal/js: the nil
// of the loader is the null of JavaScript.
func nullOf(v any) any {
	if v == nil {
		return js.Null
	}
	return v
}

func nullsOf(list []any) []any {
	out := make([]any, len(list))
	for i, v := range list {
		out[i] = nullOf(v)
	}
	return out
}

// storeMappingPair is storeMappingPair(state, _result, overridableKeys,
// keyTag, keyNode, valueNode, startLine, startLineStart, startPos). The
// JSON schema has no merge type, so the branch for the merge key is not
// ported. overridableKeys stays empty without it.
func (s *state) storeMappingPair(result *js.Obj[any], keyNode, valueNode any, startLine, startLineStart, startPos int) *js.Obj[any] {
	key := s.keyString(keyNode)

	if result == nil {
		result = js.NewObj[any]()
	}

	if result.Has(key) {
		// state.line = startLine || state.line, and so on: a start value of
		// 0 keeps the current value.
		if startLine != 0 {
			s.line = startLine
		}
		if startLineStart != 0 {
			s.lineStart = startLineStart
		}
		if startPos != 0 {
			s.position = startPos
		}
		s.throwError("duplicated mapping key")
	}

	setProperty(result, key, valueNode)

	return result
}

func (s *state) readLineBreak() {
	ch := s.at(s.position)

	if ch == 0x0A /* LF */ {
		s.position++
	} else if ch == 0x0D /* CR */ {
		s.position++
		if s.at(s.position) == 0x0A /* LF */ {
			s.position++
		}
	} else {
		s.throwError("a line break is expected")
	}

	s.line += 1
	s.lineStart = s.position
	s.firstTabInLine = -1
}

func (s *state) skipSeparationSpace(allowComments bool, checkIndent int) int {
	lineBreaks := 0
	ch := s.at(s.position)

	for ch != 0 {
		for isWhiteSpace(ch) {
			if ch == 0x09 /* Tab */ && s.firstTabInLine == -1 {
				s.firstTabInLine = s.position
			}
			s.position++
			ch = s.at(s.position)
		}

		if allowComments && ch == 0x23 /* # */ {
			for {
				s.position++
				ch = s.at(s.position)
				if !(ch != 0x0A /* LF */ && ch != 0x0D /* CR */ && ch != 0) {
					break
				}
			}
		}

		if isEol(ch) {
			s.readLineBreak()

			ch = s.at(s.position)
			lineBreaks++
			s.lineIndent = 0

			for ch == 0x20 /* Space */ {
				s.lineIndent++
				s.position++
				ch = s.at(s.position)
			}
		} else {
			break
		}
	}

	if checkIndent != -1 && lineBreaks != 0 && s.lineIndent < checkIndent {
		s.throwWarning("deficient indentation")
	}

	return lineBreaks
}

func (s *state) testDocumentSeparator() bool {
	position := s.position

	ch := s.at(position)

	// Condition state.position === state.lineStart is tested
	// in parent on each call, for efficiency. No needs to test here again.
	if (ch == 0x2D /* - */ || ch == 0x2E /* . */) &&
		ch == s.at(position+1) &&
		ch == s.at(position+2) {
		position += 3

		ch = s.at(position)

		if ch == 0 || isWsOrEol(ch) {
			return true
		}
	}

	return false
}

func (s *state) writeFoldedLines(count int) {
	if count == 1 {
		s.result = append(s.units(), ' ')
	} else if count > 1 {
		u := s.units()
		for i := 0; i < count-1; i++ {
			u = append(u, '\n')
		}
		s.result = u
	}
}

// finishScalar changes the code units of a scalar to its string.
func (s *state) finishScalar() {
	s.result = js.FromUTF16Units(s.units())
}

func (s *state) readPlainScalar(nodeIndent int, withinFlowCollection bool) bool {
	var (
		captureStart, captureEnd int
		hasPendingContent        bool
		line, lineStart          int
		lineIndent               int
	)
	kind := s.kind
	result := s.result

	ch := s.at(s.position)

	if isWsOrEol(ch) ||
		isFlowIndicator(ch) ||
		ch == 0x23 /* # */ ||
		ch == 0x26 /* & */ ||
		ch == 0x2A /* * */ ||
		ch == 0x21 /* ! */ ||
		ch == 0x7C /* | */ ||
		ch == 0x3E /* > */ ||
		ch == 0x27 /* ' */ ||
		ch == 0x22 /* " */ ||
		ch == 0x25 /* % */ ||
		ch == 0x40 /* @ */ ||
		ch == 0x60 /* ` */ {
		return false
	}

	if ch == 0x3F /* ? */ || ch == 0x2D /* - */ {
		following := s.at(s.position + 1)

		if isWsOrEol(following) ||
			withinFlowCollection && isFlowIndicator(following) {
			return false
		}
	}

	s.kind = "scalar"
	s.result = []rune{}
	captureStart = s.position
	captureEnd = s.position
	hasPendingContent = false

	for ch != 0 {
		if ch == 0x3A /* : */ {
			following := s.at(s.position + 1)

			if isWsOrEol(following) ||
				withinFlowCollection && isFlowIndicator(following) {
				break
			}
		} else if ch == 0x23 /* # */ {
			preceding := s.at(s.position - 1)

			if isWsOrEol(preceding) {
				break
			}
		} else if (s.position == s.lineStart && s.testDocumentSeparator()) ||
			withinFlowCollection && isFlowIndicator(ch) {
			break
		} else if isEol(ch) {
			line = s.line
			lineStart = s.lineStart
			lineIndent = s.lineIndent
			s.skipSeparationSpace(false, -1)

			if s.lineIndent >= nodeIndent {
				hasPendingContent = true
				ch = s.at(s.position)
				continue
			} else {
				s.position = captureEnd
				s.line = line
				s.lineStart = lineStart
				s.lineIndent = lineIndent
				break
			}
		}

		if hasPendingContent {
			s.captureSegment(captureStart, captureEnd, false)
			s.writeFoldedLines(s.line - line)
			captureStart = s.position
			captureEnd = s.position
			hasPendingContent = false
		}

		if !isWhiteSpace(ch) {
			captureEnd = s.position + 1
		}

		s.position++
		ch = s.at(s.position)
	}

	s.captureSegment(captureStart, captureEnd, false)

	if len(s.units()) > 0 {
		s.finishScalar()
		return true
	}

	s.kind = kind
	s.result = result
	return false
}

func (s *state) readSingleQuotedScalar(nodeIndent int) bool {
	var captureStart, captureEnd int

	ch := s.at(s.position)

	if ch != 0x27 /* ' */ {
		return false
	}

	s.kind = "scalar"
	s.result = []rune{}
	s.position++
	captureStart = s.position
	captureEnd = s.position

	for {
		ch = s.at(s.position)
		if ch == 0 {
			break
		}
		if ch == 0x27 /* ' */ {
			s.captureSegment(captureStart, s.position, true)
			s.position++
			ch = s.at(s.position)

			if ch == 0x27 /* ' */ {
				captureStart = s.position
				s.position++
				captureEnd = s.position
			} else {
				s.finishScalar()
				return true
			}
		} else if isEol(ch) {
			s.captureSegment(captureStart, captureEnd, true)
			s.writeFoldedLines(s.skipSeparationSpace(false, nodeIndent))
			captureStart = s.position
			captureEnd = s.position
		} else if s.position == s.lineStart && s.testDocumentSeparator() {
			s.throwError("unexpected end of the document within a single quoted scalar")
		} else {
			s.position++
			if !isWhiteSpace(ch) {
				captureEnd = s.position
			}
		}
	}

	s.throwError("unexpected end of the stream within a single quoted scalar")
	return false
}

func (s *state) readDoubleQuotedScalar(nodeIndent int) bool {
	var (
		captureStart, captureEnd int
		tmp                      int
	)

	ch := s.at(s.position)

	if ch != 0x22 /* " */ {
		return false
	}

	s.kind = "scalar"
	s.result = []rune{}
	s.position++
	captureStart = s.position
	captureEnd = s.position

	for {
		ch = s.at(s.position)
		if ch == 0 {
			break
		}
		if ch == 0x22 /* " */ {
			s.captureSegment(captureStart, s.position, true)
			s.position++
			s.finishScalar()
			return true
		} else if ch == 0x5C /* \ */ {
			s.captureSegment(captureStart, s.position, true)
			s.position++
			ch = s.at(s.position)

			if isEol(ch) {
				s.skipSeparationSpace(false, nodeIndent)

				// TODO: rework to inline fn with no type cast?
			} else if sequence, ok := simpleEscapeSequence(ch); ch >= 0 && ch < 256 && ok {
				s.result = append(s.units(), sequence)
				s.position++
			} else if tmp = escapedHexLen(ch); tmp > 0 {
				hexLength := tmp
				hexResult := 0

				for ; hexLength > 0; hexLength-- {
					s.position++
					ch = s.at(s.position)

					if tmp = fromHexCode(ch); tmp >= 0 {
						// The shift of JavaScript works on 32 bits.
						hexResult = int(int32(uint32(hexResult)<<4)) + tmp
					} else {
						s.throwError("expected hexadecimal character")
					}
				}

				s.result = appendCodepointJS(s.units(), hexResult)

				s.position++
			} else {
				s.throwError("unknown escape sequence")
			}

			captureStart = s.position
			captureEnd = s.position
		} else if isEol(ch) {
			s.captureSegment(captureStart, captureEnd, true)
			s.writeFoldedLines(s.skipSeparationSpace(false, nodeIndent))
			captureStart = s.position
			captureEnd = s.position
		} else if s.position == s.lineStart && s.testDocumentSeparator() {
			s.throwError("unexpected end of the document within a double quoted scalar")
		} else {
			s.position++
			if !isWhiteSpace(ch) {
				captureEnd = s.position
			}
		}
	}

	s.throwError("unexpected end of the stream within a double quoted scalar")
	return false
}

// appendCodepointJS is result += charFromCodepoint(c) for the value of an
// escape sequence, which can be any 32-bit integer: String.fromCharCode
// keeps the low 16 bits of each argument.
func appendCodepointJS(units []rune, c int) []rune {
	if c <= 0xFFFF {
		return append(units, rune(uint16(c)))
	}
	return append(units,
		rune(uint16(((c-0x010000)>>10)+0xD800)),
		rune(uint16(((c-0x010000)&0x03FF)+0xDC00)),
	)
}

func (s *state) readFlowCollection(nodeIndent int) bool {
	var (
		readNext               = true
		line, lineStart, pos   int
		tag                    = s.tag
		anchor                 = s.anchor
		terminator             int
		isPair, isExplicitPair bool
		isMapping              bool
		keyNode, valueNode     any
		mapping                *js.Obj[any]
		sequence               *seq
		result                 any
	)

	ch := s.at(s.position)

	if ch == 0x5B /* [ */ {
		terminator = 0x5D /* ] */
		isMapping = false
		sequence = &seq{items: []any{}}
		result = sequence
	} else if ch == 0x7B /* { */ {
		terminator = 0x7D /* } */
		isMapping = true
		mapping = js.NewObj[any]()
		result = mapping
	} else {
		return false
	}

	if s.anchor != null {
		s.storeAnchor(s.anchor, result)
	}

	s.position++
	ch = s.at(s.position)

	for ch != 0 {
		s.skipSeparationSpace(true, nodeIndent)

		ch = s.at(s.position)

		if ch == terminator {
			s.position++
			s.tag = tag
			s.anchor = anchor
			if isMapping {
				s.kind = "mapping"
			} else {
				s.kind = "sequence"
			}
			s.result = result
			return true
		} else if !readNext {
			s.throwError("missed comma between flow collection entries")
		} else if ch == 0x2C /* , */ {
			// "flow collection entries can never be completely empty", as per YAML 1.2, section 7.4
			s.throwError("expected the node content, but found ','")
		}

		keyNode = nil
		valueNode = nil
		isPair = false
		isExplicitPair = false

		if ch == 0x3F /* ? */ {
			following := s.at(s.position + 1)

			if isWsOrEol(following) {
				isPair = true
				isExplicitPair = true
				s.position++
				s.skipSeparationSpace(true, nodeIndent)
			}
		}

		line = s.line // Save the current line.
		lineStart = s.lineStart
		pos = s.position
		s.composeNode(nodeIndent, contextFlowIn, false, true)
		keyNode = s.result
		s.skipSeparationSpace(true, nodeIndent)

		ch = s.at(s.position)

		if (isExplicitPair || s.line == line) && ch == 0x3A /* : */ {
			isPair = true
			s.position++
			s.skipSeparationSpace(true, nodeIndent)
			s.composeNode(nodeIndent, contextFlowIn, false, true)
			valueNode = s.result
		}

		if isMapping {
			s.storeMappingPair(mapping, keyNode, valueNode, line, lineStart, pos)
		} else if isPair {
			sequence.items = append(sequence.items, s.storeMappingPair(nil, keyNode, valueNode, line, lineStart, pos))
		} else {
			sequence.items = append(sequence.items, keyNode)
		}

		s.skipSeparationSpace(true, nodeIndent)

		ch = s.at(s.position)

		if ch == 0x2C /* , */ {
			readNext = true
			s.position++
			ch = s.at(s.position)
		} else {
			readNext = false
		}
	}

	s.throwError("unexpected end of the stream within a flow collection")
	return false
}

func (s *state) readBlockScalar(nodeIndent int) bool {
	var (
		folding        bool
		chomping       = chompingClip
		didReadContent = false
		detectedIndent = false
		textIndent     = nodeIndent
		emptyLines     = 0
		atMoreIndented = false
		tmp            int
	)

	ch := s.at(s.position)

	if ch == 0x7C /* | */ {
		folding = false
	} else if ch == 0x3E /* > */ {
		folding = true
	} else {
		return false
	}

	s.kind = "scalar"
	result := []rune{}
	s.result = result
	// appendBreaks is state.result += common.repeat('\n', count).
	appendBreaks := func(count int) {
		u := s.units()
		for i := 0; i < count; i++ {
			u = append(u, '\n')
		}
		s.result = u
	}

	for ch != 0 {
		s.position++
		ch = s.at(s.position)

		if ch == 0x2B /* + */ || ch == 0x2D /* - */ {
			if chompingClip == chomping {
				if ch == 0x2B /* + */ {
					chomping = chompingKeep
				} else {
					chomping = chompingStrip
				}
			} else {
				s.throwError("repeat of a chomping mode identifier")
			}
		} else if tmp = fromDecimalCode(ch); tmp >= 0 {
			if tmp == 0 {
				s.throwError("bad explicit indentation width of a block scalar; it cannot be less than one")
			} else if !detectedIndent {
				textIndent = nodeIndent + tmp - 1
				detectedIndent = true
			} else {
				s.throwError("repeat of an indentation width identifier")
			}
		} else {
			break
		}
	}

	if isWhiteSpace(ch) {
		for {
			s.position++
			ch = s.at(s.position)
			if !isWhiteSpace(ch) {
				break
			}
		}

		if ch == 0x23 /* # */ {
			for {
				s.position++
				ch = s.at(s.position)
				if !(!isEol(ch) && ch != 0) {
					break
				}
			}
		}
	}

	for ch != 0 {
		s.readLineBreak()
		s.lineIndent = 0

		ch = s.at(s.position)

		for (!detectedIndent || s.lineIndent < textIndent) &&
			ch == 0x20 /* Space */ {
			s.lineIndent++
			s.position++
			ch = s.at(s.position)
		}

		if !detectedIndent && s.lineIndent > textIndent {
			textIndent = s.lineIndent
		}

		if isEol(ch) {
			emptyLines++
			continue
		}

		if !detectedIndent && textIndent == 0 {
			s.throwError("missing indentation for block scalar")
		}

		// End of the scalar.
		if s.lineIndent < textIndent {
			// Perform the chomping.
			if chomping == chompingKeep {
				if didReadContent {
					appendBreaks(1 + emptyLines)
				} else {
					appendBreaks(emptyLines)
				}
			} else if chomping == chompingClip {
				if didReadContent { // i.e. only if the scalar is not empty.
					appendBreaks(1)
				}
			}

			// Break this `while` cycle and go to the funciton's epilogue.
			break
		}

		// Folded style: use fancy rules to handle line breaks.
		if folding {
			// Lines starting with white space characters (more-indented lines) are not folded.
			if isWhiteSpace(ch) {
				atMoreIndented = true
				// except for the first content line (cf. Example 8.1)
				if didReadContent {
					appendBreaks(1 + emptyLines)
				} else {
					appendBreaks(emptyLines)
				}

				// End of more-indented block.
			} else if atMoreIndented {
				atMoreIndented = false
				appendBreaks(emptyLines + 1)

				// Just one line break - perceive as the same line.
			} else if emptyLines == 0 {
				if didReadContent { // i.e. only if we have already read some scalar content.
					s.result = append(s.units(), ' ')
				}

				// Several line breaks - perceive as different lines.
			} else {
				appendBreaks(emptyLines)
			}

			// Literal style: just add exact number of line breaks between content lines.
		} else {
			// Keep all line breaks except the header line break.
			if didReadContent {
				appendBreaks(1 + emptyLines)
			} else {
				appendBreaks(emptyLines)
			}
		}

		didReadContent = true
		detectedIndent = true
		emptyLines = 0
		captureStart := s.position

		for !isEol(ch) && ch != 0 {
			s.position++
			ch = s.at(s.position)
		}

		s.captureSegment(captureStart, s.position, false)
	}

	s.finishScalar()
	return true
}

func (s *state) readBlockSequence(nodeIndent int) bool {
	tag := s.tag
	anchor := s.anchor
	result := &seq{items: []any{}}
	detected := false

	// there is a leading tab before this token, so it can't be a block sequence/mapping;
	// it can still be flow sequence/mapping or a scalar
	if s.firstTabInLine != -1 {
		return false
	}

	if s.anchor != null {
		s.storeAnchor(s.anchor, result)
	}

	ch := s.at(s.position)

	for ch != 0 {
		if s.firstTabInLine != -1 {
			s.position = s.firstTabInLine
			s.throwError("tab characters must not be used in indentation")
		}

		if ch != 0x2D /* - */ {
			break
		}

		following := s.at(s.position + 1)

		if !isWsOrEol(following) {
			break
		}

		detected = true
		s.position++

		if s.skipSeparationSpace(true, -1) != 0 {
			if s.lineIndent <= nodeIndent {
				result.items = append(result.items, nil)
				ch = s.at(s.position)
				continue
			}
		}

		line := s.line
		s.composeNode(nodeIndent, contextBlockIn, false, true)
		result.items = append(result.items, s.result)
		s.skipSeparationSpace(true, -1)

		ch = s.at(s.position)

		if (s.line == line || s.lineIndent > nodeIndent) && ch != 0 {
			s.throwError("bad indentation of a sequence entry")
		} else if s.lineIndent < nodeIndent {
			break
		}
	}

	if detected {
		s.tag = tag
		s.anchor = anchor
		s.kind = "sequence"
		s.result = result
		return true
	}
	return false
}

func (s *state) readBlockMapping(nodeIndent, flowIndent int) bool {
	var (
		allowCompact                  bool
		keyLine, keyLineStart, keyPos int
		tag                           = s.tag
		anchor                        = s.anchor
		result                        = js.NewObj[any]()
		keyNode, valueNode            any
		atExplicitKey                 = false
		detected                      = false
	)

	// there is a leading tab before this token, so it can't be a block sequence/mapping;
	// it can still be flow sequence/mapping or a scalar
	if s.firstTabInLine != -1 {
		return false
	}

	if s.anchor != null {
		s.storeAnchor(s.anchor, result)
	}

	ch := s.at(s.position)

	for ch != 0 {
		if !atExplicitKey && s.firstTabInLine != -1 {
			s.position = s.firstTabInLine
			s.throwError("tab characters must not be used in indentation")
		}

		following := s.at(s.position + 1)
		line := s.line // Save the current line.

		//
		// Explicit notation case. There are two separate blocks:
		// first for the key (denoted by "?") and second for the value (denoted by ":")
		//
		if (ch == 0x3F /* ? */ || ch == 0x3A /* : */) && isWsOrEol(following) {
			if ch == 0x3F /* ? */ {
				if atExplicitKey {
					s.storeMappingPair(result, keyNode, nil, keyLine, keyLineStart, keyPos)
					keyNode = nil
					valueNode = nil
				}

				detected = true
				atExplicitKey = true
				allowCompact = true
			} else if atExplicitKey {
				// i.e. 0x3A/* : */ === character after the explicit key.
				atExplicitKey = false
				allowCompact = true
			} else {
				s.throwError("incomplete explicit mapping pair; a key node is missed; or followed by a non-tabulated empty line")
			}

			s.position += 1
			ch = following

			//
			// Implicit notation case. Flow-style node as the key first, then ":", and the value.
			//
		} else {
			keyLine = s.line
			keyLineStart = s.lineStart
			keyPos = s.position

			if !s.composeNode(flowIndent, contextFlowOut, false, true) {
				// Neither implicit nor explicit notation.
				// Reading is done. Go to the epilogue.
				break
			}

			if s.line == line {
				ch = s.at(s.position)

				for isWhiteSpace(ch) {
					s.position++
					ch = s.at(s.position)
				}

				if ch == 0x3A /* : */ {
					s.position++
					ch = s.at(s.position)

					if !isWsOrEol(ch) {
						s.throwError("a whitespace character is expected after the key-value separator within a block mapping")
					}

					if atExplicitKey {
						s.storeMappingPair(result, keyNode, nil, keyLine, keyLineStart, keyPos)
						keyNode = nil
						valueNode = nil
					}

					detected = true
					atExplicitKey = false
					allowCompact = false
					keyNode = s.result
				} else if detected {
					s.throwError("can not read an implicit mapping pair; a colon is missed")
				} else {
					s.tag = tag
					s.anchor = anchor
					return true // Keep the result of `composeNode`.
				}
			} else if detected {
				s.throwError("can not read a block mapping entry; a multiline key may not be an implicit key")
			} else {
				s.tag = tag
				s.anchor = anchor
				return true // Keep the result of `composeNode`.
			}
		}

		//
		// Common reading code for both explicit and implicit notations.
		//
		if s.line == line || s.lineIndent > nodeIndent {
			if atExplicitKey {
				keyLine = s.line
				keyLineStart = s.lineStart
				keyPos = s.position
			}

			if s.composeNode(nodeIndent, contextBlockOut, true, allowCompact) {
				if atExplicitKey {
					keyNode = s.result
				} else {
					valueNode = s.result
				}
			}

			if !atExplicitKey {
				s.storeMappingPair(result, keyNode, valueNode, keyLine, keyLineStart, keyPos)
				keyNode = nil
				valueNode = nil
			}

			s.skipSeparationSpace(true, -1)
			ch = s.at(s.position)
		}

		if (s.line == line || s.lineIndent > nodeIndent) && ch != 0 {
			s.throwError("bad indentation of a mapping entry")
		} else if s.lineIndent < nodeIndent {
			break
		}
	}

	//
	// Epilogue.
	//

	// Special case: last mapping's node contains only the key in explicit notation.
	if atExplicitKey {
		s.storeMappingPair(result, keyNode, nil, keyLine, keyLineStart, keyPos)
	}

	// Expose the resulting mapping.
	if detected {
		s.tag = tag
		s.anchor = anchor
		s.kind = "mapping"
		s.result = result
	}

	return detected
}

func (s *state) readTagProperty() bool {
	var (
		isVerbatim = false
		isNamed    = false
		tagHandle  string
		tagName    []rune
	)

	ch := s.at(s.position)

	if ch != 0x21 /* ! */ {
		return false
	}

	if s.tag != null {
		s.throwError("duplication of a tag property")
	}

	s.position++
	ch = s.at(s.position)

	if ch == 0x3C /* < */ {
		isVerbatim = true
		s.position++
		ch = s.at(s.position)
	} else if ch == 0x21 /* ! */ {
		isNamed = true
		tagHandle = "!!"
		s.position++
		ch = s.at(s.position)
	} else {
		tagHandle = "!"
	}

	position := s.position

	if isVerbatim {
		for {
			s.position++
			ch = s.at(s.position)
			if !(ch != 0 && ch != 0x3E /* > */) {
				break
			}
		}

		if s.position < s.length {
			tagName = s.slice(position, s.position)
			s.position++
			ch = s.at(s.position)
		} else {
			s.throwError("unexpected end of the stream within a verbatim tag")
		}
	} else {
		for ch != 0 && !isWsOrEol(ch) {
			if ch == 0x21 /* ! */ {
				if !isNamed {
					handle := s.slice(position-1, s.position+1)

					if !isTagHandle(handle) {
						s.throwError("named tag handle cannot contain such characters")
					}

					tagHandle = js.FromUTF16Units(handle)
					isNamed = true
					position = s.position + 1
				} else {
					s.throwError("tag suffix cannot contain exclamation marks")
				}
			}

			s.position++
			ch = s.at(s.position)
		}

		tagName = s.slice(position, s.position)

		if hasFlowIndicator(tagName) {
			s.throwError("tag suffix cannot contain flow indicator characters")
		}
	}

	if len(tagName) > 0 && !isTagURI(tagName) {
		s.throwError("tag name cannot contain such characters: " + js.FromUTF16Units(tagName))
	}

	decoded, ok := decodeURIComponent(tagName)
	if !ok {
		s.throwError("tag name is malformed: " + js.FromUTF16Units(tagName))
	}
	name := js.FromUTF16Units(decoded)

	if isVerbatim {
		s.tag = name
	} else if prefix, ok := s.tagMap[tagHandle]; ok {
		s.tag = prefix + name
	} else if tagHandle == "!" {
		s.tag = "!" + name
	} else if tagHandle == "!!" {
		s.tag = "tag:yaml.org,2002:" + name
	} else {
		s.throwError(`undeclared tag handle "` + tagHandle + `"`)
	}

	return true
}

func (s *state) readAnchorProperty() bool {
	ch := s.at(s.position)

	if ch != 0x26 /* & */ {
		return false
	}

	if s.anchor != null {
		s.throwError("duplication of an anchor property")
	}

	s.position++
	ch = s.at(s.position)
	position := s.position

	for ch != 0 && !isWsOrEol(ch) && !isFlowIndicator(ch) {
		s.position++
		ch = s.at(s.position)
	}

	if s.position == position {
		s.throwError("name of an anchor node must contain at least one character")
	}

	s.anchor = js.FromUTF16Units(s.slice(position, s.position))
	return true
}

func (s *state) readAlias() bool {
	ch := s.at(s.position)

	if ch != 0x2A /* * */ {
		return false
	}

	s.position++
	ch = s.at(s.position)
	position := s.position

	for ch != 0 && !isWsOrEol(ch) && !isFlowIndicator(ch) {
		s.position++
		ch = s.at(s.position)
	}

	if s.position == position {
		s.throwError("name of an alias node must contain at least one character")
	}

	alias := js.FromUTF16Units(s.slice(position, s.position))

	value, ok := s.anchorMap[alias]
	if !ok {
		s.throwError(`unidentified alias "` + alias + `"`)
	}

	s.result = value
	s.skipSeparationSpace(true, -1)
	return true
}

func (s *state) tryReadBlockMappingFromProperty(propertyStart *snapshot, nodeIndent, flowIndent int) bool {
	fallbackState := s.snapshotState()

	s.beginAnchorTransaction()
	s.restoreState(propertyStart)
	s.tag = null
	s.anchor = null
	s.kind = null
	s.result = nil

	if s.readBlockMapping(nodeIndent, flowIndent) && s.kind == "mapping" {
		s.commitAnchorTransaction()
		return true
	}

	s.rollbackAnchorTransaction()
	s.restoreState(fallbackState)
	return false
}

func (s *state) composeNode(parentIndent, nodeContext int, allowToSeek, allowCompact bool) bool {
	var (
		allowBlockStyles      bool
		allowBlockScalars     bool
		allowBlockCollections bool
		indentStatus          = 1 // 1: this>parent, 0: this=parent, -1: this<parent
		atNewLine             = false
		hasContent            = false
		propertyStart         *snapshot
		flowIndent            int
		blockIndent           int
	)

	if s.depth >= maxDepth {
		s.throwError("nesting exceeded maxDepth (" + strconv.Itoa(maxDepth) + ")")
	}

	s.depth += 1

	s.tag = null
	s.anchor = null
	s.kind = null
	s.result = nil

	allowBlockStyles = contextBlockOut == nodeContext || contextBlockIn == nodeContext
	allowBlockScalars = allowBlockStyles
	allowBlockCollections = allowBlockStyles

	if allowToSeek {
		if s.skipSeparationSpace(true, -1) != 0 {
			atNewLine = true

			if s.lineIndent > parentIndent {
				indentStatus = 1
			} else if s.lineIndent == parentIndent {
				indentStatus = 0
			} else if s.lineIndent < parentIndent {
				indentStatus = -1
			}
		}
	}

	if indentStatus == 1 {
		for {
			ch := s.at(s.position)
			propertyState := s.snapshotState()

			// A property that starts on a new line after an earlier property of
			// the same kind belongs to the first key of a nested block mapping,
			// not to the node itself.
			if atNewLine &&
				(ch == 0x21 /* ! */ && s.tag != null ||
					ch == 0x26 /* & */ && s.anchor != null) {
				break
			}

			if !s.readTagProperty() && !s.readAnchorProperty() {
				break
			}

			if propertyStart == nil {
				propertyStart = propertyState
			}

			if s.skipSeparationSpace(true, -1) != 0 {
				atNewLine = true
				allowBlockCollections = allowBlockStyles

				if s.lineIndent > parentIndent {
					indentStatus = 1
				} else if s.lineIndent == parentIndent {
					indentStatus = 0
				} else if s.lineIndent < parentIndent {
					indentStatus = -1
				}
			} else {
				allowBlockCollections = false
			}
		}
	}

	if allowBlockCollections {
		allowBlockCollections = atNewLine || allowCompact
	}

	if indentStatus == 1 || contextBlockOut == nodeContext {
		if contextFlowIn == nodeContext || contextFlowOut == nodeContext {
			flowIndent = parentIndent
		} else {
			flowIndent = parentIndent + 1
		}

		blockIndent = s.position - s.lineStart

		if indentStatus == 1 {
			if allowBlockCollections &&
				(s.readBlockSequence(blockIndent) ||
					s.readBlockMapping(blockIndent, flowIndent)) ||
				s.readFlowCollection(flowIndent) {
				hasContent = true
			} else {
				ch := s.at(s.position)

				if propertyStart != nil &&
					allowBlockStyles &&
					!allowBlockCollections &&
					ch != 0x7C /* | */ &&
					ch != 0x3E /* > */ &&
					s.tryReadBlockMappingFromProperty(
						propertyStart,
						propertyStart.position-propertyStart.lineStart,
						flowIndent,
					) {
					hasContent = true
				} else if (allowBlockScalars && s.readBlockScalar(flowIndent)) ||
					s.readSingleQuotedScalar(flowIndent) ||
					s.readDoubleQuotedScalar(flowIndent) {
					hasContent = true
				} else if s.readAlias() {
					hasContent = true

					if s.tag != null || s.anchor != null {
						s.throwError("alias node should not have any properties")
					}
				} else if s.readPlainScalar(flowIndent, contextFlowIn == nodeContext) {
					hasContent = true

					if s.tag == null {
						s.tag = "?"
					}
				}

				if s.anchor != null {
					s.storeAnchor(s.anchor, s.result)
				}
			}
		} else if indentStatus == 0 {
			// Special case: block sequences are allowed to have same indentation level as the parent.
			// http://www.yaml.org/spec/1.2/spec.html#id2799784
			hasContent = allowBlockCollections && s.readBlockSequence(blockIndent)
		}
	}

	if s.tag == null {
		if s.anchor != null {
			s.storeAnchor(s.anchor, s.result)
		}
	} else if s.tag == "?" {
		// Implicit resolving is not allowed for non-scalar types, and '?'
		// non-specific tag is only automatically assigned to plain scalars.
		//
		// We only need to check kind conformity in case user explicitly assigns '?'
		// tag, for example like this: "!<?> [0]"
		//
		if s.result != nil && s.kind != "scalar" {
			s.throwError(`unacceptable node kind for !<?> tag; it should be "scalar", not "` + s.kindText() + `"`)
		}

		for _, typ := range implicitTypes {
			if typ.resolve(s.result) { // `state.result` updated in resolver if matched
				s.result = typ.construct(s.result)
				s.tag = typ.tag
				if s.anchor != null {
					s.storeAnchor(s.anchor, s.result)
				}
				break
			}
		}
	} else if s.tag != "!" {
		kind := s.kind
		if kind == null {
			kind = "fallback"
		}
		typ := lookupType(kind, s.tag)

		if typ == nil {
			s.throwError("unknown tag !<" + s.tag + ">")
		}

		if s.result != nil && typ.kind != s.kind {
			s.throwError("unacceptable node kind for !<" + s.tag + `> tag; it should be "` + typ.kind + `", not "` + s.kindText() + `"`)
		}

		if !typ.resolve(s.result) { // `state.result` updated in resolver if matched
			s.throwError("cannot resolve a node with !<" + s.tag + "> explicit tag")
		} else {
			s.result = typ.construct(s.result)
			if s.anchor != null {
				s.storeAnchor(s.anchor, s.result)
			}
		}
	}

	s.depth -= 1
	return s.tag != null || s.anchor != null || hasContent
}

// kindText is the kind of the state in a message: the text "null" for a
// kind that is null.
func (s *state) kindText() string {
	if s.kind == null {
		return "null"
	}
	return s.kind
}

func (s *state) readDocument() {
	documentStart := s.position
	hasDirectives := false
	var ch int

	s.version = null
	s.checkLineBreaks = false
	s.tagMap = map[string]string{}
	s.anchorMap = map[string]any{}

	for {
		ch = s.at(s.position)
		if ch == 0 {
			break
		}
		s.skipSeparationSpace(true, -1)

		ch = s.at(s.position)

		if s.lineIndent > 0 || ch != 0x25 /* % */ {
			break
		}

		hasDirectives = true
		s.position++
		ch = s.at(s.position)
		position := s.position

		for ch != 0 && !isWsOrEol(ch) {
			s.position++
			ch = s.at(s.position)
		}

		directiveName := js.FromUTF16Units(s.slice(position, s.position))
		directiveArgs := [][]rune{}

		if s.position-position < 1 {
			s.throwError("directive name must not be less than one character in length")
		}

		for ch != 0 {
			for isWhiteSpace(ch) {
				s.position++
				ch = s.at(s.position)
			}

			if ch == 0x23 /* # */ {
				for {
					s.position++
					ch = s.at(s.position)
					if !(ch != 0 && !isEol(ch)) {
						break
					}
				}
				break
			}

			if isEol(ch) {
				break
			}

			position = s.position

			for ch != 0 && !isWsOrEol(ch) {
				s.position++
				ch = s.at(s.position)
			}

			directiveArgs = append(directiveArgs, s.slice(position, s.position))
		}

		if ch != 0 {
			s.readLineBreak()
		}

		switch directiveName {
		case "YAML":
			s.handleYamlDirective(directiveArgs)
		case "TAG":
			s.handleTagDirective(directiveArgs)
		default:
			s.throwWarning(`unknown document directive "` + directiveName + `"`)
		}
	}

	s.skipSeparationSpace(true, -1)

	if s.lineIndent == 0 &&
		s.at(s.position) == 0x2D /* - */ &&
		s.at(s.position+1) == 0x2D /* - */ &&
		s.at(s.position+2) == 0x2D /* - */ {
		s.position += 3
		s.skipSeparationSpace(true, -1)
	} else if hasDirectives {
		s.throwError("directives end mark is expected")
	}

	s.composeNode(s.lineIndent-1, contextBlockOut, false, true)
	s.skipSeparationSpace(true, -1)

	// The check of the non-ASCII line breaks only makes a warning.
	_ = documentStart

	s.documents = append(s.documents, s.result)

	if s.position == s.lineStart && s.testDocumentSeparator() {
		if s.at(s.position) == 0x2E /* . */ {
			s.position += 3
			s.skipSeparationSpace(true, -1)
		}
		return
	}

	if s.position < (s.length - 1) {
		s.throwError("end of the stream or a document separator is expected")
	}
}

// loadDocuments is loadDocuments(input, options) with the JSON schema.
func loadDocuments(text string) []any {
	input := js.UTF16Units(text)

	if len(input) != 0 {
		// Add tailing `\n` if not exists
		if input[len(input)-1] != 0x0A /* LF */ &&
			input[len(input)-1] != 0x0D /* CR */ {
			input = append(input, '\n')
		}

		// Strip BOM
		if input[0] == 0xFEFF {
			input = input[1:]
		}
	}

	s := &state{
		input:          input,
		length:         len(input),
		firstTabInLine: -1,
		version:        null,
		tag:            null,
		anchor:         null,
		kind:           null,
	}

	nullpos := -1
	for i, u := range input {
		if u == 0 {
			nullpos = i
			break
		}
	}

	if nullpos != -1 {
		s.position = nullpos
		// generateError omits the last code unit of the input: here the NUL
		// at the end is not there yet.
		s.throwError("null byte is not allowed in input")
	}

	// Use 0 as string terminator. That significantly simplifies bounds check.
	s.input = append(append(make([]rune, 0, len(input)+1), input...), 0)

	for s.at(s.position) == 0x20 /* Space */ {
		s.lineIndent += 1
		s.position += 1
	}

	for s.position < (s.length - 1) {
		s.readDocument()
	}

	return s.documents
}

// export converts a document of the loader to a dynamic value of
// internal/js: an array is a []any, and the null of JavaScript is js.Null.
// A value that an alias repeats stays one value. The error is ErrCyclic for
// a document that contains itself.
func export(doc any) (any, error) {
	e := &exporter{
		seqs:    map[*seq][]any{},
		objs:    map[*js.Obj[any]]bool{},
		onStack: map[any]bool{},
	}
	return e.value(doc)
}

type exporter struct {
	seqs    map[*seq][]any
	objs    map[*js.Obj[any]]bool
	onStack map[any]bool
}

func (e *exporter) value(v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return js.Null, nil
	case *seq:
		if e.onStack[x] {
			return nil, ErrCyclic
		}
		if done, ok := e.seqs[x]; ok {
			return done, nil
		}
		e.onStack[x] = true
		out := make([]any, len(x.items))
		for i, item := range x.items {
			c, err := e.value(item)
			if err != nil {
				return nil, err
			}
			out[i] = c
		}
		delete(e.onStack, x)
		e.seqs[x] = out
		return out, nil
	case *js.Obj[any]:
		if e.onStack[x] {
			return nil, ErrCyclic
		}
		if e.objs[x] {
			return x, nil
		}
		e.onStack[x] = true
		for _, key := range x.Keys() {
			c, err := e.value(x.Get(key))
			if err != nil {
				return nil, err
			}
			x.Set(key, c)
		}
		delete(e.onStack, x)
		e.objs[x] = true
		return x, nil
	}
	return v, nil
}
