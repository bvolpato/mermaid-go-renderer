// Ported from js-yaml 4.3.0 (MIT), lib/loader.js, lib/schema/json.js,
// lib/schema/failsafe.js, lib/type/str.js, lib/type/seq.js, lib/type/map.js,
// lib/type/null.js, lib/type/bool.js, lib/type/int.js and lib/type/float.js,
// as bundled in mermaid 12.2.0.
//
// Copyright (C) 2011-2015 by Vitaly Puzrin (MIT).

package config

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

const (
	yamlContextFlowIn   = 1
	yamlContextFlowOut  = 2
	yamlContextBlockIn  = 3
	yamlContextBlockOut = 4

	yamlChompingClip  = 1
	yamlChompingStrip = 2
	yamlChompingKeep  = 3
)

const (
	yamlKindNone     = 0
	yamlKindScalar   = 1
	yamlKindSequence = 2
	yamlKindMapping  = 3
)

// yamlException is the YAMLException that js-yaml throws.
func yamlException(format string, args ...any) *Error {
	e := typeError(format, args...)
	e.Name = "YAMLException"
	return e
}

// loadYAML is yaml.load(text, { schema: yaml.JSON_SCHEMA }). The result is
// nil (undefined) for a text with no document.
func loadYAML(text string) (any, error) {
	docs, err := loadYAMLDocuments(text)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, nil
	}
	if len(docs) == 1 {
		return docs[0], nil
	}
	return nil, yamlException("expected a single document in the stream, but found more")
}

type yamlAnchorBackup struct {
	existed bool
	value   any
}

type yamlSnapshot struct {
	position       int
	line           int
	lineStart      int
	lineIndent     int
	firstTabInLine int
	hasTag         bool
	tag            string
	hasAnchor      bool
	anchor         string
	kind           uint8
	result         any
}

type yamlState struct {
	input          []rune // UTF-16 code units with trailing 0 sentinel
	length         int    // original UTF-16 length (excluding trailing 0 sentinel)
	position       int
	line           int
	lineStart      int
	lineIndent     int
	depth          int
	maxDepth       int
	firstTabInLine int

	version         string
	hasVersion      bool
	checkLineBreaks bool
	tagMap          map[string]string
	anchorMap       map[string]any
	anchorTx        []map[string]yamlAnchorBackup
	activeAnchors   map[string]bool

	hasTag    bool
	tag       string
	hasAnchor bool
	anchor    string
	kind      uint8
	result    any

	documents []any
}

type yamlParseError struct {
	err *Error
}

func (s *yamlState) throwError(msg string) {
	panic(yamlParseError{err: yamlException("%s", msg)})
}

func (s *yamlState) charAt(pos int) rune {
	if pos < 0 || pos >= len(s.input) {
		return 0
	}
	return s.input[pos]
}

func (s *yamlState) sliceStr(start, end int) string {
	return js.FromUTF16Units(s.input[start:end])
}

func yamlIsEol(c rune) bool {
	return c == 0x0A || c == 0x0D
}

func yamlIsWhiteSpace(c rune) bool {
	return c == 0x09 || c == 0x20
}

func yamlIsWsOrEol(c rune) bool {
	return c == 0x09 || c == 0x20 || c == 0x0A || c == 0x0D
}

func yamlIsFlowIndicator(c rune) bool {
	return c == ',' || c == '[' || c == ']' || c == '{' || c == '}'
}

func yamlFromHexCode(c rune) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	lc := c | 0x20
	if lc >= 'a' && lc <= 'f' {
		return int(lc - 'a' + 10)
	}
	return -1
}

func yamlEscapedHexLen(c rune) int {
	switch c {
	case 'x':
		return 2
	case 'u':
		return 4
	case 'U':
		return 8
	}
	return 0
}

func yamlFromDecimalCode(c rune) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	return -1
}

func yamlSimpleEscape(c rune) (string, bool) {
	switch c {
	case '0':
		return "\x00", true
	case 'a':
		return "\x07", true
	case 'b':
		return "\x08", true
	case 't', 0x09:
		return "\x09", true
	case 'n':
		return "\x0A", true
	case 'v':
		return "\x0B", true
	case 'f':
		return "\x0C", true
	case 'r':
		return "\x0D", true
	case 'e':
		return "\x1B", true
	case ' ':
		return " ", true
	case '"':
		return "\"", true
	case '/':
		return "/", true
	case '\\':
		return "\\", true
	case 'N':
		return "\u0085", true
	case '_':
		return "\u00A0", true
	case 'L':
		return "\u2028", true
	case 'P':
		return "\u2029", true
	}
	return "", false
}

func yamlCharFromCodepoint(c int) string {
	if c <= 0xFFFF {
		return js.FromUTF16Units([]rune{rune(c)})
	}
	return js.FromUTF16Units([]rune{
		rune(((c - 0x010000) >> 10) + 0xD800),
		rune(((c - 0x010000) & 0x03FF) + 0xDC00),
	})
}

func yamlHasNonPrintable(units []rune) bool {
	for i := 0; i < len(units); i++ {
		c := units[i]
		switch {
		case c <= 0x08 || c == 0x0B || c == 0x0C || (c >= 0x0E && c <= 0x1F) ||
			(c >= 0x7F && c <= 0x84) || (c >= 0x86 && c <= 0x9F) ||
			c == 0xFFFE || c == 0xFFFF:
			return true
		case c >= 0xD800 && c <= 0xDBFF:
			if i+1 >= len(units) || units[i+1] < 0xDC00 || units[i+1] > 0xDFFF {
				return true
			}
			i++
		case c >= 0xDC00 && c <= 0xDFFF:
			return true
		}
	}
	return false
}

func yamlIsValidTagHandle(s string) bool {
	if s == "!" || s == "!!" {
		return true
	}
	if len(s) < 3 || s[0] != '!' || s[len(s)-1] != '!' {
		return false
	}
	for i := 1; i < len(s)-1; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '-') {
			return false
		}
	}
	return true
}

func yamlIsValidTagURI(s string) bool {
	if s == "" {
		return false
	}
	// First character: ! or anything except , [ ] { }
	_, firstLen := utf8.DecodeRuneInString(s)
	if firstLen == 1 {
		switch s[0] {
		case ',', '[', ']', '{', '}':
			return false
		}
	}
	for i := firstLen; i < len(s); {
		c := s[i]
		if c == '%' {
			if i+2 >= len(s) || yamlFromHexCode(rune(s[i+1])) < 0 || yamlFromHexCode(rune(s[i+2])) < 0 {
				return false
			}
			i += 3
			continue
		}
		if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			strings.IndexByte("-#;/?:@&=+$,_.!~*'()[]", c) >= 0 {
			i++
			continue
		}
		return false
	}
	return true
}

func yamlDecodeURIComponent(s string) (string, bool) {
	if strings.IndexByte(s, '%') < 0 {
		return s, true
	}
	var out []byte
	for i := 0; i < len(s); {
		if s[i] != '%' {
			out = append(out, s[i])
			i++
			continue
		}
		// Decode a contiguous sequence of percent-encoded bytes and verify UTF-8.
		start := len(out)
		for i < len(s) && s[i] == '%' {
			if i+2 >= len(s) {
				return "", false
			}
			hi := yamlFromHexCode(rune(s[i+1]))
			lo := yamlFromHexCode(rune(s[i+2]))
			if hi < 0 || lo < 0 {
				return "", false
			}
			out = append(out, byte(hi<<4|lo))
			i += 3
		}
		if !utf8.Valid(out[start:]) {
			return "", false
		}
	}
	return string(out), true
}

func (s *yamlState) storeAnchor(name string, value any) {
	if len(s.anchorTx) > 0 {
		tx := s.anchorTx[len(s.anchorTx)-1]
		if _, ok := tx[name]; !ok {
			old, existed := s.anchorMap[name]
			tx[name] = yamlAnchorBackup{existed: existed, value: old}
		}
	}
	s.anchorMap[name] = value
}

func (s *yamlState) beginAnchorTransaction() {
	s.anchorTx = append(s.anchorTx, map[string]yamlAnchorBackup{})
}

func (s *yamlState) commitAnchorTransaction() {
	tx := s.anchorTx[len(s.anchorTx)-1]
	s.anchorTx = s.anchorTx[:len(s.anchorTx)-1]
	if len(s.anchorTx) == 0 {
		return
	}
	parent := s.anchorTx[len(s.anchorTx)-1]
	for name, entry := range tx {
		if _, ok := parent[name]; !ok {
			parent[name] = entry
		}
	}
}

func (s *yamlState) rollbackAnchorTransaction() {
	tx := s.anchorTx[len(s.anchorTx)-1]
	s.anchorTx = s.anchorTx[:len(s.anchorTx)-1]
	for name, entry := range tx {
		if entry.existed {
			s.anchorMap[name] = entry.value
		} else {
			delete(s.anchorMap, name)
		}
	}
}

func (s *yamlState) snapshot() yamlSnapshot {
	return yamlSnapshot{
		position:       s.position,
		line:           s.line,
		lineStart:      s.lineStart,
		lineIndent:     s.lineIndent,
		firstTabInLine: s.firstTabInLine,
		hasTag:         s.hasTag,
		tag:            s.tag,
		hasAnchor:      s.hasAnchor,
		anchor:         s.anchor,
		kind:           s.kind,
		result:         s.result,
	}
}

func (s *yamlState) restore(snap yamlSnapshot) {
	s.position = snap.position
	s.line = snap.line
	s.lineStart = snap.lineStart
	s.lineIndent = snap.lineIndent
	s.firstTabInLine = snap.firstTabInLine
	s.hasTag = snap.hasTag
	s.tag = snap.tag
	s.hasAnchor = snap.hasAnchor
	s.anchor = snap.anchor
	s.kind = snap.kind
	s.result = snap.result
}

func (s *yamlState) captureSegment(sb *strings.Builder, start, end int, checkJSON bool) {
	if start >= end {
		return
	}
	seg := s.input[start:end]
	if checkJSON {
		for _, c := range seg {
			if !(c == 0x09 || c >= 0x20) {
				s.throwError("expected valid JSON character")
			}
		}
	} else if yamlHasNonPrintable(seg) {
		s.throwError("the stream contains non-printable characters")
	}
	sb.WriteString(js.FromUTF16Units(seg))
}

func (s *yamlState) storeMappingPair(dst *js.Obj[any], keyNode, valueNode any, startLine, startLineStart, startPos int) *js.Obj[any] {
	key, err := mappingKey(keyNode)
	if err != nil {
		s.throwError(err.Error())
	}
	if dst == nil {
		dst = js.NewObj[any]()
	}
	if dst.Has(key) {
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
	dst.Set(key, valueNode)
	return dst
}

func (s *yamlState) readLineBreak() {
	ch := s.charAt(s.position)
	if ch == 0x0A {
		s.position++
	} else if ch == 0x0D {
		s.position++
		if s.charAt(s.position) == 0x0A {
			s.position++
		}
	} else {
		s.throwError("a line break is expected")
	}
	s.line++
	s.lineStart = s.position
	s.firstTabInLine = -1
}

func (s *yamlState) skipSeparationSpace(allowComments bool, checkIndent int) int {
	lineBreaks := 0
	ch := s.charAt(s.position)

	for ch != 0 {
		for yamlIsWhiteSpace(ch) {
			if ch == 0x09 && s.firstTabInLine == -1 {
				s.firstTabInLine = s.position
			}
			s.position++
			ch = s.charAt(s.position)
		}

		if allowComments && ch == '#' {
			for {
				s.position++
				ch = s.charAt(s.position)
				if ch == 0x0A || ch == 0x0D || ch == 0 {
					break
				}
			}
		}

		if yamlIsEol(ch) {
			s.readLineBreak()
			ch = s.charAt(s.position)
			lineBreaks++
			s.lineIndent = 0
			for ch == ' ' {
				s.lineIndent++
				s.position++
				ch = s.charAt(s.position)
			}
		} else {
			break
		}
	}

	_ = checkIndent // onWarning is nil in mermaid
	return lineBreaks
}

func (s *yamlState) testDocumentSeparator() bool {
	pos := s.position
	ch := s.charAt(pos)
	if (ch == '-' || ch == '.') && ch == s.charAt(pos+1) && ch == s.charAt(pos+2) {
		ch = s.charAt(pos + 3)
		if ch == 0 || yamlIsWsOrEol(ch) {
			return true
		}
	}
	return false
}

func writeFoldedLines(sb *strings.Builder, count int) {
	if count == 1 {
		sb.WriteByte(' ')
	} else if count > 1 {
		for i := 0; i < count-1; i++ {
			sb.WriteByte('\n')
		}
	}
}

func (s *yamlState) readPlainScalar(nodeIndent int, withinFlowCollection bool) bool {
	ch := s.charAt(s.position)
	if yamlIsWsOrEol(ch) || yamlIsFlowIndicator(ch) ||
		ch == '#' || ch == '&' || ch == '*' || ch == '!' || ch == '|' ||
		ch == '>' || ch == '\'' || ch == '"' || ch == '%' || ch == '@' || ch == '`' {
		return false
	}

	if ch == '?' || ch == '-' {
		following := s.charAt(s.position + 1)
		if yamlIsWsOrEol(following) || (withinFlowCollection && yamlIsFlowIndicator(following)) {
			return false
		}
	}

	oldKind := s.kind
	oldResult := s.result
	s.kind = yamlKindScalar
	var sb strings.Builder
	captureStart := s.position
	captureEnd := s.position
	hasPendingContent := false
	var prevLine, prevLineStart, prevLineIndent int

	for ch != 0 {
		if ch == ':' {
			following := s.charAt(s.position + 1)
			if yamlIsWsOrEol(following) || (withinFlowCollection && yamlIsFlowIndicator(following)) {
				break
			}
		} else if ch == '#' {
			preceding := s.charAt(s.position - 1)
			if yamlIsWsOrEol(preceding) {
				break
			}
		} else if (s.position == s.lineStart && s.testDocumentSeparator()) ||
			(withinFlowCollection && yamlIsFlowIndicator(ch)) {
			break
		} else if yamlIsEol(ch) {
			prevLine = s.line
			prevLineStart = s.lineStart
			prevLineIndent = s.lineIndent
			s.skipSeparationSpace(false, -1)

			if s.lineIndent >= nodeIndent {
				hasPendingContent = true
				ch = s.charAt(s.position)
				continue
			}
			s.position = captureEnd
			s.line = prevLine
			s.lineStart = prevLineStart
			s.lineIndent = prevLineIndent
			break
		}

		if hasPendingContent {
			s.captureSegment(&sb, captureStart, captureEnd, false)
			writeFoldedLines(&sb, s.line-prevLine)
			captureStart = s.position
			captureEnd = s.position
			hasPendingContent = false
		}

		if !yamlIsWhiteSpace(ch) {
			captureEnd = s.position + 1
		}

		s.position++
		ch = s.charAt(s.position)
	}

	s.captureSegment(&sb, captureStart, captureEnd, false)
	if sb.Len() > 0 {
		s.result = sb.String()
		return true
	}

	s.kind = oldKind
	s.result = oldResult
	return false
}

func (s *yamlState) readSingleQuotedScalar(nodeIndent int) bool {
	ch := s.charAt(s.position)
	if ch != '\'' {
		return false
	}
	s.kind = yamlKindScalar
	var sb strings.Builder
	s.position++
	captureStart := s.position
	captureEnd := s.position

	for {
		ch = s.charAt(s.position)
		if ch == 0 {
			break
		}
		if ch == '\'' {
			s.captureSegment(&sb, captureStart, s.position, true)
			s.position++
			ch = s.charAt(s.position)
			if ch == '\'' {
				captureStart = s.position
				s.position++
				captureEnd = s.position
			} else {
				s.result = sb.String()
				return true
			}
		} else if yamlIsEol(ch) {
			s.captureSegment(&sb, captureStart, captureEnd, true)
			writeFoldedLines(&sb, s.skipSeparationSpace(false, nodeIndent))
			captureStart = s.position
			captureEnd = s.position
		} else if s.position == s.lineStart && s.testDocumentSeparator() {
			s.throwError("unexpected end of the document within a single quoted scalar")
		} else {
			s.position++
			if !yamlIsWhiteSpace(ch) {
				captureEnd = s.position
			}
		}
	}
	s.throwError("unexpected end of the stream within a single quoted scalar")
	return false
}

func (s *yamlState) readDoubleQuotedScalar(nodeIndent int) bool {
	ch := s.charAt(s.position)
	if ch != '"' {
		return false
	}
	s.kind = yamlKindScalar
	var sb strings.Builder
	s.position++
	captureStart := s.position
	captureEnd := s.position

	for {
		ch = s.charAt(s.position)
		if ch == 0 {
			break
		}
		if ch == '"' {
			s.captureSegment(&sb, captureStart, s.position, true)
			s.position++
			s.result = sb.String()
			return true
		} else if ch == '\\' {
			s.captureSegment(&sb, captureStart, s.position, true)
			s.position++
			ch = s.charAt(s.position)

			if yamlIsEol(ch) {
				s.skipSeparationSpace(false, nodeIndent)
			} else if esc, ok := yamlSimpleEscape(ch); ok {
				sb.WriteString(esc)
				s.position++
			} else if hexLen := yamlEscapedHexLen(ch); hexLen > 0 {
				hexResult := 0
				for ; hexLen > 0; hexLen-- {
					s.position++
					ch = s.charAt(s.position)
					d := yamlFromHexCode(ch)
					if d < 0 {
						s.throwError("expected hexadecimal character")
					}
					hexResult = (hexResult << 4) + d
				}
				sb.WriteString(yamlCharFromCodepoint(hexResult))
				s.position++
			} else {
				s.throwError("unknown escape sequence")
			}

			captureStart = s.position
			captureEnd = s.position
		} else if yamlIsEol(ch) {
			s.captureSegment(&sb, captureStart, captureEnd, true)
			writeFoldedLines(&sb, s.skipSeparationSpace(false, nodeIndent))
			captureStart = s.position
			captureEnd = s.position
		} else if s.position == s.lineStart && s.testDocumentSeparator() {
			s.throwError("unexpected end of the document within a double quoted scalar")
		} else {
			s.position++
			if !yamlIsWhiteSpace(ch) {
				captureEnd = s.position
			}
		}
	}
	s.throwError("unexpected end of the stream within a double quoted scalar")
	return false
}

func (s *yamlState) readFlowCollection(nodeIndent int) bool {
	ch := s.charAt(s.position)
	var terminator rune
	var isMapping bool
	if ch == '[' {
		terminator = ']'
		isMapping = false
	} else if ch == '{' {
		terminator = '}'
		isMapping = true
	} else {
		return false
	}

	savedTag, savedHasTag := s.tag, s.hasTag
	savedAnchor, savedHasAnchor := s.anchor, s.hasAnchor

	var seq []any
	var mapping *js.Obj[any]
	if isMapping {
		mapping = js.NewObj[any]()
		if savedHasAnchor {
			s.storeAnchor(savedAnchor, mapping)
			s.activeAnchors[savedAnchor] = true
			defer delete(s.activeAnchors, savedAnchor)
		}
	} else {
		seq = []any{}
		if savedHasAnchor {
			s.storeAnchor(savedAnchor, seq)
			s.activeAnchors[savedAnchor] = true
			defer delete(s.activeAnchors, savedAnchor)
		}
	}

	readNext := true
	s.position++
	ch = s.charAt(s.position)

	for ch != 0 {
		s.skipSeparationSpace(true, nodeIndent)
		ch = s.charAt(s.position)

		if ch == terminator {
			s.position++
			s.tag, s.hasTag = savedTag, savedHasTag
			s.anchor, s.hasAnchor = savedAnchor, savedHasAnchor
			if isMapping {
				s.kind = yamlKindMapping
				s.result = mapping
			} else {
				s.kind = yamlKindSequence
				s.result = seq
				if savedHasAnchor {
					s.storeAnchor(savedAnchor, seq)
				}
			}
			return true
		} else if !readNext {
			s.throwError("missed comma between flow collection entries")
		} else if ch == ',' {
			s.throwError("expected the node content, but found ','")
		}

		var keyNode any = js.Null
		var valueNode any = js.Null
		isPair := false
		isExplicitPair := false

		if ch == '?' {
			following := s.charAt(s.position + 1)
			if yamlIsWsOrEol(following) {
				isPair = true
				isExplicitPair = true
				s.position++
				s.skipSeparationSpace(true, nodeIndent)
			}
		}

		savedLine := s.line
		savedLineStart := s.lineStart
		savedPos := s.position
		s.composeNode(nodeIndent, yamlContextFlowIn, false, true)
		keyNode = s.result
		s.skipSeparationSpace(true, nodeIndent)

		ch = s.charAt(s.position)
		if (isExplicitPair || s.line == savedLine) && ch == ':' {
			isPair = true
			s.position++
			s.skipSeparationSpace(true, nodeIndent)
			s.composeNode(nodeIndent, yamlContextFlowIn, false, true)
			valueNode = s.result
		}

		if isMapping {
			s.storeMappingPair(mapping, keyNode, valueNode, savedLine, savedLineStart, savedPos)
		} else if isPair {
			seq = append(seq, s.storeMappingPair(nil, keyNode, valueNode, savedLine, savedLineStart, savedPos))
		} else {
			seq = append(seq, keyNode)
		}

		s.skipSeparationSpace(true, nodeIndent)
		ch = s.charAt(s.position)
		if ch == ',' {
			readNext = true
			s.position++
			ch = s.charAt(s.position)
		} else {
			readNext = false
		}
	}

	s.throwError("unexpected end of the stream within a flow collection")
	return false
}

func (s *yamlState) readBlockScalar(nodeIndent int) bool {
	ch := s.charAt(s.position)
	var folding bool
	if ch == '|' {
		folding = false
	} else if ch == '>' {
		folding = true
	} else {
		return false
	}

	chomping := yamlChompingClip
	didReadContent := false
	detectedIndent := false
	textIndent := nodeIndent
	emptyLines := 0
	atMoreIndented := false

	s.kind = yamlKindScalar
	var sb strings.Builder

	for ch != 0 {
		s.position++
		ch = s.charAt(s.position)
		if ch == '+' || ch == '-' {
			if chomping == yamlChompingClip {
				if ch == '+' {
					chomping = yamlChompingKeep
				} else {
					chomping = yamlChompingStrip
				}
			} else {
				s.throwError("repeat of a chomping mode identifier")
			}
		} else if tmp := yamlFromDecimalCode(ch); tmp >= 0 {
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

	if yamlIsWhiteSpace(ch) {
		for {
			s.position++
			ch = s.charAt(s.position)
			if !yamlIsWhiteSpace(ch) {
				break
			}
		}
		if ch == '#' {
			for {
				s.position++
				ch = s.charAt(s.position)
				if yamlIsEol(ch) || ch == 0 {
					break
				}
			}
		}
	}

	writeNewlines := func(n int) {
		for i := 0; i < n; i++ {
			sb.WriteByte('\n')
		}
	}

	for ch != 0 {
		s.readLineBreak()
		s.lineIndent = 0
		ch = s.charAt(s.position)

		for (!detectedIndent || s.lineIndent < textIndent) && ch == ' ' {
			s.lineIndent++
			s.position++
			ch = s.charAt(s.position)
		}

		if !detectedIndent && s.lineIndent > textIndent {
			textIndent = s.lineIndent
		}

		if yamlIsEol(ch) {
			emptyLines++
			continue
		}

		if !detectedIndent && textIndent == 0 {
			s.throwError("missing indentation for block scalar")
		}

		if s.lineIndent < textIndent {
			if chomping == yamlChompingKeep {
				if didReadContent {
					writeNewlines(1 + emptyLines)
				} else {
					writeNewlines(emptyLines)
				}
			} else if chomping == yamlChompingClip {
				if didReadContent {
					sb.WriteByte('\n')
				}
			}
			break
		}

		if folding {
			if yamlIsWhiteSpace(ch) {
				atMoreIndented = true
				if didReadContent {
					writeNewlines(1 + emptyLines)
				} else {
					writeNewlines(emptyLines)
				}
			} else if atMoreIndented {
				atMoreIndented = false
				writeNewlines(emptyLines + 1)
			} else if emptyLines == 0 {
				if didReadContent {
					sb.WriteByte(' ')
				}
			} else {
				writeNewlines(emptyLines)
			}
		} else {
			if didReadContent {
				writeNewlines(1 + emptyLines)
			} else {
				writeNewlines(emptyLines)
			}
		}

		didReadContent = true
		detectedIndent = true
		emptyLines = 0
		captureStart := s.position

		for !yamlIsEol(ch) && ch != 0 {
			s.position++
			ch = s.charAt(s.position)
		}

		s.captureSegment(&sb, captureStart, s.position, false)
	}

	s.result = sb.String()
	return true
}

func (s *yamlState) readBlockSequence(nodeIndent int) bool {
	if s.firstTabInLine != -1 {
		return false
	}
	savedTag, savedHasTag := s.tag, s.hasTag
	savedAnchor, savedHasAnchor := s.anchor, s.hasAnchor
	seq := []any{}
	detected := false

	if savedHasAnchor {
		s.storeAnchor(savedAnchor, seq)
		s.activeAnchors[savedAnchor] = true
		defer delete(s.activeAnchors, savedAnchor)
	}

	ch := s.charAt(s.position)
	for ch != 0 {
		if s.firstTabInLine != -1 {
			s.position = s.firstTabInLine
			s.throwError("tab characters must not be used in indentation")
		}
		if ch != '-' {
			break
		}
		following := s.charAt(s.position + 1)
		if !yamlIsWsOrEol(following) {
			break
		}

		detected = true
		s.position++

		if s.skipSeparationSpace(true, -1) > 0 {
			if s.lineIndent <= nodeIndent {
				seq = append(seq, js.Null)
				ch = s.charAt(s.position)
				continue
			}
		}

		savedLine := s.line
		s.composeNode(nodeIndent, yamlContextBlockIn, false, true)
		seq = append(seq, s.result)
		s.skipSeparationSpace(true, -1)

		ch = s.charAt(s.position)
		if (s.line == savedLine || s.lineIndent > nodeIndent) && ch != 0 {
			s.throwError("bad indentation of a sequence entry")
		} else if s.lineIndent < nodeIndent {
			break
		}
	}

	if detected {
		s.tag, s.hasTag = savedTag, savedHasTag
		s.anchor, s.hasAnchor = savedAnchor, savedHasAnchor
		s.kind = yamlKindSequence
		s.result = seq
		if savedHasAnchor {
			s.storeAnchor(savedAnchor, seq)
		}
		return true
	}
	return false
}

func (s *yamlState) readBlockMapping(nodeIndent, flowIndent int) bool {
	if s.firstTabInLine != -1 {
		return false
	}

	savedTag, savedHasTag := s.tag, s.hasTag
	savedAnchor, savedHasAnchor := s.anchor, s.hasAnchor
	mapping := js.NewObj[any]()

	var keyNode any = js.Null
	var valueNode any = js.Null
	var keyLine, keyLineStart, keyPos int
	allowCompact := false
	atExplicitKey := false
	detected := false

	if savedHasAnchor {
		s.storeAnchor(savedAnchor, mapping)
		s.activeAnchors[savedAnchor] = true
		defer delete(s.activeAnchors, savedAnchor)
	}

	ch := s.charAt(s.position)
	for ch != 0 {
		if !atExplicitKey && s.firstTabInLine != -1 {
			s.position = s.firstTabInLine
			s.throwError("tab characters must not be used in indentation")
		}

		following := s.charAt(s.position + 1)
		savedLine := s.line

		if (ch == '?' || ch == ':') && yamlIsWsOrEol(following) {
			if ch == '?' {
				if atExplicitKey {
					s.storeMappingPair(mapping, keyNode, js.Null, keyLine, keyLineStart, keyPos)
					keyNode, valueNode = js.Null, js.Null
				}
				detected = true
				atExplicitKey = true
				allowCompact = true
			} else if atExplicitKey {
				atExplicitKey = false
				allowCompact = true
			} else {
				s.throwError("incomplete explicit mapping pair; a key node is missed; or followed by a non-tabulated empty line")
			}

			s.position++
			ch = following
		} else {
			keyLine = s.line
			keyLineStart = s.lineStart
			keyPos = s.position

			if !s.composeNode(flowIndent, yamlContextFlowOut, false, true) {
				break
			}

			if s.line == savedLine {
				ch = s.charAt(s.position)
				for yamlIsWhiteSpace(ch) {
					s.position++
					ch = s.charAt(s.position)
				}

				if ch == ':' {
					s.position++
					ch = s.charAt(s.position)
					if !yamlIsWsOrEol(ch) {
						s.throwError("a whitespace character is expected after the key-value separator within a block mapping")
					}
					if atExplicitKey {
						s.storeMappingPair(mapping, keyNode, js.Null, keyLine, keyLineStart, keyPos)
						keyNode, valueNode = js.Null, js.Null
					}
					detected = true
					atExplicitKey = false
					allowCompact = false
					keyNode = s.result
				} else if detected {
					s.throwError("can not read an implicit mapping pair; a colon is missed")
				} else {
					s.tag, s.hasTag = savedTag, savedHasTag
					s.anchor, s.hasAnchor = savedAnchor, savedHasAnchor
					return true
				}
			} else if detected {
				s.throwError("can not read a block mapping entry; a multiline key may not be an implicit key")
			} else {
				s.tag, s.hasTag = savedTag, savedHasTag
				s.anchor, s.hasAnchor = savedAnchor, savedHasAnchor
				return true
			}
		}

		if s.line == savedLine || s.lineIndent > nodeIndent {
			if atExplicitKey {
				keyLine = s.line
				keyLineStart = s.lineStart
				keyPos = s.position
			}
			if s.composeNode(nodeIndent, yamlContextBlockOut, true, allowCompact) {
				if atExplicitKey {
					keyNode = s.result
				} else {
					valueNode = s.result
				}
			}
			if !atExplicitKey {
				s.storeMappingPair(mapping, keyNode, valueNode, keyLine, keyLineStart, keyPos)
				keyNode, valueNode = js.Null, js.Null
			}
			s.skipSeparationSpace(true, -1)
			ch = s.charAt(s.position)
		}

		if (s.line == savedLine || s.lineIndent > nodeIndent) && ch != 0 {
			s.throwError("bad indentation of a mapping entry")
		} else if s.lineIndent < nodeIndent {
			break
		}
	}

	if atExplicitKey {
		s.storeMappingPair(mapping, keyNode, js.Null, keyLine, keyLineStart, keyPos)
	}

	if detected {
		s.tag, s.hasTag = savedTag, savedHasTag
		s.anchor, s.hasAnchor = savedAnchor, savedHasAnchor
		s.kind = yamlKindMapping
		s.result = mapping
	}

	return detected
}

func (s *yamlState) readTagProperty() bool {
	ch := s.charAt(s.position)
	if ch != '!' {
		return false
	}
	if s.hasTag {
		s.throwError("duplication of a tag property")
	}

	isVerbatim := false
	isNamed := false
	var tagHandle, tagName string

	s.position++
	ch = s.charAt(s.position)

	if ch == '<' {
		isVerbatim = true
		s.position++
		ch = s.charAt(s.position)
	} else if ch == '!' {
		isNamed = true
		tagHandle = "!!"
		s.position++
		ch = s.charAt(s.position)
	} else {
		tagHandle = "!"
	}

	startPos := s.position
	if isVerbatim {
		for {
			s.position++
			ch = s.charAt(s.position)
			if ch == 0 || ch == '>' {
				break
			}
		}
		if s.position < s.length {
			tagName = s.sliceStr(startPos, s.position)
			s.position++
		} else {
			s.throwError("unexpected end of the stream within a verbatim tag")
		}
	} else {
		for ch != 0 && !yamlIsWsOrEol(ch) {
			if ch == '!' {
				if !isNamed {
					tagHandle = s.sliceStr(startPos-1, s.position+1)
					if !yamlIsValidTagHandle(tagHandle) {
						s.throwError("named tag handle cannot contain such characters")
					}
					isNamed = true
					startPos = s.position + 1
				} else {
					s.throwError("tag suffix cannot contain exclamation marks")
				}
			}
			s.position++
			ch = s.charAt(s.position)
		}

		tagName = s.sliceStr(startPos, s.position)
		if strings.ContainsAny(tagName, ",[]{}") {
			s.throwError("tag suffix cannot contain flow indicator characters")
		}
	}

	if tagName != "" && !yamlIsValidTagURI(tagName) {
		s.throwError("tag name cannot contain such characters: " + tagName)
	}

	decoded, ok := yamlDecodeURIComponent(tagName)
	if !ok {
		s.throwError("tag name is malformed: " + tagName)
	}
	tagName = decoded

	if isVerbatim {
		s.tag = tagName
	} else if prefix, ok := s.tagMap[tagHandle]; ok {
		s.tag = prefix + tagName
	} else if tagHandle == "!" {
		s.tag = "!" + tagName
	} else if tagHandle == "!!" {
		s.tag = "tag:yaml.org,2002:" + tagName
	} else {
		s.throwError(`undeclared tag handle "` + tagHandle + `"`)
	}
	s.hasTag = true
	return true
}

func (s *yamlState) readAnchorProperty() bool {
	ch := s.charAt(s.position)
	if ch != '&' {
		return false
	}
	if s.hasAnchor {
		s.throwError("duplication of an anchor property")
	}

	s.position++
	ch = s.charAt(s.position)
	startPos := s.position
	for ch != 0 && !yamlIsWsOrEol(ch) && !yamlIsFlowIndicator(ch) {
		s.position++
		ch = s.charAt(s.position)
	}
	if s.position == startPos {
		s.throwError("name of an anchor node must contain at least one character")
	}
	s.anchor = s.sliceStr(startPos, s.position)
	s.hasAnchor = true
	return true
}

func (s *yamlState) readAlias() bool {
	ch := s.charAt(s.position)
	if ch != '*' {
		return false
	}

	s.position++
	ch = s.charAt(s.position)
	startPos := s.position
	for ch != 0 && !yamlIsWsOrEol(ch) && !yamlIsFlowIndicator(ch) {
		s.position++
		ch = s.charAt(s.position)
	}
	if s.position == startPos {
		s.throwError("name of an alias node must contain at least one character")
	}

	alias := s.sliceStr(startPos, s.position)
	val, ok := s.anchorMap[alias]
	if !ok {
		s.throwError(`unidentified alias "` + alias + `"`)
	}
	if s.activeAnchors[alias] {
		s.throwError(fmt.Sprintf("recursive alias %q is not supported", alias))
	}
	s.result = val
	s.skipSeparationSpace(true, -1)
	return true
}

func (s *yamlState) tryReadBlockMappingFromProperty(propertyStart yamlSnapshot, nodeIndent, flowIndent int) bool {
	fallback := s.snapshot()
	s.beginAnchorTransaction()
	s.restore(propertyStart)

	s.hasTag = false
	s.tag = ""
	s.hasAnchor = false
	s.anchor = ""
	s.kind = yamlKindNone
	s.result = js.Null

	if s.readBlockMapping(nodeIndent, flowIndent) && s.kind == yamlKindMapping {
		s.commitAnchorTransaction()
		return true
	}

	s.rollbackAnchorTransaction()
	s.restore(fallback)
	return false
}

func yamlKindName(k uint8) string {
	switch k {
	case yamlKindScalar:
		return "scalar"
	case yamlKindSequence:
		return "sequence"
	case yamlKindMapping:
		return "mapping"
	}
	return "null"
}

func (s *yamlState) composeNode(parentIndent, nodeContext int, allowToSeek, allowCompact bool) bool {
	if s.depth >= s.maxDepth {
		s.throwError(fmt.Sprintf("nesting exceeded maxDepth (%d)", s.maxDepth))
	}
	s.depth++
	defer func() { s.depth-- }()

	s.hasTag = false
	s.tag = ""
	s.hasAnchor = false
	s.anchor = ""
	s.kind = yamlKindNone
	s.result = js.Null

	allowBlockStyles := nodeContext == yamlContextBlockOut || nodeContext == yamlContextBlockIn
	allowBlockScalars := allowBlockStyles
	allowBlockCollections := allowBlockStyles

	indentStatus := 1
	atNewLine := false
	hasContent := false
	var propertyStart *yamlSnapshot

	if allowToSeek {
		if s.skipSeparationSpace(true, -1) > 0 {
			atNewLine = true
			if s.lineIndent > parentIndent {
				indentStatus = 1
			} else if s.lineIndent == parentIndent {
				indentStatus = 0
			} else {
				indentStatus = -1
			}
		}
	}

	if indentStatus == 1 {
		for {
			ch := s.charAt(s.position)
			propState := s.snapshot()

			if atNewLine && ((ch == '!' && s.hasTag) || (ch == '&' && s.hasAnchor)) {
				break
			}
			if !s.readTagProperty() && !s.readAnchorProperty() {
				break
			}
			if propertyStart == nil {
				snap := propState
				propertyStart = &snap
			}
			if s.skipSeparationSpace(true, -1) > 0 {
				atNewLine = true
				allowBlockCollections = allowBlockStyles
				if s.lineIndent > parentIndent {
					indentStatus = 1
				} else if s.lineIndent == parentIndent {
					indentStatus = 0
				} else {
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

	if indentStatus == 1 || nodeContext == yamlContextBlockOut {
		var flowIndent int
		if nodeContext == yamlContextFlowIn || nodeContext == yamlContextFlowOut {
			flowIndent = parentIndent
		} else {
			flowIndent = parentIndent + 1
		}
		blockIndent := s.position - s.lineStart

		if indentStatus == 1 {
			if (allowBlockCollections &&
				(s.readBlockSequence(blockIndent) || s.readBlockMapping(blockIndent, flowIndent))) ||
				s.readFlowCollection(flowIndent) {
				hasContent = true
			} else {
				ch := s.charAt(s.position)
				if propertyStart != nil && allowBlockStyles && !allowBlockCollections &&
					ch != '|' && ch != '>' &&
					s.tryReadBlockMappingFromProperty(*propertyStart, propertyStart.position-propertyStart.lineStart, flowIndent) {
					hasContent = true
				} else if (allowBlockScalars && s.readBlockScalar(flowIndent)) ||
					s.readSingleQuotedScalar(flowIndent) ||
					s.readDoubleQuotedScalar(flowIndent) {
					hasContent = true
				} else if s.readAlias() {
					hasContent = true
					if s.hasTag || s.hasAnchor {
						s.throwError("alias node should not have any properties")
					}
				} else if s.readPlainScalar(flowIndent, nodeContext == yamlContextFlowIn) {
					hasContent = true
					if !s.hasTag {
						s.hasTag = true
						s.tag = "?"
					}
				}

				if s.hasAnchor {
					s.storeAnchor(s.anchor, s.result)
				}
			}
		} else if indentStatus == 0 {
			hasContent = allowBlockCollections && s.readBlockSequence(blockIndent)
		}
	}

	if !s.hasTag {
		if s.hasAnchor {
			s.storeAnchor(s.anchor, s.result)
		}
	} else if s.tag == "?" {
		if s.result != js.Null && s.kind != yamlKindScalar {
			s.throwError(`unacceptable node kind for !<?> tag; it should be "scalar", not "` + yamlKindName(s.kind) + `"`)
		}
		if str, ok := s.result.(string); ok {
			switch {
			case resolveYamlNull(str):
				s.result = js.Null
				s.tag = "tag:yaml.org,2002:null"
			case resolveYamlBoolean(str):
				s.result = constructYamlBoolean(str)
				s.tag = "tag:yaml.org,2002:bool"
			case resolveYamlInteger(str):
				s.result = parseYamlInteger(str)
				s.tag = "tag:yaml.org,2002:int"
			case resolveYamlFloat(str):
				s.result = constructYamlFloat(str)
				s.tag = "tag:yaml.org,2002:float"
			}
			if s.hasAnchor {
				s.storeAnchor(s.anchor, s.result)
			}
		}
	} else if s.tag != "!" {
		s.resolveExplicitTag()
	}

	return s.hasTag || s.hasAnchor || hasContent
}

func (s *yamlState) resolveExplicitTag() {
	var wantKind uint8
	switch s.tag {
	case "tag:yaml.org,2002:str", "tag:yaml.org,2002:null", "tag:yaml.org,2002:bool", "tag:yaml.org,2002:int", "tag:yaml.org,2002:float":
		wantKind = yamlKindScalar
	case "tag:yaml.org,2002:seq":
		wantKind = yamlKindSequence
	case "tag:yaml.org,2002:map":
		wantKind = yamlKindMapping
	default:
		s.throwError("unknown tag !<" + s.tag + ">")
	}

	if s.result != js.Null && wantKind != s.kind {
		s.throwError(`unacceptable node kind for !<` + s.tag + `> tag; it should be "` + yamlKindName(wantKind) + `", not "` + yamlKindName(s.kind) + `"`)
	}

	isNull := s.result == js.Null
	switch s.tag {
	case "tag:yaml.org,2002:str":
		if isNull {
			s.result = ""
		}
	case "tag:yaml.org,2002:seq":
		if isNull {
			s.result = []any{}
		}
	case "tag:yaml.org,2002:map":
		if isNull {
			s.result = js.NewObj[any]()
		}
	case "tag:yaml.org,2002:null":
		if isNull {
			s.result = js.Null
		} else if str, ok := s.result.(string); ok && resolveYamlNull(str) {
			s.result = js.Null
		} else {
			s.throwError("cannot resolve a node with !<" + s.tag + "> explicit tag")
		}
	case "tag:yaml.org,2002:bool":
		if str, ok := s.result.(string); ok && resolveYamlBoolean(str) {
			s.result = constructYamlBoolean(str)
		} else {
			s.throwError("cannot resolve a node with !<" + s.tag + "> explicit tag")
		}
	case "tag:yaml.org,2002:int":
		if str, ok := s.result.(string); ok && resolveYamlInteger(str) {
			s.result = parseYamlInteger(str)
		} else {
			s.throwError("cannot resolve a node with !<" + s.tag + "> explicit tag")
		}
	case "tag:yaml.org,2002:float":
		if str, ok := s.result.(string); ok && resolveYamlFloat(str) {
			s.result = constructYamlFloat(str)
		} else {
			s.throwError("cannot resolve a node with !<" + s.tag + "> explicit tag")
		}
	}

	if s.hasAnchor {
		s.storeAnchor(s.anchor, s.result)
	}
}

func (s *yamlState) handleDirective(name string, args []string) {
	switch name {
	case "YAML":
		if s.hasVersion {
			s.throwError("duplication of %YAML directive")
		}
		if len(args) != 1 {
			s.throwError("YAML directive accepts exactly one argument")
		}
		arg := args[0]
		dot := strings.IndexByte(arg, '.')
		if dot <= 0 || dot == len(arg)-1 {
			s.throwError("ill-formed argument of the YAML directive")
		}
		majorStr, minorStr := arg[:dot], arg[dot+1:]
		for i := 0; i < len(majorStr); i++ {
			if !isDecCode(majorStr[i]) {
				s.throwError("ill-formed argument of the YAML directive")
			}
		}
		for i := 0; i < len(minorStr); i++ {
			if !isDecCode(minorStr[i]) {
				s.throwError("ill-formed argument of the YAML directive")
			}
		}
		major, _ := strconv.Atoi(majorStr)
		minor, _ := strconv.Atoi(minorStr)
		if major != 1 {
			s.throwError("unacceptable YAML version of the document")
		}
		s.hasVersion = true
		s.version = arg
		s.checkLineBreaks = minor < 2
	case "TAG":
		if len(args) != 2 {
			s.throwError("TAG directive accepts exactly two arguments")
		}
		handle, prefix := args[0], args[1]
		if !yamlIsValidTagHandle(handle) {
			s.throwError("ill-formed tag handle (first argument) of the TAG directive")
		}
		if _, ok := s.tagMap[handle]; ok {
			s.throwError(`there is a previously declared suffix for "` + handle + `" tag handle`)
		}
		if !yamlIsValidTagURI(prefix) {
			s.throwError("ill-formed tag prefix (second argument) of the TAG directive")
		}
		decoded, ok := yamlDecodeURIComponent(prefix)
		if !ok {
			s.throwError("tag prefix is malformed: " + prefix)
		}
		s.tagMap[handle] = decoded
	}
}

func (s *yamlState) readDocument() {
	hasDirectives := false
	s.hasVersion = false
	s.version = ""
	s.checkLineBreaks = false
	s.tagMap = map[string]string{}
	s.anchorMap = map[string]any{}
	s.activeAnchors = map[string]bool{}

	for {
		ch := s.charAt(s.position)
		if ch == 0 {
			break
		}
		s.skipSeparationSpace(true, -1)
		ch = s.charAt(s.position)
		if s.lineIndent > 0 || ch != '%' {
			break
		}

		hasDirectives = true
		s.position++
		ch = s.charAt(s.position)
		startPos := s.position
		for ch != 0 && !yamlIsWsOrEol(ch) {
			s.position++
			ch = s.charAt(s.position)
		}
		directiveName := s.sliceStr(startPos, s.position)
		var directiveArgs []string
		if len(directiveName) < 1 {
			s.throwError("directive name must not be less than one character in length")
		}

		for ch != 0 {
			for yamlIsWhiteSpace(ch) {
				s.position++
				ch = s.charAt(s.position)
			}
			if ch == '#' {
				for {
					s.position++
					ch = s.charAt(s.position)
					if ch == 0 || yamlIsEol(ch) {
						break
					}
				}
				break
			}
			if yamlIsEol(ch) {
				break
			}
			startPos = s.position
			for ch != 0 && !yamlIsWsOrEol(ch) {
				s.position++
				ch = s.charAt(s.position)
			}
			directiveArgs = append(directiveArgs, s.sliceStr(startPos, s.position))
		}

		if ch != 0 {
			s.readLineBreak()
		}
		s.handleDirective(directiveName, directiveArgs)
	}

	s.skipSeparationSpace(true, -1)

	if s.lineIndent == 0 &&
		s.charAt(s.position) == '-' &&
		s.charAt(s.position+1) == '-' &&
		s.charAt(s.position+2) == '-' {
		s.position += 3
		s.skipSeparationSpace(true, -1)
	} else if hasDirectives {
		s.throwError("directives end mark is expected")
	}

	s.composeNode(s.lineIndent-1, yamlContextBlockOut, false, true)
	s.skipSeparationSpace(true, -1)

	s.documents = append(s.documents, s.result)

	if s.position == s.lineStart && s.testDocumentSeparator() {
		if s.charAt(s.position) == '.' {
			s.position += 3
			s.skipSeparationSpace(true, -1)
		}
		return
	}

	if s.position < s.length-1 {
		s.throwError("end of the stream or a document separator is expected")
	}
}

func loadYAMLDocuments(input string) (docs []any, err error) {
	if input != "" {
		last := input[len(input)-1]
		if last != '\n' && last != '\r' {
			input += "\n"
		}
		input = strings.TrimPrefix(input, "\ufeff")
	}

	units := js.UTF16Units(input)
	for i, c := range units {
		if c == 0 {
			_ = i
			return nil, yamlException("null byte is not allowed in input")
		}
	}

	length := len(units)
	units = append(units, 0) // sentinel

	s := &yamlState{
		input:          units,
		length:         length,
		maxDepth:       100,
		firstTabInLine: -1,
	}

	defer func() {
		if r := recover(); r != nil {
			if pe, ok := r.(yamlParseError); ok {
				docs, err = nil, pe.err
				return
			}
			panic(r)
		}
	}()

	for s.charAt(s.position) == ' ' {
		s.lineIndent++
		s.position++
	}

	for s.position < s.length-1 {
		s.readDocument()
	}

	return s.documents, nil
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

	at := func(i int) byte {
		if i < max {
			return data[i]
		}
		return 0
	}

	ch := at(index)
	if ch == '-' || ch == '+' {
		index++
		ch = at(index)
	}

	if ch == '0' {
		if index+1 == max {
			return true
		}
		index++
		ch = at(index)

		if ch == 'b' {
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
