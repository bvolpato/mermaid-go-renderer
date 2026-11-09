// Ported from chevrotain 13.2.0, chevrotain/src/scan/lexer_public.ts
// (Lexer.tokenizeInternal) (Apache-2.0), and from langium 4.4.0,
// langium/src/parser/lexer.ts (DefaultLexer) and
// langium/src/parser/token-builder.ts (DefaultTokenBuilder) (MIT).
//
// chevrotain has the Apache License 2.0. Its files have no copyright line,
// and the package has no NOTICE file. The part of this file that follows
// chevrotain is a Go translation. It is a changed version of
// scan/lexer_public.ts. LICENSE-chevrotain.txt in this directory has the
// license.

package langium

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/dlclark/regexp2/v2"
)

// Group says what the lexer does with the tokens of a token type.
type Group uint8

const (
	// Default tokens go to the parser.
	Default Group = iota
	// Skipped tokens are dropped: the group Lexer.SKIPPED of Chevrotain.
	Skipped
	// Hidden tokens go to LexResult.Hidden: the group "hidden" of Langium.
	Hidden
)

// tokenSpec is one token type in the generated vocabulary.
type tokenSpec struct {
	name string
	// literal is the pattern of a token type that Chevrotain matches as a
	// string.
	literal string
	// source is the JavaScript source of a regular expression pattern.
	// pattern is the same expression for regexp2 without options: every
	// character class is a set of explicit UTF-16 code units.
	source  string
	pattern string
	group   Group
	// longerAlt holds the indexes of the token types of LONGER_ALT.
	longerAlt []int
}

// TokenType is one token type of a grammar.
type TokenType struct {
	// Name is the keyword text for a keyword, and the name of the terminal
	// rule for a terminal.
	Name string
	// Index is the position in the vocabulary. The lexer tries the token
	// types in this order.
	Index int
	Group Group
	// Literal is the text of a token type with a string pattern.
	Literal string
	// Source is the JavaScript source of a regular expression pattern.
	Source string
	// LongerAlt holds the token types that win over this one when they
	// match a longer text.
	LongerAlt []*TokenType

	literal []rune
	re      *regexp2.Regexp
}

// match is the test of one pattern at an offset: the length of the match in
// code units, or -1.
func (t *TokenType) match(units []rune, offset int) int {
	if t.re == nil {
		n := len(t.literal)
		if offset+n > len(units) {
			return -1
		}
		for i, u := range t.literal {
			if units[offset+i] != u {
				return -1
			}
		}
		return n
	}
	length := -1
	// The pattern starts with \G, so a match starts at offset. An error is a
	// timeout, and no timeout is set.
	_ = t.re.FindRunesMatches(units, offset, func(v regexp2.MatchView) bool {
		_, length, _ = v.GroupRange(0)
		return false
	})
	return length
}

// Token is one token of the text.
type Token struct {
	Type *TokenType
	// Image is the text of the token.
	Image string
	// Offset is the start of the token in UTF-16 code units.
	Offset int
	// Line and Column are the start of the token. Both count from 1, and
	// Column counts code units. They are 0 for the EOF token.
	Line, Column int
}

// LexError is a part of the text that no token type matches.
type LexError struct {
	Offset, Length int
	Line, Column   int
	Message        string
}

// LexResult is the result of Lexer.Tokenize.
type LexResult struct {
	// Tokens holds the tokens that the parser sees.
	Tokens []Token
	// Hidden holds the tokens of the Hidden group.
	Hidden []Token
	Errors []LexError
}

// Lexer is the lexer of one grammar. A Lexer is safe for concurrent use.
type Lexer struct {
	// Grammar is the name of the grammar.
	Grammar string
	types   []*TokenType
	byName  map[string]*TokenType
}

var (
	lexersMu sync.Mutex
	lexers   = map[string]*Lexer{}
)

// Grammars returns the names of the grammars that have a lexer.
func Grammars() []string {
	names := make([]string, 0, len(vocabularies))
	for name := range vocabularies {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// NewLexer returns the lexer of a grammar. The name is the diagram type that
// parse(diagramType, text) of the parser package takes, for example "pie".
func NewLexer(grammar string) (*Lexer, error) {
	lexersMu.Lock()
	defer lexersMu.Unlock()
	if l, ok := lexers[grammar]; ok {
		return l, nil
	}
	specs, ok := vocabularies[grammar]
	if !ok {
		return nil, fmt.Errorf("langium: no grammar %q", grammar)
	}
	l := &Lexer{Grammar: grammar, byName: map[string]*TokenType{}}
	for i, spec := range specs {
		t := &TokenType{Name: spec.name, Index: i, Group: spec.group, Literal: spec.literal, Source: spec.source}
		if spec.pattern == "" {
			t.literal = js.UTF16Units(spec.literal)
		} else {
			// \G anchors the match at the start offset, as the sticky flag
			// does. The backtracking stack has no limit, as in V8.
			re, err := regexp2.Compile(`\G(?:`+spec.pattern+`)`, regexp2.None, regexp2.OptionMaxBacktrackingStackSize(-1))
			if err != nil {
				return nil, fmt.Errorf("langium: grammar %s, token %s: %w", grammar, spec.name, err)
			}
			t.re = re
		}
		l.types = append(l.types, t)
		l.byName[t.Name] = t
	}
	for i, spec := range specs {
		for _, alt := range spec.longerAlt {
			l.types[i].LongerAlt = append(l.types[i].LongerAlt, l.types[alt])
		}
	}
	lexers[grammar] = l
	return l, nil
}

// MustLexer is NewLexer for a package variable. It panics when the grammar
// does not exist.
func MustLexer(grammar string) *Lexer {
	l, err := NewLexer(grammar)
	if err != nil {
		panic(err)
	}
	return l
}

// TokenTypes returns the token types in the order in which the lexer tries
// them.
func (l *Lexer) TokenTypes() []*TokenType {
	return append([]*TokenType(nil), l.types...)
}

// TokenType returns the token type with the name, or nil.
func (l *Lexer) TokenType(name string) *TokenType {
	return l.byName[name]
}

// lineTerminators counts the matches of the lineTerminatorsPattern of
// Chevrotain, /\n|\r\n?/g, in image. last is the offset of the last code
// unit of the last match.
func lineTerminators(image []rune) (count, last int) {
	for i := 0; i < len(image); i++ {
		switch image[i] {
		case '\n':
			count++
			last = i
		case '\r':
			if i+1 < len(image) && image[i+1] == '\n' {
				i++
			}
			count++
			last = i
		}
	}
	return count, last
}

// Tokenize is DefaultLexer.tokenize: Lexer.tokenizeInternal of Chevrotain
// with one mode and with error recovery.
func (l *Lexer) Tokenize(text string) *LexResult {
	units := js.UTF16Units(text)
	orgLength := len(units)
	result := &LexResult{}
	offset := 0
	line, column := 1, 1

	for offset < orgLength {
		imageLength := -1
		var currConfig *TokenType
		for _, config := range l.types {
			imageLength = config.match(units, offset)
			if imageLength == -1 {
				continue
			}
			currConfig = config
			// A longer alternative wins when it matches more text.
			for _, alt := range config.LongerAlt {
				if altLength := alt.match(units, offset); altLength > 0 && altLength > imageLength {
					imageLength = altLength
					currConfig = alt
					break
				}
			}
			break
		}
		if imageLength == 0 {
			// Chevrotain does not leave this position: a pattern that
			// matches nothing is a defect of the grammar. No grammar of
			// mermaid has one. Stop with an error.
			result.Errors = append(result.Errors, LexError{
				Offset: offset, Line: line, Column: column,
				Message: fmt.Sprintf("token type %s matches an empty text at offset: %d", currConfig.Name, offset),
			})
			break
		}
		if imageLength != -1 {
			image := units[offset : offset+imageLength]
			if currConfig.Group != Skipped {
				token := Token{Type: currConfig, Image: js.FromUTF16Units(image), Offset: offset, Line: line, Column: column}
				if currConfig.Group == Hidden {
					result.Hidden = append(result.Hidden, token)
				} else {
					result.Tokens = append(result.Tokens, token)
				}
			}
			if count, last := lineTerminators(image); count != 0 {
				line += count
				column = imageLength - last
			} else {
				column += imageLength
			}
			offset += imageLength
			continue
		}

		// No token type matches. Skip to the next offset where one does.
		errorStartOffset := offset
		errorLine, errorColumn := line, column
		foundResyncPoint := false
		for !foundResyncPoint && offset < orgLength {
			offset++
			for _, config := range l.types {
				if config.match(units, offset) != -1 {
					foundResyncPoint = true
					break
				}
			}
		}
		errLength := offset - errorStartOffset
		column += errLength
		result.Errors = append(result.Errors, LexError{
			Offset: errorStartOffset,
			Length: errLength,
			Line:   errorLine,
			Column: errorColumn,
			Message: fmt.Sprintf("unexpected character: ->%s<- at offset: %d, skipped %d characters.",
				js.FromUTF16Units(units[errorStartOffset:errorStartOffset+1]), errorStartOffset, errLength),
		})
	}
	return result
}

// Error is the error that parse of the parser package throws: the class
// MermaidParseError.
type Error struct {
	// Lexer holds the lexer errors and Parser the parser errors, as lines
	// "Lexer error on line 1, column 5: ..." and "Parse error on line ...".
	Lexer  []string
	Parser []string
}

func (e *Error) Error() string {
	return "Parsing failed: " + strings.Join(e.Lexer, "\n") + " " + strings.Join(e.Parser, "\n")
}

// Parse tokenizes the text and returns a Parser at the first token. It
// returns an error when the text has a lexer error, because mermaid then
// rejects the text whatever the parser makes of it.
func (l *Lexer) Parse(text string) (*Parser, error) {
	result := l.Tokenize(text)
	if len(result.Errors) > 0 {
		e := &Error{}
		for _, le := range result.Errors {
			e.Lexer = append(e.Lexer, fmt.Sprintf("Lexer error on line %d, column %d: %s", le.Line, le.Column, le.Message))
		}
		return nil, e
	}
	return &Parser{tokens: result.Tokens, hidden: result.Hidden}, nil
}
