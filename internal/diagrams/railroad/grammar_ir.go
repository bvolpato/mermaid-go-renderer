// Ported from mermaid 12.2.0,
// packages/parser/src/language/railroad/railroad.langium and
// valueConverter.ts: the grammar of the diagram type railroad
// ("railroad-beta", the intermediate representation syntax) (MIT).

package railroad

import "github.com/bvolpato/mermaid-go-renderer/internal/langium"

// railroadAST is the AST node Railroad.
type railroadAST struct {
	langium.Common
	rules []*railroadRule
}

// railroadRule is the AST node RailroadRule.
type railroadRule struct {
	name       string
	definition *railroadExpr
}

// railroadExpr is the AST node RailroadExpression. typ is its $type, and it
// says which of the other fields the node has.
type railroadExpr struct {
	// typ is "RailroadSequenceExpr", "RailroadChoiceExpr",
	// "RailroadOptionalExpr", "RailroadOneOrMoreExpr",
	// "RailroadZeroOrMoreExpr", "RailroadTerminalExpr",
	// "RailroadNonTerminalExpr" or "RailroadSpecialExpr".
	typ string
	// elements is the property of RailroadSequenceExpr.
	elements []*railroadExpr
	// alternatives is the property of RailroadChoiceExpr.
	alternatives []*railroadExpr
	// element is the property of RailroadOptionalExpr, RailroadOneOrMoreExpr
	// and RailroadZeroOrMoreExpr.
	element *railroadExpr
	// value is the property of RailroadTerminalExpr.
	value string
	// name is the property of RailroadNonTerminalExpr.
	name string
	// text is the property of RailroadSpecialExpr.
	text string
}

var irLexer = langium.MustLexer("railroad")

// parseRailroad is createRailroadServices().Railroad.parser.LangiumParser
// .parse(text). It returns an error where the real parser reports a lexer
// error or a parser error. The grammar is:
//
//	entry Railroad returns Railroad:
//	    'railroad-beta'
//	    ((title=TITLE | accTitle=ACC_TITLE | accDescr=ACC_DESCR))*
//	    rules+=RailroadRule*
//	;
func parseRailroad(text string) (*railroadAST, error) {
	p, err := irLexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &railroadAST{rules: []*railroadRule{}}
	if err := header(p, "railroad-beta", &ast.Common, decodeEscapedString); err != nil {
		return nil, err
	}
	for p.At("RR_ID") {
		rule, err := parseRailroadRule(p)
		if err != nil {
			return nil, err
		}
		ast.rules = append(ast.rules, rule)
	}
	return ast, p.End()
}

// parseRailroadRule is the rule
//
//	RailroadRule returns RailroadRule:
//	    name=RR_ID '=' definition=RailroadExpression ';'
//	;
func parseRailroadRule(p *langium.Parser) (*railroadRule, error) {
	name, err := p.Expect("RR_ID")
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect("="); err != nil {
		return nil, err
	}
	definition, err := parseRailroadExpression(p)
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(";"); err != nil {
		return nil, err
	}
	return &railroadRule{name: name.Image, definition: definition}, nil
}

// parseRailroadExpression is the rule RailroadExpression and its eight
// alternatives. Each alternative starts with its own keyword:
//
//	RailroadSequenceExpr:    'sequence' '(' elements+=RailroadExpression (',' elements+=RailroadExpression)* ')'
//	RailroadChoiceExpr:      'choice' '(' alternatives+=RailroadExpression (',' alternatives+=RailroadExpression)* ')'
//	RailroadOptionalExpr:    'optional' '(' element=RailroadExpression ')'
//	RailroadOneOrMoreExpr:   'oneOrMore' '(' element=RailroadExpression ')'
//	RailroadZeroOrMoreExpr:  'zeroOrMore' '(' element=RailroadExpression ')'
//	RailroadTerminalExpr:    'terminal' '(' value=RR_STRING ')'
//	RailroadNonTerminalExpr: 'nonterminal' '(' name=RR_STRING ')'
//	RailroadSpecialExpr:     'special' '(' text=RR_STRING ')'
func parseRailroadExpression(p *langium.Parser) (*railroadExpr, error) {
	if !p.At("sequence", "choice", "optional", "oneOrMore", "zeroOrMore", "terminal", "nonterminal", "special") {
		return nil, p.Unexpected("sequence", "choice", "optional", "oneOrMore", "zeroOrMore", "terminal", "nonterminal", "special")
	}
	keyword := p.Next().Type.Name
	if _, err := p.Expect("("); err != nil {
		return nil, err
	}
	expr := &railroadExpr{}
	switch keyword {
	case "sequence", "choice":
		list := []*railroadExpr{}
		for {
			item, err := parseRailroadExpression(p)
			if err != nil {
				return nil, err
			}
			list = append(list, item)
			if p.Accept(",") == nil {
				break
			}
		}
		if keyword == "sequence" {
			expr.typ = "RailroadSequenceExpr"
			expr.elements = list
		} else {
			expr.typ = "RailroadChoiceExpr"
			expr.alternatives = list
		}
	case "optional", "oneOrMore", "zeroOrMore":
		element, err := parseRailroadExpression(p)
		if err != nil {
			return nil, err
		}
		expr.element = element
		switch keyword {
		case "optional":
			expr.typ = "RailroadOptionalExpr"
		case "oneOrMore":
			expr.typ = "RailroadOneOrMoreExpr"
		default:
			expr.typ = "RailroadZeroOrMoreExpr"
		}
	default:
		str, err := p.Expect("RR_STRING")
		if err != nil {
			return nil, err
		}
		// The converter of RR_STRING is decodeEscapedString.
		value := decodeEscapedString(str.Image)
		switch keyword {
		case "terminal":
			expr.typ = "RailroadTerminalExpr"
			expr.value = value
		case "nonterminal":
			expr.typ = "RailroadNonTerminalExpr"
			expr.name = value
		default:
			expr.typ = "RailroadSpecialExpr"
			expr.text = value
		}
	}
	if _, err := p.Expect(")"); err != nil {
		return nil, err
	}
	return expr, nil
}
