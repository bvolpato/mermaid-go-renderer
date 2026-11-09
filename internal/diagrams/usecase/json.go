// Ported from mermaid 12.2.0, diagrams/usecase/parser/usecaseJson.ts (MIT).
//
// Upstream calls JSON.parse and builds its own error from the message of
// the SyntaxError of V8: it reads the position from the message text. So
// this file also has JSON.parse with the messages of V8 (jsonParse, a port
// of the parser and of the error reporting of v8/src/json/json-parser.cc).
// The texts are UTF-16 code units, and the positions count them.
//
// The port does not record the version of V8. In V8 15.6.75, the version of
// the reference browser, json-parser.cc has the BSD-3-Clause license of V8
// and the line "Copyright 2016 the V8 project authors. All rights
// reserved." internal/js/LICENSE-V8.txt has the license.

package usecase

import (
	"regexp"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// orderedJSONObject is OrderedJsonObject.
type orderedJSONObject struct {
	value         *js.Obj[any]
	propertyOrder *js.Obj[[]string]
}

// jsonError is UsecaseJsonError. The name of the JavaScript error is
// UsecaseJsonError.
type jsonError struct {
	message string
	line    int
	column  int
}

func (e *jsonError) Error() string { return e.message }

func newJSONError(message string, line, column int) *jsonError {
	return &jsonError{
		message: message + " (line " + strconv.Itoa(line) + ", column " + strconv.Itoa(column) + ")",
		line:    line,
		column:  column,
	}
}

type lineColumn struct {
	line   int
	column int
}

// locationAtOffset is locationAtOffset(text, offset, startLine, startColumn).
// The offset is a JavaScript number: it can be larger than the text.
func locationAtOffset(text []rune, offset float64, startLine, startColumn int) lineColumn {
	line := startLine
	column := startColumn
	end := int(min(max(offset, 0), float64(len(text))))

	for index := 0; index < end; index++ {
		character := text[index]
		if character == '\r' {
			line++
			column = 1
		} else if character == '\n' {
			if index == 0 || text[index-1] != '\r' {
				line++
				column = 1
			}
		} else {
			column++
		}
	}

	return lineColumn{line, column}
}

var (
	rePosition      = regexp.MustCompile(`\bposition (\d+)\b`)
	reLocalLocation = regexp.MustCompile(`\bline (\d+) column (\d+)\b`)
	reUnexpectedEnd = regexp.MustCompile(`(?i)unexpected end|end of json input`)
)

// locationFromErrorMessage is locationFromErrorMessage(message, text,
// startLine, startColumn). The second result is false for undefined.
func locationFromErrorMessage(message string, text []rune, startLine, startColumn int) (lineColumn, bool) {
	if position := rePosition.FindStringSubmatch(message); position != nil {
		return locationAtOffset(text, js.ParseInt(position[1], 10), startLine, startColumn), true
	}

	// The message of V8 has the line and the column only together with the
	// position. This branch is reached when the message quotes a part of
	// the JSON text that reads like a location: at most 20 characters, so
	// the numbers are small.
	if localLocation := reLocalLocation.FindStringSubmatch(message); localLocation != nil {
		localLine := int(js.ParseInt(localLocation[1], 10))
		localColumn := int(js.ParseInt(localLocation[2], 10))
		column := localColumn
		if localLine == 1 {
			column = startColumn + localColumn - 1
		}
		return lineColumn{
			line:   startLine + localLine - 1,
			column: column,
		}, true
	}

	if reUnexpectedEnd.MatchString(message) {
		return locationAtOffset(text, float64(len(text)), startLine, startColumn), true
	}

	return lineColumn{}, false
}

// jsonWalkError is JsonWalkError: the offset of an invalid JSON token.
type jsonWalkError struct {
	offset int
}

// propertyOrderCollector is PropertyOrderCollector. It walks JSON tokens
// only after JSON.parse has authoritatively accepted or rejected the text.
// Its output is property order on success and a portable diagnostic offset
// on failure.
type propertyOrderCollector struct {
	offset int
	// propertyOrder has, by JSON pointer, the property names of an object.
	// A list grows while its object is read, so the object holds pointers.
	propertyOrder *js.Obj[*[]string]
	text          []rune
}

func newPropertyOrderCollector(text []rune) *propertyOrderCollector {
	return &propertyOrderCollector{propertyOrder: js.NewObj[*[]string](), text: text}
}

// collect is collect(). The error is the JsonWalkError of upstream.
func (c *propertyOrderCollector) collect() (order *js.Obj[[]string], walkErr *jsonWalkError) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*jsonWalkError)
			if !ok {
				panic(r)
			}
			order, walkErr = nil, e
		}
	}()
	c.skipWhitespace()
	c.collectValue("")
	c.skipWhitespace()
	if c.offset != len(c.text) {
		panic(&jsonWalkError{c.offset})
	}
	out := js.NewObj[[]string]()
	c.propertyOrder.Each(func(pointer string, names *[]string) { out.Set(pointer, *names) })
	return out, nil
}

// char is this.text[i]: the code unit, or -1 for undefined.
func (c *propertyOrderCollector) char(i int) rune { return at(c.text, i) }

func (c *propertyOrderCollector) collectValue(pointer string) {
	c.skipWhitespace()
	character := c.char(c.offset)

	if character == '{' {
		c.collectObject(pointer)
	} else if character == '[' {
		c.collectArray(pointer)
	} else if character == '"' {
		c.readString(false)
	} else if character == 't' {
		c.consumeLiteral("true")
	} else if character == 'f' {
		c.consumeLiteral("false")
	} else if character == 'n' {
		c.consumeLiteral("null")
	} else if character == '-' || (character >= '0' && character <= '9') {
		c.consumeNumber()
	} else {
		panic(&jsonWalkError{c.offset})
	}
}

// escapePointerPart is part.replaceAll('~', '~0').replaceAll('/', '~1').
func escapePointerPart(part string) string {
	out := make([]byte, 0, len(part))
	for i := 0; i < len(part); i++ {
		switch part[i] {
		case '~':
			out = append(out, '~', '0')
		case '/':
			out = append(out, '~', '1')
		default:
			out = append(out, part[i])
		}
	}
	return string(out)
}

func (c *propertyOrderCollector) collectObject(pointer string) {
	c.offset++
	order := &[]string{}
	seen := map[string]bool{}
	c.propertyOrder.Set(pointer, order)
	c.skipWhitespace()

	if c.char(c.offset) == '}' {
		c.offset++
		return
	}

	for c.offset < len(c.text) {
		if c.char(c.offset) != '"' {
			panic(&jsonWalkError{c.offset})
		}
		property := c.readString(true)
		propertyPointer := pointer + "/" + escapePointerPart(property)

		if seen[property] {
			c.deletePointerSubtree(propertyPointer)
		} else {
			seen[property] = true
			*order = append(*order, property)
		}

		c.skipWhitespace()
		if c.char(c.offset) != ':' {
			panic(&jsonWalkError{c.offset})
		}
		c.offset++
		c.collectValue(propertyPointer)
		c.skipWhitespace()

		if c.char(c.offset) == '}' {
			c.offset++
			return
		}
		if c.char(c.offset) != ',' {
			panic(&jsonWalkError{c.offset})
		}

		c.offset++
		c.skipWhitespace()
	}

	panic(&jsonWalkError{c.offset})
}

func (c *propertyOrderCollector) collectArray(pointer string) {
	c.offset++
	c.skipWhitespace()

	if c.char(c.offset) == ']' {
		c.offset++
		return
	}

	index := 0
	for c.offset < len(c.text) {
		c.collectValue(pointer + "/" + strconv.Itoa(index))
		index++
		c.skipWhitespace()

		if c.char(c.offset) == ']' {
			c.offset++
			return
		}
		if c.char(c.offset) != ',' {
			panic(&jsonWalkError{c.offset})
		}

		c.offset++
		c.skipWhitespace()
	}

	panic(&jsonWalkError{c.offset})
}

func (c *propertyOrderCollector) readString(decode bool) string {
	c.offset++
	var value []rune

	for c.offset < len(c.text) {
		characterOffset := c.offset
		character := c.text[c.offset]
		c.offset++
		if character == '"' {
			return js.FromUTF16Units(value)
		}
		if character < 0x20 {
			panic(&jsonWalkError{characterOffset})
		}
		if character != '\\' {
			if decode {
				value = append(value, character)
			}
			continue
		}

		escapeOffset := c.offset
		escape := c.char(c.offset)
		c.offset++
		switch escape {
		case '"', '\\', '/':
			if decode {
				value = append(value, escape)
			}
		case 'b':
			if decode {
				value = append(value, '\b')
			}
		case 'f':
			if decode {
				value = append(value, '\f')
			}
		case 'n':
			if decode {
				value = append(value, '\n')
			}
		case 'r':
			if decode {
				value = append(value, '\r')
			}
		case 't':
			if decode {
				value = append(value, '\t')
			}
		case 'u':
			// const codeUnit = this.text.slice(this.offset, this.offset + 4)
			// /^[\dA-Fa-f]{4}$/u.test(codeUnit)
			codeUnit := rune(0)
			valid := c.offset+4 <= len(c.text)
			for i := 0; valid && i < 4; i++ {
				digit := hexValue(c.text[c.offset+i])
				if digit < 0 {
					valid = false
					break
				}
				codeUnit = codeUnit*16 + rune(digit)
			}
			if !valid {
				panic(&jsonWalkError{c.offset})
			}
			if decode {
				value = append(value, codeUnit)
			}
			c.offset += 4
		default:
			panic(&jsonWalkError{escapeOffset})
		}
	}

	panic(&jsonWalkError{c.offset})
}

func hexValue(c rune) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func (c *propertyOrderCollector) consumeLiteral(literal string) {
	index := 0
	for i := 0; i < len(literal); i++ {
		if c.char(c.offset+index) != rune(literal[i]) {
			panic(&jsonWalkError{c.offset + index})
		}
		index++
	}
	c.offset += len(literal)
}

func (c *propertyOrderCollector) consumeNumber() {
	if c.char(c.offset) == '-' {
		c.offset++
	}

	if c.char(c.offset) == '0' {
		c.offset++
	} else if c.char(c.offset) >= '1' && c.char(c.offset) <= '9' {
		for c.char(c.offset) >= '0' && c.char(c.offset) <= '9' {
			c.offset++
		}
	} else {
		panic(&jsonWalkError{c.offset})
	}

	if c.char(c.offset) == '.' {
		c.offset++
		// this.text[this.offset] < '0' || this.text[this.offset] > '9': both
		// comparisons are false for undefined, past the end of the text.
		if c.offset < len(c.text) && (c.char(c.offset) < '0' || c.char(c.offset) > '9') {
			panic(&jsonWalkError{c.offset})
		}
		for c.char(c.offset) >= '0' && c.char(c.offset) <= '9' {
			c.offset++
		}
	}

	if c.char(c.offset) == 'e' || c.char(c.offset) == 'E' {
		c.offset++
		if c.char(c.offset) == '+' || c.char(c.offset) == '-' {
			c.offset++
		}
		if c.offset < len(c.text) && (c.char(c.offset) < '0' || c.char(c.offset) > '9') {
			panic(&jsonWalkError{c.offset})
		}
		for c.char(c.offset) >= '0' && c.char(c.offset) <= '9' {
			c.offset++
		}
	}
}

func (c *propertyOrderCollector) skipWhitespace() {
	for c.offset < len(c.text) {
		character := c.text[c.offset]
		if character != ' ' && character != '\t' && character != '\n' && character != '\r' {
			return
		}
		c.offset++
	}
}

func (c *propertyOrderCollector) deletePointerSubtree(pointer string) {
	descendantPrefix := pointer + "/"
	for _, existingPointer := range c.propertyOrder.Keys() {
		if existingPointer == pointer || (len(existingPointer) >= len(descendantPrefix) && existingPointer[:len(descendantPrefix)] == descendantPrefix) {
			c.propertyOrder.Delete(existingPointer)
		}
	}
}

// parseOrderedJSONObject is parseOrderedJsonObject(jsonText, startLine,
// startColumn).
func parseOrderedJSONObject(jsonText string, startLine, startColumn int) (*orderedJSONObject, error) {
	text := js.UTF16Units(jsonText)
	parsed, message, ok := jsonParse(text)
	if !ok {
		location, found := locationFromErrorMessage(message, text, startLine, startColumn)
		if !found {
			invalidOffset := 0
			if _, walkError := newPropertyOrderCollector(text).collect(); walkError != nil {
				invalidOffset = walkError.offset
			}
			location = locationAtOffset(text, float64(invalidOffset), startLine, startColumn)
		}
		return nil, newJSONError("Invalid JSON: "+message, location.line, location.column)
	}

	object, isObject := parsed.(*js.Obj[any])
	if !isObject {
		return nil, newJSONError("JSON value must have an object root", startLine, startColumn)
	}

	propertyOrder, walkError := newPropertyOrderCollector(text).collect()
	if walkError != nil {
		// The collector accepts every text that JSON.parse accepts.
		return nil, newJSONError("Invalid JSON token", startLine, startColumn)
	}
	return &orderedJSONObject{
		value:         object,
		propertyOrder: propertyOrder,
	}, nil
}

// ---- JSON.parse of V8 ----

// jsonToken is JsonToken of json-parser.h.
type jsonToken int

const (
	jsonNumber jsonToken = iota
	jsonString
	jsonLBrace
	jsonRBrace
	jsonLBrack
	jsonRBrack
	jsonTrueLiteral
	jsonFalseLiteral
	jsonNullLiteral
	jsonWhitespace
	jsonColon
	jsonComma
	jsonIllegal
	jsonEOS
)

// The message templates of V8 for JSON.parse that take the position, the
// line and the column.
const (
	msgExpectedPropNameOrRBrace           = "Expected property name or '}' in JSON"
	msgExpectedCommaOrRBrack              = "Expected ',' or ']' after array element in JSON"
	msgExpectedCommaOrRBrace              = "Expected ',' or '}' after property value in JSON"
	msgExpectedColonAfterPropertyName     = "Expected ':' after property name in JSON"
	msgUnterminatedString                 = "Unterminated string in JSON"
	msgBadControlCharacter                = "Bad control character in string literal in JSON"
	msgBadUnicodeEscape                   = "Bad Unicode escape in JSON"
	msgNoNumberAfterMinusSign             = "No number after minus sign in JSON"
	msgBadEscapedCharacter                = "Bad escaped character in JSON"
	msgExponentPartMissingNumber          = "Exponent part is missing a number in JSON"
	msgExpectedDoubleQuotedPropertyName   = "Expected double-quoted property name in JSON"
	msgUnterminatedFractionalNumber       = "Unterminated fractional number in JSON"
	msgUnexpectedNonWhiteSpaceCharacter   = "Unexpected non-whitespace character after JSON"
	msgUnexpectedTokenNumber              = "Unexpected number in JSON"
	msgUnexpectedTokenString              = "Unexpected string in JSON"
	jsonMaxContextCharacters              = 10
	jsonMinOriginalSourceLengthForContext = jsonMaxContextCharacters*2 + 1
)

// jsonSyntaxError carries the message of the SyntaxError of JSON.parse out
// of the parser.
type jsonSyntaxError struct {
	message string
}

// jsonParser is JsonParser of json-parser.cc.
type jsonParser struct {
	text   []rune
	cursor int
}

// jsonParse is JSON.parse(text). When the text is not valid JSON, ok is
// false and message is the message of the SyntaxError of V8.
func jsonParse(text []rune) (value any, message string, ok bool) {
	p := &jsonParser{text: text}
	defer func() {
		if r := recover(); r != nil {
			e, isSyntaxError := r.(*jsonSyntaxError)
			if !isSyntaxError {
				panic(r)
			}
			value, message, ok = nil, e.message, false
		}
	}()
	value = p.parseValue()
	p.skipWhitespace()
	if p.cursor < len(p.text) {
		p.reportUnexpectedToken(p.peek(), msgUnexpectedNonWhiteSpaceCharacter)
	}
	return value, "", true
}

// tokenOf is one_char_json_tokens: the token that a code unit starts.
func jsonTokenOf(c rune) jsonToken {
	switch {
	case c == '"':
		return jsonString
	case c == '-' || (c >= '0' && c <= '9'):
		return jsonNumber
	case c == '{':
		return jsonLBrace
	case c == '}':
		return jsonRBrace
	case c == '[':
		return jsonLBrack
	case c == ']':
		return jsonRBrack
	case c == ':':
		return jsonColon
	case c == ',':
		return jsonComma
	case c == 't':
		return jsonTrueLiteral
	case c == 'f':
		return jsonFalseLiteral
	case c == 'n':
		return jsonNullLiteral
	case c == ' ' || c == '\t' || c == '\n' || c == '\r':
		return jsonWhitespace
	}
	return jsonIllegal
}

// peek is the token at the cursor.
func (p *jsonParser) peek() jsonToken {
	if p.cursor >= len(p.text) {
		return jsonEOS
	}
	return jsonTokenOf(p.text[p.cursor])
}

func (p *jsonParser) skipWhitespace() {
	for p.cursor < len(p.text) && jsonTokenOf(p.text[p.cursor]) == jsonWhitespace {
		p.cursor++
	}
}

// reportUnexpectedToken is ReportUnexpectedToken(token, errorMessage): it
// raises the SyntaxError. With no errorMessage ("") the message comes from
// the token, as in LookUpErrorMessageForJsonToken.
func (p *jsonParser) reportUnexpectedToken(token jsonToken, errorMessage string) {
	pos := p.cursor
	withLocation := func(message string) string {
		// CalculateFileLocation
		line := 1
		lastLineBreak := 0
		for cursor := 0; cursor < pos && cursor < len(p.text); cursor++ {
			if p.text[cursor] == '\r' && cursor < pos-1 && p.text[cursor+1] == '\n' {
				// \r\n counts as a single line terminator.
				cursor++
			}
			if p.text[cursor] == '\r' || p.text[cursor] == '\n' {
				line++
				lastLineBreak = cursor + 1
			}
		}
		column := 1 + pos - lastLineBreak
		return message + " at position " + strconv.Itoa(pos) + " (line " + strconv.Itoa(line) + " column " + strconv.Itoa(column) + ")"
	}
	if errorMessage != "" {
		panic(&jsonSyntaxError{withLocation(errorMessage)})
	}
	switch token {
	case jsonEOS:
		panic(&jsonSyntaxError{"Unexpected end of JSON input"})
	case jsonNumber:
		panic(&jsonSyntaxError{withLocation(msgUnexpectedTokenNumber)})
	case jsonString:
		panic(&jsonSyntaxError{withLocation(msgUnexpectedTokenString)})
	}
	// The texts "NaN", "Infinity", "undefined" and "[object Object]" have
	// their own message in V8. A text of this diagram starts with a brace.
	arg := js.FromUTF16Units(p.text[pos : pos+1])
	length := len(p.text)
	// only provide context
	if length < jsonMinOriginalSourceLengthForContext {
		panic(&jsonSyntaxError{"Unexpected token '" + arg + "', \"" + js.FromUTF16Units(p.text) + "\" is not valid JSON"})
	}
	substringStart := 0
	substringEnd := length
	var prefix, suffix string
	if pos < jsonMaxContextCharacters {
		suffix = "..."
		substringEnd = pos + jsonMaxContextCharacters
	} else if pos < length-jsonMaxContextCharacters {
		prefix, suffix = "...", "..."
		substringStart = pos - jsonMaxContextCharacters
		substringEnd = pos + jsonMaxContextCharacters
	} else {
		prefix = "..."
		substringStart = pos - jsonMaxContextCharacters
	}
	panic(&jsonSyntaxError{
		"Unexpected token '" + arg + "', " + prefix + "\"" +
			js.FromUTF16Units(p.text[substringStart:substringEnd]) + "\"" + suffix + " is not valid JSON",
	})
}

// reportUnexpectedCharacter is ReportUnexpectedCharacter: the report for
// the token at the cursor.
func (p *jsonParser) reportUnexpectedCharacter() {
	p.reportUnexpectedToken(p.peek(), "")
}

// expect is Expect(token, errorMessage): it consumes the token at the
// cursor, or reports the error message.
func (p *jsonParser) expect(token jsonToken, errorMessage string) {
	if p.peek() == token {
		p.cursor++
	} else {
		p.reportUnexpectedToken(p.peek(), errorMessage)
	}
}

// check is Check(token): it skips white space and consumes the token when
// it is next.
func (p *jsonParser) check(token jsonToken) bool {
	p.skipWhitespace()
	if p.peek() != token {
		return false
	}
	p.cursor++
	return true
}

// parseValue is ParseJsonValue. It recurses where V8 keeps its own stack:
// the order of the checks is the same.
func (p *jsonParser) parseValue() any {
	p.skipWhitespace()
	switch p.peek() {
	case jsonString:
		p.cursor++
		return p.scanString()

	case jsonNumber:
		return p.parseNumber()

	case jsonLBrace:
		p.cursor++
		object := js.NewObj[any]()
		if p.check(jsonRBrace) {
			return object
		}
		// Parse the property key.
		p.skipWhitespace()
		p.expect(jsonString, msgExpectedPropNameOrRBrace)
		for {
			key := p.scanString()
			p.skipWhitespace()
			p.expect(jsonColon, msgExpectedColonAfterPropertyName)
			object.Set(key, p.parseValue())
			if p.check(jsonComma) {
				// Parse the property key.
				p.skipWhitespace()
				p.expect(jsonString, msgExpectedDoubleQuotedPropertyName)
				continue
			}
			p.expect(jsonRBrace, msgExpectedCommaOrRBrace)
			return object
		}

	case jsonLBrack:
		p.cursor++
		array := []any{}
		if p.check(jsonRBrack) {
			return array
		}
		for {
			array = append(array, p.parseValue())
			if p.check(jsonComma) {
				continue
			}
			p.expect(jsonRBrack, msgExpectedCommaOrRBrack)
			return array
		}

	case jsonTrueLiteral:
		p.scanLiteral("true")
		return true

	case jsonFalseLiteral:
		p.scanLiteral("false")
		return false

	case jsonNullLiteral:
		p.scanLiteral("null")
		return js.Null
	}
	// COLON, COMMA, ILLEGAL, RBRACE, RBRACK and EOS.
	p.reportUnexpectedCharacter()
	return nil
}

// scanLiteral is ScanLiteral(s).
func (p *jsonParser) scanLiteral(s string) {
	remaining := len(p.text) - p.cursor
	if remaining >= len(s) && hasPrefixAt(p.text, p.cursor, s) {
		p.cursor += len(s)
		return
	}

	p.cursor++
	for i := 0; i < min(len(s)-1, remaining-1); i++ {
		if rune(s[1+i]) != p.text[p.cursor] {
			p.reportUnexpectedCharacter()
			return
		}
		p.cursor++
	}
	p.reportUnexpectedToken(jsonEOS, "")
}

// scanString is ScanJsonString: the cursor is after the opening quote.
func (p *jsonParser) scanString() string {
	var value []rune
	for {
		if p.cursor >= len(p.text) {
			p.reportUnexpectedToken(jsonIllegal, msgUnterminatedString)
		}
		c := p.text[p.cursor]
		if c == '"' {
			p.cursor++
			return js.FromUTF16Units(value)
		}
		if c == '\\' {
			p.cursor++
			c := at(p.text, p.cursor)
			if c < 0 || c > 0xff {
				// The end of the text, or a character outside Latin-1.
				p.reportUnexpectedCharacter()
			}
			switch c {
			case '"', '/', '\\':
				value = append(value, c)
			case 'b':
				value = append(value, '\b')
			case 'f':
				value = append(value, '\f')
			case 'n':
				value = append(value, '\n')
			case 'r':
				value = append(value, '\r')
			case 't':
				value = append(value, '\t')
			case 'u':
				unit := rune(0)
				for i := 0; i < 4; i++ {
					p.cursor++
					digit := hexValue(at(p.text, p.cursor))
					if digit < 0 {
						p.reportUnexpectedToken(jsonIllegal, msgBadUnicodeEscape)
					}
					unit = unit*16 + rune(digit)
				}
				value = append(value, unit)
			default:
				p.reportUnexpectedToken(jsonIllegal, msgBadEscapedCharacter)
			}
			p.cursor++
			continue
		}
		if c < 0x20 {
			p.reportUnexpectedToken(jsonIllegal, msgBadControlCharacter)
		}
		value = append(value, c)
		p.cursor++
	}
}

// parseNumber is ParseJsonNumber.
func (p *jsonParser) parseNumber() float64 {
	start := p.cursor
	c := p.text[p.cursor]
	if c == '-' {
		p.cursor++
		c = at(p.text, p.cursor)
		if !isDigit(c) {
			p.reportUnexpectedToken(jsonIllegal, msgNoNumberAfterMinusSign)
		}
	}

	if c == '0' {
		// Prefix zero is only allowed if it's the only digit before
		// a decimal point or exponent.
		p.cursor++
		c = at(p.text, p.cursor)
		if isDigit(c) {
			p.reportUnexpectedToken(jsonNumber, "")
		}
	} else {
		p.cursor += countWhile(p.text, p.cursor, isDigit)
	}

	if at(p.text, p.cursor) == '.' {
		p.cursor++
		if !isDigit(at(p.text, p.cursor)) {
			p.reportUnexpectedToken(jsonIllegal, msgUnterminatedFractionalNumber)
		}
		p.cursor += countWhile(p.text, p.cursor, isDigit)
	}

	if c := at(p.text, p.cursor); c == 'e' || c == 'E' {
		p.cursor++
		if c := at(p.text, p.cursor); c == '-' || c == '+' {
			p.cursor++
		}
		if !isDigit(at(p.text, p.cursor)) {
			p.reportUnexpectedToken(jsonIllegal, msgExponentPartMissingNumber)
		}
		p.cursor += countWhile(p.text, p.cursor, isDigit)
	}

	// StringToDouble of the number text. A number that is too large is an
	// infinity, as in JavaScript.
	number, _ := strconv.ParseFloat(string(p.text[start:p.cursor]), 64)
	return number
}
