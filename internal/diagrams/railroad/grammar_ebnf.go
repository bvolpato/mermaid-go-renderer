// Ported from mermaid 12.2.0,
// packages/parser/src/language/railroad-ebnf/railroad-ebnf.langium and
// valueConverter.ts: the grammar of the diagram type railroadEbnf
// ("railroad-ebnf-beta") (MIT).

package railroad

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// ebnfAST is the AST node RailroadEbnf.
type ebnfAST struct {
	langium.Common
	rules []*ebnfRule
}

// ebnfRule is the AST node EbnfRule.
type ebnfRule struct {
	name       string
	definition *ebnfChoice
}

// ebnfChoice is the AST node EbnfChoice.
type ebnfChoice struct {
	alternatives []*ebnfSequence
}

// ebnfSequence is the AST node EbnfSequence.
type ebnfSequence struct {
	elements []*ebnfTerm
}

// ebnfTerm is the AST node EbnfTerm.
type ebnfTerm struct {
	base      *ebnfPrimary
	postfixes []*ebnfPostfix
}

// ebnfPrimary is the AST node EbnfPrimary. typ is its $type.
type ebnfPrimary struct {
	// typ is "EbnfTerminal", "EbnfNonTerminal", "EbnfSpecial", "EbnfGroup",
	// "EbnfOptional" or "EbnfRepetition".
	typ string
	// value is the property of EbnfTerminal.
	value string
	// name is the property of EbnfNonTerminal.
	name string
	// text is the property of EbnfSpecial.
	text string
	// element is the property of EbnfGroup, EbnfOptional and EbnfRepetition.
	element *ebnfChoice
}

// ebnfPostfix is the AST node EbnfPostfix. typ is its $type.
type ebnfPostfix struct {
	// typ is "EbnfOptionalPostfix", "EbnfZeroOrMorePostfix",
	// "EbnfOneOrMorePostfix" or "EbnfExceptionPostfix".
	typ string
	// operator is the property of the first three types: "?", "*" or "+".
	operator string
	// except is the property of EbnfExceptionPostfix.
	except *ebnfPrimary
}

var ebnfLexer = langium.MustLexer("railroadEbnf")

// parseEbnf is createRailroadEbnfServices().RailroadEbnf.parser
// .LangiumParser.parse(text). The grammar is:
//
//	entry RailroadEbnf returns RailroadEbnf:
//	    'railroad-ebnf-beta'
//	    ((title=TITLE | accTitle=ACC_TITLE | accDescr=ACC_DESCR))*
//	    rules+=EbnfRule*
//	;
func parseEbnf(text string) (*ebnfAST, error) {
	p, err := ebnfLexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &ebnfAST{rules: []*ebnfRule{}}
	if err := header(p, "railroad-ebnf-beta", &ast.Common, decodeEscapedString); err != nil {
		return nil, err
	}
	for p.At("EBNF_ID") {
		rule, err := parseEbnfRule(p)
		if err != nil {
			return nil, err
		}
		ast.rules = append(ast.rules, rule)
	}
	return ast, p.End()
}

// parseEbnfRule is the rule
//
//	EbnfRule returns EbnfRule:
//	    name=EBNF_ID ('=' | '::=') definition=EbnfChoice ';'
//	;
func parseEbnfRule(p *langium.Parser) (*ebnfRule, error) {
	name, err := p.Expect("EBNF_ID")
	if err != nil {
		return nil, err
	}
	if !p.At("=", "::=") {
		return nil, p.Unexpected("=", "::=")
	}
	p.Next()
	definition, err := parseEbnfChoice(p)
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(";"); err != nil {
		return nil, err
	}
	return &ebnfRule{name: name.Image, definition: definition}, nil
}

// parseEbnfChoice is the rule
//
//	EbnfChoice returns EbnfChoice:
//	    alternatives+=EbnfSequence ('|' alternatives+=EbnfSequence)*
//	;
func parseEbnfChoice(p *langium.Parser) (*ebnfChoice, error) {
	choice := &ebnfChoice{}
	for {
		sequence, err := parseEbnfSequence(p)
		if err != nil {
			return nil, err
		}
		choice.alternatives = append(choice.alternatives, sequence)
		if p.Accept("|") == nil {
			return choice, nil
		}
	}
}

// atEbnfPrimary reports whether the next token starts an EbnfPrimary.
func atEbnfPrimary(p *langium.Parser) bool {
	return p.At("EBNF_STRING", "EBNF_SPECIAL_SEQUENCE", "EBNF_ID", "(", "[", "{")
}

// parseEbnfSequence is the rule
//
//	EbnfSequence returns EbnfSequence:
//	    elements+=EbnfTerm (','? elements+=EbnfTerm)*
//	;
func parseEbnfSequence(p *langium.Parser) (*ebnfSequence, error) {
	sequence := &ebnfSequence{}
	term, err := parseEbnfTerm(p)
	if err != nil {
		return nil, err
	}
	sequence.elements = append(sequence.elements, term)
	// The loop goes on with a comma or with the start of a term. No token
	// that can follow a sequence is one of them.
	for p.At(",") || atEbnfPrimary(p) {
		p.Accept(",")
		term, err := parseEbnfTerm(p)
		if err != nil {
			return nil, err
		}
		sequence.elements = append(sequence.elements, term)
	}
	return sequence, nil
}

// parseEbnfTerm is the rule EbnfTerm, with the rule EbnfPostfix and its
// four alternatives:
//
//	EbnfTerm returns EbnfTerm:
//	    base=EbnfPrimary postfixes+=EbnfPostfix*
//	;
//	EbnfOptionalPostfix:   operator='?'
//	EbnfZeroOrMorePostfix: operator='*'
//	EbnfOneOrMorePostfix:  operator='+'
//	EbnfExceptionPostfix:  '-' except=EbnfPrimary
func parseEbnfTerm(p *langium.Parser) (*ebnfTerm, error) {
	base, err := parseEbnfPrimary(p)
	if err != nil {
		return nil, err
	}
	term := &ebnfTerm{base: base, postfixes: []*ebnfPostfix{}}
	for {
		switch {
		case p.At("?"):
			p.Next()
			term.postfixes = append(term.postfixes, &ebnfPostfix{typ: "EbnfOptionalPostfix", operator: "?"})
		case p.At("*"):
			p.Next()
			term.postfixes = append(term.postfixes, &ebnfPostfix{typ: "EbnfZeroOrMorePostfix", operator: "*"})
		case p.At("+"):
			p.Next()
			term.postfixes = append(term.postfixes, &ebnfPostfix{typ: "EbnfOneOrMorePostfix", operator: "+"})
		case p.At("-"):
			p.Next()
			except, err := parseEbnfPrimary(p)
			if err != nil {
				return nil, err
			}
			term.postfixes = append(term.postfixes, &ebnfPostfix{typ: "EbnfExceptionPostfix", except: except})
		default:
			return term, nil
		}
	}
}

// parseEbnfPrimary is the rule EbnfPrimary and its six alternatives:
//
//	EbnfTerminal:    value=EBNF_STRING
//	EbnfSpecial:     text=EBNF_SPECIAL_SEQUENCE
//	EbnfNonTerminal: name=EBNF_ID
//	EbnfGroup:       '(' element=EbnfChoice ')'
//	EbnfOptional:    '[' element=EbnfChoice ']'
//	EbnfRepetition:  '{' element=EbnfChoice '}'
func parseEbnfPrimary(p *langium.Parser) (*ebnfPrimary, error) {
	switch {
	case p.At("EBNF_STRING"):
		// The converter of EBNF_STRING is decodeEscapedString.
		return &ebnfPrimary{typ: "EbnfTerminal", value: decodeEscapedString(p.Next().Image)}, nil
	case p.At("EBNF_SPECIAL_SEQUENCE"):
		// The converter of EBNF_SPECIAL_SEQUENCE is input.slice(1, -1).trim().
		return &ebnfPrimary{typ: "EbnfSpecial", text: js.TrimSpace(sliceInner(p.Next().Image))}, nil
	case p.At("EBNF_ID"):
		return &ebnfPrimary{typ: "EbnfNonTerminal", name: p.Next().Image}, nil
	case p.At("(", "[", "{"):
		open := p.Next().Type.Name
		typ, closing := "EbnfGroup", ")"
		switch open {
		case "[":
			typ, closing = "EbnfOptional", "]"
		case "{":
			typ, closing = "EbnfRepetition", "}"
		}
		element, err := parseEbnfChoice(p)
		if err != nil {
			return nil, err
		}
		if _, err := p.Expect(closing); err != nil {
			return nil, err
		}
		return &ebnfPrimary{typ: typ, element: element}, nil
	}
	return nil, p.Unexpected("EBNF_STRING", "EBNF_SPECIAL_SEQUENCE", "EBNF_ID", "(", "[", "{")
}
