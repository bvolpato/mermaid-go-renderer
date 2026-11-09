// Ported from mermaid 12.2.0, diagrams/usecase/parser/usecase.tokens.ts (MIT).
//
// The token types, their order and their lexer properties are in
// grammar_gen.go, which is taken from the real lexer object. This file has
// the patterns: one matcher for each token type. A matcher gets the text as
// UTF-16 code units, as a JavaScript regular expression without the u flag
// sees it, and returns the length of the match at an offset, or -1.

package usecase

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// tokenType is a token type of the vocabulary.
type tokenType int

// patternConfig is IPatternConfig of chevrotain: a pattern of a lexer mode.
type patternConfig struct {
	typ tokenType
	// skipped is true for a token of the group Lexer.SKIPPED.
	skipped bool
	// canLineTerminator is true when the lexer looks for line terminators
	// in the image of the token.
	canLineTerminator bool
	// longerAlt has the token types of longer_alt.
	longerAlt []tokenType
	// push is the mode of push_mode, or "".
	push string
	// pop is true for pop_mode.
	pop bool
}

// matcher returns the length of the match of a pattern at an offset of the
// text, or -1 when the pattern does not match there.
type matcher func(text []rune, offset int) int

func isWordChar(c rune) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

func isDigit(c rune) bool { return c >= '0' && c <= '9' }

func isLetter(c rune) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func isHexDigit(c rune) bool {
	return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// isBlank is the class [\t ].
func isBlank(c rune) bool { return c == '\t' || c == ' ' }

func isLineBreak(c rune) bool { return c == '\n' || c == '\r' }

// at returns the code unit at an index, or -1 past the end of the text.
func at(text []rune, i int) rune {
	if i < 0 || i >= len(text) {
		return -1
	}
	return text[i]
}

// hasPrefixAt reports whether the text has the ASCII string s at an offset.
func hasPrefixAt(text []rune, offset int, s string) bool {
	if offset < 0 || offset+len(s) > len(text) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if text[offset+i] != rune(s[i]) {
			return false
		}
	}
	return true
}

// literal is a pattern that is one fixed ASCII string.
func literal(s string) matcher {
	return func(text []rune, offset int) int {
		if hasPrefixAt(text, offset, s) {
			return len(s)
		}
		return -1
	}
}

// literalIgnoreCase is a fixed string with the i flag. Without the u flag
// a character outside ASCII never equals an ASCII letter.
func literalIgnoreCase(s string) matcher {
	return func(text []rune, offset int) int {
		if offset < 0 || offset+len(s) > len(text) {
			return -1
		}
		for i := 0; i < len(s); i++ {
			c := text[offset+i]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != rune(s[i]) {
				return -1
			}
		}
		return len(s)
	}
}

// oneOf is a pattern that is a class of single ASCII characters.
func oneOf(chars string) matcher {
	return func(text []rune, offset int) int {
		c := at(text, offset)
		for i := 0; i < len(chars); i++ {
			if c == rune(chars[i]) {
				return 1
			}
		}
		return -1
	}
}

// countWhile returns the number of code units from an offset that are in a
// class.
func countWhile(text []rune, offset int, class func(rune) bool) int {
	n := 0
	for offset+n < len(text) && class(text[offset+n]) {
		n++
	}
	return n
}

// isIndentedLineStart reports whether only spaces and tabs are between the
// start of the line and the offset.
func isIndentedLineStart(text []rune, offset int) bool {
	for index := offset - 1; index >= 0; index-- {
		character := text[index]
		if character == '\n' || character == '\r' {
			return true
		}
		if character != ' ' && character != '\t' {
			return false
		}
	}
	return true
}

// matchMarkdownString is the pattern of MARKDOWN_STRING: "` ... `".
func matchMarkdownString(text []rune, offset int) int {
	if at(text, offset) != '"' || at(text, offset+1) != '`' {
		return -1
	}
	for index := offset + 2; index < len(text)-1; index++ {
		if text[index] == '`' && text[index+1] == '"' {
			return index + 2 - offset
		}
	}
	return -1
}

// matchUnclosedMarkdownString is /"`[^]*/.
func matchUnclosedMarkdownString(text []rune, offset int) int {
	if at(text, offset) != '"' || at(text, offset+1) != '`' {
		return -1
	}
	return len(text) - offset
}

// matchComment is the pattern of COMMENT: %% at the start of a line, up to
// the end of the line.
func matchComment(text []rune, offset int) int {
	if at(text, offset) != '%' || at(text, offset+1) != '%' {
		return -1
	}
	for index := offset - 1; index >= 0; index-- {
		character := text[index]
		if character == '\n' || character == '\r' {
			break
		}
		if character != ' ' && character != '\t' {
			return -1
		}
	}
	end := offset + 2
	for end < len(text) && text[end] != '\n' && text[end] != '\r' {
		end++
	}
	return end - offset
}

// matchNewLine is /\r\n|\n|\r/.
func matchNewLine(text []rune, offset int) int {
	switch at(text, offset) {
	case '\r':
		if at(text, offset+1) == '\n' {
			return 2
		}
		return 1
	case '\n':
		return 1
	}
	return -1
}

// matchAccessibilityLine is matchAccessibilityLine with the pattern
// /^<keyword>[\t ]*:[^\n\r]*/.
func matchAccessibilityLine(keyword string) matcher {
	return func(text []rune, offset int) int {
		if !isIndentedLineStart(text, offset) {
			return -1
		}
		if !hasPrefixAt(text, offset, keyword) {
			return -1
		}
		i := offset + len(keyword)
		i += countWhile(text, i, isBlank)
		if at(text, i) != ':' {
			return -1
		}
		i++
		i += countWhile(text, i, func(c rune) bool { return !isLineBreak(c) })
		return i - offset
	}
}

// matchAccDescrBlock is the pattern of ACC_DESCR_BLOCK: accDescr { ... }.
func matchAccDescrBlock(text []rune, offset int) int {
	if !isIndentedLineStart(text, offset) {
		return -1
	}
	// /^accDescr[\t ]*{/
	if !hasPrefixAt(text, offset, "accDescr") {
		return -1
	}
	i := offset + len("accDescr")
	i += countWhile(text, i, isBlank)
	if at(text, i) != '{' {
		return -1
	}
	i++
	// text.indexOf('}', offset + opening[0].length)
	for end := i; end < len(text); end++ {
		if text[end] == '}' {
			return end + 1 - offset
		}
	}
	return -1
}

// matchJsonDeclarationStart is /json[\t ]+\w+[\t ]*@[\t ]*(?={)/.
func matchJsonDeclarationStart(text []rune, offset int) int {
	if !hasPrefixAt(text, offset, "json") {
		return -1
	}
	i := offset + 4
	n := countWhile(text, i, isBlank)
	if n == 0 {
		return -1
	}
	i += n
	n = countWhile(text, i, isWordChar)
	if n == 0 {
		return -1
	}
	i += n
	i += countWhile(text, i, isBlank)
	if at(text, i) != '@' {
		return -1
	}
	i++
	i += countWhile(text, i, isBlank)
	if at(text, i) != '{' {
		return -1
	}
	return i - offset
}

// matchJsonObject is the pattern of JSON_OBJECT_LITERAL: a brace-balanced
// object, with the braces inside strings not counted.
func matchJsonObject(text []rune, offset int) int {
	if at(text, offset) != '{' {
		return -1
	}

	depth := 0
	quoted := false
	escaped := false
	for index := offset; index < len(text); index++ {
		character := text[index]
		if quoted {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '"' {
				quoted = false
			}
			continue
		}
		if character == '"' {
			quoted = true
		} else if character == '{' {
			depth++
		} else if character == '}' {
			depth--
			if depth == 0 {
				return index + 1 - offset
			}
		}
	}

	// No match makes Chevrotain locate the lexer error at this unmatched opening brace.
	return -1
}

// matchUnclosedJsonObject is /{[^]*/.
func matchUnclosedJsonObject(text []rune, offset int) int {
	if at(text, offset) != '{' {
		return -1
	}
	return len(text) - offset
}

// matchStereotypeText is the pattern of STEREOTYPE_TEXT: the text up to >>
// on the same line, when it is not blank.
func matchStereotypeText(text []rune, offset int) int {
	if offset > len(text) {
		return -1
	}
	// text.indexOf('>>', offset)
	end := -1
	for i := offset; i+1 < len(text); i++ {
		if text[i] == '>' && text[i+1] == '>' {
			end = i
			break
		}
	}
	if end == -1 {
		return -1
	}
	for _, c := range text[offset:end] {
		if isLineBreak(c) {
			return -1
		}
	}
	image := js.FromUTF16Units(text[offset:end])
	if js.TrimSpace(image) == "" {
		return -1
	}
	return end - offset
}

// matchUnclosedStereotypeText is /[^\n\r]+/.
func matchUnclosedStereotypeText(text []rune, offset int) int {
	n := countWhile(text, offset, func(c rune) bool { return !isLineBreak(c) })
	if n == 0 {
		return -1
	}
	return n
}

// countDashes returns the number of '-' from an offset.
func countDashes(text []rune, offset int) int {
	return countWhile(text, offset, func(c rune) bool { return c == '-' })
}

// matchForwardSolid is /--+>/.
func matchForwardSolid(text []rune, offset int) int {
	n := countDashes(text, offset)
	if n < 2 || at(text, offset+n) != '>' {
		return -1
	}
	return n + 1
}

// matchBackwardSolid is /<--+/.
func matchBackwardSolid(text []rune, offset int) int {
	if at(text, offset) != '<' {
		return -1
	}
	n := countDashes(text, offset+1)
	if n < 2 {
		return -1
	}
	return n + 1
}

// matchMarkerlessSolid is /--+/.
func matchMarkerlessSolid(text []rune, offset int) int {
	n := countDashes(text, offset)
	if n < 2 {
		return -1
	}
	return n
}

// matchHashColor is /#[\dA-Fa-f]+/.
func matchHashColor(text []rune, offset int) int {
	if at(text, offset) != '#' {
		return -1
	}
	n := countWhile(text, offset+1, isHexDigit)
	if n == 0 {
		return -1
	}
	return n + 1
}

// matchPlainString is /"[^\n\r"]*"|'[^\n\r']*'/.
func matchPlainString(text []rune, offset int) int {
	quote := at(text, offset)
	if quote != '"' && quote != '\'' {
		return -1
	}
	i := offset + 1
	i += countWhile(text, i, func(c rune) bool { return !isLineBreak(c) && c != quote })
	if at(text, i) != quote {
		return -1
	}
	return i + 1 - offset
}

// matchCssIdentifier is /--[A-Z_a-z][\w-]*|[A-Z_a-z]\w*(?:-\w+)+/.
func matchCssIdentifier(text []rune, offset int) int {
	isStart := func(c rune) bool { return isLetter(c) || c == '_' }
	// --[A-Z_a-z][\w-]*
	if hasPrefixAt(text, offset, "--") && isStart(at(text, offset+2)) {
		i := offset + 3
		i += countWhile(text, i, func(c rune) bool { return isWordChar(c) || c == '-' })
		return i - offset
	}
	// [A-Z_a-z]\w*(?:-\w+)+
	if !isStart(at(text, offset)) {
		return -1
	}
	i := offset + 1
	i += countWhile(text, i, isWordChar)
	groups := 0
	for at(text, i) == '-' && isWordChar(at(text, i+1)) {
		i++
		i += countWhile(text, i, isWordChar)
		groups++
	}
	if groups == 0 {
		return -1
	}
	return i - offset
}

// matchIdentifier is /\w+/.
func matchIdentifier(text []rune, offset int) int {
	n := countWhile(text, offset, isWordChar)
	if n == 0 {
		return -1
	}
	return n
}

// matchNumber is /(?:\d+\.\d+|\d+|\.\d+)(?:[A-Za-z]+)?/.
func matchNumber(text []rune, offset int) int {
	i := offset
	if n := countWhile(text, i, isDigit); n > 0 {
		i += n
		// \d+\.\d+, or \d+ alone.
		if at(text, i) == '.' && isDigit(at(text, i+1)) {
			i++
			i += countWhile(text, i, isDigit)
		}
	} else if at(text, i) == '.' && isDigit(at(text, i+1)) {
		i++
		i += countWhile(text, i, isDigit)
	} else {
		return -1
	}
	i += countWhile(text, i, isLetter)
	return i - offset
}

// matchLabelSymbol is /[^\t\n\r !-~]+/: a run of characters outside
// printable ASCII.
func matchLabelSymbol(text []rune, offset int) int {
	n := countWhile(text, offset, func(c rune) bool {
		return c != '\t' && c != '\n' && c != '\r' && c != ' ' && !(c >= '!' && c <= '~')
	})
	if n == 0 {
		return -1
	}
	return n
}

// matchWhiteSpace is /[\t ]+/.
func matchWhiteSpace(text []rune, offset int) int {
	n := countWhile(text, offset, isBlank)
	if n == 0 {
		return -1
	}
	return n
}

// matchers has the pattern of each token type that a lexer mode lists. The
// categories LABEL_TEXT and WORD have no pattern (Lexer.NA).
var matchers = map[tokenType]matcher{
	tokHws:                       matchWhiteSpace,
	tokMarkdownString:            matchMarkdownString,
	tokUnclosedMarkdownString:    matchUnclosedMarkdownString,
	tokComment:                   matchComment,
	tokNewline:                   matchNewLine,
	tokAccDescrBlock:             matchAccDescrBlock,
	tokAccTitleLine:              matchAccessibilityLine("accTitle"),
	tokAccDescrLine:              matchAccessibilityLine("accDescr"),
	tokJsonDeclarationStart:      matchJsonDeclarationStart,
	tokJsonObjectLiteral:         matchJsonObject,
	tokUnclosedJsonObjectLiteral: matchUnclosedJsonObject,

	tokUsecase:        literal("usecase-beta"),
	tokActor:          literal("actor"),
	tokSystemBoundary: literal("systemBoundary"),
	tokEnd:            literal("end"),
	tokDirection:      literal("direction"),
	tokTd:             literal("TD"),
	tokTb:             literal("TB"),
	tokBt:             literal("BT"),
	tokLr:             literal("LR"),
	tokRl:             literal("RL"),
	tokNote:           literal("note"),
	tokFor:            literal("for"),
	tokJson:           literal("json"),
	tokClassDef:       literal("classDef"),
	tokClass:          literal("class"),
	tokStyle:          literal("style"),
	tokInclude:        literalIgnoreCase("include"),
	tokExtend:         literalIgnoreCase("extend"),
	tokTrue:           literal("true"),
	tokFalse:          literal("false"),

	tokGeneralization:         literal("--|>"),
	tokDependencyArrow:        literal("..>"),
	tokStereotypeStart:        literal("<<"),
	tokStereotypeEnd:          literal(">>"),
	tokStereotypeText:         matchStereotypeText,
	tokUnclosedStereotypeText: matchUnclosedStereotypeText,
	tokClassSeparator:         literal(":::"),
	tokForwardSolid:           matchForwardSolid,
	tokBackwardSolid:          matchBackwardSolid,
	tokForwardCircle:          literal("--o"),
	tokBackwardCircle:         literal("o--"),
	tokForwardCross:           literal("--x"),
	tokBackwardCross:          literal("x--"),
	tokMarkerlessSolid:        matchMarkerlessSolid,
	tokMetadataStart:          literal("@{"),
	tokAt:                     literal("@"),
	tokLbrace:                 literal("{"),
	tokRbrace:                 literal("}"),
	tokLbracket:               literal("["),
	tokRbracket:               literal("]"),
	tokLparen:                 literal("("),
	tokRparen:                 literal(")"),
	tokComma:                  literal(","),
	tokColon:                  literal(":"),

	tokHashColor:        matchHashColor,
	tokPlainString:      matchPlainString,
	tokCssIdentifier:    matchCssIdentifier,
	tokIdentifier:       matchIdentifier,
	tokNumber:           matchNumber,
	tokCssEscapedComma:  literal(`\,`),
	tokDash:             literal("-"),
	tokDot:              literal("."),
	tokPercent:          literal("%"),
	tokCssPunctuation:   oneOf("!#$&*+/=?^_|~"),
	tokLabelPunctuation: oneOf(";<>\\`"),
	tokLabelSymbol:      matchLabelSymbol,
}

// categoryMatchesMap is categoryMatchesMap of chevrotain: for a category,
// the token types that it matches.
var categoryMatchesMap = func() (m [numTokenTypes][numTokenTypes]bool) {
	for category, types := range categoryMatches {
		for _, t := range types {
			m[category][t] = true
		}
	}
	return m
}()

// tokenMatcher is tokenStructuredMatcher(tokInstance, tokConstructor): a
// token matches its own type and the categories of its type.
func tokenMatcher(tok *token, typ tokenType) bool {
	return tok.typ == typ || categoryMatchesMap[typ][tok.typ]
}
