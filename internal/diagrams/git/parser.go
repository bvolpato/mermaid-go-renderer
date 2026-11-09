// Ported from mermaid 12.2.0, the grammar
// parser/src/language/gitGraph/gitGraph.langium, the fragments of
// common/common.langium, the value converters of common/valueConverter.ts and
// common/matcher.ts, and the default value converter of Langium
// (convertString, convertInt) (MIT).

package git

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// gitGraphAST is the GitGraph node that the Langium parser returns. A nil
// string pointer is a property that is undefined.
type gitGraphAST struct {
	accDescr   *string
	accTitle   *string
	title      *string
	dir        *string
	statements []statement
}

// statement is one of commitAST, branchAST, mergeAST, checkoutAST and
// cherryPickingAST.
type statement interface{ astType() string }

type commitAST struct {
	id      *string
	message *string
	tags    []string
	typ     *string
}

type branchAST struct {
	name  string
	order *float64
}

type mergeAST struct {
	branch string
	id     *string
	tags   []string
	typ    *string
}

type checkoutAST struct {
	branch string
}

type cherryPickingAST struct {
	id     *string
	tags   []string
	parent *string
}

func (*commitAST) astType() string        { return "Commit" }
func (*branchAST) astType() string        { return "Branch" }
func (*mergeAST) astType() string         { return "Merge" }
func (*checkoutAST) astType() string      { return "Checkout" }
func (*cherryPickingAST) astType() string { return "CherryPicking" }

// convertString is ValueConverter.convertString: it removes the quotes and
// resolves the escapes.
func convertString(input string) string {
	if len(input) < 2 {
		return ""
	}
	body := input[1 : len(input)-1]
	if !strings.Contains(body, `\`) {
		return body
	}
	var result strings.Builder
	for i := 0; i < len(body); {
		r, size := utf8.DecodeRuneInString(body[i:])
		i += size
		if r != '\\' {
			result.WriteString(body[i-size : i])
			continue
		}
		if i >= len(body) {
			// The escaped character is the closing quote position:
			// charAt gives "". The lexer does not produce this.
			break
		}
		c1, size1 := utf8.DecodeRuneInString(body[i:])
		// convertEscapeCharacter
		switch c1 {
		case 'b':
			result.WriteByte('\b')
		case 'f':
			result.WriteByte('\f')
		case 'n':
			result.WriteByte('\n')
		case 'r':
			result.WriteByte('\r')
		case 't':
			result.WriteByte('\t')
		case 'v':
			result.WriteByte('\v')
		case '0':
			result.WriteByte(0)
		default:
			result.WriteString(body[i : i+size1])
		}
		i += size1
	}
	return result.String()
}

// convertInt is ValueConverter.convertInt: parseInt(input). The input has
// decimal digits only.
func convertInt(input string) float64 {
	return js.Number(input)
}

var (
	reBlanks     = regexp.MustCompile(`[\t ]{2,}`)
	reLineBreaks = regexp.MustCompile(`[\n\r]{2,}`)
)

// isLineTerminator reports whether r ends a line for the ^ and $ of a
// JavaScript regular expression with the m flag.
func isLineTerminator(r rune) bool {
	return r == '\n' || r == '\r' || r == 0x2028 || r == 0x2029
}

// removeLineIndent removes the white space at the start of each line:
// text.replace(/^\s*/gm, "").
func removeLineIndent(text string) string {
	var out strings.Builder
	prev := rune(-1)
	for i := 0; i < len(text); {
		if i == 0 || isLineTerminator(prev) {
			if j := skipSpace(text, i); j > i {
				prev, _ = utf8.DecodeLastRuneInString(text[:j])
				i = j
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		out.WriteString(text[i : i+size])
		prev = r
		i += size
	}
	return out.String()
}

// removeLineTrail removes the white space at the end of each line:
// text.replace(/\s+$/gm, "").
func removeLineTrail(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !js.IsWhiteSpace(r) {
			out.WriteString(text[i : i+size])
			i += size
			continue
		}
		// A run of white space from i to j. The match ends at the last
		// place of the run that is the end of a line.
		j := skipSpace(text, i)
		if j == len(text) {
			break
		}
		keepFrom := i
		for k := i + size; k < j; {
			kr, ksize := utf8.DecodeRuneInString(text[k:])
			if isLineTerminator(kr) {
				keepFrom = k
			}
			k += ksize
		}
		out.WriteString(text[keepFrom:j])
		i = j
	}
	return out.String()
}

// singleLineValue is the conversion of a single line title, accTitle or
// accDescr: match[1].trim().replace(/[\t ]{2,}/gm, ' ').
func singleLineValue(text string) string {
	return reBlanks.ReplaceAllString(js.TrimSpace(text), " ")
}

// convertAccDescr is the common converter for ACC_DESCR, with
// accessibilityDescrRegex: /accDescr(?:[\t ]*:([^\n\r]*)|\s*{([^}]*)})/.
func convertAccDescr(image string) string {
	p := strings.Index(image, "accDescr") + len("accDescr")
	if q := skipBlanks(image, p); q < len(image) && image[q] == ':' {
		return singleLineValue(image[q+1:])
	}
	// The multi line form: the text between the braces.
	open := strings.IndexByte(image[p:], '{')
	text := image[p+open+1 : len(image)-1]
	text = removeLineIndent(text)
	text = removeLineTrail(text)
	text = reBlanks.ReplaceAllString(text, " ")
	return reLineBreaks.ReplaceAllString(text, "\n")
}

// convertAccTitle is the common converter for ACC_TITLE, with
// accessibilityTitleRegex: /accTitle[\t ]*:([^\n\r]*)/.
func convertAccTitle(image string) string {
	p := strings.Index(image, "accTitle") + len("accTitle")
	p = skipBlanks(image, p)
	return singleLineValue(image[p+1:])
}

// convertTitle is the common converter for TITLE, with titleRegex:
// /title([\t ][^\n\r]*|)/.
func convertTitle(image string) string {
	p := strings.Index(image, "title") + len("title")
	return singleLineValue(image[p:])
}

// parser is the recursive descent parser for the grammar. Each decision of
// the grammar depends on the next token only, except the header.
type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() tokenType { return p.tokens[p.pos].typ }

func (p *parser) next() token {
	t := p.tokens[p.pos]
	if t.typ != tkEOF {
		p.pos++
	}
	return t
}

func (p *parser) fail(expected string) *lexError {
	t := p.tokens[p.pos]
	found := t.image
	if t.typ == tkEOF {
		found = ""
	}
	return &lexError{msg: "Expecting " + expected + " but found `" + found + "`.", offset: t.offset}
}

// expect consumes a token of the given type.
func (p *parser) expect(typ tokenType) (token, *lexError) {
	if p.peek() != typ {
		return token{}, p.fail("token of type '" + typ.String() + "'")
	}
	return p.next(), nil
}

// parseGitGraphText runs the lexer and the parser on the text of a diagram.
func parseGitGraphText(text string) (*gitGraphAST, error) {
	tokens, lexErr := tokenize(text)
	if lexErr != nil {
		return nil, lexErr.format(text)
	}
	p := &parser{tokens: tokens}
	ast, err := p.gitGraph()
	if err != nil {
		return nil, err.format(text)
	}
	return ast, nil
}

// gitGraph is the entry rule:
//
//	NEWLINE*
//	('gitGraph' | 'gitGraph' ':' | 'gitGraph:' | ('gitGraph' Direction ':'))
//	(NEWLINE | TitleAndAccessibilities | statements+=Statement)*
func (p *parser) gitGraph() (*gitGraphAST, *lexError) {
	ast := &gitGraphAST{statements: []statement{}}
	for p.peek() == tkNewline {
		p.next()
	}
	switch p.peek() {
	case tkGitGraphColon:
		p.next()
	case tkGitGraph:
		p.next()
		switch p.peek() {
		case tkColon:
			p.next()
		case tkLR, tkTB, tkBT:
			// Direction: dir=('LR' | 'TB' | 'BT')
			dir := p.next().image
			ast.dir = &dir
			if _, err := p.expect(tkColon); err != nil {
				return nil, err
			}
		}
	default:
		return nil, p.fail("keyword 'gitGraph'")
	}
	for {
		var err *lexError
		switch p.peek() {
		case tkNewline:
			p.next()
		case tkAccDescr, tkAccTitle, tkTitle:
			err = p.titleAndAccessibilities(ast)
		case tkCommit:
			err = p.commit(ast)
		case tkBranch:
			err = p.branch(ast)
		case tkMerge:
			err = p.merge(ast)
		case tkCheckout, tkSwitch:
			err = p.checkout(ast)
		case tkCherryPick:
			err = p.cherryPicking(ast)
		default:
			if _, err := p.expect(tkEOF); err != nil {
				return nil, err
			}
			return ast, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

// eol is the rule EOL: NEWLINE+ | EOF.
func (p *parser) eol() *lexError {
	switch p.peek() {
	case tkNewline:
		for p.peek() == tkNewline {
			p.next()
		}
		return nil
	case tkEOF:
		return nil
	}
	return p.fail("token of type 'EOF'")
}

// titleAndAccessibilities is the fragment
// ((accDescr=ACC_DESCR | accTitle=ACC_TITLE | title=TITLE) EOL)+.
func (p *parser) titleAndAccessibilities(ast *gitGraphAST) *lexError {
	for {
		switch p.peek() {
		case tkAccDescr:
			v := convertAccDescr(p.next().image)
			ast.accDescr = &v
		case tkAccTitle:
			v := convertAccTitle(p.next().image)
			ast.accTitle = &v
		case tkTitle:
			v := convertTitle(p.next().image)
			ast.title = &v
		default:
			return nil
		}
		if err := p.eol(); err != nil {
			return err
		}
	}
}

// stringValue reads a STRING token and converts it.
func (p *parser) stringValue() (string, *lexError) {
	t, err := p.expect(tkString)
	if err != nil {
		return "", err
	}
	return convertString(t.image), nil
}

// commitTypeValue reads ('NORMAL' | 'REVERSE' | 'HIGHLIGHT').
func (p *parser) commitTypeValue() (string, *lexError) {
	switch p.peek() {
	case tkNormal, tkReverse, tkHighlight:
		return p.next().image, nil
	}
	return "", p.fail("one of 'NORMAL', 'REVERSE', 'HIGHLIGHT'")
}

// branchName reads (REFERENCE | STRING).
func (p *parser) branchName() (string, *lexError) {
	switch p.peek() {
	case tkReference:
		return p.next().image, nil
	case tkString:
		return convertString(p.next().image), nil
	}
	return "", p.fail("one of REFERENCE, STRING")
}

// commit is the rule Commit:
//
//	'commit' ('id:' id=STRING | 'msg:'? message=STRING | 'tag:' tags+=STRING
//	  | 'type:' type=('NORMAL' | 'REVERSE' | 'HIGHLIGHT'))* EOL
func (p *parser) commit(ast *gitGraphAST) *lexError {
	p.next()
	node := &commitAST{tags: []string{}}
	for {
		switch p.peek() {
		case tkIDColon:
			p.next()
			v, err := p.stringValue()
			if err != nil {
				return err
			}
			node.id = &v
			continue
		case tkMsgColon:
			p.next()
			v, err := p.stringValue()
			if err != nil {
				return err
			}
			node.message = &v
			continue
		case tkString:
			v, err := p.stringValue()
			if err != nil {
				return err
			}
			node.message = &v
			continue
		case tkTagColon:
			p.next()
			v, err := p.stringValue()
			if err != nil {
				return err
			}
			node.tags = append(node.tags, v)
			continue
		case tkTypeColon:
			p.next()
			v, err := p.commitTypeValue()
			if err != nil {
				return err
			}
			node.typ = &v
			continue
		}
		break
	}
	if err := p.eol(); err != nil {
		return err
	}
	ast.statements = append(ast.statements, node)
	return nil
}

// branch is the rule Branch:
//
//	'branch' name=(REFERENCE|STRING) ('order:' order=INT)? EOL
func (p *parser) branch(ast *gitGraphAST) *lexError {
	p.next()
	name, err := p.branchName()
	if err != nil {
		return err
	}
	node := &branchAST{name: name}
	if p.peek() == tkOrderColon {
		p.next()
		t, err := p.expect(tkInt)
		if err != nil {
			return err
		}
		order := convertInt(t.image)
		node.order = &order
	}
	if err := p.eol(); err != nil {
		return err
	}
	ast.statements = append(ast.statements, node)
	return nil
}

// merge is the rule Merge:
//
//	'merge' branch=(REFERENCE|STRING) ('id:' id=STRING | 'tag:' tags+=STRING
//	  | 'type:' type=('NORMAL' | 'REVERSE' | 'HIGHLIGHT'))* EOL
func (p *parser) merge(ast *gitGraphAST) *lexError {
	p.next()
	name, err := p.branchName()
	if err != nil {
		return err
	}
	node := &mergeAST{branch: name, tags: []string{}}
	for {
		switch p.peek() {
		case tkIDColon:
			p.next()
			v, err := p.stringValue()
			if err != nil {
				return err
			}
			node.id = &v
			continue
		case tkTagColon:
			p.next()
			v, err := p.stringValue()
			if err != nil {
				return err
			}
			node.tags = append(node.tags, v)
			continue
		case tkTypeColon:
			p.next()
			v, err := p.commitTypeValue()
			if err != nil {
				return err
			}
			node.typ = &v
			continue
		}
		break
	}
	if err := p.eol(); err != nil {
		return err
	}
	ast.statements = append(ast.statements, node)
	return nil
}

// checkout is the rule Checkout:
//
//	('checkout'|'switch') branch=(REFERENCE|STRING) EOL
func (p *parser) checkout(ast *gitGraphAST) *lexError {
	p.next()
	name, err := p.branchName()
	if err != nil {
		return err
	}
	if err := p.eol(); err != nil {
		return err
	}
	ast.statements = append(ast.statements, &checkoutAST{branch: name})
	return nil
}

// cherryPicking is the rule CherryPicking:
//
//	'cherry-pick' ('id:' id=STRING | 'tag:' tags+=STRING
//	  | 'parent:' parent=STRING)* EOL
func (p *parser) cherryPicking(ast *gitGraphAST) *lexError {
	p.next()
	node := &cherryPickingAST{tags: []string{}}
	for {
		switch p.peek() {
		case tkIDColon:
			p.next()
			v, err := p.stringValue()
			if err != nil {
				return err
			}
			node.id = &v
			continue
		case tkTagColon:
			p.next()
			v, err := p.stringValue()
			if err != nil {
				return err
			}
			node.tags = append(node.tags, v)
			continue
		case tkParentColon:
			p.next()
			v, err := p.stringValue()
			if err != nil {
				return err
			}
			node.parent = &v
			continue
		}
		break
	}
	if err := p.eol(); err != nil {
		return err
	}
	ast.statements = append(ast.statements, node)
	return nil
}
