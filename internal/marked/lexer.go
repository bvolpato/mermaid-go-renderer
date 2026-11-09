// Package marked is a port of the lexer of marked 16.3.0, the markdown
// parser that mermaid 12.2.0 bundles. Mermaid calls marked.lexer(text) with
// the default options, and it reads the token tree. The port has the lexer,
// the tokenizer and the rules for these options. It has no parser, no
// renderer, no extensions and no hooks.
//
// Ported from marked/src/Lexer.ts and marked/src/marked.ts (marked 16.3.0).
package marked

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// inlineQueueItem is one entry of Lexer.inlineQueue: a source, and the
// token array that receives its inline tokens.
type inlineQueueItem struct {
	src    string
	tokens *[]*Token
}

// lexer is the Lexer class.
type lexer struct {
	tokens []*Token
	// links is tokens.links: the link reference definitions.
	links map[string]link
	state struct {
		inLink     bool
		inRawBlock bool
		top        bool
	}

	tokenizer   tokenizer
	inlineQueue []inlineQueueItem
}

// thrown is the panic value of lexer.throw. Lexer recovers it.
type thrown struct{ err error }

// throw stops the lexer with an error, as a JavaScript throw does.
func (l *lexer) throw(message string) {
	panic(thrown{errors.New("marked: " + message)})
}

func newLexer() *lexer {
	l := &lexer{
		tokens: []*Token{},
		links:  map[string]link{},
	}
	l.state.top = true
	l.tokenizer = tokenizer{rules: getRules(), lexer: l}
	return l
}

// Lexer is marked.lexer(src) with the default options: gfm is true, breaks
// and pedantic are false. It returns the block tokens, and each token holds
// its inline tokens.
//
// marked throws an error when no rule matches the input. The rules match
// each input, so this error is not expected. A surrogate without its
// partner in src has the three-byte form that internal/js defines. Each
// other byte that is not valid UTF-8 becomes U+FFFD.
func Lexer(src string) ([]*Token, error) {
	if plainText(src) {
		return plainTokens(src), nil
	}
	return lexSlow(src)
}

// lexSlow runs the lexer of marked.
func lexSlow(src string) (tokens []*Token, err error) {
	defer func() {
		if r := recover(); r != nil {
			t, ok := r.(thrown)
			if !ok {
				panic(r)
			}
			tokens, err = nil, t.err
		}
	}()
	return newLexer().lex(units(src)), nil
}

// plainSpecial holds the ASCII characters that plainText does not accept.
var plainSpecial = func() (t [256]bool) {
	for _, c := range []byte("\n\r\\<[`*~@") {
		t[c] = true
	}
	return t
}()

func isAlphanumeric(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// plainText reports whether src has no character that has a meaning in
// markdown, so that the lexer returns one paragraph with one text token
// that holds src: see plainTokens. Most labels are such text, and for them
// Lexer does not run a rule.
//
// The proof has two parts. src has no line feed, so only the block rules
// that match at the start of src count:
//
//   - newline and code need a space or a tab at the start.
//   - fences, hr, blockquote, list, html and def need, after zero to three
//     spaces, one of ` ~ - _ * > + < [ or digits with a dot or a
//     parenthesis after them. heading needs #.
//   - table and lheading need a line feed.
//
// So the paragraph rule takes all of src. In the inline text of the
// paragraph:
//
//   - escape, tag, link, reflink, codespan, del and autolink need one of
//     \ < [ ` ~ at their start, or "![". br needs a line feed.
//   - url needs "://" after a scheme, or "www." in any case, or an @.
//   - The text rule stops before one of \ < ! [ ` * ~ _, before "http://",
//     "https://", "ftp://" and "www.", before an email address, which has
//     an @, and before spaces that a line feed follows.
//   - emStrong needs * or _ at its start.
//
// So the text rule stops only before a "!", before a "_" and at the end. No
// rule other than the text rule matches at a "!" that no "[" follows.
// plainText accepts a "_" only between two ASCII letters or digits, as in
// snake_case. Then the text token before it ends with a letter or a digit,
// which is prevChar, and one letter or digit follows the "_". For this
// case emStrong returns no token: "_ can't be between two alphanumerics".
// So the text rule continues at each stop. The lexer joins text tokens
// that follow each other, so the result is one text token.
func plainText(src string) bool {
	if src == "" {
		return false
	}
	switch c := src[0]; {
	case c == ' ', c == '\t', c == '#', c == '>', c == '-', c == '+':
		return false
	case c >= '0' && c <= '9':
		i := 1
		for i < len(src) && src[i] >= '0' && src[i] <= '9' {
			i++
		}
		if i < len(src) && (src[i] == '.' || src[i] == ')') {
			return false
		}
	}
	ascii := true
	for i := 0; i < len(src); i++ {
		c := src[i]
		if plainSpecial[c] {
			return false
		}
		switch {
		case c == '_':
			if i == 0 || i+1 == len(src) || !isAlphanumeric(src[i-1]) || !isAlphanumeric(src[i+1]) {
				return false
			}
		case c == ':':
			if strings.HasPrefix(src[i:], "://") {
				return false
			}
		case c == 'w', c == 'W':
			if i+3 < len(src) && src[i+1]|0x20 == 'w' && src[i+2]|0x20 == 'w' && src[i+3] == '.' {
				return false
			}
		case c >= utf8.RuneSelf:
			ascii = false
		}
	}
	// The lexer of marked reads text that is not valid UTF-8, for example a
	// surrogate in the three-byte form, so that each form gives one result.
	return ascii || utf8.ValidString(src)
}

// plainTokens returns the tokens of a text that plainText accepts.
func plainTokens(src string) []*Token {
	return []*Token{{
		Type: "paragraph",
		Raw:  src,
		Text: src,
		Tokens: []*Token{{
			Type:  "text",
			Raw:   src,
			Text:  src,
			shape: shapeInlineText,
		}},
		shape: shapeTextTokens,
	}}
}

// lex is Lexer.lex: the preprocessing.
func (l *lexer) lex(src []rune) []*Token {
	// src = src.replace(other.carriageReturn, '\n'), where carriageReturn is
	// /\r\n|\r/g.
	if indexOf(src, '\r') >= 0 {
		out := make([]rune, 0, len(src))
		for i := 0; i < len(src); i++ {
			if src[i] == '\r' {
				if i+1 < len(src) && src[i+1] == '\n' {
					i++
				}
				out = append(out, '\n')
				continue
			}
			out = append(out, src[i])
		}
		src = out
	}

	l.tokens = l.blockTokens(src, l.tokens, false)

	for i := 0; i < len(l.inlineQueue); i++ {
		next := l.inlineQueue[i]
		*next.tokens = l.inlineTokens(units(next.src), *next.tokens)
	}
	l.inlineQueue = nil

	return l.tokens
}

func last(tokens []*Token) *Token {
	if len(tokens) == 0 {
		return nil
	}
	return tokens[len(tokens)-1]
}

// appendRaw adds raw to the raw of the last token, after a line feed when
// the last token does not end with one:
//
//	lastToken.raw += (lastToken.raw.endsWith('\n') ? '' : '\n') + raw
func appendRaw(lastToken *Token, raw string) {
	if !strings.HasSuffix(lastToken.Raw, "\n") {
		lastToken.Raw += "\n"
	}
	lastToken.Raw += raw
}

// setLastInlineSrc is this.inlineQueue.at(-1)!.src = src.
func (l *lexer) setLastInlineSrc(src string) {
	if len(l.inlineQueue) == 0 {
		l.throw("TypeError: Cannot set properties of undefined (setting 'src')")
	}
	l.inlineQueue[len(l.inlineQueue)-1].src = src
}

// popInline is this.inlineQueue.pop().
func (l *lexer) popInline() {
	if len(l.inlineQueue) > 0 {
		l.inlineQueue = l.inlineQueue[:len(l.inlineQueue)-1]
	}
}

// consume is src.substring(token.raw.length) for a token that the
// tokenizer has just returned.
func (l *lexer) consume(src []rune, token *Token) []rune {
	return substringFrom(src, token.n)
}

// blockTokens is Lexer.blockTokens: the block lexing. It returns the
// tokens array, which can be a new slice.
func (l *lexer) blockTokens(src []rune, tokens []*Token, lastParagraphClipped bool) []*Token {
	for len(src) > 0 {
		var token *Token

		// newline
		if token = l.tokenizer.space(src); token != nil {
			src = l.consume(src, token)
			lastToken := last(tokens)
			if token.n == 1 && lastToken != nil {
				// if there's a single \n as a spacer, it's terminating the last line,
				// so move it there so that we don't get unnecessary paragraph tags
				lastToken.Raw += "\n"
			} else {
				tokens = append(tokens, token)
			}
			continue
		}

		// code
		if token = l.tokenizer.code(src); token != nil {
			src = l.consume(src, token)
			lastToken := last(tokens)
			// An indented code block cannot interrupt a paragraph.
			if lastToken != nil && (lastToken.Type == "paragraph" || lastToken.Type == "text") {
				appendRaw(lastToken, token.Raw)
				lastToken.Text += "\n" + token.Text
				l.setLastInlineSrc(lastToken.Text)
			} else {
				tokens = append(tokens, token)
			}
			continue
		}

		// fences
		if token = l.tokenizer.fences(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// heading
		if token = l.tokenizer.heading(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// hr
		if token = l.tokenizer.hr(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// blockquote
		if token = l.tokenizer.blockquote(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// list
		if token = l.tokenizer.list(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// html
		if token = l.tokenizer.html(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// def
		if token = l.tokenizer.def(src); token != nil {
			src = l.consume(src, token)
			lastToken := last(tokens)
			if lastToken != nil && (lastToken.Type == "paragraph" || lastToken.Type == "text") {
				appendRaw(lastToken, token.Raw)
				lastToken.Text += "\n" + token.Raw
				l.setLastInlineSrc(lastToken.Text)
			} else if _, ok := l.links[token.Tag]; !ok {
				l.links[token.Tag] = link{
					href:     token.Href,
					title:    token.Title,
					hasTitle: token.HasTitle,
				}
				tokens = append(tokens, token)
			}
			continue
		}

		// table (gfm)
		if token = l.tokenizer.table(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// lheading
		if token = l.tokenizer.lheading(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// top-level paragraph
		if l.state.top {
			if token = l.tokenizer.paragraph(src); token != nil {
				lastToken := last(tokens)
				if lastParagraphClipped && lastToken != nil && lastToken.Type == "paragraph" {
					appendRaw(lastToken, token.Raw)
					lastToken.Text += "\n" + token.Text
					l.popInline()
					l.setLastInlineSrc(lastToken.Text)
				} else {
					tokens = append(tokens, token)
				}
				// There are no extensions, so cutSrc is src.
				lastParagraphClipped = false
				src = l.consume(src, token)
				continue
			}
		}

		// text
		if token = l.tokenizer.text(src); token != nil {
			src = l.consume(src, token)
			lastToken := last(tokens)
			if lastToken != nil && lastToken.Type == "text" {
				appendRaw(lastToken, token.Raw)
				lastToken.Text += "\n" + token.Text
				l.popInline()
				l.setLastInlineSrc(lastToken.Text)
			} else {
				tokens = append(tokens, token)
			}
			continue
		}

		if len(src) > 0 {
			l.throw("Infinite loop on byte: " + strconv.Itoa(int(src[0])))
		}
	}

	l.state.top = true
	return tokens
}

// inline is Lexer.inline: it queues a source for the inline lexing. tokens
// is the array of the token that receives the result.
func (l *lexer) inline(src string, tokens *[]*Token) {
	l.inlineQueue = append(l.inlineQueue, inlineQueueItem{src: src, tokens: tokens})
}

// inlineTokens is Lexer.inlineTokens: the inline lexing.
func (l *lexer) inlineTokens(src []rune, tokens []*Token) []*Token {
	rules := l.tokenizer.rules

	// String with links masked to avoid interference with em and strong
	maskedSrc := src

	// Mask out reflinks
	if len(l.links) > 0 {
		re := rules.inline.reflinkSearch
		for lastIndex := 0; ; {
			match := re.execAt(maskedSrc, lastIndex)
			if match == nil {
				break
			}
			lastIndex = match.Index + match.Length
			match0 := match.Runes()
			key := slice(match0, lastIndexOf(match0, '[')+1, -1)
			if _, ok := l.links[str(key)]; ok {
				maskedSrc = concat(
					maskedSrc[:match.Index],
					[]rune{'['}, repeat('a', len(match0)-2), []rune{']'},
					maskedSrc[lastIndex:],
				)
			}
		}
	}

	// Mask out escaped characters
	if indexOf(maskedSrc, '\\') >= 0 {
		re := rules.inline.anyPunctuation
		for lastIndex := 0; ; {
			u := newUString(maskedSrc)
			match, ok := re.execU(&u, lastIndex)
			if !ok {
				break
			}
			lastIndex = match.end()
			maskedSrc = concat(maskedSrc[:match.index()], []rune("++"), maskedSrc[lastIndex:])
		}
	}

	// Mask out other blocks
	{
		re := rules.inline.blockSkip
		for lastIndex := 0; ; {
			match := re.execAt(maskedSrc, lastIndex)
			if match == nil {
				break
			}
			lastIndex = match.Index + match.Length
			maskedSrc = concat(
				maskedSrc[:match.Index],
				[]rune{'['}, repeat('a', match.Length-2), []rune{']'},
				maskedSrc[lastIndex:],
			)
		}
	}

	keepPrevChar := false
	var prevChar []rune
	for len(src) > 0 {
		if !keepPrevChar {
			prevChar = nil
		}
		keepPrevChar = false

		var token *Token

		// escape
		if token = l.tokenizer.escape(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// tag
		if token = l.tokenizer.tag(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// link
		if token = l.tokenizer.link(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// reflink, nolink
		if token = l.tokenizer.reflink(src, l.links); token != nil {
			src = l.consume(src, token)
			lastToken := last(tokens)
			if token.Type == "text" && lastToken != nil && lastToken.Type == "text" {
				lastToken.Raw = js.Concat(lastToken.Raw, token.Raw)
				lastToken.Text = js.Concat(lastToken.Text, token.Text)
			} else {
				tokens = append(tokens, token)
			}
			continue
		}

		// em & strong
		if token = l.tokenizer.emStrong(src, maskedSrc, prevChar); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// code
		if token = l.tokenizer.codespan(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// br
		if token = l.tokenizer.br(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// del (gfm)
		if token = l.tokenizer.del(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// autolink
		if token = l.tokenizer.autolink(src); token != nil {
			src = l.consume(src, token)
			tokens = append(tokens, token)
			continue
		}

		// url (gfm)
		if !l.state.inLink {
			if token = l.tokenizer.url(src); token != nil {
				src = l.consume(src, token)
				tokens = append(tokens, token)
				continue
			}
		}

		// text
		if token = l.tokenizer.inlineText(src); token != nil {
			raw := src[:min(token.n, len(src))]
			src = l.consume(src, token)
			if rawLast := sliceFrom(raw, -1); !equal(rawLast, "_") { // Track prevChar before string of ____ started
				prevChar = rawLast
			}
			keepPrevChar = true
			lastToken := last(tokens)
			if lastToken != nil && lastToken.Type == "text" {
				lastToken.Raw = js.Concat(lastToken.Raw, token.Raw)
				lastToken.Text = js.Concat(lastToken.Text, token.Text)
			} else {
				tokens = append(tokens, token)
			}
			continue
		}

		if len(src) > 0 {
			l.throw("Infinite loop on byte: " + strconv.Itoa(int(src[0])))
		}
	}

	return tokens
}
