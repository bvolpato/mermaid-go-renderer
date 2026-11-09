// Ported from mermaid 12.2.0, the lexer that Langium builds for
// parser/src/language/gitGraph/gitGraph.langium, with the terminals of
// gitGraph/reference.langium and common/common.langium and the keyword rule of
// common/tokenBuilder.ts (MIT).

package git

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// tokenType is a token of the gitGraph lexer. The order is the order of the
// token list that Langium builds: the keywords from the longest to the
// shortest, then the terminals in grammar order. The lexer tries the tokens
// in this order and takes the first that matches.
type tokenType int

const (
	tkCherryPick tokenType = iota
	tkGitGraphColon
	tkHighlight
	tkGitGraph
	tkCheckout
	tkReverse
	tkParentColon
	tkCommit
	tkNormal
	tkBranch
	tkOrderColon
	tkSwitch
	tkTypeColon
	tkMerge
	tkMsgColon
	tkTagColon
	tkIDColon
	tkLR
	tkTB
	tkBT
	tkColon
	tkAccDescr
	tkAccTitle
	tkTitle
	tkInt
	tkString
	tkNewline
	tkWhitespace
	tkYAML
	tkDirective
	tkSingleLineComment
	tkReference
	tkEOF
)

// keywords is the keyword part of the token list. longerAlt is set for a
// keyword that the REFERENCE terminal also matches: when REFERENCE matches a
// longer text at the same place, the token is a REFERENCE.
var keywords = []struct {
	typ       tokenType
	text      string
	longerAlt bool
}{
	{tkCherryPick, "cherry-pick", true},
	{tkGitGraphColon, "gitGraph:", false},
	{tkHighlight, "HIGHLIGHT", true},
	{tkGitGraph, "gitGraph", true},
	{tkCheckout, "checkout", true},
	{tkReverse, "REVERSE", true},
	{tkParentColon, "parent:", false},
	{tkCommit, "commit", true},
	{tkNormal, "NORMAL", true},
	{tkBranch, "branch", true},
	{tkOrderColon, "order:", false},
	{tkSwitch, "switch", true},
	{tkTypeColon, "type:", false},
	{tkMerge, "merge", true},
	{tkMsgColon, "msg:", false},
	{tkTagColon, "tag:", false},
	{tkIDColon, "id:", false},
	{tkLR, "LR", true},
	{tkTB, "TB", true},
	{tkBT, "BT", true},
	{tkColon, ":", false},
}

// tokenNames are the names that an error message shows.
var tokenNames = map[tokenType]string{
	tkAccDescr:          "ACC_DESCR",
	tkAccTitle:          "ACC_TITLE",
	tkTitle:             "TITLE",
	tkInt:               "INT",
	tkString:            "STRING",
	tkNewline:           "NEWLINE",
	tkWhitespace:        "WHITESPACE",
	tkYAML:              "YAML",
	tkDirective:         "DIRECTIVE",
	tkSingleLineComment: "SINGLE_LINE_COMMENT",
	tkReference:         "REFERENCE",
	tkEOF:               "EOF",
}

func (t tokenType) String() string {
	if name, ok := tokenNames[t]; ok {
		return name
	}
	for _, k := range keywords {
		if k.typ == t {
			return k.text
		}
	}
	return "?"
}

// token is one token: its type and its text.
type token struct {
	typ   tokenType
	image string
	// offset is the byte offset of the token in the input.
	offset int
}

// isBlank is the class [\t ].
func isBlank(c byte) bool { return c == '\t' || c == ' ' }

// isWord is the class \w.
func isWord(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isLineBreak is the class [\n\r].
func isLineBreak(c byte) bool { return c == '\n' || c == '\r' }

// skipBlanks returns the position after [\t ]*.
func skipBlanks(s string, pos int) int {
	for pos < len(s) && isBlank(s[pos]) {
		pos++
	}
	return pos
}

// skipSpace returns the position after \s*, with the \s of JavaScript.
func skipSpace(s string, pos int) int {
	for pos < len(s) {
		r, size := utf8.DecodeRuneInString(s[pos:])
		if !js.IsWhiteSpace(r) {
			break
		}
		pos += size
	}
	return pos
}

// notNonSpace is the lookahead (?!\S): the end of the input, or a white
// space character.
func notNonSpace(s string, pos int) bool {
	if pos >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[pos:])
	return js.IsWhiteSpace(r)
}

// lineRest matches (?:[^\n\r]*?(?=%%)|[^\n\r]*): the text up to the first %%
// of the line, or else the rest of the line. It returns the end position.
func lineRest(s string, pos int) int {
	end := pos
	for end < len(s) && !isLineBreak(s[end]) {
		end++
	}
	if i := strings.Index(s[pos:end], "%%"); i >= 0 {
		return pos + i
	}
	return end
}

// matchNewline matches \r?\n. It returns the length, or -1.
func matchNewline(s string, pos int) int {
	if pos < len(s) && s[pos] == '\n' {
		return 1
	}
	if pos+1 < len(s) && s[pos] == '\r' && s[pos+1] == '\n' {
		return 2
	}
	return -1
}

// matchGitGraph matches /gitGraph(?:(?=%%)|(?!\S))/.
func matchGitGraph(s string, pos int) int {
	const kw = "gitGraph"
	if !strings.HasPrefix(s[pos:], kw) {
		return -1
	}
	end := pos + len(kw)
	if strings.HasPrefix(s[end:], "%%") || notNonSpace(s, end) {
		return len(kw)
	}
	return -1
}

// matchAccDescr matches
// /[\t ]*accDescr(?:[\t ]*:([^\n\r]*?(?=%%)|[^\n\r]*)|\s*{([^}]*)})/.
func matchAccDescr(s string, pos int) int {
	p := skipBlanks(s, pos)
	if !strings.HasPrefix(s[p:], "accDescr") {
		return -1
	}
	p += len("accDescr")
	if q := skipBlanks(s, p); q < len(s) && s[q] == ':' {
		return lineRest(s, q+1) - pos
	}
	q := skipSpace(s, p)
	if q >= len(s) || s[q] != '{' {
		return -1
	}
	closing := strings.IndexByte(s[q+1:], '}')
	if closing < 0 {
		return -1
	}
	return q + 1 + closing + 1 - pos
}

// matchAccTitle matches /[\t ]*accTitle[\t ]*:(?:[^\n\r]*?(?=%%)|[^\n\r]*)/.
func matchAccTitle(s string, pos int) int {
	p := skipBlanks(s, pos)
	if !strings.HasPrefix(s[p:], "accTitle") {
		return -1
	}
	p = skipBlanks(s, p+len("accTitle"))
	if p >= len(s) || s[p] != ':' {
		return -1
	}
	return lineRest(s, p+1) - pos
}

// matchTitle matches /[\t ]*title(?:[\t ][^\n\r]*?(?=%%)|[\t ][^\n\r]*|)/.
func matchTitle(s string, pos int) int {
	p := skipBlanks(s, pos)
	if !strings.HasPrefix(s[p:], "title") {
		return -1
	}
	p += len("title")
	if p < len(s) && isBlank(s[p]) {
		return lineRest(s, p+1) - pos
	}
	return p - pos
}

// matchInt matches /0|[1-9][0-9]*(?!\.)/. The lookahead belongs to the
// second alternative only, and the digits backtrack: "12." gives "1".
func matchInt(s string, pos int) int {
	if pos >= len(s) {
		return -1
	}
	if s[pos] == '0' {
		return 1
	}
	if s[pos] < '1' || s[pos] > '9' {
		return -1
	}
	end := pos + 1
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end < len(s) && s[end] == '.' {
		// Give one digit back. The next character is then a digit.
		end--
		if end == pos {
			return -1
		}
	}
	return end - pos
}

// matchString matches /"([^"\\]|\\.)*"|'([^'\\]|\\.)*'/. The dot does not
// match a line terminator.
func matchString(s string, pos int) int {
	if pos >= len(s) {
		return -1
	}
	quote := s[pos]
	if quote != '"' && quote != '\'' {
		return -1
	}
	p := pos + 1
	for p < len(s) {
		switch s[p] {
		case quote:
			return p + 1 - pos
		case '\\':
			if p+1 >= len(s) {
				return -1
			}
			r, size := utf8.DecodeRuneInString(s[p+1:])
			if r == '\n' || r == '\r' || r == 0x2028 || r == 0x2029 {
				return -1
			}
			p += 1 + size
		default:
			p++
		}
	}
	return -1
}

// matchWhitespace matches /[\t ]+/.
func matchWhitespace(s string, pos int) int {
	if end := skipBlanks(s, pos); end > pos {
		return end - pos
	}
	return -1
}

// matchYAML matches
// /---[\t ]*\r?\n(?:[\S\s]*?\r?\n)?---(?:\r?\n|(?!\S))/.
func matchYAML(s string, pos int) int {
	if !strings.HasPrefix(s[pos:], "---") {
		return -1
	}
	p := skipBlanks(s, pos+3)
	n := matchNewline(s, p)
	if n < 0 {
		return -1
	}
	p += n
	// closing matches ---(?:\r?\n|(?!\S)) and returns the end, or -1.
	closing := func(at int) int {
		if !strings.HasPrefix(s[at:], "---") {
			return -1
		}
		at += 3
		if n := matchNewline(s, at); n >= 0 {
			return at + n
		}
		if notNonSpace(s, at) {
			return at
		}
		return -1
	}
	// The optional group first: the shortest text that ends with a line
	// break and is followed by the closing.
	for q := p; q < len(s); q++ {
		if n := matchNewline(s, q); n >= 0 {
			if end := closing(q + n); end >= 0 {
				return end - pos
			}
		}
	}
	if end := closing(p); end >= 0 {
		return end - pos
	}
	return -1
}

// matchDirective matches /[\t ]*%%{[\S\s]*?}%%(?:\r?\n|(?!\S))/.
func matchDirective(s string, pos int) int {
	p := skipBlanks(s, pos)
	if !strings.HasPrefix(s[p:], "%%{") {
		return -1
	}
	p += 3
	for {
		i := strings.Index(s[p:], "}%%")
		if i < 0 {
			return -1
		}
		end := p + i + 3
		if n := matchNewline(s, end); n >= 0 {
			return end + n - pos
		}
		if notNonSpace(s, end) {
			return end - pos
		}
		p += i + 1
	}
}

// matchSingleLineComment matches /[\t ]*%%[^\n\r]*/.
func matchSingleLineComment(s string, pos int) int {
	p := skipBlanks(s, pos)
	if !strings.HasPrefix(s[p:], "%%") {
		return -1
	}
	p += 2
	for p < len(s) && !isLineBreak(s[p]) {
		p++
	}
	return p - pos
}

// matchReference matches /\w([-\./\w]*[-\w])?/.
func matchReference(s string, pos int) int {
	if pos >= len(s) || !isWord(s[pos]) {
		return -1
	}
	end := pos + 1
	for end < len(s) && (isWord(s[end]) || s[end] == '-' || s[end] == '.' || s[end] == '/') {
		end++
	}
	// The last character of the group is a dash or a word character.
	for end > pos+1 && (s[end-1] == '.' || s[end-1] == '/') {
		end--
	}
	return end - pos
}

// terminals is the terminal part of the token list, in order.
var terminals = []struct {
	typ   tokenType
	match func(s string, pos int) int
}{
	{tkAccDescr, matchAccDescr},
	{tkAccTitle, matchAccTitle},
	{tkTitle, matchTitle},
	{tkInt, matchInt},
	{tkString, matchString},
	{tkNewline, matchNewline},
	{tkWhitespace, matchWhitespace},
	{tkYAML, matchYAML},
	{tkDirective, matchDirective},
	{tkSingleLineComment, matchSingleLineComment},
	{tkReference, matchReference},
}

// lexError is an error of the lexer or the parser, with the place in the
// input.
type lexError struct {
	msg    string
	offset int
}

// lineColumn returns the line and the column of a byte offset, both from 1.
func lineColumn(s string, offset int) (line, column int) {
	if offset > len(s) {
		offset = len(s)
	}
	line = 1 + strings.Count(s[:offset], "\n")
	column = 1 + utf8.RuneCountInString(s[strings.LastIndexByte(s[:offset], '\n')+1:offset])
	return line, column
}

func (e *lexError) format(s string) error {
	line, column := lineColumn(s, e.offset)
	return fmt.Errorf("Parsing failed: Parse error on line %d, column %d: %s", line, column, e.msg)
}

// nextToken matches one token at pos. It returns the type and the length.
// The length is -1 when no token matches.
func nextToken(s string, pos int) (tokenType, int) {
	for _, k := range keywords {
		n := -1
		if k.typ == tkGitGraph {
			n = matchGitGraph(s, pos)
		} else if strings.HasPrefix(s[pos:], k.text) {
			n = len(k.text)
		}
		if n < 0 {
			continue
		}
		if k.longerAlt {
			if alt := matchReference(s, pos); alt > n {
				return tkReference, alt
			}
		}
		return k.typ, n
	}
	for _, t := range terminals {
		if n := t.match(s, pos); n > 0 {
			return t.typ, n
		}
	}
	return 0, -1
}

// tokenize splits the input into tokens. The hidden tokens (white space,
// YAML, directives and comments) are left out. The last token is EOF.
func tokenize(s string) ([]token, *lexError) {
	var tokens []token
	pos := 0
	for pos < len(s) {
		typ, n := nextToken(s, pos)
		if n < 0 {
			r, _ := utf8.DecodeRuneInString(s[pos:])
			return nil, &lexError{msg: fmt.Sprintf("unexpected character: ->%c<-", r), offset: pos}
		}
		switch typ {
		case tkWhitespace, tkYAML, tkDirective, tkSingleLineComment:
		default:
			tokens = append(tokens, token{typ: typ, image: s[pos : pos+n], offset: pos})
		}
		pos += n
	}
	tokens = append(tokens, token{typ: tkEOF, offset: len(s)})
	return tokens, nil
}
