// Ported from mermaid 12.2.0,
// packages/parser/src/language/railroad-abnf/railroad-abnf.langium and
// valueConverter.ts: the grammar of the diagram type railroadAbnf
// ("railroad-abnf-beta") (MIT).

package railroad

import "github.com/bvolpato/mermaid-go-renderer/internal/langium"

// abnfAST is the AST node RailroadAbnf.
type abnfAST struct {
	langium.Common
	rules []*abnfRule
}

// abnfRule is the AST node AbnfRule.
type abnfRule struct {
	name       string
	definition *abnfAlternation
}

// abnfAlternation is the AST node AbnfAlternation.
type abnfAlternation struct {
	alternatives []*abnfConcatenation
}

// abnfConcatenation is the AST node AbnfConcatenation.
type abnfConcatenation struct {
	elements []*abnfElement
}

// abnfElement is the AST node AbnfElement.
type abnfElement struct {
	// repeat is the text of the repeat token. hasRepeat is false while the
	// property is undefined.
	repeat    string
	hasRepeat bool
	primary   *abnfPrimary
}

// abnfPrimary is the AST node AbnfPrimary. typ is its $type.
type abnfPrimary struct {
	// typ is "AbnfStringLiteral", "AbnfNumVal", "AbnfRuleName", "AbnfGroup"
	// or "AbnfOptionalGroup".
	typ string
	// value is the property of AbnfStringLiteral and AbnfNumVal.
	value string
	// name is the property of AbnfRuleName.
	name string
	// element is the property of AbnfGroup and AbnfOptionalGroup.
	element *abnfAlternation
}

var abnfLexer = langium.MustLexer("railroadAbnf")

// parseAbnf is createRailroadAbnfServices().RailroadAbnf.parser
// .LangiumParser.parse(text). The grammar is:
//
//	entry RailroadAbnf returns RailroadAbnf:
//	    'railroad-abnf-beta'
//	    ((title=TITLE | accTitle=ACC_TITLE | accDescr=ACC_DESCR))*
//	    rules+=AbnfRule*
//	;
//
// The lexer reads ";" with more text on the same line as a comment. So the
// ";" that ends a rule must be the last text of its line.
func parseAbnf(text string) (*abnfAST, error) {
	p, err := abnfLexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &abnfAST{rules: []*abnfRule{}}
	// The converter of a quoted TITLE is trimmedValue.slice(1, -1).
	if err := header(p, "railroad-abnf-beta", &ast.Common, sliceInner); err != nil {
		return nil, err
	}
	for p.At("ABNF_RULENAME") {
		rule, err := parseAbnfRule(p)
		if err != nil {
			return nil, err
		}
		ast.rules = append(ast.rules, rule)
	}
	return ast, p.End()
}

// parseAbnfRule is the rule
//
//	AbnfRule returns AbnfRule:
//	    name=ABNF_RULENAME '=' definition=AbnfAlternation ';'
//	;
func parseAbnfRule(p *langium.Parser) (*abnfRule, error) {
	name, err := p.Expect("ABNF_RULENAME")
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect("="); err != nil {
		return nil, err
	}
	definition, err := parseAbnfAlternation(p)
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(";"); err != nil {
		return nil, err
	}
	return &abnfRule{name: name.Image, definition: definition}, nil
}

// parseAbnfAlternation is the rule
//
//	AbnfAlternation returns AbnfAlternation:
//	    alternatives+=AbnfConcatenation ('/' alternatives+=AbnfConcatenation)*
//	;
func parseAbnfAlternation(p *langium.Parser) (*abnfAlternation, error) {
	alternation := &abnfAlternation{}
	for {
		concatenation, err := parseAbnfConcatenation(p)
		if err != nil {
			return nil, err
		}
		alternation.alternatives = append(alternation.alternatives, concatenation)
		if p.Accept("/") == nil {
			return alternation, nil
		}
	}
}

// atAbnfElement reports whether the next token starts an AbnfElement.
func atAbnfElement(p *langium.Parser) bool {
	return p.At("ABNF_REPEAT", "ABNF_EXACT_REPEAT", "ABNF_STRING", "ABNF_NUMVAL", "ABNF_RULENAME", "(", "[")
}

// parseAbnfConcatenation is the rule
//
//	AbnfConcatenation returns AbnfConcatenation:
//	    elements+=AbnfElement+
//	;
func parseAbnfConcatenation(p *langium.Parser) (*abnfConcatenation, error) {
	concatenation := &abnfConcatenation{}
	for {
		element, err := parseAbnfElement(p)
		if err != nil {
			return nil, err
		}
		concatenation.elements = append(concatenation.elements, element)
		if !atAbnfElement(p) {
			return concatenation, nil
		}
	}
}

// parseAbnfElement is the rule AbnfElement, with the rule AbnfPrimary and
// its five alternatives:
//
//	AbnfElement returns AbnfElement:
//	    (repeat=ABNF_REPEAT | repeat=ABNF_EXACT_REPEAT)? primary=AbnfPrimary
//	;
//	AbnfStringLiteral: value=ABNF_STRING
//	AbnfNumVal:        value=ABNF_NUMVAL
//	AbnfRuleName:      name=ABNF_RULENAME
//	AbnfGroup:         '(' element=AbnfAlternation ')'
//	AbnfOptionalGroup: '[' element=AbnfAlternation ']'
func parseAbnfElement(p *langium.Parser) (*abnfElement, error) {
	element := &abnfElement{}
	if p.At("ABNF_REPEAT", "ABNF_EXACT_REPEAT") {
		element.repeat = p.Next().Image
		element.hasRepeat = true
	}
	switch {
	case p.At("ABNF_STRING"):
		// The converter of ABNF_STRING is input.slice(1, -1).
		element.primary = &abnfPrimary{typ: "AbnfStringLiteral", value: sliceInner(p.Next().Image)}
	case p.At("ABNF_NUMVAL"):
		element.primary = &abnfPrimary{typ: "AbnfNumVal", value: p.Next().Image}
	case p.At("ABNF_RULENAME"):
		element.primary = &abnfPrimary{typ: "AbnfRuleName", name: p.Next().Image}
	case p.At("(", "["):
		open := p.Next().Type.Name
		typ, closing := "AbnfGroup", ")"
		if open == "[" {
			typ, closing = "AbnfOptionalGroup", "]"
		}
		alternation, err := parseAbnfAlternation(p)
		if err != nil {
			return nil, err
		}
		if _, err := p.Expect(closing); err != nil {
			return nil, err
		}
		element.primary = &abnfPrimary{typ: typ, element: alternation}
	default:
		return nil, p.Unexpected("ABNF_STRING", "ABNF_NUMVAL", "ABNF_RULENAME", "(", "[")
	}
	return element, nil
}
