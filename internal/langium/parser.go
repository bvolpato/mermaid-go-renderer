// Ported from langium 4.4.0, langium/src/parser/langium-parser.ts
// (LangiumParser.consume, LangiumParserErrorMessageProvider) (MIT), from
// chevrotain 13.2.0, chevrotain/src/parse/errors_public.ts (the default error
// messages) (Apache-2.0), and from mermaid 12.2.0, packages/parser/src/parse.ts
// (MermaidParseError) (MIT).
//
// chevrotain has the Apache License 2.0. Its files have no copyright line,
// and the package has no NOTICE file. The part of this file that follows
// chevrotain is a Go translation. It is a changed version of
// parse/errors_public.ts. LICENSE-chevrotain.txt in this directory has the
// license.

package langium

import (
	"fmt"
	"strings"
)

// EOF is the name of the token type at the end of the text.
const EOF = "EOF"

var eofType = &TokenType{Name: EOF, Index: -1}

// Parser is a position in the tokens of a text. A hand-written recursive
// descent parser reads the tokens through it. See the package comment.
type Parser struct {
	tokens []Token
	hidden []Token
	pos    int
}

// Hidden returns the tokens of the Hidden group: comments, directives and
// front matter.
func (p *Parser) Hidden() []Token { return p.hidden }

// LA returns the k-th next token: LA(1) is the token that Next consumes. It
// returns the EOF token after the last token. LA(0) is the token that was
// consumed last.
func (p *Parser) LA(k int) Token {
	i := p.pos + k - 1
	if i < 0 || i >= len(p.tokens) {
		return Token{Type: eofType}
	}
	return p.tokens[i]
}

// At reports whether the next token has one of the token types.
func (p *Parser) At(names ...string) bool {
	next := p.LA(1).Type.Name
	for _, name := range names {
		if next == name {
			return true
		}
	}
	return false
}

// Next consumes the next token and returns it. At the end of the text it
// returns the EOF token and consumes nothing.
func (p *Parser) Next() Token {
	t := p.LA(1)
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return t
}

// Accept consumes the next token when it has the token type, and returns it.
// It returns nil for another token: the element is optional.
func (p *Parser) Accept(name string) *Token {
	if !p.At(name) {
		return nil
	}
	t := p.Next()
	return &t
}

// Skip consumes the next tokens as long as they have the token type, and
// returns their number: a repetition such as NEWLINE*.
func (p *Parser) Skip(name string) int {
	n := 0
	for p.At(name) {
		p.pos++
		n++
	}
	return n
}

// Expect consumes the next token, which must have the token type. For
// another token it returns the error of the real parser.
func (p *Parser) Expect(name string) (Token, error) {
	if !p.At(name) {
		return Token{}, p.errorf("Expecting token of type '%s' but found `%s`.", name, p.LA(1).Image)
	}
	return p.Next(), nil
}

// EOL is the rule EOL of common.langium: NEWLINE+ | EOF.
func (p *Parser) EOL() error {
	if p.At(EOF) {
		return nil
	}
	if p.Skip("NEWLINE") == 0 {
		return p.Unexpected("NEWLINE", EOF)
	}
	return nil
}

// End returns an error when the text has more tokens. The real parser
// reports them after the entry rule returns.
func (p *Parser) End() error {
	if !p.At(EOF) {
		return p.errorf("Expecting token of type 'EOF' but found `%s`.", p.LA(1).Image)
	}
	return nil
}

// Unexpected returns the error for a next token that fits no alternative.
// expected names the token types that fit.
func (p *Parser) Unexpected(expected ...string) error {
	var sb strings.Builder
	sb.WriteString("Expecting: one of these possible Token sequences:\n")
	for i, name := range expected {
		fmt.Fprintf(&sb, "  %d. [%s]\n", i+1, name)
	}
	fmt.Fprintf(&sb, "but found: '%s'", p.LA(1).Image)
	return p.errorf("%s", sb.String())
}

// Errorf returns a parser error at the next token.
func (p *Parser) Errorf(format string, args ...any) error {
	return p.errorf(format, args...)
}

func (p *Parser) errorf(format string, args ...any) error {
	t := p.LA(1)
	// The EOF token of Chevrotain has no position.
	line, column := "?", "?"
	if t.Type != eofType {
		line, column = fmt.Sprint(t.Line), fmt.Sprint(t.Column)
	}
	return &Error{Parser: []string{fmt.Sprintf("Parse error on line %s, column %s: %s", line, column, fmt.Sprintf(format, args...))}}
}
