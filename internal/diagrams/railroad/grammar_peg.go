// Ported from mermaid 12.2.0,
// packages/parser/src/language/railroad-peg/railroad-peg.langium and
// valueConverter.ts: the grammar of the diagram type railroadPeg
// ("railroad-peg-beta") (MIT).

package railroad

import "github.com/bvolpato/mermaid-go-renderer/internal/langium"

// pegAST is the AST node RailroadPeg.
type pegAST struct {
	langium.Common
	rules []*pegRule
}

// pegRule is the AST node PegRule.
type pegRule struct {
	name       string
	definition *pegOrderedChoice
}

// pegOrderedChoice is the AST node PegOrderedChoice.
type pegOrderedChoice struct {
	alternatives []*pegSequence
}

// pegSequence is the AST node PegSequence.
type pegSequence struct {
	elements []*pegPrefix
}

// pegPrefix is the AST node PegPrefix.
type pegPrefix struct {
	// operator is "&" or "!", or "" while the property is undefined.
	operator string
	suffix   *pegSuffix
}

// pegSuffix is the AST node PegSuffix.
type pegSuffix struct {
	primary *pegPrimary
	// operator is "?", "*" or "+", or "" while the property is undefined.
	operator string
}

// pegPrimary is the AST node PegPrimary. typ is its $type.
type pegPrimary struct {
	// typ is "PegLiteral", "PegIdentifier", "PegGroup" or "PegAny".
	typ string
	// value is the property of PegLiteral.
	value string
	// name is the property of PegIdentifier.
	name string
	// element is the property of PegGroup.
	element *pegOrderedChoice
	// dot is the property of PegAny: ".".
	dot string
}

var pegLexer = langium.MustLexer("railroadPeg")

// parsePeg is createRailroadPegServices().RailroadPeg.parser.LangiumParser
// .parse(text). The grammar is:
//
//	entry RailroadPeg returns RailroadPeg:
//	    'railroad-peg-beta'
//	    ((title=TITLE | accTitle=ACC_TITLE | accDescr=ACC_DESCR))*
//	    rules+=PegRule*
//	;
func parsePeg(text string) (*pegAST, error) {
	p, err := pegLexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &pegAST{rules: []*pegRule{}}
	if err := header(p, "railroad-peg-beta", &ast.Common, decodeEscapedString); err != nil {
		return nil, err
	}
	for p.At("PEG_ID") {
		rule, err := parsePegRule(p)
		if err != nil {
			return nil, err
		}
		ast.rules = append(ast.rules, rule)
	}
	return ast, p.End()
}

// parsePegRule is the rule
//
//	PegRule returns PegRule:
//	    name=PEG_ID '<-' definition=PegOrderedChoice ';'
//	;
func parsePegRule(p *langium.Parser) (*pegRule, error) {
	name, err := p.Expect("PEG_ID")
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect("<-"); err != nil {
		return nil, err
	}
	definition, err := parsePegOrderedChoice(p)
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(";"); err != nil {
		return nil, err
	}
	return &pegRule{name: name.Image, definition: definition}, nil
}

// parsePegOrderedChoice is the rule
//
//	PegOrderedChoice returns PegOrderedChoice:
//	    alternatives+=PegSequence ('/' alternatives+=PegSequence)*
//	;
func parsePegOrderedChoice(p *langium.Parser) (*pegOrderedChoice, error) {
	choice := &pegOrderedChoice{}
	for {
		sequence, err := parsePegSequence(p)
		if err != nil {
			return nil, err
		}
		choice.alternatives = append(choice.alternatives, sequence)
		if p.Accept("/") == nil {
			return choice, nil
		}
	}
}

// atPegPrefix reports whether the next token starts a PegPrefix.
func atPegPrefix(p *langium.Parser) bool {
	return p.At("&", "!", "PEG_STRING", "PEG_ID", "(", ".")
}

// parsePegSequence is the rule
//
//	PegSequence returns PegSequence:
//	    elements+=PegPrefix+
//	;
func parsePegSequence(p *langium.Parser) (*pegSequence, error) {
	sequence := &pegSequence{}
	for {
		prefix, err := parsePegPrefix(p)
		if err != nil {
			return nil, err
		}
		sequence.elements = append(sequence.elements, prefix)
		if !atPegPrefix(p) {
			return sequence, nil
		}
	}
}

// parsePegPrefix is the rules PegPrefix and PegSuffix:
//
//	PegPrefix returns PegPrefix:
//	    (operator='&' | operator='!')? suffix=PegSuffix
//	;
//	PegSuffix returns PegSuffix:
//	    primary=PegPrimary (operator='?' | operator='*' | operator='+')?
//	;
func parsePegPrefix(p *langium.Parser) (*pegPrefix, error) {
	prefix := &pegPrefix{}
	if p.At("&", "!") {
		prefix.operator = p.Next().Type.Name
	}
	primary, err := parsePegPrimary(p)
	if err != nil {
		return nil, err
	}
	prefix.suffix = &pegSuffix{primary: primary}
	if p.At("?", "*", "+") {
		prefix.suffix.operator = p.Next().Type.Name
	}
	return prefix, nil
}

// parsePegPrimary is the rule PegPrimary and its four alternatives:
//
//	PegLiteral:    value=PEG_STRING
//	PegIdentifier: name=PEG_ID
//	PegGroup:      '(' element=PegOrderedChoice ')'
//	PegAny:        dot='.'
func parsePegPrimary(p *langium.Parser) (*pegPrimary, error) {
	switch {
	case p.At("PEG_STRING"):
		// The converter of PEG_STRING is decodeEscapedString.
		return &pegPrimary{typ: "PegLiteral", value: decodeEscapedString(p.Next().Image)}, nil
	case p.At("PEG_ID"):
		return &pegPrimary{typ: "PegIdentifier", name: p.Next().Image}, nil
	case p.At("("):
		p.Next()
		element, err := parsePegOrderedChoice(p)
		if err != nil {
			return nil, err
		}
		if _, err := p.Expect(")"); err != nil {
			return nil, err
		}
		return &pegPrimary{typ: "PegGroup", element: element}, nil
	case p.At("."):
		p.Next()
		return &pegPrimary{typ: "PegAny", dot: "."}, nil
	}
	return nil, p.Unexpected("PEG_STRING", "PEG_ID", "(", ".")
}
